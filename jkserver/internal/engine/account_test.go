package engine_test

import (
	"encoding/json"
	"testing"
	"time"

	"jkrouter/jkserver/internal/engine"
)

func TestAccountStoreBasicOps(t *testing.T) {
	store := engine.NewAccountStore()
	a := &engine.Account{ID: 1, ProviderID: "openai", Label: "test-key-1", State: engine.StateActive, Priority: 10}
	store.Upsert(a)

	got := store.Get(1)
	if got == nil {
		t.Fatal("expected account")
	}
	if got.Label != "test-key-1" {
		t.Errorf("label mismatch: %s", got.Label)
	}
}

func TestMarkCoolingDown(t *testing.T) {
	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "openai", Label: "key1", State: engine.StateActive, Priority: 5})

	store.MarkCoolingDown(1, 60*time.Second)

	a := store.Get(1)
	if a.State != engine.StateCoolingDown {
		t.Errorf("expected cooling_down, got %s", a.State)
	}
	if a.StrikeCount != 1 {
		t.Errorf("expected strike_count=1, got %d", a.StrikeCount)
	}
	if a.CooledUntil.IsZero() {
		t.Error("expected CooledUntil to be set")
	}
}

func TestStrikeBreakerEscalation(t *testing.T) {
	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "openai", Label: "key1", State: engine.StateActive})

	store.MarkCoolingDown(1, 60*time.Second)
	a := store.Get(1)
	d1 := a.CooledUntil.Sub(time.Now())

	store.Reenable(1)
	store.MarkCoolingDown(1, 60*time.Second)
	a = store.Get(1)
	d2 := a.CooledUntil.Sub(time.Now())
	if d2 < d1 {
		t.Errorf("expected longer cool-down on second strike: d1=%v d2=%v", d1, d2)
	}
}

func TestFilterByProviderSkipsDisabled(t *testing.T) {
	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "openai", Label: "active", State: engine.StateActive})
	store.Upsert(&engine.Account{ID: 2, ProviderID: "openai", Label: "disabled", State: engine.StateDisabled})
	store.Upsert(&engine.Account{ID: 3, ProviderID: "anthropic", Label: "other", State: engine.StateActive})

	active := store.FilterByProvider("openai", engine.StateActive)
	if len(active) != 1 {
		t.Errorf("expected 1 active openai account, got %d", len(active))
	}
	if active[0].Label != "active" {
		t.Errorf("expected 'active', got %s", active[0].Label)
	}
}

func TestComboBuildCandidates(t *testing.T) {
	store := engine.NewAccountStore()
	store.Upsert(&engine.Account{ID: 1, ProviderID: "openai", Label: "key-a", State: engine.StateActive, Priority: 10})
	store.Upsert(&engine.Account{ID: 2, ProviderID: "openai", Label: "key-b", State: engine.StateCoolingDown, Priority: 5})
	store.Upsert(&engine.Account{ID: 3, ProviderID: "openai", Label: "key-c", State: engine.StateActive, Priority: 20})

	combo := &engine.Combo{ID: 1, Name: "test-combo", Accounts: []int64{1, 2, 3}}
	cands := engine.BuildComboCandidates(combo, store)
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates (skipping cooling_down), got %d", len(cands))
	}
	if cands[0].Label != "key-c" {
		t.Errorf("expected highest priority first, got %s", cands[0].Label)
	}
	if cands[1].Label != "key-a" {
		t.Errorf("expected second highest, got %s", cands[1].Label)
	}
}

func TestStripHistoryForContext(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4o",
		"messages":[
			{"role":"user","content":"turn1"},
			{"role":"assistant","content":"reply1"},
			{"role":"user","content":"turn2"},
			{"role":"assistant","content":"reply2"},
			{"role":"user","content":"turn3"}
		]
	}`)
	trimmed, err := engine.StripHistoryForContext(body, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.Unmarshal(trimmed, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages after trim, got %d", len(result.Messages))
	}
}
