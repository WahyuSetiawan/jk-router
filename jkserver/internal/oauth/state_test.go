package oauth_test

import (
	"testing"
	"time"

	"jkrouter/jkserver/internal/oauth"
)

func TestCreateAndConsume(t *testing.T) {
	store := oauth.NewStore(5 * time.Minute)
	id, st := store.Create("openai", "http://localhost/callback")
	if id == "" {
		t.Fatal("expected non-empty state ID")
	}
	if st.ProviderID != "openai" {
		t.Errorf("provider mismatch: %s", st.ProviderID)
	}
	consumed := store.Consume(id)
	if consumed == nil {
		t.Fatal("expected to consume state")
	}
	if consumed.Redirect != "http://localhost/callback" {
		t.Errorf("redirect mismatch")
	}
	if store.Consume(id) != nil {
		t.Error("expected nil on double consume")
	}
}

func TestConsumeUnknownReturnsNil(t *testing.T) {
	store := oauth.NewStore(5 * time.Minute)
	if store.Consume("nonexistent") != nil {
		t.Error("expected nil for unknown state")
	}
}

func TestExpiredStateReturnsNil(t *testing.T) {
	store := oauth.NewStore(1 * time.Millisecond)
	id, _ := store.Create("openai", "http://localhost/cb")
	time.Sleep(10 * time.Millisecond)
	if store.Consume(id) != nil {
		t.Error("expected nil for expired state")
	}
}
