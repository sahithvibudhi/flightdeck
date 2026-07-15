package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

type TokensHandler struct {
	database *sql.DB
}

func NewTokensHandler(database *sql.DB) *TokensHandler {
	return &TokensHandler{database: database}
}

// HashAPIToken is what gets stored and looked up; the plaintext never
// touches the database.
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type tokenEntry struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Scope     string  `json:"scope"`
	CreatedAt string  `json:"created_at"`
	LastUsed  *string `json:"last_used"`
}

func (h *TokensHandler) List(w http.ResponseWriter, r *http.Request) {
	tokens, err := db.ListAPITokens(h.database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tokens"})
		return
	}

	resp := []tokenEntry{}
	for _, t := range tokens {
		e := tokenEntry{ID: t.ID, Name: t.Name, Scope: t.Scope, CreatedAt: t.CreatedAt}
		if t.LastUsed.Valid {
			e.LastUsed = &t.LastUsed.String
		}
		resp = append(resp, e)
	}

	writeJSON(w, http.StatusOK, resp)
}

type createTokenRequest struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

type createTokenResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Token string `json:"token"`
}

func (h *TokensHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if len(req.Name) > 60 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must be 60 characters or less"})
		return
	}
	if req.Scope != "read" && req.Scope != "deploy" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be read or deploy"})
		return
	}

	plaintext, err := auth.GenerateAPIToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	t, err := db.InsertAPIToken(h.database, req.Name, HashAPIToken(plaintext), req.Scope)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}

	writeJSON(w, http.StatusCreated, createTokenResponse{ID: t.ID, Name: t.Name, Scope: t.Scope, Token: plaintext})
}

func (h *TokensHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := db.DeleteAPIToken(h.database, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "token deleted"})
}
