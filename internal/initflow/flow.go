package initflow

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/shellhook"
)

// ExistingConfigAction is the user's choice when a config file already exists.
type ExistingConfigAction int

const (
	// ActionContinue keeps the existing file; applyInit skips writing config.yaml.
	ActionContinue ExistingConfigAction = iota
	// ActionOverwrite proceeds through the wizard and replaces the file.
	ActionOverwrite
	// ActionExit cancels init without changes.
	ActionExit
)

// Prompter is the interface for all user-facing interactive prompts.
// The default implementation uses charmbracelet/huh TUI forms; tests can inject
// a mock via InitOptions.Prompter to exercise RunInit without a real TTY.
type Prompter interface {
	// ExistingConfig is called when a config file already exists at path.
	// validErr is nil if the file passes validation, or the validation error otherwise.
	ExistingConfig(path string, validErr error) (ExistingConfigAction, error)
	ConfigLocation(defaultDir string) (string, error)
	VaultSelection(vaults []DetectedVault) ([]string, error)
	AgentSelection(agents []DetectedAgent) ([]DetectedAgent, error)
	Sandbox() (bool, error)
	Summary(result *InitResult) (bool, error)
	// GPGPinentry asks whether to configure locksmith-pinentry in gpg-agent.conf.
	// existingPinentry is the current pinentry-program value (empty if none).
	GPGPinentry(existingPinentry string) (bool, error)
	// ShellHook asks whether to append the daemon autostart snippet to rcFile.
	ShellHook(rcFile string) (bool, error)
	// ClaudeHook asks whether to install the Locksmith UserPromptSubmit hook
	// into settingsPath (~/.claude/settings.json). Shows what will be changed.
	ClaudeHook(settingsPath string) (bool, error)
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
	ConfigPath              string
	SelectedVaults          []string
	SelectedAgents          []DetectedAgent
	SandboxEnabled          bool
	GPGPinentryConfigured   bool            // true if the user opted to configure locksmith-pinentry
	ConfigPreexisted        bool            // true when an existing config was found and kept
	ShellHookInstall        bool            // true if user agreed (or --auto) to install
	ShellHookAlreadyPresent bool            // true if hook marker was already in rc file
	ShellHookRCFile         string          // rc file path; empty when shell is unknown
	ShellHookShell          shellhook.Shell // detected shell
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

	// --- Existing config check ---
	if _, statErr := os.Stat(result.ConfigPath); statErr == nil {
		_, validErr := config.Load(result.ConfigPath)
		var action ExistingConfigAction
		if opts.Auto {
			if validErr == nil {
				action = ActionContinue
			} else {
				action = ActionOverwrite
			}
		} else {
			action, err = prompter.ExistingConfig(result.ConfigPath, validErr)
			if err != nil {
				return nil, err
			}
		}
		switch action {
		case ActionExit:
			return nil, fmt.Errorf("cancelled by user")
		case ActionContinue:
			result.ConfigPreexisted = true
		case ActionOverwrite:
			// fall through: wizard continues normally
		}
	}

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

	// --- GPG pinentry configuration (interactive only, when gopass is selected) ---
	if !opts.Auto {
		gopassSelected := false
		for _, v := range result.SelectedVaults {
			if v == config.VaultGopass {
				gopassSelected = true
				break
			}
		}
		if gopassSelected {
			gnupgDir := filepath.Join(homeDir, ".gnupg")
			existing := ReadExistingPinentry(gnupgDir)
			configure, err := prompter.GPGPinentry(existing)
			if err != nil {
				return nil, err
			}
			result.GPGPinentryConfigured = configure
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

	// Shell hook detection - determine whether to install the daemon autostart hook.
	{
		detectedShell := shellhook.DetectShell()
		rcFile, shellKnown := shellhook.RCFile(detectedShell)
		result.ShellHookShell = detectedShell
		result.ShellHookRCFile = rcFile
		if shellKnown {
			alreadyInstalled, _ := shellhook.IsInstalled(rcFile) // error treated as not installed
			if alreadyInstalled {
				result.ShellHookAlreadyPresent = true
			} else if opts.Auto {
				result.ShellHookInstall = true
			} else {
				var hookErr error
				result.ShellHookInstall, hookErr = prompter.ShellHook(rcFile)
				if hookErr != nil {
					return nil, hookErr
				}
			}
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

	if result.ConfigPreexisted {
		fmt.Printf("  config kept at %s\n", result.ConfigPath)
	} else {
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
	}

	// Apply GPG pinentry configuration if the user opted in.
	if result.GPGPinentryConfigured {
		pinentryPath, lookErr := exec.LookPath("locksmith-pinentry")
		if lookErr != nil {
			fmt.Println("  warning: locksmith-pinentry not found in PATH - run 'make init' first")
		} else {
			gnupgDir := filepath.Join(homeDir, ".gnupg")
			replaced, applyErr := ApplyGPGPinentry(gnupgDir, pinentryPath)
			if applyErr != nil {
				fmt.Printf("  warning: could not update gpg-agent.conf: %v\n", applyErr)
			} else {
				if replaced != "" {
					fmt.Printf("  gpg-agent: previous pinentry-program (%s) commented out\n", replaced)
				}
				fmt.Printf("  gpg-agent: pinentry-program set to %s\n", pinentryPath)
				exec.Command("gpgconf", "--kill", "gpg-agent").Run() //nolint:errcheck
			}
		}
	}

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

	// Apply shell hook.
	if result.ShellHookAlreadyPresent {
		fmt.Printf("  shell hook already installed (%s)\n", result.ShellHookRCFile)
	} else if result.ShellHookInstall {
		if err := shellhook.Install(result.ShellHookRCFile, result.ShellHookShell); err != nil {
			fmt.Printf("  warning: could not write to %s: %v\n", result.ShellHookRCFile, err)
			printShellFallback(result.ShellHookShell, result.ShellHookRCFile)
		} else {
			fmt.Printf("  shell hook added to %s\n", result.ShellHookRCFile)
		}
	} else {
		printShellFallback(result.ShellHookShell, result.ShellHookRCFile)
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

// ExistingConfig prompts the user when a config file already exists at path.
// validErr is nil if the config passed validation, or the error otherwise.
func (p *huhPrompter) ExistingConfig(path string, validErr error) (ExistingConfigAction, error) {
	title := fmt.Sprintf("Config already exists at %s", path)
	var desc string
	continueLabel := "Continue with existing config"
	if validErr == nil {
		desc = "The existing config is valid."
	} else {
		desc = fmt.Sprintf("The existing config is invalid: %v", validErr)
		continueLabel = "Continue with invalid config (not recommended)"
	}
	var selected ExistingConfigAction
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewSelect[ExistingConfigAction]().
			Title(title).
			Description(desc).
			Options(
				huh.NewOption(continueLabel, ActionContinue),
				huh.NewOption("Overwrite with new config", ActionOverwrite),
				huh.NewOption("Exit setup", ActionExit),
			).Value(&selected),
	)))
	if err := form.Run(); err != nil {
		return ActionExit, err
	}
	return selected, nil
}

// ShellHook asks whether to install the daemon autostart hook in rcFile.
func (p *huhPrompter) ShellHook(rcFile string) (bool, error) {
	var confirmed bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Add daemon autostart to shell config?").
			Description(fmt.Sprintf("Appends locksmith daemon autostart to %s", rcFile)).
			Value(&confirmed),
	)))
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// GPGPinentry prompts whether to configure locksmith-pinentry in gpg-agent.conf.
func (p *huhPrompter) GPGPinentry(existingPinentry string) (bool, error) {
	title := "Configure locksmith-pinentry for GPG passphrase prompts?"
	desc := "Required for gopass vault when locksmith runs as a background daemon (no TTY)."
	if existingPinentry != "" {
		desc = fmt.Sprintf(
			"WARNING: your gpg-agent.conf already has pinentry-program = %s\n"+
				"  The existing line will be commented out and replaced.\n"+
				"  You can restore it manually at any time.\n\n"+
				"Configure locksmith-pinentry?",
			existingPinentry)
	}
	var confirmed bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(title).Description(desc).Value(&confirmed),
	)))
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// ClaudeHook asks whether to install the Locksmith hook into settingsPath.
func (p *huhPrompter) ClaudeHook(settingsPath string) (bool, error) {
	var confirmed bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Install Locksmith hook for Claude Code?").
			Description(fmt.Sprintf(
				"Adds a UserPromptSubmit hook to %s.\n"+
					"The hook injects LOCKSMITH_SESSION before each prompt.\n"+
					"Existing settings are preserved.",
				settingsPath,
			)).
			Value(&confirmed),
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

// printShellFallback prints manual instructions for adding the shell hook.
func printShellFallback(s shellhook.Shell, rcFile string) {
	snippet := shellhook.Snippet(s)
	if rcFile != "" {
		fmt.Printf("\nTo start the daemon automatically, add to %s:\n\n  %s\n\n", rcFile, snippet)
	} else {
		fmt.Printf("\nTo start the daemon automatically, add to your shell config:\n\n  %s\n\n", snippet)
	}
}
