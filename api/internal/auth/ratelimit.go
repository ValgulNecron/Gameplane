package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// tokenBucket is a minimal per-key rate limiter: each key has a bucket
// that refills at `rate` tokens per second up to `burst`. Every call to
// Allow drains one token; denied calls return false without touching
// the bucket.
//
// Sized for the login path, where legitimate traffic is tiny but
// attackers will fire as fast as the argon2 cost allows. Keeping this
// in-process is fine for single-replica deployments; HA setups with
// multiple API pods should replace this with Redis later.
type tokenBucket struct {
	rate  float64 // tokens per second
	burst float64

	mu      sync.Mutex
	buckets map[string]*bucket
	lastGC  time.Time
}

type bucket struct {
	tokens     float64
	lastUpdate time.Time
}

func newTokenBucket(rate, burst float64) *tokenBucket {
	return &tokenBucket{
		rate:    rate,
		burst:   burst,
		buckets: map[string]*bucket{},
		lastGC:  time.Now(),
	}
}

func (t *tokenBucket) Allow(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()

	// Opportunistic GC — buckets untouched for >5 min are dropped.
	if now.Sub(t.lastGC) > 5*time.Minute {
		for k, b := range t.buckets {
			if now.Sub(b.lastUpdate) > 5*time.Minute {
				delete(t.buckets, k)
			}
		}
		t.lastGC = now
	}

	b, ok := t.buckets[key]
	if !ok {
		b = &bucket{tokens: t.burst, lastUpdate: now}
		t.buckets[key] = b
	}
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens += elapsed * t.rate
	if b.tokens > t.burst {
		b.tokens = t.burst
	}
	b.lastUpdate = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware enforces the bucket against the client IP. `RealIP` upstream
// must rewrite RemoteAddr to the real client (chi's middleware does this).
func (t *tokenBucket) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			host = req.RemoteAddr
		}
		if !t.Allow(host) {
			w.Header().Set("Retry-After", "30")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// LoginLimiter caps login attempts per client IP. 5/min with a burst of
// 10 tolerates accidental retries but starves a brute-forcer of argon2
// cycles quickly.
var LoginLimiter = newTokenBucket(5.0/60.0, 10)

// LoginUserLimiter caps login attempts per *username*, layered on top of
// LoginLimiter. Without it, an attacker spread across a botnet would
// still burn through usernames at full speed because each IP has its
// own bucket. 3/min with a burst of 6 blunts that without tripping on
// a user fat-fingering their password.
var LoginUserLimiter = newTokenBucket(3.0/60.0, 6)

// OIDCCallbackLimiter guards /auth/oidc/callback. Each redirect back
// from the IdP triggers a token exchange + DB writes; flooding it can
// overwhelm the IdP and the local DB. Tight burst; generous refill.
var OIDCCallbackLimiter = newTokenBucket(10.0/60.0, 10)

// MutationLimiter caps authenticated writes per IP. The login bucket is
// narrow (argon2 is expensive); this one is broad — 60/min with 60
// burst — and only exists to keep a single client from pegging the
// database on a mass create/delete loop.
var MutationLimiter = newTokenBucket(1.0, 60) // 60/min refill, 60 burst

// AllowUser is a helper for rate-limit checks keyed on something other
// than the IP (e.g. username). Returns the same bool as Middleware's
// check but lets callers produce custom 429 responses.
func (t *tokenBucket) AllowUser(key string) bool { return t.Allow(key) }
