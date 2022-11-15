package gchat

import "time"

// Message is message received from the pub/sub event
type Message struct {
	Type      string    `json:"type"`
	EventTime time.Time `json:"eventTime"`
	Message   struct {
		Name   string `json:"name"`
		Sender struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			AvatarURL   string `json:"avatarUrl"`
			Email       string `json:"email"`
			Type        string `json:"type"`
			DomainID    string `json:"domainId"`
		} `json:"sender"`
		CreateTime time.Time `json:"createTime"`
		Text       string    `json:"text"`
		Thread     struct {
			Name              string `json:"name"`
			RetentionSettings struct {
				State string `json:"state"`
			} `json:"retentionSettings"`
		} `json:"thread"`
		Space struct {
			Name            string `json:"name"`
			SingleUserBotDm bool   `json:"singleUserBotDm"`
			SpaceType       string `json:"spaceType"`
		} `json:"space"`
		ArgumentText      string `json:"argumentText"`
		RetentionSettings struct {
			State string `json:"state"`
		} `json:"retentionSettings"`
		MessageHistoryState string `json:"messageHistoryState"`
	} `json:"message"`
	User struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		AvatarURL   string `json:"avatarUrl"`
		Email       string `json:"email"`
		Type        string `json:"type"`
		DomainID    string `json:"domainId"`
	} `json:"user"`
	Space struct {
		Name                string `json:"name"`
		Type                string `json:"type"`
		SingleUserBotDm     bool   `json:"singleUserBotDm"`
		SpaceThreadingState string `json:"spaceThreadingState"`
		SpaceType           string `json:"spaceType"`
		SpaceHistoryState   string `json:"spaceHistoryState"`
	} `json:"space"`
	ConfigCompleteRedirectURL string `json:"configCompleteRedirectUrl"`
}
