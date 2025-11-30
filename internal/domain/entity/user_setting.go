package entity

import "time"

type UserSetting struct {
	GuildID                      string
	ChannelID                    string
	UserID                       string
	EncryptedToken               string
	ExcludedRepositories         []string // Deprecated: use ExcludedIssuesRepos and ExcludedAssignRepos
	ExcludedIssuesRepositories   []string
	ExcludedAssignRepositories   []string
	UpdatedAt                    time.Time
}
