// Package sessionstate provides the in-memory store for the volatile
// per-session snapshots (UsersCount / WorldUrl / etc.) that the container
// owns. Persistent session fields stay in postgres; only the live state
// lives here so a controller restart does not stuff stale numbers into
// the UI.
package sessionstate

import (
	"sync"

	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// MemoryCache is the production SessionStateCache implementation. A plain
// RWMutex+map suffices: writes are infrequent (one per HostEvent) and
// reads are O(1).
type MemoryCache struct {
	mu       sync.RWMutex
	sessions map[string]*headlessv1.Session
}

var _ port.SessionStateCache = (*MemoryCache)(nil)

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{sessions: make(map[string]*headlessv1.Session)}
}

func (c *MemoryCache) Get(sessionID string) (*headlessv1.Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s, ok := c.sessions[sessionID]

	return s, ok
}

func (c *MemoryCache) Set(sessionID string, snapshot *headlessv1.Session) {
	if snapshot == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessions[sessionID] = snapshot
}

func (c *MemoryCache) Delete(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.sessions, sessionID)
}
