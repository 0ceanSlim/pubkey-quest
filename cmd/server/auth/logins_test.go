package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0ceanslim/grain/client/session"
)

// A login survives a restart: after one, the old cookie still signs the player
// in (with a new cookie), for every request a page fires at once, and the file
// holds no cookie. Logging out ends it for good.
func TestLoginOutlivesRestart(t *testing.T) {
	LoginsFile = filepath.Join(t.TempDir(), "logins.json")
	createLogin = func(w http.ResponseWriter, r *http.Request, req session.SessionInitRequest) (*session.UserSession, error) {
		return session.SessionMgr.CreateSession(w, r, req, nil)
	}
	t.Cleanup(func() { createLogin = session.CreateUserSession })
	restart := func() {
		FlushLogins()
		session.SessionMgr = session.NewSessionManager()
		loginsMu.Lock()
		logins, remadeLogins = nil, map[string]remadeLogin{}
		loginsMu.Unlock()
	}
	restart()

	pub := strings.Repeat("ab", 32)
	login := httptest.NewRecorder()
	s, err := session.SessionMgr.CreateSession(login, httptest.NewRequest("POST", "/api/auth/login", nil), session.SessionInitRequest{
		PublicKey: pub, RequestedMode: session.WriteMode, SigningMethod: "amber",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	old := sessionCookie(login).Value
	keepLogin(old, s)
	if raw, _ := os.ReadFile(LoginsFile); strings.Contains(string(raw), old) {
		t.Fatal("the cookie itself is on disk")
	}

	restart()

	// Who each request is signed in as, as the handlers see it.
	seen := func(cookie string) (who string, rec *httptest.ResponseRecorder) {
		rec = httptest.NewRecorder()
		KeepLogin(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			if u := session.SessionMgr.GetCurrentUser(r); u != nil {
				who = u.PublicKey + "/" + string(u.SigningMethod)
			}
		})).ServeHTTP(rec, func() *http.Request {
			r := httptest.NewRequest("GET", "/api/session/state", nil)
			r.AddCookie(&http.Cookie{Name: grainCookie, Value: cookie})
			return r
		}())
		return who, rec
	}

	var wg sync.WaitGroup
	got := make([]string, 4)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], _ = seen(old)
		}(i)
	}
	wg.Wait()
	for i, who := range got {
		if who != pub+"/amber" {
			t.Fatalf("request %d after the restart: signed in as %q", i, who)
		}
	}

	// The browser takes the new cookie; it works on, and survives another restart.
	_, rec := seen(old)
	fresh := sessionCookie(rec)
	if fresh == nil || fresh.Value == old {
		t.Fatalf("no new cookie: %v", fresh)
	}
	restart()
	if who, _ := seen(fresh.Value); who != pub+"/amber" {
		t.Fatalf("after a second restart: %q", who)
	}

	// A cookie nobody kept stays signed out.
	if who, _ := seen("not-a-login"); who != "" {
		t.Fatalf("an unknown cookie signed in as %q", who)
	}

	// Logging out forgets it, restart or not.
	_, rec = seen(fresh.Value)
	current := fresh.Value
	if c := sessionCookie(rec); c != nil {
		current = c.Value
	}
	out := httptest.NewRequest("POST", "/api/auth/logout", nil)
	out.AddCookie(&http.Cookie{Name: grainCookie, Value: current})
	(&AuthHandler{}).HandleLogout(httptest.NewRecorder(), out)
	restart()
	if who, _ := seen(current); who != "" {
		t.Fatalf("still signed in after logging out: %q", who)
	}

	// A week unused, and it's gone.
	keepLogin(old, s)
	loginsMu.Lock()
	logins[tokenHash(old)].Seen = time.Now().Add(-sessionMaxIdle - time.Hour)
	loginsDirty = true
	loginsMu.Unlock()
	restart()
	if who, _ := seen(old); who != "" {
		t.Fatalf("a stale login came back: %q", who)
	}
}
