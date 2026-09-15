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

	r := chi.NewRouter()
	r.Mount("/api/dashboard", DashboardRouter(d))
	ts := httptest.NewServer(r)
	defer ts.Close()

	// GET endpoints
	getEndpoints := []string{
		
		"/api/dashboard/providers",
		"/api/dashboard/connections",
		"/api/dashboard/combos",
		"/api/dashboard/proxy-pools",
		"/api/dashboard/api-keys",
	}
	for _, path := range getEndpoints {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET %s: status=%d body=%s", path, resp.StatusCode, string(body))
		}
		resp.Body.Close()
		t.Logf("OK GET %s (200)", path)
	}

	// POST combos
	comboBody := `{"name":"gpt-combo","model_ids":["gpt-4o","claude-sonnet"],"strategy":"fallback"}`
	resp, err := http.Post(ts.URL+"/api/dashboard/combos", "application/json", bytes.NewBufferString(comboBody))
	if err != nil {
		t.Fatalf("POST combos: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST combos: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/combos")

	// POST providers
	provBody := `{"name":"openai","label":"Main","auth_type":"api_key"}`
	resp, err = http.Post(ts.URL+"/api/dashboard/providers", "application/json", bytes.NewBufferString(provBody))
	if err != nil {
		t.Fatalf("POST providers: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST providers: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/providers")

	// POST connections
	connBody := `{"provider_id":"openai","name":"key1","secret":"sk-test123","priority":0}`
	resp, err = http.Post(ts.URL+"/api/dashboard/connections", "application/json", bytes.NewBufferString(connBody))
	if err != nil {
		t.Fatalf("POST connections: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST connections: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/connections")

	// POST proxy-pools
	poolBody := `{"name":"pool1","proxy_url":"http://x:8080","ptype":"http"}`
	resp, err = http.Post(ts.URL+"/api/dashboard/proxy-pools", "application/json", bytes.NewBufferString(poolBody))
	if err != nil {
		t.Fatalf("POST proxy-pools: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST proxy-pools: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/proxy-pools")

	// POST api-keys
	keyBody := `{"label":"test-key"}`
	resp, err = http.Post(ts.URL+"/api/dashboard/api-keys", "application/json", bytes.NewBufferString(keyBody))
	if err != nil {
		t.Fatalf("POST api-keys: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST api-keys: status=%d body=%s", resp.StatusCode, string(body))
	}
	resp.Body.Close()
	t.Logf("OK POST /api/dashboard/api-keys")

	// Verify lists have data
	for _, path := range []string{"/api/dashboard/combos", "/api/dashboard/providers", "/api/dashboard/connections", "/api/dashboard/proxy-pools"} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET %s: status=%d body=%s", path, resp.StatusCode, string(body))
		}
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
