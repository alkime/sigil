package lsp

import (
	"context"
	"sync"
)

type managerKey struct {
	language string
	root     string
}

// Manager caches LSP Clients keyed by (language, project root). Two calls with
// the same key share a single subprocess.
type Manager struct {
	mu      sync.Mutex
	clients map[managerKey]*Client
}

// NewManager returns an empty Manager.
func NewManager() *Manager {
	return &Manager{clients: map[managerKey]*Client{}}
}

// Get returns a Client for (cfg.Language, root), spawning one lazily.
// Subsequent calls with the same key return the same Client.
func (m *Manager) Get(ctx context.Context, cfg ServerConfig, root string) (*Client, error) {
	key := managerKey{language: cfg.Language, root: root}

	m.mu.Lock()
	if c, ok := m.clients[key]; ok {
		m.mu.Unlock()
		return c, nil
	}
	m.mu.Unlock()

	// Spawn outside the lock so concurrent Get calls for different keys don't serialize on initialize.
	c, err := NewClient(ctx, cfg, root)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if existing, ok := m.clients[key]; ok {
		m.mu.Unlock()
		_ = c.Close()
		return existing, nil
	}
	m.clients[key] = c
	m.mu.Unlock()
	return c, nil
}

// Close closes every cached Client. Subsequent Get calls will spawn fresh clients.
func (m *Manager) Close() error {
	m.mu.Lock()
	clients := m.clients
	m.clients = map[managerKey]*Client{}
	m.mu.Unlock()
	for _, c := range clients {
		_ = c.Close()
	}
	return nil
}
