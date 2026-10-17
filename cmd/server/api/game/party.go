package game

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/presence"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/social"
)

// ─── Parties (docs/draft/social-plan.md §9) ──────────────────────────────────
//
// Invite only someone standing where you are; joining snaps your clock to the
// leader's; from then on the leader's moves carry the party. A leader's request
// never touches a follower's session: it queues a PartySync, which the
// follower's own next request (tick or social poll) applies under its own lock.

// ChatLine is a chat message delivered to a player.
type ChatLine = social.ChatLine

// InviteView is an open party invite.
// swagger:model InviteView
type InviteView struct {
	FromNpub  string `json:"from_npub"`
	FromName  string `json:"from_name" example:"Mara"`
	ExpiresIn int    `json:"expires_in" example:"95"` // seconds
}

// PartyMemberView is one member as the party sees them.
// swagger:model PartyMemberView
type PartyMemberView struct {
	Npub        string `json:"npub"`
	Name        string `json:"name"`
	Class       string `json:"class"`
	Level       int    `json:"level"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"max_hp"`
	Leader      bool   `json:"leader"`
	Online      bool   `json:"online"`
	You         bool   `json:"you"`
	InCombat    bool   `json:"in_combat"`
	PortraitSrc string `json:"portrait_src,omitempty"`
}

// PartyView is the player's party.
// swagger:model PartyView
type PartyView struct {
	ID      int64             `json:"id"`
	YouLead bool              `json:"you_lead"`
	Members []PartyMemberView `json:"members"`
}

// partyRequest is the body for every party endpoint.
type partyRequest struct {
	Npub   string `json:"npub"`
	SaveID string `json:"save_id"`
	To     string `json:"to"`   // invite / kick / promote target npub
	From   string `json:"from"` // accept / decline: the inviter's npub
}

func (p *partyRequest) ids() (string, string) { return p.Npub, p.SaveID }

func invitesFor(npub string) []InviteView {
	now := time.Now()
	out := []InviteView{}
	for _, inv := range social.InvitesFor(npub, now) {
		name := inv.From.Npub
		if e, ok := presence.Get(inv.From.Npub); ok {
			name = e.Name
		}
		out = append(out, InviteView{FromNpub: inv.From.Npub, FromName: name, ExpiresIn: int(time.Until(inv.Expires).Seconds())})
	}
	return out
}

func partyViewFor(npub string) *PartyView {
	p := social.PartyOf(npub)
	if p == nil {
		return nil
	}
	now := time.Now()
	v := &PartyView{ID: p.ID, YouLead: p.Leader().Npub == npub}
	for i, m := range p.Members {
		mv := PartyMemberView{Npub: m.Npub, Leader: i == 0, You: m.Npub == npub}
		if e, ok := presence.Get(m.Npub); ok {
			mv.Name, mv.Class, mv.Level, mv.Online, mv.InCombat = e.Name, e.Class, e.Level, e.Online(now), e.InCombat
			mv.PortraitSrc = PortraitSrc(e.Portrait)
		}
		if s, err := session.GetSessionManager().GetSession(m.Npub, m.SaveID); err == nil {
			mv.HP, mv.MaxHP = s.SaveData.HP, s.SaveData.MaxHP
			// Mid-fight, HP lives on the fight, not the save.
			if cs := s.ActiveCombat; cs != nil {
				for i := range cs.Party {
					if cs.Party[i].ID == m.Npub {
						mv.HP, mv.MaxHP = cs.Party[i].CombatState.CurrentHP, cs.Party[i].CombatState.MaxHP
					}
				}
			}
		}
		v.Members = append(v.Members, mv)
	}
	return v
}

// partyMembersNpubs lists a party's member npubs.
func partyMembersNpubs(p *social.Party) []string {
	out := make([]string, 0, len(p.Members))
	for _, m := range p.Members {
		out = append(out, m.Npub)
	}
	return out
}

// systemLine delivers a system message to some players.
func systemLine(text string, to ...string) {
	social.Deliver(ChatLine{Channel: "system", Text: text}, to)
}

// sameSpot reports whether two online players stand where an invite is allowed:
// the same spot in town (any clock), or near each other on the same road.
func sameSpot(a, b string) bool {
	now := time.Now()
	ea, okA := presence.Get(a)
	eb, okB := presence.Get(b)
	if !okA || !okB || !ea.Online(now) || !eb.Online(now) {
		return false
	}
	if ea.Route != "" || eb.Route != "" {
		d := ea.RoutePos - eb.RoutePos
		if d < 0 {
			d = -d
		}
		return ea.Route == eb.Route && d <= presence.RoadNearby
	}
	return ea.Spot != "" && ea.Spot == eb.Spot
}

func partyFail(w http.ResponseWriter, msg string) {
	writeCombatJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": msg})
}

func partyOK(w http.ResponseWriter, npub, msg string) {
	writeCombatJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": msg, "party": partyViewFor(npub)})
}

// PartyInviteHandler godoc
// @Summary      Invite a player standing here to your party
// @Tags         Social
// @Accept       json
// @Produce      json
// @Param        request  body  partyRequest  true  "to = the invitee's npub"
// @Router       /api/party/invite [post]
func PartyInviteHandler(w http.ResponseWriter, r *http.Request) {
	var req partyRequest
	sess, err := decodeSocial(r, &req)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !sameSpot(req.Npub, req.To) {
		partyFail(w, "You can only invite someone standing right here with you.")
		return
	}
	if err := social.AddInvite(social.Member{Npub: req.Npub, SaveID: req.SaveID}, req.To, time.Now()); err != nil {
		partyFail(w, err.Error())
		return
	}
	target, _ := presence.Get(req.To)
	systemLine(fmt.Sprintf("🎉 %s invites you to their party.", sess.SaveData.D), req.To)
	partyOK(w, req.Npub, fmt.Sprintf("Invited %s.", target.Name))
}

// PartyAcceptHandler godoc
// @Summary      Accept a party invite (your clock snaps to the leader's)
// @Tags         Social
// @Accept       json
// @Produce      json
// @Param        request  body  partyRequest  true  "from = the inviter's npub"
// @Router       /api/party/accept [post]
func PartyAcceptHandler(w http.ResponseWriter, r *http.Request) {
	var req partyRequest
	sess, err := decodeSocial(r, &req)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	inv, ok := social.TakeInvite(req.Npub, req.From, time.Now())
	if !ok {
		partyFail(w, "That invite has expired.")
		return
	}
	if !sameSpot(req.Npub, req.From) {
		partyFail(w, "You need to be standing with them to join.")
		return
	}
	if sess.ActiveCombat != nil || sess.PendingEncounter != nil {
		partyFail(w, "Finish what you're doing first.")
		return
	}
	p, err := social.Join(inv.From, social.Member{Npub: req.Npub, SaveID: req.SaveID})
	if err != nil {
		partyFail(w, err.Error())
		return
	}

	// Snap to the leader: their place and clock.
	if leader, err := session.GetSessionManager().GetSession(p.Leader().Npub, p.Leader().SaveID); err == nil {
		sess.QueuePartySync(syncFromLeader(leader, ""))
	}
	sess.Lock()
	applyPartySyncs(sess)
	publishPresence(sess)
	sess.Unlock()

	systemLine(fmt.Sprintf("🎉 %s joined the party.", sess.SaveData.D), partyMembersNpubs(p)...)
	partyOK(w, req.Npub, "You joined the party.")
}

// PartyDeclineHandler godoc
// @Summary      Decline a party invite
// @Tags         Social
// @Accept       json
// @Param        request  body  partyRequest  true  "from = the inviter's npub"
// @Router       /api/party/decline [post]
func PartyDeclineHandler(w http.ResponseWriter, r *http.Request) {
	var req partyRequest
	sess, err := decodeSocial(r, &req)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, ok := social.TakeInvite(req.Npub, req.From, time.Now()); ok {
		systemLine(fmt.Sprintf("%s declined your invite.", sess.SaveData.D), req.From)
	}
	partyOK(w, req.Npub, "Declined.")
}

// PartyLeaveHandler godoc
// @Summary      Leave your party (you keep the party's place and time)
// @Tags         Social
// @Accept       json
// @Param        request  body  partyRequest  true  "Session"
// @Router       /api/party/leave [post]
func PartyLeaveHandler(w http.ResponseWriter, r *http.Request) {
	var req partyRequest
	sess, err := decodeSocial(r, &req)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	if sess.ActiveCombat != nil && len(sess.ActiveCombat.Party) > 1 {
		partyFail(w, "You can't leave the party mid-fight — flee first.")
		return
	}
	leaveParty(sess.Npub, sess.SaveData.D, "left the party")
	partyOK(w, req.Npub, "You left the party.")
}

// leaveParty takes a player out and tells whoever's left.
func leaveParty(npub, name, verb string) {
	p, was := social.Leave(npub)
	if !was {
		return
	}
	if p == nil {
		return
	}
	systemLine(fmt.Sprintf("🚪 %s %s.", name, verb), partyMembersNpubs(p)...)
	if l, ok := presence.Get(p.Leader().Npub); ok {
		systemLine(fmt.Sprintf("👑 %s now leads the party.", l.Name), partyMembersNpubs(p)...)
	}
}

// PartyKickHandler godoc
// @Summary      Remove a member (leader only)
// @Tags         Social
// @Accept       json
// @Param        request  body  partyRequest  true  "to = the member's npub"
// @Router       /api/party/kick [post]
func PartyKickHandler(w http.ResponseWriter, r *http.Request) {
	var req partyRequest
	if _, err := decodeSocial(r, &req); err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	who, _ := presence.Get(req.To)
	p, err := social.Kick(req.Npub, req.To)
	if err != nil {
		partyFail(w, err.Error())
		return
	}
	systemLine("🚪 You were removed from the party.", req.To)
	if p != nil {
		systemLine(fmt.Sprintf("🚪 %s was removed from the party.", who.Name), partyMembersNpubs(p)...)
	}
	partyOK(w, req.Npub, "Removed.")
}

// PartyPromoteHandler godoc
// @Summary      Hand leadership to another member (leader only)
// @Tags         Social
// @Accept       json
// @Param        request  body  partyRequest  true  "to = the member's npub"
// @Router       /api/party/promote [post]
func PartyPromoteHandler(w http.ResponseWriter, r *http.Request) {
	var req partyRequest
	if _, err := decodeSocial(r, &req); err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := social.Promote(req.Npub, req.To)
	if err != nil {
		partyFail(w, err.Error())
		return
	}
	if l, ok := presence.Get(req.To); ok {
		systemLine(fmt.Sprintf("👑 %s now leads the party.", l.Name), partyMembersNpubs(p)...)
	}
	partyOK(w, req.Npub, "Promoted.")
}

// ─── Chat ────────────────────────────────────────────────────────────────────

// sayRequest posts a chat line.
type sayRequest struct {
	Npub    string `json:"npub"`
	SaveID  string `json:"save_id"`
	Channel string `json:"channel"` // say | area | party
	Text    string `json:"text"`
}

func (s *sayRequest) ids() (string, string) { return s.Npub, s.SaveID }

func drainChat(npub string) []ChatLine { return social.Drain(npub) }

// SocialSayHandler godoc
// @Summary      Say something in chat
// @Description  say: players who can see you · area: everyone online in your city or on your road ·
// @Description  party: your party. ≤200 characters, 5 lines per 10 s. Nothing is stored.
// @Tags         Social
// @Accept       json
// @Produce      json
// @Param        request  body  sayRequest  true  "Line"
// @Router       /api/social/say [post]
func SocialSayHandler(w http.ResponseWriter, r *http.Request) {
	var req sayRequest
	sess, err := decodeSocial(r, &req)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}
	text := social.CleanText(req.Text)
	if text == "" {
		partyFail(w, "Nothing to say.")
		return
	}
	now := time.Now()
	var to []string
	switch req.Channel {
	case "say":
		for _, e := range playersHere(sess, now) {
			to = append(to, e.Npub)
		}
	case "area":
		for _, e := range presence.InArea(sess.Npub, sess.SaveData.Location, now) {
			to = append(to, e.Npub)
		}
	case "party":
		p := social.PartyOf(sess.Npub)
		if p == nil {
			partyFail(w, "You're not in a party.")
			return
		}
		for _, n := range partyMembersNpubs(p) {
			if n != sess.Npub {
				to = append(to, n)
			}
		}
	default:
		partyFail(w, "Unknown channel.")
		return
	}
	if !social.AllowSend(sess.Npub, now) {
		writeCombatJSON(w, http.StatusTooManyRequests, map[string]interface{}{"success": false, "error": "Slow down."})
		return
	}
	line := ChatLine{Channel: req.Channel, FromNpub: sess.Npub, FromName: sess.SaveData.D, Text: text}
	social.Deliver(line, append(to, sess.Npub))
	writeCombatJSON(w, http.StatusOK, map[string]interface{}{"success": true, "delivered": len(to)})
}

// ─── Follow the leader ───────────────────────────────────────────────────────

// followerBlockedActions are moves and time skips only the leader makes.
var followerBlockedActions = map[string]bool{
	"move": true, "enter_building": true, "exit_building": true, "move_to_room": true,
	"start_travel": true, "stop_travel": true, "resume_travel": true, "turn_back": true,
	"wait": true, "sleep": true, "rest": true, "advance_time": true, "play_show": true,
	// The clock is the leader's, and picking a lock spends time.
	"set_clock": true, "reset_idle_timer": true,
	"lock_begin": true, "lock_act": true, "pick_lock": true,
}

// partyPlace is what the leader's moves change.
type partyPlace struct {
	Location, District, Building, Room string
	Abs                                int
	Progress                           float64
	Stopped                            bool
}

func placeOf(sess *session.GameSession) partyPlace {
	s := &sess.SaveData
	return partyPlace{s.Location, s.District, s.Building, s.Room, s.CurrentDay*1440 + s.TimeOfDay, s.TravelProgress, s.TravelStopped}
}

// syncFromLeader describes the leader's current place and clock.
func syncFromLeader(leader *session.GameSession, note string) session.PartySync {
	s := &leader.SaveData
	return session.PartySync{
		Location: s.Location, District: s.District, Building: s.Building, Room: s.Room,
		CurrentDay: s.CurrentDay, TimeOfDay: s.TimeOfDay,
		TravelProgress: s.TravelProgress, TravelStopped: s.TravelStopped,
		AccrueFatigue: true, Note: note,
	}
}

// partyNews is what a leader's action tells the party besides where they are:
// places just found, and lines for the followers' log.
type partyNews struct {
	Discovered []string
	Notes      []string
}

func (n partyNews) empty() bool { return len(n.Discovered) == 0 && len(n.Notes) == 0 }

// travelNotes are the followers' log lines for the leader's road decisions.
var travelNotes = map[string]string{
	"stop_travel":   "⏸ Your party stops on the road.",
	"resume_travel": "🚶 Your party carries on.",
	"turn_back":     "↩ Your party turns back.",
	"wait":          "⏳ Your party waits a while.",
}

// queuePartyMoves sends the leader's change to every follower after an action.
// sleepKind is non-empty when the leader just slept (the followers sleep too).
func queuePartyMoves(leader *session.GameSession, before partyPlace, actionType, sleepKind string, news partyNews) {
	p := social.PartyOf(leader.Npub)
	if p == nil || p.Leader().Npub != leader.Npub {
		return
	}
	after := placeOf(leader)
	if after == before && sleepKind == "" && news.empty() {
		return
	}
	note := travelNotes[actionType]
	if note == "" && (after.Location != before.Location || after.District != before.District ||
		after.Building != before.Building || after.Room != before.Room) {
		refreshPresencePlace(leader)
		note = "🚶 Your party moves on — " + leader.PresenceWhere + "."
		if leader.PresenceWhere == "on the road" {
			note = "🚶 Your party sets out on the road."
		}
	}
	sync := syncFromLeader(leader, note)
	sync.Discovered, sync.Notes = news.Discovered, news.Notes
	switch actionType {
	case "wait":
		sync.AccrueFatigue = false
	case "rest":
		sync.Rest = true
		sync.CurrentDay, sync.TimeOfDay = before.Abs/1440, before.Abs%1440
	}
	if sleepKind != "" {
		sync.Sleep = sleepKind
		sync.CurrentDay, sync.TimeOfDay = before.Abs/1440, before.Abs%1440
		sync.Note = "😴 The party beds down for the night."
	}
	for _, m := range p.Members[1:] {
		if fs, err := session.GetSessionManager().GetSession(m.Npub, m.SaveID); err == nil {
			fs.QueuePartySync(sync)
		}
	}
}

// partyBedCheck decides whether the leader's party can sleep, and how the
// followers sleep: "inn" (everyone has a room here), "camp" (a camp set in
// someone's pack beds everyone), or "wild" (each on their own bedroll or the
// ground). Returns an error naming who has no bed.
func partyBedCheck(leader *session.GameSession) (string, error) {
	p := social.PartyOf(leader.Npub)
	if p == nil {
		return "", nil
	}
	var members []*session.GameSession
	for _, m := range p.Members {
		if s, err := session.GetSessionManager().GetSession(m.Npub, m.SaveID); err == nil {
			members = append(members, s)
		}
	}
	building := leader.SaveData.Building
	if building != "" {
		var missing []string
		for _, s := range members[1:] {
			if !gameutil.HasActiveRental(&s.SaveData, building) {
				missing = append(missing, s.SaveData.D)
			}
		}
		if len(missing) > 0 {
			return "", fmt.Errorf("Not everyone in your party has a room here: %s.", strings.Join(missing, ", "))
		}
		return "inn", nil
	}
	for _, s := range members {
		if gameutil.PlayerHasItem(&s.SaveData, "camp-set") {
			return "camp", nil
		}
	}
	var missing []string
	for _, s := range members {
		if !gameutil.PlayerHasItem(&s.SaveData, "bedroll") {
			missing = append(missing, s.SaveData.D)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("Not everyone in your party has a bed — %s needs a bedroll, or someone needs a camp set.", strings.Join(missing, ", "))
	}
	return "wild", nil
}

// applyPartySyncs applies the leader's queued moves to this follower. Call with
// the follower's own lock held. Returns whether anything was applied.
// A follower who is busy — in a fight, a walk, an unaware choice or a lock —
// isn't moved out from under it: the syncs wait until they're free.
func applyPartySyncs(sess *session.GameSession) bool {
	if sess.ActiveCombat != nil || sess.ActivePOI != nil || sess.PendingEncounter != nil || sess.LockSession != nil {
		return false
	}
	syncs := sess.TakePartySyncs()
	if len(syncs) == 0 {
		return false
	}
	for _, p := range syncs {
		applyPartySync(sess, p)
	}
	return true
}

// partyShare gives every party member standing with the leader the places
// the leader just found, with a line in their log.
func partyShare(leader *session.GameSession, ids []string, note string) {
	p := social.PartyOf(leader.Npub)
	if p == nil || p.Leader().Npub != leader.Npub || len(ids) == 0 {
		return
	}
	for _, m := range p.Members[1:] {
		if fs, err := session.GetSessionManager().GetSession(m.Npub, m.SaveID); err == nil {
			fs.QueuePartySync(session.PartySync{
				Location: fs.SaveData.Location, District: fs.SaveData.District, Building: fs.SaveData.Building, Room: fs.SaveData.Room,
				CurrentDay: fs.SaveData.CurrentDay, TimeOfDay: fs.SaveData.TimeOfDay,
				TravelProgress: fs.SaveData.TravelProgress, TravelStopped: fs.SaveData.TravelStopped,
				AccrueFatigue: true, Discovered: ids, Notes: []string{note},
			})
		}
	}
}

// partyLog sends lines to the leader's followers' logs (a POI walk, a find).
func partyLog(leader *session.GameSession, lines ...string) {
	p := social.PartyOf(leader.Npub)
	if p == nil || p.Leader().Npub != leader.Npub {
		return
	}
	var to []string
	for _, m := range p.Members[1:] {
		to = append(to, m.Npub)
	}
	for _, l := range lines {
		if l != "" {
			systemLine(l, to...)
		}
	}
}

func applyPartySync(sess *session.GameSession, p session.PartySync) {
	s := &sess.SaveData
	wasLocation := s.Location
	s.Location, s.District, s.Building, s.Room = p.Location, p.District, p.Building, p.Room
	s.TravelProgress, s.TravelStopped = p.TravelProgress, p.TravelStopped

	// Clock: move forward through AdvanceTime (effects, hunger, fatigue tick on
	// your own character); a leader behind you just sets it.
	mine, theirs := s.CurrentDay*1440+s.TimeOfDay, p.CurrentDay*1440+p.TimeOfDay
	if theirs > mine {
		gametime.AdvanceTime(s, theirs-mine, p.AccrueFatigue)
	} else if theirs < mine {
		s.CurrentDay, s.TimeOfDay = p.CurrentDay, p.TimeOfDay
	}

	switch {
	case p.Sleep == "inn":
		npc.HandleSleepAction(s, sess, data.GetNPCIDsAtLocation)
	case p.Sleep == "camp":
		npc.HandleCampSleepAction(s, sess, data.GetNPCIDsAtLocation)
	case p.Sleep != "":
		npc.HandleWildernessSleepAction(s, sess, data.GetNPCIDsAtLocation)
	case p.Rest:
		npc.HandleRestAction(s, nil)
	}

	// Arriving somewhere new with the party discovers it for you too.
	if s.Location != wasLocation && travel.GetEnvironmentData(s.Location) == nil {
		travel.RecordArrival(s, s.Location)
	}
	// So does anything the leader found on the way.
	for _, id := range p.Discovered {
		if !alreadyDiscovered(s, id) {
			s.LocationsDiscovered = append(s.LocationsDiscovered, id)
			events.Record(s, events.LocationDiscovered, id, 1)
		}
	}
	for _, line := range p.Notes {
		if line != "" {
			systemLine(line, sess.Npub)
		}
	}

	// The world around you changed: refresh buildings and NPCs on the next tick.
	sess.NPCsAtLocation = nil
	sess.NPCsLastHour = -1
	sess.BuildingsLastCheck = 0
	if p.Note != "" {
		systemLine(p.Note, sess.Npub)
	}
}

// followerClock rewrites a follower's update_time to the leader's clock (from
// the leader's presence), so the follower's own client can't run ahead.
func followerClock(sess *session.GameSession, params map[string]any) {
	p := social.PartyOf(sess.Npub)
	if p == nil {
		return
	}
	abs := sess.SaveData.CurrentDay*1440 + sess.SaveData.TimeOfDay
	if e, ok := presence.Get(p.Leader().Npub); ok && e.AbsMinutes > abs {
		abs = e.AbsMinutes
	}
	params["current_day"] = float64(abs / 1440)
	params["time_of_day"] = float64(abs % 1440)
	// Followers don't act on their own, so never auto-pause them.
	sess.LastActionGameTime = abs
	sess.LastActionTime = time.Now().Unix()
}
