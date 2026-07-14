package setup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type depStatus struct {
	name      string
	bin       string
	installed bool
	version   string
}

func checkDep(name, bin, vFlag string) depStatus {
	d := depStatus{name: name, bin: bin}
	out, err := exec.Command(bin, vFlag).CombinedOutput()
	if err != nil {
		return d
	}
	d.installed = true
	d.version = strings.TrimSpace(string(out))
	return d
}

func installGit() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("automatic git install is only supported on Linux — install git manually and try again")
	}

	type installer struct {
		check string
		args  []string
	}

	installers := []installer{
		{"apt-get", []string{"sh", "-c", "apt-get update -qq && apt-get install -y -qq git"}},
		{"yum", []string{"yum", "install", "-y", "-q", "git"}},
		{"apk", []string{"apk", "add", "--quiet", "git"}},
	}

	for _, i := range installers {
		if _, err := exec.LookPath(i.check); err == nil {
			out, err := exec.Command(i.args[0], i.args[1:]...).CombinedOutput()
			if err != nil {
				return string(out), fmt.Errorf("git install failed: %w", err)
			}
			return string(out), nil
		}
	}

	return "", fmt.Errorf("no supported package manager found (apt, yum, apk)")
}

/*
installCaddy downloads the Caddy binary from the official API.
This is the same method recommended by the Caddy project for
single-binary installs without a package manager.
*/
func installCaddy() (string, error) {
	tmp, err := downloadCaddy()
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	dest := "/usr/local/bin/caddy"
	if err := os.Rename(tmp, dest); err != nil {
		data, readErr := os.ReadFile(tmp)
		if readErr != nil {
			return "", fmt.Errorf("install failed: %w", err)
		}
		if writeErr := os.WriteFile(dest, data, 0755); writeErr != nil {
			return "", fmt.Errorf("install failed: %w", writeErr)
		}
	} else {
		os.Chmod(dest, 0755)
	}

	out, err := exec.Command("caddy", "version").Output()
	if err != nil {
		return "", fmt.Errorf("caddy installed but not working: %w", err)
	}
	return "installed caddy " + strings.TrimSpace(string(out)), nil
}

/*
downloadCaddy fetches the Caddy binary, preferring the official build
service and falling back to GitHub releases (the build service rate
limits, and some networks block one host but not the other). Returns
the path to a temp file holding the binary.
*/
func downloadCaddy() (string, error) {
	arch := runtime.GOARCH
	goos := runtime.GOOS

	official := fmt.Sprintf("https://caddyserver.com/api/download?os=%s&arch=%s", goos, arch)
	if path, err := fetchToTemp(official, false); err == nil {
		return path, nil
	}

	// Fallback: resolve the latest release tag via the GitHub API, then
	// download and extract the tarball asset.
	resp, err := http.Get("https://api.github.com/repos/caddyserver/caddy/releases/latest")
	if err != nil {
		return "", fmt.Errorf("caddy download failed from both caddyserver.com and github.com: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("caddy download failed: caddyserver.com unavailable and github.com returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parse github release: %w", err)
	}

	version := strings.TrimPrefix(release.TagName, "v")
	tarURL := fmt.Sprintf(
		"https://github.com/caddyserver/caddy/releases/download/%s/caddy_%s_%s_%s.tar.gz",
		release.TagName, version, goos, arch,
	)
	return fetchToTemp(tarURL, true)
}

func fetchToTemp(url string, isTarGz bool) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "caddy-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	var src io.Reader = resp.Body
	if isTarGz {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", fmt.Errorf("read tarball: %w", err)
		}
		tr := tar.NewReader(gz)
		found := false
		for {
			hdr, err := tr.Next()
			if err != nil {
				break
			}
			if filepath.Base(hdr.Name) == "caddy" {
				found = true
				break
			}
		}
		if !found {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", fmt.Errorf("caddy binary not found in release tarball")
		}
		src = tr
	}

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download write: %w", err)
	}
	tmp.Close()
	return tmp.Name(), nil
}
