package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

func TestSetupStatus_NeedsSetup(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := NewSetupHandler(db)

	rr := httptest.NewRecorder()
	h.Status(rr, httptest.NewRequest("GET", "/api/setup/status", nil))

	var resp map[string]bool
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp["needs_setup"] {
		t.Error("expected needs_setup=true on fresh database")
	}
}

func TestSetupComplete_CreatesAdminAndIssuesToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := NewSetupHandler(db)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "supersecret"})
	rr := httptest.NewRecorder()
	h.Complete(rr, httptest.NewRequest("POST", "/api/setup", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["token"] == "" {
		t.Error("expected a token in the response")
	}

	rr = httptest.NewRecorder()
	h.Status(rr, httptest.NewRequest("GET", "/api/setup/status", nil))
	var status map[string]bool
	json.Unmarshal(rr.Body.Bytes(), &status)
	if status["needs_setup"] {
		t.Error("expected needs_setup=false after setup")
	}
}

func TestSetupComplete_RejectsWhenAlreadyConfigured(t *testing.T) {
	db := testutil.NewTestDB(t)
	testutil.SeedConfig(t, db)
	h := NewSetupHandler(db)

	body, _ := json.Marshal(map[string]string{"username": "evil", "password": "hackerman1"})
	rr := httptest.NewRecorder()
	h.Complete(rr, httptest.NewRequest("POST", "/api/setup", bytes.NewReader(body)))

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSetupComplete_RejectsShortPassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := NewSetupHandler(db)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "short"})
	rr := httptest.NewRecorder()
	h.Complete(rr, httptest.NewRequest("POST", "/api/setup", bytes.NewReader(body)))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
