# セットアップガイド

Discord GitHub 通知 Bot をローカルまたは本番にデプロイする手順をまとめています。基本構成は「Discord Bot + PostgreSQL + GitHub PAT」です。

## 目次
- [前提条件](#前提条件)
- [Discord Bot の準備](#discord-bot-の準備)
- [データベースのセットアップ](#データベースのセットアップ)
- [環境変数の設定](#環境変数の設定)
- [アプリケーションのビルドと実行](#アプリケーションのビルドと実行)
- [Discord サーバーへの招待](#discord-サーバーへの招待)
- [トラブルシューティング](#トラブルシューティング)

---

## 前提条件

| ツール | バージョン | 備考 |
|--------|------------|------|
| Go | 1.20 以上 | `go version` で確認 |
| PostgreSQL | 14 以上 | Docker でも可 |
| Git | 最新を推奨 | リポジトリ管理 |
| Discord Bot Token | - | Developer Portal で発行 |
| GitHub PAT | `repo` scope | Bot 利用ユーザーが個別に準備 |

---

## Discord Bot の準備

1. [Discord Developer Portal](https://discord.com/developers/applications) で新規アプリケーションを作成。
2. 左メニュー「Bot」→「Add Bot」。生成された Token を控えておく。
3. 「Privileged Gateway Intents」で `Server Members Intent` は不要、`Message Content Intent` も不要。
4. 「OAuth2 > URL Generator」で以下を選択して招待 URL を生成。
   - **SCOPES**: `bot`, `applications.commands`
   - **BOT PERMISSIONS**: `Send Messages`, `Embed Links`, `Use Slash Commands`, `Read Message History`

---

## データベースのセットアップ

### Docker Compose を使う場合

```bash
docker-compose up -d
```

`docker-compose.yml` には以下のような PostgreSQL サービスが定義されています。

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:14
    environment:
      POSTGRES_USER: bot
      POSTGRES_PASSWORD: bot_password
      POSTGRES_DB: github_bot
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
volumes:
  postgres_data:
```

### 既存の PostgreSQL を使う場合

1. Bot 専用のユーザーとデータベースを作成。
2. 接続先 URL (`postgresql://user:pass@host:port/db`) を控える。

### マイグレーション

必ず **番号順** に実行します。

```bash
export DATABASE_URL="postgresql://bot:bot_password@localhost:5432/github_bot"
psql $DATABASE_URL -f migrations/001_create_user_settings.sql
psql $DATABASE_URL -f migrations/002_add_excluded_repositories.sql
psql $DATABASE_URL -f migrations/003_add_command_specific_excluded_repositories.sql
```

- `001` : `user_settings` テーブル作成
- `002` : 除外リポジトリ配列と `encrypted_token` の NULL 許可
- `003` : `/issues` 用と `/assign` 用の除外配列を追加 (`excluded_repositories` は互換目的で残存)

---

## 環境変数の設定

```bash
cp .env.example .env
```

`.env` を編集して以下を設定します。

| 変数 | 説明 |
|------|------|
| `DISCORD_TOKEN` | Discord Developer Portal で取得した Bot Token |
| `DATABASE_URL` | PostgreSQL への接続文字列 |
| `ENCRYPTION_KEY` | 32 バイトの AES キー。`openssl rand -hex 16` で生成可能 |

例:

```env
DISCORD_TOKEN=discord_token_here
DATABASE_URL=postgresql://bot:bot_password@localhost:5432/github_bot
ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
```

---

## アプリケーションのビルドと実行

### 開発用途

```bash
# 依存パッケージ
go mod download

# 実行
go run ./cmd/bot
```

### バイナリビルド

```bash
go build -o bot ./cmd/bot
./bot
```

### 本番向けビルド例 (Linux, CGO 無効)

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -ldflags="-s -w" -o bot ./cmd/bot
```

---

## Discord サーバーへの招待

1. 生成した OAuth2 URL を開き、Bot を追加したいサーバーを選択。
2. Bot を起動すると `/setting`, `/issues`, `/assign` が自動登録されます。
3. 初回利用時は `/setting action:token` で PAT を登録してもらってください。

---

## トラブルシューティング

| 症状 | チェックポイント |
|------|------------------|
| Bot が即終了する | `DISCORD_TOKEN` / `DATABASE_URL` / `ENCRYPTION_KEY` が設定されているか確認。`psql $DATABASE_URL -c "SELECT 1;"` で接続テスト。 |
| `encryption key must be 32 bytes` | `openssl rand -hex 16` などで 32 バイトの鍵を再生成。 |
| スラッシュコマンドが表示されない | Bot に `applications.commands` スコープを付与したか、再招待してキャッシュを更新。Discord クライアントを再起動。 |
| Issue 取得で 401/403 | PAT に `repo` 権限があるか、期限切れでないか確認。`/setting` で再登録。 |
| データベース接続失敗 | `docker-compose ps` で PostgreSQL の起動を確認。`DATABASE_URL` のホスト/ポートを再確認。 |

---

## 次のステップ

- [API.md](./API.md) – コマンド仕様
- [DATABASE.md](./DATABASE.md) – テーブルスキーマ
- [DEVELOPMENT.md](./DEVELOPMENT.md) – チーム開発フロー
- [ARCHITECTURE.md](./ARCHITECTURE.md) – 設計の考え方
