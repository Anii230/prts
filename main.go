package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"prts/pkg/anilist"
	"prts/pkg/player"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: prts <anime_title>")
		os.Exit(1)
	}

	searchTerm := strings.Join(os.Args[1:], " ")

	fmt.Printf("[PRTS] Fetching metadata for '%s'...\n", searchTerm)
	anime, err := anilist.Search(searchTerm)
	if err != nil {
		log.Fatalf("[PRTS Error] %v\n", err)
	}

	fmt.Printf("\n=== PRTS TERMINAL DISPLAY ===\n")
	fmt.Printf("Title:    %s\n", anime.Title)
	fmt.Printf("Episodes: %d\n", anime.Episodes)
	fmt.Printf("Score:    %d/100\n", anime.Score)
	fmt.Printf("Genres:   %s\n", strings.Join(anime.Genres, ", "))
	fmt.Println("=============================\n")

	// Phase 1 Dummy Test Stream (Big Buck Bunny HLS Stream)
	// In Phase 2, this stream URL will come directly from our Go scrapers!
	testStream := "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8"

	fmt.Println("[PRTS] Handing off stream to mpv engine...")
	if err := player.Play(testStream, ""); err != nil {
		log.Fatalf("[PRTS Error] Playback failed: %v\n", err)
	}
}
