package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
)

func TestCreateApp_RejectsInvalidName(t *testing.T) {
	h, _ := setupAppsHandler(t)

	for _, name := range []string{"My App", "UPPER", "-leading", "sp ace"} {
		body, _ := json.Marshal(map[string]interface{}{"name": name, "start_command": "echo hi"})
		rr := httptest.NewRecorder()
		h.Create(rr, httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body)))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("name %q: expected 400, got %d", name, rr.Code)
		}
	}
}

func TestCreateApp_RejectsMissingWorkDir(t *testing.T) {
	h, _ := setupAppsHandler(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "pathapp", "start_command": "echo hi", "work_dir": "/nonexistent/path/xyz",
	})
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing work_dir, got %d", rr.Code)
	}
}

func TestCreateApp_WorkDirApp(t *testing.T) {
	h, _ := setupAppsHandler(t)
	dir := t.TempDir()

	body, _ := json.Marshal(map[string]interface{}{
		"name": "pathapp", "start_command": "echo hi", "work_dir": dir,
	})
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp appResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.WorkDir != dir {
		t.Errorf("expected work_dir %q, got %q", dir, resp.WorkDir)
	}
}

func TestUpdateApp_PortChangeRewiresDomains(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "app1", 4000)
	if _, err := dbpkg.InsertDomain(db, app.ID, "example.com"); err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	var removed []string
	var added []struct {
		domain string
		port   int
	}
	SetRouteRemover(func(id string) error { removed = append(removed, id); return nil })
	SetRouteAdder(func(id, domain string, port int) error {
		added = append(added, struct {
			domain string
			port   int
		}{domain, port})
		return nil
	})
	t.Cleanup(func() {
		SetRouteRemover(func(id string) error { return nil })
		SetRouteAdder(func(id, domain string, port int) error { return nil })
	})

	body, _ := json.Marshal(map[string]interface{}{"port": 4500})
	rr := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest("PUT", "/api/apps/"+app.ID, bytes.NewReader(body)), "id", app.ID)
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(removed) != 1 || len(added) != 1 {
		t.Fatalf("expected 1 route removed and 1 added, got %d/%d", len(removed), len(added))
	}
	if added[0].domain != "example.com" || added[0].port != 4500 {
		t.Errorf("route re-added with wrong target: %+v", added[0])
	}
}

// A partial update must not wipe fields it doesn't mention. This bit us
// live: PUT {"build_command":"..."} cleared repo_url and branch, silently
// turning a git-linked app into a manual one.
func TestUpdateApp_PartialUpdatePreservesOmittedFields(t *testing.T) {
	h, database := setupAppsHandler(t)
	repo := sql.NullString{String: "https://example.com/repo.git", Valid: true}
	branch := sql.NullString{String: "prod", Valid: true}
	app, err := dbpkg.InsertApp(database, "app1", "echo hi", "npm ci", "", 4000, "/tmp/app1.log", repo, branch)
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}

	update := func(payload string) *dbpkg.App {
		t.Helper()
		rr := httptest.NewRecorder()
		req := withURLParam(httptest.NewRequest("PUT", "/api/apps/"+app.ID, bytes.NewReader([]byte(payload))), "id", app.ID)
		h.Update(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("update %s: expected 200, got %d: %s", payload, rr.Code, rr.Body.String())
		}
		got, err := dbpkg.GetApp(database, app.ID)
		if err != nil {
			t.Fatalf("reload app: %v", err)
		}
		return got
	}

	got := update(`{"build_command":"exit 1"}`)
	if !got.RepoURL.Valid || got.RepoURL.String != repo.String {
		t.Errorf("repo_url wiped by unrelated update: %+v", got.RepoURL)
	}
	if !got.Branch.Valid || got.Branch.String != "prod" {
		t.Errorf("branch wiped by unrelated update: %+v", got.Branch)
	}
	if got.BuildCmd != "exit 1" {
		t.Errorf("build_cmd not updated: %q", got.BuildCmd)
	}

	got = update(`{"start_command":"echo bye"}`)
	if got.BuildCmd != "exit 1" {
		t.Errorf("build_cmd wiped by unrelated update: %q", got.BuildCmd)
	}
	if !got.RepoURL.Valid {
		t.Errorf("repo_url wiped by unrelated update: %+v", got.RepoURL)
	}

	// Branch alone can be retargeted without resending the repo URL.
	got = update(`{"branch":"main"}`)
	if !got.RepoURL.Valid || !got.Branch.Valid || got.Branch.String != "main" {
		t.Errorf("branch-only update failed: repo=%+v branch=%+v", got.RepoURL, got.Branch)
	}

	// Explicit empty string still clears the git linkage.
	got = update(`{"repo_url":""}`)
	if got.RepoURL.Valid || got.Branch.Valid {
		t.Errorf("empty repo_url should clear linkage: repo=%+v branch=%+v", got.RepoURL, got.Branch)
	}

	// And build_command can be cleared explicitly too.
	got = update(`{"build_command":""}`)
	if got.BuildCmd != "" {
		t.Errorf("empty build_command should clear it: %q", got.BuildCmd)
	}
}

func TestUpdateApp_RejectsRenameToInvalidName(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "app1", 4000)

	body, _ := json.Marshal(map[string]interface{}{"name": "Bad Name"})
	rr := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest("PUT", "/api/apps/"+app.ID, bytes.NewReader(body)), "id", app.ID)
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
