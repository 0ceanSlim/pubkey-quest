package auth_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0ceanslim/grain/client/core/tools"
	grainsession "github.com/0ceanslim/grain/client/session"

	"pubkey-quest/cmd/server/auth"
)

// Player-state endpoints identify the player by an npub carried in the request.
// An npub is a public identifier, so trusting it let anyone who knew a player's
// npub read, overwrite or delete that player's characters. auth.RequireOwner is
// what stops that, and these tests pin the three outcomes that matter:
// not signed in, signed in as someone else, and signed in as yourself.

// testIdentity is an arbitrary valid 32-byte hex pubkey.
const (
	ownerHex     = "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	intruderHex  = "32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245"
	sessionCooke = "session_token"
)

// signedIn returns a request carrying a live session cookie for pubkeyHex.
func signedIn(t *testing.T, pubkeyHex string, req *http.Request) *http.Request {
	t.Helper()
	if grainsession.SessionMgr == nil {
		grainsession.SessionMgr = grainsession.NewSessionManager()
	}
	rec := httptest.NewRecorder()
	// CreateSession takes the request so grain can decide the cookie's Secure
	// flag, and the user's connected relays (nil here — ownership checks only
	// read the pubkey).
	if _, err := grainsession.SessionMgr.CreateSession(rec, req, grainsession.SessionInitRequest{
		PublicKey:     pubkeyHex,
		RequestedMode: "write",
		SigningMethod: "test",
	}, nil); err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

// npubFor encodes a hex pubkey the way requests carry it.
func npubFor(t *testing.T, pubkeyHex string) string {
	t.Helper()
	npub, err := tools.EncodePubkey(pubkeyHex)
	if err != nil {
		t.Fatalf("encode pubkey: %v", err)
	}
	return npub
}

// guarded wraps a handler that records whether it ran.
func guarded(ran *bool) http.HandlerFunc {
	return auth.RequireOwner(func(w http.ResponseWriter, r *http.Request) {
		*ran = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireOwnerRejectsAnonymous(t *testing.T) {
	var ran bool
	h := guarded(&ran)

	req := httptest.NewRequest(http.MethodGet, "/api/game/state?npub="+npubFor(t, ownerHex), nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for a request with no session", rec.Code)
	}
	if ran {
		t.Error("handler ran for an unauthenticated request")
	}
}

func TestRequireOwnerRejectsSomeoneElsesNpub(t *testing.T) {
	var ran bool
	h := guarded(&ran)

	// Signed in as the owner, but asking to act as the intruder's character.
	req := httptest.NewRequest(http.MethodGet, "/api/game/state?npub="+npubFor(t, intruderHex), nil)
	req = signedIn(t, ownerHex, req)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 when claiming another account's npub", rec.Code)
	}
	if ran {
		t.Error("handler ran for a request naming someone else's npub")
	}
}

func TestRequireOwnerAllowsYourOwnNpub(t *testing.T) {
	var ran bool
	h := guarded(&ran)

	req := httptest.NewRequest(http.MethodGet, "/api/game/state?npub="+npubFor(t, ownerHex), nil)
	req = signedIn(t, ownerHex, req)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for your own npub", rec.Code)
	}
	if !ran {
		t.Error("handler did not run for a legitimate request")
	}
}

// The npub also arrives in the URL path (/api/saves/{npub}) — the route that
// could delete another player's save.
func TestRequireOwnerChecksThePathNpub(t *testing.T) {
	var ran bool
	h := guarded(&ran)

	req := httptest.NewRequest(http.MethodDelete, "/api/saves/"+npubFor(t, intruderHex)+"/save_1", nil)
	req = signedIn(t, ownerHex, req)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 when deleting another account's save", rec.Code)
	}
	if ran {
		t.Error("handler ran for a cross-account save deletion")
	}
}

// …and in a JSON body, which is how every action and combat call sends it.
func TestRequireOwnerChecksTheBodyNpub(t *testing.T) {
	var ran bool
	h := guarded(&ran)

	body := `{"npub":"` + npubFor(t, intruderHex) + `","save_id":"save_1","action":{"type":"move"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/game/action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = signedIn(t, ownerHex, req)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 when acting as another account", rec.Code)
	}
	if ran {
		t.Error("handler ran for a cross-account action")
	}
}

// Peeking at the body to find the npub must not consume it — the handler still
// has to be able to read its own payload.
func TestRequireOwnerLeavesTheBodyReadable(t *testing.T) {
	body := `{"npub":"` + npubFor(t, ownerHex) + `","save_id":"save_1"}`

	var seen string
	h := auth.RequireOwner(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("handler could not read body: %v", err)
			return
		}
		seen = string(raw)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/game/action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = signedIn(t, ownerHex, req)
	h(httptest.NewRecorder(), req)

	if seen != body {
		t.Errorf("handler read %q, want the original body %q", seen, body)
	}
}

// A request with no npub at all has nothing to impersonate; the handler's own
// validation decides. It must still require a session.
func TestRequireOwnerAllowsRequestWithoutAnNpub(t *testing.T) {
	var ran bool
	h := guarded(&ran)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms", nil)
	req = signedIn(t, ownerHex, req)
	rec := httptest.NewRecorder()
	h(rec, req)

	if !ran {
		t.Errorf("handler did not run (status %d); a request with no npub should pass through", rec.Code)
	}
}

// RequireOwner publishes the identity it verified so handlers can trust it
// instead of re-reading the body they were just protected from.
func TestOwnerNpubIsAvailableToHandlers(t *testing.T) {
	want := npubFor(t, ownerHex)

	var got string
	h := auth.RequireOwner(func(w http.ResponseWriter, r *http.Request) {
		got = auth.OwnerNpub(r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/debug/sessions", nil)
	req = signedIn(t, ownerHex, req)
	h(httptest.NewRecorder(), req)

	if got != want {
		t.Errorf("OwnerNpub = %q, want the session's npub %q", got, want)
	}
}
