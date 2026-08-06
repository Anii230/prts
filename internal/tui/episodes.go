package tui

import (
	"context"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"prts/internal/anidb"
)

type (
	epDoneMsg struct{ eps []anidb.Episode }
	epErrMsg  struct{ err error }
)

type episodeModel struct {
	app      *App
	anime    *anidb.Anime
	image    string // cover art to restore when going back to detail
	episodes []anidb.Episode
	cursor   int
	offset   int
	loading  bool
	err      error
}

func newEpisodes(a *App) *episodeModel { return &episodeModel{app: a} }

func (e *episodeModel) Init() tea.Cmd { return nil }

func (e *episodeModel) load(anime *anidb.Anime) tea.Cmd {
	e.anime = anime
	e.loading = true
	e.err = nil
	e.episodes = nil
	e.cursor = 0
	e.offset = 0
	return func() tea.Msg {
		eps, err := anidb.Episodes(context.Background(), anime.ID)
		if err != nil {
			return epErrMsg{err}
		}
		return epDoneMsg{eps}
	}
}

func (e *episodeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case epDoneMsg:
		e.loading = false
		e.episodes = msg.eps
		return e, nil

	case epErrMsg:
		e.loading = false
		e.err = msg.err
		return e, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return e, tea.Quit
		case "esc":
			return e, e.app.back()
		case "up", "k":
			if len(e.episodes) > 0 && e.cursor > 0 {
				e.cursor--
				if e.cursor < e.offset {
					e.offset = e.cursor
				}
			}
		case "down", "j":
			if len(e.episodes) > 0 && e.cursor < len(e.episodes)-1 {
				e.cursor++
				visible := e.visibleCount()
				if e.cursor >= e.offset+visible {
					e.offset = e.cursor - visible + 1
				}
			}
		case "enter":
			if len(e.episodes) > 0 {
				return e, func() tea.Msg { return playMsg{ep: e.episodes[e.cursor]} }
			}
		}
	}
	return e, nil
}

func (e *episodeModel) visibleCount() int {
	h := e.app.height
	if h == 0 {
		h = 30
	}
	n := h - 7
	if n < 3 {
		n = 3
	}
	return n
}

func (e *episodeModel) View() string {
	if e.loading {
		return Header("Episodes", e.animeTitle()) + "\n" + fmtProgress("◆", "Fetching episode list…")
	}
	if e.err != nil {
		return Header("Episodes", e.animeTitle()) + "\n" + errorStyle.Render("◆ "+e.err.Error())
	}

	var b strings.Builder
	b.WriteString(Header("Episodes", e.animeTitle()))
	b.WriteString("\n")

	for i := e.offset; i < len(e.episodes) && i < e.offset+e.visibleCount(); i++ {
		ep := e.episodes[i]
		line := "  " + strconv.Itoa(ep.Number)
		if ep.Filler {
			line += faintStyle.Render("  (filler)")
		}
		if i == e.cursor {
			line = selectedStyle.Render("▸ ep " + strconv.Itoa(ep.Number))
			if ep.Filler {
				line += faintStyle.Render("  (filler)")
			}
		}
		b.WriteString(line + "\n")
	}

	if len(e.episodes) > e.offset+e.visibleCount() {
		b.WriteString(faintStyle.Render("  … more below\n"))
	}

	b.WriteString("\n" + StatusBar("↑/↓ navigate · enter watch · esc back"))
	return b.String()
}

func (e *episodeModel) animeTitle() string {
	if e.anime == nil {
		return ""
	}
	return e.anime.Title
}
