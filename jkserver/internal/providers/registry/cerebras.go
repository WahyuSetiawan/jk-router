package registry

import "net/http"

import "time"

func init() {
	Register(&Registry{
		ID:         "cerebras",
		Name:       "Cerebras",
		BaseURL:    "https://api.cerebras.ai",
		ChatPath:   "/v1/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "llama-3.1-70b", Name: "Llama 3.1 70B", Capabilities: []string{}},
			{ID: "llama-3.1-8b", Name: "Llama 3.1 8B", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
