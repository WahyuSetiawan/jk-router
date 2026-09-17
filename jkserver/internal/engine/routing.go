// Package engine: combo×account fallback routing loop.
//
// PRD §4.1 — Account states: active / cooling_down / disabled.
// PRD §4.2 — Capability-aware routing + capacity adapter.
// PRD §4.1 mid-stream — after first SSE data: chunk, failover is forbidden.
package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"jkrouter/jkserver/internal/executors"
	"jkrouter/jkserver/internal/providers/registry"
	"jkrouter/jkserver/internal/rtk"
	"jkrouter/jkserver/internal/proxypool"
	"jkrouter/jkserver/internal/translator"
)

// routeBufferWriter wraps a bytes.Buffer to implement http.ResponseWriter.
type routeBufferWriter struct {
	buf        bytes.Buffer
	header     http.Header
	statusCode int
}

func (b *routeBufferWriter) Header() http.Header            { return b.header }
func (b *routeBufferWriter) Write(p []byte) (int, error)    { return b.buf.Write(p) }
func (b *routeBufferWriter) WriteHeader(code int)           { b.statusCode = code }

func newRouteBufferWriter() *routeBufferWriter {
	return &routeBufferWriter{header: make(http.Header)}
}

// ProviderMeta holds transport-level config for one upstream provider.
type ProviderMeta struct {
	ID           string
	BaseURL      string
	ChatPath     string
	AuthHeader   string
	AuthPrefix   string
	Headers      map[string]string
	ClientFn     func(apiKey string) *http.Client
	Format       string // "openai" | "anthropic" | "gemini"
	ModelIDs     []string // known model IDs for capability ranking
}

// RoutingConfig holds dependencies injected by the API layer.
type RouteLogEntry struct {
	RequestID   string
	Model       string
	Provider    string
	AccountID   int64
	Status      string
	LatencyMs   int
}

type RoutingConfig struct {
	AccountStore    *AccountStore
	ComboStore      *ComboStore
	TranslatorReg   *translator.Registry
	ProxyPoolStore  *proxypool.Store
	CapacityAdapter *CapacityAdapter
	Providers       []*ProviderMeta
	DefaultCooldown time.Duration
	LogUsage        func(entry RouteLogEntry)
	// SaveStateFunc is called after every account state transition (429/401/403).
	// ponytail: single call per request; batch across requests if throughput matters.
	SaveStateFunc func(id int64, state string, strike int, cooledUntil int64, updatedAt int64)
	// RefreshOAuthToken is called before executing a request when the account's
	// OAuth token is expired or about to expire (within tokenRefreshAhead). It must
	// update the in-memory Account.APIKey and Engine.Account.ExpiresAt via the store,
	// and persist the new secret to the DB. Returns true if refresh succeeded.
	RefreshOAuthToken func(id int64) bool
	tokenRefreshAhead time.Duration
	// QuotaStore tracks token usage per account (sliding window). nil = quota disabled.
	QuotaStore *rtk.Saver
	// RTKFilter applies token-saving filters (caveman/ponytail/headroom/system-inject).
	// nil = no RTK filtering.
	RTKFilter *rtk.Registry
}

// DefaultRoutingConfig returns a config with sensible defaults.
func DefaultRoutingConfig() *RoutingConfig {
	return &RoutingConfig{DefaultCooldown: 60 * time.Second}
}

// ExecuteRouting runs the combo×account fallback loop for one chat completion.
func ExecuteRouting(body []byte, bearer string, stream bool, w http.ResponseWriter, cfg *RoutingConfig) {
	if cfg == nil {
		cfg = DefaultRoutingConfig()
	}

	start := time.Now()
	reqID := randomID(8)
	model, _, _, _ := parseModel(body)
	if model == "" {
		http.Error(w, `{"error":"model is required"}`, http.StatusBadRequest)
		return
	}

	requiredCaps := DetectRequiredCapabilities(body)
	reqCapList := capsToList(requiredCaps) // convert map to slice for callers

	candidates := buildCandidates(cfg, reqID, model, reqCapList)
	if len(candidates) == 0 {
		http.Error(w, fmt.Sprintf(`{"error":"no active accounts for model %q"}`, model), http.StatusServiceUnavailable)
		return
	}

	log.Printf("[routing] req=%s model=%s requiredCaps=%v candidates=%d stream=%v", reqID, model, requiredCaps, len(candidates), stream)

	var lastErr error

	for i, c := range candidates {
		reqBody := body
		if cfg.CapacityAdapter != nil && len(reqCapList) > 0 {
			if adapted, newModel, err := cfg.CapacityAdapter.WrapRequest(reqBody, reqCapList); err == nil && newModel != "" {
				reqBody = adapted
				model = newModel
			}
		}

		// Apply RTK filters if enabled (post-translate, pre-exec).
		if cfg.RTKFilter != nil {
			if filtered, applied := cfg.RTKFilter.Apply(reqBody); len(applied) > 0 {
				log.Printf("[rtk] req=%s applied filters: %v", reqID, applied)
				reqBody = filtered
			}
		}

		exe := buildExecutor(c, cfg)
		exe.StreamGuard.Store(false)

		// Refresh OAuth token if expired or about to expire.
		if cfg.RefreshOAuthToken != nil {
			a := cfg.AccountStore.Get(c.accountID)
			if a != nil && a.AuthType == "oauth" && !a.ExpiresAt.IsZero() {
				refreshAhead := cfg.tokenRefreshAhead
				if refreshAhead == 0 {
					refreshAhead = 5 * time.Minute
				}
				if time.Now().Add(refreshAhead).After(a.ExpiresAt) {
					if !cfg.RefreshOAuthToken(c.accountID) {
						log.Printf("[routing] req=%s token refresh failed for account %d, skipping", reqID, c.accountID)
						continue
					}
				}
			}
		}

		// Use a buffer writer for failed attempts so we don't leak partial responses.
		bw := newRouteBufferWriter()
		code, err := exe.ExecuteWithResult(reqBody, bearer, stream, bw)
		if err == nil && code >= 200 && code < 300 {
			// Flush successful response to the actual writer.
			w.WriteHeader(code)
			io.Copy(w, &bw.buf)
			if i > 0 {
				log.Printf("[routing] req=%s fallback ok on %s/%s#%d", reqID, c.meta.ID, model, c.accountID)
			}
			logUsage(cfg, c, reqID, start, "success", nil)
			// Record tokens for quota tracking (Sprint 5 P2).
			if cfg.QuotaStore != nil && c.accountID > 0 {
				inTokens := EstimateTokenCount(reqBody)
				// Estimate output from response body size (rough: 1 token ≈ 4 bytes).
				outTokens := len(bw.buf.Bytes()) / 4
				cfg.QuotaStore.Record(strconv.FormatInt(c.accountID, 10), inTokens, outTokens)
				// Check if over quota — if so, disable account.
				a := cfg.AccountStore.Get(c.accountID)
				if a != nil && a.QuotaLimit > 0 && cfg.QuotaStore.OverQuota(strconv.FormatInt(c.accountID, 10)) {
					log.Printf("[routing] req=%s account %d over quota, disabling", reqID, c.accountID)
					cfg.AccountStore.MarkDisabled(c.accountID)
					if cfg.SaveStateFunc != nil {
						cfg.SaveStateFunc(a.ID, string(a.State), a.StrikeCount, 0, a.UpdatedAtlas.Unix())
					}
				}
			}
			return
		}

		// Mid-stream guard: once first SSE chunk was written by a previous
		// candidate, we CANNOT switch (PRD §4.1).
		if stream && exe.StreamGuard.Load() {
			lastErr = err
			log.Printf("[routing] req=%s mid-stream guard hit: %v", reqID, err)
			emitMidStreamError(w, err)
			logUsage(cfg, c, reqID, start, "mid_stream_error", lastErr)
			return
		}

		lastErr = err

		switch {
		case code == 429 || code == 409:
			cfg.AccountStore.MarkCoolingDown(c.accountID, cfg.DefaultCooldown)
			log.Printf("[routing] req=%s %s#%d -> cooling_down", reqID, c.meta.ID, c.accountID)
			if cfg.SaveStateFunc != nil {
				a := cfg.AccountStore.Get(c.accountID)
				if a != nil {
					cooled := int64(0)
					if !a.CooledUntil.IsZero() {
						cooled = a.CooledUntil.Unix()
					}
					cfg.SaveStateFunc(a.ID, string(a.State), a.StrikeCount, cooled, a.UpdatedAtlas.Unix())
				}
			}
		case code == 401 || code == 403:
			cfg.AccountStore.MarkDisabled(c.accountID)
			log.Printf("[routing] req=%s %s#%d -> disabled (auth)", reqID, c.meta.ID, c.accountID)
			if cfg.SaveStateFunc != nil {
				a := cfg.AccountStore.Get(c.accountID)
				if a != nil {
					cfg.SaveStateFunc(a.ID, string(a.State), a.StrikeCount, 0, a.UpdatedAtlas.Unix())
				}
			}
		default:
			log.Printf("[routing] req=%s %s#%d error: %v", reqID, c.meta.ID, c.accountID, err)
		}
		logUsage(cfg, c, reqID, start, "error", lastErr)
	}

	http.Error(w, fmt.Sprintf(`{"error":"all candidates failed: %v"}`, lastErr), http.StatusBadGateway)
}

// ─────────────────────────── candidate building ──────────────────────────────

type routeCandidate struct {
	meta      *ProviderMeta
	accountID int64
	authKey   string
	poolID    *int64
}

func buildCandidates(cfg *RoutingConfig, reqID string, requestedModel string, requiredCaps []Capability) []*routeCandidate {
	combos := cfg.ComboStore.List()
	accounts := cfg.AccountStore.GetAll()
	var allCands []*routeCandidate

	// Priority path: combos × active accounts.
	if len(combos) > 0 {
		for _, combo := range combos {
			for _, aid := range combo.Accounts {
				a := cfg.AccountStore.Get(aid)
				if a == nil || a.State != StateActive {
					continue
				}
				if cfg.QuotaStore != nil && a.QuotaLimit > 0 && cfg.QuotaStore.OverQuota(strconv.FormatInt(aid, 10)) {
					log.Printf("[routing] req=%s account %d over quota, skipping", reqID, aid)
					continue
				}
				for _, m := range combo.ModelIDs {
					if meta := findMeta(cfg.Providers, m); meta != nil {
						if !candidateSupportsCaps(meta, requiredCaps) {
							continue
						}
						allCands = append(allCands, &routeCandidate{
							meta:      meta,
							accountID: a.ID,
							authKey:   a.APIKey,
							poolID:    a.ProxyPoolID,
						})
					}
				}
			}
		}
	}

	// Fallback: all active accounts whose provider is registered.
	if len(allCands) == 0 {
		for _, a := range accounts {
			if a.State != StateActive {
				continue
			}
			if cfg.QuotaStore != nil && a.QuotaLimit > 0 && cfg.QuotaStore.OverQuota(strconv.FormatInt(a.ID, 10)) {
				continue
			}
			if meta := findMeta(cfg.Providers, a.ProviderID); meta != nil {
				if !candidateSupportsCaps(meta, requiredCaps) {
					continue
				}
				allCands = append(allCands, &routeCandidate{
					meta:      meta,
					accountID: a.ID,
					authKey:   a.APIKey,
					poolID:    a.ProxyPoolID,
				})
			}
		}
	}

	return allCands
}

// candidateSupportsCaps checks whether any model in the provider's registry
// supports all required capabilities.
func candidateSupportsCaps(meta *ProviderMeta, caps []Capability) bool {
	if len(caps) == 0 {
		return true
	}
	r := registry.FindByID(meta.ID)
	if r == nil {
		// No registry info available; fall back to optimistic (allow through).
		return true
	}
	required := make(map[string]bool, len(caps))
	for _, c := range caps {
		required[string(c)] = true
	}
	for _, m := range r.Models {
		hasAll := true
		for capStr := range required {
			found := false
			for _, mc := range m.Capabilities {
				if mc == capStr {
					found = true
					break
				}
			}
			if !found {
				hasAll = false
				break
			}
		}
		if hasAll {
			return true
		}
	}
	return false
}

func findMeta(providers []*ProviderMeta, key string) *ProviderMeta {
	// Exact provider ID match first (avoids picking openrouter when openai has the model).
	for _, p := range providers {
		if p.ID == key {
			return p
		}
	}
	// Then match by model ID.
	for _, p := range providers {
		for _, mid := range p.ModelIDs {
			if mid == key {
				return p
			}
		}
	}
	return nil
}

func buildExecutor(c *routeCandidate, cfg *RoutingConfig) *executors.Executor {
	exe := &executors.Executor{
		Client:     c.meta.ClientFn(c.authKey),
		BaseURL:    c.meta.BaseURL,
		APIPath:    c.meta.ChatPath,
		AuthHeader: c.meta.AuthHeader,
		AuthPrefix: c.meta.AuthPrefix,
		Headers:    c.meta.Headers,
		MaxRetries: 1,
	}
	if cfg.ProxyPoolStore != nil && c.poolID != nil {
		pool := cfg.ProxyPoolStore.Get(*c.poolID)
		if pool != nil && pool.IsActive && pool.ProxyURL != "" {
			if pool.IsRelay() {
				// Relay mode: forward through relay URL with x-relay-* headers.
				exe.RelayURL = pool.ProxyURL
			} else {
				// Proxy mode: use proxy transport.
				orig := exe.Client
				exe.Client = &http.Client{
					Transport: pool.BuildTransport(),
					Timeout:   orig.Timeout,
				}
			}
		}
	}
	return exe
}

// ─────────────────────────── helpers ─────────────────────────────────────────

func logUsage(cfg *RoutingConfig, c *routeCandidate, reqID string, start time.Time, status string, err error) {
	if cfg == nil || cfg.LogUsage == nil || c == nil {
		return
	}
	entry := RouteLogEntry{
		RequestID: reqID,
		Model:     c.meta.ID,
		Provider:  c.meta.ID,
		AccountID: c.accountID,
		Status:    status,
		LatencyMs: int(time.Since(start).Milliseconds()),
	}
	if err != nil {
		entry.Status = "error"
	}
	cfg.LogUsage(entry)
}

func emitMidStreamError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "upstream_error",
			"message": err.Error(),
		},
	})
}

func parseModel(body []byte) (model string, stream bool, msgCount int, err error) {
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

func randomID(n int) string {
	b := make([]byte, n)
	seed := uint64(0x9e3779b97f4a7c15)
	for i := range b {
		seed = seed*6364136223846793005 + 1442695040888963407
		b[i] = byte(seed >> 8)
	}
	return fmt.Sprintf("%x", b)
}

// capsToList converts a Capability map to a slice (preserves order via deterministic scan).
func capsToList(m map[Capability]bool) []Capability {
	out := make([]Capability, 0, len(m))
	for c := range m {
		out = append(out, c)
	}
	return out
}
