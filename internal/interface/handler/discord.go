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
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "action",
					Description: "設定の種類",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "トークン設定",
							Value: "token",
						},
						{
							Name:  "/issues用 除外リポジトリ設定",
							Value: "exclude_issues",
						},
						{
							Name:  "/assign用 除外リポジトリ設定",
							Value: "exclude_assign",
						},
					},
				},
			},
		},
		{
			Name:        "assign",
			Description: "自分に割り当てられた Issue を取得します",
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
	options := i.ApplicationCommandData().Options
	action := "token"

	for _, opt := range options {
		if opt.Name == "action" {
			action = opt.StringValue()
		}
	}

	switch action {
	case "token":
		h.showTokenModal(s, i)
	case "exclude_issues":
		h.showExcludeModal(s, i, CommandTypeIssues)
	case "exclude_assign":
		h.showExcludeModal(s, i, CommandTypeAssign)
	}
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
	channelID := i.ChannelID
	userID := i.Member.User.ID

	currentExcludes, err := h.settingUsecase.GetExcludedRepositories(ctx, guildID, channelID, userID, commandType)
	if err != nil {
		fmt.Printf("Error getting excluded repositories: %v\n", err)
		currentExcludes = []string{}
	}

	excludeText := strings.Join(currentExcludes, "\n")

	var title, customID string
	if commandType == CommandTypeIssues {
		title = "/issues用 除外リポジトリ設定"
		customID = ModalIDExcludeIssues
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
	}
}

func (h *DiscordHandler) handleTokenModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	token := h.getModalInputValue(i, InputIDToken)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	err := h.settingUsecase.SaveToken(ctx, guildID, channelID, userID, token)
	if err != nil {
		var message string
		if ghErr, ok := err.(*github.GitHubError); ok {
			message = fmt.Sprintf(MsgTokenValidationFailed, ghErr.Message)
		} else {
			message = MsgTokenSaveFailed
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
			Content: MsgTokenSaved,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
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
func (h *DiscordHandler) fetchIssuesByRepository(ctx context.Context, guildID, channelID, userID string, input repositoryInput) ([]github.Issue, *github.RateLimitInfo, []usecase.RepositoryError, error) {
	switch input.inputType {
	case repoInputTypeAll:
		result, err := h.issuesUsecase.GetAllRepositoriesIssues(ctx, guildID, channelID, userID)
		if err != nil {
			if result != nil {
				return nil, result.RateLimit, nil, err
			}
			return nil, nil, nil, err
		}
		return result.Issues, result.RateLimit, result.FailedRepos, nil
	case repoInputTypeUser:
		result, err := h.issuesUsecase.GetUserIssues(ctx, guildID, channelID, userID, input.username)
		if err != nil {
			if result != nil {
				return nil, result.RateLimit, nil, err
			}
			return nil, nil, nil, err
		}
		return result.Issues, result.RateLimit, result.FailedRepos, nil
	case repoInputTypeSpecific:
		issues, rateLimit, err := h.issuesUsecase.GetRepositoryIssues(ctx, guildID, channelID, userID, input.owner, input.repo)
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
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	// Defer response for long operations
	h.respondDeferred(s, i)

	// Fetch issues based on repository input
	issues, rateLimit, failedRepos, err := h.fetchIssuesByRepository(ctx, guildID, channelID, userID, input)

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

	h.respondWithEmbeds(s, i, content, embeds)
}

func (h *DiscordHandler) handleAssignCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultContextTimeout)
	defer cancel()
	guildID := i.GuildID
	channelID := i.ChannelID
	userID := i.Member.User.ID

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	issues, rateLimit, err := h.issuesUsecase.GetAssignedIssues(ctx, guildID, channelID, userID)
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

	h.respondWithEmbeds(s, i, content, embeds)
}

func (h *DiscordHandler) respondWithEmbeds(s *discordgo.Session, i *discordgo.InteractionCreate, content string, embeds []*discordgo.MessageEmbed) {

	if len(embeds) == 0 {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	firstEmbeds := embeds
	if len(firstEmbeds) > MaxEmbedsPerMessage {
		firstEmbeds = embeds[:MaxEmbedsPerMessage]
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  &firstEmbeds,
	})

	for offset := MaxEmbedsPerMessage; offset < len(embeds); offset += MaxEmbedsPerMessage {
		end := offset + MaxEmbedsPerMessage
		if end > len(embeds) {
			end = len(embeds)
		}
		chunk := embeds[offset:end]
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: chunk,
		}); err != nil {
			fmt.Printf("Failed to send followup message: %v\n", err)
			break
		}
	}
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
