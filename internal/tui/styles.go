// Package tui implements the PRTS terminal interface. All styling is done with
// terminal text attributes only (bold, faint, borders) and deliberately avoids
// custom colors so the app follows the user's terminal theme (including
// transparency).
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true)

	subtitleStyle = lipgloss.NewStyle()

	dimStyle = lipgloss.NewStyle()

	faintStyle = lipgloss.NewStyle().
			Faint(true)

	accentStyle = lipgloss.NewStyle().
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	boxAccentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Faint(true)

	errorStyle = lipgloss.NewStyle().
			Bold(true)

	loadingStyle = lipgloss.NewStyle().
			Bold(true)
)

// banner returns the PRTS logotype as block letters.
func banner() string {
	art := `
  ██████╗ ██████╗ ████████╗███████╗
  ██╔══██╗██╔══██╗╚══██╔══╝██╔════╝
  ██████╔╝██████╔╝   ██║   ███████╗
  ██╔═══╝ ██╔══██╗   ██║   ╚════██║
  ██║     ██║  ██║   ██║   ███████║
  ╚═╝     ╚═╝  ╚═╝   ╚═╝   ╚══════╝`
	return art
}

// Header renders the PRTS banner with a title bar.
func Header(title, status string) string {
	b := banner()
	var sb strings.Builder
	for i, line := range strings.Split(b, "\n") {
		if i == 0 {
			sb.WriteString(titleStyle.Render("PRTS"))
			sb.WriteString("  ")
			sb.WriteString(accentStyle.Render(title))
		} else {
			sb.WriteString(strings.Repeat(" ", 2))
		}
		sb.WriteString(" ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	out := sb.String()
	if status != "" {
		out += subtitleStyle.Render(status) + "\n"
	}
	return out
}

// StatusBar renders a footer with the given hint text.
func StatusBar(hint string) string {
	return helpStyle.Render(hint)
}

// Fill pads every line with spaces so the alt screen is consistently sized.
// No background color is painted: the terminal's own background (and
// transparency) shows through.
func Fill(content string, width, height int) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	filled := make([]string, 0, height)
	for _, l := range lines {
		w := ansi.StringWidth(l)
		if w > width {
			// Keep at least width cells; Truncate is ANSI-safe.
			l = ansi.Truncate(l, width, "")
			w = ansi.StringWidth(l)
		}
		if w < width {
			l += strings.Repeat(" ", width-w)
		}
		filled = append(filled, l)
	}
	for len(filled) < height {
		filled = append(filled, strings.Repeat(" ", width))
	}
	return strings.Join(filled, "\n")
}

// fmtProgress formats a busy/progress line.
func fmtProgress(frame, text string) string {
	return loadingStyle.Render(frame) + "  " + dimStyle.Render(text)
}
