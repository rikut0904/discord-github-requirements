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
	NotificationChannelID        string   // Deprecated: 共通通知チャンネル
	NotificationIssuesChannelID  string   // /issues用通知チャンネル
	NotificationAssignChannelID  string   // /assign用通知チャンネル
	UpdatedAt                    time.Time
}
