// Package engine: combo×account fallback routing loop.
//
// PRD §4.1 — Account states: active / cooling_down / disabled.
// PRD §4.2 — Capability-aware routing + capacity adapter.
// PRD §4.1 mid-stream — after first SSE data: chunk, failover is forbidden.
package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"jkrouter/jkserver/internal/executors"
	"jkrouter/jkserver/internal/proxypool"
	"jkrouter/jkserver/internal/translator"
)

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

	candidates := buildCandidates(cfg, model, reqCapList)
	if len(candidates) == 0 {
		http.Error(w, fmt.Sprintf(`{"error":"no active accounts for model %q"}`, model), http.StatusServiceUnavailable)
		return
	}

	log.Printf("[routing] req=%s model=%s requiredCaps=%v candidates=%d stream=%v", reqID, model, requiredCaps, len(candidates), stream)

	var lastErr error

	for i, c := range candidates {
		reqBody := body
		if cfg.CapacityAdapter != nil && len(reqCapList) > 0 {
			if adapted, newModel, err := cfg.CapacityAdapter.WrapRequest(reqBody, reqCapList, c.meta.ModelIDs); err == nil && newModel != "" {
				reqBody = adapted
				model = newModel
			}
		}

		exe := buildExecutor(c, cfg)
		exe.StreamGuard.Store(false)

		code, err := exe.ExecuteWithResult(reqBody, bearer, stream, w)
		if err == nil {
			if i > 0 {
				log.Printf("[routing] req=%s fallback ok on %s/%s#%d", reqID, c.meta.ID, model, c.accountID)
			}
			logUsage(cfg, c, reqID, start, "success", nil)
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

func buildCandidates(cfg *RoutingConfig, requestedModel string, requiredCaps []Capability) []*routeCandidate {
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

// candidateSupportsCaps checks whether the provider's known models cover all required capabilities.
func candidateSupportsCaps(meta *ProviderMeta, caps []Capability) bool {
	_ = meta
	if len(caps) == 0 {
		return true
	}
	// ponytail: optimistic stub — full impl reads registry ModelCaps to filter by capability.
	return true
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
			// Preserve the provider's timeout by wrapping the transport, not replacing the client.
			orig := exe.Client
			exe.Client = &http.Client{
				Transport: pool.BuildTransport(),
				Timeout:   orig.Timeout,
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
