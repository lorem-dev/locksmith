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
	Agent    AgentConfig      `yaml:"agent"`
	Vaults   map[string]Vault `yaml:"vaults"`
	Keys     map[string]Key   `yaml:"keys"`
	MCP      MCPConfig        `yaml:"mcp"`
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
	// File is an optional path to a log file. If set, logs are written to
	// this file instead of stdout. Supports ~ expansion.
	// The file is rotated at 50 MB and logs older than 3 days are deleted.
	File string `yaml:"file"`
}

// AgentConfig controls how Locksmith behaves in agentic workflows.
type AgentConfig struct {
	// PassSessionToSubagents controls whether agents should pass
	// LOCKSMITH_SESSION to child agents they spawn. Default: true.
	// Uses a pointer to distinguish "false" from "not set".
	PassSessionToSubagents *bool `yaml:"pass_session_to_subagents"`
}

// PassSubagents returns the effective value of PassSessionToSubagents,
// applying the default of true when the field is not explicitly set.
func (a AgentConfig) PassSubagents() bool {
	if a.PassSessionToSubagents == nil {
		return true
	}
	return *a.PassSessionToSubagents
}

// Vault represents a configured vault backend.
type Vault struct {
	Type    string `yaml:"type"`
	Store   string `yaml:"store,omitempty"`
	Account string `yaml:"account,omitempty"`
	// Service is the default Keychain service name for keychain vaults.
	// Individual keys can override this via the "service/account" path shorthand.
	Service string `yaml:"service,omitempty"`
}

// Key is a named alias pointing to a secret in a specific vault.
type Key struct {
	Vault string `yaml:"vault"`
	Path  string `yaml:"path"`
}

// MCPConfig holds MCP server wrapper configurations.
type MCPConfig struct {
	Servers map[string]MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig configures one MCP server entry.
// Exactly one of Command or URL must be set.
type MCPServerConfig struct {
	// Local mode
	Command []string               `yaml:"command,omitempty"`
	Env     map[string]MCPEnvValue `yaml:"env,omitempty"`
	// Proxy mode
	URL       string            `yaml:"url,omitempty"`
	Transport string            `yaml:"transport,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty"`
}

// MCPEnvValue is the value of an env mapping: either a key alias string
// or a vault+path struct. Custom YAML unmarshaling handles both forms.
type MCPEnvValue struct {
	KeyAlias  string // set when unmarshaled from a plain string
	VaultName string // set when unmarshaled from vault: field
	Path      string // set when unmarshaled from path: field
}

// UnmarshalYAML supports both string and struct forms:
//
//	GITHUB_TOKEN: my-key-alias
//	API_KEY:
//	  vault: gopass
//	  path: work/api/key
func (v *MCPEnvValue) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		v.KeyAlias = value.Value
		return nil
	}
	var s struct {
		Vault string `yaml:"vault"`
		Path  string `yaml:"path"`
	}
	if err := value.Decode(&s); err != nil {
		return err
	}
	v.VaultName = s.Vault
	v.Path = s.Path
	return nil
}

// Load reads and validates the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is user-provided config file path
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return LoadFromBytes(data)
}

// LoadFromBytes parses and validates a config from raw YAML bytes.
// Useful for testing without writing files to disk.
func LoadFromBytes(data []byte) (*Config, error) {
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
		vaultDef, ok := c.Vaults[key.Vault]
		if !ok {
			return fmt.Errorf("key %q references unknown vault %q", name, key.Vault)
		}
		if key.Path == "" {
			return fmt.Errorf("key %q has empty path", name)
		}
		// Keychain paths support "service/account" shorthand but not deeper nesting.
		if vaultDef.Type == VaultKeychain && strings.Count(key.Path, "/") > 1 {
			return fmt.Errorf(
				"key %q: keychain path %q has too many segments (use \"service/account\" or \"account\")",
				name,
				key.Path,
			)
		}
	}

	for name, server := range c.MCP.Servers {
		if server.Command == nil && server.URL == "" {
			return fmt.Errorf("mcp.servers.%s: must specify either command or url", name)
		}
		if server.Command != nil && server.URL != "" {
			return fmt.Errorf("mcp.servers.%s: cannot specify both command and url", name)
		}
		if server.Transport != "" &&
			server.Transport != "auto" &&
			server.Transport != "sse" &&
			server.Transport != "http" {
			return fmt.Errorf("mcp.servers.%s: transport must be auto, sse, or http", name)
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
