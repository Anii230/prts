package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"prts/internal/anidb"
	"prts/internal/imgrender"
	"prts/internal/store"
)

type (
	detailReadyMsg struct {
		anime *anidb.Anime
	}
	detailImageMsg struct {
		anime *anidb.Anime
		image string
	}
	detailErrMsg struct{ err error }
)

type detailModel struct {
	app      *App
	result   anidb.SearchResult
	anime    *anidb.Anime
	image    string
	imgCols  int
	btnFocus int // 0 watch, 1 add, 2 back
	loading  bool
	err      error
}

func newDetail(a *App) *detailModel { return &detailModel{app: a} }

func (d *detailModel) Init() tea.Cmd { return nil }

func (d *detailModel) reset() {
	d.result = anidb.SearchResult{}
	d.anime = nil
	d.image = ""
	d.loading = false
	d.err = nil
	d.btnFocus = 0
}

func (d *detailModel) ready() bool { return d.anime != nil }

func (d *detailModel) load(result anidb.SearchResult) tea.Cmd {
	d.reset()
	d.result = result
	d.loading = true
	return func() tea.Msg {
		anime, err := anidb.Info(context.Background(), result)
		if err != nil {
			return detailErrMsg{err}
		}
		return detailReadyMsg{anime: anime}
	}
}

// renderImage fetches and renders the cover art, sized to the terminal.
func (d *detailModel) renderImage(cover string) (string, error) {
	if cover == "" {
		return "", nil
	}
	w := d.app.width
	if w == 0 {
		w = 100
	}
	cols := w / 3
	if cols > 34 {
		cols = 34
	}
	if cols < 12 {
		cols = 12
	}
	rows := cols * 3 / 2
	if rows > 34 {
		rows = 34
	}
	d.imgCols = cols
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return imgrender.Render(ctx, cover, cols, rows, "")
}

// fetchImage re-renders the cover in the background; the detail screen stays
// usable while it loads. A nil return means no image is pending.
func (d *detailModel) fetchImage() tea.Cmd {
	if d.anime == nil || d.anime.CoverURL == "" {
		return nil
	}
	anime := d.anime
	return func() tea.Msg {
		img, _ := d.renderImage(anime.CoverURL)
		return detailImageMsg{anime: anime, image: img}
	}
}

func (d *detailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detailReadyMsg:
		d.loading = false
		d.anime = msg.anime
		return d, d.fetchImage()

	case detailImageMsg:
		if msg.anime != nil && d.anime != nil && msg.anime.ID == d.anime.ID {
			d.image = msg.image
		}
		return d, nil

	case detailErrMsg:
		d.loading = false
		d.err = msg.err
		return d, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return d, tea.Quit
		case "esc":
			return d, d.app.back()
		case "left", "h", "shift+tab":
			d.btnFocus = (d.btnFocus - 1 + 3) % 3
		case "right", "l", "tab":
			d.btnFocus = (d.btnFocus + 1) % 3
		case "w":
			d.btnFocus = 0
			return d, d.activate()
		case "a":
			d.btnFocus = 1
			return d, d.activate()
		case "enter":
			return d, d.activate()
		}
	}
	return d, nil
}

// activate runs the action for the currently focused option.
func (d *detailModel) activate() tea.Cmd {
	if d.anime == nil {
		return nil
	}
	switch d.btnFocus {
	case 0: // Watch
		return func() tea.Msg { return openEpisodesMsg{anime: d.anime} }
	case 1: // Add
		return d.addToWatchlist()
	case 2: // Back
		return d.app.back()
	}
	return nil
}

func (d *detailModel) addToWatchlist() tea.Cmd {
	anime := d.anime
	return func() tea.Msg {
		f, err := store.Watchlist()
		if err != nil {
			return toastMsg{text: "failed to save watchlist"}
		}
		if err := f.Add(store.Entry{
			AnimeID: anime.ID,
			SlugID:  anime.SlugID,
			Title:   anime.Title,
			Cover:   anime.CoverURL,
		}); err != nil {
			return toastMsg{text: "failed to save watchlist"}
		}
		return toastMsg{text: "Added to watchlist"}
	}
}

func (d *detailModel) View() string {
	if d.loading {
		return Header("Anime Detail", "") + "\n" + fmtProgress("◆", "Loading metadata…")
	}
	if d.err != nil {
		return Header("Anime Detail", "") + "\n" + errorStyle.Render("◆ "+d.err.Error())
	}
	if d.anime == nil {
		return ""
	}

	w := d.app.width
	if w == 0 {
		w = 100
	}
	imgCols := d.imgCols
	if imgCols == 0 {
		imgCols = 24
	}
	infoW := w - imgCols - 4
	if infoW < 20 {
		infoW = 20
	}

	imgLines := strings.Split(strings.TrimRight(d.image, "\n"), "\n")
	if d.image == "" {
		imgLines = []string{}
	}
	// Right-pad image lines so the columns stay aligned.
	for i := range imgLines {
		runes := []rune(stripANSI(imgLines[i]))
		if len(runes) < imgCols {
			imgLines[i] = imgLines[i] + strings.Repeat(" ", imgCols-len(runes))
		}
	}

	a := d.anime
	var info []string
	info = append(info, accentStyle.Render(a.Title))
	var meta []string
	if a.MALID > 0 {
		meta = append(meta, fmt.Sprintf("MAL %d", a.MALID))
	}
	if len(a.Genres) > 0 {
		meta = append(meta, strings.Join(a.Genres, " · "))
	}
	if len(meta) > 0 {
		info = append(info, dimStyle.Render(strings.Join(meta, "   ")))
	}
	info = append(info, "")
	if a.Description != "" {
		desc := lipgloss.NewStyle().Width(infoW).Render(a.Description)
		for _, ln := range strings.Split(desc, "\n") {
			info = append(info, ln)
		}
	}

	// Options row.
	btns := d.renderButtons()
	for _, ln := range strings.Split(btns, "\n") {
		info = append(info, ln)
	}
	info = append(info, "")
	info = append(info, helpStyle.Render("w watch · a add · esc back"))

	rows := len(imgLines)
	if len(info) > rows {
		rows = len(info)
	}

	var b strings.Builder
	b.WriteString(Header("Anime Detail", a.Title))
	b.WriteString("\n")
	for i := 0; i < rows; i++ {
		left := ""
		if i < len(imgLines) {
			left = imgLines[i]
		} else {
			left = strings.Repeat(" ", imgCols)
		}
		right := ""
		if i < len(info) {
			right = info[i]
		}
		b.WriteString(left + "  " + right + "\n")
	}
	return b.String()
}

// renderButtons renders the action options as plain text with key initials,
// avoiding styled borders entirely (they misbehaved in some terminals).
func (d *detailModel) renderButtons() string {
	opts := []struct{ label, key string }{
		{"Watch", "w"},
		{"Add", "a"},
		{"Back", "esc"},
	}
	var parts []string
	for i, o := range opts {
		label := o.key + "  " + o.label
		if i == d.btnFocus {
			label = "▸ " + label
		} else {
			label = "  " + label
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "    ")
}

// stripANSI removes terminal escape sequences so rune widths are accurate.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
