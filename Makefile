
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

.PHONY: gen.proto
gen.proto:
	$(buf) generate

.PHONY: gen.wire
gen.wire:
	$(wire) ./app

.PHONY: gen.sqlc
gen.sqlc:
	$(sqlc) generate

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
