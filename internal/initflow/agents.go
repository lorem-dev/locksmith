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

// mustReadTemplate reads an embedded template by name and panics if not found.
// Templates are embedded at compile time; a missing template is a programming error.
func mustReadTemplate(name string) []byte {
	data, err := templates.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("embedded template %q not found: %v", name, err))
	}
	return data
}

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
	if err := os.MkdirAll(skillDir, 0o755); err != nil { //nolint:gosec // G301: 0755 is standard for user config dirs
		return fmt.Errorf("creating skills dir: %w", err)
	}
	skillContent := mustReadTemplate("templates/claude_skill.md.tmpl")
	if err := os.WriteFile( //nolint:gosec // G306: skill files are documentation, readable by user is intentional
		filepath.Join(skillDir, "locksmith.md"), skillContent, 0o644,
	); err != nil {
		return fmt.Errorf("writing skill: %w", err)
	}
	mdContent := mustReadTemplate("templates/claude_md.md.tmpl")
	return appendIfAbsent(
		filepath.Join(agent.ConfigDir, "CLAUDE.md"), string(mdContent), "## Locksmith Integration",
	)
}

func (w *AgentWriter) installCodex(agent DetectedAgent) error {
	if err := os.MkdirAll( //nolint:gosec // G301: 0755 is standard for user config dirs
		agent.ConfigDir,
		0o755,
	); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	content := mustReadTemplate("templates/codex_agents.md.tmpl")
	return appendIfAbsent(
		filepath.Join(agent.ConfigDir, "AGENTS.md"), string(content), "## Locksmith Integration",
	)
}

func (w *AgentWriter) installOpenCode(agent DetectedAgent) error {
	if err := os.MkdirAll( //nolint:gosec // G301: 0755 is standard for user config dirs
		agent.ConfigDir,
		0o755,
	); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	content := mustReadTemplate("templates/agent_instructions.md.tmpl")
	return appendIfAbsent(
		filepath.Join(agent.ConfigDir, "instructions.md"), string(content), "# Locksmith",
	)
}

func (w *AgentWriter) installGeneric() error {
	dir := filepath.Join(w.HomeDir, ".config", "locksmith")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: 0755 is standard for user config dirs
		return fmt.Errorf("creating config dir: %w", err)
	}
	content := mustReadTemplate("templates/agent_instructions.md.tmpl")
	if err := os.WriteFile( //nolint:gosec // G306: documentation, user-readable by design
		filepath.Join(dir, "agent-instructions.md"), content, 0o644,
	); err != nil {
		return fmt.Errorf("writing agent instructions: %w", err)
	}
	return nil
}

// appendIfAbsent appends content to filePath only if marker is not already present.
// This makes the operation idempotent.
func appendIfAbsent(filePath, content, marker string) (retErr error) {
	existing, err := os.ReadFile(filePath) //nolint:gosec // G304: filePath is derived from agent config dir
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}
	if strings.Contains(string(existing), marker) {
		return nil
	}
	prefix := ""
	if len(existing) > 0 {
		prefix = "\n\n"
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644) //nolint:gosec // G302/G304
	if err != nil {
		return fmt.Errorf("opening %s: %w", filePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("closing %s: %w", filePath, cerr)
		}
	}()
	if _, err = f.WriteString(prefix + content); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}
