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

// entry pairs a snapshot with its owning host so PruneHost can find it
// without an O(N) scan.
type entry struct {
	hostID   string
	snapshot *headlessv1.Session
}

// MemoryCache is the production SessionStateCache implementation. A plain
// RWMutex+map suffices: writes are infrequent (one per HostEvent) and
// reads are O(1).
//
// Internal layout:
//   - sessions: sessionID → entry (snapshot + owner hostID)
//   - hostSessions: hostID → set of sessionIDs (for PruneHost)
//
// Session can migrate between hosts (CustomSessionId); a later
// Set(hostID2, ...) for the same sessionID rewrites the owner index so
// PruneHost on the old host no longer touches it.
type MemoryCache struct {
	mu           sync.RWMutex
	sessions     map[string]entry
	hostSessions map[string]map[string]struct{}
}

var _ port.SessionStateCache = (*MemoryCache)(nil)

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		sessions:     make(map[string]entry),
		hostSessions: make(map[string]map[string]struct{}),
	}
}

func (c *MemoryCache) Get(sessionID string) (*headlessv1.Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.sessions[sessionID]
	if !ok {
		return nil, false
	}

	return e.snapshot, true
}

func (c *MemoryCache) Set(hostID, sessionID string, snapshot *headlessv1.Session) {
	if snapshot == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 前の owner と違う host にマイグレートしたら旧 owner index から外す
	if prev, ok := c.sessions[sessionID]; ok && prev.hostID != hostID {
		c.removeFromHostIndex(prev.hostID, sessionID)
	}

	c.sessions[sessionID] = entry{hostID: hostID, snapshot: snapshot}

	set, ok := c.hostSessions[hostID]
	if !ok {
		set = make(map[string]struct{})
		c.hostSessions[hostID] = set
	}

	set[sessionID] = struct{}{}
}

func (c *MemoryCache) Delete(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.sessions[sessionID]
	if !ok {
		return
	}

	delete(c.sessions, sessionID)
	c.removeFromHostIndex(e.hostID, sessionID)
}

func (c *MemoryCache) PruneHost(hostID string, liveSessionIDs map[string]struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	set, ok := c.hostSessions[hostID]
	if !ok {
		return
	}

	for sessionID := range set {
		if _, alive := liveSessionIDs[sessionID]; alive {
			continue
		}

		delete(c.sessions, sessionID)
		delete(set, sessionID)
	}

	if len(set) == 0 {
		delete(c.hostSessions, hostID)
	}
}

// removeFromHostIndex は呼び出し元が c.mu を Write Lock 済みである前提で
// hostSessions index から (hostID, sessionID) を取り除き、空になった host
// entry も掃除する。
func (c *MemoryCache) removeFromHostIndex(hostID, sessionID string) {
	set, ok := c.hostSessions[hostID]
	if !ok {
		return
	}

	delete(set, sessionID)

	if len(set) == 0 {
		delete(c.hostSessions, hostID)
	}
}
