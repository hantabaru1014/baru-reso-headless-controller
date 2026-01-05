.PHONY: gen.proto
gen.proto:
	go tool buf generate

.PHONY: gen.wire
gen.wire:
	go tool wire ./app

.PHONY: gen.sqlc
gen.sqlc:
	# https://github.com/sqlc-dev/sqlc/issues/3916
	CGO_CFLAGS="-DHAVE_STRCHRNUL" go tool sqlc generate

.PHONY: lint.proto
lint.proto:
	go tool buf format -w
	go tool buf lint

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
	go tool mockgen -source=adapter/hostconnector/host_connector.go -destination=adapter/hostconnector/mock/mock_host_connector.go -package=mock
	go tool mockgen -package=mock -destination=adapter/hostconnector/mock/mock_rpc_client.go github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1 HeadlessControlServiceClient
	go tool mockgen -source=lib/skyfrost/client.go -destination=lib/skyfrost/mock/mock_client.go -package=mock
	@echo "Mock generation complete!"

.PHONY: migrate.up
migrate.up:
	@go tool migrate -path db/migrations -database "$(DB_URL)" up
	@DB_NAME=$$(echo "$(DB_URL)" | sed 's/.*\/\([^?]*\).*/\1/'); \
	TEST_DB_NAME=$${DB_NAME}_test; \
	TEST_DB_URL=$$(echo "$(DB_URL)" | sed "s/$$DB_NAME/$$TEST_DB_NAME/"); \
	go tool migrate -path db/migrations -database "$$TEST_DB_URL" up

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
	go tool migrate -path db/migrations -database "$$TEST_DB_URL" up

.PHONY: test
test:
	go tool gotestsum --format dots -- ./...

.PHONY: lint
lint:
	go tool golangci-lint run ./... --fix
