package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/catalog"
	"github.com/kentny/kennel/internal/discover"
	"github.com/kentny/kennel/internal/tui"
)

//go:embed init_template.yaml
var initTemplate string

type initInputs struct {
	Date            string
	ProjectName     string
	DefaultAgent    string
	Policy          string // "deny" | "allow"
	AllowHosts      []string
	ClaudePlugins   []discover.ClaudePlugin // only populated when agent is claude
	CodexSkills     []discover.CodexSkill
	CodexSkillsNote bool
}

var (
	initFlagPath        string
	initFlagForce       bool
	initFlagYes         bool
	initFlagInteractive bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold .kennel.yaml in the current directory",
	Long: `Scaffold .kennel.yaml.

Runs an interactive TUI by default. In non-TTY environments (CI, piped stdin,
or with --yes) the shipped recommended defaults are applied without prompting:
  - default_policy: deny
  - allow_hosts: the recommended set from the catalog for the chosen agent
  - claude plugins: every plugin discovered under ~/.claude is included
  - codex skills: listed as informational comments in the YAML footer`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initFlagPath, "path", "", "target file (default: ./.kennel.yaml)")
	initCmd.Flags().BoolVarP(&initFlagForce, "force", "f", false, "overwrite existing .kennel.yaml")
	initCmd.Flags().BoolVarP(&initFlagYes, "yes", "y", false, "skip prompts and use recommended defaults")
	initCmd.Flags().BoolVarP(&initFlagInteractive, "interactive", "i", false, "force interactive mode even when stdin is not a TTY")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, _ []string) error {
	target, err := resolveTarget(initFlagPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil && !initFlagForce {
		return fmt.Errorf("%s already exists (use --force to overwrite)", target)
	}

	interactive := decideInteractive()

	inputs := initInputs{Date: time.Now().Format("2006-01-02")}

	// --- Project name ------------------------------------------------------
	suggested := inferProjectName()
	if interactive {
		got, err := tui.Input("Project name", suggested)
		if err != nil {
			return err
		}
		inputs.ProjectName = got
	} else {
		inputs.ProjectName = suggested
	}

	// --- Agent -------------------------------------------------------------
	if flagAgent != "" {
		inputs.DefaultAgent = flagAgent
	} else if interactive {
		got, err := tui.Select("Default agent", []tui.Option{
			{Value: "claude", Label: "claude — Anthropic Claude Code"},
			{Value: "codex", Label: "codex  — OpenAI Codex"},
			{Value: "custom", Label: "custom — other (edit .kennel.yaml after)"},
		})
		if err != nil {
			return err
		}
		inputs.DefaultAgent = got
	} else {
		inputs.DefaultAgent = "claude"
	}

	// --- Network policy ----------------------------------------------------
	if interactive {
		got, err := tui.Select("Network default policy", []tui.Option{
			{Value: "deny", Label: "deny  — block all outbound; pick an allow-list (recommended)"},
			{Value: "allow", Label: "allow — full outbound (quick prototyping, less safe)"},
		})
		if err != nil {
			return err
		}
		inputs.Policy = got
	} else {
		inputs.Policy = "deny"
	}

	// --- Hosts (deny mode only) -------------------------------------------
	if inputs.Policy == "deny" {
		entries := catalog.ForAgent(inputs.DefaultAgent)
		if interactive {
			opts := make([]tui.Option, len(entries))
			defaults := make([]string, 0, len(entries))
			for i, e := range entries {
				label := e.Host
				if e.Description != "" {
					label = fmt.Sprintf("%-30s [%s] %s", e.Host, e.Group, e.Description)
				}
				opts[i] = tui.Option{Value: e.Host, Label: label}
				if e.Recommended {
					defaults = append(defaults, e.Host)
				}
			}
			chosen, err := tui.MultiSelect("Allowed hosts", opts, defaults)
			if err != nil {
				return err
			}
			inputs.AllowHosts = chosen
		} else {
			inputs.AllowHosts = catalog.DefaultsForAgent(inputs.DefaultAgent)
		}
	}

	// --- Claude plugins ----------------------------------------------------
	if inputs.DefaultAgent == "claude" {
		plugins, err := discover.ClaudePlugins()
		if err != nil {
			// Discovery failure is a soft error — init should still succeed,
			// user can manually populate repos later.
			tui.Warn("could not scan ~/.claude for plugins: %v", err)
		}
		// Only offer plugins that actually resolved to a git URL.
		resolvable := make([]discover.ClaudePlugin, 0, len(plugins))
		for _, p := range plugins {
			if p.URL != "" {
				resolvable = append(resolvable, p)
			}
		}

		if interactive && len(resolvable) > 0 {
			opts := make([]tui.Option, len(resolvable))
			defaults := make([]string, 0, len(resolvable))
			for i, p := range resolvable {
				opts[i] = tui.Option{
					Value: p.Name,
					Label: fmt.Sprintf("%s  v%s  (%s)", p.Name, p.Version, p.Marketplace),
				}
				defaults = append(defaults, p.Name) // all preselected
			}
			chosen, err := tui.MultiSelect("Claude plugins to clone into the sandbox", opts, defaults)
			if err != nil {
				return err
			}
			// Translate chosen names back into the full plugin records.
			keep := make(map[string]struct{}, len(chosen))
			for _, n := range chosen {
				keep[n] = struct{}{}
			}
			for _, p := range resolvable {
				if _, ok := keep[p.Name]; ok {
					inputs.ClaudePlugins = append(inputs.ClaudePlugins, p)
				}
			}
		} else {
			// Non-interactive / no discoverable plugins: include everything
			// resolvable. Cloning is idempotent — user can trim later.
			inputs.ClaudePlugins = resolvable
		}
	}

	// --- Codex skills note -------------------------------------------------
	if inputs.DefaultAgent == "codex" {
		if skills, err := discover.CodexSkills(); err == nil && len(skills) > 0 {
			inputs.CodexSkills = skills
			inputs.CodexSkillsNote = true
		}
	}

	if err := renderConfig(target, inputs); err != nil {
		return err
	}

	tui.Info("wrote %s", target)
	tui.Dim("  project.name   = %s", inputs.ProjectName)
	tui.Dim("  default_agent  = %s", inputs.DefaultAgent)
	tui.Dim("  policy         = %s", inputs.Policy)
	if inputs.Policy == "deny" {
		tui.Dim("  allow_hosts    = %d entries", len(inputs.AllowHosts))
	}
	if len(inputs.ClaudePlugins) > 0 {
		tui.Dim("  claude plugins = %d", len(inputs.ClaudePlugins))
	}
	tui.Info("review the file, then run: kennel build && kennel run")
	return nil
}

// resolveTarget returns an absolute path for the `--path` flag. Defaults to
// ./.kennel.yaml relative to CWD.
func resolveTarget(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".kennel.yaml"), nil
}

// decideInteractive follows the priority: --yes > --interactive > TTY check.
func decideInteractive() bool {
	if initFlagYes {
		return false
	}
	if initFlagInteractive {
		return true
	}
	return tui.IsTTY()
}

// inferProjectName picks a sensible default: git remote origin URL (stripping
// `.git`) when available, directory basename otherwise. Whitespace / colons /
// slashes are replaced with hyphens so the value is safe for sandbox names.
func inferProjectName() string {
	name := ""
	if out, err := execOutput("git", "config", "--get", "remote.origin.url"); err == nil {
		url := strings.TrimSpace(out)
		if url != "" {
			last := url[strings.LastIndex(url, "/")+1:]
			name = strings.TrimSuffix(last, ".git")
		}
	}
	if name == "" {
		cwd, _ := os.Getwd()
		name = filepath.Base(cwd)
	}
	return sanitize(name)
}

// sanitize replaces path / whitespace chars so project names flow into
// sandbox names without needing escaping.
func sanitize(s string) string {
	r := strings.NewReplacer(" ", "-", ":", "-", "/", "-")
	return r.Replace(s)
}

// renderConfig executes the embedded template and writes it to target.
func renderConfig(target string, in initInputs) error {
	tmpl, err := template.New("kennel-init").Parse(initTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, in)
}
