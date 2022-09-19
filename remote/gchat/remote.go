// Copyright (c) 2022 Target Brands, Inc. All rights reserved.
//
// Use of this source code is governed by the LICENSE file in this repository.

package gchat

import (
	"context"
	"fmt"
	"io/ioutil"

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
}

// validate that Client adheres to remote interface.
var _ remote.Remote = (*Client)(nil)

// Name returns the name of the remote.
func (c *Client) Name() string {
	return "google_chat"
}

// Read messages from Google Chat.
func (c *Client) Read(inputMsgs chan<- models.Message, rules map[string]models.Rule, bot *models.Bot) {
	ctx := context.Background()

	// init client
	client, err := pubsub.NewClient(ctx, c.ProjectID, option.WithCredentialsFile(c.Credentials))
	if err != nil {
		log.Error().Msgf("google_chat unable to authenticate: %s", err.Error())
	}

	sub := client.Subscription(c.SubscriptionID)

	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		defer m.Ack()

		// Convert Google Chat Message to Flottbot Message
		message, err := toMessage(m)
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
	ctx := context.Background()

	service, err := chat.NewService(
		ctx, option.WithCredentialsFile(c.Credentials),
		option.WithScopes("https://www.googleapis.com/auth/chat.bot"),
	)
	if err != nil {
		log.Fatal().Msgf("google_chat unable to create chat service: %s", err.Error())
	}

	msgService := chat.NewSpacesMessagesService(service)

	// Best effort. If the instance goes away, so be it.
	msg := &chat.Message{
		Text: message.Output,
	}

	if message.ThreadID != "" {
		msg.Thread = &chat.Thread{
			Name: message.ThreadID,
		}
	}

	_, err = msgService.Create(message.ChannelID, msg).Do()
	if err != nil {
		log.Error().Msgf("google_chat failed to create message: %s", err.Error())
	}
}

// Reaction implementation to satisfy remote interface.
func (c *Client) Reaction(message models.Message, rule models.Rule, bot *models.Bot) {
	// Not implemented for Google Chat
}

func (c *Client) isMemberOfGroup(currentUserID string, userGroups []string, bot *models.Bot) (bool, error) {
	ctx := context.Background()

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
	service, err := admin.NewService(
		ctx, option.WithTokenSource(config.TokenSource(ctx)),
	)
	if err != nil {
		log.Error().Msgf("Unable to retrieve directory Client %v", err)
	}

	// Get groups for user
	listGroupsResponse, err := service.Groups.List().Customer("my_customer").Query(fmt.Sprintf("memberKey=%s", currentUserID)).MaxResults(200).Do()
	if err != nil {
		if e, ok := err.(*googleapi.Error); ok {
			switch e.Code {
			case 403:
				log.Fatal().Msgf("User %s is not a domain administrator with directory listing permissions.", config.Subject)
			}
		}
		log.Fatal().Msgf("Could not list Google groups for user %s using admin email %s: %v", currentUserID, config.Subject, err)
	}

	// Check user is in onen of the rule's groups
	for _, group := range listGroupsResponse.Groups {
		for _, ruleGroup := range userGroups {
			if ruleGroup == group.Email {
				return true, nil
			}
		}
	}

	// Users is not authorized
	return false, nil
}
