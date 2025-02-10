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
- 必要なファイルのダウンロードと `.env` のセットアップを行う
  ```sh
  sh <(curl -s https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/scripts/setup.sh)
  ```
- `.env` から環境変数を読み込み
  ```sh
  source .env
  ```
- DBを立ち上げる
  ```sh
  docker compose -f docker-compose.db.yml up -d
  ```
- DBマイグレーションを実行
  ```sh
  ./brhcli migrate
  ```
- 管理者ユーザの作成
  ```sh
  ./brhcli create-user <メールアドレス> <パスワード> <Resonite UserID>
  ```
- 本体を起動
  ```sh
  docker compose up -d
  ```
- 完了。 http://localhost:8014/ でアクセスできます。
  - ポートは `.env` にある `HOST` 環境変数で設定可能
  - 認証認可部分はまだちゃんと作ってないのでエンドポイント自体を何らかの信頼できる方法で保護してください(おすすめ: CloudFlare Zero Trust)
