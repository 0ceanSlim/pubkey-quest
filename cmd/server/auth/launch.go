package auth

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"pubkey-quest/cmd/server/utils"
)

// ─── Launch gate ─────────────────────────────────────────────────────────────
//
// Until the official server opens (config launch.closed), visitors see a
// "not open yet — play on the test server" page and the game API answers 503.
// Keys in launch.allow can still sign in and play, so the maintainer can check
// a release on the real server before opening it.

// openWhileClosed are paths anyone may reach while closed: assets, sign-in,
// and the public profile lookup the sign-in screen uses.
var openWhileClosed = []string{"/res/", "/dist/", "/api/auth/", "/api/profile", "/manifest.json", "/sw.js", "/pwa.js", "/favicon.ico"}

// LaunchGate wraps the server while launch.closed is set.
func LaunchGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := utils.AppConfig.Launch
		if !cfg.Closed {
			next.ServeHTTP(w, r)
			return
		}
		for _, p := range openWhileClosed {
			if strings.HasPrefix(r.URL.Path, p) {
				next.ServeHTTP(w, r)
				return
			}
		}
		// The team sign-in screen: the normal landing page, reached on purpose.
		if r.URL.Path == "/" && r.URL.Query().Has("signin") {
			next.ServeHTTP(w, r)
			return
		}
		if user := GetCurrentUser(r); user != nil && launchAllowed(user.PublicKey) {
			next.ServeHTTP(w, r)
			return
		}

		msg := cfg.Message
		if msg == "" {
			msg = "Pubkey Quest isn't open yet."
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg, "test_url": cfg.TestURL, "closed": true})
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, closedPage(msg, cfg.TestURL))
	})
}

// launchAllowed reports whether a signed-in key may play while closed.
func launchAllowed(hexKey string) bool {
	for _, k := range utils.AppConfig.Launch.Allow {
		if h, err := normalizePubkey(k); err == nil && strings.EqualFold(h, hexKey) {
			return true
		}
	}
	return false
}

func closedPage(msg, testURL string) string {
	test := ""
	if testURL != "" {
		u := html.EscapeString(testURL)
		test = `<p>Want to help test? Play on the test server — saves there are for testing and will be wiped.</p>
<p><a class="btn" href="` + u + `">Go to the test server →</a></p>`
	}
	return `<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pubkey Quest — not open yet</title>
<style>
  :root { color-scheme: dark; }
  body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
         background: #111; color: #e5e7eb; font: 15px/1.6 system-ui, sans-serif; padding: 16px; box-sizing: border-box; }
  .win { max-width: 460px; width: 100%; background: #1f1f1f; border: 2px solid; border-color: #4a4a4a #0a0a0a #0a0a0a #4a4a4a; }
  .bar { background: linear-gradient(90deg, #1e3a8a, #2563eb); color: #fff; font-weight: bold; padding: 4px 8px; }
  .body { padding: 18px 20px; }
  h1 { font-size: 20px; margin: 0 0 8px; color: #facc15; }
  .btn { display: inline-block; color: #fff; background: #15803d; padding: 6px 14px; text-decoration: none;
         border: 2px solid; border-color: #4ade80 #14532d #14532d #4ade80; }
  .small { font-size: 12px; color: #9ca3af; margin-top: 18px; }
  .small a { color: #9ca3af; }
</style></head><body>
<div class="win"><div class="bar">⚔ Pubkey Quest</div><div class="body">
<h1>Not open yet</h1>
<p>` + html.EscapeString(msg) + `</p>
` + test + `
<p class="small">On the team? <a href="/?signin=1">Sign in</a></p>
</div></div></body></html>`
}
