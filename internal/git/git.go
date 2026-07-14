package git

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

/*
Clone fetches a repository into targetDir. Credentials are passed to git
through GIT_CONFIG_* environment variables rather than embedded in the URL,
so the token never appears in the process list, the on-disk remote URL,
or git's error output.
*/
func Clone(repoURL, targetDir, branch, token string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, repoURL, targetDir)
	cmd.Env = gitEnv(token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s", redact(string(out), token))
	}
	return nil
}

/*
Pull runs git pull in the given directory and returns
the output so callers can show what changed.
*/
func Pull(dir, token string) (string, error) {
	cmd := exec.Command("git", "pull")
	cmd.Dir = dir
	cmd.Env = gitEnv(token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git pull failed: %s", redact(string(out), token))
	}
	return strings.TrimSpace(redact(string(out), token)), nil
}

/*
Head returns the current commit's full SHA and subject line, so deploys
can record exactly what was shipped.
*/
func Head(dir string) (sha, subject string, err error) {
	cmd := exec.Command("git", "log", "-1", "--format=%H%n%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git log failed: %w", err)
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	sha = lines[0]
	if len(lines) > 1 {
		subject = lines[1]
	}
	return sha, subject, nil
}

func gitEnv(token string) []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if token == "" {
		return env
	}
	basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	return append(env,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+basic,
	)
}

func redact(out, token string) string {
	out = strings.TrimSpace(out)
	if token == "" {
		return out
	}
	return strings.ReplaceAll(out, token, "***")
}
