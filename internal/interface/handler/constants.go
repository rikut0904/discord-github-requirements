package handler

import "time"

// Discord Modal IDs
const (
	ModalIDToken             = "token_modal"
	ModalIDExcludeIssues     = "exclude_issues_modal"
	ModalIDExcludeAssign     = "exclude_assign_modal"
	ModalIDExcludeDependabot = "exclude_dependabot_modal"
)

// Discord Input IDs
const (
	InputIDToken   = "token_input"
	InputIDExclude = "exclude_input"
)

// Command Types
const (
	CommandTypeIssues     = "issues"
	CommandTypeAssign     = "assign"
	CommandTypeDependabot = "dependabot"
)

// User Messages - Success
const (
	MsgTokenSaved            = "✅ GitHub Token を登録しました"
	MsgExcludeCleared        = "✅ %s用の除外リポジトリをクリアしました"
	MsgExcludeSaved          = "✅ %s用に%d件のリポジトリを除外リストに設定しました"
	MsgNoIssuesFound         = "📭 Issue が見つかりませんでした"
	MsgNoAssignedIssuesFound = "📭 割り当てられた Issue は見つかりませんでした"
)

// User Messages - Errors
const (
	MsgTokenNotFound         = "❌ トークンが登録されていません。`/setting` でトークンを登録してください。"
	MsgTokenValidationFailed = "❌ トークンの検証に失敗しました: %s"
	MsgTokenSaveFailed       = "❌ トークンの保存に失敗しました"
	MsgInvalidRepoFormat     = "❌ repository は owner/repo 形式、username 形式、または all を指定してください。"
	MsgInvalidExcludePattern = "❌ 不正な形式があります: %s\n正しい形式:\n- owner/repo (特定リポジトリ)\n- owner/* (organization全体)\n- owner (owner/*と同じ)"
	MsgExcludeSaveFailed     = "❌ 除外リポジトリの保存に失敗しました"
	MsgGitHubAPIError        = "❌ GitHub API エラー: %s"
	MsgIssueFetchFailed      = "❌ Issue の取得に失敗しました"
)

// User Messages - Warnings
const (
	MsgRateLimitWarning = "⚠️ API Rate Limit 残り: %d (リセット: %s)"
)

// Discord Limits
const (
	MaxEmbedsPerMessage       = 10
	RateLimitWarningThreshold = 10
)

// Timeouts
const (
	DefaultContextTimeout = 30 * time.Second // GitHub API calls timeout
)

// Discord Embed Colors
const (
	ColorGitHubSuccess = 0x238636 // GitHub's green color for success/open issues
)
