package dm

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// KindNote is a plain text note (NIP-01): the DM's post of each update.
const KindNote = 1

// PostsFile remembers which updates the DM has posted (update id → event id),
// so each goes out once.
var PostsFile = filepath.Join("data", "dm", "posts.json")

// UpdateNote is an update as the DM posts it: the title, the summary, the
// highlights, and a link to it on the site.
func UpdateNote(u types.UpdateEntry) *nostr.Event {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(u.Icon + " " + u.Title))
	if u.Summary != "" {
		b.WriteString("\n\n" + u.Summary)
	}
	if len(u.Highlights) > 0 {
		b.WriteString("\n")
		for _, h := range u.Highlights {
			b.WriteString("\n• " + h)
		}
	}
	link := GameURL + "/#updates/" + u.ID
	b.WriteString("\n\n" + link + "\n\n#pubkeyquest")
	return &nostr.Event{Kind: KindNote, CreatedAt: time.Now().Unix(), Content: b.String(), Tags: [][]string{
		{"t", "pubkeyquest"},
		{"r", link},
	}}
}

// PublishUpdates posts each update the DM hasn't posted yet, oldest first. On
// his first run he posts only the newest, so the whole history doesn't land
// in his feed at once.
func PublishUpdates() {
	if !Signs() {
		return
	}
	updates := utils.LoadUpdates() // newest first
	if len(updates) == 0 {
		return
	}
	posted := map[string]string{}
	data, err := os.ReadFile(PostsFile)
	first := os.IsNotExist(err)
	if err == nil {
		json.Unmarshal(data, &posted)
	}
	changed := first
	if first {
		for _, u := range updates[1:] {
			posted[u.ID] = "" // before the DM posted updates
		}
	}
	for i := len(updates) - 1; i >= 0; i-- {
		u := updates[i]
		if _, done := posted[u.ID]; done || u.ID == "" {
			continue
		}
		if ev := send(UpdateNote(u)); ev != "" {
			posted[u.ID] = ev
			changed = true
			log.Printf("📣 The Dungeon Master posted the update %q", u.ID)
		} else {
			log.Printf("⚠️ Dungeon Master: the update %q reached no relay; retrying at the next sweep", u.ID)
		}
	}
	if !changed {
		return
	}
	if data, err := json.Marshal(posted); err == nil {
		os.MkdirAll(filepath.Dir(PostsFile), 0o755)
		os.WriteFile(PostsFile, data, 0o644)
	}
}
