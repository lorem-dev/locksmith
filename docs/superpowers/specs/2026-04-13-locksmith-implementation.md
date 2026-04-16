# Locksmith Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a secure middleware daemon + CLI that gives AI agents access to secrets stored in vault providers (macOS Keychain, gopass) via a plugin architecture, with per-session caching and agent-framework integration.

**Architecture:** A Go monorepo (workspace) with a central daemon communicating with vault plugins via hashicorp/go-plugin (gRPC). CLI is a thin gRPC client to the daemon over a Unix socket. Sessions are token-based with configurable TTL. An `init` command auto-detects agents and installs instructions/permissions.

**Tech Stack:** Go 1.23+, hashicorp/go-plugin, gRPC/protobuf, cobra, charmbracelet/huh, zerolog, golangci-lint

## Coding Standards

- **Test coverage ≥ 90%** for every package. Enforced via `make test-coverage`.
- **Race detector** must pass: `make test-race`.
- **Code comments**: all exported functions and types must have godoc comments. Complex unexported logic must have inline comments.
- **Logging**: use zerolog throughout. Never use `fmt.Print*` for operational output in daemon/plugin code — only in CLI user-facing output.
- **English only** in all code, comments, commit messages, and documentation.
- **Conventional Commits** format (see CONTRIBUTING.md). No mentions of AI tools in commit messages.

---

## File Map

```
locksmith/
├── .gitignore
├── .golangci.yml
├── CHANGES.md                          # changelog (Development + versioned sections)
├── CLAUDE.md                           # project rules for AI agents
├── CONTRIBUTING.md                     # commit conventions, coverage rules
├── Makefile                            # build, lint, test, coverage, race
├── README.md                           # project description, install, quickstart
├── go.mod                              # module github.com/lorem-dev/locksmith
├── go.sum
├── go.work
├── go.work.sum
├── integration_test.go                 # build-tagged integration tests
├── .claude/
│   └── skills/
│       └── changelog/
│           └── SKILL.md               # local skill: compress changes into CHANGES.md
├── .worktrees/                         # git worktrees (gitignored)
├── .reports/                           # coverage HTML/text reports (gitignored)
├── cmd/
│   └── locksmith/
│       └── main.go
├── docs/
│   ├── architecture.md                 # system design, component diagram
│   ├── configuration.md                # config file reference
│   ├── plugins.md                      # how to write a vault plugin
│   └── plans/                          # superpowers plans (gitignored)
├── gen/
│   └── proto/                          # generated protobuf Go code (gitignored)
│       ├── vault/v1/
│       └── locksmith/v1/
├── internal/
│   ├── config/
│   │   ├── config.go                   # Config struct, Load(), Validate(), ExpandPath()
│   │   └── config_test.go
│   ├── log/
│   │   ├── log.go                      # zerolog setup, global logger, Init()
│   │   └── log_test.go
│   ├── session/
│   │   ├── session.go
│   │   └── session_test.go
│   ├── plugin/
│   │   ├── manager.go
│   │   ├── manager_test.go
│   │   └── grpc_client.go
│   ├── daemon/
│   │   ├── server.go
│   │   ├── server_test.go
│   │   ├── daemon.go
│   │   └── daemon_test.go
│   ├── cli/
│   │   ├── root.go
│   │   ├── serve.go
│   │   ├── get.go
│   │   ├── session_cmd.go
│   │   ├── vault_cmd.go
│   │   ├── config_cmd.go
│   │   ├── init_cmd.go
│   │   └── client.go
│   └── initflow/
│       ├── detect.go
│       ├── detect_test.go
│       ├── flow.go
│       ├── flow_test.go
│       ├── agents.go
│       ├── agents_test.go
│       ├── sandbox.go
│       └── templates/
│           ├── claude_skill.md.tmpl
│           ├── claude_md.md.tmpl
│           ├── codex_agents.md.tmpl
│           └── agent_instructions.md.tmpl
├── proto/
│   ├── vault/v1/vault.proto
│   └── locksmith/v1/locksmith.proto
├── sdk/
│   ├── go.mod                          # module github.com/lorem-dev/locksmith-sdk
│   ├── go.sum
│   ├── plugin.go
│   └── plugin_test.go
└── plugins/
    ├── gopass/
    │   ├── go.mod                      # module github.com/lorem-dev/locksmith-plugin-gopass
    │   ├── go.sum
    │   ├── main.go
    │   ├── provider.go
    │   └── provider_test.go
    └── keychain/
        ├── go.mod                      # module github.com/lorem-dev/locksmith-plugin-keychain
        ├── go.sum
        ├── main.go
        ├── provider_darwin.go          # CGo Security framework (build tag: darwin)
        ├── provider_stub.go            # non-darwin stub (build tag: !darwin)
        └── provider_test.go
```

---

### Task 0: Project Meta-files

**Files:**
- Create: `.gitignore`, `CLAUDE.md`, `CONTRIBUTING.md`, `CHANGES.md`

- [ ] **Step 1: Create .gitignore**

Write `.gitignore`:
```
bin/
gen/proto/
docs/plans/
.worktrees/
.reports/
*.sock
TODO.md
# Track local skills but nothing else under .claude/
.claude/*
!.claude/skills/
```

- [ ] **Step 2: Create CLAUDE.md**

Write `CLAUDE.md`:
```markdown
# Locksmith — Project Rules

## Repository: https://github.com/lorem-dev/locksmith

## Language
All code, comments, commit messages, and documentation must be in English.
When conversing with the user, always respond in the user's language.

## Module Names
- Main module: `github.com/lorem-dev/locksmith`
- SDK: `github.com/lorem-dev/locksmith-sdk`
- Gopass plugin: `github.com/lorem-dev/locksmith-plugin-gopass`
- Keychain plugin: `github.com/lorem-dev/locksmith-plugin-keychain`

## Project Structure
- `cmd/locksmith/` — CLI + daemon entry point
- `internal/` — daemon internals (not importable externally)
- `sdk/` — public SDK for vault plugin authors
- `plugins/` — default vault plugins (each is a standalone Go module)
- `proto/` — protobuf definitions
- `gen/proto/` — generated code (do not edit manually, gitignored)
- `docs/` — documentation (architecture, configuration, plugins)
- `docs/plans/` — superpowers implementation plans (gitignored, default plan location)
- `.worktrees/` — git worktrees (gitignored, default worktree location)

## Commits
Follow Conventional Commits (see CONTRIBUTING.md). No mentions of AI tools.

## Testing
- Coverage must be ≥ 90% per package
- All tests must pass under the race detector
- Run `make test-coverage` and `make test-race` before committing

## Logging
Use `internal/log` (zerolog). Never use `fmt.Print*` in daemon/plugin code.

## Documentation
All documentation lives in `docs/`. Keep `README.md` focused on install + quickstart.
Track changes in `CHANGES.md` — development changes go under `## Development`,
releases under `## Version vX.Y.Z`.

## Changelog Skill
Use the `changelog` skill (`.claude/skills/changelog/SKILL.md`) to compress changes before cutting a release.
```

- [ ] **Step 3: Create CONTRIBUTING.md**

Write `CONTRIBUTING.md`:
```markdown
# Contributing to Locksmith

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
Relates #<GITHUB-ISSUE>
```

**Types:** `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `ci`, `build`

**Rules:**
- Description in English, imperative mood ("add", not "added")
- No mentions of AI tools or agents in commit messages
- Reference issues with `Relates #123` in the footer when applicable
- Keep subject line under 72 characters
- Use `(scope)` sparingly — only reuse a scope that already exists in the git log.
  Before adding a scope, run `git log --oneline | grep "type(" | head -20` to check
  what scopes are established. Prefer no scope over inventing a new one.

**Examples:**
```
feat(session): add TTL-based expiry with memory wipe
fix(keychain): handle errSecUserCanceled from Touch ID prompt
chore: update golangci-lint to v1.57
```

## Test Coverage

- Minimum coverage per package: **90%**
- Run before submitting: `make test-coverage`
- Race detector must pass: `make test-race`
- Integration tests: `make test-integration`

## Code Style

- Follow `golangci-lint` rules defined in `.golangci.yml`
- All exported symbols must have godoc comments
- Complex unexported logic must have inline comments
```

- [ ] **Step 4: Create CHANGES.md**

Write `CHANGES.md`:
```markdown
# Changelog

## Development

- Initial project scaffold: Go workspace, module structure, Makefile
- Protobuf definitions for VaultProvider and LocksmithService
- Logging package using zerolog (stdout/JSON, configurable level)
- Configuration package with YAML loading, validation, path expansion
- Session manager with TTL, per-session secret cache, memory wipe on invalidation
- SDK package for vault plugin authors
- Plugin manager with process-based isolation via hashicorp/go-plugin
- Daemon gRPC server (LocksmithService) over Unix socket
- CLI commands: serve, get, session, vault, config, init
- gopass vault plugin
- macOS Keychain vault plugin with Touch ID (CGo, Security framework)
- `locksmith init` with TUI forms (charmbracelet/huh), agent auto-detection
- Agent integration: Claude Code, Codex
- Sandbox allowlist generation for agent restricted modes
- Documentation: README, architecture, configuration, plugins guide
```

- [ ] **Step 5: Commit**

```bash
git add .gitignore CLAUDE.md CONTRIBUTING.md CHANGES.md
git commit -m "chore: add project meta-files (CLAUDE.md, CONTRIBUTING.md, CHANGES.md)"
```

---

### Task 1: Project Scaffold and Go Workspace

**Files:**
- Create: `go.mod`, `go.work`, `Makefile`, `.golangci.yml`, `cmd/locksmith/main.go`
- Create: `sdk/go.mod`, `plugins/gopass/go.mod`, `plugins/keychain/go.mod`

- [ ] **Step 1: Initialize root Go module**

```bash
go mod init github.com/lorem-dev/locksmith
```

- [ ] **Step 2: Initialize sub-modules**

```bash
mkdir -p sdk && cd sdk && go mod init github.com/lorem-dev/locksmith-sdk && cd ..
mkdir -p plugins/gopass && cd plugins/gopass && go mod init github.com/lorem-dev/locksmith-plugin-gopass && cd ../..
mkdir -p plugins/keychain && cd plugins/keychain && go mod init github.com/lorem-dev/locksmith-plugin-keychain && cd ../..
```

- [ ] **Step 3: Create go.work**

Write `go.work`:
```go
go 1.23

use (
	.
	./sdk
	./plugins/keychain
	./plugins/gopass
)
```

- [ ] **Step 4: Create .golangci.yml**

Write `.golangci.yml`:
```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck
    - gofmt
    - goimports
    - misspell
    - unconvert
    - gocritic

linters-settings:
  gocritic:
    enabled-tags:
      - diagnostic
      - style
```

- [ ] **Step 5: Create Makefile**

Write `Makefile`:
```makefile
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
```

- [ ] **Step 6: Create minimal main.go**

Write `cmd/locksmith/main.go`:
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("locksmith")
	os.Exit(0)
}
```

- [ ] **Step 7: Verify build**

```bash
go build ./cmd/locksmith
```

Expected: builds without error.

- [ ] **Step 8: Commit**

```bash
git add go.work go.mod sdk/go.mod plugins/gopass/go.mod plugins/keychain/go.mod \
  Makefile .golangci.yml cmd/locksmith/main.go
git commit -m "chore: scaffold Go workspace with monorepo structure"
```

---

### Task 2: Protobuf Definitions and Code Generation

**Files:**
- Create: `proto/vault/v1/vault.proto`
- Create: `proto/locksmith/v1/locksmith.proto`
- Create: `gen/proto/vault/v1/*.pb.go` (generated)
- Create: `gen/proto/locksmith/v1/*.pb.go` (generated)

- [ ] **Step 1: Write vault.proto**

Write `proto/vault/v1/vault.proto`:
```protobuf
syntax = "proto3";

package vault.v1;

option go_package = "vault/v1;vaultv1";

// VaultProvider is the gRPC service that every vault plugin must implement.
service VaultProvider {
  rpc GetSecret(GetSecretRequest) returns (GetSecretResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Info(InfoRequest) returns (PluginInfo);
}

message GetSecretRequest {
  string path = 1;
  map<string, string> opts = 2;
}

message GetSecretResponse {
  bytes secret = 1;
  string content_type = 2;
}

message HealthCheckRequest {}

message HealthCheckResponse {
  bool available = 1;
  string message = 2;
}

message InfoRequest {}

message PluginInfo {
  string name = 1;
  string version = 2;
  repeated string platforms = 3;
}
```

- [ ] **Step 2: Write locksmith.proto**

Write `proto/locksmith/v1/locksmith.proto`:
```protobuf
syntax = "proto3";

package locksmith.v1;

option go_package = "locksmith/v1;locksmithv1";

// LocksmithService is the gRPC service exposed by the daemon over Unix socket.
// CLI is a thin client to this service.
service LocksmithService {
  rpc GetSecret(GetSecretRequest) returns (GetSecretResponse);
  rpc SessionStart(SessionStartRequest) returns (SessionStartResponse);
  rpc SessionEnd(SessionEndRequest) returns (SessionEndResponse);
  rpc SessionList(SessionListRequest) returns (SessionListResponse);
  rpc VaultList(VaultListRequest) returns (VaultListResponse);
  rpc VaultHealth(VaultHealthRequest) returns (VaultHealthResponse);
}

message GetSecretRequest {
  string session_id = 1;
  string key_alias = 2;
  string vault_name = 3;
  string path = 4;
}

message GetSecretResponse {
  bytes secret = 1;
  string content_type = 2;
}

message SessionStartRequest {
  string ttl = 1;
  repeated string allowed_keys = 2;
}

message SessionStartResponse {
  string session_id = 1;
  string expires_at = 2;
}

message SessionEndRequest {
  string session_id = 1;
}

message SessionEndResponse {}

message SessionListRequest {}

message SessionListResponse {
  repeated SessionInfo sessions = 1;
}

message SessionInfo {
  string session_id = 1;
  string created_at = 2;
  string expires_at = 3;
  repeated string allowed_keys = 4;
  int32 cached_secrets_count = 5;
}

message VaultListRequest {}

message VaultListResponse {
  repeated VaultInfo vaults = 1;
}

message VaultInfo {
  string name = 1;
  string type = 2;
  repeated string platforms = 3;
  string version = 4;
}

message VaultHealthRequest {}

message VaultHealthResponse {
  repeated VaultHealthInfo vaults = 1;
}

message VaultHealthInfo {
  string name = 1;
  bool available = 2;
  string message = 3;
}
```

- [ ] **Step 3: Generate Go code**

```bash
mkdir -p gen/proto/vault/v1 gen/proto/locksmith/v1
protoc \
  --go_out=gen/proto --go_opt=paths=source_relative \
  --go-grpc_out=gen/proto --go-grpc_opt=paths=source_relative \
  proto/vault/v1/vault.proto
protoc \
  --go_out=gen/proto --go_opt=paths=source_relative \
  --go-grpc_out=gen/proto --go-grpc_opt=paths=source_relative \
  proto/locksmith/v1/locksmith.proto
```

Expected: `.pb.go` and `_grpc.pb.go` files in `gen/proto/`.

- [ ] **Step 4: Add dependencies**

```bash
go get google.golang.org/grpc google.golang.org/protobuf
go mod tidy
```

- [ ] **Step 5: Verify generated code compiles**

```bash
go build ./gen/proto/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add proto/ go.mod go.sum
git commit -m "feat(proto): add VaultProvider and LocksmithService protobuf definitions"
```

---

### Task 3: Logging Package

**Files:**
- Create: `internal/log/log.go`
- Create: `internal/log/log_test.go`

- [ ] **Step 1: Write failing tests**

Write `internal/log/log_test.go`:
```go
package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/log"
)

func TestInit_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "text", Output: &buf})

	log.Info().Str("key", "val").Msg("hello")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Errorf("output %q missing message", out)
	}
}

func TestInit_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "json", Output: &buf})

	log.Info().Str("key", "val").Msg("hello")

	out := buf.String()
	if !strings.Contains(out, `"message":"hello"`) {
		t.Errorf("output %q is not JSON with message field", out)
	}
}

func TestInit_DebugLevelSuppressed(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "info", Format: "text", Output: &buf})

	log.Debug().Msg("should not appear")

	if buf.Len() > 0 {
		t.Errorf("debug message leaked at info level: %q", buf.String())
	}
}

func TestInit_DebugLevelVisible(t *testing.T) {
	var buf bytes.Buffer
	log.Init(log.Config{Level: "debug", Format: "text", Output: &buf})

	log.Debug().Msg("debug message")

	if !strings.Contains(buf.String(), "debug message") {
		t.Error("debug message not visible at debug level")
	}
}

func TestInit_InvalidLevel_DefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	// Should not panic
	log.Init(log.Config{Level: "bogus", Format: "text", Output: &buf})
	log.Info().Msg("ok")
	if buf.Len() == 0 {
		t.Error("expected output after init with invalid level")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/log/... -v
```

Expected: compilation error — package does not exist.

- [ ] **Step 3: Write implementation**

Write `internal/log/log.go`:
```go
// Package log provides a thin wrapper around zerolog for structured logging.
// Call Init() once at startup with the config from the YAML file.
// All other packages import this package and use the module-level functions
// (Info, Debug, Warn, Error) which delegate to the global logger.
package log

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Config holds logging configuration loaded from the YAML config file.
type Config struct {
	// Level is the minimum log level: "debug", "info", "warn", "error".
	Level string
	// Format is the output format: "text" (human-readable) or "json".
	Format string
	// Output overrides the writer (used in tests; defaults to os.Stdout).
	Output io.Writer
}

var logger zerolog.Logger

// Init configures the global logger. Must be called before any logging.
func Init(cfg Config) {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	var w io.Writer
	if cfg.Format == "json" {
		w = out
	} else {
		w = zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}
	}

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	logger = zerolog.New(w).Level(level).With().Timestamp().Logger()
}

// Debug returns a debug-level log event.
func Debug() *zerolog.Event { return logger.Debug() }

// Info returns an info-level log event.
func Info() *zerolog.Event { return logger.Info() }

// Warn returns a warn-level log event.
func Warn() *zerolog.Event { return logger.Warn() }

// Error returns an error-level log event.
func Error() *zerolog.Event { return logger.Error() }

// With returns the logger with additional context fields.
func With() zerolog.Context { return logger.With() }
```

- [ ] **Step 4: Add zerolog dependency**

```bash
go get github.com/rs/zerolog
go mod tidy
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/log/... -v
```

Expected: all 5 tests PASS.

- [ ] **Step 6: Run with race detector**

```bash
go test -race ./internal/log/...
```

Expected: PASS, no race conditions.

- [ ] **Step 7: Commit**

```bash
git add internal/log/ go.mod go.sum
git commit -m "feat(log): add zerolog-based logging package with text/JSON format support"
```

---

### Task 4: Configuration Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Write `internal/config/config_test.go`:
```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/config"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
defaults:
  session_ttl: 3h
  socket_path: /tmp/locksmith.sock

logging:
  level: debug
  format: json

vaults:
  keychain:
    type: keychain
  my-gopass:
    type: gopass
    store: personal

keys:
  github-token:
    vault: keychain
    path: "github-api-token"
  anthropic-key:
    vault: my-gopass
    path: "dev/anthropic"
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Defaults.SessionTTL != "3h" {
		t.Errorf("SessionTTL = %q, want %q", cfg.Defaults.SessionTTL, "3h")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "json")
	}
	if len(cfg.Vaults) != 2 {
		t.Fatalf("len(Vaults) = %d, want 2", len(cfg.Vaults))
	}
	if cfg.Vaults["my-gopass"].Store != "personal" {
		t.Errorf("gopass store = %q, want %q", cfg.Vaults["my-gopass"].Store, "personal")
	}
	if len(cfg.Keys) != 2 {
		t.Fatalf("len(Keys) = %d, want 2", len(cfg.Keys))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("Load() expected error for missing file")
	}
}

func TestValidate_MissingVaultRef(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "3h", SocketPath: "/tmp/test.sock"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"bad-key": {Vault: "nonexistent", Path: "foo"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for key referencing nonexistent vault")
	}
}

func TestValidate_Defaults(t *testing.T) {
	cfg := &config.Config{
		Vaults: map[string]config.Vault{"kc": {Type: "keychain"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if cfg.Defaults.SessionTTL != "3h" {
		t.Errorf("default SessionTTL = %q, want %q", cfg.Defaults.SessionTTL, "3h")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("default Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("default Logging.Format = %q, want %q", cfg.Logging.Format, "text")
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := config.ExpandPath("~/.config/locksmith/locksmith.sock")
	expected := filepath.Join(home, ".config", "locksmith", "locksmith.sock")
	if result != expected {
		t.Errorf("ExpandPath() = %q, want %q", result, expected)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write implementation**

Write `internal/config/config.go`:
```go
// Package config loads and validates the locksmith YAML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level structure of the locksmith config file.
type Config struct {
	Defaults Defaults         `yaml:"defaults"`
	Logging  Logging          `yaml:"logging"`
	Vaults   map[string]Vault `yaml:"vaults"`
	Keys     map[string]Key   `yaml:"keys"`
}

// Defaults holds daemon-level defaults.
type Defaults struct {
	SessionTTL string `yaml:"session_ttl"`
	SocketPath string `yaml:"socket_path"`
}

// Logging holds zerolog configuration.
type Logging struct {
	// Level is the minimum log level: "debug", "info", "warn", "error".
	Level string `yaml:"level"`
	// Format is "text" (human-readable) or "json".
	Format string `yaml:"format"`
}

// Vault represents a configured vault backend.
type Vault struct {
	Type    string `yaml:"type"`
	Store   string `yaml:"store,omitempty"`
	Account string `yaml:"account,omitempty"`
}

// Key is a named alias pointing to a secret in a specific vault.
type Key struct {
	Vault string `yaml:"vault"`
	Path  string `yaml:"path"`
}

// Load reads and validates the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate applies defaults and checks for configuration errors.
func (c *Config) Validate() error {
	if c.Defaults.SessionTTL == "" {
		c.Defaults.SessionTTL = "3h"
	}
	if c.Defaults.SocketPath == "" {
		c.Defaults.SocketPath = ExpandPath("~/.config/locksmith/locksmith.sock")
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}

	for name, key := range c.Keys {
		if _, ok := c.Vaults[key.Vault]; !ok {
			return fmt.Errorf("key %q references unknown vault %q", name, key.Vault)
		}
		if key.Path == "" {
			return fmt.Errorf("key %q has empty path", name)
		}
	}

	return nil
}

// DefaultConfigPath returns the default config file path (~/.config/locksmith/config.yaml).
func DefaultConfigPath() string {
	return ExpandPath("~/.config/locksmith/config.yaml")
}

// ExpandPath replaces a leading ~ with the user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
```

- [ ] **Step 4: Add yaml dependency**

```bash
go get gopkg.in/yaml.v3
go mod tidy
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/config/... -v
go test -race ./internal/config/...
```

Expected: all 4 tests PASS, race detector clean.

- [ ] **Step 6: Check coverage**

```bash
mkdir -p .reports
go test -coverprofile=.reports/coverage.out ./internal/config/...
go tool cover -func=.reports/coverage.out | grep total
```

Expected: total coverage ≥ 90%.

- [ ] **Step 7: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat(config): add YAML config loader with logging settings and path expansion"
```

---

### Task 5: Session Manager

**Files:**
- Create: `internal/session/session.go`
- Create: `internal/session/session_test.go`

- [ ] **Step 1: Write failing tests**

Write `internal/session/session_test.go`:
```go
package session_test

import (
	"testing"
	"time"

	"github.com/lorem-dev/locksmith/internal/session"
)

func TestStore_Create(t *testing.T) {
	s := session.NewStore()
	sess, err := s.Create(3*time.Hour, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if len(sess.ID) != 67 { // "ls_" + 64 hex chars
		t.Errorf("ID length = %d, want 67", len(sess.ID))
	}
	if !sess.ExpiresAt.After(time.Now().Add(2*time.Hour + 59*time.Minute)) {
		t.Error("ExpiresAt too early")
	}
}

func TestStore_Create_WithAllowedKeys(t *testing.T) {
	s := session.NewStore()
	sess, err := s.Create(time.Hour, []string{"key1", "key2"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if len(sess.AllowedKeys) != 2 {
		t.Fatalf("AllowedKeys len = %d, want 2", len(sess.AllowedKeys))
	}
}

func TestStore_Get(t *testing.T) {
	s := session.NewStore()
	created, _ := s.Create(time.Hour, nil)
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := session.NewStore()
	_, err := s.Get("ls_nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for nonexistent session")
	}
}

func TestStore_Get_Expired(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(1*time.Millisecond, nil)
	time.Sleep(5 * time.Millisecond)
	_, err := s.Get(sess.ID)
	if err == nil {
		t.Fatal("Get() expected error for expired session")
	}
}

func TestStore_Delete(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, nil)
	if err := s.Delete(sess.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err := s.Get(sess.ID)
	if err == nil {
		t.Fatal("Get() expected error after Delete()")
	}
}

func TestStore_CacheSecret(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, nil)
	s.CacheSecret(sess.ID, "github-token", []byte("ghp_secret123"))
	val, ok := s.GetCachedSecret(sess.ID, "github-token")
	if !ok {
		t.Fatal("GetCachedSecret() returned not ok")
	}
	if string(val) != "ghp_secret123" {
		t.Errorf("cached secret = %q, want %q", string(val), "ghp_secret123")
	}
}

func TestStore_CacheSecret_NotAllowed(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, []string{"allowed-key"})
	_ = sess
	_, ok := s.GetCachedSecret(sess.ID, "forbidden-key")
	if ok {
		t.Fatal("GetCachedSecret() should return not ok for non-allowed key")
	}
}

func TestStore_Delete_WipesSecrets(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, nil)
	secret := []byte("sensitive-data")
	s.CacheSecret(sess.ID, "key", secret)
	s.Delete(sess.ID)
	for i, b := range secret {
		if b != 0 {
			t.Errorf("secret byte[%d] = %d, want 0 (not wiped)", i, b)
		}
	}
}

func TestStore_List(t *testing.T) {
	s := session.NewStore()
	s.Create(time.Hour, nil)
	s.Create(time.Hour, []string{"k1"})
	list := s.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
}

func TestStore_Cleanup(t *testing.T) {
	s := session.NewStore()
	s.Create(1*time.Millisecond, nil)
	s.Create(time.Hour, nil)
	time.Sleep(5 * time.Millisecond)
	removed := s.Cleanup()
	if removed != 1 {
		t.Errorf("Cleanup() removed %d, want 1", removed)
	}
	if len(s.List()) != 1 {
		t.Errorf("List() after cleanup = %d, want 1", len(s.List()))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/session/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write implementation**

Write `internal/session/session.go`:
```go
// Package session manages agent sessions: creation, TTL-based expiry,
// per-session secret caching, and secure memory wipe on invalidation.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Session represents an active agent session.
type Session struct {
	ID          string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	AllowedKeys []string // empty means all keys are allowed
	secrets     map[string][]byte
}

// Store is a thread-safe in-memory store for active sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewStore creates a new empty session store.
func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

// Create allocates a new session with the given TTL and optional key allowlist.
func (s *Store) Create(ttl time.Duration, allowedKeys []string) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generating session ID: %w", err)
	}
	now := time.Now()
	sess := &Session{
		ID:          id,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		AllowedKeys: allowedKeys,
		secrets:     make(map[string][]byte),
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return sess, nil
}

// Get returns the session for id, or an error if not found or expired.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if time.Now().After(sess.ExpiresAt) {
		s.Delete(id) //nolint:errcheck
		return nil, fmt.Errorf("session %q expired", id)
	}
	return sess, nil
}

// Delete invalidates a session and wipes all cached secrets from memory.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	for key, secret := range sess.secrets {
		wipeBytes(secret)
		delete(sess.secrets, key)
	}
	delete(s.sessions, id)
	return nil
}

// CacheSecret stores a secret value bound to the given session and key name.
func (s *Store) CacheSecret(sessionID, keyName string, secret []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	sess.secrets[keyName] = secret
}

// GetCachedSecret retrieves a cached secret. Returns (nil, false) if the key
// is not cached, the session does not exist, or the key is not in the allowlist.
func (s *Store) GetCachedSecret(sessionID, keyName string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if !sess.isKeyAllowed(keyName) {
		return nil, false
	}
	val, ok := sess.secrets[keyName]
	return val, ok
}

// List returns a snapshot of all active sessions (without cached secrets).
func (s *Store) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, Session{
			ID:          sess.ID,
			CreatedAt:   sess.CreatedAt,
			ExpiresAt:   sess.ExpiresAt,
			AllowedKeys: sess.AllowedKeys,
		})
	}
	return result
}

// Cleanup removes expired sessions and returns the count removed.
func (s *Store) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	now := time.Now()
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			for key, secret := range sess.secrets {
				wipeBytes(secret)
				delete(sess.secrets, key)
			}
			delete(s.sessions, id)
			removed++
		}
	}
	return removed
}

func (sess *Session) isKeyAllowed(keyName string) bool {
	if len(sess.AllowedKeys) == 0 {
		return true
	}
	for _, k := range sess.AllowedKeys {
		if k == keyName {
			return true
		}
	}
	return false
}

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ls_" + hex.EncodeToString(b), nil
}

// wipeBytes zeroes a byte slice to prevent secrets from lingering in memory.
func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/session/... -v
go test -race ./internal/session/...
```

Expected: all 10 tests PASS, race detector clean.

- [ ] **Step 5: Check coverage**

```bash
mkdir -p .reports
go test -coverprofile=.reports/coverage.out ./internal/session/...
go tool cover -func=.reports/coverage.out | grep total
```

Expected: ≥ 90%.

- [ ] **Step 6: Commit**

```bash
git add internal/session/
git commit -m "feat(session): add session manager with TTL, secret caching, and memory wipe"
```

---

### Task 6: SDK — Plugin Helper Library

**Files:**
- Create: `sdk/plugin.go`
- Create: `sdk/plugin_test.go`

- [ ] **Step 1: Write failing test**

Write `sdk/plugin_test.go`:
```go
package sdk_test

import (
	"context"
	"testing"

	sdk "github.com/lorem-dev/locksmith-sdk"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

type mockProvider struct{}

func (m *mockProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return &vaultv1.GetSecretResponse{Secret: []byte("test-secret"), ContentType: "text/plain"}, nil
}

func (m *mockProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "ok"}, nil
}

func (m *mockProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{Name: "mock", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}

func TestGRPCServer_GetSecret(t *testing.T) {
	server := sdk.NewGRPCServer(&mockProvider{})
	if server == nil {
		t.Fatal("NewGRPCServer() returned nil")
	}
	resp, err := server.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

func TestGRPCServer_HealthCheck(t *testing.T) {
	server := sdk.NewGRPCServer(&mockProvider{})
	resp, err := server.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

func TestGRPCServer_Info(t *testing.T) {
	server := sdk.NewGRPCServer(&mockProvider{})
	resp, err := server.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if resp.Name != "mock" {
		t.Errorf("Name = %q, want %q", resp.Name, "mock")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd sdk && go test ./... -v
```

Expected: compilation error.

- [ ] **Step 3: Write implementation**

Write `sdk/plugin.go`:
```go
// Package sdk provides helpers for building locksmith vault plugins.
// Plugin authors should implement the Provider interface and call Serve()
// from their main function.
package sdk

import (
	"context"
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// Provider is the interface that vault plugin binaries must implement.
// Each method is called by the daemon over gRPC.
type Provider interface {
	GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error)
	HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error)
	Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error)
}

// Handshake is the go-plugin handshake config. Both the daemon and every plugin
// binary must use the same values for the connection to be established.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "LOCKSMITH_PLUGIN",
	MagicCookieValue: "vault-provider",
}

// PluginMap is the map passed to go-plugin. Key is the plugin type name.
var PluginMap = map[string]goplugin.Plugin{
	"vault": &VaultGRPCPlugin{},
}

// Serve starts the plugin gRPC server. Call this from the plugin's main().
func Serve(provider Provider) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]goplugin.Plugin{"vault": &VaultGRPCPlugin{Impl: provider}},
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
}

// NewClientConfig returns a go-plugin ClientConfig for a given plugin binary path.
func NewClientConfig(binaryPath string) *goplugin.ClientConfig {
	return &goplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          PluginMap,
		Cmd:              exec.Command(binaryPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	}
}

// VaultGRPCPlugin implements goplugin.GRPCPlugin.
type VaultGRPCPlugin struct {
	goplugin.Plugin
	Impl Provider
}

func (p *VaultGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	vaultv1.RegisterVaultProviderServer(s, NewGRPCServer(p.Impl))
	return nil
}

func (p *VaultGRPCPlugin) GRPCClient(ctx context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: vaultv1.NewVaultProviderClient(c)}, nil
}

// GRPCServer is the server-side adapter that bridges go-plugin gRPC to Provider.
type GRPCServer struct {
	vaultv1.UnimplementedVaultProviderServer
	impl Provider
}

// NewGRPCServer wraps a Provider into a VaultProvider gRPC server.
func NewGRPCServer(impl Provider) *GRPCServer {
	return &GRPCServer{impl: impl}
}

func (s *GRPCServer) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return s.impl.GetSecret(ctx, req)
}

func (s *GRPCServer) HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return s.impl.HealthCheck(ctx, req)
}

func (s *GRPCServer) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return s.impl.Info(ctx, req)
}

// GRPCClient is the client-side adapter used by the daemon's plugin manager.
type GRPCClient struct {
	client vaultv1.VaultProviderClient
}

func (c *GRPCClient) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return c.client.GetSecret(ctx, req)
}

func (c *GRPCClient) HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, req)
}

func (c *GRPCClient) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return c.client.Info(ctx, req)
}
```

- [ ] **Step 4: Add dependencies to sdk/go.mod**

```bash
cd sdk
go get github.com/hashicorp/go-plugin google.golang.org/grpc google.golang.org/protobuf
# The replace directive below is for standalone builds outside the workspace.
# In go.work mode the workspace handles cross-module resolution automatically,
# so this replace is redundant but harmless when go.work is present.
go mod edit -replace github.com/lorem-dev/locksmith=../
go mod tidy
```

- [ ] **Step 5: Run tests**

```bash
cd sdk
go test ./... -v
go test -race ./...
```

Expected: all 3 tests PASS, race detector clean.

- [ ] **Step 6: Commit**

```bash
git add sdk/
git commit -m "feat(sdk): add plugin helper library with gRPC server/client adapters"
```

---

### Task 7: Plugin Manager

**Files:**
- Create: `internal/plugin/manager.go`
- Create: `internal/plugin/manager_test.go`
- Create: `internal/plugin/grpc_client.go`

- [ ] **Step 1: Write failing tests**

Write `internal/plugin/manager_test.go`:
```go
package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/plugin"
)

func TestDiscover_FindsPluginInDir(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "locksmith-plugin-test")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	found := plugin.Discover([]string{dir})
	if len(found) != 1 {
		t.Fatalf("Discover() found %d plugins, want 1", len(found))
	}
	if found["test"] != fakeBin {
		t.Errorf("plugin path = %q, want %q", found["test"], fakeBin)
	}
}

func TestDiscover_IgnoresNonPlugin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "other-binary"), []byte("#!/bin/sh\n"), 0o755)

	found := plugin.Discover([]string{dir})
	if len(found) != 0 {
		t.Fatalf("Discover() found %d plugins, want 0", len(found))
	}
}

func TestDiscover_IgnoresNonExecutable(t *testing.T) {
	dir := t.TempDir()
	// Write without execute bit
	os.WriteFile(filepath.Join(dir, "locksmith-plugin-nope"), []byte("#!/bin/sh\n"), 0o644)

	found := plugin.Discover([]string{dir})
	if len(found) != 0 {
		t.Fatalf("Discover() should ignore non-executable, found %d", len(found))
	}
}

func TestDiscover_MultiplePlugins(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"locksmith-plugin-keychain", "locksmith-plugin-gopass"} {
		os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755)
	}

	found := plugin.Discover([]string{dir})
	if len(found) != 2 {
		t.Fatalf("Discover() found %d plugins, want 2", len(found))
	}
	if _, ok := found["keychain"]; !ok {
		t.Error("missing keychain plugin")
	}
	if _, ok := found["gopass"]; !ok {
		t.Error("missing gopass plugin")
	}
}

func TestDiscover_FirstWins(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	bin1 := filepath.Join(dir1, "locksmith-plugin-test")
	bin2 := filepath.Join(dir2, "locksmith-plugin-test")
	os.WriteFile(bin1, []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(bin2, []byte("#!/bin/sh\n"), 0o755)

	found := plugin.Discover([]string{dir1, dir2})
	if found["test"] != bin1 {
		t.Errorf("first dir should win, got %q", found["test"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/plugin/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write implementation**

Write `internal/plugin/manager.go`:
```go
// Package plugin manages the lifecycle of vault provider plugin processes.
// Plugins are discovered on disk, launched as child processes, and communicated
// with over gRPC using the hashicorp/go-plugin framework.
package plugin

import (
	"fmt"
	"os"
	"strings"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
	sdk "github.com/lorem-dev/locksmith-sdk"
	"github.com/lorem-dev/locksmith/internal/log"
)

const pluginPrefix = "locksmith-plugin-"

// Manager owns the set of running vault plugin processes.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*runningPlugin
}

type runningPlugin struct {
	client   *goplugin.Client
	provider sdk.Provider
}

// NewManager creates a new, empty plugin manager.
func NewManager() *Manager {
	return &Manager{plugins: make(map[string]*runningPlugin)}
}

// Discover searches the given directories for locksmith-plugin-* executables.
// Returns a map of vault type name → binary path. First match wins.
func Discover(searchDirs []string) map[string]string {
	found := make(map[string]string)
	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), pluginPrefix) {
				continue
			}
			vaultType := strings.TrimPrefix(entry.Name(), pluginPrefix)
			fullPath := fmt.Sprintf("%s/%s", dir, entry.Name())

			info, err := entry.Info()
			if err != nil || info.Mode()&0o111 == 0 {
				continue // skip non-executable
			}
			if _, exists := found[vaultType]; !exists {
				found[vaultType] = fullPath
				log.Debug().Str("vault", vaultType).Str("path", fullPath).Msg("discovered plugin")
			}
		}
	}
	return found
}

// DefaultSearchDirs returns the standard plugin lookup paths.
func DefaultSearchDirs() []string {
	dirs := []string{}
	if execPath, err := os.Executable(); err == nil {
		dirs = append(dirs, execPath[:strings.LastIndex(execPath, "/")])
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, home+"/.config/locksmith/plugins")
	}
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		dirs = append(dirs, strings.Split(pathEnv, ":")...)
	}
	return dirs
}

// Launch starts a vault plugin process for the given vault type.
func (m *Manager) Launch(vaultType, binaryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[vaultType]; exists {
		return fmt.Errorf("plugin %q already running", vaultType)
	}

	client := goplugin.NewClient(sdk.NewClientConfig(binaryPath))

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("connecting to plugin %q: %w", vaultType, err)
	}

	raw, err := rpcClient.Dispense("vault")
	if err != nil {
		client.Kill()
		return fmt.Errorf("dispensing plugin %q: %w", vaultType, err)
	}

	provider, ok := raw.(sdk.Provider)
	if !ok {
		client.Kill()
		return fmt.Errorf("plugin %q does not implement Provider", vaultType)
	}

	m.plugins[vaultType] = &runningPlugin{client: client, provider: provider}
	log.Info().Str("vault", vaultType).Str("binary", binaryPath).Msg("plugin launched")
	return nil
}

// Get returns the Provider for a vault type, or an error if not loaded.
func (m *Manager) Get(vaultType string) (sdk.Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, ok := m.plugins[vaultType]
	if !ok {
		return nil, fmt.Errorf("no plugin loaded for vault type %q", vaultType)
	}
	return rp.provider, nil
}

// Types returns the list of loaded vault type names.
func (m *Manager) Types() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	types := make([]string, 0, len(m.plugins))
	for t := range m.plugins {
		types = append(types, t)
	}
	return types
}

// Kill stops all running plugin processes.
func (m *Manager) Kill() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, rp := range m.plugins {
		rp.client.Kill()
		delete(m.plugins, name)
		log.Debug().Str("vault", name).Msg("plugin killed")
	}
}
```

Write `internal/plugin/grpc_client.go`:
```go
package plugin

// grpc_client.go — the daemon calls Manager.Get(vaultType) which returns an
// sdk.Provider backed by sdk.GRPCClient. No additional wrapper is needed here;
// this file documents the design decision.
```

- [ ] **Step 4: Add hashicorp/go-plugin**

```bash
go get github.com/hashicorp/go-plugin
go mod tidy
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/plugin/... -v
go test -race ./internal/plugin/...
```

Expected: all 5 discovery tests PASS, race detector clean.

- [ ] **Step 6: Commit**

```bash
git add internal/plugin/ go.mod go.sum
git commit -m "feat(plugin): add plugin manager with discovery and go-plugin integration"
```

---

### Task 8: Daemon — LocksmithService gRPC Server

**Files:**
- Create: `internal/daemon/server.go`, `internal/daemon/server_test.go`
- Create: `internal/daemon/daemon.go`, `internal/daemon/daemon_test.go`

- [ ] **Step 1: Write failing tests for LocksmithService**

Write `internal/daemon/server_test.go`:
```go
package daemon_test

import (
	"context"
	"testing"
	"time"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/session"
	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

func newTestServer() *daemon.Server {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"test-key": {Vault: "keychain", Path: "test-path"}},
	}
	return daemon.NewServer(cfg, session.NewStore(), nil)
}

func TestSessionStart(t *testing.T) {
	srv := newTestServer()
	resp, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{Ttl: "2h"})
	if err != nil {
		t.Fatalf("SessionStart() error: %v", err)
	}
	if resp.SessionId == "" {
		t.Error("SessionId is empty")
	}
	if resp.ExpiresAt == "" {
		t.Error("ExpiresAt is empty")
	}
}

func TestSessionStart_DefaultTTL(t *testing.T) {
	srv := newTestServer()
	resp, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	if err != nil {
		t.Fatalf("SessionStart() error: %v", err)
	}
	parsed, _ := time.Parse(time.RFC3339, resp.ExpiresAt)
	if time.Until(parsed) < 59*time.Minute {
		t.Error("default TTL not applied correctly")
	}
}

func TestSessionEnd(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.SessionEnd(context.Background(), &locksmithv1.SessionEndRequest{SessionId: startResp.SessionId})
	if err != nil {
		t.Fatalf("SessionEnd() error: %v", err)
	}
	listResp, _ := srv.SessionList(context.Background(), &locksmithv1.SessionListRequest{})
	if len(listResp.Sessions) != 0 {
		t.Errorf("sessions after end = %d, want 0", len(listResp.Sessions))
	}
}

func TestSessionList(t *testing.T) {
	srv := newTestServer()
	srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	resp, err := srv.SessionList(context.Background(), &locksmithv1.SessionListRequest{})
	if err != nil {
		t.Fatalf("SessionList() error: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("sessions = %d, want 2", len(resp.Sessions))
	}
}

func TestGetSecret_NoSession(t *testing.T) {
	srv := newTestServer()
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: "ls_nonexistent",
		KeyAlias:  "test-key",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error with invalid session")
	}
}

func TestGetSecret_UnknownAlias(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "nonexistent-alias",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for unknown key alias")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/daemon/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write server.go**

Write `internal/daemon/server.go`:
```go
// Package daemon implements the LocksmithService gRPC server and the Unix socket
// daemon lifecycle (Start, Stop, signal handling, session cleanup).
package daemon

import (
	"context"
	"fmt"
	"time"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	"github.com/lorem-dev/locksmith/internal/session"
)

// Server is the gRPC implementation of LocksmithService.
type Server struct {
	locksmithv1.UnimplementedLocksmithServiceServer
	cfg     *config.Config
	store   *session.Store
	plugins *pluginpkg.Manager
}

// NewServer creates a LocksmithService server backed by the given store and plugin manager.
func NewServer(cfg *config.Config, store *session.Store, plugins *pluginpkg.Manager) *Server {
	return &Server{cfg: cfg, store: store, plugins: plugins}
}

// SessionStart creates a new agent session with the requested TTL and key restrictions.
func (s *Server) SessionStart(_ context.Context, req *locksmithv1.SessionStartRequest) (*locksmithv1.SessionStartResponse, error) {
	ttl, err := parseTTL(req.Ttl, s.cfg.Defaults.SessionTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid TTL: %w", err)
	}
	sess, err := s.store.Create(ttl, req.AllowedKeys)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	log.Info().Str("session", sess.ID).Dur("ttl", ttl).Msg("session started")
	return &locksmithv1.SessionStartResponse{
		SessionId: sess.ID,
		ExpiresAt: sess.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// SessionEnd invalidates a session and wipes its cached secrets.
func (s *Server) SessionEnd(_ context.Context, req *locksmithv1.SessionEndRequest) (*locksmithv1.SessionEndResponse, error) {
	if err := s.store.Delete(req.SessionId); err != nil {
		return nil, err
	}
	log.Info().Str("session", req.SessionId).Msg("session ended")
	return &locksmithv1.SessionEndResponse{}, nil
}

// SessionList returns metadata for all active sessions.
func (s *Server) SessionList(_ context.Context, _ *locksmithv1.SessionListRequest) (*locksmithv1.SessionListResponse, error) {
	sessions := s.store.List()
	infos := make([]*locksmithv1.SessionInfo, len(sessions))
	for i, sess := range sessions {
		infos[i] = &locksmithv1.SessionInfo{
			SessionId:   sess.ID,
			CreatedAt:   sess.CreatedAt.Format(time.RFC3339),
			ExpiresAt:   sess.ExpiresAt.Format(time.RFC3339),
			AllowedKeys: sess.AllowedKeys,
		}
	}
	return &locksmithv1.SessionListResponse{Sessions: infos}, nil
}

// GetSecret retrieves a secret from the appropriate vault plugin, serving from
// the session cache when possible to avoid repeated vault authorization prompts.
func (s *Server) GetSecret(ctx context.Context, req *locksmithv1.GetSecretRequest) (*locksmithv1.GetSecretResponse, error) {
	if _, err := s.store.Get(req.SessionId); err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	vaultType, path, opts, err := s.resolveKey(req)
	if err != nil {
		return nil, err
	}

	cacheKey := vaultType + ":" + path
	if cached, ok := s.store.GetCachedSecret(req.SessionId, cacheKey); ok {
		log.Debug().Str("session", req.SessionId).Str("key", cacheKey).Msg("serving secret from cache")
		return &locksmithv1.GetSecretResponse{Secret: cached, ContentType: "text/plain"}, nil
	}

	if s.plugins == nil {
		return nil, fmt.Errorf("no plugin manager available")
	}
	provider, err := s.plugins.Get(vaultType)
	if err != nil {
		return nil, fmt.Errorf("vault plugin: %w", err)
	}

	resp, err := provider.GetSecret(ctx, &vaultv1.GetSecretRequest{Path: path, Opts: opts})
	if err != nil {
		return nil, fmt.Errorf("fetching secret from vault: %w", err)
	}

	s.store.CacheSecret(req.SessionId, cacheKey, resp.Secret)
	log.Info().Str("session", req.SessionId).Str("vault", vaultType).Str("path", path).Msg("secret retrieved and cached")

	return &locksmithv1.GetSecretResponse{Secret: resp.Secret, ContentType: resp.ContentType}, nil
}

// VaultList returns info for all loaded vault plugins.
func (s *Server) VaultList(_ context.Context, _ *locksmithv1.VaultListRequest) (*locksmithv1.VaultListResponse, error) {
	if s.plugins == nil {
		return &locksmithv1.VaultListResponse{}, nil
	}
	var vaults []*locksmithv1.VaultInfo
	for _, vaultType := range s.plugins.Types() {
		info := &locksmithv1.VaultInfo{Name: vaultType, Type: vaultType}
		if provider, err := s.plugins.Get(vaultType); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if pi, err := provider.Info(ctx, &vaultv1.InfoRequest{}); err == nil {
				info.Version = pi.Version
				info.Platforms = pi.Platforms
			}
			cancel()
		}
		vaults = append(vaults, info)
	}
	return &locksmithv1.VaultListResponse{Vaults: vaults}, nil
}

// VaultHealth returns availability status for all loaded vault plugins.
func (s *Server) VaultHealth(_ context.Context, _ *locksmithv1.VaultHealthRequest) (*locksmithv1.VaultHealthResponse, error) {
	if s.plugins == nil {
		return &locksmithv1.VaultHealthResponse{}, nil
	}
	var results []*locksmithv1.VaultHealthInfo
	for _, vaultType := range s.plugins.Types() {
		result := &locksmithv1.VaultHealthInfo{Name: vaultType}
		if provider, err := s.plugins.Get(vaultType); err != nil {
			result.Message = err.Error()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if h, err := provider.HealthCheck(ctx, &vaultv1.HealthCheckRequest{}); err != nil {
				result.Message = err.Error()
			} else {
				result.Available = h.Available
				result.Message = h.Message
			}
			cancel()
		}
		results = append(results, result)
	}
	return &locksmithv1.VaultHealthResponse{Vaults: results}, nil
}

// resolveKey returns vault type, secret path, and extra opts for a GetSecret request.
// Supports both key alias lookup and direct vault+path fallback.
func (s *Server) resolveKey(req *locksmithv1.GetSecretRequest) (vaultType, path string, opts map[string]string, err error) {
	opts = make(map[string]string)
	if req.KeyAlias != "" {
		keyDef, ok := s.cfg.Keys[req.KeyAlias]
		if !ok {
			return "", "", nil, fmt.Errorf("unknown key alias %q", req.KeyAlias)
		}
		vaultDef, ok := s.cfg.Vaults[keyDef.Vault]
		if !ok {
			return "", "", nil, fmt.Errorf("key %q references unknown vault %q", req.KeyAlias, keyDef.Vault)
		}
		if vaultDef.Store != "" {
			opts["store"] = vaultDef.Store
		}
		return vaultDef.Type, keyDef.Path, opts, nil
	}
	if req.VaultName == "" || req.Path == "" {
		return "", "", nil, fmt.Errorf("either key_alias or both vault_name and path are required")
	}
	if vaultDef, ok := s.cfg.Vaults[req.VaultName]; ok {
		if vaultDef.Store != "" {
			opts["store"] = vaultDef.Store
		}
		return vaultDef.Type, req.Path, opts, nil
	}
	return req.VaultName, req.Path, opts, nil
}

func parseTTL(requested, defaultTTL string) (time.Duration, error) {
	ttlStr := requested
	if ttlStr == "" {
		ttlStr = defaultTTL
	}
	return time.ParseDuration(ttlStr)
}
```

- [ ] **Step 4: Write daemon.go**

Write `internal/daemon/daemon.go`:
```go
package daemon

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/grpc"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	"github.com/lorem-dev/locksmith/internal/session"
)

// Daemon manages the daemon lifecycle: plugin loading, gRPC server, session cleanup.
type Daemon struct {
	cfg        *config.Config
	store      *session.Store
	plugins    *pluginpkg.Manager
	grpcServer *grpc.Server
	listener   net.Listener
}

// New creates a Daemon from the given config.
func New(cfg *config.Config) *Daemon {
	return &Daemon{
		cfg:     cfg,
		store:   session.NewStore(),
		plugins: pluginpkg.NewManager(),
	}
}

// Start initialises the Unix socket, loads vault plugins, and begins serving gRPC.
// Blocks until Stop() is called or an error occurs.
func (d *Daemon) Start() error {
	socketPath := config.ExpandPath(d.cfg.Defaults.SocketPath)

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stale socket: %w", err)
	}

	if err := d.loadPlugins(); err != nil {
		return fmt.Errorf("loading plugins: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", socketPath, err)
	}
	d.listener = listener

	if err := os.Chmod(socketPath, 0o600); err != nil {
		listener.Close()
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	d.grpcServer = grpc.NewServer()
	locksmithv1.RegisterLocksmithServiceServer(d.grpcServer, NewServer(d.cfg, d.store, d.plugins))

	go d.cleanupLoop()

	log.Info().Str("socket", socketPath).Msg("locksmith daemon listening")
	return d.grpcServer.Serve(listener)
}

// Stop gracefully shuts down the gRPC server and kills plugin processes.
func (d *Daemon) Stop() {
	if d.grpcServer != nil {
		d.grpcServer.GracefulStop()
	}
	if d.listener != nil {
		d.listener.Close()
	}
	d.plugins.Kill()
	log.Info().Msg("locksmith daemon stopped")
}

// WaitForShutdown blocks until SIGINT or SIGTERM, then calls Stop().
func (d *Daemon) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info().Msg("received shutdown signal")
	d.Stop()
}

func (d *Daemon) loadPlugins() error {
	discovered := pluginpkg.Discover(pluginpkg.DefaultSearchDirs())
	for name, vault := range d.cfg.Vaults {
		binaryPath, ok := discovered[vault.Type]
		if !ok {
			log.Warn().Str("vault", name).Str("type", vault.Type).Msg("no plugin binary found for vault type")
			continue
		}
		if err := d.plugins.Launch(vault.Type, binaryPath); err != nil {
			return fmt.Errorf("launching plugin %q: %w", vault.Type, err)
		}
	}
	return nil
}

func (d *Daemon) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if removed := d.store.Cleanup(); removed > 0 {
			log.Info().Int("removed", removed).Msg("expired sessions cleaned up")
		}
	}
}
```

- [ ] **Step 5: Write daemon_test.go**

Write `internal/daemon/daemon_test.go`:
```go
package daemon_test

import (
	"testing"
	"time"
)

func TestParseTTL_Explicit(t *testing.T) {
	d, err := parseTTL("2h", "3h")
	if err != nil {
		t.Fatalf("parseTTL() error: %v", err)
	}
	if d != 2*time.Hour {
		t.Errorf("duration = %v, want 2h", d)
	}
}

func TestParseTTL_Default(t *testing.T) {
	d, err := parseTTL("", "3h")
	if err != nil {
		t.Fatalf("parseTTL() error: %v", err)
	}
	if d != 3*time.Hour {
		t.Errorf("duration = %v, want 3h", d)
	}
}

func TestParseTTL_Invalid(t *testing.T) {
	_, err := parseTTL("notaduration", "3h")
	if err == nil {
		t.Fatal("parseTTL() expected error for invalid input")
	}
}
```

Note: `parseTTL` is unexported, so it must be exported or the test must live in `package daemon` (not `daemon_test`). Change the package declaration to `package daemon` and remove the `_test` suffix to access unexported helpers, or export `ParseTTL` in `server.go`. Prefer `package daemon` black-box tests for the lifecycle (Start/Stop) via the integration test in Task 15 — add the above as `package daemon` (internal) tests.

- [ ] **Step 6: Run tests**

```bash
go test ./internal/daemon/... -v
go test -race ./internal/daemon/...
```

Expected: all server tests + daemon unit tests PASS, race detector clean.

- [ ] **Step 7: Commit**

```bash
git add internal/daemon/ go.mod go.sum
git commit -m "feat(daemon): add LocksmithService gRPC server with Unix socket lifecycle"
```

---

### Task 9: CLI Commands

**Files:**
- Create: `internal/cli/root.go`, `internal/cli/client.go`
- Create: `internal/cli/serve.go`, `internal/cli/get.go`
- Create: `internal/cli/session_cmd.go`, `internal/cli/vault_cmd.go`, `internal/cli/config_cmd.go`
- Modify: `cmd/locksmith/main.go`

- [ ] **Step 1: Write client.go**

Write `internal/cli/client.go`:
```go
// Package cli implements all locksmith subcommands via cobra.
package cli

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lorem-dev/locksmith/internal/config"
	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// dialDaemon connects to the locksmith daemon Unix socket and returns a client.
// Returns an error with a helpful hint if the daemon is not running.
func dialDaemon(socketPath string) (locksmithv1.LocksmithServiceClient, *grpc.ClientConn, error) {
	if socketPath == "" {
		socketPath = config.ExpandPath("~/.config/locksmith/locksmith.sock")
	}
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot connect to locksmith daemon at %s: %w\nHint: run 'locksmith serve' first",
			socketPath, err,
		)
	}
	return locksmithv1.NewLocksmithServiceClient(conn), conn, nil
}
```

- [ ] **Step 2: Write root.go**

Write `internal/cli/root.go`:
```go
package cli

import "github.com/spf13/cobra"

var cfgFile string

// NewRootCmd builds the cobra root command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "locksmith",
		Short: "Secure secret middleware for AI agents",
		Long:  "Locksmith gives AI agents secure access to secrets from vault providers (macOS Keychain, gopass, etc.) with per-session caching and Touch ID support.",
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/locksmith/config.yaml)")
	root.AddCommand(
		newServeCmd(),
		newGetCmd(),
		newSessionCmd(),
		newVaultCmd(),
		newConfigCmd(),
		newInitCmd(),
	)
	return root
}
```

- [ ] **Step 3: Write serve.go**

Write `internal/cli/serve.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/log"
)

// Note: `locksmith serve --daemon` (background/detach mode) is deferred to a future task.
// For now, use `locksmith serve &` in the shell to run it in the background.
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the locksmith daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := cfgFile
			if cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			log.Init(log.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format})

			d := daemon.New(cfg)
			go d.WaitForShutdown()

			if err := d.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
```

- [ ] **Step 4: Write get.go**

Write `internal/cli/get.go`:
```go
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

func newGetCmd() *cobra.Command {
	var keyAlias, vaultName, path string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Retrieve a secret from a vault provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := os.Getenv("LOCKSMITH_SESSION")
			if sessionID == "" {
				return fmt.Errorf("LOCKSMITH_SESSION not set — run 'locksmith session start' first")
			}

			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resp, err := client.GetSecret(ctx, &locksmithv1.GetSecretRequest{
				SessionId: sessionID,
				KeyAlias:  keyAlias,
				VaultName: vaultName,
				Path:      path,
			})
			if err != nil {
				return fmt.Errorf("getting secret: %w", err)
			}

			fmt.Print(string(resp.Secret))
			return nil
		},
	}

	cmd.Flags().StringVar(&keyAlias, "key", "", "key alias from config")
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault name (fallback, requires --path)")
	cmd.Flags().StringVar(&path, "path", "", "secret path in vault (fallback, requires --vault)")
	return cmd
}
```

- [ ] **Step 5: Write session_cmd.go**

Write `internal/cli/session_cmd.go`:
```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Manage agent sessions"}
	cmd.AddCommand(newSessionStartCmd(), newSessionEndCmd(), newSessionListCmd())
	return cmd
}

func newSessionStartCmd() *cobra.Command {
	var ttl string
	var keys []string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{
				Ttl: ttl, AllowedKeys: keys,
			})
			if err != nil {
				return fmt.Errorf("starting session: %w", err)
			}
			out, _ := json.MarshalIndent(map[string]string{
				"session_id": resp.SessionId,
				"expires_at": resp.ExpiresAt,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&ttl, "ttl", "", "session TTL (default: from config)")
	cmd.Flags().StringSliceVar(&keys, "keys", nil, "restrict to specific key aliases")
	return cmd
}

func newSessionEndCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "end",
		Short: "End an agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				sessionID = os.Getenv("LOCKSMITH_SESSION")
			}
			if sessionID == "" {
				return fmt.Errorf("session ID required: use --session or set LOCKSMITH_SESSION")
			}
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := client.SessionEnd(ctx, &locksmithv1.SessionEndRequest{SessionId: sessionID}); err != nil {
				return fmt.Errorf("ending session: %w", err)
			}
			fmt.Println("session ended")
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "session ID (default: $LOCKSMITH_SESSION)")
	return cmd
}

func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := client.SessionList(ctx, &locksmithv1.SessionListRequest{})
			if err != nil {
				return fmt.Errorf("listing sessions: %w", err)
			}
			if len(resp.Sessions) == 0 {
				fmt.Println("no active sessions")
				return nil
			}
			out, _ := json.MarshalIndent(resp.Sessions, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}
```

- [ ] **Step 6: Write vault_cmd.go and config_cmd.go**

Write `internal/cli/vault_cmd.go`:
```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "vault", Short: "Manage vault providers"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List available vault providers",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, conn, err := dialDaemon("")
				if err != nil {
					return err
				}
				defer conn.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				resp, err := client.VaultList(ctx, &locksmithv1.VaultListRequest{})
				if err != nil {
					return fmt.Errorf("listing vaults: %w", err)
				}
				if len(resp.Vaults) == 0 {
					fmt.Println("no vault providers loaded")
					return nil
				}
				out, _ := json.MarshalIndent(resp.Vaults, "", "  ")
				fmt.Println(string(out))
				return nil
			},
		},
		&cobra.Command{
			Use:   "health",
			Short: "Check health of vault providers",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, conn, err := dialDaemon("")
				if err != nil {
					return err
				}
				defer conn.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				resp, err := client.VaultHealth(ctx, &locksmithv1.VaultHealthRequest{})
				if err != nil {
					return fmt.Errorf("checking vault health: %w", err)
				}
				for _, v := range resp.Vaults {
					status := "UNAVAILABLE"
					if v.Available {
						status = "OK"
					}
					fmt.Printf("%-20s %s  %s\n", v.Name, status, v.Message)
				}
				return nil
			},
		},
	)
	return cmd
}
```

Write `internal/cli/config_cmd.go`:
```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration management"}
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Validate config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := cfgFile
			if cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
			fmt.Printf("config OK: %s\n  vaults: %d\n  keys:   %d\n  ttl:    %s\n",
				cfgPath, len(cfg.Vaults), len(cfg.Keys), cfg.Defaults.SessionTTL)
			return nil
		},
	})
	return cmd
}
```

- [ ] **Step 7: Update main.go**

Write `cmd/locksmith/main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 8: Add cobra dependency**

```bash
go get github.com/spf13/cobra
go mod tidy
```

- [ ] **Step 9: Build and verify**

```bash
go build -o bin/locksmith ./cmd/locksmith
./bin/locksmith --help
./bin/locksmith session --help
./bin/locksmith get --help
```

Expected: all commands listed with correct flags.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/ cmd/locksmith/ go.mod go.sum
git commit -m "feat(cli): add serve, get, session, vault, config subcommands"
```

---

### Task 10: Gopass Plugin

**Files:**
- Create: `plugins/gopass/main.go`, `plugins/gopass/provider.go`, `plugins/gopass/provider_test.go`

- [ ] **Step 1: Write failing tests**

Write `plugins/gopass/provider_test.go`:
```go
package main

import (
	"context"
	"os/exec"
	"testing"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

func TestGopassProvider_Info(t *testing.T) {
	p := &GopassProvider{}
	info, err := p.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "gopass" {
		t.Errorf("Name = %q, want %q", info.Name, "gopass")
	}
	if len(info.Platforms) == 0 {
		t.Error("Platforms is empty")
	}
}

func TestGopassProvider_HealthCheck_NotInstalled(t *testing.T) {
	if _, err := exec.LookPath("gopass"); err == nil {
		t.Skip("gopass is installed — skipping not-installed test")
	}
	p := &GopassProvider{}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if resp.Available {
		t.Error("Available should be false when gopass is not installed")
	}
}

func TestGopassProvider_GetSecret_InvalidPath(t *testing.T) {
	if _, err := exec.LookPath("gopass"); err != nil {
		t.Skip("gopass not installed")
	}
	p := &GopassProvider{}
	_, err := p.GetSecret(context.Background(), &vaultv1.GetSecretRequest{
		Path: "locksmith-test/nonexistent-key-12345",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for nonexistent key")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd plugins/gopass && go test ./... -v
```

Expected: compilation error.

- [ ] **Step 3: Write provider.go**

Write `plugins/gopass/provider.go`:
```go
// Package main implements the locksmith gopass vault plugin.
// It shells out to the `gopass` CLI to retrieve secrets.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// GopassProvider retrieves secrets from a gopass password store.
type GopassProvider struct{}

// GetSecret fetches a secret from gopass by path. Optionally uses a named store
// via opts["store"]. Authorization (GPG passphrase / Touch ID) is handled by gopass.
func (p *GopassProvider) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	secretPath := req.Path
	if store, ok := req.Opts["store"]; ok && store != "" {
		secretPath = store + "/" + req.Path
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gopass", "show", "-o", secretPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gopass show %q: %s: %w", secretPath, strings.TrimSpace(stderr.String()), err)
	}

	return &vaultv1.GetSecretResponse{
		Secret:      bytes.TrimRight(stdout.Bytes(), "\n"),
		ContentType: "text/plain",
	}, nil
}

// HealthCheck verifies that gopass is installed and the store is initialized.
func (p *GopassProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	path, err := exec.LookPath("gopass")
	if err != nil {
		return &vaultv1.HealthCheckResponse{Available: false, Message: "gopass not found in PATH"}, nil
	}
	if err := exec.Command("gopass", "ls", "--flat").Run(); err != nil {
		return &vaultv1.HealthCheckResponse{
			Available: false,
			Message:   fmt.Sprintf("gopass at %s is not initialized: %v", path, err),
		}, nil
	}
	return &vaultv1.HealthCheckResponse{Available: true, Message: fmt.Sprintf("gopass available at %s", path)}, nil
}

// Info returns plugin metadata.
func (p *GopassProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{
		Name:      "gopass",
		Version:   "0.1.0",
		Platforms: []string{"darwin", "linux"},
	}, nil
}
```

Write `plugins/gopass/main.go`:
```go
package main

import sdk "github.com/lorem-dev/locksmith-sdk"

func main() { sdk.Serve(&GopassProvider{}) }
```

- [ ] **Step 4: Add dependencies**

```bash
cd plugins/gopass
go mod edit -replace github.com/lorem-dev/locksmith-sdk=../../sdk
go mod edit -replace github.com/lorem-dev/locksmith=../../
go get github.com/lorem-dev/locksmith-sdk
go mod tidy
```

- [ ] **Step 5: Run tests and build**

```bash
cd plugins/gopass
go test ./... -v
go build -o ../../bin/locksmith-plugin-gopass .
```

Expected: tests PASS, binary created.

- [ ] **Step 6: Commit**

```bash
git add plugins/gopass/
git commit -m "feat(plugins): add gopass vault plugin"
```

---

### Task 11: Keychain Plugin (macOS)

**Files:**
- Create: `plugins/keychain/main.go`
- Create: `plugins/keychain/provider_darwin.go` (CGo, build tag: darwin)
- Create: `plugins/keychain/provider_stub.go` (build tag: !darwin)
- Create: `plugins/keychain/provider_test.go`

- [ ] **Step 1: Write failing tests**

Write `plugins/keychain/provider_test.go`:
```go
package main

import (
	"context"
	"runtime"
	"testing"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

func TestKeychainProvider_Info(t *testing.T) {
	p := &KeychainProvider{}
	info, err := p.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "keychain" {
		t.Errorf("Name = %q, want %q", info.Name, "keychain")
	}
	if len(info.Platforms) == 0 {
		t.Error("Platforms is empty")
	}
}

func TestKeychainProvider_HealthCheck(t *testing.T) {
	p := &KeychainProvider{}
	resp, err := p.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if runtime.GOOS == "darwin" && !resp.Available {
		t.Error("keychain should be available on macOS")
	}
	if runtime.GOOS != "darwin" && resp.Available {
		t.Error("keychain should not be available on non-macOS")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd plugins/keychain && go test ./... -v
```

Expected: compilation error.

- [ ] **Step 3: Write provider_darwin.go (CGo)**

Write `plugins/keychain/provider_darwin.go`:
```go
//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	char* data;
	int   length;
	int   error_code;
	char* error_msg;
} KeychainResult;

// keychainGetPassword retrieves a generic password from the macOS Keychain.
// Passing kSecUseOperationPrompt causes the OS to show a Touch ID / password dialog.
static KeychainResult keychainGetPassword(const char* service, const char* account) {
	KeychainResult result = {NULL, 0, 0, NULL};

	CFStringRef svcRef  = CFStringCreateWithCString(NULL, service,  kCFStringEncodingUTF8);
	CFStringRef accRef  = CFStringCreateWithCString(NULL, account,  kCFStringEncodingUTF8);
	CFStringRef prompt  = CFStringCreateWithCString(NULL,
		"Locksmith wants to access a secret", kCFStringEncodingUTF8);

	CFMutableDictionaryRef q = CFDictionaryCreateMutable(NULL, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFDictionarySetValue(q, kSecClass,              kSecClassGenericPassword);
	CFDictionarySetValue(q, kSecAttrService,        svcRef);
	CFDictionarySetValue(q, kSecAttrAccount,        accRef);
	CFDictionarySetValue(q, kSecReturnData,         kCFBooleanTrue);
	CFDictionarySetValue(q, kSecMatchLimit,         kSecMatchLimitOne);
	CFDictionarySetValue(q, kSecUseOperationPrompt, prompt);

	CFTypeRef dataRef = NULL;
	OSStatus status = SecItemCopyMatching(q, &dataRef);

	if (status == errSecSuccess && dataRef != NULL) {
		CFDataRef data    = (CFDataRef)dataRef;
		result.length     = (int)CFDataGetLength(data);
		result.data       = (char*)malloc(result.length);
		memcpy(result.data, CFDataGetBytePtr(data), result.length);
		CFRelease(dataRef);
	} else {
		result.error_code = (int)status;
		result.error_msg  = (char*)malloc(64);
		snprintf(result.error_msg, 64, "SecItemCopyMatching failed: %d", (int)status);
	}

	CFRelease(q); CFRelease(svcRef); CFRelease(accRef); CFRelease(prompt);
	return result;
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// KeychainProvider retrieves secrets from the macOS Keychain using the Security framework.
// Authorization (Touch ID or password) is triggered by the OS when SecItemCopyMatching is called.
type KeychainProvider struct{}

// GetSecret retrieves a secret from the macOS Keychain. The service defaults to "locksmith"
// and can be overridden via opts["service"]. The path is used as the account name.
func (p *KeychainProvider) GetSecret(_ context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	service := "locksmith"
	if svc, ok := req.Opts["service"]; ok && svc != "" {
		service = svc
	}

	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cAccount := C.CString(req.Path)
	defer C.free(unsafe.Pointer(cAccount))

	result := C.keychainGetPassword(cService, cAccount)
	if result.error_code != 0 {
		msg := C.GoString(result.error_msg)
		C.free(unsafe.Pointer(result.error_msg))
		return nil, fmt.Errorf("keychain: %s", msg)
	}

	secret := C.GoBytes(unsafe.Pointer(result.data), result.length)
	C.memset(unsafe.Pointer(result.data), 0, C.size_t(result.length))
	C.free(unsafe.Pointer(result.data))

	return &vaultv1.GetSecretResponse{Secret: secret, ContentType: "text/plain"}, nil
}

// HealthCheck confirms the macOS Keychain is accessible.
func (p *KeychainProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "macOS Keychain available"}, nil
}

// Info returns plugin metadata.
func (p *KeychainProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{Name: "keychain", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}
```

- [ ] **Step 4: Write provider_stub.go**

Write `plugins/keychain/provider_stub.go`:
```go
//go:build !darwin

// Package main implements the locksmith-plugin-keychain binary.
// On non-macOS platforms, all operations return an unavailable error.
package main

import (
	"context"
	"fmt"
	"runtime"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

// KeychainProvider is a no-op stub on non-macOS platforms.
type KeychainProvider struct{}

func (p *KeychainProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return nil, fmt.Errorf("keychain is only available on macOS (current OS: %s)", runtime.GOOS)
}

func (p *KeychainProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{
		Available: false,
		Message:   fmt.Sprintf("keychain is only available on macOS (current OS: %s)", runtime.GOOS),
	}, nil
}

func (p *KeychainProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{Name: "keychain", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}
```

Write `plugins/keychain/main.go`:
```go
package main

import sdk "github.com/lorem-dev/locksmith-sdk"

func main() { sdk.Serve(&KeychainProvider{}) }
```

- [ ] **Step 5: Add dependencies**

```bash
cd plugins/keychain
go mod edit -replace github.com/lorem-dev/locksmith-sdk=../../sdk
go mod edit -replace github.com/lorem-dev/locksmith=../../
go get github.com/lorem-dev/locksmith-sdk
go mod tidy
```

- [ ] **Step 6: Run tests and build**

```bash
cd plugins/keychain
go test ./... -v
go build -o ../../bin/locksmith-plugin-keychain .
```

Expected: tests PASS, binary created.

- [ ] **Step 7: Commit**

```bash
git add plugins/keychain/
git commit -m "feat(plugins): add macOS Keychain vault plugin with Touch ID support via CGo"
```

---

### Task 12: Init Command — Agent Detection and TUI

**Files:**
- Create: `internal/initflow/detect.go`, `detect_test.go`
- Create: `internal/initflow/agents.go`, `agents_test.go`
- Create: `internal/initflow/sandbox.go`
- Create: `internal/initflow/flow.go`, `flow_test.go`
- Create: `internal/initflow/templates/*.tmpl`
- Create: `internal/cli/init_cmd.go`

- [ ] **Step 1: Write failing tests for agent detection**

Write `internal/initflow/detect_test.go`:
```go
package initflow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func TestDetectAgents_ClaudeCode_ByDir(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)

	agents := initflow.DetectAgents(home)
	for _, a := range agents {
		if a.Name == "Claude Code" {
			if !a.Detected {
				t.Error("Claude Code should be detected via directory")
			}
			return
		}
	}
	t.Error("Claude Code not in agent list")
}

func TestDetectAgents_NoneFound(t *testing.T) {
	home := t.TempDir()
	for _, a := range initflow.DetectAgents(home) {
		if a.Detected {
			t.Errorf("agent %q should not be detected in empty home", a.Name)
		}
	}
}

func TestDetectVaults_ContainsGopass(t *testing.T) {
	vaults := initflow.DetectVaults()
	for _, v := range vaults {
		if v.Type == "gopass" {
			return
		}
	}
	t.Error("gopass should always be in vault list")
}

func TestDetectVaults_KeychainOnMacOS(t *testing.T) {
	vaults := initflow.DetectVaults()
	for _, v := range vaults {
		if v.Type == "keychain" {
			// Just verify it's listed — Available depends on runtime OS
			return
		}
	}
	t.Error("keychain should always be in vault list")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/initflow/... -v
```

Expected: compilation error.

- [ ] **Step 3: Write detect.go**

Write `internal/initflow/detect.go`:
```go
// Package initflow implements the interactive locksmith setup wizard.
package initflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// DetectedAgent describes an AI agent installation found on the system.
type DetectedAgent struct {
	Name       string
	Detected   bool
	ConfigDir  string
	HomePath   string // path relative to home to check for existence
	BinaryName string // CLI binary name to check in PATH as fallback
}

// DetectedVault describes a vault backend found on the system.
type DetectedVault struct {
	Type      string
	Detected  bool
	Available bool // false on unsupported platforms
}

// DetectAgents scans homeDir for known AI agent installations.
// Detection uses two strategies: directory existence and CLI binary in PATH.
func DetectAgents(homeDir string) []DetectedAgent {
	agents := []DetectedAgent{
		{Name: "Claude Code", HomePath: ".claude", BinaryName: "claude"},
		{Name: "Codex", HomePath: ".codex", BinaryName: "codex"},
		{Name: "OpenCode", HomePath: filepath.Join(".config", "opencode"), BinaryName: "opencode"},
	}

	for i := range agents {
		a := &agents[i]
		dirPath := filepath.Join(homeDir, a.HomePath)

		// Strategy 1: config directory exists
		if _, err := os.Stat(dirPath); err == nil {
			a.Detected = true
			a.ConfigDir = dirPath
		}

		// Strategy 2: CLI binary in PATH (fallback — agent may not have created dir yet)
		if !a.Detected {
			if _, err := exec.LookPath(a.BinaryName); err == nil {
				a.Detected = true
				a.ConfigDir = dirPath // use expected dir even if not yet created
			}
		}
	}

	return agents
}

// DetectVaults returns all known vault types with platform availability and
// installation status for the current system.
func DetectVaults() []DetectedVault {
	vaults := []DetectedVault{
		{Type: "keychain", Available: runtime.GOOS == "darwin"},
		{Type: "gopass", Available: true},
		{Type: "1password", Available: true},
		{Type: "gnome-keyring", Available: runtime.GOOS == "linux"},
	}

	for i := range vaults {
		v := &vaults[i]
		switch v.Type {
		case "keychain":
			v.Detected = runtime.GOOS == "darwin"
		case "gopass":
			v.Detected = binaryExists("gopass")
		case "1password":
			v.Detected = binaryExists("op")
		case "gnome-keyring":
			v.Detected = binaryExists("secret-tool")
		}
	}
	return vaults
}

func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
```

- [ ] **Step 4: Run detection tests**

```bash
go test ./internal/initflow/... -v -run TestDetect
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Write templates**

Write `internal/initflow/templates/claude_skill.md.tmpl`:
```
---
name: locksmith-auth
description: Use when MCP server requires authentication headers
---

When an MCP server config contains `$(locksmith get ...)` in headers:
1. Check if LOCKSMITH_SESSION is set in environment
2. If not — run `locksmith session start` and export the token
3. Use `locksmith get --key <name>` to retrieve secrets
4. Pass retrieved value as the header value
5. Delegate LOCKSMITH_SESSION to sub-agents via environment

Never hardcode secrets. Never cache secrets outside of locksmith.
```

Write `internal/initflow/templates/claude_md.md.tmpl`:
```
## Locksmith Integration

This environment uses [Locksmith](https://github.com/lorem-dev/locksmith) for secure MCP secret management.

- Retrieve secrets: `locksmith get --key <alias>`
- Check session: `echo $LOCKSMITH_SESSION`
- Start session: `locksmith session start` then `export LOCKSMITH_SESSION=<id>`
- Delegate to sub-agents by passing `LOCKSMITH_SESSION` in environment
```

Write `internal/initflow/templates/codex_agents.md.tmpl`:
```
## Locksmith Integration

Secrets are managed by Locksmith. Use `locksmith get --key <alias>` to retrieve them.

Before accessing secrets:
1. Ensure `LOCKSMITH_SESSION` is set in the environment
2. If not set, run `locksmith session start` and export the session ID
3. Pass `LOCKSMITH_SESSION` to sub-agents
```

Write `internal/initflow/templates/agent_instructions.md.tmpl`:
```
# Locksmith — Agent Instructions

## Retrieving Secrets

Use `locksmith get --key <alias>` to retrieve secrets from configured vaults.

## Session Management

1. Start: `locksmith session start` → outputs session ID
2. Export: `export LOCKSMITH_SESSION=<session_id>`
3. All subsequent `locksmith get` calls use this session
4. End: `locksmith session end`

## Sub-Agents

Pass `LOCKSMITH_SESSION` to sub-agents via environment. They inherit the parent session's access.
```

- [ ] **Step 6: Write agents.go**

Write `internal/initflow/agents.go`:
```go
package initflow

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*
var templates embed.FS

// AgentWriter installs locksmith instructions into AI agent configuration directories.
type AgentWriter struct {
	HomeDir string
}

// NewAgentWriter creates an AgentWriter for the given home directory.
func NewAgentWriter(homeDir string) *AgentWriter {
	return &AgentWriter{HomeDir: homeDir}
}

// Install writes locksmith instructions for the given agent.
func (w *AgentWriter) Install(agent DetectedAgent) error {
	switch agent.Name {
	case "Claude Code":
		return w.installClaudeCode(agent)
	case "Codex":
		return w.installCodex(agent)
	case "OpenCode":
		return w.installOpenCode(agent)
	default:
		return w.installGeneric()
	}
}

func (w *AgentWriter) installClaudeCode(agent DetectedAgent) error {
	skillDir := filepath.Join(agent.ConfigDir, "skills")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return fmt.Errorf("creating skills dir: %w", err)
	}
	skillContent, _ := templates.ReadFile("templates/claude_skill.md.tmpl")
	if err := os.WriteFile(filepath.Join(skillDir, "locksmith.md"), skillContent, 0o644); err != nil {
		return fmt.Errorf("writing skill: %w", err)
	}
	mdContent, _ := templates.ReadFile("templates/claude_md.md.tmpl")
	return appendIfAbsent(filepath.Join(agent.ConfigDir, "CLAUDE.md"), string(mdContent), "## Locksmith Integration")
}

func (w *AgentWriter) installCodex(agent DetectedAgent) error {
	if err := os.MkdirAll(agent.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	content, _ := templates.ReadFile("templates/codex_agents.md.tmpl")
	return appendIfAbsent(filepath.Join(agent.ConfigDir, "AGENTS.md"), string(content), "## Locksmith Integration")
}

func (w *AgentWriter) installOpenCode(agent DetectedAgent) error {
	if err := os.MkdirAll(agent.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	content, _ := templates.ReadFile("templates/agent_instructions.md.tmpl")
	return appendIfAbsent(filepath.Join(agent.ConfigDir, "instructions.md"), string(content), "# Locksmith")
}

func (w *AgentWriter) installGeneric() error {
	dir := filepath.Join(w.HomeDir, ".config", "locksmith")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	content, _ := templates.ReadFile("templates/agent_instructions.md.tmpl")
	return os.WriteFile(filepath.Join(dir, "agent-instructions.md"), content, 0o644)
}

// appendIfAbsent appends content to filePath only if marker is not already present.
// This makes the operation idempotent.
func appendIfAbsent(filePath, content, marker string) error {
	existing, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(existing), marker) {
		return nil
	}
	prefix := ""
	if len(existing) > 0 {
		prefix = "\n\n"
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(prefix + content)
	return err
}

```

- [ ] **Step 7: Write agents_test.go**

Write `internal/initflow/agents_test.go`:
```go
package initflow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func TestInstall_ClaudeCode_CreatesSkillAndUpdatesClaudeMd(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(claudeDir, "skills", "locksmith.md")); err != nil {
		t.Error("skill file not created")
	}
	content, _ := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if !strings.Contains(string(content), "Locksmith Integration") {
		t.Error("CLAUDE.md missing Locksmith section")
	}
}

func TestInstall_ClaudeCode_Idempotent(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	agent := initflow.DetectedAgent{Name: "Claude Code", Detected: true, ConfigDir: claudeDir}
	writer := initflow.NewAgentWriter(home)
	writer.Install(agent)
	writer.Install(agent) // second call must not duplicate

	content, _ := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if strings.Count(string(content), "## Locksmith Integration") != 1 {
		t.Errorf("section duplicated; count = %d", strings.Count(string(content), "## Locksmith Integration"))
	}
}

func TestInstall_Codex(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	agent := initflow.DetectedAgent{Name: "Codex", Detected: true, ConfigDir: codexDir}
	writer := initflow.NewAgentWriter(home)
	if err := writer.Install(agent); err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(codexDir, "AGENTS.md"))
	if !strings.Contains(string(content), "Locksmith") {
		t.Error("AGENTS.md missing Locksmith section")
	}
}
```

- [ ] **Step 8: Write sandbox.go**

Write `internal/initflow/sandbox.go`:
```go
package initflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// locksmithAllowList is the set of locksmith commands to permit in agent sandboxes.
var locksmithAllowList = []string{
	"Bash(locksmith get *)",
	"Bash(locksmith session start *)",
	"Bash(locksmith session end)",
	"Bash(locksmith vault list)",
	"Bash(locksmith vault health)",
}

// InstallSandboxPermissions adds locksmith commands to the agent's permission allowlist.
// Note: "Bash(...)" refers to the Claude Code tool name, not the user's shell —
// it works regardless of whether the user runs bash, zsh, or fish.
func InstallSandboxPermissions(agent DetectedAgent) error {
	switch agent.Name {
	case "Claude Code":
		return installClaudeSandbox(agent)
	case "Codex":
		return installCodexSandbox(agent)
	}
	return nil
}

func installClaudeSandbox(agent DetectedAgent) error {
	settingsPath := filepath.Join(agent.ConfigDir, "settings.json")
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &settings) //nolint:errcheck
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	perms, _ := settings["permissions"].(map[string]interface{})
	if perms == nil {
		perms = make(map[string]interface{})
	}
	allowList, _ := perms["allow"].([]interface{})

	existing := make(map[string]bool)
	for _, item := range allowList {
		if s, ok := item.(string); ok {
			existing[s] = true
		}
	}
	for _, perm := range locksmithAllowList {
		if !existing[perm] {
			allowList = append(allowList, perm)
		}
	}

	perms["allow"] = allowList
	settings["permissions"] = perms
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	return os.WriteFile(settingsPath, out, 0o644)
}

func installCodexSandbox(agent DetectedAgent) error {
	policyPath := filepath.Join(agent.ConfigDir, "policy.yaml")
	content := "# Locksmith permissions\nallow:\n"
	for _, perm := range locksmithAllowList {
		cmd := perm
		if len(cmd) > 5 && cmd[:5] == "Bash(" {
			cmd = cmd[5 : len(cmd)-1]
		}
		content += fmt.Sprintf("  - %q\n", cmd)
	}
	return appendIfAbsent(policyPath, content, "# Locksmith permissions")
}
```

- [ ] **Step 9: Write flow.go and init_cmd.go**

Write `internal/initflow/flow.go`:
```go
package initflow

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"

	"github.com/lorem-dev/locksmith/internal/config"
)

// InitOptions controls the behaviour of RunInit.
type InitOptions struct {
	NoTUI      bool
	Auto       bool
	AgentOnly  string
	SkipAgents bool
}

// InitResult holds the resolved configuration from the init wizard.
type InitResult struct {
	ConfigPath     string
	SelectedVaults []string
	SelectedAgents []DetectedAgent
	SandboxEnabled bool
}

// RunInit runs the interactive setup wizard. In --auto mode all prompts are
// skipped and detected defaults are applied. In --no-tui mode huh's accessible
// mode is used (plain text prompts), which also activates automatically when
// TERM=dumb or stdin is not a TTY.
func RunInit(opts InitOptions) (*InitResult, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	// Use accessible mode (no TUI) if explicitly requested, TERM=dumb, or non-TTY stdin
	accessible := opts.NoTUI || os.Getenv("TERM") == "dumb" || !isTerminal()

	result := &InitResult{}
	defaultConfigDir := filepath.Join(homeDir, ".config", "locksmith")

	// --- Config location ---
	configDir := defaultConfigDir
	if !opts.Auto {
		configDir, err = promptConfigLocation(defaultConfigDir, accessible)
		if err != nil {
			return nil, err
		}
	}
	result.ConfigPath = filepath.Join(configDir, "config.yaml")

	// --- Vault selection ---
	detectedVaults := DetectVaults()
	if opts.Auto {
		for _, v := range detectedVaults {
			if v.Detected {
				result.SelectedVaults = append(result.SelectedVaults, v.Type)
			}
		}
	} else {
		result.SelectedVaults, err = promptVaultSelection(detectedVaults, accessible)
		if err != nil {
			return nil, err
		}
	}

	// --- Agent selection ---
	if !opts.SkipAgents {
		detectedAgents := DetectAgents(homeDir)
		if opts.AgentOnly != "" {
			for _, a := range detectedAgents {
				if agentMatches(a.Name, opts.AgentOnly) {
					result.SelectedAgents = append(result.SelectedAgents, a)
				}
			}
		} else if opts.Auto {
			for _, a := range detectedAgents {
				if a.Detected {
					result.SelectedAgents = append(result.SelectedAgents, a)
				}
			}
		} else {
			result.SelectedAgents, err = promptAgentSelection(detectedAgents, accessible)
			if err != nil {
				return nil, err
			}
		}
	}

	// --- Sandbox permissions ---
	if len(result.SelectedAgents) > 0 {
		if opts.Auto {
			result.SandboxEnabled = true
		} else {
			result.SandboxEnabled, err = promptSandbox(accessible)
			if err != nil {
				return nil, err
			}
		}
	}

	// --- Summary + confirmation ---
	if !opts.Auto {
		if ok, err := promptSummary(result, accessible); err != nil || !ok {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("cancelled by user")
		}
	}

	if err := applyInit(result, homeDir); err != nil {
		return nil, err
	}
	return result, nil
}

func applyInit(result *InitResult, homeDir string) error {
	if err := os.MkdirAll(filepath.Dir(result.ConfigPath), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	cfg := config.Config{
		Defaults: config.Defaults{SessionTTL: "3h", SocketPath: "~/.config/locksmith/locksmith.sock"},
		Logging:  config.Logging{Level: "info", Format: "text"},
		Vaults:   make(map[string]config.Vault),
		Keys:     make(map[string]config.Key),
	}
	for _, vt := range result.SelectedVaults {
		cfg.Vaults[vt] = config.Vault{Type: vt}
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(result.ConfigPath, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("  config written to %s\n", result.ConfigPath)

	writer := NewAgentWriter(homeDir)
	for _, agent := range result.SelectedAgents {
		if err := writer.Install(agent); err != nil {
			return fmt.Errorf("installing %s instructions: %w", agent.Name, err)
		}
		fmt.Printf("  %s: instructions installed\n", agent.Name)
		if result.SandboxEnabled {
			if err := InstallSandboxPermissions(agent); err != nil {
				return fmt.Errorf("installing sandbox for %s: %w", agent.Name, err)
			}
			fmt.Printf("  %s: sandbox permissions configured\n", agent.Name)
		}
	}
	return nil
}

func promptConfigLocation(defaultDir string, accessible bool) (string, error) {
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Where to store config?").
			Options(
				huh.NewOption(fmt.Sprintf("%s (default)", defaultDir), defaultDir),
				huh.NewOption("Custom path", "custom"),
			).Value(&selected),
	)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return "", err
	}
	if selected != "custom" {
		return selected, nil
	}
	var custom string
	form2 := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Config directory:").Value(&custom),
	)).WithAccessible(accessible)
	if err := form2.Run(); err != nil {
		return "", err
	}
	return config.ExpandPath(custom), nil
}

func promptVaultSelection(vaults []DetectedVault, accessible bool) ([]string, error) {
	var options []huh.Option[string]
	for _, v := range vaults {
		label := v.Type
		if v.Detected {
			label += " (detected)"
		}
		if !v.Available {
			label += " (not available on this platform)"
		}
		options = append(options, huh.NewOption(label, v.Type))
	}
	var selected []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Which vault backends do you use?").
			Options(options...).Value(&selected),
	)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

func promptAgentSelection(agents []DetectedAgent, accessible bool) ([]DetectedAgent, error) {
	var detected []DetectedAgent
	for _, a := range agents {
		if a.Detected {
			detected = append(detected, a)
		}
	}
	if len(detected) == 0 {
		fmt.Println("No AI agents detected.")
		return nil, nil
	}

	var selection string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Install locksmith for detected agents?").
			Options(
				huh.NewOption(fmt.Sprintf("All detected (%d)", len(detected)), "all"),
				huh.NewOption("Select manually", "manual"),
				huh.NewOption("Skip agent setup", "skip"),
			).Value(&selection),
	)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return nil, err
	}

	if selection == "skip" {
		return nil, nil
	}
	if selection == "all" {
		return detected, nil
	}

	var options []huh.Option[string]
	for _, a := range agents {
		label := a.Name
		if a.Detected {
			label += " (detected)"
		}
		options = append(options, huh.NewOption(label, a.Name))
	}
	var selectedNames []string
	form2 := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("Select agents:").Options(options...).Value(&selectedNames),
	)).WithAccessible(accessible)
	if err := form2.Run(); err != nil {
		return nil, err
	}
	var result []DetectedAgent
	for _, a := range agents {
		for _, name := range selectedNames {
			if a.Name == name {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

func promptSandbox(accessible bool) (bool, error) {
	var enabled bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Allow locksmith commands in agent sandboxes?").
			Description("locksmith get, session start/end, vault list/health").
			Value(&enabled),
	)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return false, err
	}
	return enabled, nil
}

func promptSummary(result *InitResult, accessible bool) (bool, error) {
	agentNames := make([]string, len(result.SelectedAgents))
	for i, a := range result.SelectedAgents {
		agentNames[i] = a.Name
	}
	fmt.Printf("\n── Summary ──────────────────────────\nConfig:  %s\nVaults:  %v\nAgents:  %v\nSandbox: %v\n",
		result.ConfigPath, result.SelectedVaults, agentNames, result.SandboxEnabled)

	var confirmed bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Apply?").Value(&confirmed),
	)).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

func agentMatches(name, query string) bool {
	m := func(s string) string {
		b := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			b[i] = c
		}
		return string(b)
	}
	ln, lq := m(name), m(query)
	return ln == lq || (lq == "claude" && ln == "claude code")
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
```

Write `internal/cli/init_cmd.go`:
```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

// newInitCmd returns the `locksmith init` command — an interactive setup wizard
// that configures vaults, detects AI agents, and installs instructions/permissions.
func newInitCmd() *cobra.Command {
	var noTUI, auto, skipAgents bool
	var agentOnly string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive setup for locksmith",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := initflow.RunInit(initflow.InitOptions{
				NoTUI:      noTUI,
				Auto:       auto,
				AgentOnly:  agentOnly,
				SkipAgents: skipAgents,
			})
			if err != nil {
				return err
			}
			fmt.Printf("\nSetup complete! Config: %s\nRun 'locksmith serve' to start the daemon.\n", result.ConfigPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "use plain-text prompts (also auto-enabled when TERM=dumb or non-TTY stdin)")
	cmd.Flags().BoolVar(&auto, "auto", false, "auto-detect everything, apply defaults without prompts")
	cmd.Flags().StringVar(&agentOnly, "agent", "", "install for one specific agent only")
	cmd.Flags().BoolVar(&skipAgents, "skip-agents", false, "skip agent setup")
	// Note: --update-agents flag (reinstall agent instructions) is deferred to a future task.
	return cmd
}
```

- [ ] **Step 10: Write flow_test.go**

Write `internal/initflow/flow_test.go`:
```go
package initflow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

func TestAgentMatches_CaseInsensitive(t *testing.T) {
	cases := []struct {
		name, query string
		want        bool
	}{
		{"Claude Code", "claude", true},
		{"Claude Code", "Claude Code", true},
		{"Codex", "codex", true},
		{"Codex", "claude", false},
		{"OpenCode", "opencode", true},
	}
	for _, c := range cases {
		got := agentMatchesExported(c.name, c.query)
		if got != c.want {
			t.Errorf("agentMatches(%q, %q) = %v, want %v", c.name, c.query, got, c.want)
		}
	}
}

func TestRunInit_Auto_NoAgents(t *testing.T) {
	home := t.TempDir()
	// Override HOME so DetectAgents finds nothing
	t.Setenv("HOME", home)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto:       true,
		SkipAgents: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	if result == nil {
		t.Fatal("RunInit() returned nil result")
	}
	// Config file should be written
	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Errorf("config file not created at %s: %v", result.ConfigPath, err)
	}
}

func TestRunInit_Auto_WithVaultDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto:       true,
		SkipAgents: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	// Result must never be nil
	if result == nil {
		t.Fatal("RunInit() returned nil result")
	}
}

func TestRunInit_Auto_InstallsClaudeCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Create Claude Code dir so it is detected
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)

	result, err := initflow.RunInit(initflow.InitOptions{
		Auto: true,
	})
	if err != nil {
		t.Fatalf("RunInit() error: %v", err)
	}
	found := false
	for _, a := range result.SelectedAgents {
		if a.Name == "Claude Code" {
			found = true
		}
	}
	if !found {
		t.Error("expected Claude Code in SelectedAgents")
	}
}
```

Note: `agentMatches` is unexported in `flow.go`. Either export it as `AgentMatches` for testing, or move it to a separate `match.go` file. Export it — rename in `flow.go` from `agentMatches` to `AgentMatches` and update `promptAgentSelection` to call `AgentMatches`. Then reference it as `initflow.AgentMatches` in the test.

- [ ] **Step 11: Add huh dependency**

```bash
go get github.com/charmbracelet/huh
go mod tidy
```

- [ ] **Step 12: Run all initflow tests**

```bash
go test ./internal/initflow/... -v
go test -race ./internal/initflow/...
```

Expected: all tests PASS, race detector clean.

- [ ] **Step 13: Build and verify init command**

```bash
go build -o bin/locksmith ./cmd/locksmith
./bin/locksmith init --help
```

Expected: `--no-tui`, `--auto`, `--agent`, `--skip-agents` flags shown.

- [ ] **Step 14: Commit**

```bash
git add internal/initflow/ internal/cli/init_cmd.go internal/cli/root.go go.mod go.sum
git commit -m "feat(init): add locksmith init with TUI forms, agent detection, and sandbox permissions"
```

---

### Task 13: Documentation

**Files:**
- Create: `README.md`
- Create: `docs/architecture.md`, `docs/configuration.md`, `docs/plugins.md`

- [ ] **Step 1: Write README.md**

Write `README.md`:
```markdown
# Locksmith

Secure middleware that gives AI agents access to secrets stored in vault providers
(macOS Keychain, gopass, 1Password, GNOME Keyring), with per-session caching and
vault-delegated authorization (Touch ID, GPG passphrase).

**Repository:** https://github.com/lorem-dev/locksmith

## Installation

```bash
go install github.com/lorem-dev/locksmith/cmd/locksmith@latest
```

Or build from source:

```bash
git clone https://github.com/lorem-dev/locksmith
cd locksmith
make build-all
```

## Quick Start

```bash
# 1. Initialize (interactive TUI)
locksmith init

# 2. Start the daemon
locksmith serve &

# 3. Start a session
export LOCKSMITH_SESSION=$(locksmith session start | jq -r .session_id)

# 4. Use in MCP config headers
# "Authorization": "Bearer $(locksmith get --key my-api-key)"

# 5. End session when done
locksmith session end
```

## MCP Integration

In your MCP server config:

```json
{
  "mcpServers": {
    "my-server": {
      "url": "https://api.example.com",
      "headers": {
        "Authorization": "Bearer $(locksmith get --key my-api-key)"
      }
    }
  }
}
```

## Supported Vaults

| Vault | Platform | Auth |
|-------|----------|------|
| macOS Keychain | macOS | Touch ID / password |
| gopass | macOS, Linux | GPG passphrase / Touch ID |
| 1Password | macOS, Linux | Touch ID / master password |
| GNOME Keyring | Linux | Keyring password |

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration Reference](docs/configuration.md)
- [Writing Vault Plugins](docs/plugins.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
```

- [ ] **Step 2: Write docs/architecture.md**

Write `docs/architecture.md`:
```markdown
# Locksmith Architecture

## Overview

Locksmith uses a plugin architecture: vault providers run as isolated processes
communicating with the central daemon over gRPC, using
[hashicorp/go-plugin](https://github.com/hashicorp/go-plugin).

```
locksmith CLI  ──(gRPC/Unix socket)──▶  locksmith daemon
                                              │
                                    ┌─────────┼─────────┐
                               gRPC ▼    gRPC ▼    gRPC ▼
                           keychain   gopass   1password
                           plugin     plugin   plugin
```

## Components

### Daemon
- Listens on a Unix socket (`~/.config/locksmith/locksmith.sock`)
- Exposes `LocksmithService` gRPC service to CLI clients
- Manages session lifecycle and secret caching
- Launches vault plugins on startup; kills them on shutdown

### Session Manager (`internal/session`)
- Sessions identified by `ls_<32-byte-hex>` tokens
- TTL-based expiry (default 3h, configurable)
- Per-session secret cache: secrets are fetched from vault once per session,
  then served from memory cache for subsequent calls
- On session invalidation: explicit byte-zeroing of cached secrets (`memclr`)

### Plugin Manager (`internal/plugin`)
- Discovers `locksmith-plugin-*` binaries in standard search paths
- Launches each required plugin as a child process via hashicorp/go-plugin
- Plugin processes communicate over gRPC; isolated from daemon memory space

### Vault Plugins
Each plugin is a standalone binary implementing the `VaultProvider` gRPC service:
- `GetSecret` — fetches a secret; triggers vault authorization (Touch ID, passphrase)
- `HealthCheck` — verifies the vault is installed and accessible
- `Info` — returns plugin name, version, supported platforms

### CLI
Thin gRPC client to the daemon. Reads `LOCKSMITH_SESSION` from environment.
Returns an error with a helpful hint if the daemon is not running.

## Session Delegation

Sub-agents inherit the parent session by receiving `LOCKSMITH_SESSION` as an
environment variable. The daemon validates the session token on every request.

## Security Properties

- Secrets live only in daemon process memory, never on disk
- Unix socket has `0600` permissions (owner-only)
- Plugin processes are isolated; a compromised plugin cannot access other vaults
- `locksmith get` without a valid session returns an error, not a leaked secret
```

- [ ] **Step 3: Write docs/configuration.md**

Write `docs/configuration.md`:
```markdown
# Configuration Reference

Default config path: `~/.config/locksmith/config.yaml`

```yaml
defaults:
  session_ttl: 3h                            # default session duration
  socket_path: ~/.config/locksmith/locksmith.sock

logging:
  level: info                                # debug | info | warn | error
  format: text                               # text | json

vaults:
  keychain:
    type: keychain                           # macOS Keychain
  my-gopass:
    type: gopass
    store: personal                          # optional gopass store name

keys:
  github-token:
    vault: keychain
    path: "github-api-token"                 # account name in Keychain
  anthropic-key:
    vault: my-gopass
    path: "dev/anthropic"                    # path in gopass store
```

## Vault Types

| type | Description |
|------|-------------|
| `keychain` | macOS Keychain (CGo, Touch ID) |
| `gopass` | gopass password manager (shells out to `gopass` CLI) |
| `1password` | 1Password (shells out to `op` CLI) — future |
| `gnome-keyring` | GNOME Keyring (D-Bus) — future |

## Direct Access (Without Alias)

```bash
locksmith get --vault keychain --path my-account
locksmith get --vault my-gopass --path dev/key
```
```

- [ ] **Step 4: Write docs/plugins.md**

Write `docs/plugins.md`:
```markdown
# Writing a Vault Plugin

Locksmith vault plugins are standalone Go binaries that implement the
`VaultProvider` gRPC service via the SDK.

## Quickstart

```bash
go mod init github.com/yourorg/locksmith-plugin-myvault
go get github.com/lorem-dev/locksmith-sdk
```

```go
package main

import (
    "context"
    sdk "github.com/lorem-dev/locksmith-sdk"
    vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

type MyVaultProvider struct{}

func (p *MyVaultProvider) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
    // fetch secret from your vault, trigger auth if needed
    return &vaultv1.GetSecretResponse{Secret: []byte("secret"), ContentType: "text/plain"}, nil
}

func (p *MyVaultProvider) HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
    return &vaultv1.HealthCheckResponse{Available: true, Message: "ok"}, nil
}

func (p *MyVaultProvider) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
    return &vaultv1.PluginInfo{Name: "myvault", Version: "0.1.0", Platforms: []string{"linux"}}, nil
}

func main() { sdk.Serve(&MyVaultProvider{}) }
```

## Plugin Discovery

Name your binary `locksmith-plugin-<type>` and place it in one of:
1. Same directory as the `locksmith` binary
2. `~/.config/locksmith/plugins/`
3. Anywhere in `$PATH`

The daemon loads plugins for vault types referenced in `config.yaml`.
```

- [ ] **Step 5: Commit documentation**

```bash
git add README.md docs/architecture.md docs/configuration.md docs/plugins.md
git commit -m "docs: add README, architecture, configuration, and plugin authoring guide"
```

---

### Task 14: Changelog Skill

**Files:**
- Create: `.claude/skills/changelog/SKILL.md`

- [ ] **Step 1: Create changelog skill**

Write `.claude/skills/changelog/SKILL.md`:
```bash
mkdir -p .claude/skills/changelog
```
```markdown
---
name: changelog
description: Compress development changes into a versioned CHANGES.md entry before cutting a release
---

## Instructions

Read CHANGES.md and identify all items listed under `## Development`.

Summarize those items into a concise bullet list — group related changes,
remove redundancy, keep each bullet to one line.

Ask the user for the version number (e.g. v0.1.0) and release date.

Replace the `## Development` section with:
1. A new empty `## Development` section at the top
2. A new `## Version <version> — <date>` section below it containing the compressed bullet list

Write the updated CHANGES.md. Show a diff summary to the user before saving.

Do not include AI tool names in the changelog.
```

- [ ] **Step 2: Commit**

```bash
mkdir -p .claude/skills/changelog
git add .claude/skills/changelog/SKILL.md
git commit -m "chore: add changelog compression skill"
```

---

### Task 15: Integration Test and Final Verification

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Write integration test**

Write `integration_test.go`:
```go
//go:build integration

package locksmith_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

func TestFullSessionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(`
defaults:
  session_ttl: 1h
  socket_path: `+socketPath+`
logging:
  level: error
  format: text
vaults:
  keychain:
    type: keychain
keys:
  test-key:
    vault: keychain
    path: test-secret
`), 0o644)

	cfg, err := config.Load(filepath.Join(tmpDir, "config.yaml"))
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}

	d := daemon.New(cfg)
	go func() { d.Start() }()
	defer d.Stop()

	// Wait for socket to appear
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial daemon: %v", err)
	}
	defer conn.Close()

	client := locksmithv1.NewLocksmithServiceClient(conn)
	ctx := context.Background()

	// Start session
	start, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{Ttl: "30m"})
	if err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if start.SessionId == "" {
		t.Fatal("empty session ID")
	}

	// List — expect 1
	list, _ := client.SessionList(ctx, &locksmithv1.SessionListRequest{})
	if len(list.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(list.Sessions))
	}

	// End session
	if _, err := client.SessionEnd(ctx, &locksmithv1.SessionEndRequest{SessionId: start.SessionId}); err != nil {
		t.Fatalf("SessionEnd: %v", err)
	}

	// List — expect 0
	list, _ = client.SessionList(ctx, &locksmithv1.SessionListRequest{})
	if len(list.Sessions) != 0 {
		t.Fatalf("sessions after end = %d, want 0", len(list.Sessions))
	}
}
```

- [ ] **Step 2: Run integration test**

```bash
go test -tags=integration -v -run TestFullSessionLifecycle
```

Expected: PASS.

- [ ] **Step 3: Full test suite with race detector**

```bash
make test-race
```

Expected: all PASS, no races.

- [ ] **Step 4: Coverage check**

```bash
make test-coverage
```

Expected: total coverage ≥ 90%.

- [ ] **Step 5: Lint**

```bash
make lint
```

Expected: clean.

- [ ] **Step 6: Full build**

```bash
make build-all
```

Expected: `bin/locksmith`, `bin/locksmith-plugin-keychain`, `bin/locksmith-plugin-gopass` created.

- [ ] **Step 7: Verify all CLI commands**

```bash
./bin/locksmith --help
./bin/locksmith init --help
./bin/locksmith serve --help
./bin/locksmith get --help
./bin/locksmith session --help
./bin/locksmith vault --help
./bin/locksmith config check --help
```

Expected: all show correct help with expected flags.

- [ ] **Step 8: Update CHANGES.md — initial Development entry**

Add any remaining development items to the `## Development` section of `CHANGES.md`.

- [ ] **Step 9: Final commit**

```bash
git add integration_test.go CHANGES.md
git commit -m "test: add integration test for full session lifecycle"
```
