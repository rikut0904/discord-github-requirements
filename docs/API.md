# Discord コマンド API リファレンス

Bot が公開するスラッシュコマンドの仕様をまとめています。コマンドはいずれも Discord 上で実行し、結果は Embed で返されます。

## コマンド一覧

| コマンド | 目的 | 主な引数 |
|----------|------|-----------|
| `/setting` | PAT と除外リポジトリの登録 | `action` (必須) |
| `/issues` | 指定範囲のオープン Issue を取得 | `repository` (必須) |
| `/assign` | 自分に割り当てられた Issue を取得 | なし |

---

## `/setting` – 設定モーダル

ユーザー自身の GitHub Personal Access Token (PAT) と、Issue を非表示にしたいリポジトリを登録します。設定は「ギルド × チャンネル × ユーザー」の組み合わせで保存されます。

### 引数

| 名前 | 型 | 必須 | 説明 |
|------|----|------|------|
| `action` | string | ✅ | 実行する設定操作。`token` / `exclude_issues` / `exclude_assign` |

### `action: token` – PAT 登録

1. モーダルに PAT を入力 (`ghp_` で始まる文字列など)。
2. Bot が GitHub API (`/user`) でトークンを検証します。
3. 成功すると AES-256-GCM で暗号化し、`user_settings` テーブルに保存します。
4. エラー時はモーダル送信者のみにエラーメッセージを返信します。

**必要な権限**
- `repo`
- `read:user`

### `action: exclude_issues` – `/issues` 用除外リスト

- モーダルのテキストエリアに 1 行 1 パターンで入力します。
- 許可されるパターン
  - `owner/repo` : 特定リポジトリを除外
  - `owner/*` : Organization/ユーザー配下の全リポジトリを除外
  - `owner` : `owner/*` と同義
- 入力済みの値はモーダル表示時に自動で埋め込まれます。
- 空送信すると設定がクリアされます。

### `action: exclude_assign` – `/assign` 用除外リスト

`exclude_issues` と同じ形式で、`/assign` コマンドの結果にのみ適用されます。

### バリデーションとレスポンス

| 状態 | メッセージ例 |
|------|--------------|
| 登録成功 | `✅ GitHub Token を登録しました` / `✅ issues用に3件のリポジトリを除外リストに設定しました` |
| PAT 未登録で他コマンド実行 | `❌ トークンが登録されていません。/setting でトークンを登録してください。` |
| GitHub API エラー | `❌ トークンの検証に失敗しました: 認証に失敗しました。トークンが無効または期限切れです。` |
| 除外パターン不正 | `❌ 不正な形式があります: ...` |

レスポンスはいずれもエフェメラル (送信者のみ可視) です。

---

## `/issues` – リポジトリの Issue 取得

指定した範囲のオープン Issue を Embed で一覧表示します。結果は最大 10 件ずつメッセージを分割して送信されます。

### 引数

| 名前 | 型 | 必須 | 説明 |
|------|----|------|------|
| `repository` | string | ✅ | 取得対象。以下 3 パターンのいずれか |

#### 受け付ける値

| 入力例 | 種別 | 動作 |
|--------|------|------|
| `owner/repo` | 特定リポジトリ | `owner/repo` の Issue を取得 |
| `owner` | ユーザー/Organization 全体 | 指定ユーザー (または Org) が所有する各リポジトリの Issue をすべて取得 |
| `all` | すべて | アクセス可能な全リポジトリの Issue を取得 |

### レスポンス

- Embed 1 件につき 1 Issue。タイトル、URL、状態、ラベル、担当者、更新日時を含みます。
- GitHub Rate Limit の残回数がしきい値 (10) 未満の場合、冒頭に `⚠️ API Rate Limit 残り: X (リセット: HH:MM:SS)` が表示されます。
- 「all / owner」指定時に一部リポジトリで取得失敗した場合は、失敗したリポジトリ一覧を警告として追記します。

### エラーパターン

| 条件 | 表示されるメッセージ |
|------|--------------------|
| PAT 未登録 | `❌ トークンが登録されていません ...` |
| 無効な `repository` 形式 | `❌ repository は owner/repo 形式、username 形式、または all を指定してください。` |
| GitHub API エラー | `❌ GitHub API エラー: ...` |
| Issue 0 件 | `📭 Issue が見つかりませんでした` |

---

## `/assign` – 担当 Issue 取得

自分に割り当てられているオープン Issue を GitHub API (`/issues?filter=assigned`) から取得します。結果表示・エラー処理は `/issues` と同様です。

- `/setting action:exclude_assign` で登録したパターンが適用されます。
- Issue が 1 件もない場合は `📭 割り当てられた Issue は見つかりませんでした` を返します。

---

## 使用例

```text
/setting action:token
→ PAT 登録モーダルが表示されます。

/setting action:exclude_issues
→ `/issues` で無視したい `owner/*` を入力します。

/issues repository:all
→ アクセス権のある全リポジトリの Issue をまとめて取得。

/issues repository:octocat
→ `octocat` が所有する全リポジトリの Issue を取得。

/issues repository:octocat/hello-world
→ 特定リポジトリのみ取得。

/assign
→ 自分が担当している Issue を横断的に表示。
```

---

## 関連ドキュメント

- [`docs/SETUP.md`](./SETUP.md) – Bot の起動方法
- [`docs/DATABASE.md`](./DATABASE.md) – `user_settings` テーブル仕様
- [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md) – 各レイヤーの責務
