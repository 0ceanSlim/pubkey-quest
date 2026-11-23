package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/session"
	"pubkey-quest/cmd/server/utils"
)

// Logins outlive a restart. Grain keeps login sessions in memory only, so every
// deploy signed everyone out. Each login is also kept on disk, by a hash of its
// cookie (never the cookie: the file alone signs nobody in), and when a cookie
// grain doesn't know arrives, KeepLogin makes the login again from it: a fresh
// grain session and cookie, for the same key and signer, before the request
// goes on. Logging out forgets it; a week unused (sessionMaxIdle) drops it.

// LoginsFile is where kept logins live.
var LoginsFile = filepath.Join("data", "logins.json")

// grainCookie is grain's session cookie (session.NewSessionManager).
const grainCookie = "grain-session"

// createLogin makes a grain login as HandleLogin does (and fetches the player's
// relays in the background). A test seam.
var createLogin = session.CreateUserSession

// A login, as kept.
type keptLogin struct {
	PublicKey     string                         `json:"pubkey"`
	Mode          session.SessionInteractionMode `json:"mode"`
	SigningMethod session.SigningMethod          `json:"signing_method"`
	Seen          time.Time                      `json:"seen"`
}

// A login made again from an old cookie: the old cookie's other requests (a page
// fires several at once) ride the new one too, for a moment.
type remadeLogin struct {
	token string
	at    time.Time
}

const (
	loginSeenEvery = 10 * time.Minute // how stale Seen may get before it's bumped
	remadeFor      = time.Minute
)

var (
	loginsMu     sync.Mutex
	logins       map[string]*keptLogin // by tokenHash
	loginsDirty  bool
	remadeLogins = map[string]remadeLogin{} // old tokenHash → new cookie
)

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// loadLoginsLocked reads the kept logins once. Caller holds loginsMu.
func loadLoginsLocked() {
	if logins != nil {
		return
	}
	logins = map[string]*keptLogin{}
	data, err := os.ReadFile(LoginsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("⚠️ Kept logins unreadable: %v", err)
		}
		return
	}
	if err := json.Unmarshal(data, &logins); err != nil {
		log.Printf("⚠️ Kept logins unreadable: %v", err)
		logins = map[string]*keptLogin{}
	}
}

// saveLoginsLocked writes the kept logins, dropping any unused for a week.
// Caller holds loginsMu.
func saveLoginsLocked() {
	for h, l := range logins {
		if time.Since(l.Seen) > sessionMaxIdle {
			delete(logins, h)
		}
	}
	data, err := json.Marshal(logins)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(LoginsFile), 0o700)
	}
	if err == nil {
		tmp := LoginsFile + ".tmp"
		if err = os.WriteFile(tmp, data, 0o600); err == nil {
			err = os.Rename(tmp, LoginsFile)
		}
	}
	if err != nil {
		log.Printf("❌ Couldn't keep logins: %v", err)
		return
	}
	loginsDirty = false
}

// keepLogin remembers a login under its cookie.
func keepLogin(token string, s *session.UserSession) {
	if token == "" || s == nil {
		return
	}
	loginsMu.Lock()
	defer loginsMu.Unlock()
	loadLoginsLocked()
	logins[tokenHash(token)] = &keptLogin{PublicKey: s.PublicKey, Mode: s.Mode, SigningMethod: s.SigningMethod, Seen: time.Now()}
	saveLoginsLocked()
}

// forgetLogin drops a login (logging out).
func forgetLogin(token string) {
	if token == "" {
		return
	}
	loginsMu.Lock()
	defer loginsMu.Unlock()
	loadLoginsLocked()
	delete(logins, tokenHash(token))
	saveLoginsLocked()
}

// FlushLogins writes the logins' last-seen times (hourly, and on shutdown).
func FlushLogins() {
	loginsMu.Lock()
	defer loginsMu.Unlock()
	if logins != nil && loginsDirty {
		saveLoginsLocked()
	}
}

// sessionCookie reads the session cookie a handler just set.
func sessionCookie(w http.ResponseWriter) *http.Cookie {
	for _, c := range (&http.Response{Header: w.Header()}).Cookies() {
		if c.Name == grainCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

// withSessionCookie is r carrying token as its session cookie.
func withSessionCookie(r *http.Request, token string) *http.Request {
	r2 := r.Clone(r.Context())
	cookies := r2.Cookies()
	r2.Header.Del("Cookie")
	for _, c := range cookies {
		if c.Name == grainCookie {
			c = &http.Cookie{Name: grainCookie, Value: token}
		}
		r2.AddCookie(c)
	}
	return r2
}

// keepsLogin reports whether a path can need the login: the game's pages and
// API, not its art, scripts or wiki.
func keepsLogin(path string) bool {
	for _, p := range []string{"/res/", "/dist/", "/wiki", "/favicon", "/sw.js", "/manifest"} {
		if strings.HasPrefix(path, p) {
			return false
		}
	}
	return true
}

// KeepLogin makes a kept login again when its cookie arrives after a restart,
// and notes when kept logins are used.
func KeepLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mgr := session.SessionMgr
		if mgr == nil || !keepsLogin(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		token := mgr.GetSessionToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := tokenHash(token)
		if mgr.GetUserSession(token) != nil {
			loginsMu.Lock()
			loadLoginsLocked()
			if l := logins[h]; l != nil && time.Since(l.Seen) > loginSeenEvery {
				l.Seen = time.Now()
				loginsDirty = true
			}
			loginsMu.Unlock()
			next.ServeHTTP(w, r)
			return
		}
		if fresh := remakeLogin(w, r, h); fresh != "" {
			r = withSessionCookie(r, fresh)
		}
		next.ServeHTTP(w, r)
	})
}

// remakeLogin makes the kept login for an unknown cookie again, setting its new
// cookie on w; "" when there's none to make.
func remakeLogin(w http.ResponseWriter, r *http.Request, h string) string {
	loginsMu.Lock()
	defer loginsMu.Unlock()
	loadLoginsLocked()

	// Another request with this cookie already made it again: share that.
	if re, ok := remadeLogins[h]; ok && time.Since(re.at) < remadeFor {
		if session.SessionMgr.GetUserSession(re.token) != nil {
			http.SetCookie(w, &http.Cookie{
				Name: grainCookie, Value: re.token, Path: "/", MaxAge: int(sessionMaxIdle / time.Second),
				HttpOnly: true, Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
				SameSite: http.SameSiteStrictMode,
			})
			return re.token
		}
	}
	for old, re := range remadeLogins {
		if time.Since(re.at) >= remadeFor {
			delete(remadeLogins, old)
		}
	}

	kept := logins[h]
	if kept == nil {
		return ""
	}
	delete(logins, h)
	if time.Since(kept.Seen) > sessionMaxIdle {
		saveLoginsLocked()
		return ""
	}
	// The whitelist may have changed since they logged in.
	if err := NewAuthHandler(&utils.AppConfig).checkWhitelistAccess(r, kept.PublicKey); err != nil {
		saveLoginsLocked()
		return ""
	}
	s, err := createLogin(w, r, session.SessionInitRequest{
		RequestedMode: kept.Mode,
		SigningMethod: kept.SigningMethod,
		PublicKey:     kept.PublicKey,
	})
	c := sessionCookie(w)
	if err != nil || c == nil {
		log.Printf("⚠️ Couldn't restore login for %s...: %v", kept.PublicKey[:16], err)
		saveLoginsLocked()
		return ""
	}
	logins[tokenHash(c.Value)] = &keptLogin{PublicKey: s.PublicKey, Mode: s.Mode, SigningMethod: s.SigningMethod, Seen: time.Now()}
	remadeLogins[h] = remadeLogin{token: c.Value, at: time.Now()}
	saveLoginsLocked()
	log.Printf("🔄 Login restored after a restart: %s...", kept.PublicKey[:16])
	return c.Value
}
