package api

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/git"
	"github.com/sahithvibudhi/flightdeck/internal/process"
)

var appNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type AppsHandler struct {
	database *sql.DB
	pm       *process.Manager
	dataDir  string
}

func NewAppsHandler(database *sql.DB, pm *process.Manager, dataDir string) *AppsHandler {
	return &AppsHandler{database: database, pm: pm, dataDir: dataDir}
}

type createAppRequest struct {
	Name       string `json:"name"`
	StartCmd   string `json:"start_command"`
	BuildCmd   string `json:"build_command"`
	Port       int    `json:"port"`
	RepoURL    string `json:"repo_url"`
	Branch     string `json:"branch"`
	WorkDir    string `json:"work_dir"`
	HealthPath string `json:"health_path"`
}

type updateAppRequest struct {
	Name       string  `json:"name"`
	StartCmd   string  `json:"start_command"`
	BuildCmd   string  `json:"build_command"`
	Port       int     `json:"port"`
	RepoURL    string  `json:"repo_url"`
	Branch     string  `json:"branch"`
	WorkDir    string  `json:"work_dir"`
	HealthPath *string `json:"health_path"`
}

type appResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Port          int      `json:"port"`
	StartCmd      string   `json:"start_command"`
	BuildCmd      string   `json:"build_command"`
	WorkDir       string   `json:"work_dir"`
	WebhookSecret string   `json:"webhook_secret"`
	HealthPath    string   `json:"health_path"`
	Status        string   `json:"status"`
	RepoURL       *string  `json:"repo_url"`
	Branch        *string  `json:"branch"`
	Domains       []string `json:"domains"`
	CPU           float64  `json:"cpu_percent"`
	Memory        float64  `json:"memory_mb"`
	CreatedAt     string   `json:"created_at"`
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
		ID:            a.ID,
		Name:          a.Name,
		Port:          a.Port,
		StartCmd:      a.StartCmd,
		BuildCmd:      a.BuildCmd,
		WorkDir:       a.WorkDir,
		WebhookSecret: a.WebhookSecret,
		HealthPath:    a.HealthPath,
		Status:        a.Status,
		Domains:       domainNames,
		CPU:           metrics.CPU,
		Memory:        metrics.Memory,
		CreatedAt:     a.CreatedAt,
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

	if !appNameRe.MatchString(req.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must be lowercase letters, digits, and hyphens (max 63 chars)"})
		return
	}

	if req.WorkDir != "" && req.RepoURL != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "work_dir and repo_url are mutually exclusive"})
		return
	}

	if req.Branch == "" {
		req.Branch = "main"
	}

	var port int
	if req.Port > 0 {
		port = req.Port
	} else {
		var err error
		port, err = db.NextPort(h.database)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to assign port"})
			return
		}
	}

	appDir := filepath.Join(h.dataDir, "apps", req.Name)
	logPath := filepath.Join(appDir, "app.log")

	if req.WorkDir != "" {
		if !filepath.IsAbs(req.WorkDir) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "work_dir must be an absolute path"})
			return
		}
		info, err := os.Stat(req.WorkDir)
		if err != nil || !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "work_dir does not exist or is not a directory"})
			return
		}
		// Apps running from an existing server path keep their logs under
		// the data dir so we never write into the user's directory tree.
		logPath = filepath.Join(h.dataDir, "logs", req.Name+".log")
		if err := os.MkdirAll(filepath.Join(h.dataDir, "logs"), 0755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create log directory"})
			return
		}
	}

	var repoURL, branch sql.NullString
	if req.RepoURL != "" {
		repoURL = sql.NullString{String: req.RepoURL, Valid: true}
		branch = sql.NullString{String: req.Branch, Valid: true}

		if err := git.Clone(req.RepoURL, appDir, req.Branch, h.gitToken()); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
	}

	app, err := db.InsertApp(h.database, req.Name, req.StartCmd, req.BuildCmd, req.WorkDir, port, logPath, repoURL, branch)
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

	if req.HealthPath != "" {
		if err := db.SetHealthPath(h.database, app.ID, req.HealthPath); err == nil {
			app.HealthPath = req.HealthPath
		}
	}

	writeJSON(w, http.StatusCreated, h.buildAppResponse(app))
}

func (h *AppsHandler) gitToken() string {
	cfg, err := db.GetConfig(h.database)
	if err != nil || !cfg.GitToken.Valid {
		return ""
	}
	return cfg.GitToken.String
}

func (h *AppsHandler) appDir(app *db.App) string {
	if app.WorkDir != "" {
		return app.WorkDir
	}
	return filepath.Join(h.dataDir, "apps", app.Name)
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
	app, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	h.pm.StopApp(id)

	domains, _ := db.ListDomains(h.database, id)
	for _, d := range domains {
		_ = removeRoute(d.ID)
	}

	if err := db.DeleteApp(h.database, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete app"})
		return
	}

	// Clean up the managed app directory and log. A work_dir belongs to
	// the user and is never touched.
	if app.WorkDir == "" {
		os.RemoveAll(filepath.Join(h.dataDir, "apps", app.Name))
	} else {
		os.Remove(app.LogPath)
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

	output, err := git.Pull(h.appDir(app), h.gitToken())
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

func (h *AppsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	var req updateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.StartCmd == "" {
		req.StartCmd = existing.StartCmd
	}
	if req.Port <= 0 {
		req.Port = existing.Port
	}
	if req.WorkDir == "" {
		req.WorkDir = existing.WorkDir
	}

	if !appNameRe.MatchString(req.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must be lowercase letters, digits, and hyphens (max 63 chars)"})
		return
	}

	renamed := req.Name != existing.Name
	if renamed && h.pm.IsRunning(id) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "stop the app before renaming it"})
		return
	}

	logPath := existing.LogPath
	if renamed && existing.WorkDir == "" {
		oldDir := filepath.Join(h.dataDir, "apps", existing.Name)
		newDir := filepath.Join(h.dataDir, "apps", req.Name)
		if _, err := os.Stat(oldDir); err == nil {
			if err := os.Rename(oldDir, newDir); err != nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "failed to move app directory: " + err.Error()})
				return
			}
		}
		logPath = filepath.Join(newDir, "app.log")
	}

	var repoURL, branch sql.NullString
	if req.RepoURL != "" {
		repoURL = sql.NullString{String: req.RepoURL, Valid: true}
		b := req.Branch
		if b == "" {
			b = "main"
		}
		branch = sql.NullString{String: b, Valid: true}
	}

	if err := db.UpdateApp(h.database, id, req.Name, req.StartCmd, req.BuildCmd, req.WorkDir, req.Port, logPath, repoURL, branch); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "update failed: " + err.Error()})
		return
	}

	if req.HealthPath != nil {
		if err := db.SetHealthPath(h.database, id, *req.HealthPath); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save health path"})
			return
		}
	}

	app, _ := db.GetApp(h.database, id)
	if app == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reload app"})
		return
	}

	// Config edits must take effect immediately: domains re-point at the
	// new port, and a running process is restarted with the new command,
	// port, and directory. Previously edits were silently inert until a
	// manual stop/start, and port changes broke existing domains.
	if app.Port != existing.Port {
		domains, _ := db.ListDomains(h.database, id)
		for _, d := range domains {
			_ = removeRoute(d.ID)
			if err := addRoute(d.ID, d.Domain, app.Port); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "app updated but failed to re-route domain " + d.Domain})
				return
			}
		}
	}

	configChanged := app.Port != existing.Port || app.StartCmd != existing.StartCmd || app.WorkDir != existing.WorkDir
	if configChanged && h.pm.IsRunning(id) {
		if err := h.pm.RestartApp(app); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "app updated but restart failed: " + err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, h.buildAppResponse(app))
}

func (h *AppsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := db.GetApp(h.database, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp("", "flightdeck-upload-*.zip")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create temp file"})
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save upload"})
		return
	}

	appDir := h.appDir(app)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create app directory"})
		return
	}

	if err := extractZip(tmpFile.Name(), appDir); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "failed to extract zip: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "uploaded and extracted"})
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dest, f.Name)
		// Zip-slip protection
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

var removeRoute = func(id string) error { return nil }

func SetRouteRemover(fn func(string) error) {
	removeRoute = fn
}

var addRoute = func(id, domain string, port int) error { return nil }

func SetRouteAdder(fn func(string, string, int) error) {
	addRoute = fn
}
