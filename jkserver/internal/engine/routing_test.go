package engine_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jkrouter/jkserver/internal/engine"
	"jkrouter/jkserver/internal/providers/registry"
)

func newTestProvider(id, baseURL string) *engine.ProviderMeta {
	return &engine.ProviderMeta{
		ID:         id,
		BaseURL:    baseURL,
		ChatPath:   "/v1/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer",
		ClientFn:   func(_ string) *http.Client { return &http.Client{Timeout: 5 * time.Second} },
		ModelIDs:   []string{"gpt-4o", "text-only"},
	}
}

func TestComboFallbackSecondAccount(t *testing.T) {
	attempt1 := 0
	upstream1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt1++
		if attempt1 == 1 {
			w.WriteHeader(429)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer upstream1.Close()

	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"2","choices":[{"message":{"content":"from acc2"},"finish_reason":"stop"}]}`))
	}))
	defer upstream2.Close()

	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "p1", APIKey: "key1", State: engine.StateActive})
	store.Upsert(&engine.Account{ID: 2, ProviderID: "p2", APIKey: "key2", State: engine.StateActive})

	comboStore := engine.NewComboStore()
	comboStore.Add("test-combo", "fallback", []string{"gpt-4o"}, []int64{1, 2})

	var logged []engine.RouteLogEntry
	cfg := &engine.RoutingConfig{
		AccountStore:   store,
		ComboStore:     comboStore,
		Providers: []*engine.ProviderMeta{
			newTestProvider("p1", upstream1.URL),
			newTestProvider("p2", upstream2.URL),
		},
		DefaultCooldown: 60 * time.Second,
		LogUsage: func(e engine.RouteLogEntry) {
			logged = append(logged, e)
		},
		SaveStateFunc: func(id int64, state string, strike int, cooledUntil, updatedAt int64) {
			a := store.Get(id)
			if a != nil {
				a.StrikeCount = strike
				a.State = engine.AccountState(state)
			}
		},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})

	w := httptest.NewRecorder()
	engine.ExecuteRouting(body, "key1", false, w, cfg)

	if w.Code != 200 {
		t.Fatalf("expected 200 after fallback, got %d body=%s", w.Code, w.Body.String())
	}
	if attempt1 < 2 {
		t.Errorf("expected upstream1 to be hit at least twice (original + retry), got %d", attempt1)
	}
}

func TestCapabilityFilteringRejectsUnsupported(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "text-provider", APIKey: "key1", State: engine.StateActive})

	comboStore := engine.NewComboStore()
	comboStore.Add("test", "fallback", []string{"text-only"}, []int64{1})

	// Register a provider that has NO vision models.
	registry.UnregisterForTest("text-provider")
	registry.Register(&registry.Registry{
		ID: "text-provider",
		Models: []registry.Model{
			{ID: "text-only", Name: "Text Only", Capabilities: []string{}},
		},
	})
	defer registry.UnregisterForTest("text-provider")

	cfg := &engine.RoutingConfig{
		AccountStore:   store,
		ComboStore:     comboStore,
		Providers: []*engine.ProviderMeta{
			{
				ID:         "text-provider",
				BaseURL:    upstream.URL,
				ChatPath:   "/v1/chat/completions",
				AuthHeader: "Authorization",
				AuthPrefix: "Bearer",
				ClientFn:   func(_ string) *http.Client { return &http.Client{Timeout: 5 * time.Second} },
				ModelIDs:   []string{"text-only"},
			},
		},
		DefaultCooldown: 60 * time.Second,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model": "text-only",
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": "what's in this?"},
				{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,abc"}},
			}},
		},
	})

	w := httptest.NewRecorder()
	engine.ExecuteRouting(body, "key1", false, w, cfg)

	if w.Code == 200 {
		resp, _ := io.ReadAll(w.Body)
		t.Errorf("expected failure when no candidates support vision, got 200 body=%s", string(resp))
	}
}

func TestProxyPoolTransportApplied(t *testing.T) {
	var gotAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "p1", APIKey: "key1", State: engine.StateActive})

	comboStore := engine.NewComboStore()
	comboStore.Add("test", "fallback", []string{"gpt-4o"}, []int64{1})

	cfg := &engine.RoutingConfig{
		AccountStore:   store,
		ComboStore:     comboStore,
		Providers: []*engine.ProviderMeta{
			newTestProvider("p1", upstream.URL),
		},
		DefaultCooldown: 60 * time.Second,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})

	w := httptest.NewRecorder()
	engine.ExecuteRouting(body, "key1", false, w, cfg)

	if gotAuthHeader != "Bearer key1" {
		t.Errorf("expected 'Bearer key1', got '%s'", gotAuthHeader)
	}
}

func TestNoCandidatesWhenAllDisabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"id":"1","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "p1", APIKey: "key1", State: engine.StateDisabled})
	store.Upsert(&engine.Account{ID: 2, ProviderID: "p2", APIKey: "key2", State: engine.StateCoolingDown})

	comboStore := engine.NewComboStore()
	comboStore.Add("test", "fallback", []string{"gpt-4o"}, []int64{1, 2})

	cfg := &engine.RoutingConfig{
		AccountStore:   store,
		ComboStore:     comboStore,
		Providers: []*engine.ProviderMeta{
			newTestProvider("p1", upstream.URL),
			newTestProvider("p2", upstream.URL),
		},
		DefaultCooldown: 60 * time.Second,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})

	w := httptest.NewRecorder()
	engine.ExecuteRouting(body, "key1", false, w, cfg)

	if w.Code == 200 {
		resp, _ := io.ReadAll(w.Body)
		t.Errorf("expected failure when all accounts disabled/cooldown, got 200 body=%s", string(resp))
	}
}
