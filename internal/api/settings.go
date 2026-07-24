package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/notify"
	"github.com/sahithvibudhi/flightdeck/internal/proxy"
	"github.com/sahithvibudhi/flightdeck/internal/setup"
	"github.com/sahithvibudhi/flightdeck/internal/system"
)

type SettingsHandler struct {
	database *sql.DB
}

func NewSettingsHandler(database *sql.DB) *SettingsHandler {
	return &SettingsHandler{database: database}
}

// onCaddyInstalled is invoked after Caddy is installed through the API so
// main can start the proxy and register routes without a restart.
var onCaddyInstalled = func() error { return nil }

func SetCaddyInstalledHook(fn func() error) {
	onCaddyInstalled = fn
}

type settingsResponse struct {
	PanelDomain         *string `json:"panel_domain"`
	AdminUsername       string  `json:"admin_username"`
	HasGitToken         bool    `json:"has_git_token"`
	NotifyDiscord       string  `json:"notify_discord"`
	NotifyTelegramToken string  `json:"notify_telegram_token"`
	NotifyTelegramChat  string  `json:"notify_telegram_chat"`
	NotifyWebhook       string  `json:"notify_webhook"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}

	resp := settingsResponse{
		AdminUsername:       cfg.AdminUsername,
		HasGitToken:         cfg.GitToken.Valid && cfg.GitToken.String != "",
		NotifyDiscord:       cfg.NotifyDiscord,
		NotifyTelegramToken: cfg.NotifyTelegramToken,
		NotifyTelegramChat:  cfg.NotifyTelegramChat,
		NotifyWebhook:       cfg.NotifyWebhook,
	}
	if cfg.PanelDomain.Valid {
		resp.PanelDomain = &cfg.PanelDomain.String
	}

	writeJSON(w, http.StatusOK, resp)
}

type updateNotificationsRequest struct {
	Discord       string `json:"discord"`
	TelegramToken string `json:"telegram_token"`
	TelegramChat  string `json:"telegram_chat"`
	Webhook       string `json:"webhook"`
}

func (h *SettingsHandler) UpdateNotifications(w http.ResponseWriter, r *http.Request) {
	var req updateNotificationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	for _, u := range []string{req.Discord, req.Webhook} {
		if u != "" && !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook URLs must start with http:// or https://"})
			return
		}
	}

	if err := db.UpdateNotifications(h.database,
		strings.TrimSpace(req.Discord),
		strings.TrimSpace(req.TelegramToken),
		strings.TrimSpace(req.TelegramChat),
		strings.TrimSpace(req.Webhook)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save notification settings"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "notification settings saved"})
}

func (h *SettingsHandler) TestNotifications(w http.ResponseWriter, r *http.Request) {
	if !notify.Configured(h.database) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no notification channel configured"})
		return
	}
	if err := notify.Send(h.database, "flightdeck test", "Notifications are working."); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "test notification sent"})
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

	// Empty domain means "remove the panel domain" and skips validation.
	if req.Domain != "" {
		domain, err := normalizeDomain(req.Domain)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		req.Domain = domain
	}

	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get config"})
		return
	}
	if cfg.PanelDomain.Valid {
		proxy.RemoveRoute("flightdeck-panel")
	}

	if req.Domain != "" {
		if err := proxy.AddRoute("flightdeck-panel", req.Domain, 3000); err != nil {
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

/*
Version is stamped by the release build via ldflags; "dev" for local
builds. Exposed through /api/system so the UI can show what is running.
*/
var Version = "dev"

type systemInfoResponse struct {
	system.Info
	Version string `json:"version"`
}

func (h *SettingsHandler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, systemInfoResponse{Info: system.Detect(), Version: Version})
}

func (h *SettingsHandler) ServerMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, system.GetHistory(h.database))
}

type installRuntimeRequest struct {
	Name string `json:"name"`
}

func (h *SettingsHandler) InstallRuntime(w http.ResponseWriter, r *http.Request) {
	var req installRuntimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	output, err := setup.InstallRuntime(req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if strings.EqualFold(req.Name, "caddy") {
		if err := onCaddyInstalled(); err != nil {
			writeJSON(w, http.StatusOK, map[string]string{
				"message": "caddy installed, but starting it failed: " + err.Error(),
				"output":  output,
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": req.Name + " installed", "output": output})
}
