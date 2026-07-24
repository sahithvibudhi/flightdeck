package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
)

func TestRotateWebhookSecret(t *testing.T) {
	h, database := setupAppsHandler(t)
	app := seedTestApp(t, database, "hookapp", 4000)
	if err := dbpkg.SetWebhookSecret(database, app.ID, "oldsecret"); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.RotateWebhookSecret(rr, withURLParam(httptest.NewRequest("POST", "/x", nil), "id", app.ID))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["webhook_secret"] == "" || resp["webhook_secret"] == "oldsecret" {
		t.Fatalf("secret not rotated: %q", resp["webhook_secret"])
	}

	got, err := dbpkg.GetApp(database, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.WebhookSecret != resp["webhook_secret"] {
		t.Errorf("stored secret %q does not match returned %q", got.WebhookSecret, resp["webhook_secret"])
	}

	// Unknown app is a 404.
	rr2 := httptest.NewRecorder()
	h.RotateWebhookSecret(rr2, withURLParam(httptest.NewRequest("POST", "/x", nil), "id", "nope"))
	if rr2.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown app, got %d", rr2.Code)
	}
}
