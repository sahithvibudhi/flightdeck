package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
)

func TestWebhookAuthorized_ValidSignature(t *testing.T) {
	secret := "topsecret"
	body := []byte(`{"ref":"refs/heads/main"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	r := httptest.NewRequest("POST", "/hooks/x", bytes.NewReader(body))
	r.Header.Set("X-Hub-Signature-256", sig)

	if !webhookAuthorized(r, secret, body) {
		t.Error("expected valid signature to be accepted")
	}
}

func TestWebhookAuthorized_InvalidSignature(t *testing.T) {
	body := []byte(`{}`)
	r := httptest.NewRequest("POST", "/hooks/x", bytes.NewReader(body))
	r.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")

	if webhookAuthorized(r, "topsecret", body) {
		t.Error("expected invalid signature to be rejected")
	}
}

func TestWebhookAuthorized_QuerySecret(t *testing.T) {
	body := []byte(`{}`)
	r := httptest.NewRequest("POST", "/hooks/x?secret=topsecret", bytes.NewReader(body))
	if !webhookAuthorized(r, "topsecret", body) {
		t.Error("expected matching query secret to be accepted")
	}

	r = httptest.NewRequest("POST", "/hooks/x?secret=wrong", bytes.NewReader(body))
	if webhookAuthorized(r, "topsecret", body) {
		t.Error("expected wrong query secret to be rejected")
	}
}

func TestWebhookAuthorized_NoCredentials(t *testing.T) {
	body := []byte(`{}`)
	r := httptest.NewRequest("POST", "/hooks/x", bytes.NewReader(body))
	if webhookAuthorized(r, "topsecret", body) {
		t.Error("expected request without credentials to be rejected")
	}
}

func TestWebhook_RejectsBadSecret(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "hookapp", 4000)
	if err := dbpkg.SetWebhookSecret(db, app.ID, "topsecret"); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}

	rr := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest("POST", "/hooks/"+app.ID+"?secret=nope", bytes.NewReader([]byte(`{}`))), "id", app.ID)
	h.Webhook(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCreateApp_GeneratesWebhookSecret(t *testing.T) {
	h, _ := setupAppsHandler(t)

	body := []byte(`{"name":"hooked","start_command":"echo hi"}`)
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest("POST", "/api/apps", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("webhook_secret")) {
		t.Error("expected webhook_secret in response")
	}
}

func TestDeployments_RecordAndList(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "depapp", 4000)

	id, err := dbpkg.InsertDeployment(db, app.ID, "webhook")
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	if err := dbpkg.FinishDeployment(db, id, "success", "pulled"); err != nil {
		t.Fatalf("finish deployment: %v", err)
	}

	rr := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest("GET", "/api/apps/"+app.ID+"/deployments", nil), "id", app.ID)
	h.ListDeployments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"status":"success"`)) {
		t.Errorf("expected success deployment in list, got %s", rr.Body.String())
	}
}
