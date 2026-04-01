package system

import (
	"database/sql"
	"testing"

	"github.com/nestops/nestops/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := InitMetricsTable(database); err != nil {
		t.Fatalf("init metrics: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestInitMetricsTable(t *testing.T) {
	database := openTestDB(t)

	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM server_metrics`).Scan(&count)
	if err != nil {
		t.Fatalf("table should exist: %v", err)
	}
}

func TestCollectAndStore(t *testing.T) {
	database := openTestDB(t)
	CollectAndStore(database)

	history := GetHistory(database)
	if len(history.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(history.Snapshots))
	}
	if history.Snapshots[0].MemoryTotalMB <= 0 {
		t.Error("expected positive total memory")
	}
}

func TestGetHistory_Empty(t *testing.T) {
	database := openTestDB(t)
	history := GetHistory(database)
	if len(history.Snapshots) != 0 {
		t.Errorf("expected 0, got %d", len(history.Snapshots))
	}
}

func TestGetHistory_Order(t *testing.T) {
	database := openTestDB(t)
	CollectAndStore(database)
	CollectAndStore(database)
	CollectAndStore(database)

	history := GetHistory(database)
	if len(history.Snapshots) != 3 {
		t.Fatalf("expected 3, got %d", len(history.Snapshots))
	}
	if history.Snapshots[0].Timestamp > history.Snapshots[2].Timestamp {
		t.Error("expected chronological order")
	}
}

func TestCleanup(t *testing.T) {
	database := openTestDB(t)
	database.Exec(`INSERT INTO server_metrics (cpu, mem_used, mem_total, disk_used, disk_total, created_at) VALUES (1, 1, 1, 1, 1, datetime('now', '-25 hours'))`)
	database.Exec(`INSERT INTO server_metrics (cpu, mem_used, mem_total, disk_used, disk_total, created_at) VALUES (2, 2, 2, 2, 2, datetime('now'))`)

	Cleanup(database)

	history := GetHistory(database)
	if len(history.Snapshots) != 1 {
		t.Fatalf("expected 1 after cleanup, got %d", len(history.Snapshots))
	}
	if history.Snapshots[0].CPUPercent != 2 {
		t.Error("wrong snapshot survived cleanup")
	}
}
