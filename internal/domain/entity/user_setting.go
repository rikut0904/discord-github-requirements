package entity

import "time"

type UserSetting struct {
	GuildID                     string
	ChannelID                   string // Setting保存時のチャンネル（主キー用）
	UserID                      string
	EncryptedToken              string
	ExcludedRepositories        []string // Deprecated: use ExcludedIssuesRepos and ExcludedAssignRepos
	ExcludedIssuesRepositories  []string
	ExcludedAssignRepositories  []string
	NotificationChannelID       string // Deprecated: 共通通知チャンネル
	NotificationIssuesChannelID string // /issues用通知チャンネル
	NotificationAssignChannelID string // /assign用通知チャンネル
	UpdatedAt                   time.Time
}

// NotificationChannelForIssues returns the effective channel ID for /issues notifications.
func (u *UserSetting) NotificationChannelForIssues() string {
	if u == nil {
		return ""
	}
	if u.NotificationIssuesChannelID != "" {
		return u.NotificationIssuesChannelID
	}
	return u.NotificationChannelID
}

// NotificationChannelForAssign returns the effective channel ID for /assign notifications.
func (u *UserSetting) NotificationChannelForAssign() string {
	if u == nil {
		return ""
	}
	if u.NotificationAssignChannelID != "" {
		return u.NotificationAssignChannelID
	}
	return u.NotificationChannelID
}
