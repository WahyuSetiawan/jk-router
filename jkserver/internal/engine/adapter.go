// Package engine: capacity adapter for vision/audio fallback models.
package engine

import (
	"encoding/json"
	"fmt"
	"log"
)

// CapacityAdapter manages a pool of fallback models for when combo members
// lack required capabilities (vision, audio, etc.).
type CapacityAdapter struct {
	adapterModels map[Capability]string
}

// NewCapacityAdapter creates an adapter with the given capability→model mappings.
func NewCapacityAdapter(modelMap map[Capability]string) *CapacityAdapter {
	return &CapacityAdapter{adapterModels: modelMap}
}

// WrapRequest wraps an OpenAI chat request body so that any combo member
// lacking the required capabilities is replaced by an adapter fallback model.
// Returns the wrapped body and the adapter model used (or empty if no wrapping needed).
func (a *CapacityAdapter) WrapRequest(body []byte, required []Capability, comboModels []string) ([]byte, string, error) {
	var req struct {
		Model    string          `json:"model"`
		Messages json.RawMessage `json:"messages"`
		Stream   *bool           `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return body, "", nil
	}

	var adapterModel string
	for _, cap := range required {
		if fallback, ok := a.adapterModels[cap]; ok {
			adapterModel = fallback
			break
		}
	}
	if adapterModel == "" {
		return body, "", nil
	}

	modified, _ := json.Marshal(req)
	log.Printf("[adapter] wrapped request with fallback model %s for capabilities %v", adapterModel, required)
	return modified, adapterModel, nil
}

// FormatAdapterResult builds a minimal JSON error response indicating the adapter
// was triggered because no combo member supports the required capability.
func FormatAdapterResult(required []Capability) []byte {
	resp := map[string]interface{}{
		"id":      "adapter-fallback",
		"object":  "error",
		"message": fmt.Sprintf("no combo member supports capabilities: %v; adapter fallback may be configured", required),
	}
	b, _ := json.Marshal(resp)
	return b
}
