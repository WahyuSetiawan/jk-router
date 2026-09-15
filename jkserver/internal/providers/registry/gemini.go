package registry

import "net/http"

import "time"

func init() {
	Register(&Registry{
		ID:         "gemini",
		Name:       "Google Gemini",
		BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
		ChatPath:   "/openai/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Capabilities: []string{"vision"}},
			{ID: "gemini-2.0-flash-lite", Name: "Gemini 2.0 Flash Lite", Capabilities: []string{"vision"}},
			{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro", Capabilities: []string{"vision"}},
			{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash", Capabilities: []string{"vision"}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
