package process

import (
	"net"
	"os"
	"syscall"
	"testing"
)

// The test process opens a real listener and the /proc plumbing must
// find it: inode table, fd scan, and port extraction all exercised
// against the live kernel rather than fixtures.
func TestListeningPortsFindsOwnListener(t *testing.T) {
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		t.Skip("no /proc/net/tcp on this platform")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	want := ln.Addr().(*net.TCPAddr).Port

	inodes := listenInodes()
	if inodes == nil {
		t.Fatal("listenInodes returned nil on Linux")
	}

	ports := listeningPorts([]int{os.Getpid()}, inodes)
	for _, p := range ports {
		if p == want {
			return
		}
	}
	t.Fatalf("port %d not found in %v", want, ports)
}

func TestGroupPidsIncludesSelfGroup(t *testing.T) {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc on this platform")
	}

	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Skipf("getpgid: %v", err)
	}

	pids := groupPids(pgid)
	for _, p := range pids {
		if p == os.Getpid() {
			return
		}
	}
	t.Fatalf("own pid %d not in group %d listing %v", os.Getpid(), pgid, pids)
}
