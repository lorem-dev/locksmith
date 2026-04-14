.PHONY: build build-plugins build-all lint test test-coverage test-race test-integration proto clean

build:
	go build -o bin/locksmith ./cmd/locksmith

build-plugins:
	go build -o bin/locksmith-plugin-keychain ./plugins/keychain
	go build -o bin/locksmith-plugin-gopass ./plugins/gopass

build-all: build build-plugins

lint:
	golangci-lint run ./...

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

proto:
	protoc \
		--go_out=gen/proto --go_opt=paths=source_relative \
		--go-grpc_out=gen/proto --go-grpc_opt=paths=source_relative \
		proto/vault/v1/vault.proto \
		proto/locksmith/v1/locksmith.proto

clean:
	rm -rf bin/ .reports/
