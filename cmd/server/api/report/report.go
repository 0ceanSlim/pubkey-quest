package report

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxMessageLen caps the freeform text a player can submit, to keep issue
// comments and the log sane and blunt abuse.
const maxMessageLen = 4000

// reportsDir is where the growing JSONL logs live. It sits under data/ alongside
// saves — operational data, not game content, and gitignored. It is a var so
// tests can redirect it to a temp directory.
var reportsDir = "data/reports"

// logMu serializes appends so concurrent submissions don't interleave lines.
var logMu sync.Mutex

// ---------------------------------------------------------------------------
// Rate limiting — a light per-IP throttle so the unauthenticated endpoints
// can't be trivially flooded on the public test server.
// ---------------------------------------------------------------------------

const (
	rateWindow = time.Minute
	rateMax    = 5 // submissions per IP per window (across both report types)
)

var (
	rateMu   sync.Mutex
	rateHits = map[string][]time.Time{}
)

// allow reports whether the given IP is under its rate limit, recording this hit.
func allow(ip string) bool {
	rateMu.Lock()
	defer rateMu.Unlock()

	cutoff := nowFunc().Add(-rateWindow)
	kept := rateHits[ip][:0]
	for _, t := range rateHits[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rateMax {
		rateHits[ip] = kept
		return false
	}
	rateHits[ip] = append(kept, nowFunc())
	return true
}

// nowFunc is a seam for tests; production uses time.Now.
var nowFunc = time.Now

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// First hop is the original client.
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// appendLog appends one record as a JSON line to data/reports/<file>.
func appendLog(file string, record any) error {
	logMu.Lock()
	defer logMu.Unlock()

	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return fmt.Errorf("create reports dir: %w", err)
	}

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(reportsDir, file), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write log: %w", err)
	}
	return nil
}

// sanitize trims and length-caps player text.
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxMessageLen {
		s = s[:maxMessageLen]
	}
	return s
}

// shortNpub abbreviates an npub for one-line summaries.
func shortNpub(npub string) string {
	if len(npub) <= 16 {
		return npub
	}
	return npub[:12] + "…" + npub[len(npub)-4:]
}
