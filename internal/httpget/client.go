// Package httpget provides a small HTTP client that mimics a browser to
// get past Cloudflare bot checks on streaming sites. If the plain Go client
// is challenged, it retries the request using curl-impersonate (which
// clones Chrome/Firefox TLS fingerprints), exactly like ani-cli does.
package httpget

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// chromeUA matches a modern desktop Chrome user agent.
const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// impersonateBinaries are tried in order when the plain client is blocked.
var impersonateBinaries = []string{
	"curl_firefox135",
	"curl_chrome136",
	"curl_chrome116",
	"curl_ff117",
	"curl",
}

var client = &http.Client{
	Timeout: 25 * time.Second,
	Transport: &http.Transport{
		ForceAttemptHTTP2: true,
	},
}

// Get fetches a URL and returns the response body. It transparently retries
// through curl-impersonate if the page is a Cloudflare challenge.
func Get(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if isCloudflareChallenge(resp.StatusCode, body) {
		return impersonate(ctx, target)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: unexpected status %d", target, resp.StatusCode)
	}
	return body, nil
}

// isCloudflareChallenge reports whether a response is a Cloudflare
// "Just a moment" interstitial or a generic bot-block.
func isCloudflareChallenge(status int, body []byte) bool {
	if status != http.StatusForbidden && status != http.StatusServiceUnavailable && status != http.StatusTooManyRequests {
		return false
	}
	head := body
	if len(head) > 4096 {
		head = head[:4096]
	}
	s := strings.ToLower(string(head))
	return strings.Contains(s, "just a moment") || strings.Contains(s, "cf-chl") || strings.Contains(s, "captcha")
}

// impersonate retries the request via curl-impersonate binaries, which are
// compiled with Chrome/Firefox TLS fingerprints that defeat the challenge.
func impersonate(ctx context.Context, target string) ([]byte, error) {
	for _, bin := range impersonateBinaries {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		out, err := exec.CommandContext(ctx, path,
			"-sL", "--max-time", "25",
			"-A", chromeUA,
			"-H", "Accept: application/json, text/plain, */*",
			target,
		).Output()
		if err != nil {
			continue
		}
		if len(out) == 0 || isCloudflareChallenge(http.StatusForbidden, out) {
			continue
		}
		return out, nil
	}
	return nil, fmt.Errorf("request blocked by Cloudflare and no curl-impersonate fallback succeeded: %s", target)
}
