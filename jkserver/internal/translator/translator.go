// Package translator provides format-translation between AI client protocols
// (OpenAI Chat Completions, Anthropic Messages, Gemini) and upstream providers.
//
// Design (PRD §4):
//   - pivot = OpenAI is the canonical intermediate format
//   - direct-route pairs (openai↔claude) are implemented here because they're
//     fragile and high-frequency
//   - additional providers route through openai pivot when no direct path exists
package translator

import (
	"fmt"
	"io"
	"sync"
)

// Format identifies a message protocol.
type Format string

const (
	FormatOpenAI    Format = "openai"
	FormatAnthropic Format = "anthropic"
	FormatGemini    Format = "gemini"
)

// Pair identifies a translation direction: "from:to".
type Pair string

// PairFor formats a from→to pair string.
func PairFor(from, to Format) Pair { return Pair(fmt.Sprintf("%s:%s", from, to)) }

// TranslateRequest converts a request body from src to dst format.
type TranslateRequest func(reqBody []byte, from, to Format) ([]byte, error)

// TranslateResponse converts an upstream response body from src to dst format.
type TranslateResponse func(respBody []byte, src, dst Format, respHeaders map[string][]string) ([]byte, error)

// TranslateStream reads src-format SSE events from src and writes dst-format
// SSE events to dst.
type TranslateStream func(src io.Reader, dst io.Writer, srcFmt, dstFmt Format) error

// Translator handles one translation pair in both directions.
type Translator struct {
	Request  TranslateRequest
	Response TranslateResponse
	Stream   TranslateStream
}

// Registry stores available format translators.
type Registry struct {
	mu       sync.RWMutex
	transl   map[Pair]*Translator
	defaultT *Translator
}

// NewRegistry creates an empty registry. Callers must register translators
// before use (see package init functions).
func NewRegistry() *Registry {
	return &Registry{transl: make(map[Pair]*Translator)}
}

// Register adds a translator for the given pair.
func (r *Registry) Register(pair Pair, t *Translator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transl[pair] = t
	reverse := Pair(fmt.Sprintf("%s:%s", pair.To(), pair.From()))
	if _, ok := r.transl[reverse]; !ok {
		r.transl[reverse] = &Translator{
			Request:  reverseReq(t.Request),
			Response: reverseResp(t.Response),
			Stream:   t.Stream,
		}
	}
}

// SetDefault sets the fallback translator for unknown pairs.
func (r *Registry) SetDefault(t *Translator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultT = t
}

// Get returns the translator for the given pair, or the default if none found.
func (r *Registry) Get(pair Pair) *Translator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.transl[pair]; ok {
		return t
	}
	return r.defaultT
}

// To returns the destination format of a pair.
func (p Pair) To() Format {
	parts := splitPair(p)
	if len(parts) == 2 {
		return Format(parts[1])
	}
	return ""
}

// From returns the source format of a pair.
func (p Pair) From() Format {
	parts := splitPair(p)
	if len(parts) == 2 {
		return Format(parts[0])
	}
	return ""
}

func splitPair(p Pair) []string {
	n := 0
	for i := 0; i < len(p); i++ {
		if p[i] == ':' {
			n++
			if n == 2 {
				return []string{string(p[:i]), string(p[i+1:])}
			}
		}
	}
	return nil
}

func reverseReq(fn TranslateRequest) TranslateRequest { return fn }
func reverseResp(fn TranslateResponse) TranslateResponse { return fn }
