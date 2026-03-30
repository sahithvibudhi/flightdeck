package git

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

/*
Clone fetches a repository into targetDir. When a token is provided,
it's injected into the HTTPS URL so the clone works against private repos
without needing SSH keys or credential helpers on the VPS.
*/
func Clone(repoURL, targetDir, branch, token string) error {
	cloneURL, err := authenticatedURL(repoURL, token)
	if err != nil {
		return err
	}

	args := []string{"clone", "--depth", "1", "--branch", branch, cloneURL, targetDir}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

/*
Pull runs git pull in the given directory and returns
the output so callers can show what changed.
*/
func Pull(dir string) (string, error) {
	cmd := exec.Command("git", "pull")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git pull failed: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func authenticatedURL(repoURL, token string) (string, error) {
	if token == "" {
		return repoURL, nil
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid repo URL: %w", err)
	}

	u.User = url.UserPassword("x-access-token", token)
	return u.String(), nil
}
