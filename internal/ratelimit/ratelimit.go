// Package ratelimit provides a tiny in-memory failure counter used to slow down
// password guessing. It is intentionally simple: one entry per key (client IP),
// with a rolling lockout window. Nothing is persisted — a restart clears it.
package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	fails int
	until time.Time
}

type Guard struct {
	mu     sync.Mutex
	items  map[string]*entry
	max    int
	window time.Duration
}

// New returns a guard that locks a key after max consecutive failures for window.
func New(max int, window time.Duration) *Guard {
	return &Guard{items: make(map[string]*entry), max: max, window: window}
}

// Blocked reports whether the key is currently locked out.
func (g *Guard) Blocked(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.items[key]
	if !ok || e.fails < g.max {
		return false
	}
	if time.Now().After(e.until) {
		// The lockout expired: forget the key so the next attempt starts clean.
		delete(g.items, key)
		return false
	}
	return true
}

// Fail records a failed attempt and starts the lockout once the limit is hit.
func (g *Guard) Fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.items[key]
	if !ok {
		e = &entry{}
		g.items[key] = e
	}
	e.fails++
	if e.fails >= g.max {
		e.until = time.Now().Add(g.window)
	}
	// Opportunistic cleanup keeps the map from growing without bound.
	if len(g.items) > 4096 {
		now := time.Now()
		for k, v := range g.items {
			if !v.until.IsZero() && now.After(v.until) {
				delete(g.items, k)
			}
		}
	}
}

// Reset clears the counter after a successful sign-in.
func (g *Guard) Reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.items, key)
}

// RetryAfter reports how long the key stays locked (zero when not locked).
func (g *Guard) RetryAfter(key string) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.items[key]
	if !ok || e.fails < g.max {
		return 0
	}
	d := time.Until(e.until)
	if d < 0 {
		return 0
	}
	return d
}
