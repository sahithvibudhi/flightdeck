package system

import (
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Runtime struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
}

type CaddyStatus struct {
	Running bool   `json:"running"`
	Version string `json:"version,omitempty"`
}

type Info struct {
	Runtimes []Runtime   `json:"runtimes"`
	Caddy    CaddyStatus `json:"caddy"`
	OS       string      `json:"os"`
	Arch     string      `json:"arch"`
}

var checks = []struct {
	name    string
	bin     string
	vFlag   string
	trimmer func(string) string
}{
	{"Git", "git", "--version", func(s string) string { return strings.TrimPrefix(s, "git version ") }},
	{"Node.js", "node", "--version", func(s string) string { return strings.TrimPrefix(s, "v") }},
	{"Python", "python3", "--version", func(s string) string { return strings.TrimPrefix(s, "Python ") }},
	{"Go", "go", "version", func(s string) string {
		parts := strings.Fields(s)
		for _, p := range parts {
			if strings.HasPrefix(p, "go1") {
				return strings.TrimPrefix(p, "go")
			}
		}
		return s
	}},
	{"Docker", "docker", "--version", func(s string) string {
		s = strings.TrimPrefix(s, "Docker version ")
		if i := strings.Index(s, ","); i > 0 {
			return s[:i]
		}
		return s
	}},
	{"Bun", "bun", "--version", nil},
	{"Deno", "deno", "--version", func(s string) string {
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "deno") {
				return strings.TrimPrefix(strings.Fields(line)[1], "v")
			}
		}
		return s
	}},
}

func Detect() Info {
	info := Info{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Runtimes: make([]Runtime, 0, len(checks)),
	}

	for _, c := range checks {
		r := Runtime{Name: c.name}
		out, err := exec.Command(c.bin, c.vFlag).CombinedOutput()
		if err != nil {
			info.Runtimes = append(info.Runtimes, r)
			continue
		}
		r.Installed = true
		ver := strings.TrimSpace(string(out))
		if c.trimmer != nil {
			ver = c.trimmer(ver)
		}
		r.Version = ver
		info.Runtimes = append(info.Runtimes, r)
	}

	info.Caddy = checkCaddy()

	return info
}

func checkCaddy() CaddyStatus {
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("http://localhost:2019/config/")
	if err != nil {
		return CaddyStatus{Running: false}
	}
	resp.Body.Close()

	status := CaddyStatus{Running: true}
	out, err := exec.Command("caddy", "version").Output()
	if err == nil {
		status.Version = strings.Fields(strings.TrimSpace(string(out)))[0]
	}
	return status
}
