package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"prts/internal/anidb"
	"prts/internal/player"
	"prts/internal/store"
)

// Config carries playback options set on the command line.
type Config struct {
	Quality  string
	Dub      bool
	Download bool
	Player   string
}

// PlayRequest describes one episode the user chose to watch.
type PlayRequest struct {
	Anime   *anidb.Anime
	Episode anidb.Episode
	Lang    string
}

// Result is what the TUI hands back to main when it exits.
type Result struct {
	Error error
}

type screen int

const (
	screenMenu screen = iota
	screenSearch
	screenTrending
	screenContinue
	screenWatchlist
	screenDetail
	screenEpisodes
)

// Navigation and playback messages sent between models.
type (
	openSearchMsg    struct{}
	openTrendingMsg  struct{}
	openContinueMsg  struct{}
	openWatchlistMsg struct{}
	openDetailMsg    struct{ result anidb.SearchResult }
	openEpisodesMsg  struct{ anime *anidb.Anime }
	backMsg          struct{}
	toastMsg         struct{ text string }
	clearToastMsg    struct{}
	playMsg          struct{ ep anidb.Episode }

	resolveDoneMsg struct {
		req     *PlayRequest
		url     string
		quality string
	}
	resolveErrMsg struct{ err error }
	playerDoneMsg struct{ err error }
)

// playState tracks the episode playback lifecycle inside the TUI.
type playState int

const (
	playIdle playState = iota
	playResolving
	playRunning
	playPost
)

// App is the root bubbletea model that routes between screens.
type App struct {
	cfg    Config
	width  int
	height int
	screen screen

	initialQuery string
	detailFrom   screen

	menu   *menuModel
	search *searchModel
	list   *listModel
	detail *detailModel
	ep     *episodeModel

	playState   playState
	pending     *PlayRequest
	lastQuality string
	lastErr     error
	postFocus   int

	result *Result
	toast  string
}

// New builds the root app. initialQuery pre-fills the search box.
func New(cfg Config, initialQuery string) *App {
	a := &App{
		cfg:          cfg,
		initialQuery: initialQuery,
		screen:       screenMenu,
	}
	a.menu = newMenu(a)
	a.search = newSearch(a)
	a.list = newList(a)
	a.detail = newDetail(a)
	a.ep = newEpisodes(a)
	return a
}

// Run starts the TUI and returns the final result.
func (a *App) Run() *Result {
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()
	if err != nil {
		return &Result{Error: err}
	}
	if app, ok := final.(*App); ok && app.result != nil {
		return app.result
	}
	return &Result{}
}

func (a *App) Init() tea.Cmd {
	if a.initialQuery != "" {
		a.screen = screenSearch
		a.search.query = a.initialQuery
		a.search.results = nil
		return a.search.doSearch()
	}
	a.screen = screenMenu
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.detail.ready() {
			a.detail.image = ""
			cmds = append(cmds, a.detail.fetchImage())
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.playState == playPost {
			return a, a.postPlayUpdate(msg)
		}
		if msg.String() == "esc" && a.screen != screenMenu {
			cmds = append(cmds, a.back())
		}

	case openSearchMsg:
		a.screen = screenSearch
		a.search.query = ""
		a.search.results = nil
		return a, tea.Batch(cmds...)

	case openTrendingMsg:
		a.screen = screenTrending
		a.list.kind = listTrending
		cmds = append(cmds, a.list.load())
		return a, tea.Batch(cmds...)

	case openContinueMsg:
		a.screen = screenContinue
		a.list.kind = listHistory
		cmds = append(cmds, a.list.load())
		return a, tea.Batch(cmds...)

	case openWatchlistMsg:
		a.screen = screenWatchlist
		a.list.kind = listWatchlist
		cmds = append(cmds, a.list.load())
		return a, tea.Batch(cmds...)

	case openDetailMsg:
		a.detailFrom = a.screen
		a.screen = screenDetail
		a.detail.reset()
		cmds = append(cmds, a.detail.load(msg.result))
		return a, tea.Batch(cmds...)

	case openEpisodesMsg:
		a.ep.image = a.detail.image
		a.screen = screenEpisodes
		cmds = append(cmds, a.ep.load(msg.anime))
		return a, tea.Batch(cmds...)

	case playMsg:
		return a, a.startPlay(msg.ep)

	case resolveDoneMsg:
		a.playState = playRunning
		a.pending = msg.req
		a.lastQuality = msg.quality
		if a.cfg.Download {
			return a, a.runDownload(msg)
		}
		return a, a.runPlayer(msg)

	case resolveErrMsg:
		a.playState = playIdle
		a.lastErr = msg.err
		cmds = append(cmds, a.toastCmd("failed to resolve stream: "+msg.err.Error()))
		return a, tea.Batch(cmds...)

	case playerDoneMsg:
		if a.cfg.Download {
			a.playState = playIdle
			if msg.err != nil {
				cmds = append(cmds, a.toastCmd("download failed: "+msg.err.Error()))
			} else {
				cmds = append(cmds, a.toastCmd("downloaded"))
			}
			return a, tea.Batch(cmds...)
		}
		recordHistory(a.pending)
		a.playState = playPost
		a.lastErr = msg.err
		a.postFocus = 0
		return a, tea.Batch(cmds...)

	case toastMsg:
		a.toast = msg.text
		cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearToastMsg{}
		}))
		return a, tea.Batch(cmds...)

	case clearToastMsg:
		a.toast = ""
		return a, tea.Batch(cmds...)
	}

	// While a stream is being resolved, ignore navigation input.
	if a.playState == playResolving {
		return a, tea.Batch(cmds...)
	}

	// Delegate to the active screen.
	switch a.screen {
	case screenMenu:
		m, c := a.menu.Update(msg)
		a.menu = m.(*menuModel)
		cmds = append(cmds, c)
	case screenSearch:
		m, c := a.search.Update(msg)
		a.search = m.(*searchModel)
		cmds = append(cmds, c)
	case screenTrending, screenContinue, screenWatchlist:
		m, c := a.list.Update(msg)
		a.list = m.(*listModel)
		cmds = append(cmds, c)
	case screenDetail:
		m, c := a.detail.Update(msg)
		a.detail = m.(*detailModel)
		cmds = append(cmds, c)
	case screenEpisodes:
		m, c := a.ep.Update(msg)
		a.ep = m.(*episodeModel)
		cmds = append(cmds, c)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) View() string {
	var v string
	switch {
	case a.playState == playPost:
		v = a.postPlayView()
	case a.playState == playResolving:
		v = a.resolvingView()
	default:
		switch a.screen {
		case screenMenu:
			v = a.menu.View()
		case screenSearch:
			v = a.search.View()
		case screenTrending, screenContinue, screenWatchlist:
			v = a.list.View()
		case screenDetail:
			v = a.detail.View()
		case screenEpisodes:
			v = a.ep.View()
		}
	}

	if a.toast != "" {
		v += "\n" + accentStyle.Render("◆ "+a.toast)
	}

	w, h := a.width, a.height
	if w == 0 {
		w = 100
	}
	if h == 0 {
		h = 30
	}
	return Fill(v, w, h)
}

// back navigates to the previous screen.
func (a *App) back() tea.Cmd {
	switch a.screen {
	case screenSearch, screenTrending, screenContinue, screenWatchlist:
		a.screen = screenMenu
	case screenDetail:
		a.screen = a.detailFrom
	case screenEpisodes:
		// Restore the detail screen state without refetching.
		a.detail.anime = a.ep.anime
		a.detail.image = a.ep.image
		a.detail.loading = false
		a.detail.err = nil
		a.screen = screenDetail
	}
	return nil
}

func langFor(cfg Config) string {
	if cfg.Dub {
		return "eng"
	}
	return "jpn"
}

// toastCmd returns a command that surfaces a transient status message.
func (a *App) toastCmd(text string) tea.Cmd {
	return func() tea.Msg { return toastMsg{text: text} }
}

// ---- Playback ----

// startPlay resolves and plays the given episode inside the TUI.
func (a *App) startPlay(ep anidb.Episode) tea.Cmd {
	a.pending = &PlayRequest{Anime: a.ep.anime, Episode: ep, Lang: langFor(a.cfg)}
	a.playState = playResolving
	a.lastErr = nil
	return a.resolveCmd(a.pending)
}

// resolveCmd resolves embed -> master -> quality for one episode.
func (a *App) resolveCmd(req *PlayRequest) tea.Cmd {
	pref := a.cfg.Quality
	return func() tea.Msg {
		embed, err := anidb.EmbedURL(context.Background(), req.Episode.ID, req.Lang)
		if err != nil {
			return resolveErrMsg{err}
		}
		master, err := anidb.MasterPlaylist(context.Background(), embed)
		if err != nil {
			return resolveErrMsg{err}
		}
		quals, err := anidb.Qualities(context.Background(), master)
		if err != nil {
			return resolveErrMsg{err}
		}
		q, err := anidb.PickQuality(quals, pref)
		if err != nil {
			return resolveErrMsg{err}
		}
		return resolveDoneMsg{req: req, url: q.URL, quality: q.Label}
	}
}

// runPlayer launches the external player, releasing the terminal for its
// duration. The TUI resumes when the player exits.
func (a *App) runPlayer(msg resolveDoneMsg) tea.Cmd {
	cmd := player.Command(a.cfg.Player, mediaTitle(msg.req), msg.url)
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return playerDoneMsg{err} })
}

// runDownload downloads the stream via yt-dlp/ffmpeg, releasing the terminal
// for its duration.
func (a *App) runDownload(msg resolveDoneMsg) tea.Cmd {
	cmd, err := player.DownloadCommand(msg.url, ".", mediaTitle(msg.req))
	if err != nil {
		return func() tea.Msg { return playerDoneMsg{err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return playerDoneMsg{err} })
}

func mediaTitle(req *PlayRequest) string {
	return fmt.Sprintf("%s Episode %d", req.Anime.Title, req.Episode.Number)
}

// resolvingView shows progress while the stream URL is being resolved.
func (a *App) resolvingView() string {
	title := ""
	if a.pending != nil {
		title = mediaTitle(a.pending)
	}
	return Header("Playback", title) + "\n" + fmtProgress("◆", "Resolving stream…")
}

// ---- Post-play menu ----

func (a *App) postPlayView() string {
	var b strings.Builder
	b.WriteString(Header("Playback", mediaTitle(a.pending)))
	b.WriteString("\n")
	b.WriteString(accentStyle.Render("  Finished "+mediaTitle(a.pending)) + "\n")
	if a.lastQuality != "" {
		b.WriteString("  Quality: " + a.lastQuality + "\n")
	}
	if a.lastErr != nil {
		b.WriteString("\n" + errorStyle.Render("◆ player exited with an error: "+a.lastErr.Error()) + "\n")
	}
	b.WriteString("\n")

	opts := []struct{ label, key string }{
		{"Next episode", "n"},
		{"Replay", "r"},
		{"Episode list", "e"},
		{"Quit", "q"},
	}
	for i, o := range opts {
		mark := "  "
		if i == a.postFocus {
			mark = "▸ "
		}
		b.WriteString(mark + o.key + "  " + o.label + "\n")
	}

	b.WriteString("\n" + StatusBar("↑/↓ navigate · enter select · n/r/e/q shortcuts"))
	return b.String()
}

func (a *App) postPlayUpdate(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		a.postFocus = (a.postFocus - 1 + 4) % 4
	case "down", "j":
		a.postFocus = (a.postFocus + 1) % 4
	case "n":
		return a.playNext()
	case "r":
		return a.replay()
	case "e", "esc":
		a.playState = playIdle
		a.screen = screenEpisodes
		return nil
	case "q":
		a.result = &Result{}
		return tea.Quit
	case "enter":
		switch a.postFocus {
		case 0:
			return a.playNext()
		case 1:
			return a.replay()
		case 2:
			a.playState = playIdle
			a.screen = screenEpisodes
			return nil
		case 3:
			a.result = &Result{}
			return tea.Quit
		}
	}
	return nil
}

func (a *App) playNext() tea.Cmd {
	next := a.pending.Episode.Number + 1
	for i, ep := range a.ep.episodes {
		if ep.Number == next {
			a.ep.cursor = i
			return a.startPlay(ep)
		}
	}
	return func() tea.Msg { return toastMsg{text: "no next episode"} }
}

func (a *App) replay() tea.Cmd {
	return a.startPlay(a.pending.Episode)
}

// recordHistory saves the watched episode for the "Continue" menu.
func recordHistory(pr *PlayRequest) {
	if pr == nil {
		return
	}
	f, err := store.History()
	if err != nil {
		return
	}
	_ = f.Add(store.Entry{
		AnimeID: pr.Anime.ID,
		SlugID:  pr.Anime.SlugID,
		Title:   pr.Anime.Title,
		Cover:   pr.Anime.CoverURL,
		Episode: pr.Episode.Number,
	})
}
