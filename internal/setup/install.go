package setup

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func InstallRuntime(name string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("automatic install is only supported on Linux")
	}

	switch strings.ToLower(name) {
	case "node.js", "node":
		return runInstall([]installOption{
			{"apt-get", []string{"sh", "-c", "curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - && apt-get install -y nodejs"}},
			{"yum", []string{"sh", "-c", "curl -fsSL https://rpm.nodesource.com/setup_lts.x | bash - && yum install -y nodejs"}},
			{"apk", []string{"apk", "add", "--quiet", "nodejs", "npm"}},
		})
	case "python":
		return runInstall([]installOption{
			{"apt-get", []string{"sh", "-c", "apt-get update -qq && apt-get install -y -qq python3 python3-pip python3-venv"}},
			{"yum", []string{"yum", "install", "-y", "python3", "python3-pip"}},
			{"apk", []string{"apk", "add", "--quiet", "python3", "py3-pip"}},
		})
	case "go":
		return runInstall([]installOption{
			{"sh", []string{"sh", "-c", "curl -fsSL https://go.dev/dl/go1.23.4.linux-" + runtime.GOARCH + ".tar.gz | tar -C /usr/local -xzf - && ln -sf /usr/local/go/bin/go /usr/local/bin/go"}},
		})
	case "docker":
		return runInstall([]installOption{
			{"sh", []string{"sh", "-c", "curl -fsSL https://get.docker.com | sh"}},
		})
	case "bun":
		return runInstall([]installOption{
			{"sh", []string{"sh", "-c", "curl -fsSL https://bun.sh/install | bash"}},
		})
	case "deno":
		return runInstall([]installOption{
			{"sh", []string{"sh", "-c", "curl -fsSL https://deno.land/install.sh | sh"}},
		})
	case "caddy":
		return installCaddy()
	case "git":
		return installGit()
	default:
		return "", fmt.Errorf("unknown runtime: %s", name)
	}
}

type installOption struct {
	check string
	args  []string
}

func runInstall(options []installOption) (string, error) {
	for _, opt := range options {
		if _, err := exec.LookPath(opt.check); err == nil {
			cmd := exec.Command(opt.args[0], opt.args[1:]...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out), fmt.Errorf("install failed: %w\n%s", err, string(out))
			}
			return string(out), nil
		}
	}
	return "", fmt.Errorf("no supported package manager found")
}
