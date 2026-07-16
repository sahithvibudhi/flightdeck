package api

import (
	"testing"
	"time"

	dbpkg "github.com/sahithvibudhi/flightdeck/internal/db"
)

/*
Deploys for the same app must not run concurrently: a second trigger
while one is in flight is coalesced into a single pending re-run.
*/
func TestRunDeploy_SerializesPerApp(t *testing.T) {
	h, db := setupAppsHandler(t)
	app := seedTestApp(t, db, "serialapp", 4000)

	// Simulate an in-flight deploy so the next trigger must queue.
	h.deployMu.Lock()
	h.deploying[app.ID] = true
	h.deployMu.Unlock()

	depID, queued, err := h.runDeploy(app, "webhook")
	if err != nil {
		t.Fatalf("runDeploy: %v", err)
	}
	if !queued || depID != "" {
		t.Fatalf("expected queued deploy, got queued=%v depID=%q", queued, depID)
	}

	deps, _ := dbpkg.ListDeployments(db, app.ID, 20)
	if len(deps) != 0 {
		t.Fatalf("queued deploy must not record a deployment yet, found %d", len(deps))
	}

	// Finishing the in-flight deploy runs the pending one for real.
	h.finishAndRunPending(app.ID)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		deps, _ = dbpkg.ListDeployments(db, app.ID, 20)
		if len(deps) == 1 && deps[0].Status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(deps) != 1 {
		t.Fatalf("expected exactly 1 deployment after pending run, got %d", len(deps))
	}
	if deps[0].TriggeredBy != "webhook" {
		t.Errorf("pending deploy lost its trigger: %q", deps[0].TriggeredBy)
	}

	h.deployMu.Lock()
	stillDeploying := h.deploying[app.ID]
	_, stillPending := h.pendingDeploy[app.ID]
	h.deployMu.Unlock()
	if stillDeploying || stillPending {
		t.Error("deploy state not cleaned up after pending run finished")
	}
}

func TestRunDeploy_IndependentAppsDontBlock(t *testing.T) {
	h, db := setupAppsHandler(t)
	app1 := seedTestApp(t, db, "app-one", 4000)
	app2 := seedTestApp(t, db, "app-two", 4001)

	h.deployMu.Lock()
	h.deploying[app1.ID] = true
	h.deployMu.Unlock()

	_, queued, err := h.runDeploy(app2, "manual")
	if err != nil {
		t.Fatalf("runDeploy: %v", err)
	}
	if queued {
		t.Error("a deploy on app-one must not queue deploys for app-two")
	}

	// The deploy runs in a background goroutine; let it finish before
	// the test ends or it races t.TempDir cleanup and the DB close.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		deps, _ := dbpkg.ListDeployments(db, app2.ID, 5)
		if len(deps) == 1 && deps[0].Status != "running" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	h.pm.StopApp(app2.ID)
}
