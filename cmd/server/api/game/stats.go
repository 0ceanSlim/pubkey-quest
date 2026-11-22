package game

import (
	"log"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/stats"
)

// commitStats counts the play a kept save covers: the session's pending
// lifetime stats up to mark (cmd/server/stats). A failure keeps them pending
// for the next save.
func commitStats(sess *session.GameSession, mark int) {
	if _, err := stats.Commit(sess.Npub, sess.SaveID, mark); err != nil {
		log.Printf("⚠️ Stats for %s:%s wait for the next save: %v", sess.Npub, sess.SaveID, err)
	}
}

// CommitSavedStats is commitStats for a relay save, once a relay has kept it:
// mark is where the stats stood when the save was prepared.
func CommitSavedStats(sess *session.GameSession, mark int) { commitStats(sess, mark) }
