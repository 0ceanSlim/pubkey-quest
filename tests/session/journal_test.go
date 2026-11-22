package session_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0ceanslim/grain/client/core/tools"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

func TestJournalRoundTrip(t *testing.T) {
	session.JournalDir = t.TempDir()
	npub, saveID := "npub1test", "save_123"

	sess := &session.GameSession{
		Npub:     npub,
		SaveID:   saveID,
		SaveData: types.SaveFile{Experience: 4242, HP: 17, Class: "Rogue"},
	}
	if err := session.WriteJournal(sess); err != nil {
		t.Fatalf("WriteJournal: %v", err)
	}

	// No deliberate save on disk → the journal is recoverable.
	rec := session.RecoverJournaledSave(npub, saveID, filepath.Join(t.TempDir(), "nope.json"))
	if rec == nil || rec.Experience != 4242 || rec.HP != 17 {
		t.Fatalf("recover = %+v, want XP 4242 / HP 17", rec)
	}
	if rec.InternalNpub != npub || rec.InternalID != saveID {
		t.Errorf("internal ids not populated: %q / %q", rec.InternalNpub, rec.InternalID)
	}

	// Removed on a clean transition → nothing left to recover.
	session.RemoveJournal(npub, saveID)
	if rec := session.RecoverJournaledSave(npub, saveID, "nope.json"); rec != nil {
		t.Errorf("recover after remove = %+v, want nil", rec)
	}
}

// A journal older than the last deliberate save must not be resurrected.
func TestJournalNotNewerThanDisk(t *testing.T) {
	session.JournalDir = t.TempDir()
	npub, saveID := "npub1m", "save_m"
	if err := session.WriteJournal(&session.GameSession{Npub: npub, SaveID: saveID, SaveData: types.SaveFile{Experience: 7}}); err != nil {
		t.Fatal(err)
	}

	diskPath := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(diskPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Disk save newer than the journal → don't recover.
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(diskPath, future, future)
	if rec := session.RecoverJournaledSave(npub, saveID, diskPath); rec != nil {
		t.Errorf("disk newer than journal: want nil, got %+v", rec)
	}

	// Disk save older than the journal (crash after progress) → recover.
	past := time.Now().Add(-time.Hour)
	_ = os.Chtimes(diskPath, past, past)
	if rec := session.RecoverJournaledSave(npub, saveID, diskPath); rec == nil || rec.Experience != 7 {
		t.Errorf("disk older than journal: want recovered XP 7, got %+v", rec)
	}
}

// What was dropped since the last save is journaled with the play it came
// from: a session recovered from the journal gets its drops back, still its
// own; one loaded from the deliberate save doesn't.
func TestJournalKeepsOwnDrops(t *testing.T) {
	session.JournalDir = t.TempDir()
	hex := "7e7e9c42a91bfef19fa929e5fda1b72e0ebc1a4c1141673e2794234d86addf4e"
	npub, err := tools.EncodePubkey(hex)
	if err != nil {
		t.Fatal(err)
	}
	saveID := "save_77"
	here := world.Spot{Key: "kingdom|center||"}

	sess := &session.GameSession{Npub: npub, SaveID: saveID, Ground: world.NewGroundStore(),
		SaveData: types.SaveFile{Location: "kingdom", District: "center"}}
	sess.Ground.Add(here, "torch", 2)
	if err := session.WriteJournal(sess); err != nil {
		t.Fatal(err)
	}
	load := func(string, string) (*types.SaveFile, error) { return &types.SaveFile{}, nil }

	got, err := session.NewSessionManager().LoadSession(npub, saveID, load, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if l := got.Ground.List(here, npub); len(l) != 1 || l[0].Quantity != 2 || !l[0].Yours {
		t.Fatalf("recovered drops: %+v", l)
	}

	session.RemoveJournal(npub, saveID) // a clean quit
	got, _ = session.NewSessionManager().LoadSession(npub, saveID, load, nil, nil, nil)
	if l := got.Ground.List(here, npub); len(l) != 0 {
		t.Fatalf("drops came back without the play they belonged to: %+v", l)
	}
}

// The stats play added since the last save are journaled with it, and come
// back with it after a crash; a clean load doesn't bring them.
func TestJournalKeepsPendingStats(t *testing.T) {
	session.JournalDir = t.TempDir()
	oldOn, oldDir := stats.Enabled, stats.Dir
	stats.Enabled, stats.Dir = true, t.TempDir()
	t.Cleanup(func() { stats.Enabled, stats.Dir = oldOn, oldDir })

	npub, err := tools.EncodePubkey("7e7e9c42a91bfef19fa929e5fda1b72e0ebc1a4c1141673e2794234d86addf4e")
	if err != nil {
		t.Fatal(err)
	}
	saveID := "save_78"
	load := func(string, string) (*types.SaveFile, error) { return &types.SaveFile{}, nil }

	sess, err := session.NewSessionManager().LoadSession(npub, saveID, load, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stats.Count(&sess.SaveData, "fish_landed", 3)
	if err := session.WriteJournal(sess); err != nil {
		t.Fatal(err)
	}

	got, err := session.NewSessionManager().LoadSession(npub, saveID, load, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stats.Commit(npub, saveID, got.Stats.Mark())
	if r, _ := stats.Load(npub); r.Counters["fish_landed"] != 3 {
		t.Fatalf("after the crash, fish landed %d, want 3", r.Counters["fish_landed"])
	}

	stats.Count(&got.SaveData, "fish_landed", 5)
	session.RemoveJournal(npub, saveID) // a clean quit
	got, _ = session.NewSessionManager().LoadSession(npub, saveID, load, nil, nil, nil)
	stats.Commit(npub, saveID, got.Stats.Mark())
	if r, _ := stats.Load(npub); r.Counters["fish_landed"] != 3 {
		t.Fatalf("stats came back without the play they belonged to: %d", r.Counters["fish_landed"])
	}
}
