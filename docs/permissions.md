# ユーザー権限システム設計

このドキュメントは本アプリケーションのユーザー権限システムの仕様書兼ユーザー向けマニュアルです。実装はこのドキュメントに準拠します。

## 1. 概要

権限モデルは **グループ単位の RBAC (Role-Based Access Control)** を採用します。

- リソース (Host, Session, Account) は必ずいずれかの **グループ** に所属する
- ユーザーはグループに対して **ロール** を持つメンバーとして所属する
- ロールは **パーミッション** の集合で、何ができるかを定義する
- カスタムロールを作成可能。プリセットされる「seedロール」は編集・削除不可

権限とは独立した概念として、すべてのリソースに `created_by` (作成者) を保持します。

## 2. グループ

### 2.1 種別

| type | 用途 | 作成タイミング | 編集/削除 | メンバー追加 |
|---|---|---|---|---|
| `personal` | ユーザー個人のリソース置き場 | ユーザー作成時に自動 | 名前編集不可 / 削除はユーザー削除時のみ | 不可 (本人のみ) |
| `normal` | 複数ユーザーで共有 | ユーザーが任意に作成 | 可 | 可 |
| `system` | システム全体の管理権限を持つグループ | デプロイ時に singleton 自動生成 | 削除不可・名前固定 | 可 (CLI/system:group.manage 経由) |

### 2.2 命名規則

- `personal` グループは `<user-id>-personal` のような形式で自動命名され、表示名・名前ともユーザー編集不可
- `system` グループは固定名 (`system`)
- `normal` グループはユーザーが自由に命名

### 2.3 メンバーシップ

- 各メンバーは `(group, user)` の組み合わせで1つのロールを持つ
- メンバー追加者 (`added_by`) を記録する
- `personal` グループのロール変更は **`system:group.manage` 保持者のみ** 可能 (自己昇格防止)

## 3. ロール

### 3.1 分類

| group_id | is_builtin | 種類 | 編集 | 削除 | 管理権限 |
|---|---|---|---|---|---|
| NULL | true | **seedロール** | 不可 | 不可 | - |
| NULL | false | **グローバルカスタム** | 可 | 可 | `system:role.manage` |
| NOT NULL | false | **グループ内カスタム** | 可 | 可 | `group:members.manage` |

- グローバルカスタムロールは、scope に合致するすべてのグループで割り当て可能
- グループ内カスタムロールはそのグループ内でのみ割り当て可能
- グループ内カスタムロールは、グループ削除時に連動して削除

### 3.2 scope

| scope | 割り当て可能なグループ種別 |
|---|---|
| `normal` | `personal`, `normal` |
| `system` | `system` |

scopeが合致しない組み合わせは作成・割り当て不可。

### 3.3 seedロール一覧

| ロール名 | scope | パーミッション |
|---|---|---|
| `admin` | normal | `host:*`, `session:*`, `account:*`, `group:members.manage`, `group:edit` |
| `user` | normal | `host:*`, `session:*`, `account:*` |
| `session-operator` | normal | `host:read`, `host:use`, `session:*`, `account:read`, `account:use` |
| `system-admin` | system | `system:*` |

(`:*` はそのリソース種別のすべてのアクションを意味する)

## 4. パーミッション一覧

### 4.1 normal scope (Host/Session/Account/グループ管理)

| key | 説明 |
|---|---|
| `host:read` | ホスト一覧・詳細・ログ閲覧 |
| `host:write` | ホスト作成・更新・起動・停止・再起動・削除 |
| `host:use` | ホストを指定してセッションを開始 |
| `session:read` | セッション一覧・詳細閲覧 |
| `session:write` | セッション作成・更新・停止・ユーザー招待/kick/ban/ロール変更 |
| `account:read` | アカウント一覧・詳細・ストレージ情報閲覧 |
| `account:write` | アカウント作成・認証情報更新・削除 |
| `account:use` | アカウントを指定してセッションを開始 |
| `group:members.manage` | メンバー追加/削除/ロール変更、グループ内カスタムロール管理 |
| `group:edit` | グループ名等メタデータ編集 |

### 4.2 system scope (システム管理)

| key | 説明 |
|---|---|
| `system:user.create` | システムユーザーアカウントの作成 |
| `system:user.delete` | システムユーザーアカウントの削除 |
| `system:user.list` | 全システムユーザーの一覧閲覧 |
| `system:group.list` | 全グループの一覧閲覧 |
| `system:group.manage` | 全グループへの管理操作 (personal含む)、personalグループのロール変更 |
| `system:role.manage` | グローバルカスタムロールの作成・編集・削除 |

## 5. リソースと権限のマッピング

### 5.1 制約

- **同一グループ制約**: セッション作成時、`session.group_id == host.group_id == account.group_id` を満たすこと
- リソース作成時は `group_id` を明示的に指定する。指定しない/できない場合は作成者の personal グループ
- `created_by` は権限とは無関係。誰がリソースを作成したかの記録用途

### 5.2 既存 RPC 権限マッピング (代表例)

| RPC カテゴリ | 必要パーミッション (操作対象グループに対して) |
|---|---|
| `ListHosts` / `GetHost` / ホスト統計取得 | `host:read` |
| `StartHeadlessHost` / `StopHeadlessHost` / ホスト更新 | `host:write` |
| `CreateSession` (ホスト指定) | `host:use` + `account:use` + `session:write` |
| `ListSessions` / `GetSession` | `session:read` |
| `UpdateSession` / `StopSession` / `InviteUser` / `KickUser` / `BanUser` / `UpdateUserRole` | `session:write` |
| `ListAccounts` / `GetAccount` / `GetAccountStorageInfo` | `account:read` |
| `CreateAccount` / `UpdateAccountCredentials` / `DeleteAccount` | `account:write` |

詳細マッピングは proto / RPC 実装と合わせて随時更新する。

## 6. データモデル

### 6.1 新規テーブル

```
groups
  id          text PRIMARY KEY
  name        text NOT NULL
  type        text NOT NULL CHECK (type IN ('personal','normal','system'))
  created_at  timestamptz NOT NULL DEFAULT now()
  updated_at  timestamptz NOT NULL DEFAULT now()

roles
  id          text PRIMARY KEY
  group_id    text REFERENCES groups(id) ON DELETE CASCADE  -- NULL = グローバル
  name        text NOT NULL
  scope       text NOT NULL CHECK (scope IN ('normal','system'))
  is_builtin  boolean NOT NULL DEFAULT false
  created_at  timestamptz NOT NULL DEFAULT now()
  updated_at  timestamptz NOT NULL DEFAULT now()
  UNIQUE (group_id, name)  -- NULL は別扱いだがグローバル内で重複防止

role_permissions
  role_id         text NOT NULL REFERENCES roles(id) ON DELETE CASCADE
  permission_key  text NOT NULL
  PRIMARY KEY (role_id, permission_key)

group_members
  group_id   text NOT NULL REFERENCES groups(id) ON DELETE CASCADE
  user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE
  role_id    text NOT NULL REFERENCES roles(id)
  added_by   text REFERENCES users(id)  -- NULL: システム自動作成
  joined_at  timestamptz NOT NULL DEFAULT now()
  PRIMARY KEY (group_id, user_id)
```

### 6.2 既存テーブル変更

```
hosts
  + group_id    text NOT NULL REFERENCES groups(id)
  + created_by  text REFERENCES users(id)   -- 旧 owner_id をリネーム
  - owner_id    (削除)

sessions
  + group_id    text NOT NULL REFERENCES groups(id)
  + created_by  text REFERENCES users(id)   -- 旧 owner_id をリネーム
  - owner_id    (削除)

accounts
  + group_id    text NOT NULL REFERENCES groups(id)
  + created_by  text REFERENCES users(id)   -- 新規 (nullable)
```

## 7. マイグレーション戦略

既存データの安全な移行のため、以下の順序で1マイグレーションファイル内で実施:

1. 新テーブル (`groups`, `roles`, `role_permissions`, `group_members`) を作成
2. seedロール (`admin`, `user`, `session-operator`, `system-admin`) を投入 (`is_builtin=true`, `group_id=NULL`)
3. `system` グループ (singleton) を作成。初期メンバーなし
4. 既存ユーザーごとに **personal グループ** を自動生成し、本人を `admin` (seed) として登録
5. **"移行前全体グループ" (normal)** を1つ作成し、既存ユーザー全員を `admin` で登録
6. 既存の `hosts`, `sessions`, `accounts` に `group_id` を追加し、すべて移行前全体グループに所属させる
7. 既存の `owner_id` を `created_by` にコピーして列名変更 (accounts は新規 nullable 列追加)
8. 旧 `owner_id` 列を drop

## 8. CLI コマンド

| コマンド | 目的 |
|---|---|
| `brhcli system-admin add <userID>` | systemグループに `system-admin` ロールで追加 |
| `brhcli system-admin remove <userID>` | systemグループから削除 |
| `brhcli user create <email> <password> <userID> [--personal-role <role>]` | ユーザー作成時に personal グループに付与するロールを指定可能 (省略時 `admin`) |

## 9. 新規 RPC (概要)

新たに以下のサービス群が必要:

### GroupService
- `CreateGroup` / `GetGroup` / `ListGroups` / `UpdateGroup` / `DeleteGroup`
- `ListGroupMembers` / `AddGroupMember` / `RemoveGroupMember` / `UpdateGroupMemberRole`

### RoleService
- `ListRoles` (グローバル + 指定グループのカスタム)
- `CreateRole` / `UpdateRole` / `DeleteRole`
- `ListPermissions` (利用可能 permission_key 一覧)

### 既存 RPC の変更
- 各 RPC のリクエストには関連リソース ID (host_id, session_id, account_id, group_id) が含まれている前提
- 認証 interceptor の後段で **権限 interceptor** が動作し、対象リソースの `group_id` を引いて該当ユーザーの権限を判定
- 不足時は `permission_denied` を返す
- システム権限 (`system:*`) を持つユーザーは該当 normal/personal グループ権限を持っているとみなす (`system:group.manage` がある場合)

## 10. フロントエンド

### 10.1 画面追加
- グループ管理 (一覧・作成・編集・削除)
- グループ詳細 (メンバー一覧・ロール変更・カスタムロール管理)
- グローバルロール管理 (system:role.manage 保持者向け)
- システム管理画面 (system:user.* / system:group.list 保持者向け)

### 10.2 UI 出し分け
- ログインユーザーの保持パーミッションをログイン後の API で取得し、メニュー項目とボタンを動的に出し分け
- 操作対象のグループに対する権限が無い場合はボタン非活性 + 理由のツールチップ表示

## 11. セキュリティ上の不変条件

- seedロールは何があっても編集・削除されない (アプリ層とDBレベル両方で防御)
- personal グループの本人ロールは `group:members.manage` では変更不可、必ず `system:group.manage` 経由
- system グループの最後の `system-admin` メンバーを削除する場合の挙動: 現状ガードなし。CLIで再投入可能とする方針 (運用ドキュメントで周知)
- リソースの `group_id` 変更 (transfer) は別途 RPC を設計し、移動先・移動元の双方で適切な権限を要求する (本リリースのスコープ外)

## 12. 将来拡張の余地

- リソース個別の ACL (グループ全体ではなく特定メンバーへの個別付与)
- ロール継承 (継承元ロールの permission を引き継ぐ)
- グループの階層構造 (組織 → サブグループ)
- リソースの transfer (グループ間移動) RPC
