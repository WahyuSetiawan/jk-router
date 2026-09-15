// Package registry holds provider configurations for AI model providers.
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Model describes a single AI model known to a provider.
type Model struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"` // "vision", "audio", etc.
}

// Registry holds the compiled configuration for one upstream provider.
type Registry struct {
	ID              string
	Name            string
	BaseURL         string
	ChatPath        string
	AuthHeader      string
	AuthPrefix      string
	Headers         map[string]string
	Models          []Model
	ClientFn        func(apiKey string) *http.Client
	ValidateURL     string // upstream /models endpoint for auto-refresh
	LastRefreshTime time.Time
}

// DefaultClient returns a standard http.Client with a 120s timeout.
func DefaultClient(_ string) *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}

// ListModels returns a copy of the manual model catalog.
func (r *Registry) ListModels() []Model {
	out := make([]Model, len(r.Models))
	copy(out, r.Models)
	return out
}

// RefreshModels fetches the latest model list from ValidateURL and merges
// new models into the catalog. Existing manual models are never removed.
func (r *Registry) RefreshModels(apiKey string) error {
	if r.ValidateURL == "" {
		return fmt.Errorf("provider %s: no validateUrl configured, skip auto-refresh", r.ID)
	}
	client := r.ClientFn(apiKey)
	resp, err := client.Get(r.ValidateURL)
	if err != nil {
		return fmt.Errorf("provider %s: fetch models failed: %w", r.ID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return fmt.Errorf("provider %s: read models body failed: %w", r.ID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("provider %s: unexpected status %d", r.ID, resp.StatusCode)
	}

	// Parse OpenAI-compatible /models response: { "data": [{ "id": "...", "modalities": [...] }, ...] }
	var wrapper struct {
		Data []struct {
			ID         string   `json:"id"`
			Name       string   `json:"name"`
			Modalities []string `json:"modalities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		// Some providers return plain arrays; attempt direct parse.
		var models []struct {
			ID string `json:"id"`
		}
		if err2 := json.Unmarshal(body, &models); err2 != nil {
			return fmt.Errorf("provider %s: unparseable models response: %w", r.ID, err)
		}
		existing := make(map[string]bool)
		for _, m := range r.Models {
			existing[m.ID] = true
		}
		for _, m := range models {
			if !existing[m.ID] {
				r.Models = append(r.Models, Model{ID: m.ID})
			}
		}
		r.LastRefreshTime = time.Now()
		return nil
	}

	existing := make(map[string]bool)
	for _, m := range r.Models {
		existing[m.ID] = true
	}
	for _, item := range wrapper.Data {
		if existing[item.ID] {
			continue
		}
		caps := inferCapabilities(item.Modalities)
		r.Models = append(r.Models, Model{
			ID:           item.ID,
			Name:         item.Name,
			Capabilities: caps,
		})
	}
	r.LastRefreshTime = time.Now()
	return nil
}

// inferCapabilities maps modalities strings to capability labels.
func inferCapabilities(modalities []string) []string {
	capSet := make(map[string]bool)
	for _, m := range modalities {
		switch m {
		case "text", "image":
			capSet["vision"] = true
		case "audio":
			capSet["audio"] = true
		case "video":
			capSet["video"] = true
		}
	}
	var caps []string
	for c := range capSet {
		caps = append(caps, c)
	}
	return caps
}

var (
	allRegistriesMu sync.RWMutex
	allRegistries   []*Registry
)

// Register adds a provider registry to the global list (called from init()).
func Register(r *Registry) {
	allRegistriesMu.Lock()
	defer allRegistriesMu.Unlock()
	allRegistries = append(allRegistries, r)
}

// GetRegistries returns a read-only snapshot of all registered providers.
func GetRegistries() []*Registry {
	allRegistriesMu.RLock()
	defer allRegistriesMu.RUnlock()
	out := make([]*Registry, len(allRegistries))
	copy(out, allRegistries)
	return out
}

// FindByID returns the registry for the given provider ID, or nil.
func FindByID(id string) *Registry {
	allRegistriesMu.RLock()
	defer allRegistriesMu.RUnlock()
	for _, r := range allRegistries {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// ListAllModelIDs returns every model ID across all providers (flat list).
func ListAllModelIDs() []string {
	allRegistriesMu.RLock()
	defer allRegistriesMu.RUnlock()
	seen := make(map[string]bool)
	var ids []string
	for _, r := range allRegistries {
		for _, m := range r.Models {
			if !seen[m.ID] {
				seen[m.ID] = true
				ids = append(ids, m.ID)
			}
		}
	}
	return ids
}

// UnregisterForTest removes a registry by ID (test helper).
func UnregisterForTest(id string) {
	allRegistriesMu.Lock()
	defer allRegistriesMu.Unlock()
	filtered := allRegistries[:0]
	for _, r := range allRegistries {
		if r.ID != id {
			filtered = append(filtered, r)
		}
	}
	allRegistries = filtered
}
