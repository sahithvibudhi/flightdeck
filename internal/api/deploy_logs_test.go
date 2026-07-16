package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
)

func TestDeployLogBuf_SplitsLinesAndFansOut(t *testing.T) {
	hub := newDeployLogHub()
	b := hub.start("dep1")

	fmt.Fprint(b, "one\ntwo\npart")
	if len(b.lines) != 2 || b.lines[0] != "one" || b.lines[1] != "two" {
		t.Fatalf("unexpected lines: %#v", b.lines)
	}
	if b.partial != "part" {
		t.Fatalf("expected partial %q, got %q", "part", b.partial)
	}

	snapshot, ch := b.snapshotAndSubscribe()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot: %#v", snapshot)
	}

	fmt.Fprint(b, "ial\nthree\n")
	if got := <-ch; got != "partial" {
		t.Errorf("subscriber got %q, want %q", got, "partial")
	}
	if got := <-ch; got != "three" {
		t.Errorf("subscriber got %q, want %q", got, "three")
	}

	if text := b.text(); text != "one\ntwo\npartial\nthree" {
		t.Errorf("text: %q", text)
	}
	b.unsubscribe(ch)

	hub.finish("dep1")
	if hub.get("dep1") != nil {
		t.Error("buffer still registered after finish")
	}
	select {
	case <-b.done:
	default:
		t.Error("done channel not closed after finish")
	}
}

// A full manual deploy of a non-git app must capture the restart stages
// and serve them from the logs endpoint after the deploy finishes.
func TestDeploymentLogs_CapturedAndServed(t *testing.T) {
	h, database := setupAppsHandler(t)
	app, err := dbpkg.InsertApp(database, "logapp", "sleep 60", "echo building-now", "", 4790,
		t.TempDir()+"/app.log", sql.NullString{}, sql.NullString{})
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.DeployNow(rr, withURLParam(httptest.NewRequest("POST", "/x", nil), "id", app.ID))
	if rr.Code != http.StatusAccepted {
		t.Fatalf("deploy: expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	depID := resp["deployment_id"]

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		deps, _ := dbpkg.ListDeployments(database, app.ID, 5)
		if len(deps) > 0 && deps[0].Status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Cleanup(func() { h.pm.StopApp(app.ID) })

	lr := httptest.NewRecorder()
	lreq := withRollbackParams(httptest.NewRequest("GET", "/x", nil), app.ID, depID)
	h.DeploymentLogs(lr, lreq)
	if lr.Code != http.StatusOK {
		t.Fatalf("logs: expected 200, got %d: %s", lr.Code, lr.Body.String())
	}

	var logs struct {
		Lines   []string `json:"lines"`
		Running bool     `json:"running"`
	}
	json.Unmarshal(lr.Body.Bytes(), &logs)
	joined := strings.Join(logs.Lines, "\n")
	if logs.Running {
		t.Error("deployment should be finished")
	}
	if !strings.Contains(joined, "building-now") {
		t.Errorf("build output missing from deploy log: %q", joined)
	}
	if !strings.Contains(joined, "Deploy complete") {
		t.Errorf("stage line missing from deploy log: %q", joined)
	}

	// The SSE endpoint serves the stored log for finished deployments
	// and terminates with a done event.
	sr := httptest.NewRecorder()
	sreq := withRollbackParams(httptest.NewRequest("GET", "/x", nil), app.ID, depID)
	h.DeploymentLogsStream(sr, sreq)
	body := sr.Body.String()
	if !strings.Contains(body, "data: === Deploy complete ===") {
		t.Errorf("stream missing stored log: %q", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Errorf("stream missing done event: %q", body)
	}
}
