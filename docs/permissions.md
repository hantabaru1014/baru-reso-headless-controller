# ユーザー権限システム マニュアル

本アプリケーションは **グループ単位の RBAC (Role-Based Access Control)** を採用しています。
管理者・利用者が「何ができるか」「どう設定するか」を知るためのガイドです。

## 1. 概要

- リソース (Host / Session / Account) は必ずいずれかの **グループ** に所属する
- ユーザーはグループに **ロール** を持つメンバーとして所属する
- ロールは **パーミッション** の集合で、何ができるかを定義する
- カスタムロールを作成可能。プリセットされる「seedロール」は編集・削除不可
- リソースには作成者を表す情報 (`created_by`) が記録されるが、権限判定には影響しない

## 2. グループ

### 2.1 種別

| 種別 | 用途 | 作成タイミング | 編集/削除 | メンバー追加 |
|---|---|---|---|---|
| `personal` | ユーザー個人のリソース置き場 | ユーザー作成時に自動 | 名前編集不可 / 削除はユーザー削除時のみ | 不可 (本人のみ) |
| `normal` | 複数ユーザーで共有 | `system:group.manage` 保持者が作成 | 可 | 可 |
| `system` | システム全体の管理権限を持つグループ | デプロイ時に singleton 自動生成 | 削除不可・名前固定 | CLI / `system:group.manage` 経由 |

### 2.2 命名規則

- `personal` は `<user-id>-personal` の形で自動命名 (表示名・名前ともユーザー編集不可)
- `system` は固定名 `system`
- `normal` は作成者が自由に命名可能

### 2.3 メンバーシップ

- 各メンバーは `(group, user)` の組み合わせで 1 つのロールを持つ
- メンバー追加者 (`added_by`) が記録される
- `personal` グループのロール変更は **`system:group.manage` 保持者のみ** 可能 (自己昇格防止)

## 3. ロール

### 3.1 分類

| 種類 | 編集 | 削除 | 管理権限 |
|---|---|---|---|
| **seedロール** (プリセット) | 不可 | 不可 | - |
| **グローバルカスタム** (全グループで利用可) | 可 | 可 | `system:role.manage` |
| **グループ内カスタム** (そのグループ専用) | 可 | 可 | `group:members.manage` |

- グローバルカスタムロールは scope に合致するすべてのグループで割り当て可能
- グループ内カスタムロールはそのグループ内でのみ割り当て可能 (グループ削除時に連動削除)

### 3.2 scope

| scope | 割り当て可能なグループ種別 |
|---|---|
| `normal` | `personal`, `normal` |
| `system` | `system` |

scope が合致しないロールは作成・割り当てできません。

### 3.3 seedロール一覧

| ロール名 | scope | パーミッション |
|---|---|---|
| `admin` | normal | `host:*`, `session:*`, `account:*`, `group:members.manage`, `group:edit` |
| `user` | normal | `host:*`, `session:*`, `account:*` |
| `session-operator` | normal | `host:read`, `host:use`, `session:*`, `account:read`, `account:use` |
| `system-admin` | system | `system:*` |

(`:*` はそのリソース種別のすべてのアクションを意味します)

## 4. パーミッション一覧

### 4.1 normal scope (Host / Session / Account / グループ管理)

| key | 説明 |
|---|---|
| `host:read` | ホスト一覧・詳細・ログ閲覧 |
| `host:write` | ホスト作成・更新・起動・停止・再起動・削除 |
| `host:use` | ホストを指定してセッションを開始 |
| `session:read` | セッション一覧・詳細閲覧 |
| `session:write` | セッション作成・更新・停止・ユーザー招待 / kick / ban / ロール変更 |
| `account:read` | アカウント一覧・詳細・ストレージ情報・コンタクト一覧閲覧 |
| `account:write` | アカウント作成・認証情報更新・削除 |
| `account:use` | アカウントを指定してセッションを開始、DM (コンタクトメッセージ) の閲覧・送信 |
| `group:members.manage` | メンバー追加/削除/ロール変更、グループ内カスタムロール管理 |
| `group:edit` | グループ名等メタデータ編集 |

### 4.2 system scope (システム管理)

| key | 説明 |
|---|---|
| `system:user.create` | システムユーザーアカウントの作成 (招待トークン発行) |
| `system:user.delete` | システムユーザーアカウントの削除 |
| `system:user.list` | (将来用 / 現状ユーザー一覧は認証のみで取得可能) |
| `system:group.list` | 全グループの一覧閲覧 |
| `system:group.manage` | 全グループへの管理操作 (personal含む)、personalグループのロール変更、グループ作成 |
| `system:role.manage` | グローバルカスタムロールの作成・編集・削除 |

## 5. 操作と必要権限

### 5.1 共通ルール

- 操作対象のリソースが所属する **グループ** に対して、必要なパーミッションを持っているかが判定される
- `system:group.manage` を持つユーザーは normal/personal scope のすべての権限を暗黙に持つ (システム管理者の包括権限)

### 5.2 リソース別ガイド

| 操作したいこと | 必要な権限 |
|---|---|
| ホストを起動・停止・削除 | 対象グループに `host:write` |
| 自分のセッションを建てる (任意ホスト指定) | 対象グループに `host:use` + `account:use` + `session:write` |
| セッションを停止 / 設定変更 / kick / ban | 対象グループに `session:write` |
| アカウントを追加・更新 | 対象グループに `account:write` |
| グループにメンバーを招待・削除 | 対象グループに `group:members.manage` |
| グループ名を変更 | 対象グループに `group:edit` |
| 新しいグループを作る | `system:group.manage` |
| 新しいユーザーを招待 (登録URL発行) | `system:user.create` |

### 5.3 同一グループ制約

セッションは「ホスト」と「Resoniteアカウント」を組み合わせて起動します。
**3つは同じグループに所属している必要があります**:

```
session.group_id == host.group_id == account.group_id
```

別グループのリソースを混ぜたセッションは作成できません。
ホスト・アカウントを別グループに移したい場合は、それぞれを移動してから操作してください。

### 5.4 カスタムロール作成時の制約 (権限昇格防止)

グループ内カスタムロールを作成・編集するとき、または既存ロールをメンバーに付与するとき、
**作成者・付与者が持っていないパーミッションを含めることはできません**。

例: グループ A で `group:members.manage` だけを持つユーザーが、`host:write` を含むロールを作成・付与しようとすると拒否されます。`system:group.manage` 保持者 (system-admin) のみがすべてのロールを自由に作成・付与できます。

## 6. ユーザー追加 (招待) フロー

1. system-admin が **管理画面 → ユーザー管理** から「ユーザーを招待」を押す
2. Resonite ID と、招待先ユーザーの personal グループに付与する **デフォルトロール** を選択
3. 登録 URL が発行されるので招待相手に渡す
4. 招待相手は URL を開いて任意の user_id・パスワードを設定して登録完了
5. 登録時に personal グループが自動作成され、指定したロールが付与される

> personal グループの初期ロールは招待トークンと紐付けて DB に保存されるため、URL を改竄しても別のロールにはなりません。

## 7. CLI コマンド

| コマンド | 用途 |
|---|---|
| `brhcli user invite <resoniteID>` | 招待トークンを発行して登録URLを表示 (CLI 経由は seed-admin 固定) |
| `brhcli user create <id> <password> <resoniteID> [--personal-role <role>]` | 開発・運用向け: ユーザーを直接作成 |
| `brhcli user delete <id>` | ユーザーを削除 |
| `brhcli system-admin add <userID>` | system グループに `system-admin` ロールで追加 |
| `brhcli system-admin remove <userID>` | system グループから削除 (最後の 1 人は削除不可) |
| `brhcli migrate` | DB マイグレーションの適用 |

CLI は内部的に固定の **system ユーザー** として実行されるため、すべての権限を持ちます。

## 8. フロントエンドからの操作

- ヘッダー上部のドロップダウンで **操作対象グループ** を切替えられます。一覧画面 (Hosts / Sessions / Accounts) はこのグループ単位で絞り込まれます
- 各画面のボタンは、選択中グループに対する権限が無い場合は非活性化され、ホバーで理由が表示されます
- グループ管理は `/groups` から、システム管理 (ユーザー招待など) は `/admin` から
- カスタムロール作成・グループメンバー管理は対応するグループの詳細画面から

## 9. セキュリティ上の不変条件

- seedロールは編集・削除不可 (アプリ層・DBレベルの両方で防御)
- 未登録の RPC は **デフォルトで拒否** される (新規 RPC 追加時の権限設定漏れを防ぐため)
- personal グループのロール変更は本人では実施不可。`system:group.manage` 保持者が代行
- system グループから **最後のメンバーを削除しようとするとブロックされる** (システム復旧不能を防ぐため)
- 招待トークンと personal グループのロールは DB に永続化され、URL クエリでは渡らない (改竄防止)
- 招待トークンは **SHA-256 ハッシュで DB 保存** され、single-use は条件付き UPDATE でアトミックに保証される (並行登録での二重利用防止)
- 招待トークンの personal ロールは発行時に検証される (normal scope のグローバルロールのみ指定可)
- カスタムロールに含められるパーミッションは、付与する側が持っている範囲に制限される (権限昇格防止)
- 通知ストリーム (`SubscribeNotifications`) のイベントは、イベント元 host が属するグループへの閲覧権限で subscriber ごとにフィルタされる (グループ分離)
- system グループの singleton は DB の partial unique index でも強制される

## 10. 将来拡張の余地

- リソース個別の ACL (グループ全体ではなく特定メンバーへの個別付与)
- ロール継承 (継承元ロールのパーミッションを引き継ぐ)
- グループの階層構造 (組織 → サブグループ)
- リソースの transfer (グループ間移動) RPC
