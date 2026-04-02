package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

type EnvsHandler struct {
	database *sql.DB
}

func NewEnvsHandler(database *sql.DB) *EnvsHandler {
	return &EnvsHandler{database: database}
}

type envEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h *EnvsHandler) List(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")

	envs, err := db.ListEnvs(h.database, appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list envs"})
		return
	}

	var resp []envEntry
	for _, e := range envs {
		resp = append(resp, envEntry{Key: e.Key, Value: e.Value})
	}
	if resp == nil {
		resp = []envEntry{}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *EnvsHandler) Replace(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")

	var entries []envEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var envs []db.Env
	for _, e := range entries {
		envs = append(envs, db.Env{AppID: appID, Key: e.Key, Value: e.Value})
	}

	if err := db.ReplaceEnvs(h.database, appID, envs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update envs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "env vars updated"})
}
