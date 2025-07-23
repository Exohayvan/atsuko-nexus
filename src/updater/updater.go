// Package updater handles fetching, downloading, and applying the latest release of the Atsuko Nexus binary from GitHub.
// It compares the current version against available releases and applies an update if a newer version is found.
package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/version"

	"github.com/Masterminds/semver/v3"
)

const (
	repoOwner = "Exohayvan"
	repoName  = "atsuko-nexus"
)

type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func RunUpdater() {
	logger.Log("INFO", "updater", "Checking for updates...")

	currentVersion := version.Get()
	channel := detectChannel(currentVersion)

	logger.Log("DEBUG", "updater", "Current version: "+currentVersion)
	logger.Log("DEBUG", "updater", "Current channel: "+channel)

	releases, err := fetchAllReleases()
	if err != nil {
		logger.Log("ERROR", "updater", "Failed to fetch releases: "+err.Error())
		return
	}

	filtered := filterReleasesByChannel(releases, channel)
	if len(filtered) == 0 {
		logger.Log("ERROR", "updater", "No releases available in current channel")
		return
	}

	latest := filtered[0]

	// Use semantic version comparison
	currVer, err1 := semver.NewVersion(strings.TrimPrefix(currentVersion, "v"))
	latestVer, err2 := semver.NewVersion(strings.TrimPrefix(latest.TagName, "v"))

	if err1 == nil && err2 == nil && !latestVer.GreaterThan(currVer) {
		logger.Log("INFO", "updater", "Already up to date: "+currentVersion)
		return
	}

	targetName := buildTargetName() + ".zip"
	assetURL := findAssetURL(&latest, targetName)
	if assetURL == "" {
		logger.Log("ERROR", "updater", "No matching asset found: "+targetName)
		return
	}

	logger.Log("INFO", "updater", fmt.Sprintf("Updating from %s to %s", currentVersion, latest.TagName))

	tmpZip := "atsuko_update.zip"
	if err := downloadFile(tmpZip, assetURL); err != nil {
		logger.Log("ERROR", "updater", "Download failed: "+err.Error())
		return
	}

	tmpBin := "atsuko_tmp"
	if err := extractBinaryFromZip(tmpZip, tmpBin); err != nil {
		logger.Log("ERROR", "updater", "Unzip failed: "+err.Error())
		return
	}
	_ = os.Remove(tmpZip)

	if err := applyUpdate(tmpBin); err != nil {
		logger.Log("ERROR", "updater", "Failed to apply update: "+err.Error())
		fmt.Println("Failed to apply update: " + err.Error())
		return
	}

	logger.Log("INFO", "updater", "Update applied successfully. Please restart the application manually.")
	time.Sleep(3 * time.Second)
	os.Exit(0)
}

func detectChannel(version string) string {
	version = strings.ToLower(version)
	switch {
	case strings.Contains(version, "alpha"):
		return "alpha"
	case strings.Contains(version, "beta"):
		return "beta"
	default:
		return "stable"
	}
}

func fetchAllReleases() ([]GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", repoOwner, repoName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	// Semantic version sort
	sort.SliceStable(releases, func(i, j int) bool {
		vi, err1 := semver.NewVersion(strings.TrimPrefix(releases[i].TagName, "v"))
		vj, err2 := semver.NewVersion(strings.TrimPrefix(releases[j].TagName, "v"))
		if err1 != nil || err2 != nil {
			return releases[i].TagName > releases[j].TagName
		}
		return vi.GreaterThan(vj)
	})

	return releases, nil
}

func filterReleasesByChannel(all []GitHubRelease, channel string) []GitHubRelease {
	var filtered []GitHubRelease
	for _, r := range all {
		lower := strings.ToLower(r.TagName)
		switch channel {
		case "alpha":
			if strings.Contains(lower, "alpha") {
				filtered = append(filtered, r)
			}
		case "beta":
			if strings.Contains(lower, "beta") && !strings.Contains(lower, "alpha") {
				filtered = append(filtered, r)
			}
		case "stable":
			if !strings.Contains(lower, "alpha") && !strings.Contains(lower, "beta") {
				filtered = append(filtered, r)
			}
		}
	}
	return filtered
}

func buildTargetName() string {
	platform := runtime.GOOS
	arch := runtime.GOARCH

	var platformLabel string
	switch platform {
	case "darwin":
		platformLabel = "macos"
	default:
		platformLabel = platform
	}

	return fmt.Sprintf("atsuko-%s-%s", platformLabel, arch)
}

func findAssetURL(release *GitHubRelease, targetName string) string {
	for _, asset := range release.Assets {
		if strings.EqualFold(asset.Name, targetName) {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func downloadFile(filepath, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinaryFromZip(zipPath, outputPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if len(r.File) == 0 {
		return fmt.Errorf("zip archive is empty")
	}

	f := r.File[0]
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, rc); err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(outputPath, 0755); err != nil {
			return err
		}
	}

	return nil
}

func applyUpdate(tempBinary string) error {
	currentBinary, err := os.Executable()
	if err != nil {
		return err
	}

	backup := currentBinary + ".bak"
	_ = os.Rename(currentBinary, backup)

	err = os.Rename(tempBinary, currentBinary)
	if err != nil {
		_ = os.Rename(backup, currentBinary)
		return err
	}

	if runtime.GOOS != "windows" {
		err = os.Chmod(currentBinary, 0755)
	}

	return err
}
