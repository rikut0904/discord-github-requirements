# Discord GitHub 通知 Bot

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14+-336791?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Discord から GitHub Issues を横断検索できるスラッシュコマンド Bot です。ユーザーは自分の GitHub Personal Access Token (PAT) を登録し、アクセス可能なリポジトリの Issue を `/issues` や `/assign` コマンドで取得できます。

## 特徴

- 🔐 **ユーザー単位の安全なトークン管理**: モーダル入力 → GitHub API で検証 → AES-256-GCM で暗号化して保存。
- 📂 **柔軟なリポジトリ指定**: `owner/repo`・`owner` (ユーザー/Organization 全体)・`all` の 3 形式をサポート。
- 🚫 **コマンド別の除外設定**: `/setting` から `/issues` 用と `/assign` 用に別々の除外パターンを登録可能。
- 📊 **GitHub Rate Limit を可視化**: 残り回数が少ない場合に警告を表示。
- 🛠️ **クリーンアーキテクチャ**: ドメイン/ユースケース/インターフェース/インフラを分離し、保守・テストしやすい構成。

## スラッシュコマンド概要

| コマンド | 説明 |
|----------|------|
| `/setting` | PAT 登録、`/issues` 用除外リスト、`/assign` 用除外リストをモーダルで編集 |
| `/issues repository:<owner/repo|owner|all>` | 対象リポジトリのオープン Issue を取得。`owner` のみを指定するとそのユーザー/Organization の全リポジトリ、`all` はアクセス可能な全リポジトリを対象にします |
| `/assign` | 自分に割り当てられたオープン Issue を取得 |

詳細なパラメータやレスポンス形式は [`docs/API.md`](docs/API.md) を参照してください。

## クイックスタート

### 前提条件

- Go 1.20 以上
- PostgreSQL 14 以上
- Discord Bot Token
- GitHub Personal Access Token (repo 権限必須)

### セットアップ

```bash
# 1. リポジトリを取得
git clone https://github.com/your-org/github-discord-bot.git
cd github-discord-bot

# 2. 依存関係を取得
go mod download

# 3. データベースを用意（例: Docker Compose）
docker-compose up -d

# 4. マイグレーションを順番に適用
export DATABASE_URL="postgresql://bot:bot_password@localhost:5432/github_bot"
psql $DATABASE_URL -f migrations/001_create_user_settings.sql
psql $DATABASE_URL -f migrations/002_create_user_notification_channels.sql

# 5. 環境変数を設定
cp .env.example .env
# DISCORD_TOKEN / DATABASE_URL / ENCRYPTION_KEY (32byte) を記入

# 6. 実行
go run ./cmd/bot
```

### よく使う開発コマンド

```bash
# テスト
go test ./...

# ビルド
go build -o bot ./cmd/bot

# 生成したバイナリを実行
./bot
```

## アーキテクチャ

```
cmd/bot              # エントリーポイント
internal/
  ├── domain        # エンティティ・リポジトリインターフェース
  ├── usecase       # 設定・Issue 関連ユースケース
  ├── interface     # Discord ハンドラ
  └── infrastructure
       ├── database # PostgreSQL 実装
       ├── crypto   # AES-256-GCM 実装
       └── github   # GitHub API クライアント
migrations/          # SQL マイグレーション
```

各層の責務や処理フローは [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) にまとめています。

## ドキュメント

- [`docs/SETUP.md`](docs/SETUP.md) - 詳細なセットアップ手順
- [`docs/API.md`](docs/API.md) - スラッシュコマンド仕様
- [`docs/DATABASE.md`](docs/DATABASE.md) - スキーマとマイグレーション
- [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) - 開発プロセス

## ライセンス

本プロジェクトは [MIT License](LICENSE) の下で公開されています。
