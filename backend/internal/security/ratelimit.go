package security

import (
	"sync"
	"time"
)

const defaultMaxEntries = 10000

type loginEntry struct {
	windowStart time.Time
	count       int
}

// LoginLimiter is a bounded fixed-window limiter for login attempts.
// It is deliberately simple and per-process; a multi-instance deployment would
// need a shared store.
type LoginLimiter struct {
	limit      int
	window     time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]loginEntry
}

func NewLoginLimiter(limit int, window time.Duration) *LoginLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &LoginLimiter{
		limit:      limit,
		window:     window,
		maxEntries: defaultMaxEntries,
		now:        time.Now,
		entries:    make(map[string]loginEntry),
	}
}

// Allow reports whether the attempt is allowed and, when denied, how long to wait.
func (l *LoginLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.windowStart) >= l.window {
		l.entries[key] = loginEntry{windowStart: now, count: 1}
		l.cleanup(now)
		return true, 0
	}

	if entry.count >= l.limit {
		retry := l.window - now.Sub(entry.windowStart)
		if retry < time.Second {
			retry = time.Second
		}
		return false, retry
	}

	entry.count++
	l.entries[key] = entry
	return true, 0
}

func (l *LoginLimiter) cleanup(now time.Time) {
	if len(l.entries) <= l.maxEntries {
		return
	}
	for key, entry := range l.entries {
		if now.Sub(entry.windowStart) >= l.window {
			delete(l.entries, key)
		}
	}
	if len(l.entries) > l.maxEntries {
		// Pathological cardinality: reset rather than grow without bound.
		l.entries = make(map[string]loginEntry)
	}
}
