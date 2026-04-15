package initflow

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"

	"github.com/lorem-dev/locksmith/internal/config"
)

// Prompter is the interface for all user-facing interactive prompts.
// The default implementation uses charmbracelet/huh TUI forms; tests can inject
// a mock via InitOptions.Prompter to exercise RunInit without a real TTY.
type Prompter interface {
	ConfigLocation(defaultDir string) (string, error)
	VaultSelection(vaults []DetectedVault) ([]string, error)
	AgentSelection(agents []DetectedAgent) ([]DetectedAgent, error)
	Sandbox() (bool, error)
	Summary(result *InitResult) (bool, error)
}

// InitOptions controls the behaviour of RunInit.
type InitOptions struct {
	NoTUI      bool
	Auto       bool
	AgentOnly  string
	SkipAgents bool
	// Prompter overrides the default huh-based prompts. Nil uses the TUI default.
	// Inject a mock in tests to drive non-auto RunInit flows without a real TTY.
	Prompter Prompter
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

	// Use accessible mode (no TUI) if explicitly requested, TERM=dumb, or non-TTY stdin.
	accessible := opts.NoTUI || os.Getenv("TERM") == "dumb" || !isTerminal()

	var prompter Prompter
	if opts.Prompter != nil {
		prompter = opts.Prompter
	} else {
		prompter = NewHuhPrompter(accessible, nil, nil)
	}

	result := &InitResult{}
	defaultConfigDir := filepath.Join(homeDir, ".config", "locksmith")

	// --- Config location ---
	configDir := defaultConfigDir
	if !opts.Auto {
		configDir, err = prompter.ConfigLocation(defaultConfigDir)
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
		result.SelectedVaults, err = prompter.VaultSelection(detectedVaults)
		if err != nil {
			return nil, err
		}
	}

	// --- Agent selection ---
	if !opts.SkipAgents {
		detectedAgents := DetectAgents(homeDir)
		if opts.AgentOnly != "" {
			for _, a := range detectedAgents {
				if AgentMatches(a.Name, opts.AgentOnly) {
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
			result.SelectedAgents, err = prompter.AgentSelection(detectedAgents)
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
			result.SandboxEnabled, err = prompter.Sandbox()
			if err != nil {
				return nil, err
			}
		}
	}

	// --- Summary + confirmation ---
	if !opts.Auto {
		if ok, err := prompter.Summary(result); err != nil || !ok {
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

// huhPrompter is the production Prompter that drives charmbracelet/huh TUI forms.
type huhPrompter struct {
	accessible bool
	input      io.Reader // nil = os.Stdin
	output     io.Writer // nil = os.Stderr (huh default for TUI output)
}

// NewHuhPrompter creates a Prompter backed by charmbracelet/huh TUI forms.
// Pass nil for input and output to use the OS defaults (os.Stdin / os.Stderr).
// Inject custom readers/writers in tests to simulate user input without a real TTY.
func NewHuhPrompter(accessible bool, input io.Reader, output io.Writer) Prompter {
	return &huhPrompter{accessible: accessible, input: input, output: output}
}

// formWith applies shared I/O options to a form.
func (p *huhPrompter) formWith(f *huh.Form) *huh.Form {
	f = f.WithAccessible(p.accessible)
	if p.input != nil {
		f = f.WithInput(p.input)
	}
	if p.output != nil {
		f = f.WithOutput(p.output)
	}
	return f
}

// ConfigLocation prompts for the config directory.
func (p *huhPrompter) ConfigLocation(defaultDir string) (string, error) {
	var selected string
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Where to store config?").
			Options(
				huh.NewOption(fmt.Sprintf("%s (default)", defaultDir), defaultDir),
				huh.NewOption("Custom path", "custom"),
			).Value(&selected),
	)))
	if err := form.Run(); err != nil {
		return "", err
	}
	if selected != "custom" {
		return selected, nil
	}
	var custom string
	form2 := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Config directory:").Value(&custom),
	)))
	if err := form2.Run(); err != nil {
		return "", err
	}
	return config.ExpandPath(custom), nil
}

// VaultSelection prompts for vault backend selection.
func (p *huhPrompter) VaultSelection(vaults []DetectedVault) ([]string, error) {
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
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Which vault backends do you use?").
			Options(options...).Value(&selected),
	)))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

// AgentSelection prompts which detected agents to configure.
func (p *huhPrompter) AgentSelection(agents []DetectedAgent) ([]DetectedAgent, error) {
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
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Install locksmith for detected agents?").
			Options(
				huh.NewOption(fmt.Sprintf("All detected (%d)", len(detected)), "all"),
				huh.NewOption("Select manually", "manual"),
				huh.NewOption("Skip agent setup", "skip"),
			).Value(&selection),
	)))
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
	form2 := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("Select agents:").Options(options...).Value(&selectedNames),
	)))
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

// Sandbox prompts whether to install sandbox permission allowlists.
func (p *huhPrompter) Sandbox() (bool, error) {
	var enabled bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Allow locksmith commands in agent sandboxes?").
			Description("locksmith get, session start/end, vault list/health").
			Value(&enabled),
	)))
	if err := form.Run(); err != nil {
		return false, err
	}
	return enabled, nil
}

// Summary shows a summary and asks the user to confirm or cancel.
func (p *huhPrompter) Summary(result *InitResult) (bool, error) {
	agentNames := make([]string, len(result.SelectedAgents))
	for i, a := range result.SelectedAgents {
		agentNames[i] = a.Name
	}
	fmt.Printf("\n── Summary ──────────────────────────\nConfig:  %s\nVaults:  %v\nAgents:  %v\nSandbox: %v\n",
		result.ConfigPath, result.SelectedVaults, agentNames, result.SandboxEnabled)

	var confirmed bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Apply?").Value(&confirmed),
	)))
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// AgentMatches returns true if agent name matches the query (case-insensitive).
// "claude" matches "Claude Code" as a convenience alias.
func AgentMatches(name, query string) bool {
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
