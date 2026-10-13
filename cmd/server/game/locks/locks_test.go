package locks

import (
	"math/rand"
	"strings"
	"testing"

	"pubkey-quest/types"
)

// These tests sit inside the package so they can read the hidden binding order
// (the client never can) and drive exact sequences with a seeded rng.

func session(t *testing.T, lock types.Lock, thieving int, seed int64) (*Session, *tumblers) {
	t.Helper()
	s, err := NewSession("room:x", "Test Door", Target{Kind: "room", RoomID: "r"}, StyleTumblers, lock, thieving, rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatal(err)
	}
	return s, s.puzzle.(*tumblers)
}

func lift(t *testing.T, s *Session, pin int) Outcome {
	t.Helper()
	out, err := s.Act(Action{Kind: "lift", Pin: pin}, s.Version)
	if err != nil {
		t.Fatalf("lift %d: %v", pin, err)
	}
	return out
}

// wrongPin is an unset pin that isn't binding.
func wrongPin(tm *tumblers) int {
	for p := 0; p < tm.pins; p++ {
		if !tm.set[p] && p != tm.order[tm.progress] {
			return p
		}
	}
	return -1
}

func TestPinsAndBudgetFromDCAndThieving(t *testing.T) {
	for dc, want := range map[int]int{8: 3, 10: 3, 12: 4, 15: 5, 18: 6} {
		if got := pinsFor(dc); got != want {
			t.Errorf("DC %d: %d pins, want %d", dc, got, want)
		}
	}
	tm := newTumblers(12) // 4 pins
	if got := tm.strainBudget(10); got != 3 {
		t.Errorf("Thieving 10 budget %d, want 3", got)
	}
	if got := tm.strainBudget(16); got != 6 {
		t.Errorf("Thieving 16 (+3) budget %d, want 6", got)
	}
	if got := tm.strainBudget(2); got != 2 {
		t.Errorf("budget floor is 2, got %d", got)
	}
}

func TestSettingEveryPinInOrderOpens(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 12}, 10, 1)
	order := append([]int(nil), tm.order...)
	for i, p := range order {
		out := lift(t, s, p)
		if i < len(order)-1 && (out.Opened || !tm.set[p]) {
			t.Fatalf("pin %d should set without opening: %+v", p, out)
		}
		if i == len(order)-1 && !out.Opened {
			t.Fatalf("last pin should open the lock: %+v", out)
		}
	}
	if s.Status != StatusOpened || s.Strain != 0 || s.Version != len(order) {
		t.Errorf("status=%s strain=%d version=%d", s.Status, s.Strain, s.Version)
	}
}

func TestWrongPinStrainsAndDropsSetPins(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 12}, 10, 2)
	lift(t, s, tm.order[0])
	out := lift(t, s, wrongPin(tm))
	if out.PickBroke || s.Strain != 1 {
		t.Fatalf("one mistake: strain=%d %+v", s.Strain, out)
	}
	for p, set := range tm.set {
		if set {
			t.Errorf("pin %d still set after a slip", p)
		}
	}
	if tm.progress != 0 {
		t.Error("progress should reset after a slip")
	}
}

func TestStrainLimitBreaksPickAndReseats(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 12}, 10, 3) // budget 3
	var out Outcome
	for i := 0; i < s.StrainMax; i++ {
		out = lift(t, s, wrongPin(tm))
	}
	if !out.PickBroke || out.Jammed || s.Broken != 1 {
		t.Fatalf("budget spent: %+v broken=%d", out, s.Broken)
	}
	if s.Status != StatusActive || s.Strain != 0 {
		t.Errorf("next pick should start fresh: status=%s strain=%d", s.Status, s.Strain)
	}
}

func TestBrokenPickCanJam(t *testing.T) {
	// DC 30 with Thieving 1 (−5) can never pass the jam check's roll, so any
	// break on a lock that jams on a miss of 1 jams it.
	s, tm := session(t, types.Lock{DC: 30, JamOnMiss: 1}, 1, 4)
	var out Outcome
	for s.Status == StatusActive {
		out = lift(t, s, wrongPin(tm))
	}
	if !out.PickBroke || !out.Jammed || s.Status != StatusJammed {
		t.Fatalf("expected a jam: %+v status=%s", out, s.Status)
	}
	if _, err := s.Act(Action{Kind: "lift", Pin: 0}, s.Version); err == nil {
		t.Error("a jammed lock must refuse further picking")
	}
}

func TestNoJamWhenTheLockDoesntJam(t *testing.T) {
	if JamCheck(types.Lock{DC: 30}, 1, rand.New(rand.NewSource(1))) {
		t.Error("JamOnMiss 0 means it never jams")
	}
}

func TestStaleAndDuplicateActionsAreRefused(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 12}, 10, 5)
	v := s.Version
	lift(t, s, tm.order[0])
	// The same request again (old version): refused, nothing changes.
	before := s.Strain
	if _, err := s.Act(Action{Kind: "lift", Pin: wrongPin(tm)}, v); err != ErrStale {
		t.Fatalf("replay: err=%v", err)
	}
	if s.Strain != before || s.Version != v+1 {
		t.Error("a stale action changed the session")
	}
}

func TestInvalidActionsChangeNothing(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 12}, 10, 6)
	lift(t, s, tm.order[0])
	v := s.Version
	for _, a := range []Action{{Kind: "lift", Pin: 9}, {Kind: "lift", Pin: -1}, {Kind: "lift", Pin: tm.order[0]}, {Kind: "dance"}} {
		if _, err := s.Act(a, v); err == nil {
			t.Errorf("%+v should be refused", a)
		}
	}
	if s.Version != v || s.Strain != 0 {
		t.Error("refused actions changed the session")
	}
}

func TestNothingAfterTheLockOpens(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 8}, 10, 7)
	for _, p := range append([]int(nil), tm.order...) {
		lift(t, s, p)
	}
	if _, err := s.Act(Action{Kind: "give_up"}, s.Version); err == nil {
		t.Error("an opened session must refuse further actions")
	}
}

func TestGiveUp(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 12}, 10, 8)
	out, err := s.Act(Action{Kind: "give_up"}, s.Version)
	if err != nil || !out.GaveUp || out.PickWorked || s.Status != StatusAbandoned {
		t.Fatalf("untried give up: %+v %v", out, err)
	}
	s, tm = session(t, types.Lock{DC: 12}, 10, 8)
	lift(t, s, wrongPin(tm))
	out, _ = s.Act(Action{Kind: "give_up"}, s.Version)
	if !out.PickWorked {
		t.Error("giving up after working the pick should cost the time spent")
	}
}

func TestInspectOncePerPickAndOnlyRulesOutWrongPins(t *testing.T) {
	s, tm := session(t, types.Lock{DC: 8}, 30, 9) // Thieving 30 always passes DC 8
	out, err := s.Act(Action{Kind: "inspect"}, s.Version)
	if err != nil {
		t.Fatal(err)
	}
	ruled := -1
	for p, r := range tm.ruledOut {
		if r {
			ruled = p
		}
	}
	if ruled < 0 || ruled == tm.order[0] || !strings.Contains(out.Lines[0], "isn't the one binding") {
		t.Fatalf("inspect ruled out %d (binding %d): %v", ruled, tm.order[0], out.Lines)
	}
	if _, err := s.Act(Action{Kind: "inspect"}, s.Version); err == nil {
		t.Error("a second inspect with the same pick should be refused")
	}
}

func TestGlintOnlyWithSkillAndOnlyOnTheBindingPin(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		s, tm := session(t, types.Lock{DC: 18}, 10, seed) // +0: never glints
		if tm.glint != -1 {
			t.Fatal("Thieving +0 should never glint")
		}
		s, tm = session(t, types.Lock{DC: 18}, 20, seed) // +5: often glints
		for s.Status == StatusActive && tm.progress < tm.pins {
			if tm.glint >= 0 && tm.glint != tm.order[tm.progress] {
				t.Fatalf("glint on %d, binding %d", tm.glint, tm.order[tm.progress])
			}
			lift(t, s, tm.order[tm.progress])
		}
	}
}

func TestViewNeverCarriesTheSolution(t *testing.T) {
	s, _ := session(t, types.Lock{DC: 15}, 10, 10)
	for k := range s.View() {
		if k == "order" || k == "progress" || k == "solution" {
			t.Errorf("view leaks %q", k)
		}
	}
}

func TestUnpickableLock(t *testing.T) {
	if _, err := NewSession("x", "x", Target{}, StyleTumblers, types.Lock{Key: "k"}, 10, rand.New(rand.NewSource(1))); err == nil {
		t.Error("a DC 0 lock can't be picked")
	}
}

func TestMemoryLastsTheDay(t *testing.T) {
	m := Memory{}
	m.Set("a", 3, true, false)
	if !m.IsOpen("a", 3) || m.IsOpen("a", 4) {
		t.Error("picked open should last only that day")
	}
	m.Set("b", 3, false, true)
	if !m.IsJammed("b", 3) || m.IsJammed("b", 4) {
		t.Error("jammed should last only that day")
	}
}
