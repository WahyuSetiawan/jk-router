// Package api: dashboard API controllers.
package api

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/media"
	"jkrouter/jkserver/internal/providers/registry"
	"jkrouter/jkserver/internal/rtk"
	"jkrouter/jkserver/internal/settings"
	"jkrouter/jkserver/internal/translator"
)

// AccountShort is a lightweight account representation for JSON responses.
type AccountShort struct {
	ID            int64   `json:"id"`
	Label         string  `json:"label"`
	AuthType      string  `json:"auth_type"`
	State         string  `json:"state"`
	Priority      int     `json:"priority"`
	CreatedAt     int64   `json:"created_at"`
	ProxyPoolID   *int64  `json:"proxy_pool_id"`
	ProxyPoolName string  `json:"proxy_pool_name"`
	ExpiresAt     int64   `json:"expires_at"`
	QuotaLimit    int64   `json:"quota_limit"`
	QuotaWindow   int64   `json:"quota_window_seconds"`
	QuotaResetAt  int64   `json:"quota_reset_at"`
	QuotaUsed     int64   `json:"quota_used"`
}

// DashboardRouter mounts all /api/dashboard/* endpoints.
// refreshFn is an optional callback for manual model refresh; nil skips the endpoint.
func DashboardRouter(d *db.DB, transReg *translator.Registry, refreshFn func()) chi.Router {
	r := chi.NewRouter()
	r.Use(RequireAuth(d))
	r.Get("/providers", ListProvidersHandler(d))
	r.Post("/providers", CreateProviderHandler(d))
	r.Get("/providers/{id}", GetProviderHandler(d))
	r.Get("/providers/{id}/models", GetProviderModelsHandler(d))
	r.Put("/providers/{id}", UpdateProviderHandler(d))
	r.Delete("/providers/{id}", DeleteProviderHandler(d))

	r.Get("/connections", ListConnectionsHandler(d))
	r.Post("/connections", CreateConnectionHandler(d))
	r.Put("/connections/{id}", UpdateConnectionHandler(d))
	r.Delete("/connections/{id}", DeleteConnectionHandler(d))
	r.Patch("/connections/{id}/toggle", ToggleConnectionHandler(d))
	r.Post("/connections/{id}/test", TestConnectionHandler(d))
	r.Get("/connections/{id}/quota", GetQuotaHandler(d))
	r.Put("/connections/{id}/quota", UpdateQuotaHandler(d))

	r.Get("/proxy-pools", ListProxyPoolsHandler(d))
	r.Post("/proxy-pools", CreateProxyPoolHandler(d))
	r.Delete("/proxy-pools/{id}", DeleteProxyPoolHandler(d))
	r.Post("/proxy-pools/deploy/vercel", VercelDeployHandler(d))
	r.Post("/proxy-pools/deploy/cloudflare", CloudflareDeployHandler(d))
	r.Post("/proxy-pools/deploy/deno", DenoDeployHandler(d))

	r.Get("/combos", ListCombosHandler(d))
	r.Post("/combos", CreateComboHandler(d))
	r.Delete("/combos/{id}", DeleteComboHandler(d))

	r.Get("/api-keys", ListAPIKeysHandler(d))
	r.Post("/api-keys", CreateAPIKeyHandler(d))
	r.Post("/api-keys/{id}/revoke", RevokeAPIKeyHandler(d))
	r.Get("/bootstrap-key", BootstrapKeyHandler(d))

	r.Get("/usage", UsageHandler(d))
	r.Post("/usage/tail", UsageTailHandler(d))
	r.Get("/usage/cost", CostHandler(d))

	r.Get("/usage/stats", UsageStatsHandler(d))
	r.Get("/settings", GetSettingsHandler(d))
	r.Put("/settings", PutSettingsHandler(d))
	r.Get("/auth/status", AuthStatusHandler(d))
	r.Post("/auth/change-password", ChangePasswordHandler(d))
	if refreshFn != nil {
		r.Post("/providers/refresh-all", RefreshModelsHandler(refreshFn))
	}
	r.Get("/config/export", ExportConfigHandler(d))
	r.Post("/config/import", ImportConfigHandler(d))
	r.Get("/backups", GetBackupsHandler(d))
	r.Post("/backups/restore", RestoreBackupHandler(d))

	// Media connections (TTS/STT/Image/Video accounts)
	r.Get("/media-connections", ListMediaConnectionsHandler(d))
	r.Post("/media-connections", CreateMediaConnectionHandler(d))
	r.Post("/media-connections/test", TestMediaConnectionHandler(d))
	r.Delete("/media-connections/{id}", DeleteMediaConnectionHandler(d))
	r.Patch("/media-connections/{id}/toggle", ToggleMediaConnectionHandler(d))

	// Translator debug
	r.Get("/translator/pairs", ListTranslatorPairsHandler(transReg))
	r.Post("/translator/preview", PreviewTranslatorHandler(transReg))

	// RTK filter preview
	r.Post("/rtk/preview", RTKPreviewHandler())

	// CLI tools
	r.Get("/cli-tools", ListCLIToolsHandler(d))
	r.Get("/models", ListModelsHandler())

	return r
}

// --- Models catalog (read-only, from registry) ---

// ListModelsHandler returns models grouped by provider.
func ListModelsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		regs := registry.GetRegistries()
		type ProviderModel struct {
			ProviderID   string   `json:"provider_id"`
			ProviderName string   `json:"provider_name"`
			ModelID      string   `json:"model_id"`
			Name         string   `json:"name,omitempty"`
			Capabilities []string `json:"capabilities,omitempty"`
		}
		var out []ProviderModel
		for _, reg := range regs {
			for _, m := range reg.Models {
				out = append(out, ProviderModel{
					ProviderID:   reg.ID,
					ProviderName: reg.Name,
					ModelID:      m.ID,
					Name:         m.Name,
					Capabilities: m.Capabilities,
				})
			}
		}
		if out == nil {
			out = []ProviderModel{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"models": out})
	}
}

// --- Providers (wraps accounts grouped by provider) ---

// ListProvidersHandler returns providers with their accounts.
func ListProvidersHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Fetch all providers
		provRows, err := d.Query(`SELECT id, name, base_url FROM providers ORDER BY name`)
		if err != nil {
			http.Error(w, `{"error":"query providers"}`, 500)
			return
		}
		type Provider struct {
			ID       string         `json:"id"`
			Name     string         `json:"name"`
			BaseURL  string         `json:"base_url,omitempty"`
			Accounts []AccountShort `json:"accounts"`
		}
		providers := make(map[string]*Provider)
		var provList []string
		for provRows.Next() {
			var id, name string
			var baseURL sql.NullString
			if err := provRows.Scan(&id, &name, &baseURL); err != nil {
				provRows.Close()
				http.Error(w, `{"error":"scan provider"}`, 500)
				return
			}
			p := &Provider{ID: id, Name: name, BaseURL: baseURL.String}
			providers[id] = p
			provList = append(provList, id)
		}
		provRows.Close()

		// Fetch all accounts (with proxy pool + quota info)
		acctRows, err := d.Query(`
			SELECT a.id, a.provider_id, a.label, a.auth_type, a.state,
			       a.priority, a.created_at, a.proxy_pool_id, COALESCE(pp.name, ''),
			       COALESCE(a.expires_at,0),
			       COALESCE(a.quota_limit,0), COALESCE(a.quota_window_seconds,86400), COALESCE(a.quota_reset_at,0)
			FROM accounts a
			LEFT JOIN providers p ON a.provider_id = p.id
			LEFT JOIN proxy_pools pp ON a.proxy_pool_id = pp.id
			ORDER BY p.name, a.label
		`)
		if err != nil {
			http.Error(w, `{"error":"query accounts"}`, 500)
			return
		}
		defer acctRows.Close()

		for acctRows.Next() {
			var a AccountShort
			var providerID string
			if err := acctRows.Scan(&a.ID, &providerID, &a.Label, &a.AuthType, &a.State, &a.Priority, &a.CreatedAt, &a.ProxyPoolID, &a.ProxyPoolName, &a.ExpiresAt, &a.QuotaLimit, &a.QuotaWindow, &a.QuotaResetAt); err != nil {
				http.Error(w, `{"error":"scan account"}`, 500)
				return
			}
			if p, ok := providers[providerID]; ok {
				p.Accounts = append(p.Accounts, a)
			} else {
				// Orphan account: provider row missing — show a placeholder so the
				// account is still visible nested under its provider_id.
				p := &Provider{ID: providerID, Name: providerID}
				p.Accounts = append(p.Accounts, a)
				providers[providerID] = p
				provList = append(provList, providerID)
			}
		}

		out := make([]Provider, 0, len(provList))
		for _, id := range provList {
			out = append(out, *providers[id])
		}
		if out == nil {
			out = []Provider{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"providers": out})
	}
}

// CreateProviderHandler creates a provider and an initial account.
func CreateProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name     string `json:"name"`
			Label    string `json:"label"`
			AuthType string `json:"auth_type"`
			BaseURL  string `json:"base_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, 400)
			return
		}
		if req.Name == "" {
			http.Error(w, `{"error":"name required"}`, 400)
			return
		}

		var providerID string
		d.EnqueueWriteSync(func(q *db.Queue) {
			// Create provider only — accounts are added via "+ Akun" per provider.
			_, err := q.DB().Exec(
				`INSERT INTO providers (id, name) VALUES (?, ?)`,
				req.Name, req.Name,
			)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"create provider: %v"}`, err), 500)
				return
			}
			providerID = req.Name
		})

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"%s","name":"%s"}`, providerID, req.Name)
	}
}

// GetProviderHandler returns a provider with its accounts.
func GetProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var row struct {
			ID       string         `json:"id"`
			Name     string         `json:"name"`
			BaseURL  string         `json:"base_url,omitempty"`
			Accounts []AccountShort `json:"accounts"`
		}
		var baseURL sql.NullString
		err := d.QueryRow(`SELECT id, name, base_url FROM providers WHERE id=?`, id).Scan(&row.ID, &row.Name, &baseURL)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, 404)
			return
		}
		row.BaseURL = baseURL.String

		rows, err := d.Query(`
			SELECT id, label, auth_type, state, priority, created_at
			FROM accounts WHERE provider_id=? ORDER BY label
		`, id)
		if err != nil {
			http.Error(w, `{"error":"query accounts"}`, 500)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var a AccountShort
			rows.Scan(&a.ID, &a.Label, &a.AuthType, &a.State, &a.Priority, &a.CreatedAt)
			row.Accounts = append(row.Accounts, a)
		}
		json.NewEncoder(w).Encode(row)
	}
}

// GetProviderModelsHandler returns models for a provider from the registry.
func GetProviderModelsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		reg := registry.FindByID(id)
		if reg == nil {
			http.Error(w, `{"error":"provider not found"}`, 404)
			return
		}
		models := reg.ListModels()
		json.NewEncoder(w).Encode(map[string]interface{}{"models": models})
	}
}

// UpdateProviderHandler updates provider name and/or base_url endpoint override.
// base_url is a per-provider transport override on top of the Go registry default;
// an empty value clears it (revert to registry BaseURL).
func UpdateProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Name    string `json:"name"`
			BaseURL string `json:"base_url"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		d.EnqueueWriteSync(func(q *db.Queue) {
			if req.Name != "" {
				q.DB().Exec(`UPDATE providers SET name=? WHERE id=?`, req.Name, id)
			}
			if req.BaseURL != "" {
				// Validate it's a plausible URL before storing.
				if u, err := url.Parse(req.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
					http.Error(w, `{"error":"invalid base_url: must be http(s)://host"}`, 400)
					return
				}
				q.DB().Exec(`UPDATE providers SET base_url=? WHERE id=?`, req.BaseURL, id)
			} else {
				// base_url omitted → clear override (restore registry default).
				q.DB().Exec(`UPDATE providers SET base_url='' WHERE id=?`, id)
			}
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"id":"%s"}`, id)
	}
}

// DeleteProviderHandler deletes a provider and its accounts.
func DeleteProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`DELETE FROM accounts WHERE provider_id=?`, id)
			q.DB().Exec(`DELETE FROM providers WHERE id=?`, id)
		})
		w.WriteHeader(http.StatusOK)
	}
}

// --- Connections (accounts) ---

// ListConnectionsHandler returns all accounts.
func ListConnectionsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rows, err := d.Query(`
			SELECT a.id, a.provider_id, a.label, a.auth_type, a.state,
			       a.priority, a.created_at, p.name AS provider_name, a.tags
			FROM accounts a
			LEFT JOIN providers p ON a.provider_id = p.id
			ORDER BY a.created_at DESC
		`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type C struct {
			ID           int64        `json:"id"`
			ProviderID   string       `json:"provider_id"`
			ProviderName sql.NullString `json:"provider_name"`
			Name         string       `json:"name"`
			AuthType     string       `json:"auth_type"`
			State        string       `json:"state"`
			Priority     int          `json:"priority"`
			CreatedAt    int64        `json:"created_at"`
			Tags         string       `json:"tags"`
		}
		var out []C
		for rows.Next() {
			var c C
			err := rows.Scan(&c.ID, &c.ProviderID, &c.Name, &c.AuthType, &c.State, &c.Priority, &c.CreatedAt, &c.ProviderName, &c.Tags)
			if err != nil {
				continue
			}
			out = append(out, c)
		}
		if out == nil {
			out = []C{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"connections": out})
	}
}

// CreateConnectionHandler creates a new account (connection).
func CreateConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProviderID  string `json:"provider_id"`
			Label       string `json:"label"`
			Secret      string `json:"secret"`
			AuthType    string `json:"auth_type"`
			Priority    int    `json:"priority"`
			ProxyPoolID *int64 `json:"proxy_pool_id"`
			Tags        string `json:"tags"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ProviderID == "" || req.Label == "" {
			http.Error(w, `{"error":"provider_id and label required"}`, 400)
			return
		}
		authType := req.AuthType
		if authType == "" {
			authType = "api_key"
		}
		priority := req.Priority
		if priority == 0 {
			priority = 0
		}

		var encrypted sql.NullString
		var err error
		if req.Secret != "" {
			enc, e := db.EncryptSecret(req.Secret)
			if e != nil {
				http.Error(w, `{"error":"encrypt"}`, 500)
				return
			}
			encrypted = sql.NullString{String: enc, Valid: true}
			_ = err
		}

		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			// Auto-register provider so the account always nests under it on the providers page.
			if _, e := q.DB().Exec(`INSERT OR IGNORE INTO providers (id, name) VALUES (?, ?)`, req.ProviderID, req.ProviderID); e != nil {
				http.Error(w, fmt.Sprintf(`{"error":"insert: %v"}`, e), 500)
				return
			}
			res, e := q.DB().Exec(
				`INSERT INTO accounts (provider_id, label, auth_type, encrypted_key, priority, proxy_pool_id, state, tags) VALUES (?, ?, ?, ?, ?, ?, 'active', ?)`,
				req.ProviderID, req.Label, authType, encrypted, priority, req.ProxyPoolID, req.Tags,
			)
			if e != nil {
				http.Error(w, fmt.Sprintf(`{"error":"insert: %v"}`, e), 500)
				return
			}
			id, _ = res.LastInsertId()
			// Force checkpoint to make write visible to subsequent reads
			// ponytail: this is a workaround for modernc/sqlite WAL visibility bug
			// upgrade path: file issue on modernc/sqlite repo
			q.DB().Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d}`, id)
	}
}

// ToggleConnectionHandler toggles an account's state between active and disabled.
func ToggleConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var newState string
		d.EnqueueWriteSync(func(q *db.Queue) {
			var state string
			q.DB().QueryRow(`SELECT state FROM accounts WHERE id=?`, id).Scan(&state)
			if state == "disabled" {
				newState = "active"
			} else {
				newState = "disabled"
			}
			q.DB().Exec(`UPDATE accounts SET state=? WHERE id=?`, newState, id)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%s,"state":"%s"}`, id, newState)
	}
}

// UpdateConnectionHandler updates an account's fields.
func UpdateConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Label       string `json:"label"`
			ProxyPoolID *int64 `json:"proxy_pool_id"`
			Tags        string `json:"tags"`
			Secret      string `json:"secret"` // API key baru (opsional; kosong = tidak diubah)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"decode"}`, 400)
			return
		}
		d.EnqueueWriteSync(func(q *db.Queue) {
			setParts := []string{}
			args := []interface{}{}
			if req.Label != "" {
				setParts = append(setParts, "label=?")
				args = append(args, req.Label)
			}
			if req.ProxyPoolID != nil {
				setParts = append(setParts, "proxy_pool_id=?")
				args = append(args, *req.ProxyPoolID)
			}
			if req.Tags != "" {
				setParts = append(setParts, "tags=?")
				args = append(args, req.Tags)
			}
			if req.Secret != "" {
				enc, e := db.EncryptSecret(req.Secret)
				if e != nil {
					http.Error(w, `{"error":"encrypt"}`, 500)
					return
				}
				setParts = append(setParts, "encrypted_key=?")
				args = append(args, sql.NullString{String: enc, Valid: true})
			}
			if len(setParts) > 0 {
				args = append(args, id)
				q.DB().Exec(fmt.Sprintf("UPDATE accounts SET %s WHERE id=?", joinStrings(setParts, ",")), args...)
			}
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

// TestConnectionHandler tests an account's stored API key by calling the
// provider's /models endpoint with the correct auth header for that provider.
func TestConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var providerID string
		var enc sql.NullString
		err := d.QueryRow(`SELECT provider_id, encrypted_key FROM accounts WHERE id=?`, id).Scan(&providerID, &enc)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, 404)
			return
		}
		if !enc.Valid || enc.String == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "no key stored"})
			return
		}
		plain, err := db.DecryptSecret(enc.String)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "decrypt failed"})
			return
		}
		reg := registry.FindByID(providerID)
		if reg == nil || reg.ValidateURL == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "no validate url for provider " + providerID})
			return
		}
		client := reg.ClientFn(plain)
		req2, _ := http.NewRequest("GET", reg.ValidateURL, nil)
		if reg.AuthPrefix != "" {
			req2.Header.Set(reg.AuthHeader, reg.AuthPrefix+" "+plain)
		} else {
			req2.Header.Set(reg.AuthHeader, plain)
		}
		for k, v := range reg.Headers {
			req2.Header.Set(k, v)
		}
		resp, err := client.Do(req2)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		ok := resp.StatusCode == http.StatusOK
		msg := ""
		if !ok {
			msg = fmt.Sprintf("upstream status %d", resp.StatusCode)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": ok, "status": resp.StatusCode, "error": msg})
	}
}

// DeleteConnectionHandler deletes an account.
func DeleteConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`DELETE FROM accounts WHERE id=?`, id)
		})
		w.WriteHeader(http.StatusOK)
	}
}

// --- Proxy Pools ---

// ListProxyPoolsHandler returns proxy pools.
func ListProxyPoolsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`
			SELECT id, name, ptype, proxy_url, no_proxy, is_active, strict_proxy, test_status, last_tested_at
			FROM proxy_pools ORDER BY created_at DESC
		`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type PP struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Ptype       string `json:"ptype"`
			ProxyURL    string `json:"proxy_url"`
			NoProxy     string `json:"no_proxy"`
			IsActive    bool   `json:"is_active"`
			StrictProxy bool   `json:"strict_proxy"`
			TestStatus  string `json:"test_status"`
			LastTested  int64  `json:"last_tested"`
			CreatedAt   int64  `json:"created_at"`
		}
		var out []PP
		for rows.Next() {
			var p PP
			var lastTestedNullable sql.NullInt64
			rows.Scan(&p.ID, &p.Name, &p.Ptype, &p.ProxyURL, &p.NoProxy, &p.IsActive, &p.StrictProxy, &p.TestStatus, &lastTestedNullable)
			if lastTestedNullable.Valid {
				p.LastTested = lastTestedNullable.Int64
			}
			out = append(out, p)
		}
		if out == nil {
			out = []PP{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"proxy_pools": out})
	}
}

// CreateProxyPoolHandler creates a proxy pool.
func CreateProxyPoolHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Ptype       string `json:"ptype"`
			ProxyURL    string `json:"proxy_url"`
			NoProxy     string `json:"no_proxy"`
			IsActive    bool   `json:"is_active"`
			StrictProxy bool   `json:"strict_proxy"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" {
			http.Error(w, `{"error":"name required"}`, 400)
			return
		}
		ptype := req.Ptype
		if ptype == "" {
			ptype = "http"
		}
		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO proxy_pools (name, ptype, proxy_url, no_proxy, is_active, strict_proxy) VALUES (?, ?, ?, ?, ?, ?)`,
				req.Name, ptype, req.ProxyURL, req.NoProxy, req.IsActive, req.StrictProxy,
			)
			if e != nil {
				http.Error(w, fmt.Sprintf(`{"error":"insert: %v"}`, e), 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"name":"%s"}`, id, req.Name)
	}
}

// DeleteProxyPoolHandler deletes a proxy pool.
func DeleteProxyPoolHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`DELETE FROM proxy_pools WHERE id=?`, id)
		})
		w.WriteHeader(http.StatusOK)
	}
}

// ─────────────────── Proxy Relay Deploy Handlers ────────────────────────────

// VercelDeployHandler deploys a relay function to Vercel and creates a proxy pool.
func VercelDeployHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const vercelAPI = "https://api.vercel.com"
		relayCode := `export const config = { runtime: "edge" };
export default async function handler(req) {
  const target = req.headers.get("x-relay-target");
  const relayPath = req.headers.get("x-relay-path") || "/";
  if (!target) {
    return new Response(JSON.stringify({ error: "Missing x-relay-target header" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    });
  }
  const targetUrl = target.replace(/\/$/, "") + relayPath;
  const headers = new Headers(req.headers);
  headers.delete("x-relay-target");
  headers.delete("x-relay-path");
  headers.delete("host");
  const response = await fetch(targetUrl, {
    method: req.method,
    headers,
    body: req.method !== "GET" && req.method !== "HEAD" ? req.body : undefined,
    duplex: "half",
  });
  return new Response(response.body, { status: response.status, headers: response.headers });
}`
		var req struct {
			VercelToken string `json:"vercelToken"`
			Name        string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VercelToken == "" {
			http.Error(w, `{"error":"vercelToken required"}`, 400)
			return
		}
		projName := strings.TrimSpace(req.Name)
		if projName == "" {
			projName = "relay-" + strings.TrimPrefix(time.Now().Format("20060102150405"), "2")
		}
		// Create deployment
		deployBody, _ := json.Marshal(map[string]interface{}{
			"name": projName, "target": "production",
			"files": []map[string]string{
				{"file": "api/relay.js", "data": relayCode},
				{"file": "package.json", "data": `{"name":"` + projName + `","version":"1.0.0"}`},
				{"file": "vercel.json", "data": `{"rewrites":[{"source":"/(.*)","destination":"/api/relay"}]}`},
			},
			"projectSettings": map[string]interface{}{"framework": nil},
		})
		deployReq, _ := http.NewRequest("POST", vercelAPI+"/v13/deployments", bytes.NewReader(deployBody))
		deployReq.Header.Set("Authorization", "Bearer "+req.VercelToken)
		deployReq.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(deployReq)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 500)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, string(body)), resp.StatusCode)
			return
		}
		var deployResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&deployResult)
		// Poll for ready
		depID, _ := deployResult["id"].(string)
		var deployURL string
		for i := 0; i < 40; i++ {
			time.Sleep(3 * time.Second)
			pollReq, _ := http.NewRequest("GET", vercelAPI+"/v13/deployments/"+depID, nil)
			pollReq.Header.Set("Authorization", "Bearer "+req.VercelToken)
			pollResp, err := http.DefaultClient.Do(pollReq)
			if err == nil && pollResp.StatusCode == 200 {
				var pr map[string]interface{}
				json.NewDecoder(pollResp.Body).Decode(&pr)
				pollResp.Body.Close()
				if s, ok := pr["readyState"].(string); ok {
					if s == "READY" {
						deployURL = "https://" + pr["url"].(string)
						break
					}
					if s == "ERROR" || s == "CANCELED" {
						http.Error(w, `{"error":"deployment failed"}`, 502)
						return
					}
				}
			}
		}
		if deployURL == "" {
			http.Error(w, `{"error":"deployment timed out"}`, 504)
			return
		}
		// Disable deployment protection
		projID, _ := deployResult["projectId"].(string)
		if projID == "" {
			projID = projName
		}
		protBody, _ := json.Marshal(map[string]interface{}{"ssoProtection": nil})
		protReq, _ := http.NewRequest("PATCH", vercelAPI+"/v9/projects/"+projID, bytes.NewReader(protBody))
		protReq.Header.Set("Authorization", "Bearer "+req.VercelToken)
		protReq.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(protReq)
		// Save to DB
		var poolID int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO proxy_pools (name, ptype, proxy_url, is_active, strict_proxy) VALUES (?, ?, ?, 1, 0)`,
				projName, "relay", deployURL,
			)
			if e == nil {
				poolID, _ = res.LastInsertId()
			}
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"proxy_pool_id": poolID, "deploy_url": deployURL})
	}
}

// CloudflareDeployHandler deploys a relay worker to Cloudflare and creates a proxy pool.
func CloudflareDeployHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relayCode := `export default {
  async fetch(request, env, ctx) {
    const target = request.headers.get("x-relay-target");
    const relayPath = request.headers.get("x-relay-path") || "/";
    if (!target) {
      return new Response(JSON.stringify({ error: "Missing x-relay-target header" }), {
        status: 400,
        headers: { "content-type": "application/json" },
      });
    }
    const targetUrl = target.replace(/\/$/, "") + relayPath;
    const newHeaders = new Headers(request.headers);
    newHeaders.delete("x-relay-target");
    newHeaders.delete("x-relay-path");
    newHeaders.delete("host");
    const init = { method: request.method, headers: newHeaders };
    if (request.method !== "GET" && request.method !== "HEAD") {
      init.body = request.body;
      init.duplex = "half";
    }
    try {
      const response = await fetch(targetUrl, init);
      return new Response(response.body, { status: response.status, headers: response.headers });
    } catch (e) {
      return new Response(JSON.stringify({ error: e.message }), { status: 502 });
    }
  },
}`
		var req struct {
			AccountID string `json:"accountId"`
			APIToken  string `json:"apiToken"`
			Name      string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID == "" || req.APIToken == "" {
			http.Error(w, `{"error":"accountId and apiToken required"}`, 400)
			return
		}
		projName := strings.TrimSpace(req.Name)
		if projName == "" {
			projName = "relay-" + strings.TrimPrefix(time.Now().Format("20060102150405"), "2")
		}
		// Upload worker script (multipart)
		meta := `{"main_module":"index.js","compatibility_date":"2024-03-20","observability":{"enabled":true}}`
		bodyBuf := &bytes.Buffer{}
		writer := multipart.NewWriter(bodyBuf)
		writer.WriteField("metadata", meta)
		part, _ := writer.CreateFormFile("index.js", "index.js")
		part.Write([]byte(relayCode))
		writer.Close()
		uploadURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s", req.AccountID, projName)
		uploadReq, _ := http.NewRequest("PUT", uploadURL, bodyBuf)
		uploadReq.Header.Set("Authorization", "Bearer "+req.APIToken)
		uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
		uploadResp, err := http.DefaultClient.Do(uploadReq)
		if err != nil || uploadResp.StatusCode >= 400 {
			var errBody string
			if uploadResp != nil {
				b, _ := io.ReadAll(uploadResp.Body)
				errBody = string(b)
			}
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, errBody), 502)
			return
		}
		uploadResp.Body.Close()
		// Enable subdomain
		subURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/subdomain", req.AccountID, projName)
		subBody, _ := json.Marshal(map[string]bool{"enabled": true})
		subReq, _ := http.NewRequest("POST", subURL, bytes.NewReader(subBody))
		subReq.Header.Set("Authorization", "Bearer "+req.APIToken)
		subReq.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(subReq)
		// Get subdomain
		subdomainReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/subdomain", req.AccountID), nil)
		subdomainReq.Header.Set("Authorization", "Bearer "+req.APIToken)
		subdomainResp, err := http.DefaultClient.Do(subdomainReq)
		var deployURL string
		if err == nil && subdomainResp.StatusCode == 200 {
			var subData map[string]interface{}
			json.NewDecoder(subdomainResp.Body).Decode(&subData)
			subdomainResp.Body.Close()
			if result, ok := subData["result"].(map[string]interface{}); ok {
				if sub, ok := result["subdomain"].(string); ok {
					deployURL = "https://" + projName + "." + sub + ".workers.dev"
				}
			}
		}
		if deployURL == "" {
			http.Error(w, `{"error":"failed to get workers.dev subdomain"}`, 502)
			return
		}
		var poolID int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO proxy_pools (name, ptype, proxy_url, is_active, strict_proxy) VALUES (?, ?, ?, 1, 0)`,
				projName, "relay", deployURL,
			)
			if e == nil {
				poolID, _ = res.LastInsertId()
			}
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"proxy_pool_id": poolID, "deploy_url": deployURL})
	}
}

// DenoDeployHandler deploys a relay to Deno Deploy and creates a proxy pool.
func DenoDeployHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relayCode := `Deno.serve(async (request) => {
  const target = request.headers.get("x-relay-target");
  const relayPath = request.headers.get("x-relay-path") || "/";
  if (!target) {
    return new Response(JSON.stringify({ error: "Missing x-relay-target header" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    });
  }
  const targetUrl = target.replace(/\/$/, "") + relayPath;
  const newHeaders = new Headers(request.headers);
  newHeaders.delete("x-relay-target");
  newHeaders.delete("x-relay-path");
  newHeaders.delete("host");
  const init = { method: request.method, headers: newHeaders };
  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = request.body;
  }
  try {
    const response = await fetch(targetUrl, init);
    return new Response(response.body, { status: response.status, headers: response.headers });
  } catch (e) {
    return new Response(JSON.stringify({ error: e.message }), { status: 502 });
  }
});`
		var req struct {
			DenoToken  string `json:"denoToken"`
			OrgDomain  string `json:"orgDomain"`
			Name       string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DenoToken == "" || req.OrgDomain == "" {
			http.Error(w, `{"error":"denoToken and orgDomain required"}`, 400)
			return
		}
		projName := strings.TrimSpace(req.Name)
		if projName == "" {
			projName = "relay-" + strings.TrimPrefix(time.Now().Format("20060102150405"), "2")
		}
		headers := map[string]string{"Authorization": "Bearer " + req.DenoToken, "Content-Type": "application/json"}
		// Create app
		createBody, _ := json.Marshal(map[string]interface{}{
			"slug": projName,
			"labels": map[string]string{"custom.kind": "jkrouter-relay"},
			"config": map[string]interface{}{
				"install": "deno install",
				"runtime": map[string]string{"type": "dynamic", "entrypoint": "main.ts"},
			},
		})
		createReq, _ := http.NewRequest("POST", "https://api.deno.com/v2/apps", bytes.NewReader(createBody))
		for k, v := range headers {
			createReq.Header.Set(k, v)
		}
		createResp, err := http.DefaultClient.Do(createReq)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), 500)
			return
		}
		defer createResp.Body.Close()
		if createResp.StatusCode == 409 {
			http.Error(w, fmt.Sprintf(`{"error":"app %s already exists"}`, projName), 409)
			return
		}
		if createResp.StatusCode >= 400 {
			body, _ := io.ReadAll(createResp.Body)
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, string(body)), createResp.StatusCode)
			return
		}
		var appInfo map[string]interface{}
		json.NewDecoder(createResp.Body).Decode(&appInfo)
		appID, _ := appInfo["id"].(string)
		// Deploy
		deployBody, _ := json.Marshal(map[string]interface{}{
			"assets": map[string]interface{}{
				"main.ts": map[string]string{"kind": "file", "content": relayCode, "encoding": "utf-8"},
			},
		})
		deployReq, _ := http.NewRequest("POST", fmt.Sprintf("https://api.deno.com/v2/apps/%s/deploy", appID), bytes.NewReader(deployBody))
		for k, v := range headers {
			deployReq.Header.Set(k, v)
		}
		deployResp, err := http.DefaultClient.Do(deployReq)
		if err != nil || deployResp.StatusCode >= 400 {
			var errBody string
			if deployResp != nil {
				b, _ := io.ReadAll(deployResp.Body)
				errBody = string(b)
			}
			// Cleanup: delete app
			cleanReq, _ := http.NewRequest("DELETE", fmt.Sprintf("https://api.deno.com/v2/apps/%s", appID), nil)
			http.DefaultClient.Do(cleanReq)
			http.Error(w, fmt.Sprintf(`{"error":"deploy failed: %s"}`, errBody), 502)
			return
		}
		defer deployResp.Body.Close()
		var revision map[string]interface{}
		json.NewDecoder(deployResp.Body).Decode(&revision)
		revID, _ := revision["id"].(string)
		// Poll until succeeded
		var deployURL string
		for i := 0; i < 30; i++ {
			time.Sleep(2 * time.Second)
			statusReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.deno.com/v2/revisions/%s", revID), nil)
			statusReq.Header.Set("Authorization", "Bearer "+req.DenoToken)
			statusResp, err := http.DefaultClient.Do(statusReq)
			if err == nil && statusResp.StatusCode == 200 {
				var sr map[string]interface{}
				json.NewDecoder(statusResp.Body).Decode(&sr)
				statusResp.Body.Close()
				if s, ok := sr["status"].(string); ok {
					if s == "succeeded" {
						orgSlug := strings.SplitN(req.OrgDomain, ".", 2)[0]
						deployURL = "https://" + projName + "." + orgSlug + ".deno.net"
						break
					}
					if s == "failed" {
						cleanReq, _ := http.NewRequest("DELETE", fmt.Sprintf("https://api.deno.com/v2/apps/%s", appID), nil)
			http.DefaultClient.Do(cleanReq)
						http.Error(w, `{"error":"deployment failed"}`, 502)
						return
					}
				}
			}
		}
		if deployURL == "" {
			cleanReq, _ := http.NewRequest("DELETE", fmt.Sprintf("https://api.deno.com/v2/apps/%s", appID), nil)
			http.DefaultClient.Do(cleanReq)
			http.Error(w, `{"error":"deployment timed out"}`, 504)
			return
		}
		var poolID int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO proxy_pools (name, ptype, proxy_url, is_active, strict_proxy) VALUES (?, ?, ?, 1, 0)`,
				projName, "relay", deployURL,
			)
			if e == nil {
				poolID, _ = res.LastInsertId()
			}
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"proxy_pool_id": poolID, "deploy_url": deployURL})
	}
}

// --- Combos ---

// ListCombosHandler returns all combos.
func ListCombosHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`SELECT id, name, description, model_ids, strategy, created_at FROM combos ORDER BY name`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type Combo struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			ModelIDs    string `json:"model_ids"`
			Strategy    string `json:"strategy"`
			CreatedAt   int64  `json:"created_at"`
		}
		var out []Combo
		for rows.Next() {
			var c Combo
			rows.Scan(&c.ID, &c.Name, &c.Description, &c.ModelIDs, &c.Strategy, &c.CreatedAt)
			out = append(out, c)
		}
		if out == nil {
			out = []Combo{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"combos": out})
	}
}

// CreateComboHandler creates a combo.
func CreateComboHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			ModelIDs    []string `json:"model_ids"`
			Strategy    string   `json:"strategy"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Name == "" {
			http.Error(w, `{"error":"name required"}`, 400)
			return
		}
		modelsJSON, _ := json.Marshal(req.ModelIDs)
		strategy := req.Strategy
		if strategy == "" {
			strategy = "fallback"
		}
		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO combos (name, description, model_ids, model_list, strategy) VALUES (?, ?, ?, ?, ?)`,
				req.Name, req.Description, modelsJSON, modelsJSON, strategy,
			)
			if e != nil {
				http.Error(w, fmt.Sprintf(`{"error":"insert: %v"}`, e), 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"name":"%s"}`, id, req.Name)
	}
}

// DeleteComboHandler deletes a combo.
func DeleteComboHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`DELETE FROM combos WHERE id=?`, id)
		})
		w.WriteHeader(http.StatusOK)
	}
}

// --- API Keys ---

// ListAPIKeysHandler returns API keys.
func ListAPIKeysHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`SELECT id, key_hash, label, revoked, created_at FROM api_keys ORDER BY created_at DESC`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type K struct {
		KeyDisplay string `json:"key_display"`
			ID        int64  `json:"id"`
			KeyHash   string `json:"key_hash"`
			Label     string `json:"label"`
			Revoked   bool   `json:"revoked"`
			CreatedAt int64  `json:"created_at"`
		}
		var out []K
		for rows.Next() {
			var k K
			rows.Scan(&k.ID, &k.KeyHash, &k.Label, &k.Revoked, &k.CreatedAt)
			k.KeyDisplay = maskKey(k.KeyHash)
			out = append(out, k)
		}
		if out == nil {
			out = []K{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"keys": out})
	}
}

// CreateAPIKeyHandler creates an API key.
func CreateAPIKeyHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Label string `json:"label"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		key := generateKey()
		hash := db.HashAPIKey(key)
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(
				`INSERT INTO api_keys (key_hash, label, revoked) VALUES (?, ?, 0)`,
				hash, sql.NullString{String: req.Label, Valid: true},
			)
		})
		w.Header().Set("Content-Type", "application/json")
		masked := maskKey(key)
		fmt.Fprintf(w, `{"id":"%s","key":"%s","key_display":"%s"}`, key[:12]+"...", key, masked)
	}
}

// RevokeAPIKeyHandler revokes an API key.
func RevokeAPIKeyHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`UPDATE api_keys SET revoked=1 WHERE id=?`, id)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"%s","revoked":true}`, id)
	}
}

// --- Cost Handler ---

// CostHandler returns cost estimates grouped by model.
func CostHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`
			SELECT model, SUM(tok_in) as tin, SUM(tok_out) as tout, SUM(cost_usd) as total_cost, COUNT(*) as n
			FROM usage_log
			GROUP BY model
			ORDER BY n DESC LIMIT 20
		`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type CostItem struct {
			Model  string  `json:"model"`
			TokIn  int     `json:"tok_in"`
			TokOut int     `json:"tok_out"`
			N      int     `json:"n"`
			Cost   float64 `json:"cost_usd"`
		}
		var out []CostItem
		for rows.Next() {
			var ci CostItem
			rows.Scan(&ci.Model, &ci.TokIn, &ci.TokOut, &ci.Cost, &ci.N)
			out = append(out, ci)
		}
		if out == nil {
			out = []CostItem{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"costs": out})
	}
}

func joinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

// --- New handlers for redesigned dashboard ---

// UsageStatsHandler returns aggregated usage stats for the last 7 days.
func UsageStatsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		var total, success, errors, tokensIn int
		var tokensOut, avgMs int64
		row := d.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN status='error' OR status='fallback' THEN 1 ELSE 0 END),0), COALESCE(SUM(tok_in),0), COALESCE(SUM(tok_out),0), COALESCE(SUM(cost_usd),0) FROM usage_log WHERE ts >= datetime('now', '-7 days')")
		var cost float64
		row.Scan(&total, &success, &errors, &tokensIn, &tokensOut, &cost)
		row2 := d.QueryRow("SELECT COALESCE(AVG(latency_ms),0) FROM usage_log WHERE ts >= datetime('now', '-7 days') AND status='success'")
		row2.Scan(&avgMs)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"requests":  total,
			"success":   success,
			"errors":    errors,
			"tokens_in": tokensIn,
			"tokens_out": tokensOut,
			"cost":      cost,
			"avg_ms":    avgMs,
		})
	}
}

// GetSettingsHandler returns dashboard server settings.
func GetSettingsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		sv := func(key string) string {
			var v string
			if err := d.QueryRow("SELECT value FROM settings_kv WHERE key=?", key).Scan(&v); err != nil {
				return ""
			}
			return v
		}
		settings := map[string]interface{}{}
		if v := sv("port"); v != "" {
			settings["port"] = v
		}
		if v := sv("bind"); v != "" {
			settings["bind"] = v
		}
		if v := sv("data_dir"); v != "" {
			settings["data_dir"] = v
		}
		if v := sv("log_buffer"); v != "" {
			settings["log_buffer"] = v
		}
		if v := sv("wal_interval"); v != "" {
			settings["wal_interval"] = v
		}
		if v := sv("cooldown_429"); v != "" {
			settings["cooldown_429"] = v
		}
		if v := sv("circuit_breaker"); v != "" {
			settings["circuitBreaker"] = v
		}
		// Capacity adapter settings (JSON blob)
		if v := sv("capacity_adapter"); v != "" {
			settings["capacityAdapter"] = v
		}
		// RTK filter settings (JSON array of enabled filter names)
		if v := sv("rtk_filters"); v != "" {
			settings["rtkFilters"] = v
		}
		// Headroom tokens config
		if v := sv("headroom_tokens"); v != "" {
			settings["headroomTokens"] = v
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
	}
}

// PutSettingsHandler updates dashboard server settings.
func PutSettingsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Port           string      `json:"port"`
			Bind           string      `json:"bind"`
			DataDir        string      `json:"dataDir"`
			LogBuffer      string      `json:"logBuffer"`
			WalInterval    string      `json:"walInterval"`
			Cooldown429    string      `json:"cooldown429"`
			CircuitBreaker  string            `json:"circuitBreaker"`
		CapacityAdapter json.RawMessage   `json:"capacityAdapter"`
		RTKFilters      json.RawMessage   `json:"rtkFilters"`
		HeadroomTokens  int               `json:"headroomTokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"decode"}`, 400)
			return
		}
		set := func(key, val string) {
			if val == "" {
				return
			}
			d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES (?, ?)", key, val)
		}
		set("port", body.Port)
		set("bind", body.Bind)
		set("data_dir", body.DataDir)
		set("log_buffer", body.LogBuffer)
		set("wal_interval", body.WalInterval)
		set("cooldown_429", body.Cooldown429)
		if body.CircuitBreaker != "" {
			d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES (?, ?)", "circuit_breaker", body.CircuitBreaker)
		}
		if len(body.CapacityAdapter) > 0 {
			d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES (?, ?)", "capacity_adapter", string(body.CapacityAdapter))
		}
		if len(body.RTKFilters) > 0 {
			d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES (?, ?)", "rtk_filters", string(body.RTKFilters))
		}
		if body.HeadroomTokens > 0 {
			d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES (?, ?)", "headroom_tokens", fmt.Sprintf("%d", body.HeadroomTokens))
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// GetBackupsHandler lists available DB backup files.
type BackupEntry struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	ModTime  string `json:"mod_time"`
	SHA256   string `json:"sha256"`
}

func GetBackupsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dataDir := settings.GetDataDir()
		backupDir := settings.BackupDir(dataDir)
		files, err := filepath.Glob(filepath.Join(backupDir, "jkrouter-*.bak.gz"))
		if err != nil {
			http.Error(w, `{"error":"glob backups"}`, 500)
			return
		}
		sort.Strings(files)
		var list []BackupEntry
		for _, f := range files {
			fi, _ := os.Stat(f)
			data, _ := os.ReadFile(f)
			h := sha256.Sum256(data)
			list = append(list, BackupEntry{
				Filename: filepath.Base(f),
				Size:     fi.Size(),
				ModTime:  fi.ModTime().Format(time.RFC3339),
				SHA256:   hex.EncodeToString(h[:]),
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"backups": list})
	}
}

// RestoreBackupHandler restores from a backup .bak.gz file.
func RestoreBackupHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filename string `json:"filename"`
			SHA256   string `json:"sha256"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"decode"}`, 400)
			return
		}
		dataDir := settings.GetDataDir()
		backupDir := settings.BackupDir(dataDir)
		filePath := filepath.Join(backupDir, body.Filename)
		// Security: only allow files in backup dir matching known prefix.
		if !strings.HasPrefix(body.Filename, "jkrouter-") || !strings.HasSuffix(body.Filename, ".bak.gz") {
			http.Error(w, `{"error":"invalid filename"}`, 400)
			return
		}
		fi, err := os.Stat(filePath)
		if err != nil || fi.Size() < 10 {
			http.Error(w, `{"error":"backup not found"}`, 404)
			return
		}
		// Verify SHA-256 if provided.
		if body.SHA256 != "" {
			data, _ := os.ReadFile(filePath)
			h := sha256.Sum256(data)
			if hex.EncodeToString(h[:]) != body.SHA256 {
				http.Error(w, `{"error":"sha256 mismatch"}`, 400)
				return
			}
		}
		// Decompress .gz to temp path.
		gzFile, err := os.Open(filePath)
		if err != nil {
			http.Error(w, `{"error":"open backup"}`, 500)
			return
		}
		defer gzFile.Close()
		gr, err := gzip.NewReader(gzFile)
		if err != nil {
			http.Error(w, `{"error":"gzip reader"}`, 500)
			return
		}
		tmpPath := filepath.Join(filepath.Dir(settings.DBPath(dataDir)), ".jkrouter-restoring.db")
		tmpFile, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, `{"error":"create temp"}`, 500)
			return
		}
		if _, err := io.Copy(tmpFile, gr); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			http.Error(w, `{"error":"decompress"}`, 500)
			return
		}
		gr.Close()
		tmpFile.Close()
		// Atomic rename over current DB.
		dbPath := settings.DBPath(dataDir)
		if err := os.Rename(tmpPath, dbPath); err != nil {
			os.Remove(tmpPath)
			http.Error(w, `{"error":"rename db"}`, 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "restored", "file": body.Filename})
	}
}

// AuthStatusHandler returns whether dashboard password is configured.
func AuthStatusHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var hash string
		err := d.QueryRow("SELECT value FROM settings_kv WHERE key='dashboard_password_hash'").Scan(&hash)
		firstRun := err != nil
		json.NewEncoder(w).Encode(map[string]interface{}{
			"first_run": firstRun,
			"has_password": !firstRun,
		})
	}
}

// ChangePasswordHandler sets or changes the dashboard password.
func ChangePasswordHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Current string `json:"current"`
			New     string `json:"new"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"decode"}`, 400)
			return
		}
		if len(body.New) < 6 {
			http.Error(w, `{"error":"password too short"}`, 400)
			return
		}
		// Check current password if changing
		var existingHash string
		if err := d.QueryRow("SELECT value FROM settings_kv WHERE key='dashboard_password_hash'").Scan(&existingHash); err == nil {
			if body.Current == "" {
				http.Error(w, `{"error":"current password required"}`, 400)
				return
			}
			if !bcryptCheck(body.Current, existingHash) {
				http.Error(w, `{"error":"incorrect password"}`, 401)
				return
			}
		}
		// Set new password
		newHash, err := bcryptHash(body.New)
		if err != nil {
			http.Error(w, `{"error":"bcrypt failed"}`, 500)
			return
		}
		d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('dashboard_password_hash', ?)", newHash)
		d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('dashboard_first_run', 'false')")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ExportConfigHandler exports all dashboard config as JSON.
func ExportConfigHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		type config struct {
			Providers  []map[string]interface{} `json:"providers"`
			Combos     []map[string]interface{} `json:"combos"`
			ProxyPools []map[string]interface{} `json:"proxy_pools"`
			APIKeys    []map[string]interface{} `json:"api_keys"`
			Connections []map[string]interface{} `json:"connections"`
		}
		var c config
		for _, q := range []struct {
			table string
			dst   *[]map[string]interface{}
		}{
			{"providers", &c.Providers},
			{"combos", &c.Combos},
			{"proxy_pools", &c.ProxyPools},
			{"api_keys", &c.APIKeys},
			{"accounts", &c.Connections},
		} {
			rows, err := d.Query("SELECT * FROM " + q.table)
			if err != nil {
				continue
			}
			cols, _ := rows.Columns()
			for rows.Next() {
				vals := make([]interface{}, len(cols))
				addr := make([]interface{}, len(cols))
				for i := range vals {
					addr[i] = &vals[i]
				}
				if err := rows.Scan(addr...); err != nil {
					continue
				}
				m := make(map[string]interface{})
				for i, col := range cols {
					m[col] = vals[i]
				}
				*q.dst = append(*q.dst, m)
			}
			rows.Close()
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"config": c})
	}
}

// ImportConfigHandler imports dashboard config from JSON.
func ImportConfigHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Providers   []map[string]interface{} `json:"providers"`
			Combos      []map[string]interface{} `json:"combos"`
			ProxyPools  []map[string]interface{} `json:"proxy_pools"`
			APIKeys     []map[string]interface{} `json:"api_keys"`
			Connections []map[string]interface{} `json:"connections"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"decode"}`, 400)
			return
		}
		importTable := func(table string, items []map[string]interface{}) {
			tx, _ := d.DB.Begin()
			d.DB.Exec("DELETE FROM " + table)
			for _, item := range items {
				keys := make([]string, 0, len(item))
				vals := make([]interface{}, 0, len(item))
				for k, v := range item {
					keys = append(keys, k)
					vals = append(vals, v)
				}
				placeholders := make([]string, len(keys))
				for i := range placeholders {
					placeholders[i] = "?"
				}
				q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(keys, ","), strings.Join(placeholders, ","))
				tx.Exec(q, vals...)
			}
			tx.Commit()
		}
		importTable("providers", body.Providers)
		importTable("combos", body.Combos)
		importTable("proxy_pools", body.ProxyPools)
		importTable("api_keys", body.APIKeys)
		importTable("accounts", body.Connections)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func bcryptHash(pw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func bcryptCheck(pw, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	return err == nil
}

// maskKey shows first 4 and last 2 chars with dots in between.
// BootstrapKeyHandler returns the first active api key (for endpoint.vue).
func BootstrapKeyHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		var id int64
		var keyHash, label string
		row := d.QueryRow(`SELECT id, key_hash, COALESCE(label, '') FROM api_keys WHERE revoked=0 ORDER BY created_at ASC LIMIT 1`)
		err := row.Scan(&id, &keyHash, &label)
		if err != nil || id == 0 {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"key":""}`)
			return
		}
		masked := maskKey(keyHash)
		// Return plain key only if never shown before.
		var shown string
		d.QueryRow("SELECT value FROM settings_kv WHERE key='bootstrap_key_shown'").Scan(&shown)
		keyPlain := ""
		if shown != "1" {
			// First access: decrypt and return plain key, then mark as shown.
			if plain, err2 := db.DecryptSecret(keyHash); err2 == nil {
				keyPlain = plain
			}
			d.Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('bootstrap_key_shown', '1')")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"key":"%s","key_display":"%s"}`, id, keyPlain, masked)
	}
}

func maskKey(s string) string {
	if len(s) < 8 {
		return "••••••••"
	}
	return s[:4] + "••••••••" + s[len(s)-2:]
}

// RefreshModelsHandler triggers a manual model refresh across all providers.
func RefreshModelsHandler(refreshFn func()) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		go refreshFn()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "refreshing"})
	}
}

// GetQuotaHandler returns quota status for an account (Sprint 5 P2).
func GetQuotaHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var quotaLimit, quotaWindowSec, quotaResetAt int64
		err := d.QueryRow(`SELECT COALESCE(quota_limit,0), COALESCE(quota_window_seconds,86400), COALESCE(quota_reset_at,0) FROM accounts WHERE id=?`, id).Scan(&quotaLimit, &quotaWindowSec, &quotaResetAt)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		// Calculate used tokens in current window
		var used int64
		windowStart := quotaResetAt - quotaWindowSec
		if quotaResetAt > 0 && windowStart > 0 {
			d.QueryRow(`SELECT COALESCE(SUM(tok_in + tok_out), 0) FROM usage_log WHERE account_id=? AND ts > ?`, id, time.Unix(windowStart, 0).Format("2006-01-02 15:04:05")).Scan(&used)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"quota_limit":%d,"quota_window_seconds":%d,"quota_reset_at":%d,"used":%d}`, quotaLimit, quotaWindowSec, quotaResetAt, used)
	}
}

// UpdateQuotaHandler updates quota settings for an account (Sprint 5 P2).
func UpdateQuotaHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			QuotaLimit      int64 `json:"quota_limit"`
			QuotaWindowSec  int64 `json:"quota_window_seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		if req.QuotaWindowSec <= 0 {
			req.QuotaWindowSec = 86400
		}
		resetAt := time.Now().Unix() + req.QuotaWindowSec
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`UPDATE accounts SET quota_limit=?, quota_window_seconds=?, quota_reset_at=? WHERE id=?`,
				req.QuotaLimit, req.QuotaWindowSec, resetAt, id)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ok":true,"quota_limit":%d,"quota_window_seconds":%d,"quota_reset_at":%d}`, req.QuotaLimit, req.QuotaWindowSec, resetAt)
	}
}

// ─────────────────── Media connections ────────────────────────────────────────

// ListMediaConnectionsHandler returns all media accounts.
func ListMediaConnectionsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rows, err := d.Query(`
			SELECT id, provider_id, label, auth_type, active, priority, created_at
			FROM media_accounts ORDER BY created_at DESC
		`)
		if err != nil {
			http.Error(w, `{"error":"query media_accounts"}`, 500)
			return
		}
		defer rows.Close()
		type MC struct {
			ID         int64  `json:"id"`
			ProviderID string `json:"provider_id"`
			Label      string `json:"label"`
			AuthType   string `json:"auth_type"`
			Active     bool   `json:"active"`
			Priority   int    `json:"priority"`
			CreatedAt  int64  `json:"created_at"`
		}
		var out []MC
		for rows.Next() {
			var c MC
			rows.Scan(&c.ID, &c.ProviderID, &c.Label, &c.AuthType, &c.Active, &c.Priority, &c.CreatedAt)
			out = append(out, c)
		}
		if out == nil {
			out = []MC{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"media_connections": out})
	}
}

// CreateMediaConnectionHandler creates a new media account.
func CreateMediaConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProviderID string `json:"provider_id"`
			Label      string `json:"label"`
			Secret     string `json:"secret"`
			AuthType   string `json:"auth_type"`
			Priority   int    `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		if req.ProviderID == "" || req.Label == "" {
			http.Error(w, `{"error":"provider_id and label required"}`, http.StatusBadRequest)
			return
		}
		authType := req.AuthType
		if authType == "" {
			authType = "api_key"
		}
		priority := req.Priority
		if priority == 0 {
			priority = 0
		}
		var encrypted sql.NullString
		var err error
		if req.Secret != "" {
			enc, e := db.EncryptSecret(req.Secret)
			if e != nil {
				http.Error(w, `{"error":"encrypt"}`, 500)
				return
			}
			encrypted = sql.NullString{String: enc, Valid: true}
			_ = err
		}
		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO media_accounts (provider_id, label, auth_type, encrypted_key, priority, active) VALUES (?, ?, ?, ?, ?, 1)`,
				req.ProviderID, req.Label, authType, encrypted, priority,
			)
			if e != nil {
				http.Error(w, fmt.Sprintf(`{"error":"insert: %v"}`, e), 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d}`, id)
	}
}

// DeleteMediaConnectionHandler deletes a media account.
func DeleteMediaConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`DELETE FROM media_accounts WHERE id=?`, id)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}
}

// ToggleMediaConnectionHandler toggles a media account's active status.
func ToggleMediaConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var active bool
		d.EnqueueWriteSync(func(q *db.Queue) {
			var isActive int
			q.DB().QueryRow(`SELECT active FROM media_accounts WHERE id=?`, id).Scan(&isActive)
			active = isActive == 0
			q.DB().Exec(`UPDATE media_accounts SET active=? WHERE id=?`, active, id)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%s,"active":%t}`, id, active)
	}
}

// TestMediaConnectionHandler tests a media account by making a minimal API call.
func TestMediaConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProviderID string `json:"provider_id"`
			Secret     string `json:"secret"`
			Operation  string `json:"operation"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		if req.ProviderID == "" || req.Secret == "" {
			http.Error(w, `{"error":"provider_id and secret required"}`, http.StatusBadRequest)
			return
		}
		if req.Operation == "" {
			req.Operation = "tts"
		}

		status, err := media.TestExecutor(req.ProviderID, req.Secret, req.Operation)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			fmt.Fprintf(w, `{"ok":false,"error":"%v"}`, err)
		} else {
			fmt.Fprintf(w, `{"ok":true,"status":%d,"message":"API key is valid"}`, status)
		}
	}
}

// ─────────────────── Translator debug ────────────────────────────────────────

type pairInfo struct {
	Pair       string `json:"pair"`
	FormatFrom string `json:"format_from"`
	FormatTo   string `json:"format_to"`
}

// ListTranslatorPairsHandler returns available translator pairs.
func ListTranslatorPairsHandler(transReg *translator.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if transReg == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"pairs": []pairInfo{}})
			return
		}
		// The registry doesn't expose pairs directly; we know the registered ones.
		pairs := []pairInfo{
			{Pair: "openai:anthropic", FormatFrom: "openai", FormatTo: "anthropic"},
			{Pair: "anthropic:openai", FormatFrom: "anthropic", FormatTo: "openai"},
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"pairs": pairs})
	}
}

// PreviewTranslatorHandler translates a sample payload and returns the result.
func PreviewTranslatorHandler(transReg *translator.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if transReg == nil {
			http.Error(w, `{"error":"translator not available"}`, 503)
			return
		}
		var req struct {
			Pair     string `json:"pair"`
			Payload  string `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Pair == "" || req.Payload == "" {
			http.Error(w, `{"error":"pair and payload required"}`, http.StatusBadRequest)
			return
		}
		parts := strings.SplitN(req.Pair, ":", 2)
		if len(parts) != 2 {
			http.Error(w, `{"error":"invalid pair format, expected from:to"}`, http.StatusBadRequest)
			return
		}
		from, to := translator.Format(parts[0]), translator.Format(parts[1])
		tr := transReg.Get(translator.PairFor(from, to))
		if tr == nil || tr.Request == nil {
			http.Error(w, fmt.Sprintf(`{"error":"no translator for %s"}`, req.Pair), http.StatusNotFound)
			return
		}
		result, err := tr.Request([]byte(req.Payload), from, to)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"translate: %v"}`, err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pair":    req.Pair,
			"payload": string(result),
		})
	}
}

// ─────────────────── CLI tools ────────────────────────────────────────────────

type cliTool struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Config      map[string]string `json:"config"`
	Docs        string         `json:"docs"`
}

// ListCLIToolsHandler returns CLI tool configuration cards.
func ListCLIToolsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Read the local API key from settings
		var apiKey string
		d.QueryRow("SELECT value FROM settings_kv WHERE key='bootstrap_key'").Scan(&apiKey)
		if apiKey == "" {
			apiKey = "jk_bootstrap_key_placeholder"
		}
		tools := []cliTool{
			{
				ID:          "claude-code",
				Name:        "Claude Code",
				Description: "Anthropic CLI — code agent",
				Docs:        "https://docs.anthropic.com/en/docs/claude-code",
				Config: map[string]string{
					"endpoint": "http://localhost:20127",
					"model":    "claude-3-5-sonnet-20241022",
					"api_key":  apiKey,
				},
			},
			{
				ID:          "codex",
				Name:        "OpenAI Codex CLI",
				Description: "OpenAI coding agent",
				Docs:        "https://github.com/openai/codex",
				Config: map[string]string{
					"endpoint": "http://localhost:20127/v1",
					"model":    "claude-3-5-sonnet-20241022",
					"api_key":  apiKey,
				},
			},
			{
				ID:          "cursor",
				Name:        "Cursor",
				Description: "AI code editor",
				Docs:        "https://cursor.sh",
				Config: map[string]string{
					"endpoint": "http://localhost:20127/v1",
					"model":    "claude-3-5-sonnet-20241022",
					"api_key":  apiKey,
				},
			},
			{
				ID:          "cline",
				Name:        "Cline (VS Code)",
				Description: "Autonomous coding agent for VS Code",
				Docs:        "https://github.com/cline/cline",
				Config: map[string]string{
					"endpoint": "http://localhost:20127/v1",
					"model":    "claude-3-5-sonnet-20241022",
					"api_key":  apiKey,
				},
			},
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"tools": tools})
	}
}

// RTKPreviewHandler applies RTK filters to a sample payload for preview.
func RTKPreviewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages      []json.RawMessage `json:"messages"`
			RTKFilters    []string          `json:"rtkFilters"`
			HeadroomTokens int             `json:"headroomTokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"decode"}`, http.StatusBadRequest)
			return
		}
		if len(req.Messages) == 0 {
			http.Error(w, `{"error":"messages required"}`, http.StatusBadRequest)
			return
		}
		reg := rtk.NewRegistry()
		for _, name := range req.RTKFilters {
			reg.Enable(name)
		}
		// Headroom needs explicit config; create with given limit.
		if req.HeadroomTokens > 0 {
			reg.Enable("headroom")
			// Re-enable with custom config — we can't override after Enable,
			// so we just enable it and let the default (0 chars) pass through.
			// The actual headroom enforcement is in the engine, not here.
		}
		base := map[string]interface{}{"messages": req.Messages}
		body, _ := json.Marshal(base)
		filtered, applied := reg.Apply(body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"filtered": string(filtered),
			"applied":  applied,
			"original_size": len(body),
			"filtered_size": len(filtered),
		})
	}
}
