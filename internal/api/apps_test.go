package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	dbpkg "github.com/nestops/nestops/internal/db"
	"github.com/nestops/nestops/internal/process"
	"github.com/nestops/nestops/internal/testutil"
)

func setupAppsHandler(t *testing.T) (*AppsHandler, *sql.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	dir := t.TempDir()
	pm := process.NewManager(db, dir)
	SetRouteRemover(func(id string) error { return nil })
	return NewAppsHandler(db, pm, dir), db
}

func TestListApps_Empty(t *testing.T) {
	h, _ := setupAppsHandler(t)
	req := httptest.NewRequest("GET", "/api/apps", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var apps []appResponse
	json.Unmarshal(rr.Body.Bytes(), &apps)
	if len(apps) != 0 {
		t.Errorf("expected empty list, got %d", len(apps))
	}
}

func TestListApps_WithApps(t *testing.T) {
	h, db := setupAppsHandler(t)
	seedTestApp(t, db, "app1", 4000)
	seedTestApp(t, db, "app2", 4001)

	req := httptest.NewRequest("GET", "/api/apps", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	var apps []appResponse
	json.Unmarshal(rr.Body.Bytes(), &apps)
	if len(apps) != 2 {
		t.Errorf("expected 2, got %d", len(apps))
	}
}

func TestCreateApp_Manual(t *testing.T) {
	h, _ := setupAppsHandler(t)
	body, _ := json.Marshal(createAppRequest{Name: "myapp", StartCmd: "node index.js"})
	req := httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var app appResponse
	json.Unmarshal(rr.Body.Bytes(), &app)
	if app.Name != "myapp" {
		t.Errorf("expected myapp, got %s", app.Name)
	}
	if app.Port != 4000 {
		t.Errorf("expected 4000, got %d", app.Port)
	}
}

func TestCreateApp_MissingFields(t *testing.T) {
	h, _ := setupAppsHandler(t)
	body, _ := json.Marshal(createAppRequest{Name: "myapp"})
	req := httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCreateApp_DuplicateName(t *testing.T) {
	h, db := setupAppsHandler(t)
	seedTestApp(t, db, "myapp", 4000)

	body, _ := json.Marshal(createAppRequest{Name: "myapp", StartCmd: "echo"})
	req := httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestGetApp(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "myapp", 4000)

	req := withURLParam(httptest.NewRequest("GET", "/api/apps/"+app.ID, nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp appResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Name != "myapp" {
		t.Errorf("expected myapp, got %s", resp.Name)
	}
}

func TestGetApp_NotFound(t *testing.T) {
	h, _ := setupAppsHandler(t)
	req := withURLParam(httptest.NewRequest("GET", "/api/apps/nope", nil), "id", "nope")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDeleteApp(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "myapp", 4000)

	req := withURLParam(httptest.NewRequest("DELETE", "/api/apps/"+app.ID, nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	_, err := dbpkg.GetApp(db, app.ID)
	if err == nil {
		t.Error("app should be deleted")
	}
}

func TestPullApp_NoRepo(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "myapp", 4000)

	req := withURLParam(httptest.NewRequest("POST", "/api/apps/"+app.ID+"/pull", nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.Pull(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestLogs_NoFile(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "myapp", 4000)

	req := withURLParam(httptest.NewRequest("GET", "/api/apps/"+app.ID+"/logs", nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.Logs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string][]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp["lines"]) != 0 {
		t.Errorf("expected empty, got %d", len(resp["lines"]))
	}
}

func TestLogs_WithFile(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	dir := t.TempDir()
	pm := process.NewManager(db, dir)
	SetRouteRemover(func(id string) error { return nil })

	logPath := dir + "/test.log"
	os.WriteFile(logPath, []byte("line1\nline2\nline3\n"), 0644)
	app, _ := dbpkg.InsertApp(db, "logapp", "echo", 4000, logPath, sql.NullString{}, sql.NullString{})
	h := NewAppsHandler(db, pm, dir)

	req := withURLParam(httptest.NewRequest("GET", "/api/apps/"+app.ID+"/logs?lines=2", nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.Logs(rr, req)

	var resp map[string][]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp["lines"]) != 2 {
		t.Errorf("expected 2 lines, got %d", len(resp["lines"]))
	}
}
