package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTailLog_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	os.WriteFile(path, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	lines, err := TailLog(path, 3)
	if err != nil {
		t.Fatalf("TailLog: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line3" || lines[1] != "line4" || lines[2] != "line5" {
		t.Errorf("expected last 3 lines, got %v", lines)
	}
}

func TestTailLog_NonExistent(t *testing.T) {
	lines, err := TailLog("/tmp/does-not-exist-"+t.Name(), 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil, got %v", lines)
	}
}

func TestTailLog_FewerLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	os.WriteFile(path, []byte("only\ntwo\n"), 0644)

	lines, err := TailLog(path, 100)
	if err != nil {
		t.Fatalf("TailLog: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}
