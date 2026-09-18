package media

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

)

// mockStore is a minimal Store implementation for testing.
type mockStore struct {
	accounts []*Account
}

func (m *mockStore) LoadAccounts(fn func(*Account)) {
	for _, a := range m.accounts {
		fn(a)
	}
}

func TestMediaRouterReturns401WithoutBearer(t *testing.T) {
	store := &mockStore{}
	r := Router(store)

	// All media endpoints require Bearer auth.
	endpoints := []struct {
		method string
		path   string
		body   []byte
	}{
		{"POST", "/audio/speech", []byte(`{"model":"openai","input":"hi"}`)},
		{"POST", "/images/generations", []byte(`{"model":"dall-e-3"}`)},
		{"POST", "/videos/generations", []byte(`{"model":"runway"}`)},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewReader(ep.body))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401 without Bearer, got %d", ep.method, ep.path, rec.Code)
		}
	}
}

func TestMediaRouterTTSNoActiveAccounts(t *testing.T) {
	store := &mockStore{} // no accounts
	r := Router(store)

	req := httptest.NewRequest("POST", "/audio/speech", bytes.NewReader([]byte(`{"model":"openai","input":"hi"}`)))
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	// Should return 503 because no active TTS accounts
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 503 with no accounts, got %d", rec.Code)
	}
}

func TestMediaRouterImageNoActiveAccounts(t *testing.T) {
	store := &mockStore{}
	r := Router(store)

	req := httptest.NewRequest("POST", "/images/generations", bytes.NewReader([]byte(`{"model":"dall-e-3"}`)))
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 503 with no accounts, got %d", rec.Code)
	}
}
