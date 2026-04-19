.PHONY: build build-plugins build-all lint test test-coverage test-race test-integration proto install-tools init clean

# Tool versions - bump here to upgrade everywhere
BUF_VERSION ?= v1.68.1
PROTOC_GEN_GO_VERSION ?= v1.36.11
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.1
GOLANGCI_LINT_VERSION ?= v1.64.8
TEST_TIMEOUT ?= 3m
export TEST_TIMEOUT

# Resolve GOBIN so tools installed via 'go install' are always found,
# even when $GOBIN / $GOPATH/bin is not in $PATH.
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

# Prepend GOBIN to PATH for all make recipes so buf can find
# protoc-gen-go / protoc-gen-go-grpc plugin binaries.
export PATH := $(GOBIN):$(PATH)

# First-time setup: install tools, download dependencies, and generate protobuf code.
# Run this once after cloning the repository.
init: install-tools
	go install ./cmd/locksmith-pinentry
	go work sync
	mkdir -p gen/proto
	$(GOBIN)/buf generate
	$(GOBIN)/buf lint
	@echo "Ready. Run 'make build-all' to compile."

build:
	go build -o bin/locksmith ./cmd/locksmith
	go build -o bin/locksmith-pinentry ./cmd/locksmith-pinentry

build-plugins:
	go run ./.scripts/build-plugins

build-all: build build-plugins

lint: install-tools
	$(GOBIN)/golangci-lint run ./...
	$(GOBIN)/buf lint

# Run unit tests across all workspace modules.
test:
	./.scripts/test.sh

# Run with race detector across all workspace modules.
test-race:
	./.scripts/test-race.sh

# Run with coverage report across all workspace modules.
# Saves per-module HTML reports to .reports/ and prints a summary table.
test-coverage:
	./.scripts/test-coverage.sh

# Run integration tests (require daemon + plugins)
test-integration:
	go test -tags=integration -v ./...

# Regenerate protobuf Go code and verify linting.
# Installs pinned tool versions into GOPATH/bin on each run (no-op if already at correct version).
proto: install-tools
	mkdir -p gen/proto
	$(GOBIN)/buf generate
	$(GOBIN)/buf lint

# Install all code-generation and lint tools at pinned versions.
# Safe to run repeatedly - go install is idempotent for the same version.
install-tools:
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

clean:
	rm -rf bin/ .reports/
