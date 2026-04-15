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

lint:
	golangci-lint run ./...
	buf lint

# Run unit tests across all modules
test:
	go test ./...
	cd sdk && go test ./...
	cd plugins/gopass && go test ./...
	cd plugins/keychain && go test ./...

# Run with race detector
test-race:
	go test -race ./...
	cd sdk && go test -race ./...
	cd plugins/gopass && go test -race ./...
	cd plugins/keychain && go test -race ./...

# Run with coverage report — saves HTML and text reports to .reports/
# Sub-modules are checked separately to catch gaps in sdk/ and plugins/
test-coverage:
	mkdir -p .reports
	go test -coverprofile=.reports/coverage.out -covermode=atomic ./...
	go tool cover -func=.reports/coverage.out
	go tool cover -html=.reports/coverage.out -o .reports/coverage.html
	cd sdk && go test -coverprofile=../.reports/coverage-sdk.out -covermode=atomic ./...
	go tool cover -func=.reports/coverage-sdk.out
	go tool cover -html=.reports/coverage-sdk.out -o .reports/coverage-sdk.html
	cd plugins/gopass && go test -coverprofile=../../.reports/coverage-gopass.out -covermode=atomic ./...
	go tool cover -func=.reports/coverage-gopass.out
	go tool cover -html=.reports/coverage-gopass.out -o .reports/coverage-gopass.html
	cd plugins/keychain && go test -coverprofile=../../.reports/coverage-keychain.out -covermode=atomic ./...
	go tool cover -func=.reports/coverage-keychain.out
	go tool cover -html=.reports/coverage-keychain.out -o .reports/coverage-keychain.html

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
