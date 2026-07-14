package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSample(t *testing.T) {
	h, _ := setupAppsHandler(t)

	req := httptest.NewRequest("POST", "/api/apps/sample", nil)
	rr := httptest.NewRecorder()
	h.CreateSample(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var app appResponse
	json.Unmarshal(rr.Body.Bytes(), &app)
	t.Cleanup(func() { h.pm.StopApp(app.ID) })

	if app.Name != "hello-flightdeck" {
		t.Errorf("expected hello-flightdeck, got %s", app.Name)
	}
	if app.WebhookSecret == "" {
		t.Error("expected a webhook secret")
	}
	if app.HealthPath != "/" {
		t.Errorf("expected health path /, got %s", app.HealthPath)
	}

	indexPath := filepath.Join(h.dataDir, "apps", "hello-flightdeck", "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		t.Errorf("expected index.html at %s: %v", indexPath, err)
	}

	// A second call must not create a duplicate.
	rr2 := httptest.NewRecorder()
	h.CreateSample(rr2, httptest.NewRequest("POST", "/api/apps/sample", nil))

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 on repeat, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var app2 appResponse
	json.Unmarshal(rr2.Body.Bytes(), &app2)
	if app2.ID != app.ID {
		t.Errorf("expected same app id %s, got %s", app.ID, app2.ID)
	}
}
