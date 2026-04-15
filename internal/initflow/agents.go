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
