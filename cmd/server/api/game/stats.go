package game

import (
	"log"
	"slices"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/badges"
	"pubkey-quest/cmd/server/game/boards"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

// commitStats counts the play a kept save covers: the session's pending
// lifetime stats up to mark (cmd/server/stats), then the badges the kept save
// earned (game/badges) and where its character stands on the boards
// (game/boards). A failure keeps them pending for the next save. It returns
// the badges newly earned.
func commitStats(sess *session.GameSession, mark int, kept *SaveFile) []string {
	var earned []string
	judge := func(r *stats.Record, now int64) bool {
		level := 0
		if adv, err := loadAdvancement(); err == nil {
			level = character.GetLevelFromXP(kept.Experience, adv)
		}
		earned = badges.Judge(badges.Config().Badges, badges.Catalog(), r, kept, level, now)
		standing := boards.Standing(kept, level, gameutil.NetWorth(kept),
			quest.QuestPoints(kept, serverdb.GetQuestByID), placesFound(kept))
		stood := boards.Stand(r, sess.SaveID, standing, now)
		return len(earned) > 0 || stood
	}
	if _, err := stats.Commit(sess.Npub, sess.SaveID, mark, judge); err != nil {
		log.Printf("⚠️ Stats for %s:%s wait for the next save: %v", sess.Npub, sess.SaveID, err)
		return nil
	}
	if len(earned) > 0 {
		log.Printf("🏅 %s earned %v", sess.Npub, earned)
	}
	return earned
}

// CommitSavedStats is commitStats for a relay save, once a relay has kept it:
// mark is where the stats stood when the save was prepared, and kept is the
// save as it was then.
func CommitSavedStats(sess *session.GameSession, mark int, kept *SaveFile) []string {
	return commitStats(sess, mark, kept)
}

// placesFound is how many of the realm's places a character has found:
// towns, wild regions and places of note (the Cartographer's list).
func placesFound(save *SaveFile) int {
	n := 0
	for _, id := range badges.Catalog()[types.CollectionPlaces] {
		if slices.Contains(save.LocationsDiscovered, id) {
			n++
		}
	}
	return n
}

// noteFound counts an item found out in the world (mined, dug, looted; not
// bought or given) toward the collections: a gem's type, an ore.
func noteFound(state *SaveFile, itemID string) {
	if g, ok := gems.Parse(gems.Config(), itemID); ok && g.Piece == "" {
		stats.AddMember(state, stats.GemTypes, g.Type)
		return
	}
	if _, ok := miningConfig().Ores[itemID]; ok {
		stats.AddMember(state, stats.OresMined, itemID)
	}
}

// DropFromBoards takes a deleted character off the standing boards.
func DropFromBoards(npub, saveID string) {
	if !stats.Enabled {
		return
	}
	err := stats.Update(npub, func(r *stats.Record) bool { return boards.Drop(r, saveID, time.Now().Unix()) })
	if err != nil {
		log.Printf("⚠️ Couldn't take %s:%s off the boards: %v", npub, saveID, err)
	}
}
