package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nestops/nestops/internal/db"
	"github.com/nestops/nestops/internal/process"
)

type AppsHandler struct {
	database *sql.DB
	pm       *process.Manager
	dataDir  string
}

func NewAppsHandler(database *sql.DB, pm *process.Manager, dataDir string) *AppsHandler {
	return &AppsHandler{database: database, pm: pm, dataDir: dataDir}
}

type createAppRequest struct {
	Name     string `json:"name"`
	StartCmd string `json:"start_command"`
}

type appResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Port      int      `json:"port"`
	StartCmd  string   `json:"start_command"`
	Status    string   `json:"status"`
	Domains   []string `json:"domains"`
	CreatedAt string   `json:"created_at"`
}

func (h *AppsHandler) List(w http.ResponseWriter, r *http.Request) {
	apps, err := db.ListApps(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}

	var resp []appResponse
	for _, a := range apps {
		domains, _ := db.ListDomains(h.database, a.ID)
		var domainNames []string
		for _, d := range domains {
			domainNames = append(domainNames, d.Domain)
		}
		resp = append(resp, appResponse{
			ID:        a.ID,
			Name:      a.Name,
			Port:      a.Port,
			StartCmd:  a.StartCmd,
			Status:    a.Status,
			Domains:   domainNames,
			CreatedAt: a.CreatedAt,
		})
	}

	if resp == nil {
		resp = []appResponse{}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AppsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" || req.StartCmd == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and start_command are required"})
		return
	}

	port, err := db.NextPort(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to assign port"})
		return
	}

	logPath := filepath.Join(h.dataDir, "apps", req.Name, "app.log")
	app, err := db.InsertApp(h.database, req.Name, req.StartCmd, port, logPath)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "app name already exists or port conflict"})
		return
	}

	writeJSON(w, http.StatusCreated, appResponse{
		ID:        app.ID,
		Name:      app.Name,
		Port:      app.Port,
		StartCmd:  app.StartCmd,
		Status:    app.Status,
		Domains:   []string{},
		CreatedAt: app.CreatedAt,
	})
}

func (h *AppsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	domains, _ := db.ListDomains(h.database, app.ID)
	var domainNames []string
	for _, d := range domains {
		domainNames = append(domainNames, d.Domain)
	}

	writeJSON(w, http.StatusOK, appResponse{
		ID:        app.ID,
		Name:      app.Name,
		Port:      app.Port,
		StartCmd:  app.StartCmd,
		Status:    app.Status,
		Domains:   domainNames,
		CreatedAt: app.CreatedAt,
	})
}

func (h *AppsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Stop the app if running
	h.pm.StopApp(id)

	// Remove all caddy routes for this app's domains
	domains, _ := db.ListDomains(h.database, id)
	for _, d := range domains {
		// Best effort removal from Caddy
		_ = removeRoute(d.ID)
	}

	if err := db.DeleteApp(h.database, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete app"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "app deleted"})
}

func (h *AppsHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if err := h.pm.StartApp(app); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "app started"})
}

func (h *AppsHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.pm.StopApp(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "app stopped"})
}

func (h *AppsHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if err := h.pm.RestartApp(app); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "app restarted"})
}

func (h *AppsHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	lines := 100
	if q := r.URL.Query().Get("lines"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			lines = n
		}
	}

	logLines, err := process.TailLog(app.LogPath, lines)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read logs"})
		return
	}

	if logLines == nil {
		logLines = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"lines": logLines})
}

// removeRoute is a package-level helper that calls proxy.RemoveRoute
// It's set via SetRouteRemover to avoid import cycles.
var removeRoute = func(id string) error { return nil }

func SetRouteRemover(fn func(string) error) {
	removeRoute = fn
}
