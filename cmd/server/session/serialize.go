package session

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// ─── One request at a time per character ─────────────────────────────────────
//
// A session's state (the save's maps, the fight, the shop counter) is changed
// by many handlers. If two requests for the same character ran at once, a
// reward could be paid twice (two "end the fight" calls both see the loot)
// and two writes to one map crash the whole server. So every API request that
// names a character slot (npub + save_id) takes that slot's lock for its
// whole run, in SerializeRequests, in front of every handler: a new handler is
// covered without remembering to lock. The lock is per slot, not per session
// object, so it also covers the request that loads the session and a reload
// that replaces it. Party moves never take another player's lock: they queue
// PartySyncs, which that player's own next request applies.

type slotLock struct {
	mu    sync.Mutex
	users int // requests holding or waiting; the entry goes when it's 0
}

var slotLocks = struct {
	sync.Mutex
	m map[string]*slotLock
}{m: map[string]*slotLock{}}

// LockSlot waits for a character slot and returns its unlock.
func LockSlot(npub, saveID string) (unlock func()) {
	key := sessionKey(npub, saveID)
	slotLocks.Lock()
	l := slotLocks.m[key]
	if l == nil {
		l = &slotLock{}
		slotLocks.m[key] = l
	}
	l.users++
	slotLocks.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		slotLocks.Lock()
		if l.users--; l.users == 0 {
			delete(slotLocks.m, key)
		}
		slotLocks.Unlock()
	}
}

// maxSerializedBody caps the body read to find the slot (the identity gate
// caps protected routes at the same size).
const maxSerializedBody = 1 << 20

// SerializeRequests runs an API request under the lock of the slot it names,
// from the query (npub, save_id) or a JSON body's top-level fields. A request
// that names no valid slot runs as it is.
func SerializeRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		npub, saveID, err := namedSlot(r)
		if err != "" {
			http.Error(w, err, http.StatusBadRequest)
			return
		}
		if ValidSlot(npub, saveID) != nil {
			next.ServeHTTP(w, r)
			return
		}
		unlock := LockSlot(npub, saveID)
		defer unlock()
		next.ServeHTTP(w, r)
	})
}

// namedSlot reads the slot a request names. The body is buffered and put back
// for the handler. A request naming two different slots (query and body) is
// refused: the handler might act on the one that isn't locked.
func namedSlot(r *http.Request) (npub, saveID, problem string) {
	q := r.URL.Query()
	npub, saveID = q.Get("npub"), q.Get("save_id")
	// /api/saves/{npub}/{save id}/… names it in the path.
	if rest, ok := strings.CutPrefix(r.URL.Path, "/api/saves/"); ok {
		if parts := strings.Split(rest, "/"); len(parts) >= 2 && ValidNpub(parts[0]) {
			npub, saveID = parts[0], parts[1]
		}
	}
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return npub, saveID, ""
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxSerializedBody+1))
	r.Body.Close()
	if err != nil || len(body) > maxSerializedBody {
		return "", "", "request body too large"
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var probe struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
	}
	if json.Unmarshal(body, &probe) == nil {
		if (npub != "" && probe.Npub != "" && probe.Npub != npub) || (saveID != "" && probe.SaveID != "" && probe.SaveID != saveID) {
			return "", "", "the request names two characters"
		}
		if probe.Npub != "" {
			npub = probe.Npub
		}
		if probe.SaveID != "" {
			saveID = probe.SaveID
		}
	}
	return npub, saveID, ""
}
