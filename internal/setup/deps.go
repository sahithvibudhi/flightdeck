package setup

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

func installGit() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("automatic git install is only supported on Linux — install git manually and try again")
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
			cmd := exec.Command(i.args[0], i.args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	return fmt.Errorf("no supported package manager found (apt, yum, apk)")
}

/*
installCaddy downloads the Caddy binary from the official API.
This is the same method recommended by the Caddy project for
single-binary installs without a package manager.
*/
func installCaddy() error {
	arch := runtime.GOARCH
	goos := runtime.GOOS

	url := fmt.Sprintf("https://caddyserver.com/api/download?os=%s&arch=%s", goos, arch)
	fmt.Printf("  Downloading from %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "caddy-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("download write: %w", err)
	}
	tmp.Close()

	dest := "/usr/local/bin/caddy"
	if err := os.Rename(tmp.Name(), dest); err != nil {
		data, readErr := os.ReadFile(tmp.Name())
		if readErr != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		if writeErr := os.WriteFile(dest, data, 0755); writeErr != nil {
			return fmt.Errorf("install failed: %w", writeErr)
		}
	} else {
		os.Chmod(dest, 0755)
	}

	out, err := exec.Command("caddy", "version").Output()
	if err != nil {
		return fmt.Errorf("caddy installed but not working: %w", err)
	}
	fmt.Printf("  Installed caddy %s\n", strings.TrimSpace(string(out)))
	return nil
}
