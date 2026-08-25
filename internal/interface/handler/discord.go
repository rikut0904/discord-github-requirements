package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github-discord-bot/internal/domain/entity"
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
	// Guild IDを指定した場合はローカル開発向けに即時反映されるサーバーコマンドとして登録する。
	// 未指定時は従来どおりグローバルコマンドとして登録する。
	guildID := os.Getenv("DISCORD_GUILD_ID")
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "setting",
			Description: "GitHub Personal Access Token を登録・更新します",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "設定の種類",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "トークン設定", Value: "token"},
						{Name: "通知チャンネル設定", Value: "notification_channel"},
						{Name: "/issues用 除外リポジトリ設定", Value: "exclude_issues"},
						{Name: "/assign用 除外リポジトリ設定", Value: "exclude_assign"},
						{Name: "/dependabot用 除外リポジトリ設定", Value: "exclude_dependabot"},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "notification_scope",
					Description: "通知チャンネル設定の対象/操作 (all・issues・assign・confirm・clear)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "all (共通)", Value: "all"},
						{Name: "issues のみ", Value: "issues"},
						{Name: "assign のみ", Value: "assign"},
						{Name: "dependabot のみ", Value: "dependabot"},
						{Name: "確認", Value: "confirm"},
						{Name: "解除", Value: "clear"},
					},
				},
			},
		},
		{
			Name:        "assign",
			Description: "自分に割り当てられた Issue を取得します",
		},
		{
			Name:        "dependabot",
			Description: "Dependabot が作成したオープン PR を取得します",
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
			},
		},
	}

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
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
	case "dependabot":
		h.handleDependabotCommand(s, i)
	case "issues":
		h.handleIssuesCommand(s, i)
	}
}

func (h *DiscordHandler) handleSettingCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	action := "token"
	notificationScope := "all"

	for _, opt := range options {
		switch opt.Name {
		case "action":
			action = opt.StringValue()
		case "notification_scope":
			notificationScope = opt.StringValue()
		}
	}

	switch action {
	case "token":
		h.showTokenModal(s, i)
	case "notification_channel":
		switch notificationScope {
		case "confirm":
			h.handleNotificationChannelConfirm(s, i)
			return
		case "clear":
			h.handleNotificationChannelClear(s, i)
			return
		case CommandTypeIssues:
			h.handleNotificationChannelSetting(s, i, CommandTypeIssues)
		case CommandTypeAssign:
			h.handleNotificationChannelSetting(s, i, CommandTypeAssign)
		case CommandTypeDependabot:
			h.handleNotificationChannelSetting(s, i, CommandTypeDependabot)
		default:
			h.handleNotificationChannelSetting(s, i, "all")
		}
	case "exclude_issues":
		h.showExcludeModal(s, i, CommandTypeIssues)
	case "exclude_assign":
		h.showExcludeModal(s, i, CommandTypeAssign)
	case "exclude_dependabot":
		h.showExcludeModal(s, i, CommandTypeDependabot)
	default:
		h.respondWithError(s, i, "❌ 未対応のアクションです。")
	}
}

func (h *DiscordHandler) handleNotificationChannelSetting(s *discordgo.Session, i *discordgo.InteractionCreate, commandType string) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()

	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	// 現在のチャンネルを通知チャンネルとして保存
	err := h.settingUsecase.SaveNotificationChannel(ctx, guildID, channelID, userID, commandType, channelID)
	if err != nil {
		h.respondWithError(s, i, "❌ 通知チャンネルの設定に失敗しました")
		return
	}

	var message string
	switch commandType {
	case CommandTypeIssues:
		message = fmt.Sprintf("✅ このチャンネル (<#%s>) を /issues 用通知チャンネルに設定しました。", channelID)
	case CommandTypeAssign:
		message = fmt.Sprintf("✅ このチャンネル (<#%s>) を /assign 用通知チャンネルに設定しました。", channelID)
	case CommandTypeDependabot:
		message = fmt.Sprintf("✅ このチャンネル (<#%s>) を /dependabot 用通知チャンネルに設定しました。", channelID)
	default:
		message = fmt.Sprintf("✅ このチャンネル (<#%s>) を通知チャンネルとして設定しました（/issues・/assign・/dependabot共通）。", channelID)
	}

	h.respondWithSuccess(s, i, message)
}

func (h *DiscordHandler) handleNotificationChannelConfirm(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()

	guildID := i.GuildID
	userID := i.Member.User.ID

	setting, err := h.settingUsecase.GetUserSetting(ctx, guildID, userID)
	if err != nil {
		h.respondWithError(s, i, "❌ 通知チャンネル情報の取得に失敗しました")
		return
	}

	var (
		commonChannel     string
		issuesChannel     string
		assignChannel     string
		dependabotChannel string
	)

	if setting != nil {
		commonChannel = setting.NotificationChannelID
		issuesChannel = setting.NotificationChannelForIssues()
		assignChannel = setting.NotificationChannelForAssign()
		dependabotChannel = setting.NotificationChannelForDependabot()
	}

	message := fmt.Sprintf("📋 通知チャンネル設定状況:\n- /issues: %s\n- /assign: %s\n- /dependabot: %s\n- 共通(旧設定): %s",
		formatChannelMention(issuesChannel),
		formatChannelMention(assignChannel),
		formatChannelMention(dependabotChannel),
		formatChannelMention(commonChannel),
	)

	h.respondWithSuccess(s, i, message)
}

func (h *DiscordHandler) handleNotificationChannelClear(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()

	guildID := i.GuildID
	userID := i.Member.User.ID

	if err := h.settingUsecase.ClearNotificationChannels(ctx, guildID, userID); err != nil {
		h.respondWithError(s, i, "❌ 通知チャンネル設定の解除に失敗しました")
		return
	}

	h.respondWithSuccess(s, i, "🧹 通知チャンネル設定をすべて解除しました。")
}

func (h *DiscordHandler) showTokenModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: ModalIDToken,
			Title:    "GitHub Token 設定",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    InputIDToken,
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

func (h *DiscordHandler) showExcludeModal(s *discordgo.Session, i *discordgo.InteractionCreate, commandType string) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	guildID := i.GuildID
	userID := i.Member.User.ID

	currentExcludes, err := h.settingUsecase.GetExcludedRepositories(ctx, guildID, userID, commandType)
	if err != nil {
		fmt.Printf("Error getting excluded repositories: %v\n", err)
		currentExcludes = []string{}
	}

	excludeText := strings.Join(currentExcludes, "\n")

	var title, customID string
	if commandType == CommandTypeIssues {
		title = "/issues用 除外リポジトリ設定"
		customID = ModalIDExcludeIssues
	} else if commandType == CommandTypeDependabot {
		title = "/dependabot用 除外リポジトリ設定"
		customID = ModalIDExcludeDependabot
	} else {
		title = "/assign用 除外リポジトリ設定"
		customID = ModalIDExcludeAssign
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    InputIDExclude,
							Label:       "除外パターン (1行に1つ)",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "owner/repo (特定リポジトリ)\nowner/* (organization全体)\nowner (owner/*と同じ)",
							Required:    false,
							Value:       excludeText,
							MaxLength:   4000,
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
	switch i.ModalSubmitData().CustomID {
	case ModalIDToken:
		h.handleTokenModalSubmit(s, i)
	case ModalIDExcludeIssues:
		h.handleExcludeModalSubmit(s, i, CommandTypeIssues)
	case ModalIDExcludeAssign:
		h.handleExcludeModalSubmit(s, i, CommandTypeAssign)
	case ModalIDExcludeDependabot:
		h.handleExcludeModalSubmit(s, i, CommandTypeDependabot)
	}
}

func (h *DiscordHandler) handleTokenModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// GitHub API の検証と DB 保存には 3 秒以上かかることがあるため、
	// 先に Discord の応答を確定させてから処理を実行する。
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	}); err != nil {
		log.Printf("token modal: failed to defer interaction response: %v", err)
		return
	}

	token := strings.TrimSpace(h.getModalInputValue(i, InputIDToken))

	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	err := h.settingUsecase.SaveToken(ctx, guildID, channelID, userID, token)
	if err != nil {
		log.Printf("token modal: failed to save token for guild=%s user=%s: %v", guildID, userID, err)
		var message string
		if ghErr, ok := err.(*github.GitHubError); ok {
			message = fmt.Sprintf(MsgTokenValidationFailed, ghErr.Message)
		} else {
			message = MsgTokenSaveFailed
		}

		if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &message}); err != nil {
			log.Printf("token modal: failed to edit error response: %v", err)
		}
		return
	}

	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: stringPtr(MsgTokenSaved)}); err != nil {
		log.Printf("token modal: failed to edit success response: %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func (h *DiscordHandler) handleExcludeModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, commandType string) {
	excludeText := h.getModalInputValue(i, InputIDExclude)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	var repositories []string
	if excludeText != "" {
		lines := strings.Split(excludeText, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				if !isValidExcludePattern(line) {
					message := fmt.Sprintf(MsgInvalidExcludePattern, line)
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: message,
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}
				repositories = append(repositories, line)
			}
		}
	}

	err := h.settingUsecase.SaveExcludedRepositories(ctx, guildID, channelID, userID, repositories, commandType)
	if err != nil {
		message := MsgExcludeSaveFailed
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: message,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var message string
	if len(repositories) == 0 {
		message = fmt.Sprintf(MsgExcludeCleared, commandType)
	} else {
		message = fmt.Sprintf(MsgExcludeSaved, commandType, len(repositories))
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// formatIssuesFetchError は Issue 取得時のエラーを適切なメッセージに変換します
func (h *DiscordHandler) formatIssuesFetchError(err error) string {
	if err == usecase.ErrTokenNotFound {
		return MsgTokenNotFound
	}
	if ghErr, ok := err.(*github.GitHubError); ok {
		return fmt.Sprintf(MsgGitHubAPIError, ghErr.Message)
	}
	return MsgIssueFetchFailed
}

// getModalInputValue はモーダルから指定されたCustomIDの値を取得します
func (h *DiscordHandler) getModalInputValue(i *discordgo.InteractionCreate, customID string) string {
	for _, comp := range i.ModalSubmitData().Components {
		if row, ok := comp.(*discordgo.ActionsRow); ok {
			for _, rowComp := range row.Components {
				if input, ok := rowComp.(*discordgo.TextInput); ok && input.CustomID == customID {
					return input.Value
				}
			}
		}
	}
	return ""
}

// respondWithError は通常のエラーレスポンスを送信します
func (h *DiscordHandler) respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// respondEditWithError はDeferred Responseのエラーメッセージを編集します
func (h *DiscordHandler) respondEditWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &message,
	})
}

// respondWithSuccess は成功メッセージを送信します
func (h *DiscordHandler) respondWithSuccess(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// respondDeferred はDeferred Responseを送信します（長時間かかる処理の前に呼ぶ）
func (h *DiscordHandler) respondDeferred(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
}

// sendEmbedsToChannel は指定されたチャンネルにembedsを送信します
func (h *DiscordHandler) sendEmbedsToChannel(s *discordgo.Session, channelID string, content string, embeds []*discordgo.MessageEmbed) {
	// Discordの制限: 1メッセージあたり最大10 embeds
	for i := 0; i < len(embeds); i += MaxEmbedsPerMessage {
		end := i + MaxEmbedsPerMessage
		if end > len(embeds) {
			end = len(embeds)
		}

		messageContent := ""
		if i == 0 && content != "" {
			messageContent = content
		}

		s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content: messageContent,
			Embeds:  embeds[i:end],
		})
	}
}

// repositoryInputType はリポジトリ入力の種類を表します
type repositoryInputType int

const (
	repoInputTypeAll repositoryInputType = iota
	repoInputTypeUser
	repoInputTypeSpecific
	repoInputTypeInvalid
)

// repositoryInput はパースされたリポジトリ入力を表します
type repositoryInput struct {
	inputType repositoryInputType
	owner     string
	repo      string
	username  string
}

// parseRepositoryInput はリポジトリ入力をパースして検証します
func parseRepositoryInput(repoInput string) repositoryInput {
	if strings.ToLower(repoInput) == "all" {
		return repositoryInput{inputType: repoInputTypeAll}
	}

	parts := strings.Split(repoInput, "/")
	if len(parts) == 1 {
		username := strings.TrimSpace(parts[0])
		if username == "" {
			return repositoryInput{inputType: repoInputTypeInvalid}
		}
		return repositoryInput{
			inputType: repoInputTypeUser,
			username:  username,
		}
	}

	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return repositoryInput{
			inputType: repoInputTypeSpecific,
			owner:     parts[0],
			repo:      parts[1],
		}
	}

	return repositoryInput{inputType: repoInputTypeInvalid}
}

// fetchIssuesByRepository はリポジトリ入力に基づいてissuesを取得します
func (h *DiscordHandler) fetchIssuesByRepository(ctx context.Context, guildID, userID string, input repositoryInput) ([]github.Issue, *github.RateLimitInfo, []usecase.RepositoryError, error) {
	switch input.inputType {
	case repoInputTypeAll:
		result, err := h.issuesUsecase.GetAllRepositoriesIssues(ctx, guildID, userID)
		if err != nil {
			if result != nil {
				return nil, result.RateLimit, nil, err
			}
			return nil, nil, nil, err
		}
		return result.Issues, result.RateLimit, result.FailedRepos, nil
	case repoInputTypeUser:
		result, err := h.issuesUsecase.GetUserIssues(ctx, guildID, userID, input.username)
		if err != nil {
			if result != nil {
				return nil, result.RateLimit, nil, err
			}
			return nil, nil, nil, err
		}
		return result.Issues, result.RateLimit, result.FailedRepos, nil
	case repoInputTypeSpecific:
		issues, rateLimit, err := h.issuesUsecase.GetRepositoryIssues(ctx, guildID, userID, input.owner, input.repo)
		return issues, rateLimit, nil, err
	default:
		return nil, nil, nil, fmt.Errorf("unexpected repository input type: %d", input.inputType)
	}
}

func (h *DiscordHandler) handleIssuesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	repoInput := ""

	for _, opt := range options {
		if opt.Name == "repository" {
			repoInput = opt.StringValue()
		}
	}

	if repoInput == "" {
		h.respondWithError(s, i, MsgInvalidRepoFormat)
		return
	}

	// Parse and validate repository input
	input := parseRepositoryInput(repoInput)
	if input.inputType == repoInputTypeInvalid {
		h.respondWithError(s, i, MsgInvalidRepoFormat)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	currentChannelID := i.ChannelID

	// Defer response for long operations
	h.respondDeferred(s, i)

	// Get notification channel for issues command
	notificationChannelID, _, err := h.getNotificationChannelForCommand(ctx, s, i, "issues")
	if err != nil {
		return
	}

	// Fetch issues based on repository input
	issues, rateLimit, failedRepos, err := h.fetchIssuesByRepository(ctx, i.GuildID, i.Member.User.ID, input)

	if err != nil {
		h.respondEditWithError(s, i, h.formatIssuesFetchError(err))
		return
	}

	if len(issues) == 0 {
		h.respondEditWithError(s, i, MsgNoIssuesFound)
		return
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(issues))
	for _, issue := range issues {
		embed := createIssueEmbed(issue)
		embeds = append(embeds, embed)
	}

	var content string
	if rateLimit != nil && rateLimit.Remaining < RateLimitWarningThreshold {
		content = fmt.Sprintf(MsgRateLimitWarning,
			rateLimit.Remaining,
			rateLimit.ResetAt.Format("15:04:05"))
	}

	// Add failed repositories warning if any
	if len(failedRepos) > 0 {
		failedRepoNames := make([]string, 0, len(failedRepos))
		for _, failedRepo := range failedRepos {
			failedRepoNames = append(failedRepoNames, failedRepo.RepositoryName)
		}
		failedMsg := fmt.Sprintf("⚠️ 以下のリポジトリでエラーが発生しました (%d件):\n- %s",
			len(failedRepos),
			strings.Join(failedRepoNames, "\n- "))

		if len(content) > 0 {
			content += "\n\n" + failedMsg
		} else {
			content = failedMsg
		}
	}

	// Send completion message to the channel where command was executed
	completionMsg := "✅ Issue一覧を取得しました。"
	if currentChannelID != notificationChannelID {
		completionMsg = fmt.Sprintf("✅ Issue一覧を取得しました。結果は <#%s> に送信されました。", notificationChannelID)
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &completionMsg,
	})

	// Send issues to notification channel
	h.sendEmbedsToChannel(s, notificationChannelID, content, embeds)
}

func (h *DiscordHandler) handleAssignCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	currentChannelID := i.ChannelID

	h.respondDeferred(s, i)

	// Get notification channel for assign command
	notificationChannelID, _, err := h.getNotificationChannelForCommand(ctx, s, i, "assign")
	if err != nil {
		return
	}

	issues, rateLimit, err := h.issuesUsecase.GetAssignedIssues(ctx, i.GuildID, i.Member.User.ID)
	if err != nil {
		h.respondEditWithError(s, i, h.formatIssuesFetchError(err))
		return
	}

	if len(issues) == 0 {
		h.respondEditWithError(s, i, MsgNoAssignedIssuesFound)
		return
	}

	embeds := make([]*discordgo.MessageEmbed, 0, len(issues))
	for _, issue := range issues {
		embed := createIssueEmbed(issue)
		embeds = append(embeds, embed)
	}

	var content string
	if rateLimit != nil && rateLimit.Remaining < RateLimitWarningThreshold {
		content = fmt.Sprintf(MsgRateLimitWarning,
			rateLimit.Remaining,
			rateLimit.ResetAt.Format("15:04:05"))
	}

	// Send completion message to the channel where command was executed
	completionMsg := "✅ 割り当てられたIssue一覧を取得しました。"
	if currentChannelID != notificationChannelID {
		completionMsg = fmt.Sprintf("✅ 割り当てられたIssue一覧を取得しました。結果は <#%s> に送信されました。", notificationChannelID)
	}
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &completionMsg,
	})

	// Send issues to notification channel
	h.sendEmbedsToChannel(s, notificationChannelID, content, embeds)
}

func (h *DiscordHandler) handleDependabotCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()

	h.respondDeferred(s, i)
	notificationChannelID, _, err := h.getNotificationChannelForCommand(ctx, s, i, CommandTypeDependabot)
	if err != nil {
		return
	}
	pullRequests, rateLimit, err := h.issuesUsecase.GetDependabotPullRequests(ctx, i.GuildID, i.Member.User.ID)
	if err != nil {
		h.respondEditWithError(s, i, h.formatIssuesFetchError(err))
		return
	}
	if len(pullRequests) == 0 {
		h.respondEditWithError(s, i, "📭 Dependabot のオープン PR は見つかりませんでした")
		return
	}

	content := "✅ Dependabot のオープン PR を取得しました。"
	if rateLimit != nil && rateLimit.Remaining < RateLimitWarningThreshold {
		content += fmt.Sprintf("\n"+MsgRateLimitWarning, rateLimit.Remaining, rateLimit.ResetAt.Format("15:04:05"))
	}
	if i.ChannelID != notificationChannelID {
		content += fmt.Sprintf(" 結果は <#%s> に送信されました。", notificationChannelID)
	}

	// 初回の応答を完了メッセージに置き換え、Embed本体は通常メッセージとして送る。
	h.respondEditWithError(s, i, content)
	embeds := make([]*discordgo.MessageEmbed, 0, len(pullRequests))
	for _, pullRequest := range pullRequests {
		repositoryName := "不明"
		if pullRequest.Repository != nil {
			repositoryName = pullRequest.Repository.FullName
		}
		embeds = append(embeds, &discordgo.MessageEmbed{
			Title: fmt.Sprintf("#%d %s", pullRequest.Number, pullRequest.Title),
			URL:   pullRequest.HTMLURL,
			Color: ColorGitHubSuccess,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Repository", Value: repositoryName, Inline: true},
				{Name: "Author", Value: pullRequest.Author.Login, Inline: true},
				{Name: "Updated", Value: pullRequest.UpdatedAt.Format(time.RFC3339), Inline: true},
			},
		})
	}
	h.sendEmbedsToChannel(s, notificationChannelID, "", embeds)
}

// getNotificationChannelForCommand はユーザー設定を取得し、指定されたコマンドタイプの通知チャンネルIDを返します。
// 設定が存在しない、または通知チャンネルが設定されていない場合はエラーを返します。
func (h *DiscordHandler) getNotificationChannelForCommand(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, commandType string) (string, *entity.UserSetting, error) {
	guildID := i.GuildID
	userID := i.Member.User.ID

	setting, err := h.settingUsecase.GetUserSetting(ctx, guildID, userID)
	if err != nil || setting == nil {
		h.respondEditWithError(s, i, MsgTokenNotFound)
		return "", nil, fmt.Errorf("user setting not found")
	}

	var notificationChannelID string
	var errorMsg string

	switch commandType {
	case "issues":
		notificationChannelID = setting.NotificationChannelForIssues()
		errorMsg = "❌ /issues用通知チャンネルが設定されていません。`/setting action:notification_channel notification_scope:issues` で設定してください。"
	case "assign":
		notificationChannelID = setting.NotificationChannelForAssign()
		errorMsg = "❌ /assign用通知チャンネルが設定されていません。`/setting action:notification_channel notification_scope:assign` で設定してください。"
	case "dependabot":
		notificationChannelID = setting.NotificationChannelForDependabot()
		errorMsg = "❌ /dependabot用通知チャンネルが設定されていません。`/setting action:notification_channel notification_scope:dependabot` で設定してください。"
	default:
		h.respondEditWithError(s, i, "❌ 不明なコマンドタイプです")
		return "", nil, fmt.Errorf("unknown command type: %s", commandType)
	}

	if notificationChannelID == "" {
		h.respondEditWithError(s, i, errorMsg)
		return "", nil, fmt.Errorf("notification channel not configured for %s", commandType)
	}

	return notificationChannelID, setting, nil
}

func isValidExcludePattern(pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}

	// Check for invalid characters
	if strings.ContainsAny(pattern, " \t\r\n") {
		return false
	}

	parts := strings.Split(pattern, "/")

	// Pattern: "owner"
	if len(parts) == 1 {
		// Just owner name (already validated as non-empty)
		return true
	}

	// Pattern: "owner/*" or "owner/repo"
	if len(parts) == 2 {
		if parts[0] == "" {
			return false
		}
		// Allow "owner/*" or "owner/repo"
		return parts[1] == "*" || parts[1] != ""
	}

	return false
}

func formatChannelMention(channelID string) string {
	if channelID == "" {
		return "未設定"
	}
	return fmt.Sprintf("<#%s>", channelID)
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
		Color:  ColorGitHubSuccess,
		Fields: fields,
	}
}
