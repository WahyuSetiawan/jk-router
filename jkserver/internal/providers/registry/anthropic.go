package registry

import "net/http"

import "time"

func init() {
	Register(&Registry{
		ID:         "anthropic",
		Name:       "Anthropic",
		BaseURL:    "https://api.anthropic.com",
		ChatPath:   "/v1/messages",
		AuthHeader: "x-api-key",
		AuthPrefix: "",
		Headers: map[string]string{
			"Content-Type":        "application/json",
			"anthropic-version":  "2023-06-01",
		},
		Models: []Model{
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Capabilities: []string{"vision"}},
			{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Capabilities: []string{}},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Capabilities: []string{"vision"}},
			{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Capabilities: []string{"vision"}},
			{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Capabilities: []string{"vision"}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
