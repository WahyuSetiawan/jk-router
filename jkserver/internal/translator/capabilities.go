// Package translator — capability detection helpers.
package translator

import "encoding/json"

// Capability describes a modality supported by a model.
type Capability string

const (
	CapVision  Capability = "vision"
	CapAudio   Capability = "audio"
	CapPDF     Capability = "pdf"
	CapVideo   Capability = "video"
)

// RequiredCapabilities scans an OpenAI-format chat request body and returns
// which modalities are present in the messages.
func RequiredCapabilities(body []byte) map[Capability]bool {
	caps := make(map[Capability]bool)
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return caps
	}
	for _, raw := range req.Messages {
		detectCapsFromMessage(raw, caps)
	}
	return caps
}

func detectCapsFromMessage(raw json.RawMessage, caps map[Capability]bool) {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	if len(msg.Content) == 0 {
		return
	}
	var blocks []map[string]interface{}
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return
	}
	for _, b := range blocks {
		switch b["type"] {
		case "image_url", "image":
			caps[CapVision] = true
		case "input_audio", "audio":
			caps[CapAudio] = true
		case "input_document", "pdf":
			caps[CapPDF] = true
		}
	}
}

// ModelCaps is a lightweight model-with-capabilities descriptor used in the
// engine layer to avoid importing the registry package.
type ModelCaps struct {
	ID           string
	Capabilities []Capability
}

// HasCapability checks if a ModelCaps has the given capability.
func (m ModelCaps) HasCapability(c Capability) bool {
	for _, cap := range m.Capabilities {
		if cap == c {
			return true
		}
	}
	return false
}

// HasAnyCapability returns true if model has any of the requested caps.
func HasAnyCapability(models []ModelCaps, needed []Capability) bool {
	for _, m := range models {
		for _, n := range needed {
			if m.HasCapability(n) {
				return true
			}
		}
	}
	return false
}
