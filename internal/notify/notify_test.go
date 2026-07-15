package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/testutil"
)

type capture struct {
	mu     sync.Mutex
	bodies map[string]string
}

func (c *capture) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.bodies[r.URL.Path] = string(body)
		c.mu.Unlock()
		w.WriteHeader(200)
	}
}

func (c *capture) get(path string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bodies[path]
}

func TestSend_PostsToConfiguredChannels(t *testing.T) {
	cap := &capture{bodies: map[string]string{}}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	oldBase := telegramBase
	telegramBase = srv.URL
	defer func() { telegramBase = oldBase }()

	database := testutil.NewTestDB(t)
	testutil.SeedConfig(t, database)
	if err := db.UpdateNotifications(database, srv.URL+"/discord", "tok123", "chat456", srv.URL+"/hook"); err != nil {
		t.Fatal(err)
	}

	if !Configured(database) {
		t.Fatal("expected Configured to be true")
	}

	if err := Send(database, "Deploy succeeded: demo", "abc1234 fix things"); err != nil {
		t.Fatalf("send: %v", err)
	}

	if !strings.Contains(cap.get("/discord"), "Deploy succeeded: demo") {
		t.Errorf("discord body missing title: %q", cap.get("/discord"))
	}

	var tg map[string]string
	json.Unmarshal([]byte(cap.get("/bottok123/sendMessage")), &tg)
	if tg["chat_id"] != "chat456" || !strings.Contains(tg["text"], "abc1234") {
		t.Errorf("telegram payload wrong: %v", tg)
	}

	var wh map[string]string
	json.Unmarshal([]byte(cap.get("/hook")), &wh)
	if wh["title"] != "Deploy succeeded: demo" || wh["timestamp"] == "" {
		t.Errorf("webhook payload wrong: %v", wh)
	}
}

func TestSend_ReportsChannelErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	database := testutil.NewTestDB(t)
	testutil.SeedConfig(t, database)
	db.UpdateNotifications(database, srv.URL, "", "", "")

	err := Send(database, "t", "m")
	if err == nil || !strings.Contains(err.Error(), "discord") {
		t.Errorf("expected discord error, got %v", err)
	}
}

func TestConfigured_FalseWhenEmpty(t *testing.T) {
	database := testutil.NewTestDB(t)
	testutil.SeedConfig(t, database)
	if Configured(database) {
		t.Error("expected Configured to be false with no channels set")
	}
}
