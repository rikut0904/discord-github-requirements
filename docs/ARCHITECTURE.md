# アーキテクチャ設計ドキュメント

Discord GitHub 通知 Bot はクリーンアーキテクチャを採用し、Discord ハンドラ・ユースケース・インフラを明確に分離しています。本書では各レイヤーの責務と代表的なデータフローを説明します。

## 目次
- [全体像](#全体像)
- [ディレクトリ構成](#ディレクトリ構成)
- [レイヤー別の責務](#レイヤー別の責務)
- [主要フロー](#主要フロー)
- [技術選定](#技術選定)
- [セキュリティ設計](#セキュリティ設計)
- [設計原則](#設計原則)

---

## 全体像

```
┌────────────────────────────────────────────┐
│                External Services             │
│  Discord API                     GitHub API  │
└────────┬──────────────────────────────┬──────┘
         │                              │
         ▼                              ▼
┌────────────────────────────────────────────────────────────┐
│ Interface Layer                                            │
│  - Discord Handler (slash commands, modals, embeds)        │
└────────┬───────────────────────────────────────────────────┘
         │ invokes                                           
┌────────▼───────────────────────────────────────────────────┐
│ Usecase Layer                                              │
│  - SettingUsecase (PAT 登録/除外設定)                     │
│  - IssuesUsecase  (/issues, /assign の取得ロジック)        │
└────────┬───────────────────────────────────────────────────┘
         │ depends on                                        
┌────────▼───────────────────────────────────────────────────┐
│ Domain Layer                                               │
│  - Entity: UserSetting                                     │
│  - Repository Interface                                    │
└────────┬───────────────────────────────────────────────────┘
         │ implemented by                                    
┌────────▼───────────────────────────────────────────────────┐
│ Infrastructure Layer                                       │
│  - database/postgres (UserSettingRepository)               │
│  - crypto/aes (AES-256-GCM)                                │
│  - github/client (REST client + rate limit info)           │
└────────────────────────────────────────────────────────────┘
```

依存方向は常に内側 (Domain) に向き、内側のレイヤーは外側の実装詳細を知りません。

---

## ディレクトリ構成

```
cmd/bot/                  エントリーポイント (環境変数ロード → DI → Discord 起動)
internal/
  domain/
    entity/user_setting.go      PAT・除外設定を表すモデル
    repository/user_setting.go  Repository インターフェース
  usecase/
    setting.go                  PAT 登録・除外設定更新
    issues.go                   GitHub API とのやり取り
  interface/handler/
    discord.go, constants.go    コマンド/モーダル処理
  infrastructure/
    database/postgres.go        repository.UserSettingRepository 実装
    crypto/aes.go               AES-256-GCM 暗号化
    github/client.go            GitHub REST API クライアント
migrations/                 `001`〜`002` の SQL
```

---

## レイヤー別の責務

### Domain Layer

最小限のビジネスルールを保持します。代表的なエンティティ:

```go
type UserSetting struct {
    GuildID                    string
    ChannelID                  string
    UserID                     string
    EncryptedToken             string
    ExcludedRepositories       []string // 旧設定 (互換用)
    ExcludedIssuesRepositories []string
    ExcludedAssignRepositories []string
    NotificationChannelID      string
    NotificationIssuesChannelID string
    NotificationAssignChannelID string
    UpdatedAt                  time.Time
}
```

Repository インターフェースはインフラ層実装を抽象化します。

```go
type UserSettingRepository interface {
    Save(ctx context.Context, setting *entity.UserSetting) error
    FindByGuildAndUser(ctx context.Context, guildID, userID string) (*entity.UserSetting, error)
    SaveNotificationChannelSetting(ctx context.Context, guildID, userID, scope, channelID string) error
    GetNotificationChannels(ctx context.Context, guildID, userID string) (map[string]string, error)
    ClearNotificationChannels(ctx context.Context, guildID, userID string) error
    Delete(ctx context.Context, guildID, userID string) error
    DeleteByGuild(ctx context.Context, guildID string) error
}
```

### Usecase Layer

#### SettingUsecase
- PAT を GitHub API で検証 (`client.ValidateToken`)
- AES-256 で暗号化して保存
- `/issues`・`/assign` 用の除外リストを更新/取得
- 通知チャンネル設定を scope ごとに登録/確認/クリア

#### IssuesUsecase
- PAT 復号 → GitHub API 呼び出し (`GetAllRepositoryIssues`, `GetAllAssignedIssues` など)
- 除外リストに基づくフィルタリング
- Organization / ユーザー全体取得時のリポジトリ横断処理
- Rate limit 情報や失敗したリポジトリ一覧の集約

### Interface Layer

`handler.DiscordHandler` が Discord の Interaction イベントを受け取り、ユースケースを呼び出します。
- `/setting`: モーダル表示、入力値のバリデーション
- `/issues`: 入力文字列を `owner/repo` / `owner` / `all` の 3 種類にパース
- `/assign`: 割り当て Issue を取得し Embed に整形
- エラー/警告メッセージの整形、10 件ごとのメッセージ分割

### Infrastructure Layer

- **database/postgres**: `user_settings` / `user_notification_channels` テーブルへの CRUD。`COALESCE` を使いモーダルから送信されなかったフィールドを維持します。
- **crypto/aes**: 32 バイト鍵で AES-256-GCM を実装。暗号化結果は Base64 文字列。
- **github/client**: 認証ヘッダーを付与した `net/http` クライアント。ページングを `collectAllPages` で抽象化し、Rate Limit をレスポンスヘッダーから解析します。

---

## 主要フロー

### PAT 登録 (`/setting action:token`)
```
Discord Modal
   ↓入力
Handler.showTokenModal → SettingUsecase.SaveToken
   ↓ GitHub API で検証 (401/403 をメッセージ化)
Crypto.AESCrypto.Encrypt
   ↓ encrypted_token を repository.Save へ
PostgreSQL user_settings に upsert
   ↓
Handler がエフェメラルで成功メッセージ
```

### `/issues repository:all`
```
Slash command 受信
   ↓ repository 文字列を解析 (all)
Handler.respondDeferred
   ↓ IssuesUsecase.GetAllRepositoriesIssues
Repository.Find → Crypto.Decrypt
   ↓ GitHub.Client.GetAllUserRepositories
fetchIssuesFromRepositories (各 repo で Issue 取得)
   ↓ 除外リストを適用 / 失敗リポジトリを収集
Handler.createIssueEmbed → 最大 10 件ずつ送信
   ↓ Rate Limit 警告/失敗リストを本文に追記
ユーザーに結果を返信
```

### `/assign`
```
Handler.handleAssignCommand
   ↓ IssuesUsecase.GetAssignedIssues
GitHub.Client.GetAllAssignedIssues
   ↓ ExcludedAssignRepositories でフィルター
結果 0 件ならメッセージ、>0 件なら Embed を送信
```

---

## 技術選定

| カテゴリ | 採用技術 | 主な理由 |
|----------|----------|-----------|
| 言語 | Go 1.20 | マルチゴルーチン + シンプルなデプロイフロー |
| Discord SDK | [discordgo](https://github.com/bwmarrin/discordgo) | スラッシュコマンド対応・安定した実績 |
| DB | PostgreSQL 14 | TEXT[] / JSON / トランザクションサポート |
| HTTP Client | `net/http` | コンテキスト制御と標準実装 |
| 暗号化 | `crypto/cipher` (AES-256-GCM) | 認証付き暗号・標準ライブラリのみで完結 |

---

## セキュリティ設計

- **PAT 暗号化**: 32 バイト鍵に固定し、GCM で暗号化 + Base64。鍵長が異なる場合は起動時にエラー。
- **アクセス制御**: 設定はギルド + ユーザーで分離し、通知チャンネルも scope ごとに個別保存。
- **入力バリデーション**: リポジトリ入力は `/`, `all` などを解析し、その他はエラー。除外パターンも `owner[/repo|/*]` のみ許可し、空白や改行を拒否。
- **エラーメッセージのサニタイズ**: GitHub API のメッセージのみユーザーに転送し、内部エラーは汎用メッセージで隠蔽。
- **ログ**: PAT や入力値はログに出力しません (エラーは `log.Printf` で内容のみ)。

---

## 設計原則

- **依存性逆転**: Usecase は repository インターフェースにのみ依存し、PostgreSQL 実装を知りません。
- **SRP**: Discord ハンドラは入出力整形に専念し、ビジネスルールは Usecase に委譲。
- **YAGNI**: 通知チャンネルなどの未実装フィールドを削除し、現在必要なデータのみ保持。
- **DRY**: GitHub API のページング処理は `collectAllPages` に集約。

---

## 参考

- [docs/SETUP.md](./SETUP.md) – 実行環境の構築
- [docs/DATABASE.md](./DATABASE.md) – `user_settings` テーブル詳細
- [docs/API.md](./API.md) – コマンド仕様
