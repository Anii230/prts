package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"prts/internal/anidb"
)

type (
	searchDoneMsg struct{ results []anidb.SearchResult }
	searchErrMsg  struct{ err error }
)

type searchModel struct {
	app     *App
	query   string
	cursor  int
	offset  int
	results []anidb.SearchResult
	loading bool
	err     error
}

func newSearch(a *App) *searchModel { return &searchModel{app: a} }

func (s *searchModel) Init() tea.Cmd { return nil }

func (s *searchModel) doSearch() tea.Cmd {
	query := s.query
	s.loading = true
	s.err = nil
	return func() tea.Msg {
		res, err := anidb.Search(context.Background(), query)
		if err != nil {
			return searchErrMsg{err}
		}
		return searchDoneMsg{res}
	}
}

func (s *searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case searchDoneMsg:
		s.loading = false
		s.results = msg.results
		s.cursor = 0
		s.offset = 0
		if len(s.results) == 0 {
			s.err = errNoResults
		}
		return s, nil

	case searchErrMsg:
		s.loading = false
		s.err = msg.err
		return s, nil

	case tea.KeyMsg:
		return s, s.onKey(msg)
	}
	return s, nil
}

func (s *searchModel) onKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		return s.app.back()
	case "backspace":
		if len(s.query) > 0 {
			s.query = s.query[:len(s.query)-1]
		}
	case "enter":
		if s.loading {
			return nil
		}
		if len(s.results) > 0 {
			return func() tea.Msg { return openDetailMsg{result: s.results[s.cursor]} }
		}
		if strings.TrimSpace(s.query) != "" {
			return s.doSearch()
		}
	case "up", "k":
		if len(s.results) > 0 && s.cursor > 0 {
			s.cursor--
			if s.cursor < s.offset {
				s.offset = s.cursor
			}
		}
	case "down", "j":
		if len(s.results) > 0 && s.cursor < len(s.results)-1 {
			s.cursor++
			visible := s.visibleCount()
			if s.cursor >= s.offset+visible {
				s.offset = s.cursor - visible + 1
			}
		}
	default:
		if k.Type == tea.KeyRunes {
			s.query += string(k.Runes)
		}
	}
	return nil
}

func (s *searchModel) visibleCount() int {
	h := s.app.height
	if h == 0 {
		h = 30
	}
	n := h - 8
	if n < 3 {
		n = 3
	}
	return n
}

func (s *searchModel) View() string {
	var b strings.Builder
	b.WriteString(Header("Search", "Input query and press enter"))

	// Input field
	cursor := " "
	if s.loading {
		cursor = loadingStyle.Render("●")
	}
	field := boxAccentStyle.Render(
		accentStyle.Render("search> ") + s.query + cursor,
	)
	b.WriteString(field)
	b.WriteString("\n\n")

	if s.err != nil {
		b.WriteString(errorStyle.Render("◆ "+s.err.Error()) + "\n")
	}

	for i := s.offset; i < len(s.results) && i < s.offset+s.visibleCount(); i++ {
		r := s.results[i]
		line := "  " + r.Title
		if i == s.cursor {
			line = selectedStyle.Render("▸ " + r.Title)
		}
		b.WriteString(line + "\n")
	}

	if len(s.results) > s.offset+s.visibleCount() {
		b.WriteString(faintStyle.Render("  … more results below\n"))
	}

	b.WriteString("\n" + StatusBar("type to search · enter search/open · ↑/↓ navigate · esc back"))
	return b.String()
}

var errNoResults = &searchError{msg: "no results found"}

type searchError struct{ msg string }

func (e *searchError) Error() string { return e.msg }
