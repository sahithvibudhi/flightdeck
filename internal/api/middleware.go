package api

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
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

			// Scoped API tokens carry the fd_ prefix; everything else is a JWT.
			if token := requestToken(r); strings.HasPrefix(token, "fd_") {
				t, err := db.GetAPITokenByHash(database, HashAPIToken(token))
				if err != nil {
					http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
					return
				}
				if !tokenAllowed(t.Scope, r.Method, r.URL.Path) {
					http.Error(w, `{"error":"token scope does not allow this"}`, http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			AuthMiddleware(cfg.JWTSecret)(next).ServeHTTP(w, r)
		})
	}
}

// requestToken extracts the bearer token without judging its format;
// AuthMiddleware handles error responses for the JWT path.
func requestToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); header != "" {
		if token := strings.TrimPrefix(header, "Bearer "); token != header {
			return token
		}
		return ""
	}
	return r.URL.Query().Get("token")
}

var deployActionPath = regexp.MustCompile(`^/api/apps/[^/]+/(start|stop|restart|pull|deploy)$`)

func tokenAllowed(scope, method, path string) bool {
	// Token management always requires the admin JWT.
	if strings.HasPrefix(path, "/api/tokens") {
		return false
	}
	switch scope {
	case "read":
		return method == http.MethodGet
	case "deploy":
		if method == http.MethodGet {
			return true
		}
		return method == http.MethodPost && deployActionPath.MatchString(path)
	}
	return false
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
