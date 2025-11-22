package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github-discord-bot/internal/infrastructure/github"
	"github-discord-bot/internal/usecase"

	"github.com/bwmarrin/discordgo"
)

type DiscordHandler struct {
	settingUsecase *usecase.SettingUsecase
	issuesUsecase  *usecase.IssuesUsecase
}

func NewDiscordHandler(settingUsecase *usecase.SettingUsecase, issuesUsecase *usecase.IssuesUsecase) *DiscordHandler {
	return &DiscordHandler{
		settingUsecase: settingUsecase,
		issuesUsecase:  issuesUsecase,
	}
}

func (h *DiscordHandler) RegisterCommands(s *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "setting",
			Description: "GitHub Personal Access Token を登録・更新します",
		},
		{
			Name:        "assign",
			Description: "自分に割り当てられた Issue を取得します",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "page",
					Description: "ページ番号",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "per",
					Description: "1ページあたりの件数",
					Required:    false,
				},
			},
		},
		{
			Name:        "issues",
			Description: "指定したリポジトリの Issue を取得します",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "repository",
					Description: "owner/repo 形式で指定",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "page",
					Description: "ページ番号",
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "per",
					Description: "1ページあたりの件数",
					Required:    false,
				},
			},
		},
	}

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			return fmt.Errorf("failed to create command %s: %w", cmd.Name, err)
		}
	}

	return nil
}

func (h *DiscordHandler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		h.handleCommand(s, i)
	case discordgo.InteractionModalSubmit:
		h.handleModalSubmit(s, i)
	}
}

func (h *DiscordHandler) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.ApplicationCommandData().Name {
	case "setting":
		h.handleSettingCommand(s, i)
	case "assign":
		h.handleAssignCommand(s, i)
	case "issues":
		h.handleIssuesCommand(s, i)
	}
}

func (h *DiscordHandler) handleSettingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "token_modal",
			Title:    "GitHub Token 設定",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "token_input",
							Label:       "GitHub Personal Access Token",
							Style:       discordgo.TextInputShort,
							Placeholder: "ghp_xxxxxxxxxxxx",
							Required:    true,
							MinLength:   1,
							MaxLength:   255,
						},
					},
				},
			},
		},
	})
	if err != nil {
		fmt.Printf("Error responding with modal: %v\n", err)
	}
}

func (h *DiscordHandler) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ModalSubmitData().CustomID != "token_modal" {
		return
	}

	var token string
	for _, comp := range i.ModalSubmitData().Components {
		if row, ok := comp.(*discordgo.ActionsRow); ok {
			for _, rowComp := range row.Components {
				if input, ok := rowComp.(*discordgo.TextInput); ok && input.CustomID == "token_input" {
					token = input.Value
				}
			}
		}
	}

	ctx := context.Background()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	err := h.settingUsecase.SaveToken(ctx, guildID, channelID, userID, token)
	if err != nil {
		var message string
		if ghErr, ok := err.(*github.GitHubError); ok {
			message = fmt.Sprintf("❌ トークンの検証に失敗しました: %s", ghErr.Message)
		} else {
			message = "❌ トークンの保存に失敗しました"
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✅ GitHub Token を登録しました",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (h *DiscordHandler) handleIssuesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	page := 1
	perPage := 10
	repoInput := ""

	for _, opt := range options {
		switch opt.Name {
		case "repository":
			repoInput = opt.StringValue()
		case "page":
			page = int(opt.IntValue())
		case "per":
			perPage = int(opt.IntValue())
		}
	}

	if repoInput == "" {
		message := "❌ repository は owner/repo 形式で指定してください。"
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	parts := strings.Split(repoInput, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		message := "❌ repository は owner/repo 形式で指定してください。"
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	owner := parts[0]
	repo := parts[1]

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	ctx := context.Background()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	// Defer response for long operations
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	issues, rateLimit, err := h.issuesUsecase.GetRepositoryIssues(ctx, guildID, channelID, userID, owner, repo, page, perPage)
	if err != nil {
		var message string
		if err == usecase.ErrTokenNotFound {
			message = "❌ トークンが登録されていません。`/setting` でトークンを登録してください。"
		} else if ghErr, ok := err.(*github.GitHubError); ok {
			message = fmt.Sprintf("❌ GitHub API エラー: %s", ghErr.Message)
		} else {
			message = "❌ Issue の取得に失敗しました"
		}

		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &message,
		})
		return
	}

	if len(issues) == 0 {
		message := "📭 Issue が見つかりませんでした"
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &message,
		})
		return
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(issues))
	for _, issue := range issues {
		embed := createIssueEmbed(issue)
		embeds = append(embeds, embed)
	}

	// Add rate limit info if low
	var content string
	if rateLimit != nil && rateLimit.Remaining < 10 {
		content = fmt.Sprintf("⚠️ API Rate Limit 残り: %d (リセット: %s)",
			rateLimit.Remaining,
			rateLimit.ResetAt.Format("15:04:05"))
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  &embeds,
	})
}

func (h *DiscordHandler) handleAssignCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	page := 1
	perPage := 10

	for _, opt := range options {
		switch opt.Name {
		case "page":
			page = int(opt.IntValue())
		case "per":
			perPage = int(opt.IntValue())
		}
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	ctx := context.Background()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	issues, rateLimit, err := h.issuesUsecase.GetAssignedIssues(ctx, guildID, channelID, userID, page, perPage)
	if err != nil {
		var message string
		if err == usecase.ErrTokenNotFound {
			message = "❌ トークンが登録されていません。`/setting` でトークンを登録してください。"
		} else if ghErr, ok := err.(*github.GitHubError); ok {
			message = fmt.Sprintf("❌ GitHub API エラー: %s", ghErr.Message)
		} else {
			message = "❌ Issue の取得に失敗しました"
		}

		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &message,
		})
		return
	}

	if len(issues) == 0 {
		message := "📭 割り当てられた Issue は見つかりませんでした"
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &message,
		})
		return
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(issues))
	for _, issue := range issues {
		embed := createIssueEmbed(issue)
		embeds = append(embeds, embed)
	}

	var content string
	if rateLimit != nil && rateLimit.Remaining < 10 {
		content = fmt.Sprintf("⚠️ API Rate Limit 残り: %d (リセット: %s)",
			rateLimit.Remaining,
			rateLimit.ResetAt.Format("15:04:05"))
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  &embeds,
	})
}

func createIssueEmbed(issue github.Issue) *discordgo.MessageEmbed {
	var labels []string
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}

	var assignees []string
	for _, assignee := range issue.Assignees {
		assignees = append(assignees, assignee.Login)
	}

	repoName := ""
	if issue.Repository != nil {
		repoName = issue.Repository.FullName
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Repository",
			Value:  repoName,
			Inline: true,
		},
		{
			Name:   "State",
			Value:  issue.State,
			Inline: true,
		},
	}

	if len(labels) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Labels",
			Value:  strings.Join(labels, ", "),
			Inline: true,
		})
	}

	if len(assignees) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Assignees",
			Value:  strings.Join(assignees, ", "),
			Inline: true,
		})
	}

	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Updated",
		Value:  issue.UpdatedAt.Format(time.RFC3339),
		Inline: true,
	})

	return &discordgo.MessageEmbed{
		Title:  fmt.Sprintf("#%d %s", issue.Number, issue.Title),
		URL:    issue.HTMLURL,
		Color:  0x238636,
		Fields: fields,
	}
}
