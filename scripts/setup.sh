#!/bin/sh
set -eu

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
curl -o "brhcli" -L https://github.com/hantabaru1014/baru-reso-headless-controller/releases/latest/download/brhcli-${CPU_ARCH}
chmod a+x brhcli

JWT_SECRET="$(openssl rand -base64 32)"
POSTGRES_PASSWORD="$(openssl rand -base64 32)"
FLUENTBIT_PGSQL_PASSWORD="$(openssl rand -base64 32)"
RUSTFS_ACCESS_KEY="$(openssl rand -hex 8)"
RUSTFS_SECRET_KEY="$(openssl rand -base64 32)"
DOCKER_GID="$(grep docker /etc/group | cut -d: -f3)"

DEFAULT_IMAGE="baru-reso-headless-container"
read -p "ヘッドレスのdocker image nameを入力 (default: ${DEFAULT_IMAGE}): " HEADLESS_IMAGE_NAME
HEADLESS_IMAGE_NAME=${HEADLESS_IMAGE_NAME:-$DEFAULT_IMAGE}

# ヘッドレスイメージはローカルビルドされるため、Resonite を DepotDownloader で
# 取得するための Steam 認証情報が必要
echo "ヘッドレスイメージのビルドに使う Steam 認証情報を入力してください"
read -p 'Steam ユーザー名: ' STEAM_USERNAME
read -p 'Steam パスワード: ' STEAM_PASSWORD
read -p 'Resonite headless ブランチのベータアクセスコード: ' HEADLESS_PASSWORD

read -p 'DB_URL を入力 (default: postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/brhcdb?sslmode=disable): ' DB_URL
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

# DBコンテナを起動
echo "1. データベースを起動中..."
docker compose -f docker-compose.db.yml up -d

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

echo ""
echo "✅ データベースのセットアップが完了しました！"
echo ""
echo "次のステップ:"
echo "1. 管理者ユーザを作成してください:"
echo "   ./brhcli user create <メールアドレス> <パスワード> <Resonite UserID>"
echo ""
echo "2. 本体を起動してください:"
echo "   docker compose up -d"
echo ""
echo "3. http://localhost:8014/ でアクセスできます"
echo ""
