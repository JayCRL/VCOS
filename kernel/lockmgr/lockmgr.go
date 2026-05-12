// Package lockmgr provides a string-keyed, TTL-bounded, reentrant lock
// manager for serializing access to VCOS resources (sessions, files,
// external services). It is the resource-arbitration substrate used by the
// scheduler and watchdog in Phase 1.
//
// Reentrancy: Acquire by the same Owner on the same key succeeds and bumps
// the hold count. Each Acquire must be matched by exactly one Release; the
// lock is freed when the count reaches zero or the TTL elapses.
package lockmgr

import (
	"errors"
	"sync"
	"time"
)

// ErrConflict is returned when a different owner already holds the lock.
var ErrConflict = errors.New("lockmgr: held by another owner")

// ErrNotHeld is returned by Release when the (key, owner) combination is not
// the current holder.
var ErrNotHeld = errors.New("lockmgr: not held by owner")

// Lease describes a granted lock.
type Lease struct {
	Key      string
	Owner    string
	ExpireAt time.Time
	Depth    int
}

// Manager is a TTL-bounded reentrant lock manager.
type Manager struct {
	now func() time.Time

	mu      sync.Mutex
	entries map[string]*entry

	expiredCh chan Lease
	stopCh    chan struct{}
	stopOnce  sync.Once
}

type entry struct {
	owner    string
	depth    int
	expireAt time.Time
	timer    *time.Timer
}

// New returns a new Manager. NowFunc is for testing; pass nil for time.Now.
// The expiredCh is buffered to 64; consumers (typically the watchdog) should
// drain it promptly.
func New(nowFunc func() time.Time) *Manager {
	if nowFunc == nil {
		nowFunc = time.Now
	}
	return &Manager{
		now:       nowFunc,
		entries:   make(map[string]*entry),
		expiredCh: make(chan Lease, 64),
		stopCh:    make(chan struct{}),
	}
}

// Expired returns a channel that receives leases evicted by TTL.
func (m *Manager) Expired() <-chan Lease { return m.expiredCh }

// Acquire grants the lock to owner for ttl, or returns ErrConflict if held by
// another owner. Reentrant calls succeed.
func (m *Manager) Acquire(key, owner string, ttl time.Duration) (Lease, error) {
	if key == "" || owner == "" {
		return Lease{}, errors.New("lockmgr: key and owner required")
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if e, ok := m.entries[key]; ok {
		if e.owner != owner {
			return Lease{}, ErrConflict
		}
		e.depth++
		e.expireAt = now.Add(ttl)
		if e.timer != nil {
			e.timer.Stop()
		}
		e.timer = time.AfterFunc(ttl, func() { m.expire(key, owner) })
		return Lease{Key: key, Owner: owner, ExpireAt: e.expireAt, Depth: e.depth}, nil
	}
	e := &entry{owner: owner, depth: 1, expireAt: now.Add(ttl)}
	e.timer = time.AfterFunc(ttl, func() { m.expire(key, owner) })
	m.entries[key] = e
	return Lease{Key: key, Owner: owner, ExpireAt: e.expireAt, Depth: e.depth}, nil
}

// Release decrements the reentrant depth; the lock is freed when depth hits 0.
func (m *Manager) Release(key, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok || e.owner != owner {
		return ErrNotHeld
	}
	e.depth--
	if e.depth <= 0 {
		if e.timer != nil {
			e.timer.Stop()
		}
		delete(m.entries, key)
	}
	return nil
}

// ForceRelease drops the lock regardless of owner. Used by the watchdog.
func (m *Manager) ForceRelease(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok {
		if e.timer != nil {
			e.timer.Stop()
		}
		delete(m.entries, key)
	}
}

// Holder returns the current owner and remaining TTL, or ("", 0) if free.
func (m *Manager) Holder(key string) (string, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok {
		return "", 0
	}
	return e.owner, time.Until(e.expireAt)
}

// Active returns the count of currently held keys.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// Close stops all timers and closes the expired channel.
func (m *Manager) Close() {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		for _, e := range m.entries {
			if e.timer != nil {
				e.timer.Stop()
			}
		}
		m.entries = nil
		m.mu.Unlock()
		close(m.stopCh)
		close(m.expiredCh)
	})
}

func (m *Manager) expire(key, owner string) {
	m.mu.Lock()
	e, ok := m.entries[key]
	if !ok || e.owner != owner {
		m.mu.Unlock()
		return
	}
	now := m.now()
	if e.expireAt.After(now) {
		// Renewed in the meantime; nothing to do.
		m.mu.Unlock()
		return
	}
	depth := e.depth
	expireAt := e.expireAt
	delete(m.entries, key)
	m.mu.Unlock()
	lease := Lease{Key: key, Owner: owner, ExpireAt: expireAt, Depth: depth}
	select {
	case m.expiredCh <- lease:
	case <-m.stopCh:
	default:
		// Drop on full to avoid blocking the timer goroutine.
	}
}
