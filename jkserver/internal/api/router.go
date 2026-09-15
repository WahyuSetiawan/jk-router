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
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/engine"
	"jkrouter/jkserver/internal/executors"
	"jkrouter/jkserver/internal/oauth"
	"jkrouter/jkserver/internal/providers/registry"
	"jkrouter/jkserver/internal/translator"
)

// Router wires all HTTP handlers onto a chi.Mux and returns it.
func Router(d *db.DB, transReg *translator.Registry, ul *UsageLogger) chi.Router {
	r := chi.NewRouter()

	oauthStore := oauth.NewStore(5 * time.Minute)

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

		start := time.Now()
		reqID := generateRequestID()
		model, _, _, _ := executors.ParseOpenAIChatBody(body)
		if model == "" {
			http.Error(w, `{"error":"model is required"}`, http.StatusBadRequest)
			return
		}

		requiredCaps := engine.DetectRequiredCapabilities(body)
		requiredCapsList := make([]engine.Capability, 0, len(requiredCaps))
		for c := range requiredCaps {
			requiredCapsList = append(requiredCapsList, c)
		}

		regs := registry.GetRegistries()
		type candidate struct {
			reg        *registry.Registry
			model      registry.Model
			matchScore int
		}
		var candidates []candidate
		for _, reg := range regs {
			for _, m := range reg.Models {
				modelCaps := toModelCaps(m)
				score := countMatchedCaps(modelCaps, requiredCapsList)
				candidates = append(candidates, candidate{reg: reg, model: m, matchScore: score})
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].matchScore > candidates[j].matchScore
		})

		var selected *candidate
		for i := range candidates {
			c := &candidates[i]
			if c.matchScore > 0 || len(requiredCapsList) == 0 {
				selected = c
				break
			}
		}
		if selected == nil {
			http.Error(w, fmt.Sprintf(`{"error":"no provider supports model %q"}`, model), http.StatusNotFound)
			return
		}

		client := selected.reg.ClientFn(bearer)
		exe := executors.NewExecutor(
			selected.reg.BaseURL,
			selected.reg.ChatPath,
			selected.reg.AuthHeader,
			selected.reg.AuthPrefix,
			selected.reg.Headers,
		)
		exe.Client = client

		logUsage := func(status string) {
			if ul != nil {
				ul.LogRequest(UsageEntry{
					RequestID: reqID,
					Model:  model,
					Provider: selected.reg.ID,
					Status:    status,
					LatencyMs: int(time.Since(start).Milliseconds()),
				})
			}
		}

		if selected.reg.ID == "openai" {
			err = exe.Execute(body, bearer, stream, w)
			if err != nil {
				logUsage("error")
				http.Error(w, fmt.Sprintf(`{"error":"upstream error: %v"}`, err), http.StatusBadGateway)
				return
			}
			logUsage("success")
			return
		}

		tf := transReg.Get(translator.PairFor(translator.FormatOpenAI, translator.FormatAnthropic))
		if tf == nil {
			logUsage("error")
			http.Error(w, `{"error":"no translator available for this provider"}`, http.StatusBadGateway)
			return
		}
		translatedBody, err := tf.Request(body, translator.FormatOpenAI, translator.FormatAnthropic)
		if err != nil {
			logUsage("error")
			http.Error(w, fmt.Sprintf(`{"error":"translation failed: %v"}`, err), http.StatusBadGateway)
			return
		}
		err = exe.Execute(translatedBody, bearer, stream, w)
		if err != nil {
			logUsage("error")
			http.Error(w, fmt.Sprintf(`{"error":"upstream error: %v"}`, err), http.StatusBadGateway)
			return
		}
		logUsage("success")
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

func toModelCaps(m registry.Model) engine.ModelCaps {
	caps := make([]engine.Capability, len(m.Capabilities))
	for i, c := range m.Capabilities {
		caps[i] = engine.Capability(c)
	}
	return engine.ModelCaps{ID: m.ID, Capabilities: caps}
}

func countMatchedCaps(mc engine.ModelCaps, required []engine.Capability) int {
	count := 0
	for _, c := range required {
		if mc.HasCapability(c) {
			count++
		}
	}
	return count
}

func extractBearer(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

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
