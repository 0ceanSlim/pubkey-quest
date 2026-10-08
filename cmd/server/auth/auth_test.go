package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"
)

// signedProof builds a login proof over a freshly issued challenge.
func signedProof(t *testing.T, signer *core.EventSigner, mutate func(*nostr.Event)) *nostr.Event {
	t.Helper()
	nonce, err := challenges.issue()
	if err != nil {
		t.Fatal(err)
	}
	ev := &nostr.Event{
		PubKey:    signer.PublicKey(),
		CreatedAt: time.Now().Unix(),
		Kind:      LoginProofKind,
		Tags:      [][]string{{"u", "https://test.pubkey.quest/api/auth/login"}, {"method", "POST"}, {"challenge", nonce}},
	}
	if mutate != nil {
		mutate(ev)
	}
	if err := signer.SignEvent(ev); err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestVerifyLoginProof(t *testing.T) {
	alice, _ := core.NewEventSignerFromRandom()
	bob, _ := core.NewEventSignerFromRandom()

	ok := signedProof(t, alice, nil)
	if err := verifyLoginProof(ok, alice.PublicKey(), time.Now()); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
	if err := verifyLoginProof(ok, alice.PublicKey(), time.Now()); err == nil {
		t.Error("replayed proof accepted")
	}

	cases := map[string]*nostr.Event{
		"other key's proof": signedProof(t, bob, nil),
		"wrong kind":        signedProof(t, alice, func(e *nostr.Event) { e.Kind = 1 }),
		"stale":             signedProof(t, alice, func(e *nostr.Event) { e.CreatedAt -= 3600 }),
		"wrong url":         signedProof(t, alice, func(e *nostr.Event) { e.Tags[0][1] = "https://evil.example/api/x" }),
		"unknown challenge": signedProof(t, alice, func(e *nostr.Event) { e.Tags[2][1] = "deadbeef" }),
	}
	for name, ev := range cases {
		if err := verifyLoginProof(ev, alice.PublicKey(), time.Now()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	tampered := signedProof(t, alice, nil)
	tampered.Content = "changed after signing"
	if err := verifyLoginProof(tampered, alice.PublicKey(), time.Now()); err == nil {
		t.Error("tampered proof accepted")
	}
	if err := verifyLoginProof(nil, alice.PublicKey(), time.Now()); err == nil {
		t.Error("missing proof accepted")
	}
}

func TestRequireIdentity(t *testing.T) {
	session.SessionMgr = session.NewSessionManager()
	alice, _ := core.NewEventSignerFromRandom()
	bob, _ := core.NewEventSignerFromRandom()
	aliceNpub, _ := tools.EncodePubkey(alice.PublicKey())
	bobNpub, _ := tools.EncodePubkey(bob.PublicKey())

	rec := httptest.NewRecorder()
	if _, err := session.SessionMgr.CreateSession(rec, httptest.NewRequest("GET", "/", nil), session.SessionInitRequest{
		PublicKey: alice.PublicKey(), RequestedMode: session.WriteMode, SigningMethod: session.BrowserExtension,
	}, nil); err != nil {
		t.Fatal(err)
	}
	cookie := rec.Result().Cookies()[0]

	var gotBody string
	h := RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	do := func(method, target, body string, withCookie bool) int {
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, target, rd)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if withCookie {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}

	cases := []struct {
		name, method, target, body string
		cookie                     bool
		want                       int
	}{
		{"own npub in query", "GET", "/api/session/state?npub=" + aliceNpub, "", true, 200},
		{"own hex in body", "POST", "/api/game/action", `{"npub":"` + alice.PublicKey() + `","action":"x"}`, true, 200},
		{"someone else's npub", "POST", "/api/combat/action", `{"npub":"` + bobNpub + `"}`, true, 403},
		{"someone else's saves", "GET", "/api/saves/" + bobNpub, "", true, 403},
		{"no session", "GET", "/api/session/state?npub=" + aliceNpub, "", false, 401},
		{"public route", "GET", "/api/profile?npub=" + bobNpub, "", false, 200},
		{"public sibling of exact route", "GET", "/api/skills/definitions?npub=" + bobNpub, "", false, 200},
		{"names no player, signed in", "POST", "/api/game/action", `{"action":"x"}`, true, 200},
		{"names no player, anonymous", "POST", "/api/game/action", `{"action":"x"}`, false, 401},
		{"npub-less protected route still needs a session", "GET", "/api/debug/state", "", false, 401},
	}
	for _, c := range cases {
		if got := do(c.method, c.target, c.body, c.cookie); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}

	// The handler still sees the body the middleware inspected.
	do("POST", "/api/game/action", `{"npub":"`+aliceNpub+`","action":"move"}`, true)
	if !strings.Contains(gotBody, `"action":"move"`) {
		t.Errorf("handler lost the request body: %q", gotBody)
	}
}

// RequireIdentity publishes the identity it proved, so a handler can scope its
// answer to the caller instead of re-reading the body it was just protected
// from — the body being what the gate just finished distrusting.
func TestRequireIdentityPublishesTheProvenNpub(t *testing.T) {
	session.SessionMgr = session.NewSessionManager()
	alice, _ := core.NewEventSignerFromRandom()
	wantNpub, _ := tools.EncodePubkey(alice.PublicKey())

	rec := httptest.NewRecorder()
	if _, err := session.SessionMgr.CreateSession(rec, httptest.NewRequest("GET", "/", nil), session.SessionInitRequest{
		PublicKey: alice.PublicKey(), RequestedMode: session.WriteMode, SigningMethod: session.BrowserExtension,
	}, nil); err != nil {
		t.Fatal(err)
	}

	var got string
	h := RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = OwnerNpub(r)
	}))

	req := httptest.NewRequest("GET", "/api/debug/state", nil)
	req.AddCookie(rec.Result().Cookies()[0])
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != wantNpub {
		t.Errorf("OwnerNpub = %q, want the session's npub %q", got, wantNpub)
	}

	// A request that never passed the gate has no proven identity to report.
	if leaked := OwnerNpub(httptest.NewRequest("GET", "/api/profile", nil)); leaked != "" {
		t.Errorf("OwnerNpub = %q on an ungated request, want empty", leaked)
	}
}
