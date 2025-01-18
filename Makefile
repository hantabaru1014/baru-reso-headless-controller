
BUF_VERSION := v1.35.1
BIN_DIR := $(shell pwd)/bin

# tools
buf := go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
wire := go run github.com/google/wire/cmd/wire@v0.6.0

.PHONY: install.tools
install.tools:
	mkdir -p $(BIN_DIR);
	@GOBIN=$(BIN_DIR) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION);

.PHONY: lint.proto
lint.proto:
	$(buf) format -w
	$(buf) lint

.PHONY: build.proto
build.proto:
	$(buf) generate

.PHONY: build.wire
build.wire:
	$(wire) ./app
