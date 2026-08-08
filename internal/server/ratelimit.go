package server

import (
	"sync"
	"time"
)

type rateEntry struct {
	started time.Time
	count   int
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		entries: make(map[string]rateEntry),
		limit:   limit,
		window:  window,
	}
}

func (limiter *rateLimiter) Allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, exists := limiter.entries[key]
	if !exists || now.Sub(entry.started) >= limiter.window {
		limiter.entries[key] = rateEntry{started: now, count: 1}
		return true
	}
	if entry.count >= limiter.limit {
		return false
	}
	entry.count++
	limiter.entries[key] = entry
	if len(limiter.entries) > 2048 {
		for candidate, value := range limiter.entries {
			if now.Sub(value.started) >= limiter.window {
				delete(limiter.entries, candidate)
			}
		}
	}
	return true
}
