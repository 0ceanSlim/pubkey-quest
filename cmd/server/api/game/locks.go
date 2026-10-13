package game

import (
	"errors"
	"math/rand"
	"strings"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/locks"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── Locks ───────────────────────────────────────────────────────────────────
//
// Two kinds of lock: a room's door (building.RoomAccess.Lock) and a lock node in
// the player's current POI / quest-site walk. Either opens with its key, or —
// when it's pickable (DC > 0) — with a lockpick, two ways:
//
//   - pick_lock / a lock node's "#quick": one Thieving roll (locks.Pick)
//   - lock_begin + lock_act: the Tumblers minigame (locks.Session)
//
// Both share the rules: a failure breaks one lockpick, a broken pick runs the
// same jam check, and every pick used costs locks.PickMinutes of game time.
// Picked-open and jammed state lasts the in-game day (session memory).

// lockTarget is a resolved lock: what it is, what to call it, and its id in the
// session's lock memory.
type lockTarget struct {
	target locks.Target
	lock   types.Lock
	name   string
	id     string
}

// resolveLockTarget finds the lock a request names and checks the player is at it.
func resolveLockTarget(sess *session.GameSession, kind, roomID, node string) (*lockTarget, string) {
	state := &sess.SaveData
	switch kind {
	case "room":
		if state.Building == "" || roomID == "" {
			return nil, "There's no lock here to pick."
		}
		rooms, _, err := building.GetBuildingRooms(serverdb.GetDB(), state.Location, state.Building)
		if err != nil {
			return nil, "There's no lock here to pick."
		}
		room, found := building.FindRoom(rooms, roomID)
		if !found || room.Access == nil || room.Access.Lock == nil {
			return nil, "There's no lock on that door."
		}
		return &lockTarget{
			target: locks.Target{Kind: "room", RoomID: roomID},
			lock:   *room.Access.Lock, name: room.Name, id: roomLockID(state, roomID),
		}, ""
	case "poi":
		if sess.ActivePOI == nil || node == "" || poi.BaseNode(sess.ActivePOI.CurrentNode) != node {
			return nil, "You're not at that lock."
		}
		n, ok := sess.ActivePOI.Nodes[node]
		if !ok || n.Type != types.POIStepLock || n.Lock == nil {
			return nil, "There's no lock there."
		}
		return &lockTarget{
			target: locks.Target{Kind: "poi", Node: node},
			lock:   *n.Lock, name: sess.ActivePOI.Title, id: poiLockID(sess, node),
		}, ""
	}
	return nil, "Pick what?"
}

// roomLockID names a room's lock in the session's lock memory.
func roomLockID(save *types.SaveFile, roomID string) string {
	return "room:" + save.Location + "/" + save.Building + "/" + roomID
}

// poiLockID names a lock node of the current walk in the lock memory.
func poiLockID(sess *session.GameSession, node string) string {
	return "poi:" + sess.ActivePOI.POIID + "/" + node
}

func lockMemory(sess *session.GameSession) locks.Memory {
	if sess.Locks == nil {
		sess.Locks = locks.Memory{}
	}
	return sess.Locks
}

// roomPickedOpen reports whether the player picked this room's lock today.
func roomPickedOpen(sess *session.GameSession, roomID string) bool {
	return lockMemory(sess).IsOpen(roomLockID(&sess.SaveData, roomID), sess.SaveData.CurrentDay)
}

// thievesKit adds to Thieving while carried (its item skill_bonus); the window shows it.
const thievesKit = "thieves-kit"

func lockRng() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }

func thievingOf(state *types.SaveFile) int { return buildQuestContext(state).SkillValue("thieving") }

// ─── Quick attempt ───────────────────────────────────────────────────────────

// quickPick makes one quick attempt at a lock: time passes, a failure uses up a
// lockpick, and the result is remembered for the day.
func quickPick(sess *session.GameSession, id string, lock types.Lock) locks.Result {
	state := &sess.SaveData
	gametime.AdvanceTime(state, locks.PickMinutes, true)
	r := locks.Pick(lock, thievingOf(state), lockRng())
	if !r.Opened {
		gameutil.ConsumeItem(state, locks.Lockpick, 1)
	}
	lockMemory(sess).Set(id, state.CurrentDay, r.Opened, r.Jammed)
	return r
}

// handlePickLockAction makes one quick attempt at a room's lock.
// params: room_id
func handlePickLockAction(sess *session.GameSession, params map[string]any) (*types.GameActionResponse, error) {
	state := &sess.SaveData
	roomID, _ := params["room_id"].(string)
	lt, why := resolveLockTarget(sess, "room", roomID, "")
	if lt == nil {
		return &types.GameActionResponse{Success: false, Message: why, Color: "yellow"}, nil
	}
	mem, day := lockMemory(sess), state.CurrentDay
	if mem.IsOpen(lt.id, day) {
		return &types.GameActionResponse{Success: true, Message: "It's already open.", Color: "blue"}, nil
	}
	if why := locks.CanPick(lt.lock, mem.IsJammed(lt.id, day), gameutil.CountItem(state, locks.Lockpick)); why != "" {
		return &types.GameActionResponse{Success: false, Message: why, Color: "yellow"}, nil
	}
	r := quickPick(sess, lt.id, lt.lock)
	color := "yellow"
	if r.Opened {
		color = "green"
	}
	return &types.GameActionResponse{
		Success: true, Message: r.Describe(), Color: color,
		Data: map[string]interface{}{"opened": r.Opened, "jammed": r.Jammed, "room_id": roomID},
	}, nil
}

// poiLocks serves the walker's lock nodes from the session.
type poiLocks struct{ sess *session.GameSession }

func (p poiLocks) State(node string) (bool, bool) {
	mem, id, day := lockMemory(p.sess), poiLockID(p.sess, node), p.sess.SaveData.CurrentDay
	return mem.IsOpen(id, day), mem.IsJammed(id, day)
}
func (p poiLocks) Picks() int { return gameutil.CountItem(&p.sess.SaveData, locks.Lockpick) }
func (p poiLocks) Quick(node string, lock types.Lock) locks.Result {
	return quickPick(p.sess, poiLockID(p.sess, node), lock)
}
func (p poiLocks) ItemName(id string) string {
	if item, err := serverdb.GetItemByID(id); err == nil && item != nil {
		return item.Name
	}
	return ""
}

// ─── The minigame ────────────────────────────────────────────────────────────

// handleLockBegin starts picking a lock as a minigame.
// params: kind ("room"|"poi"), room_id | node, style (default tumblers)
func handleLockBegin(sess *session.GameSession, params map[string]any) (*types.GameActionResponse, error) {
	state := &sess.SaveData
	kind, _ := params["kind"].(string)
	roomID, _ := params["room_id"].(string)
	node, _ := params["node"].(string)
	style, _ := params["style"].(string)

	lt, why := resolveLockTarget(sess, kind, roomID, node)
	if lt == nil {
		return &types.GameActionResponse{Success: false, Message: why, Color: "yellow"}, nil
	}
	mem, day := lockMemory(sess), state.CurrentDay
	if mem.IsOpen(lt.id, day) {
		return &types.GameActionResponse{Success: false, Message: "It's already open.", Color: "blue"}, nil
	}
	picks := gameutil.CountItem(state, locks.Lockpick)
	if why := locks.CanPick(lt.lock, mem.IsJammed(lt.id, day), picks); why != "" {
		return &types.GameActionResponse{Success: false, Message: why, Color: "yellow"}, nil
	}
	ls, err := locks.NewSession(lt.id, lt.name, lt.target, locks.Style(style), lt.lock, thievingOf(state), lockRng())
	if err != nil {
		return &types.GameActionResponse{Success: false, Message: err.Error(), Color: "yellow"}, nil
	}
	sess.LockSession = ls
	return &types.GameActionResponse{
		Success: true, Color: "blue",
		Message: "You slide the tension wrench in and feel for the pins.",
		Data:    map[string]interface{}{"lock": ls.View(), "picks": picks, "kit": gameutil.PlayerHasItem(state, thievesKit)},
	}, nil
}

// handleLockAct resolves one minigame move and applies its consequences.
// params: session_id, version, act ("lift"|"inspect"|"give_up"), pin
func handleLockAct(sess *session.GameSession, params map[string]any) (*types.GameActionResponse, error) {
	state := &sess.SaveData
	ls := sess.LockSession
	id, _ := params["session_id"].(string)
	if ls == nil || id == "" || id != ls.ID {
		return &types.GameActionResponse{Success: false, Message: "You're not picking that lock any more.", Color: "yellow"}, nil
	}
	act, _ := params["act"].(string)
	version := paramIntOr(params["version"], -1)
	pin := paramIntOr(params["pin"], -1)

	picks := gameutil.CountItem(state, locks.Lockpick)
	data := map[string]interface{}{}
	if act != "give_up" && picks <= 0 {
		ls.OutOfPicks()
		return finishLockAct(sess, ls, locks.Outcome{Lines: []string{"You have no lockpicks left."}}, data), nil
	}

	out, err := ls.Act(locks.Action{Kind: act, Pin: pin}, version)
	if errors.Is(err, locks.ErrStale) {
		// Already handled (a double tap, a retried request): send the current
		// state back and change nothing.
		data["lock"], data["picks"], data["stale"] = ls.View(), picks, true
		return &types.GameActionResponse{Success: true, Data: data}, nil
	}
	if err != nil {
		return &types.GameActionResponse{Success: false, Message: err.Error(), Color: "yellow"}, nil
	}

	// Consequences, applied once: the session has moved past this version.
	mem, day := lockMemory(sess), state.CurrentDay
	if out.PickBroke {
		gameutil.ConsumeItem(state, locks.Lockpick, 1)
		gametime.AdvanceTime(state, locks.PickMinutes, true)
		if gameutil.CountItem(state, locks.Lockpick) == 0 && !out.Jammed {
			ls.OutOfPicks()
			out.Lines = append(out.Lines, "That was your last lockpick.")
		}
	}
	switch {
	case out.Opened:
		gametime.AdvanceTime(state, locks.PickMinutes, true)
		mem.Set(ls.LockID, day, true, false)
	case out.Jammed:
		mem.Set(ls.LockID, day, false, true)
	case out.GaveUp && out.PickWorked:
		gametime.AdvanceTime(state, locks.PickMinutes, true)
	}
	return finishLockAct(sess, ls, out, data), nil
}

// finishLockAct builds the response, closes a finished session, and for a lock
// in a walk moves the walk on: through the door when it opened, back to the
// lock's choices otherwise.
func finishLockAct(sess *session.GameSession, ls *locks.Session, out locks.Outcome, data map[string]interface{}) *types.GameActionResponse {
	data["lock"] = ls.View()
	data["picks"] = gameutil.CountItem(&sess.SaveData, locks.Lockpick)
	data["kit"] = gameutil.PlayerHasItem(&sess.SaveData, thievesKit)
	data["lines"] = out.Lines
	if ls.Status != locks.StatusActive {
		sess.LockSession = nil
		if ls.Target.Kind == "poi" && sess.ActivePOI != nil {
			if res, err := stepPOI(sess, ls.Target.Node, data); err == nil {
				data["step"] = res
			}
		}
	}
	color := "yellow"
	if out.Opened {
		color = "green"
	}
	return &types.GameActionResponse{Success: true, Message: strings.Join(out.Lines, " "), Color: color, Data: data}
}

