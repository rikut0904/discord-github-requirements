package entity

import "time"

type UserSetting struct {
	GuildID        string
	ChannelID      string
	UserID         string
	EncryptedToken string
	UpdatedAt      time.Time
}
