// Package executors handles HTTP transport for upstream providers.
package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Executor dispatches a prepared request to an upstream provider.
type Executor struct {
	Client     *http.Client
	BaseURL    string
	APIPath    string
	AuthHeader string
	AuthPrefix string
	Headers    map[string]string
	MaxRetries int
}

// NewExecutor creates an Executor with default retry settings.
func NewExecutor(baseURL, apiPath, authHeader, authPrefix string, headers map[string]string) *Executor {
	return &Executor{
		Client:     &http.Client{},
		BaseURL:    baseURL,
		APIPath:    apiPath,
		AuthHeader: authHeader,
		AuthPrefix: authPrefix,
		Headers:    headers,
		MaxRetries: 1,
	}
}

// Execute sends reqBody to the upstream and streams the response to w.
func (e *Executor) Execute(reqBody []byte, bearer string, stream bool, w http.ResponseWriter) error {
	url := e.BaseURL + e.APIPath
	outReq, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("executor: new request: %w", err)
	}
	if bearer != "" {
		outReq.Header.Set(e.AuthHeader, e.AuthPrefix+" "+bearer)
	}
	for k, v := range e.Headers {
		outReq.Header.Set(k, v)
	}
	outReq.Header.Set("Content-Type", "application/json")
	if stream {
		outReq.Header.Set("Accept", "text/event-stream")
	}

	var resp *http.Response
	for attempt := 0; attempt <= e.MaxRetries; attempt++ {
		resp, err = e.Client.Do(outReq)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	if err != nil || resp == nil {
		return fmt.Errorf("executor: %w", err)
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		if isHopByHop(k) {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if stream && strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		_, err = io.Copy(w, resp.Body)
	} else {
		_, err = io.Copy(w, resp.Body)
	}
	return err
}

func isHopByHop(key string) bool {
	switch key {
	case "Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	}
	return false
}

// ParseOpenAIChatBody extracts high-level fields from an OpenAI-format request body.
func ParseOpenAIChatBody(body []byte) (model string, stream bool, msgCount int, err error) {
	var req struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct{} `json:"messages"`
	}
	if err = json.Unmarshal(body, &req); err != nil {
		return
	}
	model = req.Model
	stream = req.Stream
	msgCount = len(req.Messages)
	return
}
