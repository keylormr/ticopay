package api

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey string

const (
	userIDKey ctxKey = "userID"
	roleKey   ctxKey = "role"
)

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := a.jwt.Parse(token, "access")
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		// In one round-trip: reject tokens whose generation no longer matches
		// the user's (a password reset bumps token_version, revoking prior
		// sessions) and block deactivated accounts.
		var ver int
		var disabled bool
		if err := a.pool.QueryRow(r.Context(),
			`SELECT token_version, disabled FROM users WHERE id = $1`, claims.UserID,
		).Scan(&ver, &disabled); err != nil || ver != claims.Ver {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		if disabled {
			writeError(w, http.StatusForbidden, "cuenta desactivada")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requirePerm gates a route on a back-office permission. The role is read from
// the database on every request (never trusted from the client or the JWT) and
// checked against the permission matrix. Must run after requireAuth.
func (a *App) requirePerm(p Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var role string
			if err := a.pool.QueryRow(r.Context(),
				`SELECT role FROM users WHERE id = $1`, userID(r)).Scan(&role); err != nil || !roleHas(role, p) {
				writeError(w, http.StatusForbidden, "no autorizado")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), roleKey, role)))
		})
	}
}

func userID(r *http.Request) string {
	v, _ := r.Context().Value(userIDKey).(string)
	return v
}

func currentRole(r *http.Request) string {
	v, _ := r.Context().Value(roleKey).(string)
	return v
}
