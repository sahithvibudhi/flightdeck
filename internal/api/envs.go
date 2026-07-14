package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

// Keys must be valid shell identifiers: anything else (spaces, '=',
// leading digits) would corrupt the .env file written at app start.
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateEnv(e envEntry) error {
	if !envKeyRe.MatchString(e.Key) {
		return fmt.Errorf("invalid variable name %q: use letters, digits, and underscores, and don't start with a digit", e.Key)
	}
	if strings.ContainsAny(e.Value, "\n\r") {
		return fmt.Errorf("value of %q must not contain newlines", e.Key)
	}
	return nil
}

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
		if err := validateEnv(e); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		envs = append(envs, db.Env{AppID: appID, Key: e.Key, Value: e.Value})
	}

	if err := db.ReplaceEnvs(h.database, appID, envs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update envs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "env vars updated"})
}
