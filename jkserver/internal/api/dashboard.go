// Package api: dashboard API controllers.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	

	"jkrouter/jkserver/internal/db"
)

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

	r.Get("/usage", UsageHandler(d))
	r.Post("/usage/tail", UsageTailHandler(d))
	r.Get("/usage/cost", CostHandler(d))

	return r
}

// --- Providers ---

func ListProvidersHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`SELECT id, label, auth_type, base_url, created_at, updated_at FROM providers ORDER BY label`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type P struct {
			ID        int64  `json:"id"`
			Label     string `json:"label"`
			AuthType  string `json:"auth_type"`
			BaseURL   string `json:"base_url"`
			CreatedAt int64  `json:"created_at"`
			UpdatedAt int64  `json:"updated_at"`
		}
		var out []P
		for rows.Next() {
			var p P
			rows.Scan(&p.ID, &p.Label, &p.AuthType, &p.BaseURL, &p.CreatedAt, &p.UpdatedAt)
			out = append(out, p)
		}
		if out == nil {
			out = []P{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"providers": out})
	}
}

func CreateProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Label    string `json:"label"`
			AuthType string `json:"auth_type"` // "api_key" | "oauth"
			BaseURL  string `json:"base_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, 400)
			return
		}
		if req.Label == "" || req.AuthType == "" || req.BaseURL == "" {
			http.Error(w, `{"error":"label, auth_type, base_url required"}`, 400)
			return
		}
		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, err := q.DB().Exec(
				`INSERT INTO providers (label, auth_type, base_url, disabled) VALUES (?, ?, ?, 0)`,
				req.Label, req.AuthType, req.BaseURL,
			)
			if err != nil {
				http.Error(w, `{"error":"insert failed"}`, 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"label":"%s"}`, id, req.Label)
	}
}

func GetProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var row struct {
			ID        int64  `json:"id"`
			Label     string `json:"label"`
			AuthType  string `json:"auth_type"`
			BaseURL   string `json:"base_url"`
			Disabled  bool   `json:"disabled"`
			CreatedAt int64  `json:"created_at"`
		}
		err := d.QueryRow(`SELECT id, label, auth_type, base_url, disabled, created_at FROM providers WHERE id=?`, id).Scan(
			&row.ID, &row.Label, &row.AuthType, &row.BaseURL, &row.Disabled, &row.CreatedAt,
		)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, 404)
			return
		}
		json.NewEncoder(w).Encode(row)
	}
}

func UpdateProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req struct {
			Label    string `json:"label"`
			BaseURL  string `json:"base_url"`
			AuthType string `json:"auth_type"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`UPDATE providers SET label=?, base_url=?, auth_type=? WHERE id=?`,
				req.Label, req.BaseURL, req.AuthType, id)
		})
		w.WriteHeader(http.StatusOK)
	}
}

func DeleteProviderHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(`DELETE FROM providers WHERE id=?`, id)
		})
		w.WriteHeader(http.StatusOK)
	}
}

// --- Connections ---

func ListConnectionsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rows, err := d.Query(`SELECT id, provider_id, name, auth_type, disabled, priority, created_at FROM connections ORDER BY created_at DESC`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type C struct {
			ID        int64  `json:"id"`
			ProviderID string `json:"provider_id"`
			Name      string `json:"name"`
			AuthType  string `json:"auth_type"`
			Disabled  bool   `json:"disabled"`
			Priority  int    `json:"priority"`
			CreatedAt int64  `json:"created_at"`
		}
		var out []C
		for rows.Next() {
			var c C
			rows.Scan(&c.ID, &c.ProviderID, &c.Name, &c.AuthType, &c.Disabled, &c.Priority, &c.CreatedAt)
			out = append(out, c)
		}
		if out == nil {
			out = []C{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"connections": out})
	}
}

func CreateConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProviderID string `json:"provider_id"`
			Name       string `json:"name"`
			Secret     string `json:"secret"`
			AuthType   string `json:"auth_type"` // api_key | oauth
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.ProviderID == "" || req.Name == "" {
			http.Error(w, `{"error":"provider_id and name required"}`, 400)
			return
		}
		var encrypted string
		var err error
		if req.Secret != "" {
			encrypted, err = db.EncryptSecret(req.Secret)
			if err != nil {
				http.Error(w, `{"error":"encrypt"}`, 500)
				return
			}
		}
		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO connections (provider_id, name, auth_type, secret_enc, disabled) VALUES (?2, ?, ?3, 4)`,
				req.ProviderID, req.Name, req.AuthType, encrypted,
			)
			if e != nil {
				http.Error(w, `{"error":"insert"}`, 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d}`, id)
	}
}

func ToggleConnectionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var disabled bool
		var curDisabled bool
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().QueryRow(`SELECT disabled FROM connections WHERE id=?`, id).Scan(&curDisabled)
			disabled = !curDisabled
			q.DB().Exec(`UPDATE connections SET disabled=? WHERE id=?`, disabled, id)
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%s,"disabled":%t}`, id, disabled)
	}
}

// --- Proxy Pools ---

func ListProxyPoolsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`SELECT id, label, description, proxy_list, status, last_checked, created_at FROM proxy_pools ORDER BY created_at DESC`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type PP struct {
			ID          int64  `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
			ProxyList   string `json:"proxy_list"`
			Status      string `json:"status"`
			LastChecked int64  `json:"last_checked"`
			CreatedAt   int64  `json:"created_at"`
		}
		var out []PP
		for rows.Next() {
			var p PP
			rows.Scan(&p.ID, &p.Label, &p.Description, &p.ProxyList, &p.Status, &p.LastChecked, &p.CreatedAt)
			out = append(out, p)
		}
		if out == nil {
			out = []PP{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"proxy_pools": out})
	}
}

func CreateProxyPoolHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Label       string `json:"label"`
			Description string `json:"description"`
			ProxyList   string `json:"proxy_list"` // JSON array of strings
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Label == "" {
			http.Error(w, `{"error":"label required"}`, 400)
			return
		}
		var id int64
		d.EnqueueWriteSync(func(q *db.Queue) {
			res, e := q.DB().Exec(
				`INSERT INTO proxy_pools (label, description, proxy_list, status) VALUES (?, ?, ?, 'idle')`,
				req.Label, req.Description, req.ProxyList,
			)
			if e != nil {
				http.Error(w, `{"error":"insert"}`, 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"label":"%s"}`, id, req.Label)
	}
}

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
				`INSERT INTO combos (name, description, model_ids, strategy) VALUES (?, ?, ?, ?)`,
				req.Name, req.Description, modelsJSON, strategy,
			)
			if e != nil {
				http.Error(w, `{"error":"insert"}`, 500)
				return
			}
			id, _ = res.LastInsertId()
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"name":"%s"}`, id, req.Name)
	}
}

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

func ListAPIKeysHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r
		rows, err := d.Query(`SELECT id, key_hash, label, revoked, last_used, created_at FROM api_keys ORDER BY created_at DESC`)
		if err != nil {
			http.Error(w, `{"error":"query"}`, 500)
			return
		}
		defer rows.Close()
		type K struct {
			ID       int64  `json:"id"`
			KeyHash  string `json:"key_hash"`
			Label    string `json:"label"`
			Revoked  bool   `json:"revoked"`
			LastUsed string `json:"last_used"`
			CreatedAt int64 `json:"created_at"`
		}
		var out []K
		for rows.Next() {
			var k K
			rows.Scan(&k.ID, &k.KeyHash, &k.Label, &k.Revoked, &k.LastUsed, &k.CreatedAt)
			out = append(out, k)
		}
		if out == nil {
			out = []K{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"keys": out})
	}
}

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
		fmt.Fprintf(w, `{"id":"%s","key":"%s"}`, key[:12]+"...", key)
	}
}

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
			Model string `json:"model"`
			TokIn int    `json:"tok_in"`
			TokOut int   `json:"tok_out"`
			N     int    `json:"n"`
			Cost  float64 `json:"cost_usd"`
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
