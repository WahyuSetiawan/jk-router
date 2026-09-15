package registry

import "net/http"

import "time"

func init() {
	Register(&Registry{
		ID:         "deepseek",
		Name:       "DeepSeek",
		BaseURL:    "https://api.deepseek.com",
		ValidateURL: "https://api.deepseek.com/models",
		ChatPath:   "/chat/completions",
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "deepseek-chat", Name: "DeepSeek Chat", Capabilities: []string{}},
			{ID: "deepseek-coder", Name: "DeepSeek Coder", Capabilities: []string{}},
			{ID: "deepseek-reasoner", Name: "DeepSeek R1", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
