
BUF_VERSION := v1.35.1
BIN_DIR := $(shell pwd)/bin

# tools
buf := go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
wire := go run github.com/google/wire/cmd/wire@v0.6.0
sqlc := go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest

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
