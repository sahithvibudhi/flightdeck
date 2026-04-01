package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nestops/nestops/internal/testutil"
)

func TestSettings_Get(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	h := NewSettingsHandler(db)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp settingsResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.AdminUsername != "admin" {
		t.Errorf("expected admin, got %s", resp.AdminUsername)
	}
	if resp.HasGitToken {
		t.Error("expected no git token")
	}
	if resp.PanelDomain != nil {
		t.Error("expected nil panel domain")
	}
}

func TestSettings_UpdateGitToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	h := NewSettingsHandler(db)

	body, _ := json.Marshal(updateGitTokenRequest{Token: "ghp_test123"})
	req := httptest.NewRequest("PUT", "/api/settings/git-token", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.UpdateGitToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	req2 := httptest.NewRequest("GET", "/api/settings", nil)
	rr2 := httptest.NewRecorder()
	h.Get(rr2, req2)

	var resp settingsResponse
	json.Unmarshal(rr2.Body.Bytes(), &resp)
	if !resp.HasGitToken {
		t.Error("expected has_git_token after setting")
	}
}

func TestSettings_RemoveGitToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	h := NewSettingsHandler(db)

	body, _ := json.Marshal(updateGitTokenRequest{Token: "ghp_test"})
	req := httptest.NewRequest("PUT", "/api/settings/git-token", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.UpdateGitToken(rr, req)

	body2, _ := json.Marshal(updateGitTokenRequest{Token: ""})
	req2 := httptest.NewRequest("PUT", "/api/settings/git-token", bytes.NewReader(body2))
	rr2 := httptest.NewRecorder()
	h.UpdateGitToken(rr2, req2)

	req3 := httptest.NewRequest("GET", "/api/settings", nil)
	rr3 := httptest.NewRecorder()
	h.Get(rr3, req3)

	var resp settingsResponse
	json.Unmarshal(rr3.Body.Bytes(), &resp)
	if resp.HasGitToken {
		t.Error("expected no token after removal")
	}
}

func TestSettings_SystemInfo(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := NewSettingsHandler(db)

	req := httptest.NewRequest("GET", "/api/system", nil)
	rr := httptest.NewRecorder()
	h.SystemInfo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["runtimes"] == nil {
		t.Error("expected runtimes in response")
	}
}

func TestSettings_ServerMetrics(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := NewSettingsHandler(db)

	req := httptest.NewRequest("GET", "/api/system/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServerMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["snapshots"] == nil {
		t.Error("expected snapshots in response")
	}
}
