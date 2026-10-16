package game

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/presence"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/social"
	"pubkey-quest/types"
)

// ─── Presence and the social poll (docs/draft/social-plan.md §2) ─────────────
//
// Every game action, and every social poll, publishes the player's presence
// snapshot (who, where, when). Other players read only those snapshots, never
// another session's live save. The client polls /api/social/poll every few
// seconds — paused or not — which is both its heartbeat and how it hears about
// players at its spot, chat, invites and its party.

// PlayerView is another player as this player sees them.
// swagger:model PlayerView
type PlayerView struct {
	Npub        string `json:"npub"`
	Name        string `json:"name"         example:"Mara"`
	Race        string `json:"race"         example:"Human"`
	Class       string `json:"class"        example:"Druid"`
	Level       int    `json:"level"        example:"3"`
	InCombat    bool   `json:"in_combat"`
	Where       string `json:"where"        example:"Salty Anchor · Taproom"`
	Here        bool   `json:"here"`                      // at your exact spot (or nearby on your road) and within the hour
	ClockOffset int    `json:"clock_offset" example:"40"` // minutes their clock is ahead of yours (negative = behind)
	PortraitSrc string `json:"portrait_src,omitempty"`
}

// SocialPollResponse is the envelope the client polls.
// swagger:model SocialPollResponse
type SocialPollResponse struct {
	Success      bool         `json:"success"`
	Here         []PlayerView `json:"here"`       // People column: players you can see
	AreaCount    int          `json:"area_count"` // headcount: others online in this city / on this road
	AreaName     string       `json:"area_name"`
	Chat         []ChatLine   `json:"chat"`
	Invites      []InviteView `json:"invites"`
	Party        *PartyView   `json:"party"`
	PartySync    bool         `json:"party_sync,omitempty"` // the leader moved you: refresh the world
	InCombat     bool         `json:"in_combat"`            // you're in a fight (a party member may have started it)
	ClockRunning bool         `json:"clock_running"`        // play/pause (a follower's mirrors the leader's)
}

// advancementOnce caches the advancement table for presence levels.
var (
	advOnce  sync.Once
	advCache []types.AdvancementEntry
)

func cachedAdvancement() []types.AdvancementEntry {
	advOnce.Do(func() { advCache, _ = loadAdvancement() })
	return advCache
}

// refreshPresencePlace recomputes the normalized spot and display text when
// the raw place changed.
func refreshPresencePlace(sess *session.GameSession) {
	s := &sess.SaveData
	key := s.Location + "|" + s.District + "|" + s.Building + "|" + s.Room
	if key == sess.PresenceKey {
		return
	}
	sess.PresenceKey = key
	db := serverdb.GetDB()

	if env := travel.GetEnvironmentData(s.Location); env != nil {
		// On a road: no spot; position along it instead.
		sess.PresenceSpot = ""
		sess.PresenceArea = env.Name
		sess.PresenceWhere = "on the road"
		sess.RouteFirstEnd = ""
		if len(env.Connects) > 0 {
			sess.RouteFirstEnd = env.Connects[0]
		}
		return
	}
	sess.RouteFirstEnd = ""

	room := s.Room
	locName, districtName, buildingName, roomName := lookupPlaceNames(s.Location, s.District, s.Building, room)
	if s.Building != "" && room == "" {
		if rooms, def, err := building.GetBuildingRooms(db, s.Location, s.Building); err == nil && def != "" {
			room = def
			if r, ok := building.FindRoom(rooms, def); ok {
				roomName = r.Name
			}
		}
	}
	sess.PresenceSpot = strings.Join([]string{s.Location, s.District, s.Building, room}, "|")
	sess.PresenceArea = locName
	switch {
	case buildingName != "" && roomName != "":
		sess.PresenceWhere = buildingName + " · " + roomName
	case buildingName != "":
		sess.PresenceWhere = buildingName
	case districtName != "":
		sess.PresenceWhere = districtName
	default:
		sess.PresenceWhere = locName
	}
}

// lookupPlaceNames reads display names for a city place from its location
// data: the city, the district, the building and the room.
func lookupPlaceNames(location, district, buildingID, room string) (loc, dist, bld, rm string) {
	loc = location
	var props string
	if err := serverdb.GetDB().QueryRow("SELECT properties FROM locations WHERE id = ?", location).Scan(&props); err != nil {
		return
	}
	var data struct {
		Name      string `json:"name"`
		Districts map[string]struct {
			Name      string `json:"name"`
			Buildings []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Rooms []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"rooms"`
			} `json:"buildings"`
		} `json:"districts"`
	}
	if json.Unmarshal([]byte(props), &data) != nil {
		return
	}
	if data.Name != "" {
		loc = data.Name
	}
	d, ok := data.Districts[district]
	if !ok {
		return
	}
	dist = d.Name
	for _, b := range d.Buildings {
		if b.ID != buildingID {
			continue
		}
		bld = b.Name
		for _, r := range b.Rooms {
			if r.ID == room {
				rm = r.Name
			}
		}
	}
	return
}

// publishPresence refreshes this player's snapshot in the registry.
func publishPresence(sess *session.GameSession) {
	refreshPresencePlace(sess)
	s := &sess.SaveData
	e := presence.Entry{
		Npub:         sess.Npub,
		SaveID:       sess.SaveID,
		Name:         s.D,
		Race:         s.Race,
		Class:        s.Class,
		Level:        character.GetLevelFromXP(s.Experience, cachedAdvancement()),
		Spot:         sess.PresenceSpot,
		AbsMinutes:   s.CurrentDay*1440 + s.TimeOfDay,
		InCombat:     sess.ActiveCombat != nil,
		ClockRunning: sess.ClockRunning,
		ClockHeld:    sess.ActivePOI != nil || sess.PendingEncounter != nil,
		LastSeen:     time.Now(),
		Location:     s.Location,
		AreaName:     sess.PresenceArea,
		Where:        sess.PresenceWhere,
		Portrait:     s.Portrait,
	}
	if sess.PresenceSpot == "" && sess.RouteFirstEnd != "" {
		e.Route = s.Location
		e.RoutePos = s.TravelProgress
		if s.District != sess.RouteFirstEnd {
			e.RoutePos = 1 - s.TravelProgress
		}
	}
	presence.Publish(e)
}

// playerView describes another player relative to viewer.
func playerView(e presence.Entry, viewerAbs int, here bool) PlayerView {
	return PlayerView{
		Npub: e.Npub, Name: e.Name, Race: e.Race, Class: e.Class, Level: e.Level,
		InCombat: e.InCombat, Where: e.Where, Here: here,
		ClockOffset: e.AbsMinutes - viewerAbs,
		PortraitSrc: PortraitSrc(e.Portrait),
	}
}

// playersHere is who this player can see: spot-mates in town, or nearby
// travellers on the same road.
func playersHere(sess *session.GameSession, now time.Time) []presence.Entry {
	me, ok := presence.Get(sess.Npub)
	if !ok {
		return nil
	}
	if me.Route != "" {
		return presence.NearbyOnRoad(sess.Npub, me.Route, me.RoutePos, me.AbsMinutes, now)
	}
	return presence.Here(sess.Npub, me.Spot, me.AbsMinutes, now)
}

// socialSession resolves the caller's session from query or body identifiers.
func socialSession(npub, saveID string) (*session.GameSession, error) {
	if npub == "" || saveID == "" {
		return nil, fmt.Errorf("missing npub or save_id")
	}
	return session.GetSessionManager().GetSession(npub, saveID)
}

// SocialPollHandler godoc
// @Summary      The social heartbeat
// @Description  Refreshes your presence and returns players you can see, the headcount for your area,
// @Description  new chat lines, party invites and your party. Poll every few seconds, paused or not.
// @Tags         Social
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  SocialPollResponse
// @Router       /api/social/poll [get]
func SocialPollHandler(w http.ResponseWriter, r *http.Request) {
	sess, err := socialSession(r.URL.Query().Get("npub"), r.URL.Query().Get("save_id"))
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}
	sess.Lock()
	partySync := applyPartySyncs(sess)
	publishPresence(sess)
	sess.Unlock()

	now := time.Now()
	dropStaleMembers(sess.Npub, now)
	myAbs := sess.SaveData.CurrentDay*1440 + sess.SaveData.TimeOfDay
	resp := SocialPollResponse{Success: true, Here: []PlayerView{}, Chat: []ChatLine{}, Invites: []InviteView{}, PartySync: partySync}
	for _, e := range playersHere(sess, now) {
		resp.Here = append(resp.Here, playerView(e, myAbs, true))
	}
	resp.AreaCount = len(presence.InArea(sess.Npub, sess.SaveData.Location, now))
	resp.AreaName = sess.PresenceArea
	resp.Chat = drainChat(sess.Npub)
	resp.Invites = invitesFor(sess.Npub)
	resp.Party = partyViewFor(sess.Npub)
	resp.InCombat = sess.ActiveCombat != nil
	resp.ClockRunning = clockRunningFor(sess)
	writeCombatJSON(w, http.StatusOK, resp)
}

// SocialAreaResponse lists everyone online in your area.
// swagger:model SocialAreaResponse
type SocialAreaResponse struct {
	Success  bool         `json:"success"`
	AreaName string       `json:"area_name"`
	Players  []PlayerView `json:"players"`
}

// SocialAreaHandler godoc
// @Summary      Everyone online in your city or on your road
// @Tags         Social
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  SocialAreaResponse
// @Router       /api/social/area [get]
func SocialAreaHandler(w http.ResponseWriter, r *http.Request) {
	sess, err := socialSession(r.URL.Query().Get("npub"), r.URL.Query().Get("save_id"))
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}
	now := time.Now()
	myAbs := sess.SaveData.CurrentDay*1440 + sess.SaveData.TimeOfDay
	hereSet := map[string]bool{}
	for _, e := range playersHere(sess, now) {
		hereSet[e.Npub] = true
	}
	resp := SocialAreaResponse{Success: true, AreaName: sess.PresenceArea, Players: []PlayerView{}}
	for _, e := range presence.InArea(sess.Npub, sess.SaveData.Location, now) {
		resp.Players = append(resp.Players, playerView(e, myAbs, hereSet[e.Npub]))
	}
	sort.SliceStable(resp.Players, func(i, j int) bool {
		if resp.Players[i].Here != resp.Players[j].Here {
			return resp.Players[i].Here
		}
		return resp.Players[i].Where < resp.Players[j].Where
	})
	writeCombatJSON(w, http.StatusOK, resp)
}

// decodeSocial reads a JSON body into v and resolves the session.
func decodeSocial(r *http.Request, v interface{ ids() (string, string) }) (*session.GameSession, error) {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	npub, saveID := v.ids()
	return socialSession(npub, saveID)
}

// leaveSocial takes a player out of the social world when their session ends:
// off the presence list, out of their party, mailbox dropped.
func leaveSocial(npub, saveID string) {
	if e, ok := presence.Get(npub); ok && e.SaveID == saveID {
		presence.Remove(npub)
	}
	if p := social.PartyOf(npub); p != nil {
		name := npub
		if s, err := session.GetSessionManager().GetSession(npub, saveID); err == nil {
			name = s.SaveData.D
		}
		leaveParty(npub, name, "left the game")
	}
	social.Forget(npub)
}

// dropStaleMembers removes party members who've been offline over a minute.
func dropStaleMembers(npub string, now time.Time) {
	p := social.PartyOf(npub)
	if p == nil {
		return
	}
	for _, m := range p.Members {
		e, ok := presence.Get(m.Npub)
		if !ok || now.Sub(e.LastSeen) > time.Minute {
			name := m.Npub
			if ok {
				name = e.Name
			}
			leaveParty(m.Npub, name, "lost the trail (offline)")
		}
	}
}

// FriendInGame describes a friend who is playing right now, as the viewer sees
// them (where they are, clock offset, whether they're here). Nil when offline.
func FriendInGame(viewerNpub, viewerSaveID, friendNpub string) *PlayerView {
	now := time.Now()
	e, ok := presence.Get(friendNpub)
	if !ok || !e.Online(now) {
		return nil
	}
	viewerAbs, here := e.AbsMinutes, false
	if sess, err := session.GetSessionManager().GetSession(viewerNpub, viewerSaveID); err == nil {
		viewerAbs = sess.SaveData.CurrentDay*1440 + sess.SaveData.TimeOfDay
		for _, h := range playersHere(sess, now) {
			here = here || h.Npub == friendNpub
		}
	}
	v := playerView(e, viewerAbs, here)
	return &v
}
