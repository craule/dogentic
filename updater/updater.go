package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// UpdateAgent downloads the new binary and replaces the current one.
// It relies on Systemd to restart the process after exit.
func UpdateAgent(version string) error {
	// GitHub Release URL
	// In a real app, you might want to fetch "latest" tag from GitHub API first
	// to verify version, but for now we pull the specific version or latest.
	url := "https://github.com/Dogentadmin/dogent-agent/releases/latest/download/dogent-agent"

	fmt.Printf("🚀 Starting Update to latest version from %s...\n", url)

	// 1. Download New Binary
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// 2. Identify Current Binary Path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find executable path: %v", err)
	}

	// 3. Save to Temporary File
	tmpPath := exePath + ".new"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("could not create temp file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("write failed: %v", err)
	}

	// 4. Make Executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod failed: %v", err)
	}

	// 5. Replace Old Binary
	// Rename current to .bak (optional backup)
	os.Rename(exePath, exePath+".bak")

	if err := os.Rename(tmpPath, exePath); err != nil {
		// Try to restore
		os.Rename(exePath+".bak", exePath)
		return fmt.Errorf("replace failed: %v", err)
	}

	fmt.Println("✅ Update applied successfully. Restarting...")

	// 6. Restart Service (or Exit to let Systemd restart)
	// We'll try to restart via systemctl if possible, otherwise just exit.
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0) // Systemd should restart us
	}()

	return nil
}
