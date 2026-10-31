package routes

import "net/http"

// RegisterStatic registers static file serving routes
func RegisterStatic(mux *http.ServeMux) {
	mux.Handle("/res/", revalidate(http.StripPrefix("/res/", http.FileServer(http.Dir("www/res/")))))
	mux.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(http.Dir("www/dist/"))))
	mux.Handle("/data/", http.StripPrefix("/data/", http.FileServer(http.Dir("game-data/"))))
}

// revalidate makes browsers check with the server before reusing a file. Art
// is replaced under the same name (torch.png became the unlit torch), and
// without a header a browser guesses an old file stays fresh for days. An
// unchanged file costs a 304.
func revalidate(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

// RegisterPages registers all page routes
func RegisterPages(mux *http.ServeMux) {
	mux.HandleFunc("/", Index)
	mux.HandleFunc("/legacy-registry", LegacyRegistry)
	mux.HandleFunc("/alpha-registry", AlphaRegistry)
	mux.HandleFunc("/discover", Discover)
	mux.HandleFunc("/load-save", LoadSave)
	mux.HandleFunc("/saves", SavesHandler)
	mux.HandleFunc("/new-game", NewGameHandler)
	mux.HandleFunc("/game", GameHandler)
	mux.HandleFunc("/settings", SettingsHandler)
}
