package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"prts/internal/tui"
)

const version = "2.0.0"

func main() {
	fs := flag.NewFlagSet("prts", flag.ExitOnError)
	fs.Usage = usage
	quality := fs.String("q", "best", "quality: best, worst, or a number (e.g. 720)")
	download := fs.Bool("d", false, "download the episode(s) instead of playing")
	dub := fs.Bool("dub", false, "use the English dub instead of Japanese subs")
	playerBin := fs.String("p", "mpv", "player binary to use")
	showVersion := fs.Bool("v", false, "print version and exit")
	_ = fs.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("prts %s\n", version)
		return
	}

	cfg := tui.Config{
		Quality:  *quality,
		Dub:      *dub,
		Download: *download,
		Player:   *playerBin,
	}

	// Without a query the TUI opens on the main menu; with one it jumps
	// straight into search.
	query := strings.Join(fs.Args(), " ")

	app := tui.New(cfg, query)
	result := app.Run()
	if result.Error != nil {
		log.Fatalf("[PRTS] %v\n", result.Error)
	}
}

func usage() {
	fmt.Print(`prts - stream anime from the terminal (like ani-cli)

Usage:
  prts [options] [search query]

With no query, prts opens an interactive menu (Search / Trending / Continue).
With a query it jumps straight into search results.

Options:
  -q <quality>   quality: best, worst, or a number (e.g. 720)
  -d             download the episode instead of playing
  -dub           use the English dub instead of Japanese subs
  -p <player>    player binary (default: mpv)
  -v             print version and exit
  -h             show this help

After an episode finishes playing you get a menu to continue binging:
next / replay / episode list / quit.

Examples:
  prts
  prts "one piece"
  prts -q 1080 -dub "attack on titan"
  prts -d "frieren"
`)
}
