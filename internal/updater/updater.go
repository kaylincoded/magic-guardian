package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Version is set at build time via -ldflags "-X github.com/kaylincoded/magic-guardian/internal/updater.Version=v0.3.0"
var Version = "dev"

const (
	githubOwner = "kaylincoded"
	githubRepo  = "magic-guardian"
	releaseAPI  = "https://api.github.com/repos/%s/%s/releases/latest"
)

// UpdateInfo contains information about available updates.
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	DownloadURL    string `json:"download_url,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
}

// githubRelease represents the GitHub API response for a release.
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

// githubAsset represents a release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Checker handles update checking against GitHub releases.
type Checker struct {
	client *http.Client
}

// NewChecker creates a new update checker.
func NewChecker() *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Check queries GitHub for the latest release and compares to current version.
func (c *Checker) Check(ctx context.Context) (*UpdateInfo, error) {
	url := fmt.Sprintf(releaseAPI, githubOwner, githubRepo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "magic-guardian/"+Version)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases yet
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: Version,
			LatestVersion:  Version,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	info := &UpdateInfo{
		CurrentVersion: Version,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		PublishedAt:    release.PublishedAt,
	}

	// Compare versions
	info.Available = isNewer(release.TagName, Version)

	// Find download URL for current platform
	if info.Available {
		info.DownloadURL = findAssetURL(release.Assets)
	}

	return info, nil
}

// isNewer returns true if latest is newer than current.
// Handles versions like "v0.3.0", "0.3.0", "dev".
func isNewer(latest, current string) bool {
	// Dev builds always show updates available
	if current == "dev" {
		return true
	}

	// Normalize versions (remove 'v' prefix)
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	// Simple string comparison works for semver when formatted consistently
	// For more robust comparison, could use a semver library
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}

// findAssetURL finds the download URL for the current platform.
func findAssetURL(assets []githubAsset) string {
	// On Android, always return APK URL (binary updates don't work due to SELinux)
	if runtime.GOOS == "android" {
		for _, asset := range assets {
			if strings.HasSuffix(asset.Name, ".apk") {
				return asset.BrowserDownloadURL
			}
		}
		return ""
	}

	// Build expected asset name based on OS/arch
	var suffix string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			suffix = "darwin-arm64"
		} else {
			suffix = "darwin-amd64"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			suffix = "linux-arm64"
		} else {
			suffix = "linux-amd64"
		}
	case "windows":
		suffix = "windows-amd64.exe"
	default:
		return ""
	}

	// Find matching asset
	for _, asset := range assets {
		if strings.Contains(asset.Name, suffix) {
			return asset.BrowserDownloadURL
		}
	}

	return ""
}

// IsAndroid returns true if running on Android.
func IsAndroid() bool {
	return runtime.GOOS == "android"
}

// Download fetches the update binary to the specified path.
func (c *Checker) Download(ctx context.Context, url, destPath string, progress func(downloaded, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Create destination file
	out, err := createFile(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	// Copy with progress tracking
	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write: %w", writeErr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
	}

	return nil
}

// GetCurrentVersion returns the current build version.
func GetCurrentVersion() string {
	return Version
}
