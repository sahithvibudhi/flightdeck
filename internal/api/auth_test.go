package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

func TestLogin_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := auth.HashPassword("password123")
	secret := "jwt-secret"
	seedTestConfig(t, db, hash, secret)

	h := NewAuthHandler(db)
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "password123"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}

	username, err := auth.VerifyToken(resp.Token, secret)
	if err != nil {
		t.Fatalf("token verification failed: %v", err)
	}
	if username != "admin" {
		t.Errorf("expected admin, got %s", username)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := auth.HashPassword("password123")
	seedTestConfig(t, db, hash, "secret")

	h := NewAuthHandler(db)
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrong"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLogin_WrongUsername(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := auth.HashPassword("password123")
	seedTestConfig(t, db, hash, "secret")

	h := NewAuthHandler(db)
	body, _ := json.Marshal(loginRequest{Username: "hacker", Password: "password123"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLogin_BadRequest(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := NewAuthHandler(db)

	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	h.Login(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestChangePassword_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := auth.HashPassword("oldpass123")
	seedTestConfig(t, db, hash, "secret")

	h := NewAuthHandler(db)
	body, _ := json.Marshal(changePasswordRequest{Current: "oldpass123", New: "newpass123"})
	req := httptest.NewRequest("POST", "/api/auth/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body2, _ := json.Marshal(loginRequest{Username: "admin", Password: "newpass123"})
	req2 := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body2))
	rr2 := httptest.NewRecorder()
	h.Login(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Error("new password should work for login")
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := auth.HashPassword("realpass")
	seedTestConfig(t, db, hash, "secret")

	h := NewAuthHandler(db)
	body, _ := json.Marshal(changePasswordRequest{Current: "wrongpass", New: "newpass123"})
	req := httptest.NewRequest("POST", "/api/auth/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestChangePassword_TooShort(t *testing.T) {
	db := testutil.NewTestDB(t)
	hash, _ := auth.HashPassword("password123")
	seedTestConfig(t, db, hash, "secret")

	h := NewAuthHandler(db)
	body, _ := json.Marshal(changePasswordRequest{Current: "password123", New: "short"})
	req := httptest.NewRequest("POST", "/api/auth/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.ChangePassword(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
