// Package api: dashboard API controllers.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"

	"jkrouter/jkserver/internal/db"
)

// AccountShort is a lightweight account representation for JSON responses.
type AccountShort struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	AuthType  string `json:"auth_type"`
	State     string `json:"state"`
	Priority  int    `json:"priority"`
	CreatedAt int64  `json:"created_at"`
}

// DashboardRouter mounts all /api/dashboard/* endpoints.
func DashboardRouter(d *db.DB) chi.Router {
	r := chi.NewRouter()
	r.Get("/providers", ListProvidersHandler(d))
	r.Post("/providers", CreateProviderHandler(d))
	r.Get("/providers/{id}", GetProviderHandler(d))
	r.Put("/providers/{id}", UpdateProviderHandler(d))
	r.Delete("/providers/{id}", DeleteProviderHandler(d))

	r.Get("/connections", ListConnectionsHandler(d))
	r.Post("/connections", CreateConnectionHandler(d))
	r.Patch("/connections/{id}/toggle", ToggleConnectionHandler(d))

	r.Get("/proxy-pools", ListProxyPoolsHandler(d))
	r.Post("/proxy-pools", CreateProxyPoolHandler(d))
	r.Delete("/proxy-pools/{id}", DeleteProxyPoolHandler(d))

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
	r.Get("/config/export", ExportConfigHandler(d))
	r.Post("/config/import", ImportConfigHandler(d))

	return r
}

// --- Providers (wraps accounts grouped by provider) ---

// ListProvidersHandler returns providers with their accounts.
func ListProvidersHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Fetch all providers
		provRows, err := d.Query(`SELECT id, name FROM providers ORDER BY name`)
		if err != nil {
			http.Error(w, `{"error":"query providers"}`, 500)
			return
		}
		type Provider struct {
			ID       string         `json:"id"`
			Name     string         `json:"name"`
			Accounts []AccountShort `json:"accounts"`
		}
		providers := make(map[string]*Provider)
		var provList []string
		for provRows.Next() {
			var id, name string
			if err := provRows.Scan(&id, &name); err != nil {
				provRows.Close()
				http.Error(w, `{"error":"scan provider"}`, 500)
				return
			}
			p := &Provider{ID: id, Name: name}
			providers[id] = p
			provList = append(provList, id)
		}
		provRows.Close()

		// Fetch all accounts
		acctRows, err := d.Query(`
			SELECT a.id, a.provider_id, a.label, a.auth_type, a.state,
			       a.priority, a.created_at, p.name AS provider_name
			FROM accounts a
			LEFT JOIN providers p ON a.provider_id = p.id
			ORDER BY p.name, a.label
		`)
		if err != nil {
			http.Error(w, `{"error":"query accounts"}`, 500)
			return
		}
		defer acctRows.Close()

		for acctRows.Next() {
			var a AccountShort
			var providerID, providerName string
			if err := acctRows.Scan(&a.ID, &providerID, &a.Label, &a.AuthType, &a.State, &a.Priority, &a.CreatedAt, &providerName); err != nil {
				http.Error(w, `{"error":"scan account"}`, 500)
				return
			}
			if p, ok := providers[providerID]; ok {
				p.Accounts = append(p.Accounts, a)
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
		label := req.Label
		if label == "" {
			label = req.Name
		}
		authType := req.AuthType
		if authType == "" {
			authType = "api_key"
		}

		var providerID string
		var accountID int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			// Create provider
			_, err := q.DB().Exec(
				`INSERT INTO providers (id, name) VALUES (?, ?)`,
				req.Name, req.Name,
			)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"create provider: %v"}`, err), 500)
				return
			}
			providerID = req.Name

			// Create account
			var encrypted sql.NullString
			if req.BaseURL != "" {
				enc, err2 := db.EncryptSecret(req.BaseURL)
				if err2 != nil {
					http.Error(w, `{"error":"encrypt"}`, 500)
					return
				}
				encrypted = sql.NullString{String: enc, Valid: true}
			}
			res2, err := q.DB().Exec(
				`INSERT INTO accounts (provider_id, label, auth_type, encrypted_key, state) VALUES (?, ?, ?, ?, 'active')`,
				providerID, label, authType, encrypted,
			)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"create account: %v"}`, err), 500)
				return
			}
			accountID, _ = res2.LastInsertId()
		})

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"%s","account_id":%d,"name":"%s"}`, providerID, accountID, req.Name)
	}
}

// GetProviderHandler returns a provider with its accounts.
func GetProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var row struct {
			ID       string         `json:"id"`
			Name     string         `json:"name"`
			Accounts []AccountShort `json:"accounts"`
		}
		err := d.QueryRow(`SELECT id, name FROM providers WHERE id=?`, id).Scan(&row.ID, &row.Name)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, 404)
			return
		}

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

// UpdateProviderHandler updates provider and/or account info.
func UpdateProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Name     string `json:"name"`
			Label    string `json:"label"`
			BaseURL  string `json:"base_url"`
			AuthType string `json:"auth_type"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		d.EnqueueWriteSync(func(q *db.Queue) {
			if req.Name != "" {
				q.DB().Exec(`UPDATE providers SET name=? WHERE id=?`, req.Name, id)
			}
			// Update the first account for this provider (simple single-account model)
			var acctID int64
			q.DB().QueryRow(`SELECT id FROM accounts WHERE provider_id=? LIMIT 1`, id).Scan(&acctID)
			if acctID > 0 {
				updates := []string{}
				args := []interface{}{}
				if req.Label != "" {
					updates = append(updates, "label=?")
					args = append(args, req.Label)
				}
				if req.AuthType != "" {
					updates = append(updates, "auth_type=?")
					args = append(args, req.AuthType)
				}
				if req.BaseURL != "" {
					enc, err := db.EncryptSecret(req.BaseURL)
					if err == nil {
						updates = append(updates, "encrypted_key=?")
						args = append(args, enc)
					}
				}
				if len(updates) > 0 {
					q.DB().Exec(
						"UPDATE accounts SET "+
							fmt.Sprintf("%s WHERE id=?", joinStrings(updates, ","))+
							"?", append(args, acctID)...,
					)
				}
			}
		})
		w.WriteHeader(http.StatusOK)
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
			       a.priority, a.created_at, p.name AS provider_name
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
			ID           int64  `json:"id"`
			ProviderID   string `json:"provider_id"`
			ProviderName string `json:"provider_name"`
			Name         string `json:"name"`
			AuthType     string `json:"auth_type"`
			State        string `json:"state"`
			Priority     int    `json:"priority"`
			CreatedAt    int64  `json:"created_at"`
		}
		var out []C
		for rows.Next() {
			var c C
			rows.Scan(&c.ID, &c.ProviderID, &c.Name, &c.AuthType, &c.State, &c.Priority, &c.CreatedAt, &c.ProviderName)
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
			ProviderID string `json:"provider_id"`
			Name       string `json:"name"`
			Secret     string `json:"secret"`
			AuthType   string `json:"auth_type"`
			Priority   int    `json:"priority"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ProviderID == "" || req.Name == "" {
			http.Error(w, `{"error":"provider_id and name required"}`, 400)
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
				`INSERT INTO accounts (provider_id, label, auth_type, encrypted_key, priority, state) VALUES (?, ?, ?, ?, ?, 'active')`,
				req.ProviderID, req.Name, authType, encrypted, priority,
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
			SELECT model, SUM(tok_in) as tin, SUM(tok_out) as tout, COUNT(*) as n
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
			rows.Scan(&ci.Model, &ci.TokIn, &ci.TokOut, &ci.N)
			pin, pout, _ := PricingQuery(d, ci.Model)
			ci.Cost = CostForTokens(pin, pout, ci.TokIn, ci.TokOut)
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
		var p95 int64
		row := d.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN status='error' OR status='fallback' THEN 1 ELSE 0 END),0), COALESCE(SUM(tok_in),0) FROM usage_log WHERE ts >= datetime('now', '-7 days')")
		row.Scan(&total, &success, &errors, &tokensIn)
		row2 := d.QueryRow("SELECT COALESCE(AVG(latency_ms),0) FROM usage_log WHERE ts >= datetime('now', '-7 days') AND status='success'")
		row2.Scan(&p95)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"requests":  total,
			"success":   success,
			"errors":    errors,
			"tokens_in": tokensIn,
			"cost":      0.0,
			"p95_ms":    p95,
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
		json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
	}
}

// PutSettingsHandler updates dashboard server settings.
func PutSettingsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Port         string `json:"port"`
			Bind         string `json:"bind"`
			DataDir      string `json:"dataDir"`
			LogBuffer    string `json:"logBuffer"`
			WalInterval  string `json:"walInterval"`
			Cooldown429  string `json:"cooldown429"`
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
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// AuthStatusHandler returns whether dashboard password is configured.
func AuthStatusHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var hash string
		err := d.QueryRow("SELECT value FROM settings_kv WHERE key='dashboard_password_hash'").Scan(&hash)
		firstRun := err == nil
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
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"%d","key":"%s","key_display":"%s"}`, id, masked, masked)
	}
}

func maskKey(s string) string {
	if len(s) < 8 {
		return "••••••••"
	}
	return s[:4] + "••••••••" + s[len(s)-2:]
}
