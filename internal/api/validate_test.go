package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

func TestNormalizeDomain(t *testing.T) {
	longDomain := strings.Repeat("a", 60) + "." + strings.Repeat("b", 60) + "." +
		strings.Repeat("c", 60) + "." + strings.Repeat("d", 60) + ".example.com" // 255 chars, valid labels

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{"trims and lowercases", "Example.COM ", "example.com", ""},
		{"strips scheme and path", "https://foo.bar/path", "foo.bar", ""},
		{"strips trailing dot", "foo.bar.", "foo.bar", ""},
		{"strips port", "foo.bar:8080", "foo.bar", ""},
		{"subdomain", "a.b.example.com", "a.b.example.com", ""},
		{"rejects IP", "1.2.3.4", "", "use a domain name, not an IP address"},
		{"rejects no dots", "no-dots", "", "must contain a dot"},
		{"rejects leading hyphen", "-bad.com", "", "not a valid domain name"},
		{"rejects space", "a b.com", "", "not a valid domain name"},
		{"rejects empty", "  ", "", "domain is required"},
		{"rejects too long", longDomain, "", "too long"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeDomain(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("normalizeDomain(%q) = %q, expected error", tc.in, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("normalizeDomain(%q) error = %q, expected it to contain %q", tc.in, err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDomain(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("normalizeDomain(%q) = %q, expected %q", tc.in, got, tc.want)
			}
		})
	}
}

// The Add success path registers the route with Caddy directly, which is not
// available in tests, so only the validation-rejection paths are exercised
// here; normalization itself is covered by TestNormalizeDomain.
func TestAddDomain_RejectsInvalid(t *testing.T) {
	database := testutil.NewTestDB(t)
	seedTestConfig(t, database, "hash", "secret")
	app := seedTestApp(t, database, "webapp", 4000)
	h := NewDomainsHandler(database)

	tests := []struct {
		domain  string
		wantErr string
	}{
		{"1.2.3.4", "use a domain name, not an IP address"},
		{"no-dots", "must contain a dot"},
		{"-bad.com", "not a valid domain name"},
		{"", "domain is required"},
	}

	for _, tc := range tests {
		body, _ := json.Marshal(addDomainRequest{Domain: tc.domain})
		req := httptest.NewRequest("POST", "/api/apps/"+app.ID+"/domains", bytes.NewReader(body))
		req = withURLParam(req, "id", app.ID)
		rr := httptest.NewRecorder()
		h.Add(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Add(%q): expected 400, got %d: %s", tc.domain, rr.Code, rr.Body.String())
			continue
		}
		var resp map[string]string
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if !strings.Contains(resp["error"], tc.wantErr) {
			t.Errorf("Add(%q): error = %q, expected it to contain %q", tc.domain, resp["error"], tc.wantErr)
		}
	}
}
