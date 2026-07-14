package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/git"
)

type deploymentResponse struct {
	ID          string  `json:"id"`
	TriggeredBy string  `json:"triggered_by"`
	Status      string  `json:"status"`
	Detail      string  `json:"detail"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at"`
}

/*
runDeploy records a deployment and runs the pull → build → restart
pipeline in the background. Restarts are zero-downtime when the app has
a health check configured (see Manager.DeployRestart).
*/
func (h *AppsHandler) runDeploy(app *db.App, trigger string) (string, error) {
	depID, err := db.InsertDeployment(h.database, app.ID, trigger)
	if err != nil {
		return "", err
	}

	go func() {
		var detail string
		if app.RepoURL.Valid {
			out, err := git.Pull(h.appDir(app), h.gitToken())
			if err != nil {
				db.FinishDeployment(h.database, depID, "failed", err.Error())
				return
			}
			detail = out
		}
		if err := h.pm.DeployRestart(app); err != nil {
			db.FinishDeployment(h.database, depID, "failed", strings.TrimSpace(detail+"\n"+err.Error()))
			return
		}
		db.FinishDeployment(h.database, depID, "success", detail)
	}()

	return depID, nil
}

func (h *AppsHandler) DeployNow(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	depID, err := h.runDeploy(app, "manual")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record deployment"})
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

	depID, err := h.runDeploy(app, "webhook")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record deployment"})
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
