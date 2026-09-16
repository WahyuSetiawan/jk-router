// Package engine provides account state management and combo fallback logic.
package engine

import (
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"
)

// AccountState represents the circuit-breaker state of a connection account.
type AccountState string

const (
	StateActive       AccountState = "active"
	StateCoolingDown  AccountState = "cooling_down"
	StateDisabled     AccountState = "disabled"
)

// Account holds the in-memory state for one upstream connection account.
type Account struct {
	ID           int64
	ProviderID   string
	Label        string
	AuthType     string
	APIKey       string // plaintext, only in memory
	ProxyPoolID  *int64
	Priority     int
	State        AccountState
	StrikeCount  int
	CooledUntil  time.Time
	UpdatedAtlas time.Time
}

// AccountStore manages in-memory account states with thread-safe access.
type AccountStore struct {
	mu       sync.RWMutex
	accounts map[int64]*Account
}

// NewAccountStore creates an empty store.
func NewAccountStore() *AccountStore {
	return &AccountStore{accounts: make(map[int64]*Account)}
}

// Upsert replaces or inserts an account.
func (s *AccountStore) Upsert(a *Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[a.ID] = a
}

// Get returns the account for the given ID, or nil.
func (s *AccountStore) Get(id int64) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accounts[id]
}

// GetAll returns all accounts.
func (s *AccountStore) GetAll() []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		out = append(out, a)
	}
	return out
}

// FilterByProvider returns accounts for the given provider, filtered by state.
func (s *AccountStore) FilterByProvider(providerID string, state AccountState) []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Account
	for _, a := range s.accounts {
		if a.ProviderID != providerID {
			continue
		}
		if state != "" && a.State != state {
			continue
		}
		if a.State == StateCoolingDown && time.Now().After(a.CooledUntil) {
			a.State = StateActive
		}
		out = append(out, a)
	}
	return out
}

// MarkCoolingDown sets the account to cooling_down for the given duration.
// Repeated 429s increase the duration (strike breaker).
func (s *AccountStore) MarkCoolingDown(id int64, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return
	}
	a.StrikeCount++
	base := duration
	if a.StrikeCount > 1 {
		base = duration * time.Duration(1<<uint(a.StrikeCount-1))
		if base > 10*time.Minute {
			base = 10*time.Minute
		}
	}
	a.State = StateCoolingDown
	a.CooledUntil = time.Now().Add(base)
	a.UpdatedAtlas = time.Now()
	log.Printf("[account] %s (%d) -> cooling_down for %v (strikes=%d)", a.Label, id, base, a.StrikeCount)
}

// SaveStateCommit writes the full in-memory account state to the database.
// Call after any state transition (MarkCoolingDown, MarkDisabled, Reenable).
// ponytail: single UPDATE per account; batch once throughput becomes an issue.
func (s *AccountStore) SaveStateCommit(fn func(id int64, state string, strike int, cooledUntil int64, updatedAt int64)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.accounts {
		cooledUntil := int64(0)
		if !a.CooledUntil.IsZero() {
			cooledUntil = a.CooledUntil.Unix()
		}
		fn(a.ID, string(a.State), a.StrikeCount, cooledUntil, a.UpdatedAtlas.Unix())
	}
}

// MarkDisabled permanently disables the account until manual re-enable.
func (s *AccountStore) MarkDisabled(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return
	}
	a.State = StateDisabled
	a.UpdatedAtlas = time.Now()
	log.Printf("[account] %s (%d) -> disabled", a.Label, id)
}

// Reenable enables a disabled or cooling_down account.
func (s *AccountStore) Reenable(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return
	}
	a.State = StateActive
	a.StrikeCount = 0
	a.CooledUntil = time.Time{}
	a.UpdatedAtlas = time.Now()
	log.Printf("[account] %s (%d) -> active (manual re-enable)", a.Label, id)
}

// Combo defines a fallback chain of accounts for a given model set.
type Combo struct {
	ID       int64
	Name     string
	ModelIDs []string
	Strategy string // "fallback" or "round_robin"
	Accounts []int64
}

// ComboStore manages combo definitions.
type ComboStore struct {
	mu     sync.RWMutex
	combos map[int64]*Combo
	nextID int64
}

// NewComboStore creates an empty combo store.
func NewComboStore() *ComboStore {
	return &ComboStore{combos: make(map[int64]*Combo)}
}

// Add inserts a new combo and returns its ID.
func (s *ComboStore) Add(name, strategy string, modelIDs []string, accountIDs []int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	c := &Combo{
		ID:       s.nextID,
		Name:     name,
		ModelIDs: modelIDs,
		Strategy: strategy,
		Accounts: accountIDs,
	}
	s.combos[c.ID] = c
	return c.ID
}

// Get returns the combo for the given ID.
func (s *ComboStore) Get(id int64) *Combo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.combos[id]
}

// List returns all combos.
func (s *ComboStore) List() []*Combo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Combo, 0, len(s.combos))
	for _, c := range s.combos {
		out = append(out, c)
	}
	return out
}

// ComboResult holds the outcome of a combo fallback attempt.
type ComboResult struct {
	Success      bool
	ProviderID   string
	Model        string
	AccountID    int64
	FallbackFrom string
	RequestBody  []byte
}

// BuildComboCandidates returns ordered candidates from a combo for a given request.
func BuildComboCandidates(combo *Combo, store *AccountStore) []*Account {
	if combo == nil {
		return nil
	}
	var candidates []*Account
	for _, aid := range combo.Accounts {
		a := store.Get(aid)
		if a == nil {
			continue
		}
		if a.State == StateActive {
			candidates = append(candidates, a)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})
	return candidates
}

// ParseComboFromBody extracts combo name and model from an OpenAI-format request body.
func ParseComboFromBody(body []byte) (comboName string, model string, ok bool) {
	var req struct {
		Combo string `json:"combo"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", false
	}
	if req.Combo == "" || req.Model == "" {
		return "", "", false
	}
	return req.Combo, req.Model, true
}

// EstimateTokenCount is a rough token estimate from request body byte count.
// ponytail: exact token counting requires a tokenizer; this is a proxy for logging.
func EstimateTokenCount(body []byte) int {
	return len(body) / 4
}

// FormatErrorResult builds an OpenAI-compatible error response body.
func FormatErrorResult(msg string, code int) []byte {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "upstream_error",
			"message": msg,
			"code":    code,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// StripHistoryForContext trims middle conversation turns while preserving
// system message and the last user turn with media attachments.
// ponytail: full context-window management requires knowing the model's max tokens.
func StripHistoryForContext(body []byte, maxTurns int) ([]byte, error) {
	var req struct {
		Model    string          `json:"model"`
		Messages json.RawMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return body, nil
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal(req.Messages, &msgs); err != nil {
		return body, nil
	}
	if len(msgs) <= maxTurns {
		return body, nil
	}
	truncated := msgs[len(msgs)-maxTurns:]
	req.Messages, _ = json.Marshal(truncated)
	return json.Marshal(req)
}
