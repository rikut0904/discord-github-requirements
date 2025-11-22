# Discord GitHub 通知 Bot

## 概要

Discord 上で GitHub の Issues を取得・閲覧するための Bot。  
ユーザー自身の GitHub Personal Access Token (PAT) を登録し、そのユーザーがアクセス可能なすべてのリポジトリを対象に、Slash Command から Issue 情報を参照できます。  
設定は「ギルド + チャンネル + ユーザー」単位で保存され、Bot がチャンネルから削除されると即時破棄されます。

## 主な機能

### `/assign`
- 登録ユーザーの PAT で GitHub API `/issues` を呼び出し、自分に割り当てられている Issue を横断取得。
- 結果は Embed 形式で表示（タイトル、番号、ラベル、担当者、更新日時、URL など）。
- `page` / `per` オプションでページング指定が可能。

### `/issues`
- GitHub API `/repos/{owner}/{repo}/issues` を利用し、指定したリポジトリの Issue を取得。
- `repository` オプションで `owner/repo` を指定する（必須）。
- `page` / `per` オプションでページング指定が可能。

### `/setting`
- GitHub Personal Access Token をモーダルで登録/更新。
- トークン検証後、暗号化して保存。

### 初期セットアップ
Bot をチャンネルに追加すると専用スレッドを自動生成し、以下を案内：

1. GitHub API トークン入力モーダル  
2. 通知チャンネル選択ドロップダウン  

完了後は、同スレッドにセットアップ完了メッセージを投稿。  
以降のトークン更新は `/setting` から実行可能。

## 設定・保存方式

- 保存キー: `guild_id + channel_id + user_id`
- 保存内容: 暗号化済みトークン、通知チャンネル ID、更新日時
- Bot 削除（ギルド/チャンネル Remove）イベント発火時に該当キーを即時消去
- ログにはトークンを一切出力しない（管理者も閲覧不可）

## セキュリティ

- トークンは AES 等で暗号化し安全に保管、復号は Bot プロセス内部のみで実行。
- 他ユーザーが設定にアクセスしないよう、`guild/channel/user` の 3 つで ACL を厳格管理。
- GitHub 401 / 403 / 404 / 422 などのエラー時は、原因に応じたガイドを返信。
- Rate Limit 残量が閾値以下の場合、次に実行可能となる時刻を案内。

## エラーハンドリング

- **トークン未登録**: `/setting` の実行を案内
- **認証失敗 (401)**: トークン期限切れ・権限不足を表示
- **権限不足 (403/404)**: リポジトリアクセス権限に関する案内
- **入力エラー (422)**: 入力内容の再確認を促す
- **Rate Limit**: `X-RateLimit-Remaining` を監視し、残量が少ない場合に待機時間を提示

## 技術スタック

- 言語: Go
- アーキテクチャ: クリーンアーキテクチャ
- データベース: PostgreSQL
- Discord SDK: discordgo

## プロジェクト構造

```
├── cmd/bot/           # エントリーポイント
├── internal/
│   ├── domain/        # ドメイン層
│   │   ├── entity/    # エンティティ
│   │   └── repository/# リポジトリインターフェース
│   ├── usecase/       # ユースケース層
│   ├── infrastructure/# インフラ層
│   │   ├── crypto/    # 暗号化
│   │   ├── database/  # DB実装
│   │   ├── discord/   # Discord
│   │   └── github/    # GitHub API
│   └── interface/     # インターフェース層
│       └── handler/   # Discordハンドラー
└── migrations/        # DBマイグレーション
```

## 環境変数

| 変数名 | 説明 |
|--------|------|
| `DISCORD_TOKEN` | Discord Bot Token |
| `DATABASE_URL` | PostgreSQL接続URL |
| `ENCRYPTION_KEY` | AES暗号化キー（32バイト） |

## セットアップ

1. マイグレーション実行
```bash
psql $DATABASE_URL -f migrations/001_create_user_settings.sql
```

2. ビルド・実行
```bash
go build -o bot ./cmd/bot
./bot
```

## 今後の TODO

1. `/assign` コマンド実装（詳細は ISSUE_ASSIGN.md を参照）
2. 初期セットアップ用スレッド生成と通知チャンネル設定 UI
3. Bot削除時のデータ即時消去
