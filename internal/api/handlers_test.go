package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

func TestListEnvs_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	app := seedTestApp(t, db, "myapp", 4000)
	h := NewEnvsHandler(db)

	req := withURLParam(httptest.NewRequest("GET", "/api/apps/"+app.ID+"/envs", nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var envs []envEntry
	json.Unmarshal(rr.Body.Bytes(), &envs)
	if len(envs) != 0 {
		t.Errorf("expected empty, got %d", len(envs))
	}
}

func TestReplaceEnvs_API(t *testing.T) {
	db := testutil.NewTestDB(t)
	app := seedTestApp(t, db, "myapp", 4000)
	h := NewEnvsHandler(db)

	body, _ := json.Marshal([]envEntry{
		{Key: "PORT", Value: "3000"},
		{Key: "NODE_ENV", Value: "production"},
	})
	req := withURLParam(httptest.NewRequest("PUT", "/api/apps/"+app.ID+"/envs", bytes.NewReader(body)), "id", app.ID)
	rr := httptest.NewRecorder()
	h.Replace(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	envs, _ := dbpkg.ListEnvs(db, app.ID)
	if len(envs) != 2 {
		t.Errorf("expected 2, got %d", len(envs))
	}
}

func TestListDomains_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	app := seedTestApp(t, db, "myapp", 4000)
	h := NewDomainsHandler(db)

	req := withURLParam(httptest.NewRequest("GET", "/api/apps/"+app.ID+"/domains", nil), "id", app.ID)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var domains []domainEntry
	json.Unmarshal(rr.Body.Bytes(), &domains)
	if len(domains) != 0 {
		t.Errorf("expected empty, got %d", len(domains))
	}
}

func TestRemoveDomain(t *testing.T) {
	db := testutil.NewTestDB(t)
	app := seedTestApp(t, db, "myapp", 4000)
	dbpkg.InsertDomain(db, app.ID, "test.com")
	h := NewDomainsHandler(db)

	req := httptest.NewRequest("DELETE", "/api/apps/"+app.ID+"/domains/test.com", nil)
	req = withURLParam(req, "id", app.ID)

	rctx := chi.RouteContext(req.Context())
	rctx.URLParams.Add("domain", "test.com")

	rr := httptest.NewRecorder()
	h.Remove(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	domains, _ := dbpkg.ListDomains(db, app.ID)
	if len(domains) != 0 {
		t.Error("domain should be deleted")
	}
}
