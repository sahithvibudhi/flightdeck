package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/sahithvibudhi/flightdeck/internal/auth"
	"github.com/sahithvibudhi/flightdeck/internal/db"
)

type contextKey string

const userKey contextKey = "user"

/*
DBAuthMiddleware reads the JWT secret from the database on each request
instead of capturing it at startup. This lets the server boot before
first-run setup has created a config (all requests are rejected until then),
and picks up the secret the moment setup completes without a restart.
*/
func DBAuthMiddleware(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg, err := db.GetConfig(database)
			if err != nil {
				http.Error(w, `{"error":"setup required"}`, http.StatusUnauthorized)
				return
			}
			AuthMiddleware(cfg.JWTSecret)(next).ServeHTTP(w, r)
		})
	}
}

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string
			header := r.Header.Get("Authorization")
			switch {
			case header != "":
				token = strings.TrimPrefix(header, "Bearer ")
				if token == header {
					http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
					return
				}
			// EventSource can't set headers, so SSE endpoints pass the token
			// as a query parameter instead.
			case r.URL.Query().Get("token") != "":
				token = r.URL.Query().Get("token")
			default:
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			username, err := auth.VerifyToken(token, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
