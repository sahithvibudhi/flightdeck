package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

func setupEnvsHandler(t *testing.T) (*EnvsHandler, string) {
	t.Helper()
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	app := seedTestApp(t, db, "envapp", 4000)
	return NewEnvsHandler(db), app.ID
}

func replaceEnvsRequest(t *testing.T, h *EnvsHandler, appID string, entries []envEntry) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(entries)
	rr := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest("PUT", "/api/apps/"+appID+"/envs", bytes.NewReader(body)), "id", appID)
	h.Replace(rr, req)
	return rr
}

func TestReplaceEnvs_AcceptsValidKeys(t *testing.T) {
	h, appID := setupEnvsHandler(t)

	rr := replaceEnvsRequest(t, h, appID, []envEntry{
		{Key: "DATABASE_URL", Value: "postgres://localhost/db"},
		{Key: "_private", Value: "x"},
		{Key: "PORT2", Value: "8080"},
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReplaceEnvs_RejectsInvalidKeys(t *testing.T) {
	h, appID := setupEnvsHandler(t)

	for _, key := range []string{"MY KEY", "1BAD", "FOO=BAR", "a-b", ""} {
		rr := replaceEnvsRequest(t, h, appID, []envEntry{{Key: key, Value: "x"}})
		if rr.Code != http.StatusBadRequest {
			t.Errorf("key %q: expected 400, got %d", key, rr.Code)
		}
	}
}

func TestReplaceEnvs_RejectsNewlineValues(t *testing.T) {
	h, appID := setupEnvsHandler(t)

	rr := replaceEnvsRequest(t, h, appID, []envEntry{{Key: "GOOD_KEY", Value: "line1\nline2"}})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for newline in value, got %d", rr.Code)
	}
}
