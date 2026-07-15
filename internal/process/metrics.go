package process

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Metrics struct {
	CPU    float64 `json:"cpu_percent"`
	Memory float64 `json:"memory_mb"`
}

/*
GetMetrics reads CPU and memory for a process tree rooted at the given PID.
Start commands run via sh -c with Setpgid, so the leader pid is the group
id; summing over the process group counts the app and its children instead
of just the shell. Uses ps which works on both Linux and macOS. Returns
zeroes if the process doesn't exist or metrics can't be read.
*/
func GetMetrics(pid int) Metrics {
	if pid <= 0 {
		return Metrics{}
	}

	if out, err := exec.Command("ps", "-eo", "pgid=,pcpu=,rss=").Output(); err == nil {
		if cpu, rssKB, matched := sumGroup(string(out), pid); matched {
			return Metrics{
				CPU:    roundTo(cpu, 1),
				Memory: roundTo(rssKB/1024, 1),
			}
		}
	}

	// Fallback: read the single pid directly.
	out, err := exec.Command(
		"ps", "-p", strconv.Itoa(pid), "-o", "pcpu=,rss=",
	).Output()
	if err != nil {
		return Metrics{}
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return Metrics{}
	}

	cpu, _ := strconv.ParseFloat(fields[0], 64)
	rssKB, _ := strconv.ParseFloat(fields[1], 64)

	return Metrics{
		CPU:    roundTo(cpu, 1),
		Memory: roundTo(rssKB/1024, 1),
	}
}

// sumGroup sums pcpu and rss over rows of `ps -eo pgid=,pcpu=,rss=` output
// whose pgid matches the given process group.
func sumGroup(out string, pgid int) (cpu, rssKB float64, matched bool) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		g, err := strconv.Atoi(fields[0])
		if err != nil || g != pgid {
			continue
		}
		c, _ := strconv.ParseFloat(fields[1], 64)
		r, _ := strconv.ParseFloat(fields[2], 64)
		cpu += c
		rssKB += r
		matched = true
	}
	return cpu, rssKB, matched
}

func (m *Manager) GetAppMetrics(appID string) Metrics {
	m.mu.Lock()
	p, running := m.procs[appID]
	m.mu.Unlock()

	if !running || p.pid <= 0 {
		return Metrics{}
	}

	return GetMetrics(p.pid)
}

func roundTo(val float64, places int) float64 {
	rounded, _ := strconv.ParseFloat(fmt.Sprintf("%.*f", places, val), 64)
	return rounded
}
