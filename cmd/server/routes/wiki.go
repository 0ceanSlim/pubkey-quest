package routes

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"pubkey-quest/cmd/server/utils"
)

// RegisterWiki forwards /wiki/ to the Gollum wiki when config wiki.url is set.
// Gollum runs with --base-path wiki, so paths pass through unchanged.
func RegisterWiki(mux *http.ServeMux) {
	raw := utils.AppConfig.Wiki.URL
	if raw == "" {
		return
	}
	target, err := url.Parse(raw)
	if err != nil || target.Host == "" {
		log.Printf("⚠️ config wiki.url %q is not a URL — /wiki disabled", raw)
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.SetXForwarded()
		},
		// Gollum doesn't version the theme files (custom.css/js), so have browsers
		// revalidate them — otherwise a theme update hides behind a stale cache.
		ModifyResponse: func(resp *http.Response) error {
			if p := resp.Request.URL.Path; strings.HasSuffix(p, "/custom.css") || strings.HasSuffix(p, "/custom.js") {
				resp.Header.Set("Cache-Control", "no-cache")
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("❌ Wiki unreachable at %s: %v", raw, err)
			http.Error(w, "The wiki is unavailable right now.", http.StatusBadGateway)
		},
	}
	mux.Handle("/wiki/", proxy)
	mux.Handle("/wiki", http.RedirectHandler("/wiki/", http.StatusMovedPermanently))
	log.Printf("📂 Wiki at /wiki → %s", raw)
}
