package session_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pubkey-quest/cmd/server/session"
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
