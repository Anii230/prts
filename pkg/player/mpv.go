package player

import (
	"fmt"
	"os"
	"os/exec"
)

func Play(streamURL string, referer string) error {
	args := []string{streamURL}

	if referer != "" {
		args = append(args, fmt.Sprintf("--http-header-fields=Referer: %s, User-Agent: Mozilla/5.0", referer))
	}

	cmd := exec.Command("mpv", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
