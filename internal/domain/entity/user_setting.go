package entity

import "time"

type UserSetting struct {
	GuildID                      string
	ChannelID                    string // Setting保存時のチャンネル（主キー用）
	UserID                       string
	EncryptedToken               string
	ExcludedRepositories         []string // Deprecated: use ExcludedIssuesRepos and ExcludedAssignRepos
	ExcludedIssuesRepositories   []string
	ExcludedAssignRepositories   []string
	NotificationChannelID        string   // Issue通知を送信するチャンネル
	UpdatedAt                    time.Time
}
