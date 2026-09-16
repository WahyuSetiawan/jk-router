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
	"sync/atomic"
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
	// StreamGuard tracks whether the first SSE data: chunk has been sent.
	// After that point, mid-stream failover is forbidden (PRD §4.1).
	StreamGuard atomic.Bool
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
// Returns StreamStartError if called after a prior Execute already started streaming.
func (e *Executor) Execute(reqBody []byte, bearer string, stream bool, w http.ResponseWriter) error {
	if stream && e.StreamGuard.Load() {
		return fmt.Errorf("executor: mid-stream failover forbidden (point of no return)")
	}
	_, err := e.executeWithResult(reqBody, bearer, stream, w)
	return err
}

// ExecuteWithResult sends the request and returns the HTTP status for caller-side
// account-state decisions (429→cooling_down, 401→disabled).
func (e *Executor) ExecuteWithResult(reqBody []byte, bearer string, stream bool, w http.ResponseWriter) (int, error) {
	if stream && e.StreamGuard.Load() {
		return 0, fmt.Errorf("executor: mid-stream failover forbidden")
	}
	return e.executeWithResult(reqBody, bearer, stream, w)
}

func (e *Executor) executeWithResult(reqBody []byte, bearer string, stream bool, w http.ResponseWriter) (int, error) {
	url := e.BaseURL + e.APIPath
	outReq, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, fmt.Errorf("executor: new request: %w", err)
	}
	if bearer != "" {
		outReq.Header.Set(e.AuthHeader, strings.TrimSpace(e.AuthPrefix)+" "+bearer)
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
		return 0, fmt.Errorf("executor: %w", err)
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
		// Mark stream started before forwarding any bytes so subsequent
		// candidates cannot overwrite a partial SSE pipeline (PRD §4.1).
		e.StreamGuard.Store(true)
		_, err = io.Copy(w, resp.Body)
	} else {
		_, err = io.Copy(w, resp.Body)
	}
	return resp.StatusCode, err
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
