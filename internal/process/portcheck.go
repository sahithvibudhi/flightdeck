package process

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// An app that hasn't been up this long isn't checked: servers routinely
// take a few seconds to bind, and a false "not listening" warning during
// every deploy would train people to ignore the real one.
const portCheckGrace = 10 * time.Second

/*
PortCheck verifies that the app's process group actually listens on the
given port, entirely from /proc (no lsof or ss dependency). Returns
"ok", "mismatch" plus the ports the group does listen on, or "" when
nothing can be said: app not running, still inside the startup grace
period, or not on Linux.

This catches the classic first-deploy failure where the app ignores
$PORT and binds its hardcoded default, which looks healthy in the
process list while every request 502s.
*/
func (m *Manager) PortCheck(appID string, port int) (string, []int) {
	m.mu.Lock()
	p, running := m.procs[appID]
	m.mu.Unlock()

	if !running || p.pid <= 0 || time.Since(p.startedAt) < portCheckGrace {
		return "", nil
	}

	inodeToPort := listenInodes()
	if inodeToPort == nil {
		return "", nil
	}

	ports := listeningPorts(groupPids(p.pid), inodeToPort)
	for _, lp := range ports {
		if lp == port {
			return "ok", ports
		}
	}
	return "mismatch", ports
}

/*
listenInodes parses /proc/net/tcp and /proc/net/tcp6 into a map of
socket inode to local port for sockets in LISTEN state (st 0A). Returns
nil when neither file is readable (non-Linux), which callers treat as
"unknown" rather than "listening on nothing".
*/
func listenInodes() map[uint64]int {
	var found bool
	inodes := make(map[uint64]int)

	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		found = true

		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] { // first line is the header
			// sl local_address rem_address st ... inode
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			colon := strings.LastIndex(fields[1], ":")
			if colon < 0 {
				continue
			}
			port, err := strconv.ParseInt(fields[1][colon+1:], 16, 32)
			if err != nil {
				continue
			}
			inode, err := strconv.ParseUint(fields[9], 10, 64)
			if err != nil {
				continue
			}
			inodes[inode] = int(port)
		}
	}

	if !found {
		return nil
	}
	return inodes
}

/*
groupPids returns the pids whose process group matches pgid, by reading
/proc/<pid>/stat. The comm field can contain spaces and parentheses, so
parsing starts after the last ')': state pgid is the second field from
there.
*/
func groupPids(pgid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}
		rest := string(data)
		if i := strings.LastIndex(rest, ")"); i >= 0 {
			rest = rest[i+1:]
		}
		fields := strings.Fields(rest)
		// After ')': state ppid pgrp ...
		if len(fields) < 3 {
			continue
		}
		if g, err := strconv.Atoi(fields[2]); err == nil && g == pgid {
			pids = append(pids, pid)
		}
	}
	return pids
}

/*
listeningPorts maps the given pids' open socket fds through the inode
table, returning the sorted unique local ports the pids listen on.
*/
func listeningPorts(pids []int, inodeToPort map[uint64]int) []int {
	seen := make(map[int]bool)
	for _, pid := range pids {
		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // process gone, or not ours to inspect
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]"), 10, 64)
			if err != nil {
				continue
			}
			if port, ok := inodeToPort[inode]; ok {
				seen[port] = true
			}
		}
	}

	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}
