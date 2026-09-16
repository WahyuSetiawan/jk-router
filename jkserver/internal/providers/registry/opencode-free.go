package registry

import (
	"net/http"
	"time"
)

func init() {
	Register(&Registry{
		ID:          "opencode-free",
		Name:        "OpenCode Free",
		BaseURL:     "https://api.opencode.ai/v1",
		ValidateURL: "https://api.opencode.ai/v1/models",
		ChatPath:    "/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "opencode-free", Name: "OpenCode Free", Capabilities: []string{}},
			{ID: "opencode-lite", Name: "OpenCode Lite", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
