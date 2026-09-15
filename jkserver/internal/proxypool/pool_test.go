package proxypool_test

import (
	"testing"

	"jkrouter/jkserver/internal/proxypool"
)

func TestStoreCRUD(t *testing.T) {
	store := proxypool.NewStore()
	id := store.Add("test-pool", "http://proxy.local:8080", "", false)
	if id <= 0 {
		t.Fatal("expected positive ID")
	}
	p := store.Get(id)
	if p == nil {
		t.Fatal("pool not found")
	}
	if p.Name != "test-pool" {
		t.Errorf("name mismatch: %s", p.Name)
	}
	if p.ProxyURL != "http://proxy.local:8080" {
		t.Errorf("proxy URL mismatch: %s", p.ProxyURL)
	}
	list := store.List()
	if len(list) != 1 {
		t.Errorf("expected 1 pool, got %d", len(list))
	}
}

func TestUpdateStatusDeactivatesOnUnhealthy(t *testing.T) {
	store := proxypool.NewStore()
	id := store.Add("p1", "http://proxy.local:3128", "", false)
	store.UpdateStatus(id, "unhealthy")
	p := store.Get(id)
	if p.IsActive {
		t.Error("expected pool to be inactive after unhealthy status")
	}
	if p.TestStatus != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", p.TestStatus)
	}
}

func TestExtractHostPort(t *testing.T) {
	host, port, err := proxypool.ExtractHostPort("http://proxy.example.com:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "proxy.example.com" {
		t.Errorf("host mismatch: %s", host)
	}
	if port != 8080 {
		t.Errorf("port mismatch: %d", port)
	}
}

func TestExtractHostPortDefaults(t *testing.T) {
	host, port, err := proxypool.ExtractHostPort("https://proxy.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 443 {
		t.Errorf("expected default port 443, got %d", port)
	}
	_ = host
}
