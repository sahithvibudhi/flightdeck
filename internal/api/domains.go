package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/proxy"
)

type DomainsHandler struct {
	database *sql.DB
}

func NewDomainsHandler(database *sql.DB) *DomainsHandler {
	return &DomainsHandler{database: database}
}

type domainEntry struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

func (h *DomainsHandler) List(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")

	domains, err := db.ListDomains(h.database, appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list domains"})
		return
	}

	var resp []domainEntry
	for _, d := range domains {
		resp = append(resp, domainEntry{ID: d.ID, Domain: d.Domain})
	}
	if resp == nil {
		resp = []domainEntry{}
	}

	writeJSON(w, http.StatusOK, resp)
}

type addDomainRequest struct {
	Domain string `json:"domain"`
}

func (h *DomainsHandler) Add(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")

	var req addDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Domain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}

	app, err := db.GetApp(h.database, appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	d, err := db.InsertDomain(h.database, appID, req.Domain)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "domain already registered"})
		return
	}

	// Register with Caddy
	if err := proxy.AddRoute(d.ID, req.Domain, app.Port); err != nil {
		// Rollback domain from db
		db.DeleteDomain(h.database, d.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to register domain with proxy"})
		return
	}

	writeJSON(w, http.StatusCreated, domainEntry{ID: d.ID, Domain: d.Domain})
}

func (h *DomainsHandler) Remove(w http.ResponseWriter, r *http.Request) {
	domainName := chi.URLParam(r, "domain")

	d, err := db.GetDomainByName(h.database, domainName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}

	// Remove from Caddy
	proxy.RemoveRoute(d.ID)

	if err := db.DeleteDomain(h.database, d.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete domain"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "domain removed"})
}
