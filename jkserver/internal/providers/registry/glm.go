package registry

import (
	"net/http"
	"time"
)

func init() {
	Register(&Registry{
		ID:          "glm",
		Name:        "GLM (Zhipu AI)",
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4",
		ValidateURL: "https://open.bigmodel.cn/api/paas/v4/models",
		ChatPath:    "/chat/completions",
		AuthHeader:  "Authorization",
		AuthPrefix:  "Bearer",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Models: []Model{
			{ID: "glm-4-plus", Name: "GLM-4 Plus", Capabilities: []string{"vision"}},
			{ID: "glm-4-flash", Name: "GLM-4 Flash", Capabilities: []string{"vision"}},
			{ID: "glm-4-long", Name: "GLM-4 Long", Capabilities: []string{}},
		},
		ClientFn: func(_ string) *http.Client {
			return &http.Client{Timeout: 120 * time.Second}
		},
	})
}
