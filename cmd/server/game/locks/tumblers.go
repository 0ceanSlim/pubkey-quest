package locks

import (
	"fmt"

	"pubkey-quest/cmd/server/game/skillcheck"
)

// ─── Tumblers ────────────────────────────────────────────────────────────────
//
// A pin-tumbler lock: 3–6 pins (by DC), set in a hidden binding order. Only the
// binding pin will lift; lift it and it sets with a click, and the next pin in
// the order starts to bind. Lift any other pin and the set pins drop back down
// and the pick takes strain. Too much strain and the pick snaps; the next pick
// meets a reseated lock (a new order).
//
// So it's deduction: every wrong lift tells you a pin that isn't binding at
// that point, and once you know the order you can re-set it at no cost. The
// pick's strain budget is what a character's Thieving buys:
//
//	budget = pins − 1 + Thieving modifier (minimum 2)
//
// At Thieving modifier +2 and up the binding pin sometimes glints, and inspect
// (a Thieving check vs the DC, once per pick) rules out one pin. Neither solves
// the lock for you.

type tumblers struct {
	pins     int
	order    []int  // hidden: pin indices in binding order
	progress int    // pins set so far
	set      []bool // public
	ruledOut []bool // public: pins inspect showed aren't binding right now
	glint    int    // public: a pin glinting this turn (-1 none)
}

// pinsFor maps a lock's DC to its pin count.
func pinsFor(dc int) int {
	switch {
	case dc <= 10:
		return 3
	case dc <= 13:
		return 4
	case dc <= 16:
		return 5
	}
	return 6
}

func newTumblers(dc int) *tumblers {
	return &tumblers{pins: pinsFor(dc), glint: -1}
}

func (t *tumblers) strainBudget(thieving int) int {
	return max(2, t.pins-1+skillcheck.Modifier(thieving))
}

func (t *tumblers) reseat(s *Session) {
	t.order = s.rng.Perm(t.pins)
	t.progress = 0
	t.set = make([]bool, t.pins)
	t.ruledOut = make([]bool, t.pins)
	t.rollGlint(s)
}

func (t *tumblers) lift(s *Session, pin int) (mistake, opened bool, line string, err error) {
	if pin < 0 || pin >= t.pins {
		return false, false, "", fmt.Errorf("there's no pin %d", pin+1)
	}
	if t.set[pin] {
		return false, false, "", fmt.Errorf("pin %d is already set", pin+1)
	}
	if pin == t.order[t.progress] {
		t.set[pin] = true
		t.progress++
		t.ruledOut = make([]bool, t.pins) // a new pin binds now
		if t.progress == t.pins {
			t.glint = -1
			return false, true, fmt.Sprintf("Click. Pin %d sets, and the cylinder turns. The lock is open.", pin+1), nil
		}
		t.rollGlint(s)
		return false, false, fmt.Sprintf("Click. Pin %d sets.", pin+1), nil
	}

	dropped := t.progress > 0
	t.progress = 0
	t.set = make([]bool, t.pins)
	t.ruledOut = make([]bool, t.pins)
	t.rollGlint(s)
	if dropped {
		return true, false, fmt.Sprintf("Pin %d won't budge. The tension slips and your set pins drop.", pin+1), nil
	}
	return true, false, fmt.Sprintf("Pin %d won't budge. The pick strains.", pin+1), nil
}

// inspect rules out one unset pin that isn't binding right now.
func (t *tumblers) inspect(s *Session) string {
	var candidates []int
	for p := 0; p < t.pins; p++ {
		if !t.set[p] && !t.ruledOut[p] && p != t.order[t.progress] {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return "You study the pins. Nothing new: you already know what there is to know."
	}
	p := candidates[s.rng.Intn(len(candidates))]
	t.ruledOut[p] = true
	return fmt.Sprintf("You tilt the lock to the light. Pin %d sits loose in its shaft: it isn't the one binding.", p+1)
}

// rollGlint: at Thieving modifier +2 and up, the binding pin sometimes catches
// the light (15% per point above +1, up to 60%).
func (t *tumblers) rollGlint(s *Session) {
	t.glint = -1
	mod := skillcheck.Modifier(s.Thieving)
	if mod < 2 || t.progress >= t.pins {
		return
	}
	if s.rng.Intn(100) < min(60, 15*(mod-1)) {
		t.glint = t.order[t.progress]
	}
}

func (t *tumblers) view(s *Session) map[string]any {
	v := map[string]any{
		"pins":      t.pins,
		"set":       append([]bool(nil), t.set...),
		"ruled_out": append([]bool(nil), t.ruledOut...),
	}
	if t.glint >= 0 {
		v["glint"] = t.glint
	}
	return v
}
