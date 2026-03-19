package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// createFile creates a file for writing the downloaded binary.
func createFile(path string) (*os.File, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Create file with appropriate permissions
	mode := os.FileMode(0644)
	if runtime.GOOS != "windows" {
		mode = 0755 // Executable on Unix
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// ApplyUpdate replaces the current binary with the downloaded one.
// On desktop: atomic rename + exec to restart.
// On Android: the binary is in app-private storage, just replace it.
func ApplyUpdate(downloadedPath string) error {
	if runtime.GOOS == "android" {
		return applyAndroidUpdate(downloadedPath)
	}
	return applyDesktopUpdate(downloadedPath)
}

// applyDesktopUpdate replaces the running binary and restarts.
func applyDesktopUpdate(downloadedPath string) error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	// Backup current binary
	backupPath := execPath + ".backup"
	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(downloadedPath, execPath); err != nil {
		// Try to restore backup
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// Make executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(execPath, 0755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	// Remove backup (best effort)
	_ = os.Remove(backupPath)

	return nil
}

// applyAndroidUpdate handles Android-specific update logic.
// On Android, the Go binary runs from app-private storage, not the APK.
func applyAndroidUpdate(downloadedPath string) error {
	// On Android, we download the new binary to a temp location,
	// then move it to the expected location in app files directory.
	// The GuardianService will need to be restarted to pick up the new binary.

	// Get the target path (where GuardianService expects the binary)
	targetPath := getAndroidBinaryPath()
	if targetPath == "" {
		return fmt.Errorf("cannot determine android binary path")
	}

	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Backup current binary
	backupPath := targetPath + ".backup"
	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("backup current binary: %w", err)
		}
	}

	// Move new binary into place
	if err := os.Rename(downloadedPath, targetPath); err != nil {
		// Try to restore backup
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Remove backup (best effort)
	_ = os.Remove(backupPath)

	return nil
}

// getAndroidBinaryPath returns the path where the Go binary should live on Android.
// This is set by the Android app via environment variable.
func getAndroidBinaryPath() string {
	// The Android app sets this environment variable
	if path := os.Getenv("GUARDIAN_BINARY_PATH"); path != "" {
		return path
	}
	// Fallback: try to determine from current executable
	if execPath, err := os.Executable(); err == nil {
		return execPath
	}
	return ""
}

// GetDownloadPath returns a suitable path for downloading updates.
func GetDownloadPath() (string, error) {
	if runtime.GOOS == "android" {
		// Use app cache directory on Android
		cacheDir := os.Getenv("GUARDIAN_CACHE_DIR")
		if cacheDir == "" {
			cacheDir = "/data/local/tmp"
		}
		return filepath.Join(cacheDir, "magic-guardian.update"), nil
	}

	// Desktop: use temp directory
	return filepath.Join(os.TempDir(), "magic-guardian.update"), nil
}
