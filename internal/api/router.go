package api

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/sahithvibudhi/flightdeck/internal/process"
	"github.com/sahithvibudhi/flightdeck/internal/proxy"
)

func NewRouter(database *sql.DB, pm *process.Manager, dataDir string) *chi.Mux {
	r := chi.NewRouter()

	SetRouteRemover(proxy.RemoveRoute)
	SetRouteAdder(proxy.AddRoute)

	authHandler := NewAuthHandler(database)
	appsHandler := NewAppsHandler(database, pm, dataDir)
	envsHandler := NewEnvsHandler(database)
	domainsHandler := NewDomainsHandler(database)
	settingsHandler := NewSettingsHandler(database)
	setupHandler := NewSetupHandler(database)
	tokensHandler := NewTokensHandler(database)

	// Push-to-deploy webhooks authenticate with per-app HMAC secrets
	// instead of JWTs, so they live outside /api.
	r.Post("/hooks/{id}", appsHandler.Webhook)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/login", authHandler.Login)
		r.Get("/setup/status", setupHandler.Status)
		r.Post("/setup", setupHandler.Complete)

		r.Group(func(r chi.Router) {
			r.Use(DBAuthMiddleware(database))

			r.Post("/auth/password", authHandler.ChangePassword)

			r.Get("/apps", appsHandler.List)
			r.Post("/apps", appsHandler.Create)
			r.Post("/apps/sample", appsHandler.CreateSample)
			r.Get("/apps/{id}", appsHandler.Get)
			r.Put("/apps/{id}", appsHandler.Update)
			r.Delete("/apps/{id}", appsHandler.Delete)
			r.Post("/apps/{id}/upload", appsHandler.Upload)
			r.Post("/apps/{id}/start", appsHandler.Start)
			r.Post("/apps/{id}/stop", appsHandler.Stop)
			r.Post("/apps/{id}/restart", appsHandler.Restart)
			r.Post("/apps/{id}/pull", appsHandler.Pull)
			r.Post("/apps/{id}/deploy", appsHandler.DeployNow)
			r.Get("/apps/{id}/deployments", appsHandler.ListDeployments)
			r.Post("/apps/{id}/deployments/{depID}/rollback", appsHandler.Rollback)
			r.Get("/apps/{id}/logs", appsHandler.Logs)
			r.Get("/apps/{id}/logs/stream", appsHandler.LogsStream)

			r.Get("/apps/{id}/envs", envsHandler.List)
			r.Put("/apps/{id}/envs", envsHandler.Replace)

			r.Get("/apps/{id}/domains", domainsHandler.List)
			r.Post("/apps/{id}/domains", domainsHandler.Add)
			r.Delete("/apps/{id}/domains/{domain}", domainsHandler.Remove)

			r.Get("/tokens", tokensHandler.List)
			r.Post("/tokens", tokensHandler.Create)
			r.Delete("/tokens/{id}", tokensHandler.Delete)

			r.Get("/settings", settingsHandler.Get)
			r.Put("/settings/domain", settingsHandler.UpdateDomain)
			r.Put("/settings/git-token", settingsHandler.UpdateGitToken)
			r.Put("/settings/notifications", settingsHandler.UpdateNotifications)
			r.Post("/settings/notifications/test", settingsHandler.TestNotifications)
			r.Get("/system", settingsHandler.SystemInfo)
			r.Get("/system/metrics", settingsHandler.ServerMetrics)
			r.Post("/system/install", settingsHandler.InstallRuntime)
		})
	})

	return r
}
