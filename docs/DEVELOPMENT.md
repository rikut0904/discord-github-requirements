# 開発ガイド

チーム開発時の基本フロー、ツール、コーディングルールをまとめています。Bot 本体は小規模ですが、Discord / GitHub / DB の 3 種類の外部依存を扱うため運用手順を整理しています。

## 目次
- [開発環境の準備](#開発環境の準備)
- [コーディングガイドライン](#コーディングガイドライン)
- [テストと検証](#テストと検証)
- [デバッグ](#デバッグ)
- [コントリビューションフロー](#コントリビューションフロー)
- [リリースと CI](#リリースと-ci)
- [トラブルシューティング](#トラブルシューティング)

---

## 開発環境の準備

```bash
git clone https://github.com/your-org/github-discord-bot.git
cd github-discord-bot
go mod download
```

### データベース

```bash
docker-compose up -d
export DATABASE_URL="postgresql://bot:bot_password@localhost:5432/github_bot"
psql $DATABASE_URL -f migrations/001_create_user_settings.sql
psql $DATABASE_URL -f migrations/002_create_user_notification_channels.sql
```

### 環境変数

`.env` を作成し、`DISCORD_TOKEN` / `DATABASE_URL` / `ENCRYPTION_KEY` (32 バイト) をセット。VS Code などのエディタでは `.env` を読み込むプラグインを使うと便利です。

### 実行

```bash
go run ./cmd/bot        # 開発時
./bot                   # ビルド済みバイナリ
```

---

## コーディングガイドライン

- **スタイル**: `gofmt` / `goimports` で自動整形。可能であれば `golangci-lint` を導入。
- **命名**: パッケージ・変数は小文字のキャメルケース。定数は `MsgTokenSaved` のようにプレフィックスでグループ化。
- **エラー処理**: `fmt.Errorf("context: %w", err)` でラップし、ユーザー向け文言は `internal/interface/handler/constants.go` で一元管理。
- **コメント**: 複雑な処理 (例: 除外パターンマッチ、ページング処理) の前には 1 行コメントで目的を明示。
- **依存注入**: `cmd/bot/main.go` でユースケースやリポジトリを組み立てる。新しい依存を追加する場合はここで渡す。

---

## テストと検証

```bash
# 全テスト
go test ./...

# カバレッジ付
go test -cover ./internal/...
```

- ユースケースはリポジトリ/暗号実装をモックしてユニットテストする。
- GitHub API との統合テストは環境変数 `GITHUB_TOKEN_FOR_TEST` を利用し、CI ではスキップする方針。
- データベースを伴うテストでは `TEST_DATABASE_URL` を使用し、副作用が残らないように `t.Cleanup` で削除。

---

## デバッグ

- **ログ**: `log.Printf` で十分。センシティブ情報は出さない。
- **Delve**: `dlv debug ./cmd/bot` でブレークポイントを設定可能。
- **Rate Limit 調査**: GitHub API から返る `X-RateLimit-*` を `github.Client` のログに出すときは開発環境に限定。

---

## コントリビューションフロー

1. Issue 作成 → チームで要件合意。
2. ブランチ作成: `git checkout -b feature/<短く説明>`。
3. コード変更 → `go test ./...` → `golangci-lint run`。
4. コミットメッセージ: `feat: add assign exclude modal` のように conventional commits を推奨。
5. Pull Request に以下を記載
   - 目的
   - 変更点の概要
   - 動作確認方法 (スクリーンショット/ログがあると親切)
6. レビュー後に `main` へマージ。

---

## リリースと CI

- **GitHub Actions**: `go test ./...` と `golangci-lint run` を最低限実行する CI を想定。
- **タグ付け**: 機能追加時に `vX.Y.Z` のタグを付け、Release Note に主な変更を列挙。
- **デプロイ**: Heroku/Railway などコンテナ実行環境を想定。`.env` と同じ内容を環境変数に設定し、マイグレーションは手動または起動スクリプトで実施。

---

## トラブルシューティング

| 問題 | 対処 |
|------|------|
| PAT 登録で 401 エラー | PAT が `repo` スコープを持つか確認。GitHub 側で再生成。 |
| `/issues repository:all` が遅い | 除外パターンで対象を絞る、もしくは `owner` / `owner/repo` を使うよう案内。GitHub API の Rate Limit に注意。 |
| Discord でモーダルが開かない | Bot が最新のスラッシュコマンドを登録できているか、Bot 再起動＆アプリケーションコマンドを再登録。 |
| DB の設定が消えた | `user_settings` の `updated_at` を監視し、古いデータ削除ジョブが動いていないか確認。 |

---

## 参考リンク

- [docs/SETUP.md](./SETUP.md)
- [docs/API.md](./API.md)
- [docs/ARCHITECTURE.md](./ARCHITECTURE.md)
- [docs/DATABASE.md](./DATABASE.md)
