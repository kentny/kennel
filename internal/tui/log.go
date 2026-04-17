// Package tui holds the thin interaction layer: logging helpers and huh-backed
// prompt wrappers. Nothing in kennel talks to the terminal directly — it goes
// through here so NO_COLOR / non-tty detection are applied in one place.
package tui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dimStyle    = lipgloss.NewStyle().Faint(true)
)

// writer is the destination for log output. Kept as a package-level var so
// tests can swap in a bytes.Buffer, and so subcommands can force-redirect if
// they need structured output elsewhere later.
var writer io.Writer = os.Stderr

// colorEnabled reports whether the log helpers should emit ANSI escapes.
// Respects NO_COLOR (https://no-color.org) and falls back to TTY detection.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if f, ok := writer.(*os.File); ok {
		return isatty.IsTerminal(f.Fd())
	}
	return false
}

// paint applies style iff colors are enabled; plain-text otherwise so logs
// captured to a file stay grep-friendly.
func paint(style lipgloss.Style, s string) string {
	if !colorEnabled() {
		return s
	}
	return style.Render(s)
}

// Info prints a "==> ..." progress line, matching the bash implementation's
// cadence so users migrating from the old version see the same output shape.
func Info(format string, a ...any) {
	fmt.Fprintf(writer, "%s %s\n", paint(cyanStyle, "==>"), fmt.Sprintf(format, a...))
}

// Warn prints a yellow "warn: ..." line.
func Warn(format string, a ...any) {
	fmt.Fprintf(writer, "%s %s\n", paint(yellowStyle, "warn:"), fmt.Sprintf(format, a...))
}

// Error prints a red "error: ..." line. Does not exit — callers decide.
func Error(format string, a ...any) {
	fmt.Fprintf(writer, "%s %s\n", paint(redStyle, "error:"), fmt.Sprintf(format, a...))
}

// Dim prints a faint/grey line. Used for secondary info under a primary line
// (e.g. the summary shown after `kennel init`).
func Dim(format string, a ...any) {
	fmt.Fprintf(writer, "%s\n", paint(dimStyle, fmt.Sprintf(format, a...)))
}
