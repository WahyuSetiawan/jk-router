// Package proxypool manages proxy pool configuration and health testing.
package proxypool

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Pool represents a proxy pool configuration.
type Pool struct {
	ID           int64
	Name         string
	Type         string // "http", "socks5", "relay"
	ProxyURL     string
	NoProxy      string
	StrictProxy  bool
	IsActive     bool
	TestStatus   string // "untested", "healthy", "unhealthy"
	LastTestedAt time.Time
}

// IsRelay returns true if this pool is a relay-type pool.
func (p *Pool) IsRelay() bool { return p.Type == "relay" }

// Store manages proxy pools in memory.
type Store struct {
	mu     sync.RWMutex
	pools  map[int64]*Pool
	nextID int64
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{pools: make(map[int64]*Pool)}
}

// Add creates a new pool and returns its ID.
func (s *Store) Add(name, proxyURL, noProxy string, strict bool) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	p := &Pool{
		ID:          s.nextID,
		Name:        name,
		ProxyURL:    proxyURL,
		NoProxy:     noProxy,
		StrictProxy: strict,
		Type:        "http",
		IsActive:    true,
		TestStatus:  "untested",
	}
	s.pools[p.ID] = p
	return p.ID
}

// AddWithType creates a new pool with explicit type and returns its ID.
func (s *Store) AddWithType(name, ptype, proxyURL, noProxy string, strict bool) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	p := &Pool{
		ID:          s.nextID,
		Name:        name,
		Type:        ptype,
		ProxyURL:    proxyURL,
		NoProxy:     noProxy,
		StrictProxy: strict,
		IsActive:    true,
		TestStatus:  "untested",
	}
	s.pools[p.ID] = p
	return p.ID
}

// Get returns the pool for the given ID.
func (s *Store) Get(id int64) *Pool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pools[id]
}

// List returns all pools.
func (s *Store) List() []*Pool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Pool, 0, len(s.pools))
	for _, p := range s.pools {
		out = append(out, p)
	}
	return out
}

// UpdateStatus updates the health status of a pool (auto-deactivate on unhealthy).
func (s *Store) UpdateStatus(id int64, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pools[id]
	if !ok {
		return
	}
	p.TestStatus = status
	p.LastTestedAt = time.Now()
	if status == "unhealthy" && p.IsActive {
		p.IsActive = false
	}
}

// BuildTransport returns an http.Transport configured with this pool's proxy settings.
func (p *Pool) BuildTransport() *http.Transport {
	tr := &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if p.ProxyURL != "" {
		proxyURL, err := url.Parse(p.ProxyURL)
		if err == nil {
			tr.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return tr
}

// HealthCheck probes the proxy with a lightweight GET to confirm reachability.
func (p *Pool) HealthCheck(timeout time.Duration) string {
	if p.ProxyURL == "" {
		return "no_proxy_configured"
	}
	proxyURL, err := url.Parse(p.ProxyURL)
	if err != nil {
		return "invalid_url"
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			DialContext:         (&net.Dialer{Timeout: timeout}).DialContext,
			TLSHandshakeTimeout: timeout,
		},
	}
	resp, err := client.Get("http://ipinfo.io/json")
	if err != nil {
		return "unhealthy"
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "unhealthy"
	}
	return "healthy"
}

// ExtractHostPort splits a proxy URL into host and port.
func ExtractHostPort(proxyURL string) (string, int, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return "", 0, err
	}
	host := u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		if strings.HasPrefix(proxyURL, "https://") {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 0, err
	}
	return host, port, nil
}
