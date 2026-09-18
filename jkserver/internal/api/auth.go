// Package api: dashboard auth middleware.
//
// Dashboard endpoints are protected by a bcrypt password stored in settings_kv.
// The middleware checks for a session cookie; if missing it redirects to /login.
// First-run (no password hash) is allowed — the Profile page handles setup.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"jkrouter/jkserver/internal/db"
)

const sessionCookieName = "jkr_session"

// RequireAuth returns a chi middleware that gates /api/dashboard/* on a valid
// session cookie. Skipped when dashboard_password_hash is not set yet (first run).
func RequireAuth(d *db.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for status + login-related endpoints.
			path := r.URL.Path
			if strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/api/dashboard/auth/") || path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if password is configured.
			var hash string
			err := d.QueryRow("SELECT value FROM settings_kv WHERE key='dashboard_password_hash'").Scan(&hash)
			if err != nil {
				// No password set yet — first run, allow access.
				next.ServeHTTP(w, r)
				return
			}

			// Validate session cookie.
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			if cookie.Value != hash {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoginHandler handles POST /api/dashboard/auth/login.
func LoginHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		var hash string
		err := d.QueryRow("SELECT value FROM settings_kv WHERE key='dashboard_password_hash'").Scan(&hash)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "password not configured"})
			return
		}
		if !bcryptCompare(body.Password, hash) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "incorrect password"})
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    hash,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

// LogoutHandler clears the session cookie.
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:   sessionCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}
}

func bcryptCompare(pw, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	return err == nil
}
