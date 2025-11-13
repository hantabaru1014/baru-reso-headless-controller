# baru-reso-headless-controller

[baru-reso-headless-container](https://github.com/hantabaru1014/baru-reso-headless-container) の管理ダッシュボードWebアプリ

ステータス:
(WIP) 開発の初期段階なのでいろいろ破壊します  
とりあえず最低限コンテナのマネジメントとセッション管理ができる

## 動作環境
- `network: host` が利用できるdocker
- 対応CPU archはAMD64 or ARM64

## Setup
以下はcontrollerとPostgresSQLをdocker-composeで立ち上げる場合の手順。  
k8sで立ち上げたり、既存のpostgresに接続したい場合はsetup.shの実行まで行ったらcomposeファイルや.envを見て良しなにやってください。  
また、baru-reso-headless-containerのdockerイメージのレジストリにアクセスできる状態である必要があります。

- 空のディレクトリを用意してカレントディレクトリとする
- 必要なファイルのダウンロード、`.env` のセットアップ、DBの起動とマイグレーションを行う
  ```sh
  sh <(curl -s https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/scripts/setup.sh)
  ```
  このスクリプトは以下を自動的に実行します：
  - 必要なファイルのダウンロード（docker-compose.yml、brhcliなど）
  - `.env` ファイルの生成
  - データベースの起動
  - データベースマイグレーションの実行
  - fluentbitユーザーのパスワード設定
- 管理者ユーザの作成
  ```sh
  ./brhcli user create <メールアドレス> <パスワード> <Resonite UserID>
  ```
- 本体を起動
  ```sh
  docker compose up -d
  ```
- 完了。 http://localhost:8014/ でアクセスできます。
  - ポートは `.env` にある `HOST` 環境変数で設定可能
  - 認証認可部分はまだちゃんと作ってないのでエンドポイント自体を何らかの信頼できる方法で保護してください(おすすめ: CloudFlare Zero Trust)

## 既存環境のアップグレード

既に稼働中の環境で最新版にアップグレードする場合、以下のコマンドを実行してください：

```sh
sh <(curl -s https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/scripts/auto-upgrade.sh)
```

このスクリプトは以下を自動的に実行します：
- 最新のdocker-compose.yml、brhcli等のダウンロード
- データベースマイグレーションの実行
- データベースの状態に応じたFluentBit設定（`.pgpass`ファイルの作成など）
- fluentdコンテナの再起動

**注意事項:**
- このスクリプトは`.env`ファイルが存在する既存環境向けです
- 新規セットアップの場合は `setup.sh` を使用してください
- アップグレード前に重要なデータのバックアップを推奨します

## 開発

### テスト

#### 初回セットアップ

テスト用データベースの作成とマイグレーション:
```sh
make test.setup
```
このコマンドは、環境変数 `DB_URL` で指定されたデータベース名に `_test` を追加したテストデータベースを作成し、マイグレーションを実行します。

#### テストの実行

```sh
make test
```

#### テストの構造

- **テストデータベース**: 実際の PostgreSQL データベースを使用しますが、データベース名に `_test` サフィックスが付きます
- **モック**: `mockgen` を使用して、外部依存である `HostConnector` インターフェースのモックを生成します
  - Dockerコンテナを実行せずにテストできるよう、`HostConnector`のみをモック化
  - Repository層は実際の実装を使用し、データベース操作も含めてテスト
  - コード変更後は `make gen.mock` でmock生成が必要
- **テストヘルパー**: `testutil/` パッケージに共通のテストユーティリティが含まれています
