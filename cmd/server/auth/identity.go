package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
)

// ─── Cookie-bound identity (M5.6 G1) ─────────────────────────────────────────
//
// Game handlers identify the player by an `npub` they are handed — in the query
// string, the JSON body, or the /api/saves/{npub} path. Left alone, anyone could
// pass someone else's npub and read or play their character. RequireIdentity
// sits in front of the whole mux and, for player-state routes, insists that the
// claimed npub is the one this browser's login session proved (challenge.go).
//
// Doing it once here (instead of in ~60 handlers) means a new endpoint under a
// protected prefix is covered automatically.

// protectedPrefixes are the routes that read or change a player's own state.
// Static game data, character previews, profiles, reports and auth stay public.
var protectedPrefixes = []string{
	"/api/saves/",
	"/api/session/",
	"/api/game/",
	"/api/combat/",
	"/api/poi/",
	"/api/quests/",
	"/api/shop/",
	"/api/spells/prepare",
	"/api/spells/prep-queue",
	"/api/spells/cancel-prep",
	"/api/spells/unslot",
	"/api/progression/",
	"/api/rooms",
	"/api/character/create-save",
	"/api/debug/",
}

// exactProtected are protected paths that must match exactly (a prefix would
// also catch public siblings, e.g. /api/skills/definitions).
var exactProtected = map[string]bool{
	"/api/skills": true,
}

// maxIdentityBody caps how much of a request body is buffered to find its npub.
const maxIdentityBody = 1 << 20

func isProtectedPath(path string) bool {
	if exactProtected[path] {
		return true
	}
	for _, p := range protectedPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// RequireIdentity wraps the server's handler. On protected routes that name a
// player, it rejects the request unless a login session exists (401,
// auth_required — the client re-proves its key and retries) and the named
// player is the session's own key (403).
func RequireIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isProtectedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		claimed, err := claimedNpub(r)
		if err != nil {
			writeIdentityError(w, http.StatusBadRequest, "Invalid request body", false)
			return
		}

		// A session is required on every protected route, including one that
		// names no player: /api/debug/sessions takes no npub, and letting it
		// through unauthenticated is how it used to hand every player's save
		// data to anyone who asked.
		user := GetCurrentUser(r)
		if user == nil {
			writeIdentityError(w, http.StatusUnauthorized, "Not logged in", true)
			return
		}

		if claimed != "" {
			claimedHex, err := normalizePubkey(claimed)
			if err != nil || !strings.EqualFold(claimedHex, user.PublicKey) {
				log.Printf("🚫 Identity mismatch on %s: session %s... claimed %s", r.URL.Path, user.PublicKey[:16], claimed)
				writeIdentityError(w, http.StatusForbidden, "That character belongs to another player", false)
				return
			}
		}

		// Hand the proven identity down. Handlers that need to know who is
		// calling should read this rather than the request body — the body is
		// what we just finished distrusting.
		npub, err := tools.EncodePubkey(user.PublicKey)
		if err != nil {
			log.Printf("⚠️ Could not encode session pubkey: %v", err)
			writeIdentityError(w, http.StatusUnauthorized, "Not logged in", true)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ownerKey{}, npub)))
	})
}

// ownerKey is the private context key under which RequireIdentity stores the
// npub it proved for a request.
type ownerKey struct{}

// OwnerNpub returns the npub RequireIdentity proved for this request, or "" when
// the request did not pass through it.
func OwnerNpub(r *http.Request) string {
	npub, _ := r.Context().Value(ownerKey{}).(string)
	return npub
}

// claimedNpub finds the player a request names: /api/saves/{npub}, the `npub`
// query parameter, or a top-level `npub` field in a JSON body. The body is
// buffered and restored so the handler still reads it.
func claimedNpub(r *http.Request) (string, error) {
	if rest, ok := strings.CutPrefix(r.URL.Path, "/api/saves/"); ok {
		if npub, _, _ := strings.Cut(rest, "/"); npub != "" {
			return npub, nil
		}
	}
	if npub := r.URL.Query().Get("npub"); npub != "" {
		return npub, nil
	}
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return "", nil
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
		return "", nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxIdentityBody+1))
	r.Body.Close()
	if err != nil || len(body) > maxIdentityBody {
		return "", errBodyTooLarge
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}

	var probe struct {
		Npub string `json:"npub"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		// Not a JSON object (or malformed) — let the handler produce its own error.
		return "", nil
	}
	return probe.Npub, nil
}

var errBodyTooLarge = &identityErr{"request body too large"}

type identityErr struct{ msg string }

func (e *identityErr) Error() string { return e.msg }

func writeIdentityError(w http.ResponseWriter, status int, msg string, authRequired bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"success":       false,
		"error":         msg,
		"auth_required": authRequired,
	})
}
