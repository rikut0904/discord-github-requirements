package repository

import (
	"context"
	"github-discord-bot/internal/domain/entity"
)

type UserSettingRepository interface {
	Save(ctx context.Context, setting *entity.UserSetting) error
	Find(ctx context.Context, guildID, channelID, userID string) (*entity.UserSetting, error)
	FindByGuildAndUser(ctx context.Context, guildID, userID string) (*entity.UserSetting, error)
	Delete(ctx context.Context, guildID, channelID, userID string) error
	DeleteByGuild(ctx context.Context, guildID string) error
	DeleteByChannel(ctx context.Context, guildID, channelID string) error
}
