package testutil

import (
	"database/sql"
	"testing"

	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/system"
)

func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := system.InitMetricsTable(database); err != nil {
		t.Fatalf("failed to init metrics table: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func SeedConfig(t *testing.T, database *sql.DB) {
	t.Helper()
	err := db.InsertConfig(database, &db.Config{
		AdminUsername: "admin",
		AdminPassword: "$2a$12$LJ3m4ys3Lg6Wk1HjUOiQ3.1HtPiHxjYKxGTjFv5rRBagllzf7.hS",
		JWTSecret:     "test-secret-key-for-testing-only",
	})
	if err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}
}
