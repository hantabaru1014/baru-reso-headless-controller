# 開発ガイド

## アーキテクチャ

| 領域 | 技術 |
|---|---|
| バックエンド | Go (Clean Architecture)、Connect (gRPC-Web)、Wire (DI)、sqlc |
| フロントエンド | React + Vite、TanStack Query、Jotai、Tailwind CSS + shadcn/ui |
| データベース | PostgreSQL (golang-migrate によるマイグレーション。app 起動時に自動実行) |
| ストレージ | RustFS (S3互換。ワールドバイナリの保存に使用) |
| ログ | fluent-bit 経由でコンテナログを PostgreSQL に集約 |
| イメージビルド | DepotDownloader で Resonite を取得し、[baru-reso-headless-container](https://github.com/hantabaru1014/baru-reso-headless-container) を docker build (BuildKit) でローカルビルド |

### ヘッドレスイメージのローカルビルド

ヘッドレスコンテナのイメージはレジストリから pull せず、controller 自身がローカルでビルドする (`usecase/image_builder`)。流れ:

1. [versions.json](https://github.com/resonite-love/resonite-version-monitor) と container repo の `Headless/AppVersion` を `worker/content_poller.go` が定期ポーリングし、新バージョンを `resonite_versions` テーブルに記録・自動ビルドを enqueue
2. ビルド時は container repo を clone/fetch → DepotDownloader で対象 manifest の Resonite を取得 → `docker build` でホストの Docker デーモンにイメージを格納
3. タグは `<[prerelease-]ResoniteVersion>-<AppVersion>` 形式

必要な環境変数は `STEAM_USERNAME` / `STEAM_PASSWORD` / `HEADLESS_PASSWORD` (headless ブランチのベータアクセスコード)。`git` / `docker` (BuildKit) / DepotDownloader が実行環境に必要で、controller の Docker イメージには同梱済み。ローカル開発 (`go run`) では PATH に DepotDownloader が無ければ GitHub Releases から自動ダウンロードされる。

### ディレクトリ構成

```
proto/      Protocol Buffers 定義 (proto/headless は submodule への symlink)
pbgen/      proto から生成された Go コード (front/pbgen は TypeScript)
domain/     ドメイン層 (エンティティ・エラー)
usecase/    ユースケース層 (ビジネスロジック)
adapter/    アダプタ層 (リポジトリ、RPC サービス、HostConnector)
worker/     バックグラウンドワーカー (イベント監視、非同期 job、予約実行など)
app/        DI 設定 (Wire)、サーバ組み立て
cmd/        エントリポイント (server / cli)
db/         マイグレーション・クエリ定義 (sqlc)
lib/        インフラ・外部APIクライアント (skyfrost、blobstore など)
config/     環境変数の読み込み
front/      フロントエンド (React)
```

## 開発環境のセットアップ

### 必要なもの

- Go (バージョンは [mise.toml](../mise.toml) を参照。[mise](https://mise.jdx.dev/) の利用を推奨)
- Node.js + pnpm
- Docker (開発用 DB とヘッドレスコンテナの起動に使用)

### 初回セットアップ

```sh
# submodule (proto 定義の参照元) を含めて clone
git clone --recursive https://github.com/hantabaru1014/baru-reso-headless-controller.git
cd baru-reso-headless-controller

# フロントエンドの依存をインストール
pnpm install

# .env を用意 (必要な環境変数は scripts/setup.sh と config/config.go を参照)

# 開発用 DB (PostgreSQL / fluent-bit / RustFS) を起動
docker compose -f docker-compose.db.yml up -d

# マイグレーションを実行 (app 起動時にも自動実行される)
make migrate.up

# 管理者ユーザを作成
make build.cli
./bin/brhcli user create <メールアドレス> <パスワード> <Resonite UserID>
```

### 開発サーバの起動

```sh
# フロントエンド (Vite dev server)
pnpm dev

# バックエンド (-fdev で Vite dev server にプロキシ)
go run ./cmd/server -fdev
```

## よく使うコマンド

| コマンド | 説明 |
|---|---|
| `make gen.proto` | protobuf コード生成 (Go / TypeScript) |
| `make gen.wire` | DI コード生成 |
| `make gen.sqlc` | SQL クエリコード生成 |
| `make gen.mock` | テスト用 mock 生成 |
| `make lint` | Go の lint (golangci-lint) |
| `make lint.proto` | proto の format と lint |
| `make build.cli` | CLI (`./bin/brhcli`) のビルド |
| `make build.docker` | Docker イメージのビルド |
| `make exec.psql` | 開発用 DB に psql で接続 |
| `pnpm lint` / `pnpm lint:fix` | フロントエンドの lint |
| `pnpm typecheck` | TypeScript の型チェック |
| `pnpm storybook` | Storybook の起動 |

スキーマ (proto / SQL / Wire 設定) を変更したら対応する `make gen.*` を実行してください。

### データベースマイグレーション

マイグレーションファイルの作成:

```sh
go tool migrate create -ext sql -dir db/migrations <file_name>
```

適用は `make migrate.up` (開発用 DB とテスト用 DB の両方に適用)、または app 起動時の自動実行で行われます。

## テスト

### 初回セットアップ

テスト用データベースの作成とマイグレーション:

```sh
make test.setup
```

このコマンドは、環境変数 `DB_URL` で指定されたデータベース名に `_test` を追加したテストデータベースを作成し、マイグレーションを実行します。

### テストの実行

```sh
make test
```

### テストの構造

- **テストデータベース**: 実際の PostgreSQL データベースを使用しますが、データベース名に `_test` サフィックスが付きます
- **モック**: `mockgen` を使用して、外部依存 (`HostConnector`、skyfrost、blobstore など) のモックを生成します
  - Docker コンテナを実行せずにテストできるよう、外部依存のみをモック化
  - Repository 層は実際の実装を使用し、データベース操作も含めてテスト
  - モック対象のインターフェース変更後は `make gen.mock` で再生成が必要
- **テストヘルパー**: `testutil/` パッケージに共通のテストユーティリティが含まれています
