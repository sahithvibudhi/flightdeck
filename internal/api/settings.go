package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/nestops/nestops/internal/db"
	"github.com/nestops/nestops/internal/proxy"
	"github.com/nestops/nestops/internal/system"
)

type SettingsHandler struct {
	database *sql.DB
}

func NewSettingsHandler(database *sql.DB) *SettingsHandler {
	return &SettingsHandler{database: database}
}

type settingsResponse struct {
	PanelDomain   *string `json:"panel_domain"`
	AdminUsername string  `json:"admin_username"`
	HasGitToken   bool    `json:"has_git_token"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}

	resp := settingsResponse{
		AdminUsername: cfg.AdminUsername,
		HasGitToken:  cfg.GitToken.Valid && cfg.GitToken.String != "",
	}
	if cfg.PanelDomain.Valid {
		resp.PanelDomain = &cfg.PanelDomain.String
	}

	writeJSON(w, http.StatusOK, resp)
}

type updateDomainRequest struct {
	Domain string `json:"domain"`
}

func (h *SettingsHandler) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	var req updateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get config"})
		return
	}
	if cfg.PanelDomain.Valid {
		proxy.RemoveRoute("nestops-panel")
	}

	if req.Domain != "" {
		if err := proxy.AddRoute("nestops-panel", req.Domain, 3000); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to configure domain"})
			return
		}
		if err := db.UpdatePanelDomain(h.database, sql.NullString{String: req.Domain, Valid: true}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save domain"})
			return
		}
	} else {
		if err := db.UpdatePanelDomain(h.database, sql.NullString{}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save domain"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "domain updated"})
}

type updateGitTokenRequest struct {
	Token string `json:"token"`
}

func (h *SettingsHandler) UpdateGitToken(w http.ResponseWriter, r *http.Request) {
	var req updateGitTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var token sql.NullString
	if req.Token != "" {
		token = sql.NullString{String: req.Token, Valid: true}
	}

	if err := db.UpdateGitToken(h.database, token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "git token updated"})
}

func (h *SettingsHandler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, system.Detect())
}

func (h *SettingsHandler) ServerMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, system.GetHistory(h.database))
}
