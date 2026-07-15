package system

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Snapshot struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsedMB float64 `json:"memory_used_mb"`
	MemoryTotalMB float64 `json:"memory_total_mb"`
	DiskUsedMB   float64 `json:"disk_used_mb"`
	DiskTotalMB  float64 `json:"disk_total_mb"`
	Timestamp    string  `json:"timestamp"`
}

type MetricsHistory struct {
	Snapshots []Snapshot `json:"snapshots"`
}

func InitMetricsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS server_metrics (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		cpu        REAL NOT NULL,
		mem_used   REAL NOT NULL,
		mem_total  REAL NOT NULL,
		disk_used  REAL NOT NULL,
		disk_total REAL NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func CollectAndStore(db *sql.DB) {
	s := collect()
	db.Exec(
		`INSERT INTO server_metrics (cpu, mem_used, mem_total, disk_used, disk_total) VALUES (?, ?, ?, ?, ?)`,
		s.CPUPercent, s.MemoryUsedMB, s.MemoryTotalMB, s.DiskUsedMB, s.DiskTotalMB,
	)
}

func Cleanup(db *sql.DB) {
	db.Exec(`DELETE FROM server_metrics WHERE created_at < datetime('now', '-24 hours')`)
}

func GetHistory(db *sql.DB) MetricsHistory {
	rows, err := db.Query(
		`SELECT cpu, mem_used, mem_total, disk_used, disk_total, created_at
		 FROM server_metrics ORDER BY created_at DESC LIMIT 120`,
	)
	if err != nil {
		return MetricsHistory{Snapshots: []Snapshot{}}
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var s Snapshot
		rows.Scan(&s.CPUPercent, &s.MemoryUsedMB, &s.MemoryTotalMB, &s.DiskUsedMB, &s.DiskTotalMB, &s.Timestamp)
		snapshots = append(snapshots, s)
	}

	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}

	if snapshots == nil {
		snapshots = []Snapshot{}
	}

	return MetricsHistory{Snapshots: snapshots}
}

/*
StartCollector runs a background loop that takes a server snapshot every
30 seconds and drops data older than 24 hours every 10 minutes.
*/
func StartCollector(db *sql.DB) {
	go func() {
		CollectAndStore(db)

		collectTicker := time.NewTicker(30 * time.Second)
		cleanupTicker := time.NewTicker(10 * time.Minute)
		defer collectTicker.Stop()
		defer cleanupTicker.Stop()

		for {
			select {
			case <-collectTicker.C:
				CollectAndStore(db)
			case <-cleanupTicker.C:
				Cleanup(db)
			}
		}
	}()
}

func collect() Snapshot {
	return Snapshot{
		CPUPercent:    getCPU(),
		MemoryUsedMB: getMemUsed(),
		MemoryTotalMB: getMemTotal(),
		DiskUsedMB:   getDiskUsed(),
		DiskTotalMB:  getDiskTotal(),
	}
}

type cpuSample struct {
	busy  uint64
	total uint64
}

var (
	cpuMu       sync.Mutex
	cpuPrev     cpuSample
	cpuHavePrev bool
)

func getCPU() float64 {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("ps", "-A", "-o", "%cpu").Output()
		if err != nil {
			return 0
		}
		var total float64
		for _, line := range strings.Split(string(out), "\n")[1:] {
			v, _ := strconv.ParseFloat(strings.TrimSpace(line), 64)
			total += v
		}
		cpus := float64(runtime.NumCPU())
		if cpus > 0 {
			total = total / cpus
		}
		return roundf(total, 1)
	}

	cur, err := readCPUSample()
	if err != nil {
		return 0
	}

	cpuMu.Lock()
	defer cpuMu.Unlock()
	prev, havePrev := cpuPrev, cpuHavePrev
	cpuPrev, cpuHavePrev = cur, true
	if !havePrev {
		return 0
	}
	return roundf(cpuPercentBetween(prev, cur), 1)
}

// readCPUSample parses the aggregate "cpu " line of /proc/stat:
// user nice system idle iowait irq softirq steal.
func readCPUSample() (cpuSample, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		if len(fields) > 8 {
			fields = fields[:8]
		}
		var total, idle uint64
		for i, f := range fields {
			v, _ := strconv.ParseUint(f, 10, 64)
			total += v
			if i == 3 || i == 4 { // idle, iowait
				idle += v
			}
		}
		return cpuSample{busy: total - idle, total: total}, nil
	}
	return cpuSample{}, fmt.Errorf("no cpu line in /proc/stat")
}

func cpuPercentBetween(prev, cur cpuSample) float64 {
	totalDelta := float64(cur.total) - float64(prev.total)
	if totalDelta <= 0 {
		return 0
	}
	pct := 100 * (float64(cur.busy) - float64(prev.busy)) / totalDelta
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

func getMemUsed() float64 {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("vm_stat").Output()
		if err != nil {
			return 0
		}
		pageSize := 16384.0
		var active, wired, compressed float64
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "page size of") {
				fmt.Sscanf(line, "Mach Virtual Memory Statistics: (page size of %f bytes)", &pageSize)
			}
			if strings.Contains(line, "Pages active") {
				active = parseVMStatValue(line)
			}
			if strings.Contains(line, "Pages wired") {
				wired = parseVMStatValue(line)
			}
			if strings.Contains(line, "Pages occupied by compressor") {
				compressed = parseVMStatValue(line)
			}
		}
		return roundf((active+wired+compressed)*pageSize/1024/1024, 0)
	}

	out, err := exec.Command("sh", "-c",
		`awk '/MemTotal/{t=$2} /MemAvailable/{a=$2} END{printf "%.0f", (t-a)/1024}' /proc/meminfo`,
	).Output()
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return v
}

func getMemTotal() float64 {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		return roundf(v/1024/1024, 0)
	}

	out, err := exec.Command("sh", "-c",
		`awk '/MemTotal/{printf "%.0f", $2/1024}' /proc/meminfo`,
	).Output()
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return v
}

func getDiskUsed() float64 {
	return parseDFColumn("/", 2)
}

func getDiskTotal() float64 {
	return parseDFColumn("/", 1)
}

func parseDFColumn(mount string, col int) float64 {
	out, err := exec.Command("df", "-m", mount).Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0
	}
	fields := strings.Fields(lines[1])
	if len(fields) <= col {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[col], 64)
	return v
}

func parseVMStatValue(line string) float64 {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(strings.TrimRight(parts[1], ".")), 64)
	return v
}

func roundf(val float64, places int) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.*f", places, val), 64)
	return v
}
