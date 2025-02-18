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
curl -O https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/docker-compose.headless-sample.yml
curl -o "brhcli" -L https://github.com/hantabaru1014/baru-reso-headless-controller/releases/latest/download/brhcli-${CPU_ARCH}
chmod a+x brhcli

JWT_SECRET="$(openssl rand -base64 32)"
POSTGRES_PASSWORD="$(openssl rand -base64 32)"
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
  ;;
esac

read -p 'DB_URL を入力 (default: postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/brhcdb?sslmode=disable): ' DB_URL
DB_URL=${DB_URL:-"postgres://postgres:$(echo $POSTGRES_PASSWORD | jq -Rr @uri)@localhost:5432/brhcdb?sslmode=disable"}

cat > .env << EOF
JWT_SECRET="${JWT_SECRET}"
HEADLESS_IMAGE_NAME="${HEADLESS_IMAGE_NAME}"
HEADLESS_REGISTRY_AUTH="${HEADLESS_REGISTRY_AUTH}"
DOCKER_GID="${DOCKER_GID}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD}"
DB_URL="${DB_URL}"
HOST=":8014"
EOF
