
BUF_VERSION := v1.54.0
BIN_DIR := $(shell pwd)/bin

# tools
buf := go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
wire := go run github.com/google/wire/cmd/wire@v0.6.0
sqlc := go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest
mockgen := go run go.uber.org/mock/mockgen@latest

.PHONY: install.tools
install.tools:
	mkdir -p $(BIN_DIR);
	@GOBIN=$(BIN_DIR) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION);
	@GOBIN=$(BIN_DIR) go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest;

.PHONY: gen.proto
gen.proto:
	$(buf) generate

.PHONY: gen.wire
gen.wire:
	$(wire) ./app

.PHONY: gen.sqlc
gen.sqlc:
	# https://github.com/sqlc-dev/sqlc/issues/3916
	CGO_CFLAGS="-DHAVE_STRCHRNUL" $(sqlc) generate

.PHONY: lint.proto
lint.proto:
	$(buf) format -w
	$(buf) lint

.PHONY: build.cli
build.cli:
	go build -o ./bin/brhcli cmd/cli/main.go

.PHONY: build.docker
build.docker:
	docker build -t ghcr.io/hantabaru1014/baru-reso-headless-controller .

.PHONY: exec.psql
exec.psql:
	docker compose -f docker-compose.db.yml exec -it db psql -U postgres -d brhcdb

.PHONY: gen.mock
gen.mock:
	@echo "Cleaning up old mock files..."
	@rm -rf adapter/hostconnector/mock
	@rm -rf lib/skyfrost/mock
	@echo "Generating new mock files..."
	@mkdir -p adapter/hostconnector/mock
	@mkdir -p lib/skyfrost/mock
	$(mockgen) -source=adapter/hostconnector/host_connector.go -destination=adapter/hostconnector/mock/mock_host_connector.go -package=mock
	$(mockgen) -package=mock -destination=adapter/hostconnector/mock/mock_rpc_client.go github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1 HeadlessControlServiceClient
	$(mockgen) -source=lib/skyfrost/client.go -destination=lib/skyfrost/mock/mock_client.go -package=mock
	@echo "Mock generation complete!"

.PHONY: migrate.up
migrate.up:
	@$(BIN_DIR)/migrate -path db/migrations -database "$(DB_URL)" up
	@DB_NAME=$$(echo "$(DB_URL)" | sed 's/.*\/\([^?]*\).*/\1/'); \
	TEST_DB_NAME=$${DB_NAME}_test; \
	TEST_DB_URL=$$(echo "$(DB_URL)" | sed "s/$$DB_NAME/$$TEST_DB_NAME/"); \
	$(BIN_DIR)/migrate -path db/migrations -database "$$TEST_DB_URL" up

.PHONY: test.setup
test.setup:
	@echo "Creating test database..."
	@DB_NAME=$$(echo "$(DB_URL)" | sed 's/.*\/\([^?]*\).*/\1/'); \
	TEST_DB_NAME=$${DB_NAME}_test; \
	TEST_DB_URL=$$(echo "$(DB_URL)" | sed "s/$$DB_NAME/$$TEST_DB_NAME/"); \
	echo "Test DB URL: $$TEST_DB_URL"; \
	echo "Checking if test database exists..."; \
	docker compose -f docker-compose.db.yml exec -T db psql -U postgres -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$$TEST_DB_NAME'" | grep -q 1 || \
	(echo "Creating test database $$TEST_DB_NAME..." && \
	docker compose -f docker-compose.db.yml exec -T db psql -U postgres -d postgres -c "CREATE DATABASE $$TEST_DB_NAME"); \
	echo "Running migrations..."; \
	$(BIN_DIR)/migrate -path db/migrations -database "$$TEST_DB_URL" up

.PHONY: test
test:
	go test -v ./...
