package mcp

import (
	"fmt"

	"github.com/lorem-dev/locksmith/internal/config"
)

// LoadServerConfig loads the named MCP server entry from the config file at path.
func LoadServerConfig(cfgPath, serverName string) (config.MCPServerConfig, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.MCPServerConfig{}, fmt.Errorf("loading config: %w", err)
	}
	server, ok := cfg.MCP.Servers[serverName]
	if !ok {
		return config.MCPServerConfig{}, fmt.Errorf("server %q not found in mcp.servers", serverName)
	}
	return server, nil
}
