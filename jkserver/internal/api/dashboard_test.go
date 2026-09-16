package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
)

func TestDashboardEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jkr-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "jkrouter.db")
	d, err := db.Open(tmpDir, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	// Set a password hash so RequireAuth middleware allows requests.
	var hash string
	d.QueryRow("SELECT value FROM settings_kv WHERE key='dashboard_password_hash'").Scan(&hash)
	if hash == "" {
		d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('dashboard_password_hash', 'test')")
	}

	r := chi.NewRouter()
	r.Mount("/api/dashboard", DashboardRouter(d, nil))
	ts := httptest.NewServer(r)
	defer ts.Close()

	client := &http.Client{}
	cookie := &http.Cookie{Name: sessionCookieName, Value: "test"}

	makeReq := func(method, path string, body io.Reader) *http.Response {
		var r io.Reader
		if body != nil {
			r = body
		}
		req, err := http.NewRequest(method, ts.URL+path, r)
		if err != nil {
			t.Fatalf("NewRequest %s %s: %v", method, path, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Do %s %s: %v", method, path, err)
		}
		return resp
	}

	// GET endpoints
	getEndpoints := []string{
		"/api/dashboard/providers",
		"/api/dashboard/connections",
		"/api/dashboard/combos",
		"/api/dashboard/proxy-pools",
		"/api/dashboard/api-keys",
	}
	for _, path := range getEndpoints {
		resp := makeReq("GET", path, nil)
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET %s: status=%d body=%s", path, resp.StatusCode, string(body))
		}
		resp.Body.Close()
		t.Logf("OK GET %s (200)", path)
	}

	// POST combos
	comboBody := `{"name":"gpt-combo","model_ids":["gpt-4o","claude-sonnet"],"strategy":"fallback"}`
	resp := makeReq("POST", "/api/dashboard/combos", bytes.NewBufferString(comboBody))
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST combos: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/combos")

	// POST providers
	provBody := `{"name":"openai","label":"Main","auth_type":"api_key"}`
	resp = makeReq("POST", "/api/dashboard/providers", bytes.NewBufferString(provBody))
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST providers: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/providers")

	// POST connections (with proxy_pool_id to test fix)
	connBody := `{"provider_id":"openai","name":"key1","secret":"sk-test123","priority":0,"proxy_pool_id":null}`
	resp = makeReq("POST", "/api/dashboard/connections", bytes.NewBufferString(connBody))
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST connections: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/connections")

	// POST proxy-pools
	poolBody := `{"name":"pool1","proxy_url":"http://x:8080","ptype":"http"}`
	resp = makeReq("POST", "/api/dashboard/proxy-pools", bytes.NewBufferString(poolBody))
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST proxy-pools: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/proxy-pools")

	// POST api-keys
	keyBody := `{"label":"test-key"}`
	resp = makeReq("POST", "/api/dashboard/api-keys", bytes.NewBufferString(keyBody))
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST api-keys: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/api-keys")

	// Verify lists have data
	for _, path := range []string{"/api/dashboard/combos", "/api/dashboard/providers", "/api/dashboard/connections", "/api/dashboard/proxy-pools"} {
		resp := makeReq("GET", path, nil)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		if len(parsed) == 0 {
			t.Fatalf("GET %s: empty response", path)
		}
		t.Logf("OK GET %s: %s", path, string(body)[:min(len(string(body)), 100)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
