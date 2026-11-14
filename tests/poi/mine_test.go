package poi_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/types"
)

// fakeMining serves mine nodes for the walker tests.
type fakeMining struct {
	left    int
	cant    string
	success bool
	quicks  int
}

func (f *fakeMining) Seam(s types.Seam) types.Seam {
	s.DC = 10
	return s
}
func (f *fakeMining) FacesLeft(string, types.Seam) int { return f.left }
func (f *fakeMining) CantMine(types.Seam) string       { return f.cant }
func (f *fakeMining) QuickMinutes() int                { return 30 }
func (f *fakeMining) Quick(string, types.Seam) poi.MineResult {
	f.quicks++
	f.left--
	return poi.MineResult{Success: f.success, Lines: []string{"rolled"}}
}

func mineNode() types.POIStep {
	return types.POIStep{Type: types.POIStepMine, Text: "A seam.", Next: "leave", Seam: &types.Seam{
		Tier: 1, Ores: []types.SeamOre{{Item: "iron-ore", Weight: 1, Quantity: [2]int{2, 2}}},
	}}
}

func TestMineNodeOffersAFace(t *testing.T) {
	deps := poi.Deps{Ctx: fakeCtx{}, Rng: rng(), Mining: &fakeMining{left: 3}}
	res := poi.Resolve(mineNode(), "face", &types.SaveFile{}, deps)
	got := strings.Join(labels(res), "|")
	if got != "Work the face (Athletics DC 10, 30 minutes; 3 faces left today)|Move on" {
		t.Fatalf("choices: %s", got)
	}
	if res.Choices[0].Next != "face#quick" || res.Choices[1].Next != "leave" {
		t.Errorf("nexts: %+v", res.Choices)
	}
}

func TestMineNodeQuickWork(t *testing.T) {
	got := map[string]int{}
	fm := &fakeMining{left: 2, success: true}
	deps := poi.Deps{Ctx: fakeCtx{}, Rng: rng(), Mining: fm,
		AddItem: func(_ *types.SaveFile, id string, n int) { got[id] += n }}
	res := poi.Resolve(mineNode(), "face#quick", &types.SaveFile{}, deps)
	if fm.quicks != 1 || got["iron-ore"] < 2 {
		t.Fatalf("a good face should grant ore: quicks %d, items %v", fm.quicks, got)
	}
	if !strings.Contains(labels(res)[0], "1 face left") {
		t.Errorf("faces left after one: %v", labels(res))
	}

	// A failed face grants nothing; the last face leaves only Move on.
	got = map[string]int{}
	fm.success = false
	res = poi.Resolve(mineNode(), "face#quick", &types.SaveFile{}, deps)
	if len(got) != 0 {
		t.Errorf("a failed face granted %v", got)
	}
	if l := strings.Join(labels(res), "|"); l != "Move on" || !strings.Contains(strings.Join(res.Outcome, " "), "worked out") {
		t.Errorf("worked out: %s %v", l, res.Outcome)
	}
	// A spent seam can't be worked again.
	poi.Resolve(mineNode(), "face#quick", &types.SaveFile{}, deps)
	if fm.quicks != 2 {
		t.Errorf("worked a spent seam: %d quicks", fm.quicks)
	}
}

func TestMineNodeNeedsAPick(t *testing.T) {
	fm := &fakeMining{left: 3, cant: "You'd need a pick."}
	deps := poi.Deps{Ctx: fakeCtx{}, Rng: rng(), Mining: fm}
	res := poi.Resolve(mineNode(), "face#quick", &types.SaveFile{}, deps)
	if fm.quicks != 0 || strings.Join(labels(res), "|") != "Move on" || !strings.Contains(strings.Join(res.Outcome, " "), "pick") {
		t.Errorf("no pick: %d quicks, %v %v", fm.quicks, labels(res), res.Outcome)
	}
}
