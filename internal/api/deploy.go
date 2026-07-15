package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/git"
	"github.com/sahithvibudhi/flightdeck/internal/notify"
)

type deploymentResponse struct {
	ID          string  `json:"id"`
	TriggeredBy string  `json:"triggered_by"`
	Status      string  `json:"status"`
	Detail      string  `json:"detail"`
	CommitSHA   string  `json:"commit_sha"`
	CommitMsg   string  `json:"commit_msg"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at"`
}

/*
runDeploy records a deployment and runs the pull → build → restart
pipeline in the background. Restarts are zero-downtime when the app has
a health check configured (see Manager.DeployRestart).

Deploys are serialized per app: if one is already running, the request
is coalesced into a single pending re-run that starts when the current
one finishes (queued=true, no deployment recorded yet). This prevents
rapid webhook pushes from racing pulls and builds in the same directory
while still guaranteeing the newest code ends up deployed.
*/
func (h *AppsHandler) runDeploy(app *db.App, trigger string) (depID string, queued bool, err error) {
	return h.startDeploy(app, trigger, "")
}

var errDeployInProgress = fmt.Errorf("a deploy is already running for this app")

/*
startDeploy takes the app's deploy slot and runs the pipeline in the
background. When resetSHA is set the working tree is reset to that
commit instead of pulled (rollback); rollbacks are not coalesced, they
fail fast if a deploy is already running.
*/
func (h *AppsHandler) startDeploy(app *db.App, trigger, resetSHA string) (depID string, queued bool, err error) {
	h.deployMu.Lock()
	if h.deploying[app.ID] {
		if resetSHA != "" {
			h.deployMu.Unlock()
			return "", false, errDeployInProgress
		}
		h.pendingDeploy[app.ID] = trigger
		h.deployMu.Unlock()
		return "", true, nil
	}
	h.deploying[app.ID] = true
	h.deployMu.Unlock()

	depID, err = db.InsertDeployment(h.database, app.ID, trigger)
	if err != nil {
		h.deployMu.Lock()
		delete(h.deploying, app.ID)
		h.deployMu.Unlock()
		return "", false, err
	}

	go h.executeDeploy(app, depID, resetSHA)

	return depID, false, nil
}

func (h *AppsHandler) executeDeploy(app *db.App, depID, resetSHA string) {
	var detail, commitLine string

	recordCommit := func() {
		if sha, msg, err := git.Head(h.appDir(app)); err == nil {
			db.SetDeploymentCommit(h.database, depID, sha, msg)
			commitLine = sha[:7] + " " + msg
		}
	}

	failed := func(msg string) {
		db.FinishDeployment(h.database, depID, "failed", msg)
		notify.Go(h.database, "Deploy failed: "+app.Name, strings.TrimSpace(commitLine+"\n"+msg))
	}

	switch {
	case resetSHA != "":
		if err := git.Reset(h.appDir(app), resetSHA); err != nil {
			failed(err.Error())
			h.finishAndRunPending(app.ID)
			return
		}
		recordCommit()
	case app.RepoURL.Valid:
		out, err := git.Pull(h.appDir(app), h.gitToken())
		if err != nil {
			failed(err.Error())
			h.finishAndRunPending(app.ID)
			return
		}
		detail = out
		recordCommit()
	}

	if err := h.pm.DeployRestart(app); err != nil {
		failed(strings.TrimSpace(detail + "\n" + err.Error()))
		h.finishAndRunPending(app.ID)
		return
	}

	db.FinishDeployment(h.database, depID, "success", detail)
	notify.Go(h.database, "Deploy succeeded: "+app.Name, strings.TrimSpace(commitLine))
	h.finishAndRunPending(app.ID)
}

func (h *AppsHandler) finishAndRunPending(appID string) {
	h.deployMu.Lock()
	delete(h.deploying, appID)
	trigger, hasPending := h.pendingDeploy[appID]
	if hasPending {
		delete(h.pendingDeploy, appID)
	}
	h.deployMu.Unlock()

	if !hasPending {
		return
	}

	// Re-fetch: the app may have been reconfigured (or deleted) while
	// the previous deploy was running.
	app, err := db.GetApp(h.database, appID)
	if err != nil {
		return
	}
	h.runDeploy(app, trigger)
}

func (h *AppsHandler) DeployNow(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	depID, queued, err := h.runDeploy(app, "manual")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record deployment"})
		return
	}
	if queued {
		writeJSON(w, http.StatusAccepted, map[string]string{"message": "a deploy is already running — queued another"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"deployment_id": depID})
}

func (h *AppsHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	deps, err := db.ListDeployments(h.database, app.ID, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list deployments"})
		return
	}

	resp := make([]deploymentResponse, 0, len(deps))
	for _, d := range deps {
		item := deploymentResponse{
			ID:          d.ID,
			TriggeredBy: d.TriggeredBy,
			Status:      d.Status,
			Detail:      d.Detail,
			CommitSHA:   d.CommitSHA,
			CommitMsg:   d.CommitMsg,
			StartedAt:   d.StartedAt,
		}
		if d.FinishedAt.Valid {
			item.FinishedAt = &d.FinishedAt.String
		}
		resp = append(resp, item)
	}
	writeJSON(w, http.StatusOK, resp)
}

/*
Rollback re-deploys a previous successful deployment by resetting the
working tree to its recorded commit, then running the normal restart
pipeline (zero-downtime when the app has a health check). A later pull
or push-to-deploy fast-forwards back to the branch tip.
*/
func (h *AppsHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if !app.RepoURL.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rollback needs a git-backed app"})
		return
	}

	depID := chi.URLParam(r, "depID")
	deps, err := db.ListDeployments(h.database, app.ID, 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load deployments"})
		return
	}

	var target *db.Deployment
	for i := range deps {
		if deps[i].ID == depID {
			target = &deps[i]
			break
		}
	}
	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found for this app"})
		return
	}
	if target.CommitSHA == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "that deployment has no recorded commit"})
		return
	}

	newDepID, _, err := h.startDeploy(app, "rollback", target.CommitSHA)
	if err == errDeployInProgress {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record deployment"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"deployment_id": newDepID, "message": "rollback started"})
}

/*
Webhook is the push-to-deploy endpoint: POST /hooks/{id}. It is
unauthenticated (no JWT) but every request must prove knowledge of the
app's webhook secret, either via a GitHub-style HMAC signature
(X-Hub-Signature-256 over the raw body) or a ?secret= query parameter
for plain curl/CI use.
*/
func (h *AppsHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if app.WebhookSecret == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "webhooks are not enabled for this app"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	if !webhookAuthorized(r, app.WebhookSecret, body) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid webhook signature"})
		return
	}

	depID, queued, err := h.runDeploy(app, "webhook")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record deployment"})
		return
	}
	if queued {
		writeJSON(w, http.StatusAccepted, map[string]string{"message": "a deploy is already running — queued another"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"deployment_id": depID, "message": "deploy started"})
}

func webhookAuthorized(r *http.Request, secret string, body []byte) bool {
	if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(sig), []byte(expected))
	}
	if q := r.URL.Query().Get("secret"); q != "" {
		return hmac.Equal([]byte(q), []byte(secret))
	}
	return false
}
