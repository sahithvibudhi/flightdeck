package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/nestops/nestops/internal/db"
	"github.com/nestops/nestops/internal/proxy"
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
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}

	resp := settingsResponse{AdminUsername: cfg.AdminUsername}
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

	// Remove old panel route if exists
	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get config"})
		return
	}
	if cfg.PanelDomain.Valid {
		proxy.RemoveRoute("nestops-panel")
	}

	if req.Domain != "" {
		// Add new panel route
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
