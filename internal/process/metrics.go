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
Uses ps which works on both Linux and macOS. Returns zeroes if the
process doesn't exist or metrics can't be read.
*/
func GetMetrics(pid int) Metrics {
	if pid <= 0 {
		return Metrics{}
	}

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
