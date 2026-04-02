package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/git"
	"github.com/sahithvibudhi/flightdeck/internal/process"
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
	RepoURL  string `json:"repo_url"`
	Branch   string `json:"branch"`
}

type appResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Port      int      `json:"port"`
	StartCmd  string   `json:"start_command"`
	Status    string   `json:"status"`
	RepoURL   *string  `json:"repo_url"`
	Branch    *string  `json:"branch"`
	Domains   []string `json:"domains"`
	CPU       float64  `json:"cpu_percent"`
	Memory    float64  `json:"memory_mb"`
	CreatedAt string   `json:"created_at"`
}

func (h *AppsHandler) buildAppResponse(a *db.App) appResponse {
	domains, _ := db.ListDomains(h.database, a.ID)
	var domainNames []string
	for _, d := range domains {
		domainNames = append(domainNames, d.Domain)
	}
	if domainNames == nil {
		domainNames = []string{}
	}

	metrics := h.pm.GetAppMetrics(a.ID)

	resp := appResponse{
		ID:        a.ID,
		Name:      a.Name,
		Port:      a.Port,
		StartCmd:  a.StartCmd,
		Status:    a.Status,
		Domains:   domainNames,
		CPU:       metrics.CPU,
		Memory:    metrics.Memory,
		CreatedAt: a.CreatedAt,
	}
	if a.RepoURL.Valid {
		resp.RepoURL = &a.RepoURL.String
	}
	if a.Branch.Valid {
		resp.Branch = &a.Branch.String
	}
	return resp
}

func (h *AppsHandler) List(w http.ResponseWriter, r *http.Request) {
	apps, err := db.ListApps(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}

	resp := make([]appResponse, 0, len(apps))
	for _, a := range apps {
		resp = append(resp, h.buildAppResponse(&a))
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

	if req.Branch == "" {
		req.Branch = "main"
	}

	port, err := db.NextPort(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to assign port"})
		return
	}

	appDir := filepath.Join(h.dataDir, "apps", req.Name)
	logPath := filepath.Join(appDir, "app.log")

	var repoURL, branch sql.NullString
	if req.RepoURL != "" {
		repoURL = sql.NullString{String: req.RepoURL, Valid: true}
		branch = sql.NullString{String: req.Branch, Valid: true}

		cfg, err := db.GetConfig(h.database)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read config"})
			return
		}

		var token string
		if cfg.GitToken.Valid {
			token = cfg.GitToken.String
		}

		if err := git.Clone(req.RepoURL, appDir, req.Branch, token); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
	}

	app, err := db.InsertApp(h.database, req.Name, req.StartCmd, port, logPath, repoURL, branch)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "app name already exists or port conflict"})
		return
	}

	writeJSON(w, http.StatusCreated, h.buildAppResponse(app))
}

func (h *AppsHandler) Get(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.buildAppResponse(app))
}

func (h *AppsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	h.pm.StopApp(id)

	domains, _ := db.ListDomains(h.database, id)
	for _, d := range domains {
		_ = removeRoute(d.ID)
	}

	if err := db.DeleteApp(h.database, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete app"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "app deleted"})
}

func (h *AppsHandler) Start(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
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
	if err := h.pm.StopApp(chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "app stopped"})
}

func (h *AppsHandler) Restart(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
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

func (h *AppsHandler) Pull(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if !app.RepoURL.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app is not linked to a git repository"})
		return
	}

	appDir := filepath.Join(h.dataDir, "apps", app.Name)
	output, err := git.Pull(appDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

func (h *AppsHandler) Logs(w http.ResponseWriter, r *http.Request) {
	app, err := db.GetApp(h.database, chi.URLParam(r, "id"))
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

var removeRoute = func(id string) error { return nil }

func SetRouteRemover(fn func(string) error) {
	removeRoute = fn
}
