package game

import (
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/world"
)

// ─── The ground ─────────────────────────────────────────────────────────────
//
// What you drop is yours alone until you save (world/ground.go): your save
// still holds it, so a reload or quitting without saving puts it back in your
// pack. A save lays it on the shared ground, for anyone at that spot.

// groundSpot is where the player stands, as the ground knows it: the spot
// presence uses (a building's default room filled in), or on a road the road
// and how far along it from its first end.
func groundSpot(sess *session.GameSession) world.Spot {
	refreshPresencePlace(sess)
	if sess.PresenceSpot != "" {
		return world.Spot{Key: sess.PresenceSpot}
	}
	s := &sess.SaveData
	pos := s.TravelProgress
	if sess.RouteFirstEnd != "" && s.District != sess.RouteFirstEnd {
		pos = 1 - pos
	}
	return world.Spot{Key: "road|" + s.Location, Road: true, Pos: pos}
}

// groundHere lists what the player sees on the ground: their own unsaved
// drops first, then the world's.
func groundHere(sess *session.GameSession) []world.GroundDrop {
	spot := groundSpot(sess)
	out := sess.Ground.List(spot, sess.Npub)
	return append(out, world.World.List(spot, sess.Npub)...)
}

// takeFromGround takes up to quantity (all: quantity < 0) of a pile the player
// sees: their own drops first, then the world's. It returns the pile, how many
// were taken and the store it came from (to put back what doesn't fit).
func takeFromGround(sess *session.GameSession, pileID, itemID string, quantity int) (world.Pile, int, *world.GroundStore) {
	spot := groundSpot(sess)
	for _, g := range []*world.GroundStore{sess.Ground, world.World} {
		if p, n := g.Take(spot, sess.Npub, pileID, itemID, quantity); n > 0 {
			return p, n, g
		}
	}
	return world.Pile{}, 0, nil
}

// publishGround lays the player's drops up to mark on the shared ground: the
// save that was just kept no longer holds them. A quest item stays the
// player's own (it shows to no one else), so no one gets a quest's item
// without doing the quest.
func publishGround(sess *session.GameSession, mark int) {
	piles := sess.Ground.Drain(mark)
	for i := range piles {
		if isQuestItem(piles[i].Item) {
			piles[i].Owner = sess.Npub
		}
	}
	world.World.Publish(piles)
}

// PublishSavedGround is publishGround for a relay save, once a relay has kept
// it: mark is where the ground stood when the save was prepared.
func PublishSavedGround(sess *session.GameSession, mark int) { publishGround(sess, mark) }
