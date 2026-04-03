package api

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/process"
	"github.com/sahithvibudhi/flightdeck/internal/proxy"
)

func NewRouter(database *sql.DB, pm *process.Manager, dataDir string, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()

	SetRouteRemover(proxy.RemoveRoute)

	authHandler := NewAuthHandler(database)
	appsHandler := NewAppsHandler(database, pm, dataDir)
	envsHandler := NewEnvsHandler(database)
	domainsHandler := NewDomainsHandler(database)
	settingsHandler := NewSettingsHandler(database)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(jwtSecret))

			r.Post("/auth/password", authHandler.ChangePassword)

			r.Get("/apps", appsHandler.List)
			r.Post("/apps", appsHandler.Create)
			r.Get("/apps/{id}", appsHandler.Get)
			r.Put("/apps/{id}", appsHandler.Update)
			r.Delete("/apps/{id}", appsHandler.Delete)
			r.Post("/apps/{id}/upload", appsHandler.Upload)
			r.Post("/apps/{id}/start", appsHandler.Start)
			r.Post("/apps/{id}/stop", appsHandler.Stop)
			r.Post("/apps/{id}/restart", appsHandler.Restart)
			r.Post("/apps/{id}/pull", appsHandler.Pull)
			r.Get("/apps/{id}/logs", appsHandler.Logs)

			r.Get("/apps/{id}/envs", envsHandler.List)
			r.Put("/apps/{id}/envs", envsHandler.Replace)

			r.Get("/apps/{id}/domains", domainsHandler.List)
			r.Post("/apps/{id}/domains", domainsHandler.Add)
			r.Delete("/apps/{id}/domains/{domain}", domainsHandler.Remove)

			r.Get("/settings", settingsHandler.Get)
			r.Put("/settings/domain", settingsHandler.UpdateDomain)
			r.Put("/settings/git-token", settingsHandler.UpdateGitToken)
			r.Get("/system", settingsHandler.SystemInfo)
			r.Get("/system/metrics", settingsHandler.ServerMetrics)
			r.Post("/system/install", settingsHandler.InstallRuntime)
		})
	})

	return r
}
