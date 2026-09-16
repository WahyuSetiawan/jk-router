// Package refresh provides periodic model auto-refresh for provider registries.
package refresh

import (
	"log"
	"sync"
	"time"

	"jkrouter/jkserver/internal/providers/registry"
)

// Manager runs periodic model refreshes across all registered providers.
type Manager struct {
	interval time.Duration
	apiKey   string
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// Option configures the Manager.
type Option func(*Manager)

// WithInterval sets the refresh interval (default 6 hours).
func WithInterval(d time.Duration) Option {
	return func(m *Manager) {
		m.interval = d
	}
}

// WithAPIKey sets the API key used for validateUrl requests.
func WithAPIKey(key string) Option {
	return func(m *Manager) {
		m.apiKey = key
	}
}

// NewManager creates a Manager with the given options.
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		interval: 6 * time.Hour,
		stopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Start launches the background refresh goroutine.
func (m *Manager) Start() {
	m.wg.Add(1)
	go m.run()
}

// Stop signals the background goroutine to exit and waits for it.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

func (m *Manager) run() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	// Run once immediately on start.
	m.refreshAll()
	for {
		select {
		case <-ticker.C:
			m.refreshAll()
		case <-m.stopCh:
			return
		}
	}
}

// RefreshAll triggers an immediate model refresh across all providers.
func (m *Manager) RefreshAll() {
	m.refreshAll()
}

func (m *Manager) refreshAll() {
	regs := registry.GetRegistries()
	for _, reg := range regs {
		if reg.ValidateURL == "" {
			continue
		}
		if err := reg.RefreshModels(m.apiKey); err != nil {
			log.Printf("[refresh] provider %s: %v", reg.ID, err)
		} else {
			log.Printf("[refresh] provider %s: refreshed %d models", reg.ID, len(reg.Models))
		}
	}
}
