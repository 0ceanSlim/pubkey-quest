package routes

import (
	"net/http"
	"pubkey-quest/cmd/server/utils"
)

func Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		fileServer := http.FileServer(http.Dir("www"))
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
		return
	}

	data := utils.PageData{
		Title: "discover",
		// The Wiki tab links to /wiki only on servers that serve one.
		CustomData: map[string]interface{}{
			"WikiEnabled": utils.AppConfig.Wiki.URL != "",
			"Updates":     loadUpdates(),
		},
	}

	utils.RenderTemplate(w, data, "index.html", false)
}
