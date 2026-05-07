#!/bin/sh
set -eu

# 引数解析
SKIP_DOWNLOAD=false
for arg in "$@"; do
  case "$arg" in
    --skip-download)
      SKIP_DOWNLOAD=true
      ;;
    -h|--help)
      cat << EOF
Usage: $0 [OPTIONS]

既存環境のアップグレードスクリプト

Options:
  --skip-download   docker-compose.yml / docker-compose.db.yml /
                    fluentd/container-logs.yaml / brhcli のダウンロードと
                    上書きをスキップする (ローカルでのテスト用)
  -h, --help        このヘルプを表示
EOF
      exit 0
      ;;
    *)
      echo "エラー: 不明な引数: $arg" >&2
      echo "ヘルプは $0 --help を参照してください" >&2
      exit 1
      ;;
  esac
done

echo "===== 既存環境のアップグレードを開始します ====="
echo ""

# .envファイルの存在確認
if [ ! -f ".env" ]; then
  echo "エラー: .env ファイルが見つかりません"
  echo "このスクリプトは既存の環境向けです。新規セットアップの場合は setup.sh を実行してください。"
  exit 1
fi

# CPU architectureを検出
CPU_ARCH="amd64"
if [ "$(uname -m)" = "arm64" ] || [ "$(uname -m)" = "aarch64" ]; then
  CPU_ARCH="arm64"
fi

if [ "$SKIP_DOWNLOAD" = "true" ]; then
  echo "1. ファイルのダウンロードをスキップしました (--skip-download)"
else
  echo "1. 最新のファイルをダウンロード中..."

  # docker-compose.ymlをダウンロード
  echo "   - docker-compose.yml"
  curl -O https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/docker-compose.yml

  # docker-compose.db.ymlをダウンロード
  echo "   - docker-compose.db.yml"
  curl -O https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/docker-compose.db.yml

  # fluentd/container-logs.yamlをダウンロード
  echo "   - fluentd/container-logs.yaml"
  mkdir -p fluentd
  curl -o fluentd/container-logs.yaml https://raw.githubusercontent.com/hantabaru1014/baru-reso-headless-controller/refs/heads/main/fluentd/container-logs.yaml

  # container pull
  docker compose pull
  docker compose -f docker-compose.db.yml pull

  # brhcliをダウンロード
  echo "   - brhcli"
  curl -o "brhcli" -L https://github.com/hantabaru1014/baru-reso-headless-controller/releases/latest/download/brhcli-${CPU_ARCH}
  chmod a+x brhcli
fi

echo "2. データベースのマイグレーション状態を確認中..."

docker compose -f docker-compose.db.yml up -d db

# container_logsテーブルが存在するかチェック
CONTAINER_LOGS_EXISTS=$(docker compose -f docker-compose.db.yml exec -T db psql -U postgres -d brhcdb -t -c "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'container_logs');" | tr -d '[:space:]')

if [ "$CONTAINER_LOGS_EXISTS" = "t" ]; then
  echo "   container_logsテーブルは既に存在します"
  NEEDS_CONTAINER_LOGS_SETUP=false
else
  echo "   container_logsテーブルが存在しません。新規セットアップが必要です"
  NEEDS_CONTAINER_LOGS_SETUP=true
fi

echo "3. データベースマイグレーションを実行中..."
./brhcli migrate

# container_logsテーブルが新規作成された場合のみ、fluentbit関連のセットアップを実行
if [ "$NEEDS_CONTAINER_LOGS_SETUP" = "true" ]; then
  echo "4. fluentbitユーザーのパスワードを設定中..."
  FLUENTBIT_PGSQL_PASSWORD="$(openssl rand -base64 32)"
  docker compose -f docker-compose.db.yml exec -T db psql -U postgres -d brhcdb -c "ALTER USER fluentbit WITH PASSWORD '${FLUENTBIT_PGSQL_PASSWORD}';"
  echo "FLUENTBIT_PGPASSWORD=\"${FLUENTBIT_PGSQL_PASSWORD}\"" >> .env
  echo "   FLUENTBIT_PGPASSWORDを.envに追加しました"
else
  echo "4. container_logsテーブルは既に存在するため、fluentbit関連のセットアップをスキップします"

  # FLUENTBIT_PGPASSWORDが.envに存在しない場合は警告
  if ! grep -q "^FLUENTBIT_PGPASSWORD=" .env 2>/dev/null; then
    echo "   ⚠️  警告: FLUENTBIT_PGPASSWORDが.envに見つかりません"
    echo "   fluentdがPostgreSQLに接続できない可能性があります"
    echo "   手動で.envにFLUENTBIT_PGPASSWORDを追加してください"
  fi
fi

# fluentdコンテナの再起動
echo "5. fluentdコンテナを再起動中..."
if docker compose -f docker-compose.db.yml ps fluentd 2>/dev/null | grep -q "Up"; then
  docker compose -f docker-compose.db.yml restart fluentd
  echo "   fluentdコンテナを再起動しました"
else
  echo "   fluentdコンテナが停止しているため、起動します"
  docker compose -f docker-compose.db.yml up -d fluentd
fi

# RustFS関連の.env項目を追加 (未設定の場合のみ)
echo "6. RustFS関連の設定を確認中..."
NEEDS_RUSTFS_SETUP=false
if ! grep -q "^RUSTFS_ACCESS_KEY=" .env 2>/dev/null; then
  RUSTFS_ACCESS_KEY="$(openssl rand -hex 16)"
  RUSTFS_SECRET_KEY="$(openssl rand -base64 32)"
  cat >> .env << EOF

# RustFS (S3互換ストレージ)
RUSTFS_ACCESS_KEY="${RUSTFS_ACCESS_KEY}"
RUSTFS_SECRET_KEY="${RUSTFS_SECRET_KEY}"
RUSTFS_ENDPOINT="localhost:9000"
RUSTFS_USE_SSL=false
WORLD_DOWNLOADS_BUCKET_NAME="world-downloads"
BLOB_TTL_DAYS=3
EOF
  echo "   RustFS関連の設定を.envに追加しました"
  NEEDS_RUSTFS_SETUP=true
else
  echo "   RustFS関連の設定は既に存在します"
fi

# rustfsコンテナの起動
echo "7. rustfsコンテナを起動中..."
if docker compose -f docker-compose.db.yml ps rustfs 2>/dev/null | grep -q "Up"; then
  echo "   rustfsコンテナは既に起動しています"
else
  docker compose -f docker-compose.db.yml up -d rustfs
  echo "   rustfsコンテナを起動しました"
fi

echo ""
echo "✅ アップグレードが完了しました！"
echo ""
echo "実行された処理:"
echo "- 最新のdocker-compose.yml、brhcli等をダウンロード"
echo "- データベースマイグレーションを実行"
if [ "$NEEDS_CONTAINER_LOGS_SETUP" = "true" ]; then
  echo "- fluentbitユーザーのパスワードを設定し、FLUENTBIT_PGPASSWORDを.envに追加"
fi
echo "- fluentdコンテナを再起動"
if [ "$NEEDS_RUSTFS_SETUP" = "true" ]; then
  echo "- RustFS関連の設定を.envに追加"
fi
echo "- rustfsコンテナを起動"
echo ""
