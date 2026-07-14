package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/setup"
)

type SetupHandler struct {
	database *sql.DB
}

func NewSetupHandler(database *sql.DB) *SetupHandler {
	return &SetupHandler{database: database}
}

func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": setup.NeedsSetup(h.database)})
}

type completeSetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
}

/*
Complete creates the admin account on first run. It is only valid while
no config exists — once setup is done the endpoint permanently returns 409,
so it cannot be used to reset credentials.
*/
func (h *SetupHandler) Complete(w http.ResponseWriter, r *http.Request) {
	if !setup.NeedsSetup(h.database) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "setup is already complete"})
		return
	}

	var req completeSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := setup.CreateConfig(h.database, req.Username, req.Password, req.Domain); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	cfg, err := db.GetConfig(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "setup saved but failed to load config"})
		return
	}

	token, err := auth.IssueToken(cfg.AdminUsername, cfg.JWTSecret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "setup saved but failed to issue token"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}
