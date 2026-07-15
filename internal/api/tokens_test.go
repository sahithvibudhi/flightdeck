package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

func TestTokens_CreateListDelete(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	h := NewTokensHandler(db)

	body, _ := json.Marshal(createTokenRequest{Name: "ci", Scope: "deploy"})
	req := httptest.NewRequest("POST", "/api/tokens", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var created createTokenResponse
	json.Unmarshal(rr.Body.Bytes(), &created)
	if !strings.HasPrefix(created.Token, "fd_") {
		t.Errorf("expected plaintext to start with fd_, got %q", created.Token)
	}
	if created.Name != "ci" || created.Scope != "deploy" {
		t.Errorf("unexpected token metadata: %+v", created)
	}

	stored, err := dbpkg.GetAPITokenByHash(db, HashAPIToken(created.Token))
	if err != nil {
		t.Fatalf("lookup by hash: %v", err)
	}
	if stored.Hash == created.Token {
		t.Error("stored hash must not equal the plaintext")
	}

	req2 := httptest.NewRequest("GET", "/api/tokens", nil)
	rr2 := httptest.NewRecorder()
	h.List(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}
	if strings.Contains(rr2.Body.String(), created.Token) {
		t.Error("plaintext token must not appear in the list response")
	}

	var list []tokenEntry
	json.Unmarshal(rr2.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("expected one token with id %s, got %+v", created.ID, list)
	}

	req3 := httptest.NewRequest("DELETE", "/api/tokens/"+created.ID, nil)
	rr3 := httptest.NewRecorder()
	h.Delete(rr3, withURLParam(req3, "id", created.ID))

	if rr3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr3.Code)
	}

	req4 := httptest.NewRequest("GET", "/api/tokens", nil)
	rr4 := httptest.NewRecorder()
	h.List(rr4, req4)

	var after []tokenEntry
	json.Unmarshal(rr4.Body.Bytes(), &after)
	if len(after) != 0 {
		t.Errorf("expected empty list after delete, got %+v", after)
	}
}

func TestTokens_CreateValidation(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")
	h := NewTokensHandler(db)

	cases := []createTokenRequest{
		{Name: "", Scope: "read"},
		{Name: "   ", Scope: "read"},
		{Name: strings.Repeat("a", 61), Scope: "read"},
		{Name: "ok", Scope: "admin"},
	}
	for _, c := range cases {
		body, _ := json.Marshal(c)
		req := httptest.NewRequest("POST", "/api/tokens", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		h.Create(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("name=%q scope=%q: expected 400, got %d", c.Name, c.Scope, rr.Code)
		}
	}
}

func TestTokenAllowed(t *testing.T) {
	cases := []struct {
		scope, method, path string
		want                bool
	}{
		{"read", "GET", "/api/apps", true},
		{"read", "POST", "/api/apps/abc/deploy", false},
		{"deploy", "GET", "/api/apps/abc", true},
		{"deploy", "POST", "/api/apps/abc/start", true},
		{"deploy", "POST", "/api/apps/abc/stop", true},
		{"deploy", "POST", "/api/apps/abc/restart", true},
		{"deploy", "POST", "/api/apps/abc/pull", true},
		{"deploy", "POST", "/api/apps/abc/deploy", true},
		{"deploy", "DELETE", "/api/apps/abc", false},
		{"deploy", "POST", "/api/apps", false},
		{"deploy", "PUT", "/api/apps/abc", false},
		{"deploy", "POST", "/api/settings/domain", false},
		{"read", "GET", "/api/tokens", false},
		{"deploy", "GET", "/api/tokens", false},
		{"deploy", "DELETE", "/api/tokens/abc", false},
	}
	for _, c := range cases {
		if got := tokenAllowed(c.scope, c.method, c.path); got != c.want {
			t.Errorf("tokenAllowed(%q, %q, %q) = %v, want %v", c.scope, c.method, c.path, got, c.want)
		}
	}
}

func TestDBAuthMiddleware_APIToken(t *testing.T) {
	db := testutil.NewTestDB(t)
	seedTestConfig(t, db, "hash", "secret")

	body, _ := json.Marshal(createTokenRequest{Name: "cli", Scope: "read"})
	crr := httptest.NewRecorder()
	NewTokensHandler(db).Create(crr, httptest.NewRequest("POST", "/api/tokens", bytes.NewReader(body)))
	var created createTokenResponse
	json.Unmarshal(crr.Body.Bytes(), &created)

	handler := DBAuthMiddleware(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/apps", nil)
	req.Header.Set("Authorization", "Bearer "+created.Token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("valid fd_ token on GET /api/apps: expected 200, got %d", rr.Code)
	}

	// Same token via ?token= (the SSE path).
	req2 := httptest.NewRequest("GET", "/api/apps?token="+created.Token, nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("fd_ token via query param: expected 200, got %d", rr2.Code)
	}

	// Out-of-scope method is refused, not just unauthenticated.
	req3 := httptest.NewRequest("DELETE", "/api/apps/abc", nil)
	req3.Header.Set("Authorization", "Bearer "+created.Token)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusForbidden {
		t.Errorf("read token on DELETE: expected 403, got %d", rr3.Code)
	}

	// Unknown fd_ token is rejected outright.
	req4 := httptest.NewRequest("GET", "/api/apps", nil)
	req4.Header.Set("Authorization", "Bearer fd_"+strings.Repeat("0", 64))
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusUnauthorized {
		t.Errorf("unknown fd_ token: expected 401, got %d", rr4.Code)
	}
}
