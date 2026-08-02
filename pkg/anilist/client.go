package anilist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Anime struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Episodes    int      `json:"episodes"`
	Score       int      `json:"score"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
}

const query = `
query ($search: String) {
  Media(search: $search, type: ANIME) {
    id
    title {
      romaji
      english
    }
    description(asHtml: false)
    episodes
    averageScore
    genres
  }
}`

func Search(title string) (*Anime, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"query": query,
		"variables": map[string]string{
			"search": title,
		},
	})

	resp, err := http.Post("https://graphql.anilist.co", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Media struct {
				ID    int `json:"id"`
				Title struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
				} `json:"title"`
				Description string   `json:"description"`
				Episodes    int      `json:"episodes"`
				Score       int      `json:"averageScore"`
				Genres      []string `json:"genres"`
			} `json:"Media"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse AniList response: %w", err)
	}

	m := result.Data.Media
	displayTitle := m.Title.Romaji
	if displayTitle == "" {
		displayTitle = m.Title.English
	}

	return &Anime{
		ID:          m.ID,
		Title:       displayTitle,
		Episodes:    m.Episodes,
		Score:       m.Score,
		Description: m.Description,
		Genres:      m.Genres,
	}, nil
}
