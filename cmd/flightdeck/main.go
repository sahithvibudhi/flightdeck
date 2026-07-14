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

	flightdeck "github.com/sahithvibudhi/flightdeck"
	"github.com/sahithvibudhi/flightdeck/internal/api"
	"github.com/sahithvibudhi/flightdeck/internal/db"
	"github.com/sahithvibudhi/flightdeck/internal/process"
	"github.com/sahithvibudhi/flightdeck/internal/proxy"
	"github.com/sahithvibudhi/flightdeck/internal/setup"
	"github.com/sahithvibudhi/flightdeck/internal/system"
	"golang.org/x/term"
)

const defaultDataDir = "/var/flightdeck"

func main() {
	dataDir := os.Getenv("FLIGHTDECK_DATA_DIR")
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "apps"), 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "flightdeck.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if setup.NeedsSetup(database) {
		seeded, err := setup.SeedFromEnv(database)
		if err != nil {
			log.Fatalf("setup failed: %v", err)
		}
		switch {
		case seeded:
			log.Println("admin account created from environment")
		case term.IsTerminal(int(os.Stdin.Fd())):
			if err := setup.RunWizard(database); err != nil {
				log.Fatalf("setup failed: %v", err)
			}
		default:
			// No TTY (e.g. running under systemd): start anyway and let the
			// user finish setup in the browser at /setup.
			log.Println("first-run setup pending — open http://<your-server-ip>:3000 to finish setup in the browser")
		}
	}

	registerRoutes := func() {
		if cfg, err := db.GetConfig(database); err == nil && cfg.PanelDomain.Valid {
			if err := proxy.AddRoute("flightdeck-panel", cfg.PanelDomain.String, 3000); err != nil {
				log.Printf("warning: failed to register panel domain: %v", err)
			}
		}

		domains, err := db.ListAllDomains(database)
		if err != nil {
			log.Printf("warning: failed to list domains: %v", err)
			return
		}
		for _, d := range domains {
			app, err := db.GetApp(database, d.AppID)
			if err != nil {
				continue
			}
			if err := proxy.AddRoute(d.ID, d.Domain, app.EffectivePort()); err != nil {
				log.Printf("warning: failed to register domain %s: %v", d.Domain, err)
			}
		}
	}

	caddy := proxy.NewCaddy(dataDir)
	if err := caddy.Start(); err != nil {
		log.Printf("warning: failed to start caddy: %v", err)
		log.Println("continuing without reverse proxy (domains won't work) — install Caddy from Settings")
	} else {
		defer caddy.Stop()
		registerRoutes()
	}

	// Installing Caddy from the Settings page starts the proxy and
	// registers all routes immediately, no restart needed.
	api.SetCaddyInstalledHook(func() error {
		if err := caddy.Start(); err != nil {
			return err
		}
		registerRoutes()
		return nil
	})

	if err := system.InitMetricsTable(database); err != nil {
		log.Fatalf("failed to init metrics table: %v", err)
	}
	system.StartCollector(database)

	pm := process.NewManager(database, dataDir)
	pm.SetRouteSwitcher(func(appID string, port int) error {
		domains, err := db.ListDomains(database, appID)
		if err != nil {
			return err
		}
		for _, d := range domains {
			proxy.RemoveRoute(d.ID)
			if err := proxy.AddRoute(d.ID, d.Domain, port); err != nil {
				return err
			}
		}
		return nil
	})
	if err := pm.RestoreRunning(); err != nil {
		log.Printf("warning: failed to restore apps: %v", err)
	}

	uiDist, err := fs.Sub(flightdeck.UIFiles, "ui/dist")
	if err != nil {
		log.Fatalf("failed to load embedded UI: %v", err)
	}

	router := api.NewRouter(database, pm, dataDir)
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
