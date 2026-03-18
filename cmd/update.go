package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	githubRepo = "datrics-ltd/gads-cli"
	apiBase    = "https://api.github.com"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Update gads to the latest version",
	Long:    `Check for the latest release and update gads to it, verifying the checksum.`,
	Example: `  gads update`,
	Args:    cobra.NoArgs,
	RunE:    runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	current := rootCmd.Version
	if current == "" {
		current = "dev"
	}

	fmt.Fprintf(os.Stderr, "Current version: %s\n", current)
	fmt.Fprintf(os.Stderr, "Checking for latest release...\n")

	latest, downloadURL, checksumURL, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("fetching latest release: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Latest version:  %s\n", latest)

	if current == latest {
		fmt.Fprintln(os.Stderr, "Already up to date.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Updating %s → %s\n", current, latest)

	// Download the binary
	fmt.Fprintf(os.Stderr, "Downloading binary...\n")
	data, err := githubDownload(downloadURL)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}

	// Verify checksum
	if checksumURL != "" {
		fmt.Fprintf(os.Stderr, "Verifying checksum...\n")
		if err := verifyChecksum(data, filepath.Base(downloadURL), checksumURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Checksum OK\n")
	}

	// Determine path to replace
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}
	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Write to a temp file next to the binary, then rename (atomic on same filesystem)
	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, ".gads-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Preserve original file permissions
	info, err := os.Stat(execPath)
	if err == nil {
		_ = os.Chmod(tmpPath, info.Mode())
	} else {
		_ = os.Chmod(tmpPath, 0755)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Updated successfully to %s\n", latest)
	return nil
}

// githubRelease is the minimal subset of the GitHub releases API response we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		URL                string `json:"url"`
	} `json:"assets"`
}

// fetchLatestRelease returns the latest tag name, binary download URL, and checksum URL.
func fetchLatestRelease() (tag, binaryURL, checksumURL string, err error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, githubRepo)
	body, err := githubGet(url)
	if err != nil {
		return "", "", "", err
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", "", fmt.Errorf("parsing release JSON: %w", err)
	}
	if rel.TagName == "" {
		return "", "", "", fmt.Errorf("no release found")
	}

	// Determine expected binary name for this platform
	binaryName := platformBinaryName()

	for _, asset := range rel.Assets {
		switch {
		case asset.Name == binaryName:
			if os.Getenv("GITHUB_TOKEN") != "" {
				binaryURL = asset.URL // API URL, requires Accept header
			} else {
				binaryURL = asset.BrowserDownloadURL
			}
		case asset.Name == "checksums.txt":
			if os.Getenv("GITHUB_TOKEN") != "" {
				checksumURL = asset.URL
			} else {
				checksumURL = asset.BrowserDownloadURL
			}
		}
	}

	if binaryURL == "" {
		return "", "", "", fmt.Errorf("no binary found for platform %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, binaryName)
	}

	return rel.TagName, binaryURL, checksumURL, nil
}

// platformBinaryName returns the asset filename for the current platform.
func platformBinaryName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	// Map Go arch names to the naming convention used in the release workflow
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
	}
	arch := archMap[goarch]
	if arch == "" {
		arch = goarch
	}
	name := fmt.Sprintf("gads-%s-%s", goos, arch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// githubGet fetches a URL using optional GITHUB_TOKEN auth and returns the body.
func githubGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("GitHub API returned %d — set GITHUB_TOKEN for private repo access", resp.StatusCode)
		}
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// githubDownload downloads a release asset, handling private repos via asset API.
func githubDownload(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// When using the asset API URL (not browser_download_url), we need octet-stream
	if strings.Contains(url, "/assets/") {
		req.Header.Set("Accept", "application/octet-stream")
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum checks the SHA256 of data against the checksums.txt file.
func verifyChecksum(data []byte, filename, checksumURL string) error {
	checksumData, err := githubDownload(checksumURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}

	// Parse checksums.txt — each line: "<hash>  <filename>"
	expected := ""
	for _, line := range strings.Split(string(checksumData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == filename {
			expected = parts[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum found for %s in checksums.txt", filename)
	}

	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}
