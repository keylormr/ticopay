package api

import (
	"sync"
	"time"
)

// loginGuard locks an account after too many failed login attempts.
// In-memory (Render free runs a single instance); resets on restart.
type loginGuard struct {
	mu    sync.Mutex
	fails map[string]*attemptInfo
}

type attemptInfo struct {
	count int
	until time.Time
}

const (
	maxLoginFails   = 5
	loginLockWindow = 15 * time.Minute
)

var loginAttempts = &loginGuard{fails: map[string]*attemptInfo{}}

// locked reports whether the key is currently locked out.
func (g *loginGuard) locked(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	a := g.fails[key]
	if a == nil {
		return false
	}
	if a.count >= maxLoginFails {
		if time.Now().Before(a.until) {
			return true
		}
		delete(g.fails, key) // window passed, forgive
	}
	return false
}

func (g *loginGuard) fail(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	a := g.fails[key]
	if a == nil {
		a = &attemptInfo{}
		g.fails[key] = a
	}
	a.count++
	if a.count >= maxLoginFails {
		a.until = time.Now().Add(loginLockWindow)
	}
}

func (g *loginGuard) reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.fails, key)
}
