// Package rtk: Reduced Token Keeper — applies token-saving filters to chat requests.
// Filters: caveman (strip verbose prefixes), ponytail (strip comments/whitespace),
// headroom (truncate long messages), system-inject (inject guiding system prompt).
package rtk

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// Filter mutates a chat request body to reduce token usage.
// Return (modified, applied) — applied=false means no change needed.
type Filter interface {
	Name() string
	Apply(body []byte) ([]byte, bool)
}

// Registry holds enabled filters and applies them in order.
type Registry struct {
	filters []Filter
	enabled map[string]bool
}

// NewRegistry creates an empty filter registry.
func NewRegistry() *Registry {
	return &Registry{enabled: make(map[string]bool)}
}

// Enable adds a filter by name. Known names: "caveman", "ponytail", "headroom", "system-inject".
func (r *Registry) Enable(name string) {
	r.enabled[name] = true
	switch name {
	case "caveman":
		r.filters = append(r.filters, &cavemanFilter{})
	case "ponytail":
		r.filters = append(r.filters, &ponytailFilter{})
	case "headroom":
		r.filters = append(r.filters, &headroomFilter{})
	case "system-inject":
		r.filters = append(r.filters, &systemInjectFilter{})
	}
}

// Apply runs all enabled filters in order. Returns modified body and list of applied filter names.
func (r *Registry) Apply(body []byte) ([]byte, []string) {
	applied := make([]string, 0)
	out := body
	for _, f := range r.filters {
		if !r.enabled[f.Name()] {
			continue
		}
		modified, ok := f.Apply(out)
		if ok {
			out = modified
			applied = append(applied, f.Name())
		}
	}
	return out, applied
}

// ─────────────────────────── Filters ─────────────────────────────────────────

// cavemanFilter: strips verbose conversational prefixes from user messages.
type cavemanFilter struct{}

func (*cavemanFilter) Name() string { return "caveman" }

var cavemanPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?im)^ *(please |kindly |could you |would you |can you |i would like you to |i need you to )+`),
	regexp.MustCompile(`(?m)\b(?:respond\s+concisely|be\s+brief|keep\s+it\s+short|no\s+fluff|straight\s+to\s+the\s+point|cut\s+the\s+fluff)\b\s*[.!?]*\s*$`),
	regexp.MustCompile(`(?m)^\s*(thanks?!?\s+in\s+advance|thank\s+you\s+for\s+your\s+help)\s*[.!?]*\s*$`),
}

func (f *cavemanFilter) Apply(body []byte) ([]byte, bool) {
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil || len(req.Messages) == 0 {
		return body, false
	}

	changed := false
	for i, raw := range req.Messages {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok || content == "" {
			continue
		}
		newContent := content
		for _, re := range cavemanPatterns {
			newContent = re.ReplaceAllString(newContent, "")
		}
		newContent = strings.TrimSpace(newContent)
		if newContent != content {
			msg["content"] = newContent
			enc, _ := json.Marshal(msg)
			req.Messages[i] = enc
			changed = true
		}
	}
	if !changed {
		return body, false
	}
	out, _ := json.Marshal(req)
	return out, true
}

// ponytailFilter: strips comments (//, /* */), extra whitespace, redundant phrasing.
type ponytailFilter struct{}

func (*ponytailFilter) Name() string { return "ponytail" }

// ponytailRE matches single-line // comments and multi-line /* */ comments.
var ponytailRE = regexp.MustCompile(`(?m)(?:^|\n)\s*(?://[^\n]*|/\*[\s\S]*?\*/)\s*(?:\n|$)`)

func (f *ponytailFilter) Apply(body []byte) ([]byte, bool) {
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil || len(req.Messages) == 0 {
		return body, false
	}

	changed := false
	for i, raw := range req.Messages {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok || content == "" {
			continue
		}
		newContent := ponytailRE.ReplaceAllString(content, "\n")
		newContent = regexp.MustCompile(`\n{3,}`).ReplaceAllString(newContent, "\n\n")
		newContent = strings.TrimSpace(newContent)
		if newContent != content {
			msg["content"] = newContent
			enc, _ := json.Marshal(msg)
			req.Messages[i] = enc
			changed = true
		}
	}
	if !changed {
		return body, false
	}
	out, _ := json.Marshal(req)
	return out, true
}

// headroomFilter: truncates long user messages to estimated max tokens per message.
type headroomFilter struct {
	maxCharsPerMessage int
}

func NewHeadroomFilter(maxTokensPerMsg int) *headroomFilter {
	return &headroomFilter{maxCharsPerMessage: maxTokensPerMsg * 4}
}

func (f *headroomFilter) Name() string { return "headroom" }

func (f *headroomFilter) Apply(body []byte) ([]byte, bool) {
	if f.maxCharsPerMessage <= 0 {
		return body, false
	}
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil || len(req.Messages) == 0 {
		return body, false
	}

	changed := false
	for i, raw := range req.Messages {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "system" {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok || len(content) <= f.maxCharsPerMessage {
			continue
		}
		truncated := content[:f.maxCharsPerMessage]
		if lastSpace := strings.LastIndex(truncated, " "); lastSpace > f.maxCharsPerMessage*3/4 {
			truncated = truncated[:lastSpace]
		}
		truncated += "… [truncated]"
		msg["content"] = truncated
		enc, _ := json.Marshal(msg)
		req.Messages[i] = enc
		changed = true
	}
	if !changed {
		return body, false
	}
	out, _ := json.Marshal(req)
	return out, true
}

// systemInjectFilter: prepends a concise behavior hint to the system message.
type systemInjectFilter struct {
	hint string
}

func NewSystemInjectFilter(hint string) *systemInjectFilter {
	if hint == "" {
		hint = "Be concise. No preamble. Get to the point."
	}
	return &systemInjectFilter{hint: hint}
}

func (f *systemInjectFilter) Name() string { return "system-inject" }

func (f *systemInjectFilter) Apply(body []byte) ([]byte, bool) {
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil || len(req.Messages) == 0 {
		return body, false
	}

	for i, raw := range req.Messages {
		var msg map[string]interface{}
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		if msg["role"] != "system" {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			continue
		}
		if strings.Contains(content, f.hint) {
			continue
		}
		msg["content"] = content + "\n\n" + f.hint
		enc, _ := json.Marshal(msg)
		req.Messages[i] = enc
		return req.Messages[0], true
	}

	// No system message — prepend one.
	sysMsg, _ := json.Marshal(map[string]string{
		"role":    "system",
		"content": f.hint,
	})
	req.Messages = append([]json.RawMessage{sysMsg}, req.Messages...)
	out, _ := json.Marshal(req)
	return out, true
}

// ─────────────────────────── JSON helpers ─────────────────────────────────────

// EnabledFilters returns the list of currently enabled filter names.
func (r *Registry) EnabledFilters() []string {
	var out []string
	for _, f := range r.filters {
		if r.enabled[f.Name()] {
			out = append(out, f.Name())
		}
	}
	return out
}

// ApplyJSONBody is a convenience wrapper for applying filters to a JSON body.
func ApplyJSONBody(body []byte, enabled []string) ([]byte, []string, error) {
	reg := NewRegistry()
	for _, name := range enabled {
		reg.Enable(name)
	}
	applied := make([]string, 0, len(enabled))
	result, appliedFilters := reg.Apply(body)
	if len(appliedFilters) > 0 {
		applied = appliedFilters
	}
	return result, applied, nil
}

// CloneBody creates a shallow copy of the body bytes.
func CloneBody(body []byte) []byte {
	cp := make([]byte, len(body))
	copy(cp, body)
	return cp
}

// TrimBytes trims whitespace from the JSON body.
func TrimBytes(body []byte) []byte {
	return bytes.TrimSpace(body)
}
