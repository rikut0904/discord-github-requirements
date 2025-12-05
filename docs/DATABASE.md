# データベース設計ドキュメント

Bot はユーザー単位の設定のみを PostgreSQL に保存します。本書ではスキーマと運用に関する基本事項をまとめます。

## 目次
- [概要](#概要)
- [テーブルスキーマ](#テーブルスキーマ)
- [マイグレーション](#マイグレーション)
- [アクセスとクエリ例](#アクセスとクエリ例)
- [メンテナンス・バックアップ](#メンテナンスバックアップ)
- [トラブルシューティング](#トラブルシューティング)

---

## 概要

| 項目 | 内容 |
|------|------|
| RDBMS | PostgreSQL 14+ |
| 接続方法 | `database/sql` + `lib/pq` |
| 保存対象 | PAT (暗号化)、コマンド別除外リスト、通知チャンネル設定 |
| テーブル数 | 2 (`user_settings`, `user_notification_channels`) |

---

## テーブルスキーマ

### `user_settings`

ユーザーが登録した PAT とコマンド別除外設定を保持します。主キーは「ギルド + ユーザー」です。`channel_id` は最後に `/setting` を実行したチャンネルを記録するためのメタデータです。

```sql
CREATE TABLE user_settings (
    guild_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    channel_id VARCHAR(32) NOT NULL,
    encrypted_token TEXT,
    excluded_repositories TEXT[] DEFAULT '{}'::TEXT[], -- 互換用 (非推奨)
    excluded_issues_repositories TEXT[] DEFAULT '{}'::TEXT[],
    excluded_assign_repositories TEXT[] DEFAULT '{}'::TEXT[],
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX idx_user_settings_guild ON user_settings(guild_id);
```

| カラム | 型 | 説明 |
|--------|----|------|
| `guild_id` | VARCHAR(32) | Discord サーバー ID (Snowflake) |
| `channel_id` | VARCHAR(32) | 最後に設定を行ったチャンネル ID (メタデータ) |
| `user_id` | VARCHAR(32) | Discord ユーザー ID |
| `encrypted_token` | TEXT (nullable) | AES-256-GCM + Base64 で暗号化した PAT |
| `excluded_repositories` | TEXT[] | 旧 `/setting action:exclude` 用。互換性のため残置 |
| `excluded_issues_repositories` | TEXT[] | `/issues` コマンドで除外するパターン |
| `excluded_assign_repositories` | TEXT[] | `/assign` コマンドで除外するパターン |
| `updated_at` | TIMESTAMP | 最終更新時刻 (UTC) |

**除外パターンフォーマット**
- `owner/repo` : 特定リポジトリ
- `owner/*` : Organization/ユーザー配下の全リポジトリ
- `owner` : `owner/*` と同義

---

### `user_notification_channels`

通知メッセージを送信するチャンネルをコマンド種別ごとに保持します。`scope` は `issues` / `assign` / `all`（旧設定）を表します。

```sql
CREATE TABLE user_notification_channels (
    guild_id VARCHAR(32) NOT NULL,
    user_id VARCHAR(32) NOT NULL,
    scope VARCHAR(16) NOT NULL,
    channel_id VARCHAR(32) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id, scope),
    CHECK (scope IN ('all', 'issues', 'assign'))
);
```

| カラム | 型 | 説明 |
|--------|----|------|
| `guild_id` | VARCHAR(32) | Discord サーバー ID |
| `user_id` | VARCHAR(32) | Discord ユーザー ID |
| `scope` | VARCHAR(16) | `issues` / `assign` / `all` |
| `channel_id` | VARCHAR(32) | 通知を送るチャンネル ID |
| `updated_at` | TIMESTAMP | 最終更新時刻 |

## マイグレーション

```
migrations/
├── 001_create_user_settings.sql
└── 002_create_user_notification_channels.sql
```

実行例:

```bash
psql $DATABASE_URL -f migrations/001_create_user_settings.sql
psql $DATABASE_URL -f migrations/002_create_user_notification_channels.sql
```

### 変更履歴

| ファイル | 内容 |
|----------|------|
| 001 | `user_settings` を作成。PAT・除外設定・最新の設定チャンネルを保存 |
| 002 | `/issues` / `/assign` で使う通知チャンネルを保持する `user_notification_channels` を作成 |

---

## アクセスとクエリ例

### 接続

```bash
# 接続テスト
psql $DATABASE_URL -c "SELECT 1;"

# 対話モード
psql $DATABASE_URL
```

### 代表的なクエリ

```sql
-- 特定ユーザーの設定状況
SELECT guild_id, channel_id, user_id,
       (encrypted_token IS NOT NULL) AS has_token,
       excluded_issues_repositories,
       excluded_assign_repositories,
       updated_at
FROM user_settings
WHERE guild_id = '123' AND user_id = '789';

-- ギルド内の登録統計
SELECT guild_id,
       COUNT(*) AS users,
       COUNT(*) FILTER (WHERE encrypted_token IS NOT NULL) AS users_with_token,
       COUNT(*) FILTER (WHERE array_length(excluded_issues_repositories, 1) > 0) AS issues_filters,
       COUNT(*) FILTER (WHERE array_length(excluded_assign_repositories, 1) > 0) AS assign_filters
FROM user_settings
GROUP BY guild_id
ORDER BY users DESC;

-- 古い設定の削除 (例: 6 ヶ月未更新)
DELETE FROM user_settings
WHERE updated_at < NOW() - INTERVAL '6 months';
```

---

## メンテナンス・バックアップ

| 作業 | コマンド例 |
|------|-----------|
| 手動バックアップ | `pg_dump $DATABASE_URL > backup.sql` |
| 圧縮バックアップ | `pg_dump $DATABASE_URL | gzip > backup_$(date +%F).sql.gz` |
| リストア | `psql $DATABASE_URL < backup.sql` |
| VACUUM | `VACUUM ANALYZE user_settings;` |
| インデックス再構築 | `REINDEX TABLE user_settings;` |

cron などで日次バックアップを取る場合:

```bash
0 3 * * * pg_dump $DATABASE_URL | gzip > /backups/user_settings_$(date +\%F).sql.gz
```

---

## トラブルシューティング

| 事象 | 対処 |
|------|------|
| テーブルが存在しない | マイグレーションが実行されたか確認。`psql $DATABASE_URL -f migrations/001_create_user_settings.sql` からやり直す |
| `encrypted_token` が NULL なのに PAT が必要と言われる | ユーザーに `/setting action:token` で再登録してもらう |
| 除外設定が効かない | パターン形式を確認 (`owner`, `owner/*`, `owner/repo` のみ許可)。不要な空白・全角文字が混入していないかチェック |

---

## 関連資料

- [SETUP.md](./SETUP.md) – Bot の起動手順
- [ARCHITECTURE.md](./ARCHITECTURE.md) – データベースレイヤーの責務
- [API.md](./API.md) – 除外パターンがどのコマンドに影響するか
