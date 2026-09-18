package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
)

// hashPassword generates a bcrypt hash for the given password.
func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate bcrypt hash: %v", err)
	}
	return string(h)
}

// newTestDB creates a temp SQLite DB and optionally sets dashboard_password_hash.
func newTestDB(t *testing.T, passwordHash string) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "jkrouter.db")
	d, err := db.Open(tmpDir, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if passwordHash != "" {
		d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('dashboard_password_hash', ?)", passwordHash)
		d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('dashboard_first_run', 'false')")
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// buildDashboardRouter returns a chi router with DashboardRouter mounted at /api/dashboard,
// plus auth endpoints (login/logout) that live outside DashboardRouter in main.go.
func buildDashboardRouter(t *testing.T, d *db.DB) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/api/dashboard/auth/login", LoginHandler(d))
	r.Post("/api/dashboard/auth/logout", LogoutHandler())
	r.Mount("/api/dashboard", DashboardRouter(d, nil, nil))
	return httptest.NewServer(r)
}

func TestLoginWrongPassword(t *testing.T) {
	hash := hashPassword(t, "correctpass")
	d := newTestDB(t, hash)
	ts := buildDashboardRouter(t, d)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"password": "wrongpass"})
	req, _ := http.NewRequest("POST", ts.URL+"/api/dashboard/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginCorrectPassword(t *testing.T) {
	hash := hashPassword(t, "correctpass")
	d := newTestDB(t, hash)
	ts := buildDashboardRouter(t, d)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"password": "correctpass"})
	req, _ := http.NewRequest("POST", ts.URL+"/api/dashboard/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cookie := resp.Cookies()
	if len(cookie) == 0 {
		t.Fatal("expected Set-Cookie header")
	}
	found := false
	for _, c := range cookie {
		if c.Name == sessionCookieName && c.Value == hash {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected session cookie with hash value, got: %v", cookie)
	}
}

func TestRequireAuthBlockedWithoutCookie(t *testing.T) {
	hash := hashPassword(t, "correctpass")
	d := newTestDB(t, hash)
	ts := buildDashboardRouter(t, d)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/dashboard/providers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireAuthAllowedWithValidCookie(t *testing.T) {
	hash := hashPassword(t, "correctpass")
	d := newTestDB(t, hash)
	ts := buildDashboardRouter(t, d)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/dashboard/providers", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: hash})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRequireAuthAllowedFirstRunNoPassword(t *testing.T) {
	// No password hash set → first run, all requests allowed.
	d := newTestDB(t, "")
	ts := buildDashboardRouter(t, d)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/dashboard/providers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (first run), got %d", resp.StatusCode)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	hash := hashPassword(t, "correctpass")
	d := newTestDB(t, hash)
	ts := buildDashboardRouter(t, d)
	defer ts.Close()

	// Login first to get cookie.
	loginBody, _ := json.Marshal(map[string]string{"password": "correctpass"})
	loginReq, _ := http.NewRequest("POST", ts.URL+"/api/dashboard/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, _ := http.DefaultClient.Do(loginReq)
	loginResp.Body.Close()

	// Now logout.
	logoutReq, _ := http.NewRequest("POST", ts.URL+"/api/dashboard/auth/logout", nil)
	logoutResp, err := http.DefaultClient.Do(logoutReq)
	if err != nil {
		t.Fatalf("logout request: %v", err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", logoutResp.StatusCode)
	}

	// Verify cookie is cleared — subsequent request should be 401.
	req, _ := http.NewRequest("GET", ts.URL+"/api/dashboard/providers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}
