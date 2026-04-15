.PHONY: build build-plugins build-all lint test test-coverage test-race test-integration proto install-tools clean

# Tool versions — bump here to upgrade everywhere
BUF_VERSION ?= v1.68.1
PROTOC_GEN_GO_VERSION ?= v1.36.11
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.1
GOLANGCI_LINT_VERSION ?= v1.64.8

build:
	go build -o bin/locksmith ./cmd/locksmith

build-plugins:
	go build -o bin/locksmith-plugin-keychain ./plugins/keychain
	go build -o bin/locksmith-plugin-gopass ./plugins/gopass

build-all: build build-plugins

lint: install-tools
	golangci-lint run ./...
	buf lint

# Run unit tests across all workspace modules.
# Uses .scripts/workspace-test which auto-discovers modules from go.work.
test:
	go run ./.scripts/workspace-test test

# Run with race detector across all workspace modules
test-race:
	go run ./.scripts/workspace-test race

# Run with coverage report across all workspace modules.
# Saves per-module HTML reports to .reports/ and prints a summary table.
test-coverage:
	go run ./.scripts/workspace-test coverage

# Run integration tests (require daemon + plugins)
test-integration:
	go test -tags=integration -v ./...

# Regenerate protobuf Go code and verify linting.
# Installs pinned tool versions into GOPATH/bin on each run (no-op if already at correct version).
proto: install-tools
	mkdir -p gen/proto
	buf generate
	buf lint

# Install all code-generation and lint tools at pinned versions.
# Safe to run repeatedly — go install is idempotent for the same version.
install-tools:
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

clean:
	rm -rf bin/ .reports/
