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
	// Service is the default Keychain service name for keychain vaults.
	// Individual keys can override this via the "service/account" path shorthand.
	Service string `yaml:"service,omitempty"`
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
		if vaultDef.Type == "keychain" && strings.Count(key.Path, "/") > 1 {
			return fmt.Errorf("key %q: keychain path %q has too many segments (use \"service/account\" or \"account\")", name, key.Path)
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
