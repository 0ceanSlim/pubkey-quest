package game

import (
	"log"
	"time"

	"pubkey-quest/cmd/server/session"
)

// ─── Idle sessions ───────────────────────────────────────────────────────────
//
// A session lives in memory until the player quits cleanly. One whose browser
// just went away stayed forever, and a client could open sessions without end
// (beta exploit audit E3). A session with no request for SessionIdleLimit is
// journaled (its unsaved play can still be recovered, as after a crash) and
// unloaded, as a quit would: it leaves its party and presence first.

// SessionIdleLimit is how long a session waits for its player.
const SessionIdleLimit = 30 * time.Minute

// EvictIdleSessions unloads every session idle for longer than limit.
func EvictIdleSessions(limit time.Duration, now time.Time) int {
	n := 0
	for _, sess := range session.GetSessionManager().GetAllSessions() {
		if now.Sub(session.IdleSince(sess)) < limit {
			continue
		}
		unlock := session.LockSlot(sess.Npub, sess.SaveID)
		if err := session.WriteJournal(sess); err != nil {
			log.Printf("⚠️ Failed to journal idle session %s:%s: %v", sess.Npub, sess.SaveID, err)
		}
		leaveSocial(sess.Npub, sess.SaveID)
		session.GetSessionManager().UnloadSession(sess.Npub, sess.SaveID)
		session.ForgetSlot(sess.Npub, sess.SaveID)
		unlock()
		n++
	}
	return n
}

// StartIdleEviction checks for idle sessions every few minutes. Call once.
func StartIdleEviction() {
	go func() {
		for range time.Tick(5 * time.Minute) {
			if n := EvictIdleSessions(SessionIdleLimit, time.Now()); n > 0 {
				log.Printf("🔄 Unloaded %d idle session(s)", n)
			}
		}
	}()
}
