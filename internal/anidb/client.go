// Package anidb implements the streaming pipeline used by ani-cli v5:
//
//	search     GET /browse?q=...            (HTML scrape)
//	info       GET /anime/<slug-id>         (HTML scrape, optional)
//	episodes   GET /api/frontend/anime/<id>/episodes
//	embed      GET /api/frontend/episode/<id>/languages
//	master     GET <embed_url>              (scrape `file: '...m3u8'`)
//	quality    GET <master.m3u8>            (parse HLS variants)
package anidb

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"prts/internal/httpget"
)

const base = "https://anidb.app"

// SearchResult is one anime card returned by the browse page.
type SearchResult struct {
	SlugID string // e.g. "one-piece-3880"
	ID     int    // numeric id, e.g. 3880
	Title  string
}

// Anime holds metadata scraped from the anime details page.
type Anime struct {
	ID          int
	SlugID      string
	Title       string
	MALID       int
	Description string
	Genres      []string
	CoverURL    string
}

// Episode is a single episode entry from the episodes API.
type Episode struct {
	ID     int
	Number int
	Filler bool
}

// Quality is one variant from the HLS master playlist.
type Quality struct {
	Label string // e.g. "1080p"
	URL   string
}

var (
	cardRe     = regexp.MustCompile(`anime/([a-z0-9-]+-\d+)"[^>]*title="([^"]+)"`)
	malIDRe    = regexp.MustCompile(`myanimelist\.net/anime/(\d+)`)
	titleRe    = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	synopsisRe = regexp.MustCompile(`(?s)<h2[^>]*>Synopsis</h2>(.*?)</div>`)
	jsonLDRe   = regexp.MustCompile(`(?s)application/ld\+json">(.*?)</script>`)
	fileRe     = regexp.MustCompile(`file:\s*'([^']+)'`)
	fileReAlt  = regexp.MustCompile(`file:\s*"([^"]+)"`)
	streamInf  = regexp.MustCompile(`#EXT-X-STREAM-INF:[^\n]*RESOLUTION=(\d+)x(\d+)[^\n]*`)
)

// Search queries the browse page and returns matching anime.
func Search(ctx context.Context, query string) ([]SearchResult, error) {
	target := fmt.Sprintf("%s/browse?q=%s", base, url.QueryEscape(query))
	body, err := httpget.Get(ctx, target)
	if err != nil {
		return nil, err
	}
	return parseCards(body)
}

// Popular returns the featured anime from the browse page (no query), used
// for the "Trending" menu entry.
func Popular(ctx context.Context) ([]SearchResult, error) {
	body, err := httpget.Get(ctx, base+"/browse")
	if err != nil {
		return nil, err
	}
	return parseCards(body)
}

func parseCards(body []byte) ([]SearchResult, error) {
	seen := make(map[string]bool)
	var results []SearchResult
	for _, m := range cardRe.FindAllStringSubmatch(string(body), -1) {
		slugID := m[1]
		title := strings.TrimSpace(html.UnescapeString(m[2]))
		if title == "" || seen[slugID] {
			continue
		}
		seen[slugID] = true
		id, _ := strconv.Atoi(slugID[strings.LastIndex(slugID, "-")+1:])
		results = append(results, SearchResult{SlugID: slugID, ID: id, Title: title})
	}
	return results, nil
}

// Info fetches extra metadata (MAL id, description, genres, cover) from the
// anime page. Fields not present on the page are left zero/empty.
func Info(ctx context.Context, r SearchResult) (*Anime, error) {
	target := fmt.Sprintf("%s/anime/%s", base, r.SlugID)
	body, err := httpget.Get(ctx, target)
	if err != nil {
		return nil, err
	}

	a := &Anime{ID: r.ID, SlugID: r.SlugID, Title: r.Title}
	s := string(body)

	if m := malIDRe.FindStringSubmatch(s); len(m) == 2 {
		a.MALID, _ = strconv.Atoi(m[1])
	}
	if m := titleRe.FindStringSubmatch(s); len(m) == 2 {
		if t := stripTags(strings.TrimSpace(m[1])); t != "" {
			a.Title = t
		}
	}
	if m := synopsisRe.FindStringSubmatch(s); len(m) == 2 {
		a.Description = strings.TrimSpace(stripTags(m[1]))
	}
	if m := jsonLDRe.FindStringSubmatch(s); len(m) == 2 {
		var ld struct {
			Image string   `json:"image"`
			Genre []string `json:"genre"`
		}
		if err := json.Unmarshal([]byte(m[1]), &ld); err == nil {
			a.CoverURL = ld.Image
			a.Genres = ld.Genre
		}
	}
	return a, nil
}

// Episodes returns the episode list for an anime.
func Episodes(ctx context.Context, animeID int) ([]Episode, error) {
	target := fmt.Sprintf("%s/api/frontend/anime/%d/episodes", base, animeID)
	body, err := httpget.Get(ctx, target)
	if err != nil {
		return nil, err
	}

	var res struct {
		Episodes []struct {
			ID     int  `json:"id"`
			Number *int `json:"number"`
			Filler bool `json:"filler"`
		} `json:"episodes"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("parse episodes: %w", err)
	}
	if len(res.Episodes) == 0 {
		return nil, fmt.Errorf("no episodes returned for anime %d", animeID)
	}

	eps := make([]Episode, 0, len(res.Episodes))
	for i, e := range res.Episodes {
		num := i + 1
		if e.Number != nil && *e.Number > 0 {
			num = *e.Number
		}
		eps = append(eps, Episode{ID: e.ID, Number: num, Filler: e.Filler})
	}
	return eps, nil
}

// EmbedURL resolves the streaming embed URL for an episode and language.
// lang is "jpn" (subs) or "eng" (dubs).
func EmbedURL(ctx context.Context, episodeID int, lang string) (string, error) {
	target := fmt.Sprintf("%s/api/frontend/episode/%d/languages", base, episodeID)
	body, err := httpget.Get(ctx, target)
	if err != nil {
		return "", err
	}

	var res struct {
		Languages []struct {
			Code     string `json:"code"`
			Name     string `json:"name"`
			EmbedURL string `json:"embed_url"`
		} `json:"languages"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("parse languages: %w", err)
	}
	if len(res.Languages) == 0 {
		return "", fmt.Errorf("no languages returned for episode %d", episodeID)
	}

	for _, l := range res.Languages {
		if strings.EqualFold(l.Code, lang) && l.EmbedURL != "" {
			return l.EmbedURL, nil
		}
	}
	// Fall back to whatever is available rather than failing outright.
	for _, l := range res.Languages {
		if l.EmbedURL != "" {
			return l.EmbedURL, nil
		}
	}
	return "", fmt.Errorf("no embed url available for episode %d (%s)", episodeID, lang)
}

// MasterPlaylist fetches an embed page and extracts the HLS master URL
// from its `file: '...'` JavaScript literal.
func MasterPlaylist(ctx context.Context, embedURL string) (string, error) {
	body, err := httpget.Get(ctx, embedURL)
	if err != nil {
		return "", err
	}
	s := string(body)
	for _, re := range []*regexp.Regexp{fileRe, fileReAlt} {
		if m := re.FindStringSubmatch(s); len(m) == 2 {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("no playlist url found in embed %s", embedURL)
}

// Qualities parses an HLS master playlist into its variants, highest first.
func Qualities(ctx context.Context, masterURL string) ([]Quality, error) {
	body, err := httpget.Get(ctx, masterURL)
	if err != nil {
		return nil, err
	}
	baseURL, err := url.Parse(masterURL)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(body), "\n")
	var quals []Quality
	for i := 0; i < len(lines); i++ {
		m := streamInf.FindStringSubmatch(lines[i])
		if len(m) != 3 {
			continue
		}
		// Skip I-FRAME variants (they have a different EXT-X tag).
		if strings.Contains(lines[i], "#EXT-X-I-FRAME") {
			continue
		}
		// The variant URL is on the next non-empty, non-comment line.
		var variant string
		for j := i + 1; j < len(lines); j++ {
			line := strings.TrimSpace(lines[j])
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			variant = line
			break
		}
		if variant == "" {
			continue
		}
		height, _ := strconv.Atoi(m[2])
		ref, err := url.Parse(variant)
		if err != nil {
			continue
		}
		abs := baseURL.ResolveReference(ref).String()
		quals = append(quals, Quality{Label: fmt.Sprintf("%dp", height), URL: abs})
	}

	sort.SliceStable(quals, func(i, j int) bool {
		return resOf(quals[i].Label) > resOf(quals[j].Label)
	})
	return quals, nil
}

// PickQuality selects a variant by preference: "" or "best" picks the highest
// resolution, "worst" the lowest, and an integer like "720" an exact match.
func PickQuality(quals []Quality, pref string) (*Quality, error) {
	if len(quals) == 0 {
		return nil, fmt.Errorf("no quality variants available")
	}

	p := strings.ToLower(strings.TrimSpace(pref))
	if p == "" || p == "best" {
		return &quals[0], nil
	}
	if p == "worst" {
		return &quals[len(quals)-1], nil
	}

	want, err := strconv.Atoi(p)
	if err == nil {
		for i := range quals {
			if resOf(quals[i].Label) == want {
				return &quals[i], nil
			}
		}
	}
	return &quals[0], nil
}

func resOf(label string) int {
	n, _ := strconv.Atoi(strings.TrimSuffix(label, "p"))
	return n
}

func stripTags(s string) string {
	re := regexp.MustCompile(`(?s)<[^>]+>`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
