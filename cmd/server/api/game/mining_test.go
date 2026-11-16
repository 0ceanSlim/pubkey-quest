package game

import (
	"strings"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/mining"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/types"
)

// Working the Ironvein Seam through the real POI walk.
func TestIronveinFacesRunOut(t *testing.T) {
	sess := wharfSession(t, "pick-miners")
	seam, err := serverdb.GetPOIByID("ironvein-seam")
	if err != nil {
		t.Fatal(err)
	}
	sess.ActivePOI = &poi.Session{POIID: seam.ID, Title: seam.Name, Nodes: seam.Nodes, CurrentNode: seam.StartNode}
	state := &sess.SaveData
	start := state.CurrentDay*1440 + state.TimeOfDay

	res, err := stepPOI(sess, "face", map[string]any{})
	if err != nil || len(res.Choices) != 3 || res.Choices[1].Next != "face"+poi.MineQuick {
		t.Fatalf("the face should offer work and moving on: %+v %v", res, err)
	}
	for i := range 3 {
		res, err = stepPOI(sess, "face"+poi.MineQuick, map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(res.Outcome, " "), "Athletics check") {
			t.Fatalf("face %d: no roll in %v", i+1, res.Outcome)
		}
	}
	if spent := state.CurrentDay*1440 + state.TimeOfDay - start; spent != 90 {
		t.Errorf("three faces took %d minutes, want 90", spent)
	}
	if len(res.Choices) != 1 || !strings.Contains(strings.Join(res.Outcome, " "), "worked out") {
		t.Fatalf("after three faces the seam should be worked out: %+v", res)
	}
	ore := gameutil.CountItem(state, "iron-ore")
	stepPOI(sess, "face"+poi.MineQuick, map[string]any{})
	if gameutil.CountItem(state, "iron-ore") != ore || state.CurrentDay*1440+state.TimeOfDay-start != 90 {
		t.Error("a worked-out seam was worked again")
	}

	// No pick, no work.
	sess.SaveData.Lim().Seams = nil
	gameutil.ConsumeItem(state, "pick-miners", 5)
	res, _ = stepPOI(sess, "face", map[string]any{})
	if len(res.Choices) != 1 || !strings.Contains(strings.Join(res.Outcome, " "), "pick") {
		t.Errorf("without a pick: %+v", res)
	}
}

// Each face is exertion (tier 1: 4 of 6 per fatigue), and a good one pays XP.
func TestAFaceTiresYou(t *testing.T) {
	sess := wharfSession(t, "pick-miners")
	sess.ActivePOI = &poi.Session{POIID: "ironvein-seam"}
	sess.SaveData.Fatigue = 0
	p := poiMining{sess}
	seam := p.Seam(*mustSeam(t))
	xp0 := sess.SaveData.Experience
	good := false
	for range 3 {
		good = p.Quick("face", seam).Success || good
	}
	if sess.SaveData.Lim().Exertion != 0 || sess.SaveData.Fatigue < 2 {
		t.Errorf("three tier-1 faces: exertion %d left, fatigue %d; want 0 and at least 2", sess.SaveData.Lim().Exertion, sess.SaveData.Fatigue)
	}
	if good != (sess.SaveData.Experience > xp0) {
		t.Errorf("XP %d → %d with a good face: %v", xp0, sess.SaveData.Experience, good)
	}
}

func mustSeam(t *testing.T) *types.Seam {
	t.Helper()
	seam, err := serverdb.GetPOIByID("ironvein-seam")
	if err != nil || seam.Nodes["face"].Seam == nil {
		t.Fatalf("ironvein-seam has no face: %v", err)
	}
	return seam.Nodes["face"].Seam
}

// The Veins minigame through the real actions: begin, a stale replay, strike
// until the arms give out, and the walk comes back to the face.
func TestVeinsThroughTheActions(t *testing.T) {
	sess := wharfSession(t, "pick-miners")
	seam, err := serverdb.GetPOIByID("ironvein-seam")
	if err != nil {
		t.Fatal(err)
	}
	sess.ActivePOI = &poi.Session{POIID: seam.ID, Title: seam.Name, Nodes: seam.Nodes, CurrentNode: "face"}
	state := &sess.SaveData
	state.HP = 999 // a cave-in mustn't end the test
	start := state.CurrentDay*1440 + state.TimeOfDay

	r := fishAction(t, sess, "mine_begin", map[string]any{"node": "face"})
	if !r.Success || sess.MineSession == nil {
		t.Fatalf("mine_begin: %s", r.Message)
	}
	ms := sess.MineSession
	view := r.Data["mine"].(map[string]any)
	if view["status"] != mining.StatusActive || len(view["cells"].([]map[string]any)) != 20 {
		t.Fatalf("view: %v", view)
	}

	// Strike every cell in turn until the face ends.
	ended := false
	for cell := 0; cell < 20 && !ended; cell++ {
		r = fishAction(t, sess, "mine_act", map[string]any{"session_id": ms.ID, "version": float64(ms.Version), "act": "strike", "cell": float64(cell)})
		if !r.Success {
			t.Fatalf("strike %d: %s", cell, r.Message)
		}
		if cell == 0 {
			if left := seamMemory(sess).Left(poiMining{sess}.seamID("face"), state.CurrentDay, 3); left != 2 {
				t.Fatalf("the first move should use a face: %d left", left)
			}
			stale := fishAction(t, sess, "mine_act", map[string]any{"session_id": ms.ID, "version": float64(0), "act": "strike", "cell": float64(1)})
			if stale.Data["stale"] != true {
				t.Fatal("a replayed move wasn't stale")
			}
		}
		ended = r.Data["step"] != nil
	}
	if !ended || sess.MineSession != nil {
		t.Fatal("the face never ended")
	}
	if ms.Gained != gameutil.CountItem(state, "iron-ore")+gameutil.CountItem(state, "copper-ore") {
		t.Errorf("gained %d ore, pack has iron %d copper %d", ms.Gained, gameutil.CountItem(state, "iron-ore"), gameutil.CountItem(state, "copper-ore"))
	}
	if spent := state.CurrentDay*1440 + state.TimeOfDay - start; spent < 5 {
		t.Errorf("working the face took %d minutes", spent)
	}

	// Doing anything else walks away from a face.
	fishAction(t, sess, "mine_begin", map[string]any{"node": "face"})
	fishAction(t, sess, "wait", map[string]any{"minutes": float64(5)})
	if sess.MineSession != nil {
		t.Error("the face survived another action")
	}
}

// The Flooded Mine's adit: the way in needs a light, and so does reading the
// rock at its face. Every strike wears the pick, and a worked face is the
// "mine-face" a daily asks for.
func TestTheAditNeedsALight(t *testing.T) {
	sess := wharfSession(t, "pick-miners")
	mine, err := serverdb.GetPOIByID("flooded-mine")
	if err != nil {
		t.Fatal(err)
	}
	sess.ActivePOI = &poi.Session{POIID: mine.ID, Title: mine.Name, Nodes: mine.Nodes, CurrentNode: "mine-shaft-bottom"}
	state := &sess.SaveData
	state.HP = 999
	gear := state.Inventory["gear_slots"].(map[string]interface{})

	res, err := stepPOI(sess, "adit-entry", map[string]any{})
	if err != nil || res.Next != "mine-shaft-bottom" {
		t.Fatalf("no light should turn you back: %+v %v", res, err)
	}
	gear["offhand"] = map[string]interface{}{"item": "torch", "quantity": 1, "l": 1, "b": 60}
	if res, _ = stepPOI(sess, "adit-entry", map[string]any{}); res.Next != "adit-loose-rock" {
		t.Fatalf("with a lit torch the adit should open: %+v", res)
	}

	// At the face in the dark: no soundings, but you can swing.
	delete(gear, "offhand")
	sess.ActivePOI.CurrentNode = "adit-face"
	r := fishAction(t, sess, "mine_begin", map[string]any{"node": "adit-face"})
	if !r.Success || r.Data["mine"].(map[string]any)["dark"] != true {
		t.Fatalf("mine_begin in the dark: %+v", r)
	}
	ms := sess.MineSession
	if r = fishAction(t, sess, "mine_act", map[string]any{"session_id": ms.ID, "version": float64(ms.Version), "act": "sound", "cell": float64(0)}); r.Success {
		t.Fatal("sounded the rock in the dark")
	}
	// Light the torch and the rock can be read again.
	gear["offhand"] = map[string]interface{}{"item": "torch", "quantity": 1, "l": 1, "b": 60}
	if r = fishAction(t, sess, "mine_act", map[string]any{"session_id": ms.ID, "version": float64(ms.Version), "act": "sound", "cell": float64(0)}); !r.Success {
		t.Fatalf("sounding by torchlight: %s", r.Message)
	}
	r = fishAction(t, sess, "mine_act", map[string]any{"session_id": ms.ID, "version": float64(ms.Version), "act": "strike", "cell": float64(1)})
	if !r.Success {
		t.Fatalf("strike: %s", r.Message)
	}
	slot := state.Inventory["general_slots"].([]interface{})[0].(map[string]interface{})
	if condition.WearOf(slot) < 1 {
		t.Errorf("the strike didn't wear the pick: %v", slot)
	}
	if burn := gear["offhand"].(map[string]interface{})["b"]; burn == 60 {
		t.Error("the torch didn't burn while you worked")
	}
}
