package engine_test

import (
	"encoding/json"
	"testing"

	"jkrouter/jkserver/internal/engine"
)

func TestDetectVisionCapability(t *testing.T) {
	req := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "what's in this image?"},
				{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,abc"}},
			},
		}},
	}
	body, _ := json.Marshal(req)
	caps := engine.DetectRequiredCapabilities(body)
	if !caps[engine.CapVision] {
		t.Errorf("expected vision capability")
	}
}

func TestDetectNoCapability(t *testing.T) {
	req := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{{
			"role":    "user",
			"content": "hello world",
		}},
	}
	body, _ := json.Marshal(req)
	caps := engine.DetectRequiredCapabilities(body)
	if len(caps) != 0 {
		t.Errorf("expected no capabilities, got %v", caps)
	}
}

func TestDetectToolCapability(t *testing.T) {
	req := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "use the weather tool"},
			{"role": "assistant", "content": "", "tool_calls": []map[string]interface{}{
				{"id": "1", "type": "function", "function": map[string]interface{}{"name": "get_weather"}},
			}},
		},
	}
	body, _ := json.Marshal(req)
	caps := engine.DetectRequiredCapabilities(body)
	if !caps[engine.CapTool] {
		t.Errorf("expected tool capability")
	}
}

func TestSortModelsByCapabilities(t *testing.T) {
	models := []engine.ModelCaps{
		{ID: "text-only-a", Capabilities: []engine.Capability{}},
		{ID: "vision-b", Capabilities: []engine.Capability{engine.CapVision}},
		{ID: "vision-c", Capabilities: []engine.Capability{engine.CapVision, engine.CapAudio}},
		{ID: "text-only-d", Capabilities: []engine.Capability{}},
	}
	sorted := engine.SortModelsByCapabilities(models, []engine.Capability{engine.CapVision})
	if sorted[0].ID != "vision-b" {
		t.Errorf("expected vision-b first, got %s", sorted[0].ID)
	}
	if sorted[1].ID != "vision-c" {
		t.Errorf("expected vision-c second, got %s", sorted[1].ID)
	}
}

func TestHasAnyCapability(t *testing.T) {
	models := []engine.ModelCaps{
		{ID: "text-only", Capabilities: []engine.Capability{}},
		{ID: "vision-model", Capabilities: []engine.Capability{engine.CapVision}},
	}
	if !engine.HasAnyCapability(models, []engine.Capability{engine.CapVision}) {
		t.Error("expected HasAnyCapability to find vision")
	}
	if engine.HasAnyCapability(models, []engine.Capability{engine.CapAudio}) {
		t.Error("should not have audio capability")
	}
}

func TestModelCapsHasCapability(t *testing.T) {
	m := engine.ModelCaps{ID: "gpt-4o", Capabilities: []engine.Capability{engine.CapVision}}
	if !m.HasCapability(engine.CapVision) {
		t.Error("expected vision capability")
	}
	if m.HasCapability(engine.CapAudio) {
		t.Error("should not have audio capability")
	}
}
