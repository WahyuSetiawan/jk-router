// Package engine provides combo fallback orchestration and capability detection.
package engine

import (
	"encoding/json"
	"sort"
)

// Capability represents an AI model capability.
type Capability string

const (
	CapVision Capability = "vision"
	CapAudio  Capability = "audio"
	CapVideo  Capability = "video"
	CapTool   Capability = "tool"
)

// DetectRequiredCapabilities scans an OpenAI-format chat request body for
// multimodal requirements (image_url blocks, audio content, etc).
func DetectRequiredCapabilities(body []byte) map[Capability]bool {
	caps := make(map[Capability]bool)

	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return caps
	}
	for _, raw := range req.Messages {
		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		content := msg["content"]
		switch v := content.(type) {
		case []interface{}:
			for _, block := range v {
				bm, ok := block.(map[string]interface{})
				if !ok {
					continue
				}
				switch bm["type"] {
				case "image_url", "image":
					caps[CapVision] = true
				case "input_audio":
					caps[CapAudio] = true
				case "video":
					caps[CapVideo] = true
				}
			}
		case string:
			_ = v
		}
		if _, ok := msg["tool_calls"]; ok {
			caps[CapTool] = true
		}
	}
	return caps
}

// ModelCaps holds a model ID and its known capabilities.
type ModelCaps struct {
	ID             string
	Capabilities   []Capability
}

// HasCapability reports whether m has the given capability.
func (m ModelCaps) HasCapability(c Capability) bool {
	for _, cap := range m.Capabilities {
		if cap == c {
			return true
		}
	}
	return false
}

// HasAnyCapability reports whether any model in the list has at least one of the required capabilities.
func HasAnyCapability(models []ModelCaps, required []Capability) bool {
	for _, m := range models {
		for _, c := range required {
			if m.HasCapability(c) {
				return true
			}
		}
	}
	return false
}

// SortModelsByCapabilities reorders models so that those matching required
// capabilities come first. Models without matches retain their relative order.
func SortModelsByCapabilities(models []ModelCaps, required []Capability) []ModelCaps {
	sorted := make([]ModelCaps, len(models))
	copy(sorted, models)
	sort.SliceStable(sorted, func(i, j int) bool {
		iMatch := hasAnyCap(sorted[i], required)
		jMatch := hasAnyCap(sorted[j], required)
		if iMatch == jMatch {
			return false // stable sort preserves original order
		}
		return iMatch // true comes before false
	})
	return sorted
}

func hasAnyCap(m ModelCaps, required []Capability) bool {
	for _, c := range required {
		if m.HasCapability(c) {
			return true
		}
	}
	return false
}
