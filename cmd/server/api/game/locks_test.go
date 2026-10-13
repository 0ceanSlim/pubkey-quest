package game

import (
	"testing"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/locks"
	"pubkey-quest/cmd/server/session"
)

// Picking Bob's spare room at the Salty Anchor through the real actions:
// lock_begin / lock_act (the minigame) and pick_lock (the quick roll).

func spareRoomSession(t *testing.T, lockpicks int) *session.GameSession {
	t.Helper()
	sess := bobSession(t)
	slots := []interface{}{}
	for i := 0; i < 4; i++ {
		slots = append(slots, map[string]interface{}{"item": "", "quantity": 0})
	}
	if lockpicks > 0 {
		slots[0] = map[string]interface{}{"item": "lockpick", "quantity": lockpicks}
	}
	sess.SaveData.Inventory = map[string]interface{}{"general_slots": slots, "gear_slots": map[string]interface{}{}}
	sess.SaveData.Room = "taproom"
	return sess
}

func lockAction(t *testing.T, sess *session.GameSession, typ string, params map[string]any) *GameActionResponse {
	t.Helper()
	resp, err := processGameAction(sess, GameAction{Type: typ, Params: params})
	if err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
	return resp
}

func TestLockedDoorRefusesWithoutKeyOrPick(t *testing.T) {
	sess := spareRoomSession(t, 0)
	if r := lockAction(t, sess, "move_to_room", map[string]any{"room_id": "spare_room"}); r.Success {
		t.Fatal("walked into a locked room with no key")
	}
	if r := lockAction(t, sess, "lock_begin", map[string]any{"kind": "room", "room_id": "spare_room"}); r.Success {
		t.Fatal("began picking with no lockpicks")
	}
}

func TestTheKeyOpensTheDoor(t *testing.T) {
	sess := spareRoomSession(t, 0)
	slots := sess.SaveData.Inventory["general_slots"].([]interface{})
	slots[1] = map[string]interface{}{"item": "anchor-spare-key", "quantity": 1}
	if r := lockAction(t, sess, "move_to_room", map[string]any{"room_id": "spare_room"}); !r.Success {
		t.Fatalf("the key should open the spare room: %s", r.Message)
	}
}

func TestPickTheSpareRoomMinigame(t *testing.T) {
	sess := spareRoomSession(t, 20)
	r := lockAction(t, sess, "lock_begin", map[string]any{"kind": "room", "room_id": "spare_room"})
	if !r.Success || sess.LockSession == nil {
		t.Fatalf("lock_begin: %s", r.Message)
	}
	ls := sess.LockSession
	view := ls.View()
	if _, leaks := view["order"]; leaks {
		t.Fatal("the view leaks the binding order")
	}
	pins := view["pins"].(int)

	// A replayed action (old version) changes nothing.
	lockAction(t, sess, "lock_act", map[string]any{"session_id": ls.ID, "version": float64(ls.Version), "act": "lift", "pin": float64(0)})
	before := *ls
	stale := lockAction(t, sess, "lock_act", map[string]any{"session_id": ls.ID, "version": float64(ls.Version - 1), "act": "lift", "pin": float64(1)})
	if stale.Data["stale"] != true || ls.Version != before.Version || ls.Strain != before.Strain {
		t.Fatal("a stale lock_act was applied")
	}

	// Play it out the way a player without luck would: learn the order by
	// trying pins, remembering what set. Every broken pick must cost exactly one
	// lockpick.
	startPicks := gameutil.CountItem(&sess.SaveData, "lockpick")
	known := []int{}
	for steps := 0; steps < 400 && sess.LockSession != nil; steps++ {
		cur := sess.LockSession
		set := cur.View()["set"].([]bool)
		progress := 0
		for _, s := range set {
			if s {
				progress++
			}
		}
		pin := -1
		if progress < len(known) {
			pin = known[progress]
		} else {
			for p := 0; p < pins; p++ {
				if !set[p] && !contains(known, p) {
					pin = p
					break
				}
			}
		}
		brokenBefore := cur.Broken
		resp := lockAction(t, sess, "lock_act", map[string]any{"session_id": cur.ID, "version": float64(cur.Version), "act": "lift", "pin": float64(pin)})
		if !resp.Success {
			t.Fatalf("lock_act: %s", resp.Message)
		}
		after := resp.Data["lock"].(map[string]any)
		if after["set"].([]bool)[pin] && progress == len(known) {
			known = append(known, pin)
		}
		if cur.Broken > brokenBefore {
			known = known[:0] // the pins reseat
		}
	}
	if sess.LockSession != nil {
		t.Fatal("the lock never resolved")
	}
	used := startPicks - gameutil.CountItem(&sess.SaveData, "lockpick")
	if used != ls.Broken {
		t.Errorf("used %d lockpicks for %d broken picks", used, ls.Broken)
	}
	switch ls.Status {
	case locks.StatusOpened:
		if r := lockAction(t, sess, "move_to_room", map[string]any{"room_id": "spare_room"}); !r.Success {
			t.Fatalf("picked open but can't enter: %s", r.Message)
		}
		// The spare room's bed needs no rental.
		sess.SaveData.TimeOfDay = 1300
		if r := lockAction(t, sess, "sleep", nil); !r.Success {
			t.Errorf("can't sleep in the spare room: %s", r.Message)
		}
	case locks.StatusJammed:
		if r := lockAction(t, sess, "lock_begin", map[string]any{"kind": "room", "room_id": "spare_room"}); r.Success {
			t.Error("a jammed lock let picking start again")
		}
	default:
		t.Fatalf("ended %s", ls.Status)
	}
}

func TestOtherActionsAbandonTheLock(t *testing.T) {
	sess := spareRoomSession(t, 5)
	lockAction(t, sess, "lock_begin", map[string]any{"kind": "room", "room_id": "spare_room"})
	lockAction(t, sess, "move_to_room", map[string]any{"room_id": "galley"})
	if sess.LockSession != nil {
		t.Fatal("walking off should abandon the lock")
	}
}

func TestQuickPickUsesAPickOnFailure(t *testing.T) {
	sess := spareRoomSession(t, 30)
	for i := 0; i < 30; i++ {
		before := gameutil.CountItem(&sess.SaveData, "lockpick")
		r := lockAction(t, sess, "pick_lock", map[string]any{"room_id": "spare_room"})
		if !r.Success {
			break // jammed, or out of picks
		}
		opened, _ := r.Data["opened"].(bool)
		after := gameutil.CountItem(&sess.SaveData, "lockpick")
		if opened {
			if after != before {
				t.Error("an opening pick shouldn't break")
			}
			if !roomPickedOpen(sess, "spare_room") {
				t.Error("picked open isn't remembered")
			}
			return
		}
		if after != before-1 {
			t.Fatalf("a failed quick pick should use one lockpick: %d → %d", before, after)
		}
	}
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestQuestItemsCanOnlyBeDestroyed(t *testing.T) {
	sess := spareRoomSession(t, 0)
	slots := sess.SaveData.Inventory["general_slots"].([]interface{})
	slots[1] = map[string]interface{}{"item": "anchor-spare-key", "quantity": 1}
	p := map[string]any{"item_id": "anchor-spare-key", "from_slot": float64(1), "from_slot_type": "general", "quantity": float64(1)}
	for _, typ := range []string{"drop_item", "stage_sell"} {
		if r := lockAction(t, sess, typ, p); r.Success {
			t.Errorf("%s of a quest item went through", typ)
		}
	}
	if !gameutil.PlayerHasItem(&sess.SaveData, "anchor-spare-key") {
		t.Fatal("a refused action took the key")
	}
	if r := lockAction(t, sess, "destroy_item", p); !r.Success || gameutil.PlayerHasItem(&sess.SaveData, "anchor-spare-key") {
		t.Errorf("destroy should remove it: %+v", r)
	}
}

func TestKeysOnAKeyringStillOpenDoors(t *testing.T) {
	sess := spareRoomSession(t, 0)
	slots := sess.SaveData.Inventory["general_slots"].([]interface{})
	slots[2] = map[string]interface{}{"item": "keyring", "quantity": 1, "contents": []interface{}{
		map[string]interface{}{"item": "anchor-spare-key", "quantity": 1},
	}}
	if r := lockAction(t, sess, "move_to_room", map[string]any{"room_id": "spare_room"}); !r.Success {
		t.Fatalf("the key on the ring should open the door: %s", r.Message)
	}
}
