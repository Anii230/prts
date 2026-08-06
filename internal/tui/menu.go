package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuModel struct {
	app    *App
	cursor int
}

type menuItem struct {
	label string
	key   string // shortcut hint
	msg   tea.Msg
}

var menuItems = []menuItem{
	{label: "Search", key: "s", msg: openSearchMsg{}},
	{label: "Trending", key: "t", msg: openTrendingMsg{}},
	{label: "Continue Watching", key: "c", msg: openContinueMsg{}},
	{label: "Exit", key: "q", msg: nil},
}

func newMenu(a *App) *menuModel {
	return &menuModel{app: a}
}

func (m *menuModel) Init() tea.Cmd { return nil }

func (m *menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "up", "k":
		m.cursor = (m.cursor - 1 + len(menuItems)) % len(menuItems)
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(menuItems)
	case "enter":
		item := menuItems[m.cursor]
		if item.msg == nil {
			return m, tea.Quit
		}
		return m, func() tea.Msg { return item.msg }
	case "s":
		return m, func() tea.Msg { return openSearchMsg{} }
	case "t":
		return m, func() tea.Msg { return openTrendingMsg{} }
	case "c":
		return m, func() tea.Msg { return openContinueMsg{} }
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *menuModel) View() string {
	var b strings.Builder
	b.WriteString(Header("Command Interface", "v2.0 · Arknights Terminal System"))
	b.WriteString("\n")

	var rows []string
	for i, item := range menuItems {
		label := "  " + item.label
		if i == m.cursor {
			label = selectedStyle.Render("▸ " + item.label)
		} else {
			label = dimStyle.Render(label)
		}
		rows = append(rows, label+"   "+faintStyle.Render(item.key))
	}

	panel := boxStyle.Width(40).Render(strings.Join(rows, "\n"))
	b.WriteString(panel)
	b.WriteString("\n\n")
	b.WriteString(StatusBar("↑/↓ navigate · enter select · s/t/c/q shortcuts · ctrl+c quit"))
	return b.String()
}
