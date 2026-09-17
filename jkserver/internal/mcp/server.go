// Package mcp implements a minimal Model Context Protocol server.
// Transport: JSON-RPC 2.0 over POST /api/mcp, SSE at GET /api/mcp/stream
package mcp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"jkrouter/jkserver/internal/db"
)

// message is a JSON-RPC 2.0 envelope
type message struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      *json.RawMessage  `json:"id,omitempty"`
	Method  string            `json:"method"`
	Params  json.RawMessage   `json:"params,omitempty"`
	Result  json.RawMessage   `json:"result,omitempty"`
	Error   *rpcError         `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolDef describes an MCP tool
type toolDef struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

var tools = []toolDef{
	{
		Name:        "jkrouter_list_models",
		Description: "List all available LLM models from configured providers",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	},
	{
		Name:        "jkrouter_list_providers",
		Description: "List all configured providers with their auth type and base URL",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	},
	{
		Name:        "jkrouter_get_usage",
		Description: "Get aggregated usage stats (requests, tokens in, cost) for the last N days",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"days":{"type":"integer","default":7}}}`),
	},
}

// Server is the MCP protocol handler
type Server struct {
	nextID atomic.Uint64
	db     *db.DB
}

func New(d *db.DB) *Server { return &Server{db: d} }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/mcp":
		s.handleRPC(w, r)
	case "/mcp/stream":
		s.handleSSE(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var msg message
	if err := json.Unmarshal(body, &msg); err != nil {
		s.respondError(w, nil, -32700, "Parse error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch msg.Method {
	case "initialize":
		s.handleInitialize(w, &msg)
	case "tools/list":
		s.handleToolsList(w, &msg)
	case "tools/call":
		s.handleToolsCall(w, &msg)
	case "notifications/initialized":
		w.WriteHeader(202) // accepted, no response
	default:
		s.respondError(w, msg.ID, -32601, "Method not found: "+msg.Method)
	}
}

func (s *Server) handleInitialize(w http.ResponseWriter, msg *message) {
	resp := s.mkResp(msg.ID, map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"serverInfo":      map[string]any{"name": "jkrouter-mcp", "version": "0.1.0"},
	})
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleToolsList(w http.ResponseWriter, msg *message) {
	items := make([]any, len(tools))
	for i, t := range tools {
		items[i] = map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		}
	}
	resp := s.mkResp(msg.ID, map[string]any{"tools": items})
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleToolsCall(w http.ResponseWriter, msg *message) {
	var params struct {
		Name     string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if msg.Params != nil {
		json.Unmarshal(msg.Params, &params)
	}
	var out any
	var err error
	switch params.Name {
	case "jkrouter_list_models":
		out, err = s.listModels()
	case "jkrouter_list_providers":
		out, err = s.listProviders()
	case "jkrouter_get_usage":
		out, err = s.getUsage(params.Arguments)
	default:
		err = fmt.Errorf("unknown tool: %s", params.Name)
	}
	if err != nil {
		s.respondError(w, msg.ID, -32000, err.Error())
		return
	}
	resp := s.mkResp(msg.ID, map[string]any{
		"content": []any{map[string]any{"type": "text", "text": fmt.Sprintf("%v", out)}},
	})
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) mkResp(id *json.RawMessage, result any) any {
	b, _ := json.Marshal(result)
	raw := json.RawMessage(b)
	return message{JSONRPC: "2.0", ID: id, Result: raw}
}

func (s *Server) respondError(w http.ResponseWriter, id *json.RawMessage, code int, msg string) {
	rawID := id
	if rawID == nil {
		rawID = &json.RawMessage{}
	}
	resp := message{JSONRPC: "2.0", ID: rawID, Error: &rpcError{Code: code, Message: msg}}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) listModels() (any, error) {
	rows, err := s.db.Query(`SELECT provider, model, caps FROM provider_models ORDER BY provider, model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		provider, model, caps string
	}
	var out []map[string]string
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.provider, &r.model, &r.caps); err != nil {
			continue
		}
		out = append(out, map[string]string{"provider": r.provider, "model": r.model, "capabilities": r.caps})
	}
	return out, nil
}

func (s *Server) listProviders() (any, error) {
	rows, err := s.db.Query(`SELECT name, label, auth_type, base_url FROM providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		name, label, authType, baseURL string
	}
	var out []map[string]string
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.name, &r.label, &r.authType, &r.baseURL); err != nil {
			continue
		}
		out = append(out, map[string]string{"name": r.name, "label": r.label, "auth_type": r.authType, "base_url": r.baseURL})
	}
	return out, nil
}

func (s *Server) getUsage(args json.RawMessage) (any, error) {
	var p struct{ Days int }
	if len(args) > 0 {
		json.Unmarshal(args, &p)
	}
	if p.Days <= 0 {
		p.Days = 7
	}
	from := time.Now().AddDate(0, 0, -p.Days).Format("2006-01-02")
	var cnt int
	var tokIn, cost sql.NullFloat64
	err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(tok_in),0), COALESCE(SUM(cost_usd),0) FROM usage_log WHERE ts >= ?`,
		from,
	).Scan(&cnt, &tokIn, &cost)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"days":      p.Days,
		"requests":  cnt,
		"tokens_in": tokIn.Float64,
		"cost_usd":  cost.Float64,
	}, nil
}

// SSE handler — emits a minimal roots list event and stays open
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeEv := func(data string) {
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
		flusher.Flush()
	}
	writeEv(`{"jsonrpc":"2.0","method":"roots/list","params":{"roots":[]}}`)
	<-r.Context().Done()
}
