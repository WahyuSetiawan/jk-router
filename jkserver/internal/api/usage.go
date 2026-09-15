// Package api: usage logging and pricing helpers.
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"net/http"

	"jkrouter/jkserver/internal/db"
)

// UsageLogger writes usage records to SQLite and ~/.jkrouter/log.txt.
type UsageLogger struct {
	db      *db.DB
	logPath string
}

// NewUsageLogger creates a logger. logPath is the file to tail-write (optional).
func NewUsageLogger(d *db.DB, logPath string) *UsageLogger {
	return &UsageLogger{db: d, logPath: logPath}
}

// UsageEntry is one row in usage_log.
type UsageEntry struct {
	RequestID    string
	Combo        string
	Model        string
	Provider     string
	AccountID    sql.NullInt64
	FallbackFrom string
	StateAtStart string
	TokIn        int
	TokOut       int
	LatencyMs    int
	Status       string // success | error | fallback
	AdapterUsed  int
}

// LogRequest records a request after dispatch completes.
func (ul *UsageLogger) LogRequest(entry UsageEntry) {
	if ul.db == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	ul.db.EnqueueWriteSync(func(q *db.Queue) {
		_, err := q.DB().Exec(
			`INSERT INTO usage_log (request_id, combo, model, provider, account_id, fallback_from,
			   state_at_start, tok_in, tok_out, latency_ms, status, adapter_used, ts)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			entry.RequestID,
			entry.Combo,
			entry.Model,
			entry.Provider,
			entry.AccountID,
			entry.FallbackFrom,
			entry.StateAtStart,
			entry.TokIn,
			entry.TokOut,
			entry.LatencyMs,
			entry.Status,
			entry.AdapterUsed,
			ts,
		)
		if err != nil {
			log.Printf("[usage] insert failed: %v", err)
		}
	})
	if ul.logPath != "" {
		appendToFile(ul.logPath, fmt.Sprintf(
			"%s | req=%s combo=%s model=%s provider=%s status=%s latency=%dms tokens_in=%d out=%d\n",
			ts, entry.RequestID, entry.Combo, entry.Model, entry.Provider,
			entry.Status, entry.LatencyMs, entry.TokIn, entry.TokOut,
		))
	}
}

// PricingQuery returns cost estimate from the pricing table.
func PricingQuery(d *db.DB, model string) (float64, float64, error) {
	row := d.QueryRow(`SELECT price_in_per_1k, price_out_per_1k FROM pricing WHERE model=?`, model)
	var pin, pout float64
	err := row.Scan(&pin, &pout)
	if err != nil {
		return 0, 0, nil // no pricing = $0
	}
	return pin, pout, nil
}

// CostForTokens returns estimated USD cost given input/output tokens and pricing.
func CostForTokens(pin, pout float64, tokIn, tokOut int) float64 {
	return float64(tokIn)/1000.0*pin + float64(tokOut)/1000.0*pout
}

// SeedPricing inserts default pricing rows if not present.
func SeedPricing(d *db.DB) {
	defaults := []struct {
		Model, Pin, POut string
	}{
		{"gpt-4o", "0.005", "0.015"},
		{"gpt-4o-mini", "0.0015", "0.006"},
		{"gpt-4.1", "0.015", "0.06"},
		{"claude-sonnet-4", "0.003", "0.015"},
		{"claude-haiku-3", "0.00025", "0.00125"},
		{"deepseek-chat", "0.0003", "0.0012"},
		{"deepseek-reasoner", "0.001", "0.004"},
		{"groq-llama3-70b", "0.00059", "0.00079"},
	}
	for _, p := range defaults {
		d.EnqueueWriteSync(func(q *db.Queue) {
			q.DB().Exec(
				`INSERT OR IGNORE INTO pricing (model, price_in_per_1k, price_out_per_1k, source) VALUES (?, ?, ?, 'seed')`,
				p.Model, p.Pin, p.POut,
			)
		})
	}
}

// --- Dashboard: usage query endpoint ---

// UsageHandler serves /api/dashboard/usage with optional filters.
func UsageHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		model    := r.URL.Query().Get("model")
		date     := r.URL.Query().Get("date")

		q := `SELECT request_id, combo, model, provider, status, latency_ms, tok_in, tok_out, adapter_used, ts
		      FROM usage_log WHERE 1=1`
		args := []interface{}{}
		if provider != "" {
			q += ` AND provider=?`
			args = append(args, provider)
		}
		if model != "" {
			q += ` AND model=?`
			args = append(args, model)
		}
		if date != "" {
			q += ` AND date(ts)=?`
			args = append(args, date)
		}
		q += ` ORDER BY ts DESC LIMIT 200`

		rows, err := d.Query(q, args...)
		if err != nil {
			http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type URow struct {
			RequestID   string `json:"request_id"`
			Combo       string `json:"combo"`
			Model       string `json:"model"`
			Provider    string `json:"provider"`
			Status      string `json:"status"`
			LatencyMs   int    `json:"latency_ms"`
			TokIn       int    `json:"tok_in"`
			TokOut      int    `json:"tok_out"`
			AdapterUsed int    `json:"adapter_used"`
			TS          string `json:"ts"`
		}
		var out []URow
		for rows.Next() {
			var u URow
			if err := rows.Scan(&u.RequestID, &u.Combo, &u.Model, &u.Provider, &u.Status, &u.LatencyMs, &u.TokIn, &u.TokOut, &u.AdapterUsed, &u.TS); err != nil {
				continue
			}
			out = append(out, u)
		}
		if out == nil {
			out = []URow{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"usage": out})
	}
}

func appendToFile(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[usage] open log file failed: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		log.Printf("[usage] write failed: %v", err)
	}
}

var _ chi.Router = nil // compile-time check

// UsageTailHandler returns the last N lines from usage_log (default 50).
func UsageTailHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 50
		if v := r.URL.Query().Get("n"); v != "" {
			fmt.Sscanf(v, "%d", &n)
			if n <= 0 || n > 500 {
				n = 50
			}
		}
		rows, err := d.Query(`SELECT ts, request_id, combo, model, provider, status, latency_ms, tok_in, tok_out FROM usage_log ORDER BY ts DESC LIMIT ?`, n)
		if err != nil {
			http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		type Item struct {
			TS        string `json:"ts"`
			RequestID string `json:"request_id"`
			Combo     string `json:"combo"`
			Model     string `json:"model"`
			Provider  string `json:"provider"`
			Status    string `json:"status"`
			LatencyMs int    `json:"latency_ms"`
			TokIn     int    `json:"tok_in"`
			TokOut    int    `json:"tok_out"`
		}
		var out []Item
		for rows.Next() {
			var it Item
			if err := rows.Scan(&it.TS, &it.RequestID, &it.Combo, &it.Model, &it.Provider, &it.Status, &it.LatencyMs, &it.TokIn, &it.TokOut); err != nil {
				continue
			}
			out = append(out, it)
		}
		if out == nil {
			out = []Item{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"tail": out})
	}
}
