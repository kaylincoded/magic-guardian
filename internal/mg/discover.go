package mg

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var versionRegex = regexp.MustCompile(`version/(\d+)`)

// DiscoverParams fetches the current game version and a fresh room ID from magicgarden.gg.
func DiscoverParams(ctx context.Context) (version string, roomID string, err error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://magicgarden.gg/", nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	// Extract room ID from redirect URL (e.g. /r/8GJG)
	loc := resp.Header.Get("Location")
	if loc != "" {
		roomID = extractRoomID(loc)
	}

	// If no redirect, read body for room ID
	if roomID == "" && resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if m := versionRegex.FindStringSubmatch(bodyStr); m != nil {
			version = m[1]
		}
		roomID = extractRoomID(bodyStr)
	}

	// Follow redirect to get version from HTML
	if loc != "" && version == "" {
		fullURL := loc
		if !strings.HasPrefix(loc, "http") {
			fullURL = "https://magicgarden.gg" + loc
		}
		req2, _ := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
		resp2, err := http.DefaultClient.Do(req2)
		if err == nil {
			defer resp2.Body.Close()
			body, _ := io.ReadAll(resp2.Body)
			if m := versionRegex.FindStringSubmatch(string(body)); m != nil {
				version = m[1]
			}
		}
	}

	if version == "" {
		return "", "", fmt.Errorf("could not discover version")
	}
	if roomID == "" {
		return "", "", fmt.Errorf("could not discover room ID")
	}

	return version, roomID, nil
}

func extractRoomID(s string) string {
	// Match /r/XXXX pattern
	re := regexp.MustCompile(`/r/([A-Za-z0-9]+)`)
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
