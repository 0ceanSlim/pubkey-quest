package session

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

// Session journaling — server crash recovery, NOT a player save.
//
// The in-memory session is the authoritative game state during play. A deliberate
// player save (roadmap §4) writes to data/saves/ and is the only thing that
// survives a *clean* quit. Journaling is the separate, server-only safety net: it
// periodically snapshots active sessions to data/sessions/ so an *unexpected*
// server crash doesn't eat an evening of progress. Journals are never signed,
// never leave the server, and are removed on every clean transition (deliberate
// save, reload, clean quit) — so a journal that's still present at load time
// means the server died with unsaved progress, and we restore from it.

// JournalDir is where crash-recovery snapshots live (a var so tests can redirect it).
var JournalDir = "data/sessions"

// journalEntry is the on-disk crash-recovery snapshot for one session.
type journalEntry struct {
	Npub    string         `json:"npub"`
	SaveID  string         `json:"save_id"`
	SavedAt int64          `json:"saved_at"`
	Save    types.SaveFile `json:"save"`
	// Ground is what the player had dropped since their last save, at the
	// same moment: the save above still lacks it, so it comes back with it.
	Ground []world.Pile `json:"ground,omitempty"`
	// Stats are the lifetime stats the play since the last save added, which
	// count once a save covers them.
	Stats []stats.Op `json:"stats,omitempty"`
}

// journalPath builds a flat, filesystem-safe path for a session's journal file.
func journalPath(npub, saveID string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")
	return filepath.Join(JournalDir, safe.Replace(npub)+"__"+safe.Replace(saveID)+".json")
}

// WriteJournal snapshots a session's game state to the journal directory.
func WriteJournal(sess *GameSession) error {
	entry := journalEntry{
		Npub:    sess.Npub,
		SaveID:  sess.SaveID,
		SavedAt: time.Now().Unix(),
		Save:    sess.SaveData,
		Ground:  sess.Ground.Snapshot(),
		Stats:   sess.Stats.Snapshot(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(JournalDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(journalPath(sess.Npub, sess.SaveID), data, 0644)
}

// RemoveJournal deletes a session's journal. Called on every clean transition
// (deliberate save, reload, clean quit) so a remaining journal implies a crash.
func RemoveJournal(npub, saveID string) {
	if err := os.Remove(journalPath(npub, saveID)); err != nil && !os.IsNotExist(err) {
		log.Printf("⚠️ Failed to remove session journal %s:%s: %v", npub, saveID, err)
	}
}

// RecoverJournaledSave returns the journaled SaveData for a session when a journal
// exists and is at least as new as the on-disk save (i.e. it holds crash-recovered
// progress past the last deliberate save). Returns nil when there's nothing to
// recover. diskSavePath is the player's deliberate-save file.
func RecoverJournaledSave(npub, saveID, diskSavePath string) *types.SaveFile {
	entry := readJournal(npub, saveID, diskSavePath)
	if entry == nil {
		return nil
	}
	save := entry.Save
	save.InternalNpub = npub
	save.InternalID = saveID
	return &save
}

// recoveredGround is the player's own drops kept in the journal that
// RecoverJournaledSave restores a session from (none if it doesn't).
func recoveredGround(npub, saveID, diskSavePath string) []world.Pile {
	if entry := readJournal(npub, saveID, diskSavePath); entry != nil {
		return entry.Ground
	}
	return nil
}

// recoveredStats is the pending stats kept in the journal that
// RecoverJournaledSave restores a session from (none if it doesn't).
func recoveredStats(npub, saveID, diskSavePath string) []stats.Op {
	if entry := readJournal(npub, saveID, diskSavePath); entry != nil {
		return entry.Stats
	}
	return nil
}

// readJournal reads a session's journal if it's at least as new as the
// deliberate save.
func readJournal(npub, saveID, diskSavePath string) *journalEntry {
	jPath := journalPath(npub, saveID)
	jInfo, err := os.Stat(jPath)
	if err != nil {
		return nil // no journal → nothing to recover
	}
	// Don't resurrect a journal older than the last deliberate save.
	if dInfo, derr := os.Stat(diskSavePath); derr == nil && jInfo.ModTime().Before(dInfo.ModTime()) {
		return nil
	}
	data, err := os.ReadFile(jPath)
	if err != nil {
		return nil
	}
	var entry journalEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		log.Printf("⚠️ Corrupt session journal %s — ignoring: %v", jPath, err)
		return nil
	}
	return &entry
}

// JournalAllSessions snapshots every active session for crash recovery.
func JournalAllSessions() {
	for _, sess := range GetSessionManager().GetAllSessions() {
		// Under the slot's lock, so the save isn't read while a request
		// changes it.
		unlock := LockSlot(sess.Npub, sess.SaveID)
		if err := WriteJournal(sess); err != nil {
			log.Printf("⚠️ Failed to journal session %s:%s: %v", sess.Npub, sess.SaveID, err)
		}
		unlock()
	}
	// The shared ground after the journals: a crash between them can lose a
	// pile, never make two of one (world.SaveWorldGround).
	world.SaveWorldGround()
}

// StartJournalLoop launches a goroutine that snapshots active sessions on the
// given interval. Call once at startup.
func StartJournalLoop(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			JournalAllSessions()
		}
	}()
	log.Printf("📓 Session journaling every %s → %s/", interval, JournalDir)
}
