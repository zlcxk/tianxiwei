package api

import (
	"sync"
	"time"
)

type rateEntry struct {
	count   int
	resetAt time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: map[string]*rateEntry{}}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.entries) > 5000 {
		for k, e := range l.entries {
			if now.After(e.resetAt) {
				delete(l.entries, k)
			}
		}
	}
	e, ok := l.entries[key]
	if !ok || now.After(e.resetAt) {
		l.entries[key] = &rateEntry{count: 1, resetAt: now.Add(window)}
		return true
	}
	if e.count >= limit {
		return false
	}
	e.count++
	return true
}

func (l *rateLimiter) reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}
