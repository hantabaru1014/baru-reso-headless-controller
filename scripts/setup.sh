#!/bin/sh
set -eu

# 必要なコマンドの存在を最初に検証する (途中で失敗しないように)
MISSING=""
for cmd in curl jq openssl docker; do
  if ! command -v "$cmd" > /dev/null 2>&1; then
    MISSING="$MISSING $cmd"
  fi
done
if [ -n "$MISSING" ]; then
  echo "Error: 必要なコマンドが見つかりません:$MISSING" >&2
  echo "インストールしてから再実行してください" >&2
  exit 1
fi
if ! docker compose version > /dev/null 2>&1; then
  echo "Error: docker compose (Compose V2 プラグイン) が利用できません" >&2
  exit 1
fi

if [ -f ".env" ]; then
  echo "Error: .env file already exists"
  exit 1
fi

CPU_ARCH="amd64"
if [ "$(uname -m)" = "arm64" ] || [ "$(uname -m)" = "aarch64" ]; then
  CPU_ARCH="arm64"
fi

curl -O https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/docker-compose.db.yml
curl -O https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/docker-compose.yml
mkdir -p fluentd
curl -o fluentd/container-logs.yaml https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/fluentd/container-logs.yaml
curl -o "brhcli" -L https://github.com/hantabaru1014/baru-reso-headless-controller/releases/latest/download/brhcli-${CPU_ARCH}
chmod a+x brhcli

JWT_SECRET="$(openssl rand -base64 32)"
POSTGRES_PASSWORD="$(openssl rand -base64 32)"
FLUENTBIT_PGSQL_PASSWORD="$(openssl rand -base64 32)"
RUSTFS_ACCESS_KEY="$(openssl rand -hex 8)"
RUSTFS_SECRET_KEY="$(openssl rand -base64 32)"
DOCKER_GID="$(grep docker /etc/group | cut -d: -f3)"

DEFAULT_IMAGE="baru-reso-headless-container"
HEADLESS_IMAGE_NAME=${HEADLESS_IMAGE_NAME:-$DEFAULT_IMAGE}

# ヘッドレスイメージはローカルビルドされるため、Resonite を DepotDownloader で
# 取得するための Steam 認証情報が必要
echo "ヘッドレスイメージのビルドに使う Steam 認証情報を入力してください"
echo "※ パスワードログインができて2段階認証 (Steam Guard) をオフにした新規の専用アカウントを用意してください"
echo "※ ベータアクセスコードはダウンロード時に指定されるため、アカウント自体で headless ブランチを有効化する必要はありません"
printf '%s' 'Steam ユーザー名: '
read -r STEAM_USERNAME
printf '%s' 'Steam パスワード: '
read -r STEAM_PASSWORD
printf '%s' 'Resonite headless ブランチのベータアクセスコード: '
read -r HEADLESS_PASSWORD

printf '%s' 'DB_URL を入力 (default: postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/brhcdb?sslmode=disable): '
read -r DB_URL
DB_URL=${DB_URL:-"postgres://postgres:$(echo $POSTGRES_PASSWORD | jq -Rr @uri)@localhost:5432/brhcdb?sslmode=disable"}

cat > .env << EOF
JWT_SECRET="${JWT_SECRET}"
HEADLESS_IMAGE_NAME="${HEADLESS_IMAGE_NAME}"
DOCKER_GID="${DOCKER_GID}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}"
FLUENTBIT_PGPASSWORD="${FLUENTBIT_PGSQL_PASSWORD}"
DB_URL="${DB_URL}"
HOST=":8014"
CONTAINER_LOGS_FLUENTD_ADDRESS=":24224"

# ヘッドレスイメージのローカルビルド用 Steam 認証情報
STEAM_USERNAME="${STEAM_USERNAME}"
STEAM_PASSWORD="${STEAM_PASSWORD}"
HEADLESS_PASSWORD="${HEADLESS_PASSWORD}"

# versions.json / builder image の確認間隔 (デフォルト: 1h)
#CONTENT_CHECK_INTERVAL=1h
# 新バージョン検知時に自動ビルドするか (デフォルト: true)
#AUTO_BUILD_NEW_VERSIONS=true
# builder image の AppVersion 更新時に自動再ビルドするか (デフォルト: true)
#AUTO_BUILD_ON_APP_VERSION_BUMP=true

# セッションが使うポートの範囲 (未設定なら固定せずランダムなポートを使う)
#SESSION_PORT_MIN=40000
#SESSION_PORT_MAX=50000
# headless container を動かしているホストのグローバル IP (またはホスト名)
# QUIC で外部から接続させるために必要。設定すると QUIC のポートも上記範囲から自動割り当てする
#HEADLESS_PUBLIC_IP=

# RustFS (S3互換ストレージ)
RUSTFS_ACCESS_KEY="${RUSTFS_ACCESS_KEY}"
RUSTFS_SECRET_KEY="${RUSTFS_SECRET_KEY}"
RUSTFS_ENDPOINT="localhost:9000"
RUSTFS_USE_SSL=false
WORLD_DOWNLOADS_BUCKET_NAME="world-downloads"
BLOB_TTL_DAYS=3
EOF

echo ""
echo "===== データベースのセットアップを開始します ====="
echo ""

# DB / RustFS コンテナを起動
# fluentd は DB 起動 & fluentbit ユーザーのパスワード設定後でないと接続に失敗して終了するため、ここでは起動しない
echo "1. データベースを起動中..."
docker compose -f docker-compose.db.yml up -d db rustfs

# DBの起動を待機
echo "2. データベースの起動を待機中..."
for i in $(seq 1 30); do
  if docker compose -f docker-compose.db.yml exec -T db pg_isready -U postgres > /dev/null 2>&1; then
    echo "   データベースが起動しました"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "エラー: データベースの起動がタイムアウトしました"
    exit 1
  fi
  sleep 1
done

# マイグレーションを実行 (app コンテナに埋め込まれたマイグレーションを one-shot 実行)
echo "3. データベースマイグレーションを実行中..."
docker compose run --rm -T app -migrate-only

# fluentbitユーザーのパスワードを設定
echo "4. fluentbitユーザーのパスワードを設定中..."
docker compose -f docker-compose.db.yml exec -T db psql -U postgres -d brhcdb -c "ALTER USER fluentbit WITH PASSWORD '${FLUENTBIT_PGSQL_PASSWORD}';"

echo "5. サービス起動中..."
docker compose -f docker-compose.db.yml up -d
docker compose up -d

echo ""
echo "✅ データベースのセットアップが完了しました！"
echo ""
echo "次のステップ:"
echo "1. 管理者ユーザを作成してください:"
echo "   ./brhcli user create <ID> <パスワード> <Resonite UserID>"
echo "   ./brhcli system-admin add <ID>"
echo ""
echo "2. http://localhost:8014/ でアクセスできます"
echo ""
