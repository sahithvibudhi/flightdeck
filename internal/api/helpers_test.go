package api

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	dbpkg "github.com/nestops/nestops/internal/db"
)

func seedTestConfig(t *testing.T, database *sql.DB, passwordHash, jwtSecret string) {
	t.Helper()
	err := dbpkg.InsertConfig(database, &dbpkg.Config{
		AdminUsername: "admin",
		AdminPassword: passwordHash,
		JWTSecret:     jwtSecret,
	})
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}
}

func seedTestApp(t *testing.T, database *sql.DB, name string, port int) *dbpkg.App {
	t.Helper()
	app, err := dbpkg.InsertApp(database, name, "echo hello", port, "/tmp/"+name+".log", sql.NullString{}, sql.NullString{})
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}
	return app
}

func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
