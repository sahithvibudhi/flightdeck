package db

import (
	"database/sql"
	"testing"
)

func TestDeploymentCommitRoundTrip(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	app, err := InsertApp(database, "app", "echo hi", "", "", 4000, "/tmp/app.log", sql.NullString{}, sql.NullString{})
	if err != nil {
		t.Fatalf("insert app: %v", err)
	}

	depID, err := InsertDeployment(database, app.ID, "webhook")
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}

	if err := SetDeploymentCommit(database, depID, "abc123def456", "fix login bug"); err != nil {
		t.Fatalf("set commit: %v", err)
	}
	if err := FinishDeployment(database, depID, "success", "pulled"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	deps, err := ListDeployments(database, app.ID, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deps))
	}
	if deps[0].CommitSHA != "abc123def456" || deps[0].CommitMsg != "fix login bug" {
		t.Errorf("commit info lost: %q %q", deps[0].CommitSHA, deps[0].CommitMsg)
	}
}
