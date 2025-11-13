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
DOCKER_GID="$(grep docker /etc/group | cut -d: -f3)"

DEFAULT_IMAGE="ghcr.io/hantabaru1014/baru-reso-headless-container"
read -p "ヘッドレスのdocker image nameを入力 (default: ${DEFAULT_IMAGE}): " HEADLESS_IMAGE_NAME
HEADLESS_IMAGE_NAME=${HEADLESS_IMAGE_NAME:-$DEFAULT_IMAGE}

read -p 'レジストリはGitHub Container Registry？ [y/N] (default: y): ' IS_GHCR
case "${IS_GHCR}" in
n|N|no|NO)
  read -p 'docker image pullに必要なAuth情報を入力: ' HEADLESS_REGISTRY_AUTH
  ;;
*)
  if command -v git >/dev/null 2>&1 && DEFAULT_GITHUB_USER="$(git config user.name)"; then
    read -p "image pullを行うGitHubのユーザー名を入力 (default: ${DEFAULT_GITHUB_USER}): " GHCR_USERNAME
    GHCR_USERNAME=${GHCR_USERNAME:-$DEFAULT_GITHUB_USER}
  else
    read -p "image pullを行うGitHubのユーザー名を入力: " GHCR_USERNAME
  fi
  read -p 'read:packages scope を持つ GitHub Personal Access Token を入力: ' GHCR_TOKEN
  HEADLESS_REGISTRY_AUTH="{\\\"username\\\":\\\"${GHCR_USERNAME}\\\",\\\"password\\\":\\\"${GHCR_TOKEN}\\\"}"
  GHCR_AUTH_TOKEN="${GHCR_TOKEN}"
  ;;
esac

read -p 'DB_URL を入力 (default: postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/brhcdb?sslmode=disable): ' DB_URL
DB_URL=${DB_URL:-"postgres://postgres:$(echo $POSTGRES_PASSWORD | jq -Rr @uri)@localhost:5432/brhcdb?sslmode=disable"}

cat > .env << EOF
JWT_SECRET="${JWT_SECRET}"
HEADLESS_IMAGE_NAME="${HEADLESS_IMAGE_NAME}"
HEADLESS_REGISTRY_AUTH="${HEADLESS_REGISTRY_AUTH}"
GHCR_AUTH_TOKEN="${GHCR_AUTH_TOKEN}"
DOCKER_GID="${DOCKER_GID}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}"
DB_URL="${DB_URL}"
HOST=":8014"
CONTAINER_LOGS_FLUENTD_ADDRESS=":24224"

# コンテナイメージの確認間隔（秒単位、デフォルト: 15秒）
IMAGE_CHECK_INTERVAL_SEC=15
# 新しいコンテナイメージを自動的にプルするか（デフォルト: false）
AUTO_PULL_NEW_IMAGE=true
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

# マイグレーションを実行
echo "3. データベースマイグレーションを実行中..."
./brhcli migrate

# fluentbitユーザーのパスワードを設定
echo "4. fluentbitユーザーのパスワードを設定中..."
docker compose -f docker-compose.db.yml exec -T db psql -U postgres -d brhcdb -c "ALTER USER fluentbit WITH PASSWORD '${FLUENTBIT_PGSQL_PASSWORD}';"

# .pgpassファイルを作成
echo "5. .pgpassファイルを作成中..."
cat > .pgpass << PGPASS_EOF
localhost:5432:brhcdb:fluentbit:${FLUENTBIT_PGSQL_PASSWORD}
PGPASS_EOF
chmod 600 .pgpass

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
