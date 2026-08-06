// Package player launches external players and downloaders for resolved
// stream URLs.
package player

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Command returns an exec.Cmd that streams url in the given player binary
// (default "mpv"). Additional flags can be supplied through the
// PRTS_PLAYER_FLAGS environment variable, mirroring ani-cli's
// ANI_CLI_PLAYER_FLAGS.
func Command(playerBin, mediaTitle, url string) *exec.Cmd {
	if playerBin == "" {
		playerBin = "mpv"
	}

	args := strings.Fields(os.Getenv("PRTS_PLAYER_FLAGS"))
	args = append(args,
		fmt.Sprintf("--force-media-title=%s", mediaTitle),
		url,
	)
	return exec.Command(playerBin, args...)
}

// Play streams url in the given player binary (default "mpv"). Additional
// flags can be supplied through the PRTS_PLAYER_FLAGS environment variable,
// mirroring ani-cli's ANI_CLI_PLAYER_FLAGS.
func Play(playerBin, mediaTitle, url string) error {
	cmd := Command(playerBin, mediaTitle, url)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DownloadCommand returns an exec.Cmd that saves the stream to dir with
// yt-dlp, falling back to ffmpeg.
func DownloadCommand(url, dir, title string) (*exec.Cmd, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, sanitize(title)+".mp4")

	if yt, err := exec.LookPath("yt-dlp"); err == nil {
		return exec.Command(yt,
			url,
			"--no-skip-unavailable-fragments",
			"--fragment-retries", "infinite",
			"-N", "16",
			"-o", out,
		), nil
	}

	if ff, err := exec.LookPath("ffmpeg"); err == nil {
		return exec.Command(ff,
			"-extension_picky", "0",
			"-loglevel", "error",
			"-stats",
			"-i", url,
			"-c", "copy",
			out,
		), nil
	}

	return nil, fmt.Errorf("neither yt-dlp nor ffmpeg is installed; cannot download")
}

// Download saves the stream to dir using yt-dlp, falling back to ffmpeg.
func Download(url, dir, title string) error {
	cmd, err := DownloadCommand(url, dir, title)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var badChars = regexp.MustCompile(`[\\/:*?"<>|]+`)

func sanitize(name string) string {
	name = badChars.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "video"
	}
	return name
}
