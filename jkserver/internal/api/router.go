// Package api exposes chi routes for OpenAI-compatible API.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/engine"
	"jkrouter/jkserver/internal/oauth"
	"jkrouter/jkserver/internal/providers/registry"
	"jkrouter/jkserver/internal/proxypool"
	"jkrouter/jkserver/internal/translator"
)

// Router wires all HTTP handlers onto a chi.Mux and returns it.
func Router(d *db.DB, transReg *translator.Registry, ul *UsageLogger) chi.Router {
	r := chi.NewRouter()

	oauthStore := oauth.NewStore(5 * time.Minute)

	// Load combo fallback state from DB.
	accountStore := loadAccountStore(d)
	comboStore := loadComboStore(d)

	// Load proxy pool store.
	poolStore := loadProxyPoolStore(d)

	// Build ProviderMeta slice from the registry package.
	providers := buildProviderMetas()

	cfg := &engine.RoutingConfig{
		AccountStore:  accountStore,
		ComboStore:    comboStore,
		TranslatorReg: transReg,
		ProxyPoolStore: poolStore,
		Providers:     providers,
		LogUsage: func(entry engine.RouteLogEntry) {
			if ul == nil {
				return
			}
			ul.LogRequest(UsageEntry{
				RequestID: entry.RequestID,
				Model:     entry.Model,
				Provider:  entry.Provider,
				Status:    entry.Status,
				LatencyMs: entry.LatencyMs,
			})
		},
		SaveStateFunc: func(id int64, state string, strike int, cooledUntil, updatedAt int64) {
			d.EnqueueWriteSync(func(q *db.Queue) {
				q.DB().Exec(`UPDATE accounts SET state=?, strike_count=?, cooled_until=?, updated_at=? WHERE id=?`,
					state, strike, cooledUntil, updatedAt, id)
			})
		},
	}

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","ts":"%s"}`, time.Now().Format(time.RFC3339))
	})

	r.Get("/models", func(w http.ResponseWriter, _ *http.Request) {
		resp := struct {
			Object string             `json:"object"`
			Data   []registry.Model `json:"data"`
		}{Object: "list", Data: listAllModels()}
		json.NewEncoder(w).Encode(resp)
	})

	r.Post("/chat/completions", func(w http.ResponseWriter, req *http.Request) {
		bearer := extractBearer(req)
		if bearer == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"missing or invalid authorization header"}}`, http.StatusUnauthorized)
			return
		}
		stream := strings.Contains(req.Header.Get("Accept"), "text/event-stream") ||
			strings.Contains(req.URL.RawQuery, "stream=true")

		body, err := io.ReadAll(io.LimitReader(req.Body, 10*1024*1024))
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to read body: %v"}`, err), http.StatusBadRequest)
			return
		}
		engine.ExecuteRouting(body, bearer, stream, w, cfg)
	})

	r.Post("/completions", func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(req.Body, 10*1024*1024))
		var comp struct {
			Model     string      `json:"model"`
			Prompt    interface{} `json:"prompt"`
			MaxTokens int         `json:"max_tokens"`
			Stream    bool        `json:"stream"`
		}
		if err := json.Unmarshal(body, &comp); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		chatBody, _ := json.Marshal(map[string]interface{}{
			"model":    comp.Model,
			"messages": []map[string]string{{"role": "user", "content": fmt.Sprint(comp.Prompt)}},
			"max_tokens": comp.MaxTokens,
			"stream":   comp.Stream,
		})
		engine.ExecuteRouting(chatBody, extractBearer(req), comp.Stream, w, cfg)
	})

	// --- Dashboard OAuth endpoints ---
	r.Get("/api/dashboard/oauth/{provider}/start", func(w http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		stateID, _ := oauthStore.Create(provider, req.URL.Query().Get("redirect"))
		redirectURL := buildOAuthAuthURL(provider, stateID, req)
		http.Redirect(w, req, redirectURL, http.StatusTemporaryRedirect)
	})

	r.Get("/api/dashboard/oauth/{provider}/callback", func(w http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		stateID := req.URL.Query().Get("state")
		code := req.URL.Query().Get("code")
		if stateID == "" || code == "" {
			http.Error(w, `{"error":"missing state or code"}`, http.StatusBadRequest)
			return
		}
		st := oauthStore.Consume(stateID)
		if st == nil {
			http.Error(w, `{"error":"invalid or expired state"}`, http.StatusBadRequest)
			return
		}
		token, err := exchangeCode(provider, code, st.Redirect)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"token exchange failed: %v"}`, err), http.StatusBadGateway)
			return
		}
		dbToken, err := json.Marshal(token)
		if err != nil {
			http.Error(w, `{"error":"marshal token"}`, http.StatusInternalServerError)
			return
		}
		encrypted, encErr := db.EncryptSecret(string(dbToken))
		if encErr != nil {
			http.Error(w, `{"error":"encrypt token"}`, http.StatusInternalServerError)
			return
		}
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(
				`INSERT INTO connections (provider_id, name, auth_type, secret_enc, priority, disabled) VALUES (?, ?, 'oauth', ?, 0, 0)`,
				provider, "oauth-"+stateID[:8], encrypted,
			)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"provider":"%s"}`, provider)
	})

	return r
}

// ─────────────────── DB loaders ─────────────────────────────────────────────

func loadAccountStore(d *db.DB) *engine.AccountStore {
	store := engine.NewAccountStore()
	var rows []*struct {
		ID          int64
		ProviderID  string
		Label       string
		AuthType    string
		EncKey      []byte
		ProxyPoolID *int64
		Priority    int
		State       string
		StrikeCount int
		CooledUntil int64
	}
	d.EnqueueWriteSync(func(q *db.Queue) {
		rs, err := q.DB().Query(`
			SELECT id, provider_id, label, auth_type, encrypted_key, proxy_pool_id,
			       priority, COALESCE(state,'active'), COALESCE(strike_count,0), COALESCE(cooled_until,0)
			FROM accounts ORDER BY priority DESC, id`)
		if err != nil {
			return
		}
		defer rs.Close()
		for rs.Next() {
			var r struct {
				ID          int64
				ProviderID  string
				Label       string
				AuthType    string
				EncKey      []byte
				ProxyPoolID *int64
				Priority    int
				State       string
				StrikeCount int
				CooledUntil int64
			}
			if err := rs.Scan(&r.ID, &r.ProviderID, &r.Label, &r.AuthType, &r.EncKey, &r.ProxyPoolID, &r.Priority, &r.State, &r.StrikeCount, &r.CooledUntil); err != nil {
				continue
			}
			rows = append(rows, &r)
		}
	})
	for _, r := range rows {
		keyPlain := ""
		if r.EncKey != nil {
			dec, err := db.DecryptSecret(string(r.EncKey))
			if err == nil {
				keyPlain = dec
			}
		}
		state := engine.StateActive
		if r.State == "cooling_down" && r.CooledUntil > 0 {
			state = engine.StateCoolingDown
		} else if r.State == "disabled" {
			state = engine.StateDisabled
		}
		store.Upsert(&engine.Account{
			ID:          r.ID,
			ProviderID:  r.ProviderID,
			Label:       r.Label,
			AuthType:    r.AuthType,
			APIKey:      keyPlain,
			ProxyPoolID: r.ProxyPoolID,
			Priority:    r.Priority,
			State:       state,
			StrikeCount: r.StrikeCount,
			CooledUntil: time.Unix(r.CooledUntil, 0),
		})
	}
	return store
}

func loadComboStore(d *db.DB) *engine.ComboStore {
	store := engine.NewComboStore()
	type row struct {
		ID        int64
		Name      string
		ModelIDs  string
		Strategy  string
		AccountID int64
	}
	var rows []row
	d.EnqueueWriteSync(func(q *db.Queue) {
		rs, err := q.DB().Query(`
			SELECT c.id, c.name, c.model_ids, c.strategy, ca.account_id
			FROM combos c LEFT JOIN combo_accounts ca ON ca.combo_id = c.id
			ORDER BY c.id, ca.priority`)
		if err != nil {
			return
		}
		defer rs.Close()
		for rs.Next() {
			var r row
			if err := rs.Scan(&r.ID, &r.Name, &r.ModelIDs, &r.Strategy, &r.AccountID); err != nil {
				continue
			}
			rows = append(rows, r)
		}
	})
	// Group by combo ID.
	comboAccounts := make(map[int64][]int64)
	for _, r := range rows {
		comboAccounts[r.ID] = append(comboAccounts[r.ID], r.AccountID)
	}
	// Insert each unique combo.
	seen := make(map[int64]bool)
	for _, r := range rows {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		store.Add(r.Name, r.Strategy, parseModelIDs(r.ModelIDs), comboAccounts[r.ID])
	}
	return store
}

func loadProxyPoolStore(d *db.DB) *proxypool.Store {
	store := proxypool.NewStore()
	d.EnqueueWriteSync(func(q *db.Queue) {
		rs, err := q.DB().Query(`SELECT id, name, proxy_url, no_proxy, strict_proxy, is_active FROM proxy_pools`)
		if err != nil || rs == nil {
			return
		}
		defer rs.Close()
		for rs.Next() {
			var id int64
			var name, proxyURL, noProxy string
			var strict, active bool
			if err := rs.Scan(&id, &name, &proxyURL, &noProxy, &strict, &active); err != nil {
				continue
			}
			store.Add(name, proxyURL, noProxy, strict)
			if p := store.Get(id); p != nil {
				p.IsActive = active
			}
		}
	})
	return store
}

// ─────────────────── provider metadata ───────────────────────────────────────

func buildProviderMetas() []*engine.ProviderMeta {
	regs := registry.GetRegistries()
	out := make([]*engine.ProviderMeta, 0, len(regs))
	for _, reg := range regs {
		out = append(out, &engine.ProviderMeta{
			ID:         reg.ID,
			BaseURL:    reg.BaseURL,
			ChatPath:   reg.ChatPath,
			AuthHeader: reg.AuthHeader,
			AuthPrefix: reg.AuthPrefix,
			Headers:    reg.Headers,
			ClientFn:   reg.ClientFn,
			Format:     regChatFormat(reg.ID),
			ModelIDs:   modelIDs(reg),
		})
	}
	return out
}

func modelIDs(reg *registry.Registry) []string {
	ids := make([]string, len(reg.Models))
	for i, m := range reg.Models {
		ids[i] = m.ID
	}
	return ids
}

func regChatFormat(id string) string {
	switch id {
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini"
	default:
		return "openai"
	}
}

// ─────────────────── helpers ─────────────────────────────────────────────────

func listAllModels() []registry.Model {
	regs := registry.GetRegistries()
	seen := make(map[string]bool)
	var out []registry.Model
	for _, reg := range regs {
		for _, m := range reg.Models {
			if !seen[m.ID] {
				seen[m.ID] = true
				out = append(out, m)
			}
		}
	}
	return out
}

func extractBearer(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func extractBearerFromReq(s string) string { return s }

func buildOAuthAuthURL(provider, stateID string, req *http.Request) string {
	switch provider {
	case "openai":
		return fmt.Sprintf("https://auth.openai.com/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=offline_access",
			req.URL.Query().Get("client_id"),
			url.QueryEscape(req.URL.Query().Get("redirect")),
			stateID)
	case "anthropic":
		return fmt.Sprintf("https://console.anthropic.com/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=%s",
			req.URL.Query().Get("client_id"),
			url.QueryEscape(req.URL.Query().Get("redirect")),
			stateID,
			url.QueryEscape("none"))
	default:
		return fmt.Sprintf("https://%s/oauth/authorize?state=%s&redirect_uri=%s", provider, stateID, url.QueryEscape(req.URL.Query().Get("redirect")))
	}
}

func exchangeCode(provider, code, redirectURL string) (map[string]string, error) {
	return map[string]string{
		"access_token": "mock_token_" + code,
		"token_type":   "bearer",
		"expires_in":   "3600",
	}, nil
}

func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// EnsureBootstrapKey creates a fresh jk_... key if none exists yet.
func EnsureBootstrapKey(d *db.DB) (string, error) {
	var count int
	d.EnqueueWriteSync(func(q *db.Queue) {
		var c int
		q.DB().QueryRow(`SELECT COUNT(*) FROM api_keys`).Scan(&c)
		count = c
	})
	if count > 0 {
		return "jk_bootstrap_key_placeholder", nil
	}
	key := generateKey()
	hash := db.HashAPIKey(key)
	d.EnqueueWriteSync(func(q *db.Queue) {
		q.DB().Exec(`INSERT INTO api_keys (key_hash, label, revoked) VALUES (?, ?, 0)`, hash, "bootstrap")
	})
	return key, nil
}

func generateKey() string {
	b := make([]byte, 24)
	rand.Read(b)
	return "jk_" + hex.EncodeToString(b)
}

func parseModelIDs(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
