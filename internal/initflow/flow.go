package initflow

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"

	"github.com/lorem-dev/locksmith/internal/bundled"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
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
	// BundleExtractPrompt is called when an existing plugin or pinentry file
	// has different content from the bundled version. Returns the user's
	// resolution choice. existingSHA and newSHA are short (8-char) hex
	// strings suitable for display.
	BundleExtractPrompt(name, existingSHA, newSHA string) (bundled.ConflictResolution, error)
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
	ConfigPath               string
	SelectedVaults           []string
	SelectedAgents           []DetectedAgent
	SandboxEnabled           bool
	GPGPinentryConfigured    bool            // true if the user opted to configure locksmith-pinentry
	ConfigPreexisted         bool            // true when an existing config was found and kept
	ShellHookInstall         bool            // true if user agreed (or --auto) to install
	ShellHookAlreadyPresent  bool            // true if hook marker was already in rc file
	ShellHookRCFile          string          // rc file path; empty when shell is unknown
	ShellHookShell           shellhook.Shell // detected shell
	ClaudeHookConfirmed      bool            // user approved (or --auto); set in RunInit before applyInit
	ClaudeHookInstalled      bool            // hook was written successfully; set in applyInit
	ClaudeHookAlreadyPresent bool            // hook was already in settings.json; install skipped
}

var (
	fmtTitle    = color.New(color.Bold)
	fmtPaths    = color.New(color.FgBlue)
	fmtErrors   = color.New(color.FgRed)
	fmtLists    = color.New(color.FgCyan)
	fmtBooleans = color.New(color.FgMagenta)
)

// DetectVaultsFnType is the function signature for detecting vault backends.
type DetectVaultsFnType func() []DetectedVault

// DetectVaultsFn is the function used to detect vault backends. Replaced in
// tests to inject a stub without touching the real filesystem.
var DetectVaultsFn DetectVaultsFnType = DetectVaults

// RunInit runs the interactive setup wizard. In --auto mode all prompts are
// skipped and detected defaults are applied. In --no-tui mode huh's accessible
// mode is used (plain text prompts), which also activates automatically when
// TERM=dumb or stdin is not a TTY.
//
//nolint:gocognit // orchestration function; complexity is inherent in the init wizard flow
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
			return nil, fmt.Errorf("selecting config location: %w", err)
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
				return nil, fmt.Errorf("prompting for existing config: %w", err)
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
	if err = selectVaults(result, opts, prompter); err != nil {
		return nil, err
	}

	// --- GPG pinentry configuration (interactive only, when gopass is selected) ---
	if !opts.Auto {
		if err = selectGPGPinentry(result, prompter, homeDir); err != nil {
			return nil, err
		}
	}

	// --- Agent selection ---
	if !opts.SkipAgents {
		if err = selectAgents(result, opts, prompter, homeDir); err != nil {
			return nil, err
		}
	}

	// --- Sandbox permissions ---
	if len(result.SelectedAgents) > 0 {
		if opts.Auto {
			result.SandboxEnabled = true
		} else {
			result.SandboxEnabled, err = prompter.Sandbox()
			if err != nil {
				return nil, fmt.Errorf("prompting for sandbox: %w", err)
			}
		}
	}

	// --- Summary + confirmation ---
	if !opts.Auto {
		ok, summaryErr := prompter.Summary(result)
		if summaryErr != nil {
			return nil, fmt.Errorf("showing summary: %w", summaryErr)
		}
		if !ok {
			return nil, fmt.Errorf("cancelled by user")
		}
	}

	// Shell hook detection - determine whether to install the daemon autostart hook.
	if err = detectShellHookConsent(result, opts, prompter); err != nil {
		return nil, err
	}

	// --- Claude Code hook installation consent ---
	if err = detectClaudeHookConsent(result, opts, prompter, homeDir); err != nil {
		return nil, err
	}

	if err := applyInit(result, homeDir, prompter, opts.Auto); err != nil {
		return nil, err
	}
	return result, nil
}

// selectVaults fills result.SelectedVaults based on detection and prompting.
func selectVaults(result *InitResult, opts InitOptions, prompter Prompter) error {
	detectedVaults := DetectVaultsFn()
	if opts.Auto {
		for _, v := range detectedVaults {
			if v.Detected && v.Implemented {
				result.SelectedVaults = append(result.SelectedVaults, v.Type)
			}
		}
		return nil
	}
	var err error
	result.SelectedVaults, err = prompter.VaultSelection(detectedVaults)
	if err != nil {
		return fmt.Errorf("selecting vaults: %w", err)
	}
	return nil
}

// selectGPGPinentry prompts for GPG pinentry configuration when gopass is selected.
func selectGPGPinentry(result *InitResult, prompter Prompter, homeDir string) error {
	gopassSelected := false
	for _, v := range result.SelectedVaults {
		if v == config.VaultGopass {
			gopassSelected = true
			break
		}
	}
	if !gopassSelected {
		return nil
	}
	gnupgDir := filepath.Join(homeDir, ".gnupg")
	existing := ReadExistingPinentry(gnupgDir)
	configure, err := prompter.GPGPinentry(existing)
	if err != nil {
		return fmt.Errorf("prompting for GPG pinentry: %w", err)
	}
	result.GPGPinentryConfigured = configure
	return nil
}

// selectAgents fills result.SelectedAgents based on detection, auto mode, or prompting.
func selectAgents(result *InitResult, opts InitOptions, prompter Prompter, homeDir string) error {
	detectedAgents := DetectAgents(homeDir)
	switch {
	case opts.AgentOnly != "":
		for _, a := range detectedAgents {
			if AgentMatches(a.Name, opts.AgentOnly) {
				result.SelectedAgents = append(result.SelectedAgents, a)
			}
		}
	case opts.Auto:
		for _, a := range detectedAgents {
			if a.Detected {
				result.SelectedAgents = append(result.SelectedAgents, a)
			}
		}
	default:
		var err error
		result.SelectedAgents, err = prompter.AgentSelection(detectedAgents)
		if err != nil {
			return fmt.Errorf("selecting agents: %w", err)
		}
	}
	return nil
}

// detectShellHookConsent determines whether to install the daemon autostart hook.
func detectShellHookConsent(result *InitResult, opts InitOptions, prompter Prompter) error {
	detectedShell := shellhook.DetectShell()
	rcFile, shellKnown := shellhook.RCFile(detectedShell)
	result.ShellHookShell = detectedShell
	result.ShellHookRCFile = rcFile
	if !shellKnown {
		return nil
	}
	alreadyInstalled := false
	if ok, isInstalledErr := shellhook.IsInstalled(rcFile); isInstalledErr == nil {
		alreadyInstalled = ok
	}
	switch {
	case alreadyInstalled:
		result.ShellHookAlreadyPresent = true
	case opts.Auto:
		result.ShellHookInstall = true
	default:
		var err error
		result.ShellHookInstall, err = prompter.ShellHook(rcFile)
		if err != nil {
			return fmt.Errorf("prompting for shell hook: %w", err)
		}
	}
	return nil
}

// detectClaudeHookConsent checks if Claude Code is selected and asks for hook install consent.
func detectClaudeHookConsent(result *InitResult, opts InitOptions, prompter Prompter, homeDir string) error {
	claudeSettingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	for _, agent := range result.SelectedAgents {
		if agent.Name != "Claude Code" {
			continue
		}
		installer := NewClaudeHookInstaller(
			filepath.Join(homeDir, ".config", "locksmith"),
			filepath.Join(homeDir, ".claude"),
		)
		switch {
		case installer.IsInstalled():
			result.ClaudeHookAlreadyPresent = true
		case opts.Auto:
			result.ClaudeHookConfirmed = true
		default:
			var err error
			result.ClaudeHookConfirmed, err = prompter.ClaudeHook(claudeSettingsPath)
			if err != nil {
				return fmt.Errorf("prompting for Claude hook: %w", err)
			}
		}
		break // only one Claude Code entry possible
	}
	return nil
}

// ExtractBundled writes the plugins required by selectedVaults plus
// locksmith-pinentry from the embedded bundle to their canonical paths.
// Returns nil and prints a warning if the bundle is empty (dev build).
func ExtractBundled(selectedVaults []string, prompter Prompter, auto bool) error {
	bundle, err := bundled.OpenBundle()
	if err != nil {
		if errors.Is(err, bundled.ErrEmptyBundle) {
			log.Warn().Msg("this build has no bundled plugins; install plugins manually or run `make build-all`")
			return nil
		}
		return fmt.Errorf("opening bundle: %w", err)
	}
	pluginsDir, err := bundled.PluginsDir()
	if err != nil {
		return fmt.Errorf("resolving plugins dir: %w", err)
	}
	pinentryPath, err := bundled.PinentryPath()
	if err != nil {
		return fmt.Errorf("resolving pinentry path: %w", err)
	}
	names := []string{"locksmith-pinentry"}
	for _, v := range selectedVaults {
		names = append(names, "locksmith-plugin-"+v)
	}
	var p bundled.ExtractPrompter
	if !auto {
		p = prompter
	}
	if err := bundled.Extract(bundle, bundled.ExtractOptions{
		Names:        names,
		PluginsDir:   pluginsDir,
		PinentryPath: pinentryPath,
		Prompter:     p,
		OnKept: func(name string, withWarning bool) {
			if withWarning {
				log.Warn().Str("entry", name).
					Msg("kept; bundled version differs - functionality may not work as expected")
			}
		},
		OnExtracted: func(name string) {
			fmt.Printf("  Binary extracted from bundle: %s\n", name)
		},
	}); err != nil {
		return fmt.Errorf("extracting bundled entries: %w", err)
	}
	return nil
}

func applyInit(result *InitResult, homeDir string, prompter Prompter, auto bool) error {
	if err := os.MkdirAll(filepath.Dir(result.ConfigPath), 0o755); err != nil { //nolint:gosec // G301: user config dir
		return fmt.Errorf("creating config dir: %w", err)
	}

	if err := ExtractBundled(result.SelectedVaults, prompter, auto); err != nil {
		return fmt.Errorf("extracting bundled plugins: %w", err)
	}

	if err := writeConfigFile(result, homeDir); err != nil {
		return err
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

		if agent.Name == "Claude Code" {
			if err := applyClaudeHook(result, homeDir); err != nil {
				return err
			}
		}
	}

	applyShellHook(result)
	return nil
}

// writeConfigFile writes the YAML config or skips if it pre-existed.
func writeConfigFile(result *InitResult, homeDir string) error {
	if result.ConfigPreexisted {
		fmt.Printf("  config kept at %s\n", fmtPaths.Sprint(result.ConfigPath))
		return nil
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
	if err := os.WriteFile(result.ConfigPath, data, 0o644); err != nil { //nolint:gosec // G306: user config file
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("  config written to %s\n", result.ConfigPath)

	// Apply GPG pinentry configuration if the user opted in.
	if result.GPGPinentryConfigured {
		applyGPGPinentryConfig(homeDir)
	}
	return nil
}

// applyGPGPinentryConfig runs the gpg-agent pinentry configuration steps.
func applyGPGPinentryConfig(homeDir string) {
	pinentryPath, pathErr := bundled.PinentryPath()
	if pathErr != nil {
		fmt.Printf("  warning: could not resolve pinentry path: %v\n", pathErr)
		return
	}
	if _, statErr := os.Stat(pinentryPath); errors.Is(statErr, fs.ErrNotExist) {
		fmt.Printf("  warning: %s not found - run `locksmith init` to extract"+
			" or `make build-all` for a non-empty bundle\n", pinentryPath)
		return
	} else if statErr != nil {
		fmt.Printf("  warning: stat %s: %v\n", pinentryPath, statErr)
		return
	}
	gnupgDir := filepath.Join(homeDir, ".gnupg")
	replaced, applyErr := ApplyGPGPinentry(gnupgDir, pinentryPath)
	if applyErr != nil {
		fmt.Printf("  warning: could not update gpg-agent.conf: %v\n", applyErr)
		return
	}
	if replaced != "" {
		fmt.Printf("  gpg-agent: previous pinentry-program (%s) commented out\n", replaced)
	}
	fmt.Printf("  gpg-agent: pinentry-program set to %s\n", pinentryPath)
	exec.Command("gpgconf", "--kill", "gpg-agent").Run() //nolint:errcheck
}

// applyClaudeHook installs or reports status of the Claude Code UserPromptSubmit hook.
func applyClaudeHook(result *InitResult, homeDir string) error {
	installer := NewClaudeHookInstaller(
		filepath.Join(homeDir, ".config", "locksmith"),
		filepath.Join(homeDir, ".claude"),
	)
	switch {
	case result.ClaudeHookAlreadyPresent:
		fmt.Printf("  Claude Code: hook already present in %s\n", fmtPaths.Sprint("~/.claude/settings.json"))
	case result.ClaudeHookConfirmed:
		if err := installer.Install(); err != nil {
			return fmt.Errorf("installing Claude Code hook: %w", err)
		}
		result.ClaudeHookInstalled = true
		fmt.Println("  Claude Code: hook installed in ~/.claude/settings.json")
		fmt.Println("  Restart Claude Code for the hook to take effect.")
	default:
		hookCmd := filepath.Join(homeDir, ".config", "locksmith", "agent-hook.sh")
		fmt.Printf("\n  To install the Claude Code hook manually:\n")
		fmt.Printf("  1. Run: locksmith init --agent claude\n")
		fmt.Printf("  2. Add to ~/.claude/settings.json:\n")
		fmt.Printf(
			"     {\"hooks\":{\"UserPromptSubmit\":[{\"matcher\":\"\",\"hooks\":[{\"type\":\"command\",\"command\":%q}]}]}}\n",
			hookCmd,
		)
	}
	return nil
}

// applyShellHook installs or reports status of the shell daemon autostart hook.
func applyShellHook(result *InitResult) {
	switch {
	case result.ShellHookAlreadyPresent:
		fmt.Printf("  shell hook already installed (%s)\n", fmtPaths.Sprint(result.ShellHookRCFile))
	case result.ShellHookInstall:
		if err := shellhook.Install(result.ShellHookRCFile, result.ShellHookShell); err != nil {
			fmt.Printf("  warning: could not write to %s: %v\n", fmtPaths.Sprint(result.ShellHookRCFile), err)
			printShellFallback(result.ShellHookShell, result.ShellHookRCFile)
		} else {
			fmt.Printf("  shell hook added to %s\n", fmtPaths.Sprint(result.ShellHookRCFile))
		}
	default:
		printShellFallback(result.ShellHookShell, result.ShellHookRCFile)
	}
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
		return "", fmt.Errorf("selecting config location: %w", err)
	}
	if selected != "custom" {
		return selected, nil
	}
	var custom string
	form2 := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Config directory:").Value(&custom),
	)))
	if err := form2.Run(); err != nil {
		return "", fmt.Errorf("entering custom config path: %w", err)
	}
	return config.ExpandPath(custom), nil
}

// VaultSelection prompts for vault backend selection.
func (p *huhPrompter) VaultSelection(vaults []DetectedVault) ([]string, error) {
	var implemented, planned []DetectedVault
	for _, v := range vaults {
		if v.Implemented {
			implemented = append(implemented, v)
		} else {
			planned = append(planned, v)
		}
	}
	if len(implemented) == 0 {
		return nil, fmt.Errorf("no implemented vault backends available on this platform")
	}

	options := make([]huh.Option[string], 0, len(implemented))
	for _, v := range implemented {
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
	for _, v := range implemented {
		if v.Detected && v.Available {
			selected = append(selected, v.Type)
		}
	}

	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Which vault backends do you use?").
			Description(plannedNote(planned)).
			Options(options...).Value(&selected),
	)))
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("selecting vaults: %w", err)
	}
	return selected, nil
}

// plannedNote formats a description listing planned vault backends.
// Returns empty string when planned is empty (huh shows no description).
func plannedNote(planned []DetectedVault) string {
	if len(planned) == 0 {
		return ""
	}
	labels := make([]string, len(planned))
	for i, v := range planned {
		labels[i] = plannedLabel(v)
	}
	return "Planned (not yet supported): " + strings.Join(labels, ", ") + "."
}

func plannedLabel(v DetectedVault) string {
	if v.PlatformNote != "" {
		return v.Type + " (" + v.PlatformNote + ")"
	}
	return v.Type
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
		return nil, fmt.Errorf("selecting agents: %w", err)
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
		return nil, fmt.Errorf("selecting agents manually: %w", err)
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
		return false, fmt.Errorf("prompting for sandbox: %w", err)
	}
	return enabled, nil
}

// Summary shows a summary and asks the user to confirm or cancel.
func (p *huhPrompter) Summary(result *InitResult) (bool, error) {
	agentNames := make([]string, len(result.SelectedAgents))
	for i, a := range result.SelectedAgents {
		agentNames[i] = a.Name
	}

	fmt.Println(fmtTitle.Sprint("-- Summary --"))

	fmt.Printf("%s %s\n", fmtTitle.Sprint("Config: "), fmtPaths.Sprint(result.ConfigPath))
	fmt.Printf("%s %s\n", fmtTitle.Sprint("Vaults: "), fmtLists.Sprint(result.SelectedVaults))
	fmt.Printf("%s %s\n", fmtTitle.Sprint("Agents: "), fmtLists.Sprint(agentNames))
	fmt.Printf("%s %s\n", fmtTitle.Sprint("Sandbox:  "), fmtBooleans.Sprint(result.SandboxEnabled))

	var confirmed bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Apply?").Value(&confirmed),
	)))
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("showing summary: %w", err)
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
		desc = fmt.Sprintf("The existing config is invalid: %v", fmtErrors.Sprint(validErr))
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
		return ActionExit, fmt.Errorf("prompting for existing config: %w", err)
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
		return false, fmt.Errorf("prompting for shell hook: %w", err)
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
			existingPinentry,
		)
	}
	var confirmed bool
	form := p.formWith(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(title).Description(desc).Value(&confirmed),
	)))
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("prompting for GPG pinentry: %w", err)
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
		return false, fmt.Errorf("prompting for Claude hook: %w", err)
	}
	return confirmed, nil
}

// BundleExtractPrompt asks the user how to resolve a sha256 mismatch on an
// already-extracted plugin or pinentry binary.
func (p *huhPrompter) BundleExtractPrompt(name, existingSHA, newSHA string) (bundled.ConflictResolution, error) {
	if p.accessible {
		return bundled.Keep, nil
	}
	var choice string
	prompt := fmt.Sprintf(
		"Existing %s differs from bundled (on disk %s vs bundled %s). Overwrite?",
		name, bundled.ShortSHA(existingSHA), bundled.ShortSHA(newSHA),
	)
	form := p.formWith(huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(prompt).
				Options(
					huh.NewOption("Overwrite this one", "y"),
					huh.NewOption("Keep this one", "n"),
					huh.NewOption("Overwrite all remaining", "all"),
					huh.NewOption("Keep all remaining", "skip"),
				).
				Value(&choice),
		),
	))
	if err := form.Run(); err != nil {
		return bundled.Keep, fmt.Errorf("BundleExtractPrompt: %w", err)
	}
	switch choice {
	case "y":
		return bundled.Overwrite, nil
	case "n":
		return bundled.Keep, nil
	case "all":
		return bundled.OverwriteAll, nil
	case "skip":
		return bundled.KeepAll, nil
	default:
		return bundled.Keep, nil
	}
}

// AgentMatches returns true if agent name matches the query (case-insensitive).
// "claude" matches "Claude Code" as a convenience alias.
func AgentMatches(name, query string) bool {
	m := func(s string) string {
		b := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c >= 'A' && c <= 'Z' {
				c += 32 // ASCII offset from uppercase to lowercase
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
		fmt.Printf("\nTo start the daemon automatically, add to %s:\n\n  %s\n\n", fmtPaths.Sprint(rcFile), snippet)
	} else {
		fmt.Printf("\nTo start the daemon automatically, add to your shell config:\n\n  %s\n\n", snippet)
	}
}
