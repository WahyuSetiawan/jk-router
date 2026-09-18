package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"jkrouter/jkserver/internal/db"
)

func newTestMCP(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "jkrouter.db")
	d, err := db.Open(tmpDir, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	srv := New(d)
	testSrv := httptest.NewServer(srv)
	t.Cleanup(testSrv.Close)
	return srv, testSrv.URL
}

func TestMCPRPCInitialize(t *testing.T) {
	srv, _ := newTestMCP(t)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "method": "initialize", "id": 1,
	})
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("initialize: %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc 2.0")
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object")
	}
	if result["serverInfo"].(map[string]interface{})["name"] != "jkrouter-mcp" {
		t.Fatalf("wrong server name")
	}
}

func TestMCPToolsList(t *testing.T) {
	srv, _ := newTestMCP(t)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "method": "tools/list", "id": 2,
	})
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("tools/list: %d", rec.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object")
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools array")
	}
	if len(tools) == 0 {
		t.Fatal("expected at least 1 tool")
	}
}

func TestMCPInvalidMethod(t *testing.T) {
	srv, _ := newTestMCP(t)

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "method": "nonexistent", "id": 3,
	})
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestMCPSSEStream(t *testing.T) {
	srv, _ := newTestMCP(t)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/mcp/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(rec, req)
		close(done)
	}()

	// Wait briefly for the initial event to be written
	time.Sleep(50 * time.Millisecond)
	cancel() // close context to unblock the handler

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not return after cancel")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("SSE stream: %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("expected Content-Type text/event-stream, got %s", ct)
	}
	body := rec.Body.String()
	if len(body) == 0 {
		t.Fatal("expected SSE data in response")
	}
}
