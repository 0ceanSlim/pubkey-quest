package routes

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	serverdb "pubkey-quest/cmd/server/db"
)

// Generated items (fish, gems) share a sprite between many ids: every size of
// a species, every size and quality of a gem. The client mostly draws an item
// as /res/img/items/<id>.png, so a request for an id with no file of its own
// is answered with the sprite its item names ("sprite", else "image").

var itemSprites sync.Map // id → file path under www/res ("" for none)

// itemSprite is the file to serve for an item id that has no sprite of its own.
func itemSprite(id string) string {
	if v, ok := itemSprites.Load(id); ok {
		return v.(string)
	}
	file := ""
	if it, err := serverdb.GetItemByID(id); err == nil && it != nil {
		var props struct {
			Sprite string `json:"sprite"`
			Image  string `json:"image"`
		}
		_ = json.Unmarshal([]byte(it.Properties), &props)
		src := props.Sprite
		if src == "" {
			src = props.Image
		}
		src, _, _ = strings.Cut(src, "#")
		if rel, ok := strings.CutPrefix(src, "/res/"); ok && rel != "img/items/"+id+".png" && !strings.Contains(rel, "..") {
			file = path.Join("www/res", rel)
		}
	}
	itemSprites.Store(id, file)
	return file
}

// itemSpriteFallback serves a generated item's shared sprite when its own
// file doesn't exist.
func itemSpriteFallback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, ok := strings.CutPrefix(r.URL.Path, "/res/img/items/"); ok && strings.HasSuffix(name, ".png") && !strings.Contains(name, "/") {
			if _, err := os.Stat(path.Join("www/res/img/items", name)); os.IsNotExist(err) {
				if file := itemSprite(strings.TrimSuffix(name, ".png")); file != "" {
					w.Header().Set("Cache-Control", "no-cache")
					http.ServeFile(w, r, file)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
