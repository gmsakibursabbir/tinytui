package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	RepoOwner = "gmsakibursabbir"
	RepoName  = "tinitui"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func GetLatestVersion() (string, *Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", RepoOwner, RepoName)
	resp, err := http.Get(url)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("failed to check update: %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", nil, err
	}

	return release.TagName, &release, nil
}

func IsNewer(current, latest string) bool {
    // Basic comparison assuming vX.Y.Z
    // For robust comparison we might want a semver lib, but text compare works for strict format
    // Ignoring 'v' prefix
    c := strings.TrimPrefix(current, "v")
    l := strings.TrimPrefix(latest, "v")
    return c != l && l > c // Simple lexicographical check (flawed if 1.10 < 1.9, but assuming standard)
	// Actually no, 1.10 < 1.9 is false, but 1.2 vs 1.10 -> 1.2 > 1.10 (string wise) is WRONG.
	// We need meaningful split.
	return compareVersions(c, l)
}

func compareVersions(v1, v2 string) bool {
	p1 := strings.Split(v1, ".")
	p2 := strings.Split(v2, ".")
	len1 := len(p1)
	len2 := len(p2)
	maxLen := len1
	if len2 > maxLen { maxLen = len2 }

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len1 { fmt.Sscanf(p1[i], "%d", &n1) }
		if i < len2 { fmt.Sscanf(p2[i], "%d", &n2) }
		if n2 > n1 { return true }
		if n1 > n2 { return false }
	}
	return false
}

func Update(release *Release) error {
	// Find asset
	goOS := runtime.GOOS
	goArch := runtime.GOARCH

	// Expected name pattern: tinytui-{os}-{arch}
	// e.g. tinytui-linux-amd64
	targetName := fmt.Sprintf("tinytui-%s-%s", goOS, goArch)
	if goOS == "windows" {
		targetName += ".exe"
	}

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == targetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s", goOS, goArch)
	}

	// Download
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Temp file
	tmpFile, err := os.CreateTemp("", "tinytui-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Validate basic
	info, err := os.Stat(tmpFile.Name())
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("download failed (empty file)")
	}

	// Chmod
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return err
	}

	// Replace
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	// Safe rename
	if err := os.Rename(tmpFile.Name(), exePath); err != nil {
		return err
	}

	return nil
}
