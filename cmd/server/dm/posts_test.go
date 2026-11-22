package dm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// withUpdates points the update notes at a file holding these (newest first).
func withUpdates(t *testing.T, updates ...types.UpdateEntry) {
	t.Helper()
	old := utils.UpdatesPath
	utils.UpdatesPath = filepath.Join(t.TempDir(), "updates.json")
	t.Cleanup(func() { utils.UpdatesPath = old })
	data, _ := json.Marshal(map[string]any{"updates": updates})
	if err := os.WriteFile(utils.UpdatesPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateNote(t *testing.T) {
	ev := UpdateNote(types.UpdateEntry{ID: "2026-11-21-badges", Icon: "🏅", Title: "Badges and highscores",
		Summary: "The Dungeon Master now keeps track.", Highlights: []string{"43 badges", "Highscores"}})
	want := "🏅 Badges and highscores\n\nThe Dungeon Master now keeps track.\n\n• 43 badges\n• Highscores\n\n" +
		"https://pubkey.quest/#updates/2026-11-21-badges\n\n#pubkeyquest"
	if ev.Kind != KindNote || ev.Content != want {
		t.Errorf("note kind %d:\n%s", ev.Kind, ev.Content)
	}
	if tag(ev, "t")[1] != "pubkeyquest" || tag(ev, "r")[1] != "https://pubkey.quest/#updates/2026-11-21-badges" {
		t.Errorf("tags %+v", ev.Tags)
	}
}

// The first run posts only the newest update; after that each new one goes
// out once, and one no relay took is tried again.
func TestPublishUpdates(t *testing.T) {
	got := asDM(t)
	withUpdates(t, types.UpdateEntry{ID: "b", Title: "B"}, types.UpdateEntry{ID: "a", Title: "A"})

	PublishUpdates()
	if len(got.events) != 1 || !strings.Contains(got.events[0].Content, "#updates/b") {
		t.Fatalf("first run posted %d: %+v", len(got.events), got.events)
	}
	PublishUpdates()
	if len(got.events) != 1 {
		t.Fatalf("posted again: %d", len(got.events))
	}

	withUpdates(t, types.UpdateEntry{ID: "c", Title: "C"}, types.UpdateEntry{ID: "b", Title: "B"}, types.UpdateEntry{ID: "a", Title: "A"})
	got.fail = true
	PublishUpdates()
	got.fail = false
	PublishUpdates()
	if len(got.events) != 2 || !strings.Contains(got.events[1].Content, "#updates/c") {
		t.Fatalf("new update: %d posted", len(got.events))
	}
	PublishUpdates()
	if len(got.events) != 2 {
		t.Errorf("posted again: %d", len(got.events))
	}
}

// Off official (no signer) nothing is posted.
func TestPublishUpdatesUnsigned(t *testing.T) {
	got := asDM(t)
	signer = nil
	withUpdates(t, types.UpdateEntry{ID: "a", Title: "A"})
	PublishUpdates()
	if len(got.events) != 0 {
		t.Errorf("posted %d without signing", len(got.events))
	}
}
