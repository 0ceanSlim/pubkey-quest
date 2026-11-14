package game

import (
	"strings"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
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
	if err != nil || len(res.Choices) != 2 || res.Choices[0].Next != "face"+poi.MineQuick {
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
	sess.Seams = nil
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
	if sess.Exertion != 0 || sess.SaveData.Fatigue < 2 {
		t.Errorf("three tier-1 faces: exertion %d left, fatigue %d; want 0 and at least 2", sess.Exertion, sess.SaveData.Fatigue)
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
