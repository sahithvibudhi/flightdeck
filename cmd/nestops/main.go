package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nestops/nestops/internal/api"
	"github.com/nestops/nestops/internal/db"
	"github.com/nestops/nestops/internal/process"
	"github.com/nestops/nestops/internal/proxy"
	"github.com/nestops/nestops/internal/setup"
)

const defaultDataDir = "/var/nestops"

func main() {
	dataDir := os.Getenv("NESTOPS_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Join(dataDir, "apps"), 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	// Open database
	dbPath := filepath.Join(dataDir, "nestops.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// First boot check
	if setup.NeedsSetup(database) {
		if err := setup.RunWizard(database); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
	}

	// Load config
	cfg, err := db.GetConfig(database)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Start Caddy
	caddy := proxy.NewCaddy(dataDir)
	if err := caddy.Start(); err != nil {
		log.Printf("warning: failed to start caddy: %v", err)
		log.Println("continuing without reverse proxy (domains won't work)")
	} else {
		defer caddy.Stop()

		// Re-register panel domain
		if cfg.PanelDomain.Valid {
			if err := proxy.AddRoute("nestops-panel", cfg.PanelDomain.String, 3000); err != nil {
				log.Printf("warning: failed to register panel domain: %v", err)
			}
		}

		// Re-register all app domains
		domains, err := db.ListAllDomains(database)
		if err != nil {
			log.Printf("warning: failed to list domains: %v", err)
		} else {
			for _, d := range domains {
				app, err := db.GetApp(database, d.AppID)
				if err != nil {
					continue
				}
				if err := proxy.AddRoute(d.ID, d.Domain, app.Port); err != nil {
					log.Printf("warning: failed to register domain %s: %v", d.Domain, err)
				}
			}
		}
	}

	// Start process manager and restore running apps
	pm := process.NewManager(database, dataDir)
	if err := pm.RestoreRunning(); err != nil {
		log.Printf("warning: failed to restore apps: %v", err)
	}

	// Start HTTP server
	router := api.NewRouter(database, pm, dataDir, cfg.JWTSecret)

	addr := ":3000"
	fmt.Printf("nestops is running on %s\n", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
