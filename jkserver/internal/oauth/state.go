// Package oauth provides device/browser OAuth flow support.
package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// State holds an in-flight OAuth state parameter with expiry.
type State struct {
	ProviderID string
	Redirect   string
	CreatedAt  time.Time
}

// Store manages active OAuth states.
type Store struct {
	mu     sync.RWMutex
	states map[string]*State
	expiry time.Duration
}

// NewStore creates a Store with the given state lifetime.
func NewStore(expiry time.Duration) *Store {
	if expiry <= 0 {
		expiry = 5 * time.Minute
	}
	return &Store{states: make(map[string]*State), expiry: expiry}
}

// Create generates a new state and returns it.
func (s *Store) Create(providerID, redirectURL string) (string, *State) {
	b := make([]byte, 16)
	rand.Read(b)
	id := hex.EncodeToString(b)
	state := &State{ProviderID: providerID, Redirect: redirectURL, CreatedAt: time.Now()}
	s.mu.Lock()
	s.states[id] = state
	s.mu.Unlock()
	return id, state
}

// Consume validates and removes a state. Returns the state or nil if expired/invalid.
func (s *Store) Consume(id string) *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	if !ok {
		return nil
	}
	delete(s.states, id)
	if time.Since(st.CreatedAt) > s.expiry {
		return nil
	}
	return st
}
