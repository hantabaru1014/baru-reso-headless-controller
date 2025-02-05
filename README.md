# baru-reso-headless-controller

[baru-reso-headless-container](https://github.com/hantabaru1014/baru-reso-headless-container) の管理ダッシュボードWebアプリ

ステータス:
(WIP) 開発の初期段階なのでいろいろ破壊します  
とりあえず最低限コンテナのマネジメントとセッション管理ができる

## 動作環境
- `network: host` が利用できるdocker
- 対応CPU archはAMD64 or ARM64

## Setup
TODO: 手順を検証してsetupスクリプトの作成

- リポジトリルートにある `docker-compose.*.yml` を作業ディレクトリに配置
- ReleasesからCPUにあった `brhcli-*` を作業ディレクトリにダウンロード
- `.env.template` を元に `.env` を作業ディレクトリに作成
  - シェル展開のように書いている箇所は実際には展開されないので、コマンドを実行した結果を `.env` に記載する
- DBを立ち上げる
  - `docker compose -f docker-compose.db.yml up -d`
- `./brhcli-amd64 migrate` を実行
- 管理者ユーザの作成
  - `./brhcli-amd64 create-user <メールアドレス> <パスワード> <Resonite UserID>`
- 本体を起動
  - `docker compose up -d`
