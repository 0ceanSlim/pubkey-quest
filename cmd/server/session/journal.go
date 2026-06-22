package session

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	save := entry.Save
	save.InternalNpub = npub
	save.InternalID = saveID
	return &save
}

// JournalAllSessions snapshots every active session for crash recovery.
func JournalAllSessions() {
	for _, sess := range GetSessionManager().GetAllSessions() {
		if err := WriteJournal(sess); err != nil {
			log.Printf("⚠️ Failed to journal session %s:%s: %v", sess.Npub, sess.SaveID, err)
		}
	}
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
