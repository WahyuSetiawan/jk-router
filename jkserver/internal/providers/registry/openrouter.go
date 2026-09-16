package registry

import (
	"net/http"
	"time"
)

func init() {
	Register(&Registry{
		ID:          "openrouter",
		Name:        "OpenRouter",
		BaseURL:     "https://openrouter.ai/api/v1",
		ValidateURL: "https://openrouter.ai/api/v1/models",
		ChatPath:    "/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "openai/gpt-4o", Name: "GPT-4o (OpenRouter)", Capabilities: []string{"vision"}},
			{ID: "anthropic/claude-sonnet-4", Name: "Claude Sonnet 4 (OpenRouter)", Capabilities: []string{}},
			{ID: "anthropic/claude-haiku-3", Name: "Claude Haiku 3 (OpenRouter)", Capabilities: []string{}},
			{ID: "google/gemini-2.0-flash-001", Name: "Gemini Flash 2.0 (OpenRouter)", Capabilities: []string{"vision"}},
			{ID: "deepseek/deepseek-chat", Name: "DeepSeek Chat (OpenRouter)", Capabilities: []string{}},
			{ID: "deepseek/deepseek-coder", Name: "DeepSeek Coder (OpenRouter)", Capabilities: []string{}},
			{ID: "meta-llama/llama-3.3-70b-instruct", Name: "Llama 3.3 70B (OpenRouter)", Capabilities: []string{}},
			{ID: "qwen/qwen-2.5-72b-instruct", Name: "Qwen 2.5 72B (OpenRouter)", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
