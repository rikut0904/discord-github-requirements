package entity

import "time"

type UserSetting struct {
	GuildID                     string
	ChannelID                   string   // 最後に設定コマンドが実行されたチャンネル
	UserID                      string
	EncryptedToken              string
	ExcludedRepositories        []string // Deprecated: use ExcludedIssuesRepositories and ExcludedAssignRepositories
	ExcludedIssuesRepositories  []string
	ExcludedAssignRepositories  []string
	NotificationChannelID       string // Deprecated: 共通通知チャンネル（all スコープ用）
	NotificationIssuesChannelID string // /issues コマンド用通知チャンネル
	NotificationAssignChannelID string // /assign コマンド用通知チャンネル
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
