package registry

import "net/http"

import "time"

func init() {
	Register(&Registry{
		ID:         "openai",
		Name:       "OpenAI",
		BaseURL:    "https://api.openai.com",
		ValidateURL: "https://api.openai.com/v1/models",
		ChatPath:   "/v1/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "gpt-4o", Name: "GPT-4 Omni", Capabilities: []string{"vision"}},
			{ID: "gpt-4o-mini", Name: "GPT-4 Omni Mini", Capabilities: []string{}},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Capabilities: []string{"vision"}},
			{ID: "gpt-4", Name: "GPT-4", Capabilities: []string{}},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Capabilities: []string{}},
			{ID: "o1", Name: "o1", Capabilities: []string{}},
			{ID: "o1-mini", Name: "o1 Mini", Capabilities: []string{}},
			{ID: "o3-mini", Name: "o3 Mini", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
