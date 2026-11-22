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
	"/api/publish",
	"/api/relays",
	"/api/social/",
	"/api/party/",
	"/api/friends",
	"/api/mutes",
	"/api/dm/",
}

// exactProtected are protected paths that must match exactly (a prefix would
// also catch public siblings, e.g. /api/skills/definitions).
var exactProtected = map[string]bool{
	"/api/skills": true,
	"/api/badges": true, // a player's own badges; /api/highscores is public
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

		claimed, err := claimedNpubs(r)
		if err != nil {
			writeIdentityError(w, http.StatusBadRequest, "Invalid request body", false)
			return
		}

		// A session is required on every protected route, including one that
		// names no player. Some carry no npub at all (the debug tools), and
		// letting those through unauthenticated is how a debug endpoint once
		// handed every player's save data to anyone who asked.
		user := GetCurrentUser(r)
		if user == nil {
			writeIdentityError(w, http.StatusUnauthorized, "Not logged in", true)
			return
		}

		for _, c := range claimed {
			claimedHex, err := normalizePubkey(c)
			if err != nil || !strings.EqualFold(claimedHex, user.PublicKey) {
				log.Printf("🚫 Identity mismatch on %s: session %s... claimed %s", r.URL.Path, user.PublicKey[:16], c)
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

// claimedNpubs finds every player a request names: /api/saves/{npub}, the
// `npub` query parameter, and a top-level `npub` field in the body. Every one
// must be the caller: a handler may read any of them, so checking only the
// first (or skipping a body whose Content-Type isn't JSON, which handlers
// decode anyway) once let a player act as someone else. The body is buffered
// and restored so the handler still reads it, and capped whatever its type.
func claimedNpubs(r *http.Request) ([]string, error) {
	var out []string
	// /api/saves/{npub}/… names the player in the path — but /api/saves/publish
	// is an action, not a player, so only a key-shaped segment counts.
	if rest, ok := strings.CutPrefix(r.URL.Path, "/api/saves/"); ok {
		if npub, _, _ := strings.Cut(rest, "/"); looksLikeKey(npub) {
			out = append(out, npub)
		}
	}
	for _, npub := range r.URL.Query()["npub"] {
		out = append(out, npub)
	}
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return out, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxIdentityBody+1))
	r.Body.Close()
	if err != nil || len(body) > maxIdentityBody {
		return nil, errBodyTooLarge
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return out, nil
	}

	// Decoded the way a handler would, ignoring Content-Type. A body that
	// isn't a JSON object names no one; the handler makes its own error.
	var probe map[string]json.RawMessage
	if json.Unmarshal(body, &probe) != nil {
		return out, nil
	}
	for k, raw := range probe {
		if !strings.EqualFold(k, "npub") { // encoding/json matches field names case-insensitively
			continue
		}
		var npub string
		if json.Unmarshal(raw, &npub) != nil {
			return nil, errBadNpub
		}
		if npub != "" {
			out = append(out, npub)
		}
	}
	return out, nil
}

var errBadNpub = &identityErr{"npub must be a string"}

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

// looksLikeKey reports whether a path segment is a player key (npub or hex)
// rather than a route word like "publish".
func looksLikeKey(s string) bool {
	if strings.HasPrefix(s, "npub1") {
		return true
	}
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
