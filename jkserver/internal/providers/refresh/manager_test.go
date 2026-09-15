package refresh_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jkrouter/jkserver/internal/providers/refresh"
	"jkrouter/jkserver/internal/providers/registry"
)

func newMockServer(models []map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"object": "list",
			"data":   models,
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestRefreshAddsNewModels(t *testing.T) {
	models := []map[string]interface{}{
		{"id": "gpt-4o", "name": "GPT-4 Omni", "modalities": []string{"text", "image"}},
		{"id": "gpt-4o-new", "name": "GPT-4 Omni New", "modalities": []string{"text", "image", "audio"}},
	}
	server := newMockServer(models)
	defer server.Close()

	reg := &registry.Registry{
		ID:          "test-provider",
		Name:        "Test Provider",
		BaseURL:     server.URL,
		ChatPath:    "/v1/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer",
		Headers:     map[string]string{"Content-Type": "application/json"},
		Models:      []registry.Model{{ID: "gpt-4o", Name: "GPT-4 Omni", Capabilities: []string{"vision"}}},
		ValidateURL: server.URL + "/v1/models",
		ClientFn:    registry.DefaultClient,
	}
	registry.Register(reg)
	defer registry.UnregisterForTest("test-provider")

	mgr := refresh.NewManager(
		refresh.WithAPIKey("test-key"),
		refresh.WithInterval(1*time.Hour),
	)
	mgr.Start()
	defer mgr.Stop()

	time.Sleep(1500 * time.Millisecond)

	fresh := registry.FindByID("test-provider")
	if fresh == nil {
		t.Fatal("test provider registry not found after refresh")
	}
	ids := make(map[string]bool)
	for _, m := range fresh.Models {
		ids[m.ID] = true
	}
	if !ids["gpt-4o"] {
		t.Error("original model 'gpt-4o' should still exist")
	}
	if !ids["gpt-4o-new"] {
		t.Error("new model 'gpt-4o-new' should have been added by refresh")
	}
	if len(fresh.Models) != 2 {
		t.Errorf("expected 2 models after refresh, got %d", len(fresh.Models))
	}
}

func TestRefreshSkipsProviderWithoutValidateURL(t *testing.T) {
	reg := &registry.Registry{
		ID:       "no-refresh-provider",
		Name:     "No Refresh",
		BaseURL:  "https://example.com",
		Models:   []registry.Model{{ID: "model-a"}},
		ClientFn: registry.DefaultClient,
	}
	registry.Register(reg)
	defer registry.UnregisterForTest("no-refresh-provider")

	mgr := refresh.NewManager(refresh.WithAPIKey("key"))
	mgr.Start()
	defer mgr.Stop()

	time.Sleep(1500 * time.Millisecond)

	fresh := registry.FindByID("no-refresh-provider")
	if fresh == nil {
		t.Fatal("provider should still exist")
	}
	if len(fresh.Models) != 1 {
		t.Errorf("expected 1 model (no refresh), got %d", len(fresh.Models))
	}
}
