#!/bin/sh
set -eu

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

# brhcliをダウンロード
echo "   - brhcli"
curl -o "brhcli" -L https://github.com/hantabaru1014/baru-reso-headless-controller/releases/latest/download/brhcli-${CPU_ARCH}
chmod a+x brhcli

echo "2. データベースのマイグレーション状態を確認中..."

# container_logsテーブルが存在するかチェック
POSTGRES_PASSWORD=$(grep "^POSTGRES_PASSWORD=" .env | cut -d '=' -f2- | tr -d '"')
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

  echo "5. .pgpassファイルを作成中..."
  cat > .pgpass << PGPASS_EOF
localhost:5432:brhcdb:fluentbit:${FLUENTBIT_PGSQL_PASSWORD}
PGPASS_EOF
  chmod 600 .pgpass
  echo "   .pgpassファイルを作成しました"
else
  echo "4. container_logsテーブルは既に存在するため、fluentbit関連のセットアップをスキップします"

  # .pgpassが存在しない場合は警告
  if [ ! -f ".pgpass" ]; then
    echo "   ⚠️  警告: .pgpassファイルが見つかりません"
    echo "   既存環境で.pgpassファイルが削除されている可能性があります"
    echo "   手動で作成するか、setup.shを参考にしてください"
  fi
fi

# fluentdコンテナの再起動
echo "6. fluentdコンテナを再起動中..."
if docker compose ps fluentd 2>/dev/null | grep -q "Up"; then
  docker compose restart fluentd
  echo "   fluentdコンテナを再起動しました"
elif docker compose ps fluentd 2>/dev/null | grep -q "fluentd"; then
  echo "   fluentdコンテナが停止しているため、起動します"
  docker compose up -d fluentd
else
  echo "   fluentdコンテナが存在しないため、スキップします"
fi

echo ""
echo "✅ アップグレードが完了しました！"
echo ""
echo "実行された処理:"
echo "- 最新のdocker-compose.yml、brhcli等をダウンロード"
echo "- データベースマイグレーションを実行"
if [ "$NEEDS_CONTAINER_LOGS_SETUP" = "true" ]; then
  echo "- .pgpassファイルを作成"
  echo "- fluentbitユーザーのパスワードを設定"
fi
echo "- fluentdコンテナを再起動"
echo ""
