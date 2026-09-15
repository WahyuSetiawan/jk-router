// Package rtk: Reduced Token Keeper - manages token usage to stay within limits.
// This is a simplified stub; full RTK requires per-provider token estimation APIs.
package rtk

import (
	"sync"
	"time"
)

// Saver tracks approximate token usage per account over a sliding window.
type Saver struct {
	mu       sync.RWMutex
	window   time.Duration
	usage    map[string][]usagePoint // key -> list of (time, input_tokens, output_tokens)
	maxTokens int
}

type usagePoint struct {
	ts    int64
	in    int
	out   int
}

// New creates a new Saver. maxTokens is the limit to stay under.
func New(window time.Duration, maxTokens int) *Saver {
	return &Saver{window: window, maxTokens: maxTokens, usage: make(map[string][]usagePoint)}
}

// Record adds a usage point for an account.
func (s *Saver) Record(accountID string, in, out int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	s.usage[accountID] = append(s.usage[accountID], usagePoint{ts: now, in: in, out: out})
	s.prune(accountID, now)
}

// Usage returns total tokens used in the window for an account.
func (s *Saver) Usage(accountID string) (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().Unix()
	inTotal, outTotal := 0, 0
	for _, p := range s.usage[accountID] {
		if now-p.ts <= int64(s.window.Seconds()) {
			inTotal += p.in
			outTotal += p.out
		}
	}
	return inTotal, outTotal
}

// OverQuota returns true if the account is approaching its limit.
func (s *Saver) OverQuota(accountID string) bool {
	in, out := s.Usage(accountID)
	return in+out > s.maxTokens
}

func (s *Saver) prune(key string, now int64) {
	pts := s.usage[key]
	cutoff := now - int64(s.window.Seconds())
	valid := pts[:0]
	for _, p := range pts {
		if p.ts >= cutoff {
			valid = append(valid, p)
		}
	}
	s.usage[key] = valid
}
