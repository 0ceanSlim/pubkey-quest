package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"pubkey-quest/cmd/server/utils"
)

// ─── Rate limit (beta exploit audit E3) ──────────────────────────────────────
//
// One address gets a steady stream of API requests with room for bursts:
// far more than a player sends (a clock tick every 0.4 s, plus clicks), and
// far less than a script hammering the server. Static files and pages aren't
// counted. The address is utils.ClientIP, which a client can't forge from
// behind the proxy.

const (
	rateRefill = 20.0 // requests a second, sustained
	rateBurst  = 120  // requests at once
	rateIdle   = 10 * time.Minute
)

type bucket struct {
	tokens float64
	at     time.Time
}

var rateBuckets = struct {
	sync.Mutex
	m     map[string]*bucket
	swept time.Time
}{m: map[string]*bucket{}}

// allowRequest takes a token from the address's bucket.
func allowRequest(ip string, now time.Time) bool {
	rateBuckets.Lock()
	defer rateBuckets.Unlock()
	if now.Sub(rateBuckets.swept) > rateIdle {
		for k, b := range rateBuckets.m {
			if now.Sub(b.at) > rateIdle {
				delete(rateBuckets.m, k)
			}
		}
		rateBuckets.swept = now
	}
	b := rateBuckets.m[ip]
	if b == nil {
		b = &bucket{tokens: rateBurst, at: now}
		rateBuckets.m[ip] = b
	}
	b.tokens = min(rateBurst, b.tokens+now.Sub(b.at).Seconds()*rateRefill)
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit wraps the server's handler: an address over its API budget gets
// 429 until its bucket refills.
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !allowRequest(utils.ClientIP(r), time.Now()) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Too many requests — slow down a moment."})
			return
		}
		next.ServeHTTP(w, r)
	})
}
