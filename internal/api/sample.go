package api

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

const sampleAppName = "hello-flightdeck"

const sampleIndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>hello-flightdeck</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    background: #000;
    color: #ededed;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }
  main { max-width: 560px; }
  h1 { font-size: 32px; margin-bottom: 12px; }
  p.lead { color: #a1a1a1; line-height: 1.6; margin-bottom: 32px; }
  code {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 0.9em;
    background: #111;
    border: 1px solid #262626;
    border-radius: 4px;
    padding: 2px 6px;
    color: #ededed;
  }
  h2 {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: #a1a1a1;
    margin-bottom: 16px;
  }
  ol { list-style: none; counter-reset: step; }
  ol li {
    counter-increment: step;
    display: flex;
    gap: 12px;
    padding: 12px 0;
    border-top: 1px solid #1a1a1a;
    line-height: 1.5;
    color: #d4d4d4;
    font-size: 14px;
  }
  ol li::before {
    content: counter(step);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    color: #a1a1a1;
    border: 1px solid #333;
    border-radius: 50%;
    width: 22px;
    height: 22px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
  }
</style>
</head>
<body>
<main>
  <h1>It works! &#128640;</h1>
  <p class="lead">
    This page is being served by flightdeck from
    <code>/var/flightdeck/apps/hello-flightdeck</code>.
  </p>
  <h2>What to try next</h2>
  <ol>
    <li>Edit this <code>index.html</code> on the server and refresh &mdash; changes are live instantly.</li>
    <li>Add a domain in the dashboard &mdash; SSL certificates are issued automatically.</li>
    <li>Set up push-to-deploy by pasting the app&rsquo;s webhook URL into your GitHub repo.</li>
  </ol>
</main>
</body>
</html>
`

// CreateSample deploys a tiny embedded static site so new users can see
// an app running without bringing their own code. Idempotent: if the
// sample already exists it just returns it.
func (h *AppsHandler) CreateSample(w http.ResponseWriter, r *http.Request) {
	apps, err := db.ListApps(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}
	for i := range apps {
		if apps[i].Name == sampleAppName {
			writeJSON(w, http.StatusOK, h.buildAppResponse(&apps[i]))
			return
		}
	}

	appDir := filepath.Join(h.dataDir, "apps", sampleAppName)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create app directory"})
		return
	}
	if err := os.WriteFile(filepath.Join(appDir, "index.html"), []byte(sampleIndexHTML), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write sample app files"})
		return
	}

	port, err := db.NextPort(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to assign port"})
		return
	}

	logPath := filepath.Join(appDir, "app.log")
	app, err := db.InsertApp(h.database, sampleAppName, "python3 -m http.server $PORT", "", "", port, logPath, sql.NullString{}, sql.NullString{})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "app name already exists or port conflict"})
		return
	}

	// Every app gets a webhook secret so push-to-deploy works out of the box.
	if secret, err := auth.GenerateSecret(); err == nil {
		if err := db.SetWebhookSecret(h.database, app.ID, secret); err == nil {
			app.WebhookSecret = secret
		}
	}

	if err := db.SetHealthPath(h.database, app.ID, "/"); err == nil {
		app.HealthPath = "/"
	}

	// Best-effort start: the sample is still usable from the dashboard
	// even if python3 is missing, so surface the error as a warning.
	var warning string
	if err := h.pm.StartApp(app); err != nil {
		warning = "app created but failed to start: " + err.Error()
	}

	resp := struct {
		appResponse
		Warning string `json:"warning,omitempty"`
	}{h.buildAppResponse(app), warning}
	writeJSON(w, http.StatusCreated, resp)
}
