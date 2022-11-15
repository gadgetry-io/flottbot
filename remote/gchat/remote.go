// Copyright (c) 2022 Target Brands, Inc. All rights reserved.
//
// Use of this source code is governed by the LICENSE file in this repository.

package gchat

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/chat/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/target/flottbot/models"
	"github.com/target/flottbot/remote"
)

/*
=======================================
Implementation for the Remote interface
=======================================
*/

// Client struct.
type Client struct {
	Credentials    string
	ProjectID      string
	SubscriptionID string
	DomainAdmin    string

	msgService   *chat.SpacesMessagesService
	adminService *admin.Service
	ctx          context.Context
}

// validate that Client adheres to remote interface.
var _ remote.Remote = (*Client)(nil)

// Name returns the name of the remote.
func (c *Client) Name() string {
	return "google_chat"
}

// Read messages from Google Chat.
func (c *Client) Read(inputMsgs chan<- models.Message, rules map[string]models.Rule, bot *models.Bot) {

	// init client
	client, err := pubsub.NewClient(c.ctx, c.ProjectID, option.WithCredentialsFile(c.Credentials))
	if err != nil {
		log.Error().Msgf("google_chat unable to authenticate: %s", err.Error())
	}

	sub := client.Subscription(c.SubscriptionID)

	err = sub.Receive(c.ctx, func(ctx context.Context, m *pubsub.Message) {
		defer m.Ack()

		// Convert Google Chat Message to Flottbot Message
		message, err := c.toMessage(m)
		if err != nil {
			log.Error().Msg(err.Error())
			return
		}

		// send to flotbot core for processing
		inputMsgs <- message
	})
	if err != nil {
		log.Fatal().Msgf("google_chat unable to create subscription against %s: %s", c.SubscriptionID, err.Error())
	}

	log.Info().Msgf("google_chat successfully subscribed to %s", c.SubscriptionID)
}

// Send messages to Google Chat.
func (c *Client) Send(message models.Message, bot *models.Bot) {
	// Best effort. If the instance goes away, so be it.
	msg := &chat.Message{
		Text: message.Output,
	}

	if message.ThreadID != "" {
		msg.Thread = &chat.Thread{
			Name: message.ThreadID,
		}
	}

	_, err := c.msgService.Create(message.ChannelID, msg).Do()
	if err != nil {
		log.Error().Msgf("google_chat failed to create message: %s", err.Error())
	}
}

// Reaction implementation to satisfy remote interface.
func (c *Client) Reaction(message models.Message, rule models.Rule, bot *models.Bot) {
	// Not implemented for Google Chat
}

func (c *Client) isMemberOfGroup(currentUserID string, userGroups []string, bot *models.Bot) (bool, error) {
	// Get groups for user
	listGroupsResponse, err := c.adminService.Groups.List().Customer("my_customer").Query(fmt.Sprintf("memberKey=%s", currentUserID)).MaxResults(200).Do()
	if err != nil {
		if e, ok := err.(*googleapi.Error); ok {
			switch e.Code {
			case 403:
				log.Fatal().Msgf("User %s is not a domain administrator with directory listing permissions.", c.DomainAdmin)
			}
		}

		log.Fatal().Msgf("Could not list Google groups for user %s using admin email %s: %v", currentUserID, c.DomainAdmin, err)
	}

	// Check user is in one of the rule's groups
	for _, group := range listGroupsResponse.Groups {
		for _, ruleGroup := range userGroups {
			if ruleGroup == group.Email {
				return true, nil
			}
		}
	}

	// User is not authorized
	return false, nil
}

func (c *Client) toMessage(m *pubsub.Message) (models.Message, error) {
	message := models.NewMessage()

	var event Message

	err := json.Unmarshal(m.Data, &event)
	if err != nil {
		return message, fmt.Errorf("google_chat was unable to parse event %s: %w", m.ID, err)
	}

	msgType, err := getMessageType(event)
	if err != nil {
		return message, err
	}

	stringTimestamp := event.EventTime.Format("2006-01-02 15:04:05")
	message.Type = msgType
	message.Timestamp = stringTimestamp

	if event.Type == "MESSAGE" {
		message.Input = strings.TrimPrefix(event.Message.ArgumentText, " ")
		message.ID = event.Message.Name
		message.Service = models.MsgServiceChat
		message.ChannelName = event.Space.Name
		message.ChannelID = event.Space.Name
		message.BotMentioned = true // Google Chat only supports @bot mentions
		message.DirectMessageOnly = event.Space.SingleUserBotDm

		if event.Space.SpaceThreadingState != "UNTHREADED_MESSAGES" {
			message.ThreadID = event.Message.Thread.Name
			message.ThreadTimestamp = stringTimestamp
		}

		// make channel variables available
		message.Vars["_channel.name"] = message.ChannelName // will be empty if it came via DM
		message.Vars["_channel.id"] = message.ChannelID
		message.Vars["_thread.id"] = message.ThreadID

		// make timestamp information available
		message.Vars["_source.timestamp"] = stringTimestamp
	}

	message.Vars["_user.id"] = event.User.Email
	message.Vars["_user.name"] = event.User.Email
	message.Vars["_user.email"] = event.User.Email
	message.Vars["_user.displayname"] = event.User.DisplayName

	return message, nil
}

type ClientOption func(*Client)

func NewClient(bot *models.Bot, opts ...ClientOption) *Client {
	c := &Client{
		Credentials:    bot.GoogleChatCredentials,
		ProjectID:      bot.GoogleChatProjectID,
		SubscriptionID: bot.GoogleChatSubscriptionID,
		DomainAdmin:    bot.GoogleChatDomainAdmin,
		ctx:            context.Background(),
	}

	service, err := chat.NewService(
		c.ctx, option.WithCredentialsFile(c.Credentials),
		option.WithScopes("https://www.googleapis.com/auth/chat.bot"),
	)
	if err != nil {
		log.Fatal().Msgf("google_chat unable to create chat service: %s", err.Error())
	}

	c.msgService = chat.NewSpacesMessagesService(service)

	// apply each option
	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithAdminSDK() ClientOption {
	return func(c *Client) {

		if c.DomainAdmin == "" {
			log.Fatal().Msgf("google_chat unable to create chat service: Please configure google_chat_domain_admin")
			return
		}

		// create the adminService if it doesn't already exist
		if c.adminService == nil {

			// Read credentials from disk
			creds, err := ioutil.ReadFile(c.Credentials)
			if err != nil {
				log.Error().Msgf("Unable to read GoogleChatCredentials file: %v", err)
			}

			// Create client config
			config, err := google.JWTConfigFromJSON(creds,
				admin.AdminDirectoryGroupReadonlyScope,
			)
			if err != nil {
				log.Error().Msgf("Unable to load JWT config from Google credentials: %v", err)
			}
			config.Subject = c.DomainAdmin

			// Create Google Directory client
			c.adminService, err = admin.NewService(
				c.ctx, option.WithTokenSource(config.TokenSource(c.ctx)),
			)
			if err != nil {
				log.Error().Msgf("Unable to retrieve directory Client %v", err)
			}
		}
	}
}
