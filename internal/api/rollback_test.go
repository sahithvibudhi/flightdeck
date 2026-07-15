package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
)

// withURLParam replaces the whole route context, so chaining it loses
// earlier params; this sets both at once.
func withRollbackParams(r *http.Request, appID, depID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", appID)
	rctx.URLParams.Add("depID", depID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	base := []string{"-c", "user.name=test", "-c", "user.email=test@test"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestRollback_ResetsToRecordedCommit(t *testing.T) {
	h, db := setupAppsHandler(t)

	// A git-backed app whose managed directory holds a real repo with
	// two commits.
	appDir := filepath.Join(h.dataDir, "apps", "rollapp")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, appDir, "init")
	os.WriteFile(filepath.Join(appDir, "f.txt"), []byte("v1"), 0644)
	gitCmd(t, appDir, "add", ".")
	gitCmd(t, appDir, "commit", "-m", "first")
	firstSHA := gitCmd(t, appDir, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(appDir, "f.txt"), []byte("v2"), 0644)
	gitCmd(t, appDir, "add", ".")
	gitCmd(t, appDir, "commit", "-m", "second")

	app, err := dbpkg.InsertApp(db, "rollapp", "echo hi", "", "", 4000, filepath.Join(appDir, "app.log"),
		sql.NullString{String: "https://example.com/x.git", Valid: true},
		sql.NullString{String: "main", Valid: true})
	if err != nil {
		t.Fatalf("insert app: %v", err)
	}

	depID, err := dbpkg.InsertDeployment(db, app.ID, "webhook")
	if err != nil {
		t.Fatal(err)
	}
	dbpkg.SetDeploymentCommit(db, depID, firstSHA, "first")
	dbpkg.FinishDeployment(db, depID, "success", "")

	rr := httptest.NewRecorder()
	req := withRollbackParams(httptest.NewRequest("POST", "/api/apps/"+app.ID+"/deployments/"+depID+"/rollback", nil), app.ID, depID)
	h.Rollback(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// The pipeline runs in the background; wait for the rollback
	// deployment to finish. Timestamps have second resolution, so find
	// it by trigger instead of relying on order.
	findRollback := func() *dbpkg.Deployment {
		deps, _ := dbpkg.ListDeployments(db, app.ID, 10)
		for i := range deps {
			if deps[i].TriggeredBy == "rollback" {
				return &deps[i]
			}
		}
		return nil
	}

	var rb *dbpkg.Deployment
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rb = findRollback()
		if rb != nil && rb.Status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if rb == nil || rb.Status != "success" {
		t.Fatalf("rollback deployment not successful: %+v", rb)
	}
	if rb.CommitSHA != firstSHA {
		t.Errorf("rollback recorded wrong commit: %s != %s", rb.CommitSHA, firstSHA)
	}

	content, _ := os.ReadFile(filepath.Join(appDir, "f.txt"))
	if string(content) != "v1" {
		t.Errorf("working tree not reset: f.txt = %q", content)
	}

	h.pm.StopApp(app.ID)
}

func TestRollback_Validation(t *testing.T) {
	h, db := setupAppsHandler(t)

	// Not git-backed.
	plain := seedTestApp(t, db, "plainapp", 4000)
	dep, _ := dbpkg.InsertDeployment(db, plain.ID, "manual")
	rr := httptest.NewRecorder()
	req := withRollbackParams(httptest.NewRequest("POST", "/x", nil), plain.ID, dep)
	h.Rollback(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("non-git app: expected 400, got %d", rr.Code)
	}

	// Deployment without a recorded commit.
	gitApp, err := dbpkg.InsertApp(db, "gitapp2", "echo hi", "", "", 4001, "/tmp/gitapp2.log",
		sql.NullString{String: "https://example.com/y.git", Valid: true},
		sql.NullString{String: "main", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	dep2, _ := dbpkg.InsertDeployment(db, gitApp.ID, "manual")
	rr = httptest.NewRecorder()
	req = withRollbackParams(httptest.NewRequest("POST", "/x", nil), gitApp.ID, dep2)
	h.Rollback(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("no commit sha: expected 400, got %d", rr.Code)
	}

	// Deployment belonging to a different app.
	other, _ := dbpkg.InsertDeployment(db, plain.ID, "manual")
	rr = httptest.NewRecorder()
	req = withRollbackParams(httptest.NewRequest("POST", "/x", nil), gitApp.ID, other)
	h.Rollback(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("foreign deployment: expected 404, got %d", rr.Code)
	}
}
