package auth

import (
	"strings"
	"sync"
	"time"
)

// LoginLimiter throttles repeated failed logins per email address: after
// maxFailures within window, further attempts are refused until the window
// since the first failure elapses. It is in-memory, which is fine for a
// single-instance household app; a restart simply forgets the counts.
type LoginLimiter struct {
	mu          sync.Mutex
	entries     map[string]*limitEntry
	maxFailures int
	window      time.Duration
}

type limitEntry struct {
	failures int
	first    time.Time
}

func NewLoginLimiter(maxFailures int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{entries: map[string]*limitEntry{}, maxFailures: maxFailures, window: window}
}

// Allowed reports whether a login attempt for key may proceed.
func (l *LoginLimiter) Allowed(key string) bool {
	key = strings.ToLower(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil {
		return true
	}
	if time.Since(e.first) > l.window {
		delete(l.entries, key)
		return true
	}
	return e.failures < l.maxFailures
}

// Fail records a failed attempt.
func (l *LoginLimiter) Fail(key string) {
	key = strings.ToLower(key)
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil || time.Since(e.first) > l.window {
		e = &limitEntry{first: time.Now()}
		l.entries[key] = e
	}
	e.failures++
}

// Reset clears the count after a successful login.
func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, strings.ToLower(key))
}
