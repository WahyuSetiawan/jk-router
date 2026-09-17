// Package api exposes chi routes for OpenAI-compatible API.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/engine"
	"jkrouter/jkserver/internal/media"
	"jkrouter/jkserver/internal/oauth"
	"jkrouter/jkserver/internal/providers/registry"
	"jkrouter/jkserver/internal/proxypool"
	"jkrouter/jkserver/internal/rtk"
	"jkrouter/jkserver/internal/mcp"
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

	// Load capacity adapter settings from DB.
	var capAdapterJSON string
	d.QueryRow("SELECT value FROM settings_kv WHERE key='capacity_adapter'").Scan(&capAdapterJSON)
	var capAdapter *engine.CapacityAdapter
	if capAdapterJSON != "" {
		var cfgMap map[string]string
		if json.Unmarshal([]byte(capAdapterJSON), &cfgMap) == nil && len(cfgMap) > 0 {
			converted := make(map[engine.Capability]string, len(cfgMap))
			for k, v := range cfgMap {
				converted[engine.Capability(k)] = v
			}
			capAdapter = engine.NewCapacityAdapter(converted)
		}
	}

	cfg := &engine.RoutingConfig{
		AccountStore:      accountStore,
		ComboStore:        comboStore,
		TranslatorReg:     transReg,
		ProxyPoolStore:    poolStore,
		CapacityAdapter:   capAdapter,
		Providers:         providers,
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
		RefreshOAuthToken: func(id int64) bool {
			var providerID, encKey string
			var expiresAt int64
			d.EnqueueWriteSync(func(q *db.Queue) {
				q.DB().QueryRow(`SELECT provider_id, encrypted_key, expires_at FROM accounts WHERE id=?`, id).Scan(&providerID, &encKey, &expiresAt)
			})
			if providerID == "" || encKey == "" {
				return false
			}
			tokenPlain, err := db.DecryptSecret(encKey)
			if err != nil {
				log.Printf("[oauth] failed to decrypt token for account %d: %v", id, err)
				return false
			}
			var token map[string]string
			if err := json.Unmarshal([]byte(tokenPlain), &token); err != nil {
				log.Printf("[oauth] failed to parse token for account %d: %v", id, err)
				return false
			}
			refreshTokenStr := token["refresh_token"]
			if refreshTokenStr == "" {
				log.Printf("[oauth] no refresh_token for account %d (%s)", id, providerID)
				return false
			}
			newToken, err := refreshToken(providerID, refreshTokenStr)
			if err != nil {
				log.Printf("[oauth] refresh failed for account %d (%s): %v", id, providerID, err)
				return false
			}
			newTokenJSON, _ := json.Marshal(newToken)
			newEnc, _ := db.EncryptSecret(string(newTokenJSON))
			var newExpiresAt int64
			if v, ok := newToken["expires_in"]; ok {
				if n, err := strconv.ParseInt(v, 10, 64); err == nil {
					newExpiresAt = time.Now().Unix() + n
				}
			}
			d.EnqueueWriteSync(func(q *db.Queue) {
				q.DB().Exec(`UPDATE accounts SET encrypted_key=?, expires_at=? WHERE id=?`, newEnc, newExpiresAt, id)
			})
			accountStore.Upsert(&engine.Account{
				APIKey:   string(newTokenJSON), // will be decrypted again on next load
				ExpiresAt: time.Unix(newExpiresAt, 0),
			})
			// Re-decrypt for immediate use
			if decoded, err := db.DecryptSecret(newEnc); err == nil {
				var t map[string]string
				json.Unmarshal([]byte(decoded), &t)
				if tk := t["access_token"]; tk != "" {
					accountStore.Upsert(&engine.Account{APIKey: tk, ExpiresAt: time.Unix(newExpiresAt, 0)})
				}
			}
			log.Printf("[oauth] token refreshed for account %d (%s)", id, providerID)
			return true
		},
		QuotaStore: rtk.New(24*time.Hour, 0), // 24h sliding window; 0 = no hard limit (soft warning only)
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
		localAddr, cleanup, err := StartOAuthLocalServer()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to start oauth callback server: %v"}`, err), http.StatusInternalServerError)
			return
		}
		defer cleanup()
		redirectURL := buildOAuthAuthURL(provider, stateID, localAddr, req)
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
		token, err := exchangeCode(provider, code)
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
		var expiresAt int64
		if v, ok := token["expires_in"]; ok {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				expiresAt = time.Now().Unix() + n
			}
		}
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(
				`INSERT INTO accounts (provider_id, label, auth_type, encrypted_key, state, expires_at) VALUES (?, ?, 'oauth', ?, 'active', ?)`,
				provider, "oauth-"+stateID[:8], encrypted, expiresAt,
			)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"provider":"%s"}`, provider)
	})

	// ── Media routes (TTS / STT / Image / Video) ──────────────────────────
	// Mounted directly (no /v1 prefix) because api.Router is already nested under /v1
	dbMedia := &mediaDBLoader{db: d}
	r.Mount("/", media.Router(dbMedia))

	// ── MCP server (Model Context Protocol) ───────────────────────────────
	r.Mount("/api/mcp", mcp.New(d))

	return r
}

// ─────────────────── media account loader ───────────────────────────────────

type mediaDBLoader struct {
	db *db.DB
}

func (m *mediaDBLoader) LoadAccounts(fn func(*media.Account)) {
	type row struct {
		id         int64
		providerID string
		encKey     []byte
		active     int
	}
	var rows []row
	m.db.EnqueueWriteSync(func(q *db.Queue) {
		rs, err := q.DB().Query(`SELECT id, provider_id, encrypted_key, active FROM media_accounts`)
		if err != nil || rs == nil {
			return
		}
		defer rs.Close()
		for rs.Next() {
			var r row
			if err := rs.Scan(&r.id, &r.providerID, &r.encKey, &r.active); err != nil {
				continue
			}
			rows = append(rows, r)
		}
	})
	for _, r := range rows {
		keyPlain := ""
		if r.encKey != nil {
			if dec, err := db.DecryptSecret(string(r.encKey)); err == nil {
				keyPlain = dec
			}
		}
		fn(&media.Account{
			ID:         r.id,
			ProviderID: r.providerID,
			APIKey:     keyPlain,
			Active:     r.active != 0,
		})
	}
}

// ─────────────────── DB loaders ─────────────────────────────────────────────

func loadAccountStore(d *db.DB) *engine.AccountStore {
	store := engine.NewAccountStore()
	var rows []*struct {
		ID             int64
		ProviderID     string
		Label          string
		AuthType       string
		EncKey         []byte
		ProxyPoolID    *int64
		Priority       int
		State          string
		StrikeCount    int
		CooledUntil    int64
		ExpiresAt      int64
		QuotaLimit     int
		QuotaWindowSec int
		QuotaResetAt   int64
	}
	d.EnqueueWriteSync(func(q *db.Queue) {
		rs, err := q.DB().Query(`
			SELECT id, provider_id, label, auth_type, encrypted_key, proxy_pool_id,
			       priority, COALESCE(state,'active'), COALESCE(strike_count,0), COALESCE(cooled_until,0), COALESCE(expires_at,0),
			       COALESCE(quota_limit,0), COALESCE(quota_window_seconds,86400), COALESCE(quota_reset_at,0)
			FROM accounts ORDER BY priority DESC, id`)
		if err != nil {
			return
		}
		defer rs.Close()
		for rs.Next() {
			var r struct {
				ID             int64
				ProviderID     string
				Label          string
				AuthType       string
				EncKey         []byte
				ProxyPoolID    *int64
				Priority       int
				State          string
				StrikeCount    int
				CooledUntil    int64
				ExpiresAt      int64
				QuotaLimit     int
				QuotaWindowSec int
				QuotaResetAt   int64
			}
			if err := rs.Scan(&r.ID, &r.ProviderID, &r.Label, &r.AuthType, &r.EncKey, &r.ProxyPoolID, &r.Priority, &r.State, &r.StrikeCount, &r.CooledUntil, &r.ExpiresAt, &r.QuotaLimit, &r.QuotaWindowSec, &r.QuotaResetAt); err != nil {
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
			ID:             r.ID,
			ProviderID:     r.ProviderID,
			Label:          r.Label,
			AuthType:       r.AuthType,
			APIKey:         keyPlain,
			ProxyPoolID:    r.ProxyPoolID,
			Priority:       r.Priority,
			State:          state,
			StrikeCount:    r.StrikeCount,
			CooledUntil:    time.Unix(r.CooledUntil, 0),
			ExpiresAt:      time.Unix(r.ExpiresAt, 0),
			QuotaLimit:     r.QuotaLimit,
			QuotaWindowSec: r.QuotaWindowSec,
			QuotaResetAt:   r.QuotaResetAt,
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

func buildOAuthAuthURL(provider, stateID, localAddr string, req *http.Request) string {
	cbURL := fmt.Sprintf("http://%s/api/dashboard/oauth/cb", localAddr)
	switch provider {
	case "openai":
		return fmt.Sprintf("https://auth.openai.com/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=offline_access",
			req.URL.Query().Get("client_id"),
			url.QueryEscape(cbURL),
			stateID)
	case "anthropic":
		return fmt.Sprintf("https://console.anthropic.com/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&state=%s&scope=%s",
			req.URL.Query().Get("client_id"),
			url.QueryEscape(cbURL),
			stateID,
			url.QueryEscape("none"))
	default:
		return fmt.Sprintf("https://%s/oauth/authorize?state=%s&redirect_uri=%s", provider, stateID, url.QueryEscape(cbURL))
	}
}

var oauthSrvMu sync.Mutex
var oauthServerAddr string // last started OAuth callback server address

// StartOAuthLocalServer spins up a temporary HTTP server to receive the OAuth callback.
// Returns the listen address and a cleanup function.
func StartOAuthLocalServer() (string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	addr := ln.Addr().String()
	oauthSrvMu.Lock()
	oauthServerAddr = addr
	oauthSrvMu.Unlock()
	srv := &http.Server{}
	go func() {
		_ = srv.Serve(ln)
	}()
	cleanup := func() { srv.Close() }
	return addr, cleanup, nil
}

func getOAuthCallbackURL(addr string) string {
	return fmt.Sprintf("http://%s/api/dashboard/oauth/cb", addr)
}

func exchangeCode(provider, code string) (map[string]string, error) {
	cbURL := getOAuthCallbackURL(oauthServerAddr)
	if provider == "openai" {
		resp, err := http.PostForm("https://auth.openai.com/oauth/token", url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {cbURL},
			"client_id":     {""},
			"client_secret": {""},
		})
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		return result, nil
	}
	if provider == "kiro" {
		resp, err := http.PostForm("https://api.kiro.dev/oauth/token", url.Values{
			"grant_type":   {"authorization_code"},
			"code":         {code},
			"redirect_uri": {cbURL},
			"client_id":    {"jkrouter"},
		})
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		return result, nil
	}
	return map[string]string{
		"access_token": "mock_token_" + code,
		"token_type":   "bearer",
		"expires_in":   "3600",
	}, nil
}

func refreshToken(provider, refreshToken string) (map[string]string, error) {
	cbURL := getOAuthCallbackURL(oauthServerAddr)
	if provider == "openai" {
		resp, err := http.PostForm("https://auth.openai.com/oauth/token", url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
			"redirect_uri":  {cbURL},
			"client_id":     {""},
			"client_secret": {""},
		})
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		return result, nil
	}
	if provider == "kiro" {
		resp, err := http.PostForm("https://api.kiro.dev/oauth/token", url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
			"redirect_uri":  {cbURL},
			"client_id":     {"jkrouter"},
		})
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		return result, nil
	}
	return nil, fmt.Errorf("provider %s: token refresh not supported", provider)
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
