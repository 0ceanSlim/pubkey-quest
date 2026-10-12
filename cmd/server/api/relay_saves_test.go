package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/api/game"
	"pubkey-quest/cmd/server/ledger"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// npub1… for a 32-byte key of all 0x01 bytes — any valid npub works here.
const testNpub = "npub1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqs8j9gdm"

// inTempSaves runs the test from a temp dir so session.SavesDirectory and the
// ledger point at scratch space.
func inTempSaves(t *testing.T) {
	t.Helper()
	wd, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	ledger.Dir = filepath.Join(dir, "data", "ledger")
	t.Cleanup(func() { os.Chdir(wd) })
	os.MkdirAll(filepath.Join(session.SavesDirectory, testNpub), 0o755)
}

func writeDiskSave(t *testing.T, saveID, name string) {
	t.Helper()
	if err := session.WriteSaveFile(session.GetSavePath(testNpub, saveID), &types.SaveFile{D: name}); err != nil {
		t.Fatal(err)
	}
}

func writeCachedEvent(t *testing.T, saveID, id string) {
	t.Helper()
	raw, _ := json.Marshal(&nostr.Event{ID: id, Kind: game.SaveKind, Tags: [][]string{{"d", game.SaveDTag(saveID)}}})
	os.WriteFile(eventPath(testNpub, saveID), raw, 0o644)
}

func TestOfficialLoadRules(t *testing.T) {
	inTempSaves(t)
	prev := utils.AppConfig.Saves
	utils.AppConfig.Saves = utils.SavesConfig{Relay: true, Validation: true, MaxSlots: 5}
	defer func() { utils.AppConfig.Saves = prev }()

	// A brand-new character (server-made, never signed) is playable.
	writeDiskSave(t, "save_1", "Fresh")
	if s, err := relaySaveSource(testNpub, "save_1"); err != nil || s.D != "Fresh" {
		t.Fatalf("new character: %v %+v", err, s)
	}

	// Once it has a signed history, it needs a ledger entry.
	writeCachedEvent(t, "save_1", "aaaa")
	if _, err := relaySaveSource(testNpub, "save_1"); err == nil {
		t.Fatal("a signed save this server never accepted loaded anyway")
	}

	// The cache matching the ledger loads straight from disk.
	ledger.Record(testNpub, game.SaveDTag("save_1"), ledger.Entry{EventID: "aaaa", Relays: []string{"wss://r"}})
	if s, err := relaySaveSource(testNpub, "save_1"); err != nil || s.D != "Fresh" {
		t.Fatalf("matching cache: %v", err)
	}

	// A cache that doesn't match (edited on disk, or a different copy) is not
	// trusted: the accepted event must be fetched — no relay client here, so
	// it fails rather than loading the wrong copy.
	writeCachedEvent(t, "save_1", "bbbb")
	if _, err := relaySaveSource(testNpub, "save_1"); err == nil || !strings.Contains(err.Error(), "couldn't be fetched") {
		t.Errorf("mismatched cache should not load: %v", err)
	}
}

func TestSameTags(t *testing.T) {
	a := [][]string{{"d", "x"}, {"client", "Pubkey Quest"}}
	if !sameTags(a, [][]string{{"d", "x"}, {"client", "Pubkey Quest"}}) {
		t.Error("identical tags should match")
	}
	if sameTags(a, [][]string{{"d", "x"}, {"client", "Other"}}) || sameTags(a, a[:1]) {
		t.Error("changed tags should not match")
	}
}
