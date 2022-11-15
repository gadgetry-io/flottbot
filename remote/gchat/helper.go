// Copyright (c) 2022 Target Brands, Inc. All rights reserved.
//
// Use of this source code is governed by the LICENSE file in this repository.

package gchat

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/target/flottbot/models"
)

type DomainEvent struct {
	User struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		AvatarURL   string `json:"avatarUrl"`
		Email       string `json:"email"`
		Type        string `json:"type"`
		DomainID    string `json:"domainId"`
	} `json:"user"`
}

// HandleOutput handles input messages for this remote.
func HandleRemoteInput(inputMsgs chan<- models.Message, rules map[string]models.Rule, bot *models.Bot) {

	c := NewClient(bot)

	// Read messages from Google Chat
	go c.Read(inputMsgs, rules, bot)
}

// HandleRemoteOutput handles output messages for this remote.
func HandleRemoteOutput(message models.Message, bot *models.Bot) {

	c := NewClient(bot)

	// Send messages to Google Chat
	go c.Send(message, bot)
}

func IsMemberOfGroup(currentUserID string, userGroups []string, bot *models.Bot) (bool, error) {

	if bot.GoogleChatDomainAdmin == "" {
		errmsg := "allow_usergroups not enabled. Please configure google_chat_domain_admin"
		log.Warn().Msg(errmsg)
		return false, fmt.Errorf(errmsg)
	}

	c := NewClient(bot, WithAdminSDK())

	// check membership
	return c.isMemberOfGroup(currentUserID, userGroups, bot)
}
