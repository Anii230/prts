package tui

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"prts/internal/anidb"
	"prts/internal/store"
)

type listKind int

const (
	listTrending listKind = iota
	listHistory
	listWatchlist
)

type (
	listDoneMsg struct{ items []listItem }
	listErrMsg  struct{ err error }
)

type listItem struct {
	result anidb.SearchResult
	entry  store.Entry
}

type listModel struct {
	app     *App
	kind    listKind
	cursor  int
	offset  int
	items   []listItem
	loading bool
	err     error
}

func newList(a *App) *listModel { return &listModel{app: a} }

func (l *listModel) Init() tea.Cmd { return nil }

func (l *listModel) load() tea.Cmd {
	l.loading = true
	l.err = nil
	kind := l.kind
	return func() tea.Msg {
		switch kind {
		case listTrending:
			res, err := anidb.Popular(context.Background())
			if err != nil {
				return listErrMsg{err}
			}
			items := make([]listItem, 0, len(res))
			for _, r := range res {
				items = append(items, listItem{result: r})
			}
			return listDoneMsg{items}
		case listHistory:
			f, err := store.History()
			if err != nil {
				return listErrMsg{err}
			}
			return listDoneMsg{entriesToItems(f.All())}
		case listWatchlist:
			f, err := store.Watchlist()
			if err != nil {
				return listErrMsg{err}
			}
			return listDoneMsg{entriesToItems(f.All())}
		}
		return nil
	}
}

func entriesToItems(es []store.Entry) []listItem {
	items := make([]listItem, 0, len(es))
	for _, e := range es {
		items = append(items, listItem{entry: e, result: anidb.SearchResult{
			SlugID: e.SlugID, ID: e.AnimeID, Title: e.Title,
		}})
	}
	return items
}

func (l *listModel) title() string {
	switch l.kind {
	case listTrending:
		return "Trending"
	case listHistory:
		return "Continue Watching"
	case listWatchlist:
		return "Watchlist"
	}
	return ""
}

func (l *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case listDoneMsg:
		l.loading = false
		l.items = msg.items
		l.cursor = 0
		l.offset = 0
		if len(l.items) == 0 {
			l.err = errNoResults
		}
		return l, nil
	case listErrMsg:
		l.loading = false
		l.err = msg.err
		return l, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return l, tea.Quit
		case "esc":
			return l, l.app.back()
		case "up", "k":
			if len(l.items) > 0 && l.cursor > 0 {
				l.cursor--
				if l.cursor < l.offset {
					l.offset = l.cursor
				}
			}
		case "down", "j":
			if len(l.items) > 0 && l.cursor < len(l.items)-1 {
				l.cursor++
				visible := l.visibleCount()
				if l.cursor >= l.offset+visible {
					l.offset = l.cursor - visible + 1
				}
			}
		case "enter":
			if len(l.items) > 0 {
				it := l.items[l.cursor]
				return l, func() tea.Msg { return openDetailMsg{result: it.result} }
			}
		}
	}
	return l, nil
}

func (l *listModel) visibleCount() int {
	h := l.app.height
	if h == 0 {
		h = 30
	}
	n := h - 8
	if n < 3 {
		n = 3
	}
	return n
}

func (l *listModel) View() string {
	var b strings.Builder
	b.WriteString(Header(l.title(), "Browse catalogue"))

	if l.loading {
		b.WriteString(fmtProgress("◆", "Loading…") + "\n")
		return b.String()
	}
	if l.err != nil {
		b.WriteString(errorStyle.Render("◆ "+l.err.Error()) + "\n")
		return b.String()
	}

	for i := l.offset; i < len(l.items) && i < l.offset+l.visibleCount(); i++ {
		it := l.items[i]
		line := "  " + it.result.Title
		if l.kind != listTrending && it.entry.Episode > 0 {
			line += faintStyle.Render("  ·  last: episode " + intStr(it.entry.Episode))
		}
		if i == l.cursor {
			line = selectedStyle.Render("▸ " + it.result.Title)
			if l.kind != listTrending && it.entry.Episode > 0 {
				line += faintStyle.Render("  ·  last: episode " + intStr(it.entry.Episode))
			}
		}
		b.WriteString(line + "\n")
	}

	if len(l.items) > l.offset+l.visibleCount() {
		b.WriteString(faintStyle.Render("  … more below\n"))
	}
	b.WriteString("\n" + StatusBar("↑/↓ navigate · enter open · esc back"))
	return b.String()
}

func intStr(n int) string {
	return strconv.Itoa(n)
}
