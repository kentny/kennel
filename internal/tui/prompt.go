package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
)

// Option is a selectable row shared by Select and MultiSelect. Value is what
// the caller receives; Label is what the user sees. Description is rendered
// as secondary text where the form style supports it.
type Option struct {
	Value       string
	Label       string
	Description string
}

// IsTTY reports whether stdin AND stdout are connected to a terminal.
// init flips to non-interactive mode when this returns false so CI pipelines,
// `kennel init | tee log.txt`, and similar work without hanging on a prompt.
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

// Input asks for a single free-form string. The default is pre-filled and
// becomes the returned value if the user hits Enter without edits.
func Input(title, def string) (string, error) {
	val := def
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				Value(&val),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	if val == "" {
		val = def
	}
	return val, nil
}

// Select presents radio-style single-choice. Arrow keys navigate, enter
// confirms. Returns the chosen Value (not Label).
func Select(title string, options []Option) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("Select: no options provided")
	}
	huhOpts := make([]huh.Option[string], len(options))
	for i, o := range options {
		huhOpts[i] = huh.NewOption(o.Label, o.Value)
	}

	var chosen string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Description(describeOptions(options)).
				Options(huhOpts...).
				Value(&chosen),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return chosen, nil
}

// MultiSelect presents a checkbox-style list. Arrow keys move, SPACE toggles,
// enter confirms — the exact convention bash tried and failed to implement.
// `defaults` pre-selects matching options (by Value).
func MultiSelect(title string, options []Option, defaults []string) ([]string, error) {
	huhOpts := make([]huh.Option[string], len(options))
	defSet := make(map[string]struct{}, len(defaults))
	for _, d := range defaults {
		defSet[d] = struct{}{}
	}
	for i, o := range options {
		opt := huh.NewOption(o.Label, o.Value)
		if _, ok := defSet[o.Value]; ok {
			opt = opt.Selected(true)
		}
		huhOpts[i] = opt
	}

	chosen := append([]string{}, defaults...) // seed so huh renders pre-selection
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(title).
				Description("SPACE toggle · ↑↓ move · ENTER confirm").
				Options(huhOpts...).
				Value(&chosen),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return chosen, nil
}

// Confirm is a y/n prompt. Returned bool follows the user's choice, not the
// default — callers should pass the default as the second arg just to
// pre-select the initial focus.
func Confirm(title string, def bool) (bool, error) {
	val := def
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Value(&val),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return val, nil
}

// describeOptions emits a short human-readable summary for Select's
// description slot. huh only supports a single description per Select, not
// per-option, so we fall back to showing the Label → Description map when
// descriptions exist.
func describeOptions(options []Option) string {
	for _, o := range options {
		if o.Description != "" {
			// At least one option has a description — render a compact block.
			out := ""
			for _, o := range options {
				if o.Description != "" {
					if out != "" {
						out += "\n"
					}
					out += "  " + o.Label + ": " + o.Description
				}
			}
			return out
		}
	}
	return ""
}
