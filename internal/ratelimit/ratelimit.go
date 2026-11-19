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
	last  time.Time // most recent failure, used for decay and eviction
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
// Failures older than the window do not count toward "consecutive": a handful
// of mistyped passwords spread over weeks must not trigger the lockout.
func (g *Guard) Fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	e, ok := g.items[key]
	if !ok {
		e = &entry{}
		g.items[key] = e
	}
	if !e.last.IsZero() && now.Sub(e.last) > g.window {
		e.fails = 0
	}
	e.fails++
	e.last = now
	if e.fails >= g.max {
		e.until = now.Add(g.window)
	}
	// Opportunistic cleanup keeps the map from growing without bound. Entries
	// that never reached the lockout have a zero `until`, so they are evicted
	// on idle time alone; otherwise a flood of one-off keys lives forever.
	if len(g.items) > 4096 {
		for k, v := range g.items {
			idle := v.last.IsZero() || now.Sub(v.last) > g.window
			expired := !v.until.IsZero() && now.After(v.until)
			if (idle && v.fails < g.max) || expired {
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
