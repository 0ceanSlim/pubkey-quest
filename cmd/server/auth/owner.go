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

// Request-ownership enforcement.
//
// Every player-state endpoint identifies the player by an npub carried in the
// request — in the URL path (/api/saves/{npub}), a query parameter, or a JSON
// body field. An npub is a *public* identifier, so taking it on trust means any
// caller who knows someone's npub can read, overwrite or delete that player's
// characters. RequireOwner closes that: the npub in the request must be the npub
// of the logged-in session, or the request is refused.
//
// This is deliberately a wrapper rather than a check inside each handler: there
// are ~40 player-state routes across a dozen files, and the guarantee is only
// worth anything if it is impossible to forget. Routes are opted in explicitly
// at registration (see api/routes.go) so adding a new one is a visible decision.
//
// Left unwrapped on purpose: static game data (no npub), the auth endpoints
// themselves, /api/profile (public Nostr metadata for any key) and
// /api/character/generate (a pure function of a public key — same npub always
// yields the same character, and it touches no stored state).

// maxBodyPeek caps how much of a request body is buffered while looking for an
// npub field. Action payloads are small; anything larger isn't one.
const maxBodyPeek = 1 << 20 // 1 MiB

// RequireOwner wraps a handler so it only runs when the request's npub belongs
// to the authenticated session. Responds 401 when nobody is logged in and 403
// when a logged-in caller names someone else's npub.
func RequireOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionNpub, sessionHex, ok := currentIdentity(r)
		if !ok {
			http.Error(w, "Not signed in", http.StatusUnauthorized)
			return
		}

		claimed, err := claimedNpub(r)
		if err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// An absent npub is fine: there is nothing to impersonate, and handlers
		// that need one reject it themselves.
		if claimed != "" && !sameIdentity(claimed, sessionNpub, sessionHex) {
			log.Printf("🚫 Ownership denied on %s: request claims %s, session is %s",
				r.URL.Path, abbreviate(claimed), abbreviate(sessionNpub))
			http.Error(w, "That character belongs to another account", http.StatusForbidden)
			return
		}

		// Hand the verified identity down. Handlers that need to know *who* is
		// calling should read this rather than the request body — the body is
		// what we just finished distrusting.
		next(w, r.WithContext(context.WithValue(r.Context(), ownerKey{}, sessionNpub)))
	}
}

// ownerKey is the private context key under which RequireOwner stores the
// verified npub.
type ownerKey struct{}

// OwnerNpub returns the npub RequireOwner verified for this request, or "" when
// the request did not pass through it.
func OwnerNpub(r *http.Request) string {
	npub, _ := r.Context().Value(ownerKey{}).(string)
	return npub
}

// currentIdentity returns the signed-in user's npub and hex pubkey.
func currentIdentity(r *http.Request) (npub, hexKey string, ok bool) {
	user := GetCurrentUser(r)
	if user == nil || user.PublicKey == "" {
		return "", "", false
	}
	encoded, err := tools.EncodePubkey(user.PublicKey)
	if err != nil {
		// A session we can't encode is unusable for an ownership comparison;
		// refuse rather than fall back to trusting the request.
		log.Printf("⚠️ Could not encode session pubkey: %v", err)
		return "", "", false
	}
	return encoded, user.PublicKey, true
}

// sameIdentity reports whether a claimed identifier is the session's own key.
// Requests carry npubs, but hex is accepted too so a caller using either form
// is compared correctly rather than rejected on format alone.
func sameIdentity(claimed, sessionNpub, sessionHex string) bool {
	if claimed == sessionNpub || strings.EqualFold(claimed, sessionHex) {
		return true
	}
	// Normalise an npub to hex as a last resort, covering mixed-case or
	// otherwise non-identical encodings of the same key.
	if decoded, err := tools.DecodeNpub(claimed); err == nil {
		return strings.EqualFold(decoded, sessionHex)
	}
	return false
}

// claimedNpub pulls the npub a request is asking to act as, looking in the URL
// path, the query string, and finally a JSON body field. The body is restored
// afterwards so the handler reads it as normal.
func claimedNpub(r *http.Request) (string, error) {
	// Path form: /api/saves/{npub}[/{saveID}]
	if rest := strings.TrimPrefix(r.URL.Path, "/api/saves/"); rest != r.URL.Path {
		if seg := strings.SplitN(rest, "/", 2)[0]; seg != "" {
			return seg, nil
		}
	}

	if q := r.URL.Query().Get("npub"); q != "" {
		return q, nil
	}

	if r.Body == nil {
		return "", nil
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
		return "", nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyPeek))
	if err != nil {
		return "", err
	}
	// Hand the body back untouched, whatever we found in it.
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return "", nil
	}

	var probe struct {
		Npub string `json:"npub"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		// Not an object, or malformed: no npub to check. The handler will deal
		// with its own parsing.
		return "", nil
	}
	return probe.Npub, nil
}

// abbreviate shortens a key for logging so we never write a full identifier to
// the log on a denial.
func abbreviate(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:12] + "…"
}
