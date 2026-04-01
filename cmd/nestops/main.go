package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	nestops "github.com/nestops/nestops"
	"github.com/nestops/nestops/internal/api"
	"github.com/nestops/nestops/internal/db"
	"github.com/nestops/nestops/internal/process"
	"github.com/nestops/nestops/internal/proxy"
	"github.com/nestops/nestops/internal/setup"
	"github.com/nestops/nestops/internal/system"
)

const defaultDataDir = "/var/nestops"

func main() {
	dataDir := os.Getenv("NESTOPS_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "apps"), 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "nestops.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if setup.NeedsSetup(database) {
		if err := setup.RunWizard(database); err != nil {
			log.Fatalf("setup failed: %v", err)
		}
	}

	cfg, err := db.GetConfig(database)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	caddy := proxy.NewCaddy(dataDir)
	if err := caddy.Start(); err != nil {
		log.Printf("warning: failed to start caddy: %v", err)
		log.Println("continuing without reverse proxy (domains won't work)")
	} else {
		defer caddy.Stop()

		if cfg.PanelDomain.Valid {
			if err := proxy.AddRoute("nestops-panel", cfg.PanelDomain.String, 3000); err != nil {
				log.Printf("warning: failed to register panel domain: %v", err)
			}
		}

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

	if err := system.InitMetricsTable(database); err != nil {
		log.Fatalf("failed to init metrics table: %v", err)
	}
	system.StartCollector(database)

	pm := process.NewManager(database, dataDir)
	if err := pm.RestoreRunning(); err != nil {
		log.Printf("warning: failed to restore apps: %v", err)
	}

	uiDist, err := fs.Sub(nestops.UIFiles, "ui/dist")
	if err != nil {
		log.Fatalf("failed to load embedded UI: %v", err)
	}

	router := api.NewRouter(database, pm, dataDir, cfg.JWTSecret)
	router.NotFound(api.StaticHandler(uiDist).ServeHTTP)

	addr := ":3000"
	server := &http.Server{Addr: addr, Handler: router}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("received %s, shutting down...", sig)
		server.Shutdown(context.Background())
	}()

	fmt.Printf("flightdeck is running on %s\n", addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
