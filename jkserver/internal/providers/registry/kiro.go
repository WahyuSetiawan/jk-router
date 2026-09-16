package registry

import (
	"net/http"
	"time"
)

func init() {
	Register(&Registry{
		ID:          "kiro",
		Name:        "Kiro AI",
		BaseURL:     "https://api.kiro.dev/v1",
		ValidateURL: "https://api.kiro.dev/v1/models",
		ChatPath:    "/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "kiro-1", Name: "Kiro 1", Capabilities: []string{"vision"}},
			{ID: "kiro-1-pro", Name: "Kiro 1 Pro", Capabilities: []string{"vision"}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
