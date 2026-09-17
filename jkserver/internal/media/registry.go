// Package media provides a separate registry for media providers (TTS, STT, image).
// Media providers use different API contracts than chat — each operation has its
// own endpoint, auth, and request shape. This package implements the registry
// and dispatch logic; routes are wired in api/router.go.
//
// OpenAI-compatible contracts:
//   POST /v1/audio/speech          — TTS  (JSON body → audio file)
//   POST /v1/audio/transcriptions — STT  (multipart form → JSON transcript)
//   POST /v1/images/generations   — Image (JSON body → image URL/data)
package media

import (
	"sync"
)

// Capability describes which media operations a provider supports.
type Capability int

const (
	CapTTS  Capability = 1 << iota // text-to-speech (/v1/audio/speech)
	CapSTT                         // speech-to-text (/v1/audio/transcriptions)
	CapImage                       // image generation (/v1/images/generations)
)

// Registry holds configuration for one upstream media provider.
type Registry struct {
	ID   string // e.g. "openai", "elevenlabs"
	Name string

	Capabilities Capability

	// TTS config
	TTSBaseURL    string
	TTSPath       string
	TTSAuthHeader string
	TTSAuthPrefix string

	// STT config
	STTBaseURL    string
	STTPath       string
	STTAuthHeader string
	STTAuthPrefix string

	// Image config
	ImageBaseURL    string
	ImagePath       string
	ImageAuthHeader string
	ImageAuthPrefix string
}

var (
	mediaMu     sync.RWMutex
	mediaRegs   []*Registry
)

// Register adds a media provider to the global list (called from init()).
func Register(r *Registry) {
	mediaMu.Lock()
	defer mediaMu.Unlock()
	mediaRegs = append(mediaRegs, r)
}

// GetRegistries returns a read-only snapshot of all registered media providers.
func GetRegistries() []*Registry {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	out := make([]*Registry, len(mediaRegs))
	copy(out, mediaRegs)
	return out
}

// FindByID returns the registry for the given provider ID, or nil.
func FindByID(id string) *Registry {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	for _, r := range mediaRegs {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// ListAllIDs returns every registered media provider ID.
func ListAllIDs() []string {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	var ids []string
	for _, r := range mediaRegs {
		ids = append(ids, r.ID)
	}
	return ids
}
