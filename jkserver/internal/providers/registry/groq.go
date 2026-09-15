package registry

import "net/http"

import "time"

func init() {
	Register(&Registry{
		ID:         "groq",
		Name:       "Groq",
		BaseURL:    "https://api.groq.com/openai",
		ChatPath:   "/v1/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Capabilities: []string{}},
			{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B", Capabilities: []string{}},
			{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
