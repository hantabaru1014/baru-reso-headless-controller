# 開発ガイド

## アーキテクチャ

| 領域 | 技術 |
|---|---|
| バックエンド | Go (Clean Architecture)、Connect (gRPC-Web)、Wire (DI)、sqlc |
| フロントエンド | React + Vite、TanStack Query、Jotai、Tailwind CSS + shadcn/ui |
| データベース | PostgreSQL (golang-migrate によるマイグレーション。app 起動時に自動実行) |
| ストレージ | RustFS (S3互換。ワールドバイナリの保存に使用) |
| ログ | fluent-bit 経由でコンテナログを PostgreSQL に集約 |
| イメージビルド | [baru-reso-headless-container](https://github.com/hantabaru1014/baru-reso-headless-container) が発行する builder image を one-shot container として起動し、ローカルでビルド |

### ヘッドレスイメージのローカルビルド

ヘッドレスコンテナのイメージはレジストリから pull せず、controller が builder image に委譲してローカルでビルドする (`usecase/image_builder`)。controller 自身は git clone / DepotDownloader / docker build を一切実行しない。流れ:

1. [versions.json](https://github.com/resonite-love/resonite-version-monitor) と builder image の `brhc.app-version` label を `worker/content_poller.go` が定期ポーリングし、新バージョンを `resonite_versions` テーブルに記録・自動ビルドを enqueue
2. ビルド時は builder image (`ghcr.io/hantabaru1014/baru-reso-headless-container/builder`) を pull し、Docker SDK で one-shot container として起動。builder が同梱の DepotDownloader で対象 manifest の Resonite を container 内 (ephemeral) に取得し、マウントされたホストの `docker.sock` 経由で inner build を実行してイメージをホストの Docker デーモンに格納する
3. controller は builder container の exit code とビルド結果 image の label (`brhc.build-id` で逆引き → `brhc.image-tag` / `brhc.resonite-version` / `brhc.app-version`) から結果を受け取る
4. タグは `<[prerelease-]ResoniteVersion>-<AppVersion>` 形式

必要な環境変数は `STEAM_USERNAME` / `STEAM_PASSWORD` / `HEADLESS_PASSWORD` (headless ブランチのベータアクセスコード)。builder image の起動先はホストの `docker.sock` で、controller の Docker イメージには git / DepotDownloader / docker CLI を焼き込まない (すべて builder image 側に移管済み)。DepotDownloader のダウンロード先は builder container 内 (ephemeral) で、cache volume は使わない (永続 volume だと古い manifest の残骸が built image に混入しうるため)。builder image 名やマウントする docker.sock パスは環境変数で上書きできる:

| 環境変数 | デフォルト | 用途 |
|---|---|---|
| `RESONITE_BUILDER_IMAGE` | `ghcr.io/hantabaru1014/baru-reso-headless-container/builder:latest` | 起動する builder image |
| `RESONITE_BUILDER_DOCKER_SOCKET` | `/var/run/docker.sock` | builder container にマウントするホスト側 docker.sock パス |

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

> [!NOTE]
> golang-migrate の CLI は DB ドライバを build tag で選ぶため、`go tool migrate` は
> DB に接続できません (`unknown driver postgres`)。`make migrate.up` / `make test.setup` は
> `make build.migrate` で `./bin/migrate` を postgres タグ付きでビルドして使います。
> 手動で `up` / `down` を叩くときも `./bin/migrate` を使ってください
> (ファイル作成は DB に繋がないので `go tool migrate create` で問題ありません)。

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
  - 全パッケージがこの 1 つの DB を共有し、各テストの開始時に `testutil.CleanupTables` が TRUNCATE でクリアします
  - そのため `make test` はパッケージを直列実行します (`-p 1`)。`go test ./...` を直に叩くとパッケージが並列に走り、互いのデータを消し合って不安定になります
  - 同じ理由で、テストの実行中に別のシェルでテストを走らせないでください
- **モック**: `mockgen` を使用して、外部依存 (`HostConnector`、skyfrost、blobstore など) のモックを生成します
  - Docker コンテナを実行せずにテストできるよう、外部依存のみをモック化
  - Repository 層は実際の実装を使用し、データベース操作も含めてテスト
  - モック対象のインターフェース変更後は `make gen.mock` で再生成が必要
- **テストヘルパー**: `testutil/` パッケージに共通のテストユーティリティが含まれています
