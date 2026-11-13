package fishing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"strings"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// ─── Cast & Reel: the fishing session ────────────────────────────────────────
//
// Like a lock session (game/locks), a Session is one sitting at the water,
// played turn by turn. The server rolls everything and hands the client only
// the public View: a fish's species and size stay hidden until it's landed.
// Each accepted action bumps Version; an action carrying an old version is
// refused, so a retried request can't count twice.
//
// Phases: watch the float (wait / strike), fight what's hooked (reel / hold /
// give line), and between catches (recast). Quit any time. Fishing barehanded
// watches a shadow instead and the strike is a grab: no fight.
//
// The session decides what happened; the caller applies it to the game (time,
// bait used up, the catch into the pack, a monster fight, the spot's daily
// count).

// Phase is where a session stands.
type Phase string

const (
	PhaseWatch Phase = "watch"
	PhaseFight Phase = "fight"
	PhaseDone  Phase = "done" // between catches: recast or quit
	PhaseOver  Phase = "over"
)

// Float is what the float (or, barehanded, the shadow) is doing.
type Float string

const (
	FloatStill  Float = "still"
	FloatTwitch Float = "twitch"
	FloatDive   Float = "dive"
)

// Fight moves.
const (
	MoveRun    = "run"
	MoveDive   = "dive"
	MoveThrash = "thrash"
	MoveRest   = "rest"
)

// Gear is the rod's fishing block (zero for bare hands).
type Gear struct {
	Line int `json:"line"`
	Cast int `json:"cast"`
	Tell int `json:"tell"` // percent added to the tell roll
}

// Outcome is what an action did, for the caller to apply.
type Outcome struct {
	Minutes  int
	Lines    []string
	Landed   *Fish  // a fish to add to the pack
	Find     string // an item fished up (a boot, a branch, a purse)
	FindQty  int
	Monster  string // something nasty is on the line: start this fight
	BaitUsed bool   // the bait is gone: eaten, stolen, or off the hook
	LureLost bool   // the line snapped and took the lure
	Escaped  bool   // a hooked fish got away
}

// Session is one sitting at the water.
type Session struct {
	ID     string
	SpotID string
	Spot   types.FishingSpot
	Gear   Gear
	Cond   Conditions
	// Where the caller is fishing from, and the bait item on the hook: the
	// engine doesn't read these.
	Kind     string // "town" | "poi"
	Node     string // the POI fishing node
	BaitItem string

	Version int
	Phase   Phase
	Float   Float
	Hint    string

	// Fight state (public).
	Tension  int
	Distance int
	Spent    bool

	// Hidden.
	bite    *Fish
	monster string
	find    string
	nibbles int
	held    bool
	stamina int
	slack   int
	move    string
	streak  int
	tell    bool
	last    *Fish // the catch on the card (Phase done)

	cfg *types.FishingConfig
	rng *mrand.Rand
}

// ErrStale is returned for an action made against an old version of the session.
var ErrStale = fmt.Errorf("that move was already made")

// NewSession starts fishing at a spot. A barehand session needs no gear.
func NewSession(cfg *types.FishingConfig, spotID string, gear Gear, c Conditions, rng *mrand.Rand) (*Session, error) {
	spot, ok := cfg.Spots[spotID]
	if !ok {
		return nil, fmt.Errorf("there's no fishing here")
	}
	if !c.Barehand && gear.Line <= 0 {
		return nil, fmt.Errorf("you need a rod to fish here")
	}
	return &Session{
		ID: newID(), SpotID: spotID, Spot: spot, Gear: gear, Cond: c,
		Phase: PhaseWatch, Float: FloatStill, cfg: cfg, rng: rng,
	}, nil
}

// SetBait changes the bait for the next cast (run out, or swapped between catches).
func (s *Session) SetBait(kind string) { s.Cond.Bait = kind }

// Act resolves one action. version must be the session's current Version.
// Errors leave the session untouched; a nil error means the action happened.
func (s *Session) Act(act string, version int) (Outcome, error) {
	var out Outcome
	if s.Phase == PhaseOver {
		return out, fmt.Errorf("you've stopped fishing")
	}
	if version != s.Version {
		return out, ErrStale
	}
	switch {
	case act == "quit":
		s.quit(&out)
	case s.Phase == PhaseWatch && act == "wait":
		s.wait(&out)
	case s.Phase == PhaseWatch && act == "strike":
		s.strike(&out)
	case s.Phase == PhaseFight && (act == "reel" || act == "hold" || act == "give"):
		s.fight(act, &out)
	case s.Phase == PhaseDone && act == "recast":
		s.Phase, s.Float, s.last = PhaseWatch, FloatStill, nil
		out.Minutes = s.cfg.Rules.RecastMinute
		out.Lines = append(out.Lines, s.say("You cast out again.", "You wade back in and watch the water."))
	default:
		return out, fmt.Errorf("you can't %s now", act)
	}
	s.Version++
	return out, nil
}

func (s *Session) say(rod, hands string) string {
	if s.Cond.Barehand {
		return hands
	}
	return rod
}

func (s *Session) quit(out *Outcome) {
	if s.Phase == PhaseFight {
		out.BaitUsed, out.Escaped = s.Cond.Bait != "", true
		out.Lines = append(out.Lines, "You let the line go slack. The fish throws the hook and is gone.")
	} else {
		out.Lines = append(out.Lines, s.say("You reel in and pack up the rod.", "You wade out of the water."))
	}
	s.Phase = PhaseOver
}

// clearBite forgets whatever was at the bait.
func (s *Session) clearBite() {
	s.bite, s.monster, s.find, s.nibbles, s.held, s.Hint = nil, "", "", 0, false, ""
	s.Float = FloatStill
}

// wait watches the float for one step.
func (s *Session) wait(out *Outcome) {
	out.Minutes = s.cfg.Rules.WaitMinutes
	switch s.Float {
	case FloatStill:
		s.rollBite(out)
	case FloatTwitch:
		shy := s.bite != nil && s.cfg.Species[s.bite.Species].Shy
		switch {
		case shy && s.nibbles < 2:
			s.nibbles++
			out.Lines = append(out.Lines, s.say("The float twitches again, barely.", "The shadow circles back, closer."))
		case shy:
			s.clearBite()
			out.Lines = append(out.Lines, s.say("The nibbling stops. Whatever it was has gone.", "The shadow slips away."))
		case s.rng.Float64() < 0.6:
			s.Float = FloatDive
			out.Lines = append(out.Lines, s.say("The float dives under!", "The shadow comes in close, right by your legs."))
		default:
			s.clearBite()
			out.Lines = append(out.Lines, s.say("The float settles. Whatever it was has moved on.", "The shadow drifts off."))
		}
	case FloatDive:
		if s.find != "" {
			out.Lines = append(out.Lines, "The float stays down, heavy and still: a snag?")
			return
		}
		if !s.held && s.rng.Float64() < 0.5 {
			s.held = true
			out.Lines = append(out.Lines, s.say("The float stays under.", "It's still there, close."))
			return
		}
		baited := s.Cond.Bait != "" && !s.Cond.Barehand
		s.clearBite()
		out.BaitUsed = baited
		out.Lines = append(out.Lines, s.say("The float pops back up. It took the bait and left.", "The shadow darts away."))
	}
}

// rollBite rolls for a bite on a still float.
func (s *Session) rollBite(out *Outcome) {
	if s.rng.Float64() >= BiteChance(s.cfg, s.Spot, s.Cond) {
		out.Lines = append(out.Lines, s.say("The float sits still on the water.", "You watch the water. Nothing moves."))
		return
	}
	r := s.rng.Float64()
	if m := s.monsterHere(); m != "" && r < s.cfg.Rules.MonsterBite {
		s.monster, s.Float = m, FloatDive
		out.Lines = append(out.Lines, s.say("The float is yanked under, hard. Something big has it.", "A big shape surges toward you through the water."))
		return
	}
	if len(s.Spot.Finds) > 0 && !s.Cond.Barehand && r < s.cfg.Rules.MonsterBite+s.cfg.Rules.FindBite {
		s.find, s.Float = pickWeighted(s.Spot.Finds, s.rng), FloatDive
		out.Lines = append(out.Lines, "The float dips heavily and stays down.")
		return
	}
	f := RollBite(s.cfg, s.Spot, s.Cond, s.rng)
	if f == nil {
		out.Lines = append(out.Lines, s.say("The float sits still on the water.", "You watch the water. Nothing moves."))
		return
	}
	s.bite, s.nibbles, s.Float = f, 1, FloatTwitch
	out.Lines = append(out.Lines, s.say("The float twitches. Something's nibbling.", "A shadow circles in the water."))
	// A sharp angler sometimes reads a shy biter from the first touch.
	if mod := skillcheck.Modifier(s.Cond.Survival); s.cfg.Species[f.Species].Shy && mod >= 3 && s.rng.Float64() < 0.15*float64(mod-2) {
		s.Hint = "A delicate, testing touch: a shy biter. Strike on the twitch."
	}
}

// monsterHere picks a monster that could take the bait now ("" = none).
func (s *Session) monsterHere() string {
	if s.Cond.Party {
		return ""
	}
	w := map[string]float64{}
	for _, m := range s.Spot.Monsters {
		day := s.Cond.day()
		if (m.When == "night" && day) || (m.When == "day" && !day) {
			continue
		}
		w[m.Monster] += m.Weight
	}
	return pickWeighted(w, s.rng)
}

// strike sets the hook (or, barehanded, grabs).
func (s *Session) strike(out *Outcome) {
	switch s.Float {
	case FloatStill:
		out.Minutes = s.cfg.Rules.RecastMinute
		out.Lines = append(out.Lines, s.say("You strike at nothing and pull the bait out. You cast again.", "You grab at the empty water."))
	case FloatTwitch:
		chance := 0.25
		if s.bite != nil && s.cfg.Species[s.bite.Species].Shy && s.nibbles >= 2 {
			chance = 0.5 // a shy fish's second touch is its bite
		}
		if !s.Cond.Barehand && s.rng.Float64() < chance {
			s.hook(out)
			return
		}
		lost := !s.Cond.Barehand && s.Cond.Bait != "" && s.rng.Float64() < 0.3
		s.clearBite()
		out.BaitUsed = lost
		if lost {
			out.Lines = append(out.Lines, "Too soon. It spooks, and takes the bait with it.")
		} else {
			out.Lines = append(out.Lines, s.say("Too soon. It spooks.", "You lunge too early. It darts away."))
		}
	case FloatDive:
		switch {
		case s.monster != "":
			out.Monster = s.monster
			out.Lines = append(out.Lines, "You strike, and the water erupts!")
			s.clearBite()
			s.Phase = PhaseOver
		case s.find != "":
			out.Find, out.FindQty = s.find, 1
			if s.find == "gold-piece" {
				out.FindQty = 5 + s.rng.Intn(46)
			}
			s.clearBite()
			s.Phase = PhaseDone
		case s.Cond.Barehand:
			s.grab(out)
		default:
			s.hook(out)
		}
	}
}

// grab is the barehand catch: an Athletics check against the spot's DC + 4,
// +2 for each size above small. No fight: your hands are the line.
func (s *Session) grab(out *Outcome) {
	f := *s.bite
	dc := s.Spot.DC + 4 + 2*max(0, SizeIndex(s.cfg, f.Size)-SizeIndex(s.cfg, "small"))
	r := skillcheck.Resolve(s.Cond.Athletics, dc, s.rng)
	s.clearBite()
	if !r.Success {
		out.Escaped = true
		out.Lines = append(out.Lines, fmt.Sprintf("You plunge your hands in and close on it, but it twists free. (Athletics %d vs %d)", r.Total, dc))
		return
	}
	s.land(f, out)
	out.Lines[len(out.Lines)-1] = fmt.Sprintf("You plunge your hands in and haul it out, thrashing! (Athletics %d vs %d) ", r.Total, dc) + out.Lines[len(out.Lines)-1]
}

// hook sets the hook: the fight starts.
func (s *Session) hook(out *Outcome) {
	s.Phase = PhaseFight
	s.Tension, s.Distance, s.slack, s.streak, s.Spent = 2, max(1, s.Gear.Cast), 0, 0, false
	s.stamina = s.cfg.Sizes[SizeIndex(s.cfg, s.bite.Size)].Stamina
	s.Float, s.Hint = FloatDive, ""
	s.nextMove()
	out.Lines = append(out.Lines, fmt.Sprintf("Hooked! The rod bends. It feels %s.", PullFeel(power(s.cfg, *s.bite))))
}

// fight resolves one turn of the fight.
func (s *Session) fight(act string, out *Outcome) {
	out.Minutes = s.cfg.Rules.FightMinutes
	f := *s.bite
	p := power(s.cfg, f)
	if s.Spent {
		p /= 2
	}
	pull, run := 0, 0
	switch s.move {
	case MoveRun:
		pull, run = p+2, 1
	case MoveDive:
		pull = p
	case MoveThrash:
		pull = p + 1
	}
	strain, reel, tire := 0, 0, 0
	switch act {
	case "reel":
		strain, tire = 2, 1
		switch s.move {
		case MoveRun:
			reel = 0
		case MoveDive:
			reel = 1
		default:
			reel = 2
		}
	case "hold":
		strain, tire = 1, 2
	case "give":
		strain, reel = -5, -1
	}
	s.Tension = max(0, s.Tension-3+pull+strain)
	if act != "give" {
		s.Tension = max(1, s.Tension) // reeling or holding keeps the line taut
	}
	s.Distance += run - reel
	s.stamina -= tire
	switch s.move {
	case MoveRun, MoveThrash:
		s.stamina -= 2 // fighting the line tires it too
	case MoveDive:
		s.stamina--
	case MoveRest:
		if act == "give" {
			s.stamina++
		}
	}
	out.Lines = append(out.Lines, fightLine(s.move, act))

	lost := func(line string, snap bool) {
		out.Escaped, out.Lines = true, append(out.Lines, line)
		out.BaitUsed = s.Cond.Bait != ""
		out.LureLost = snap
		s.clearBite()
		s.Phase = PhaseDone
	}
	switch {
	case s.Tension >= s.Gear.Line:
		lost("SNAP! The line parts and the fish is gone.", true)
		return
	case s.Distance >= s.cfg.Rules.MaxDistance:
		lost("The line runs out to the knot and snaps. It's gone.", true)
		return
	case s.move == MoveThrash && s.Tension == 0:
		lost("It thrashes on the slack line and throws the hook.", false)
		return
	}
	if s.Tension == 0 {
		s.slack++
		if s.slack >= 2 {
			lost("The line's been slack too long. It shakes the hook loose.", false)
			return
		}
	} else {
		s.slack = 0
	}
	if s.Distance <= 0 {
		s.land(f, out)
		return
	}
	if s.stamina <= 0 && !s.Spent {
		s.Spent = true
		out.Lines = append(out.Lines, "It's tiring. The fight is going out of it.")
	}
	s.nextMove()
}

// land puts a fish on the bank.
func (s *Session) land(f Fish, out *Outcome) {
	sp, size := s.cfg.Species[f.Species], s.cfg.Sizes[SizeIndex(s.cfg, f.Size)]
	out.Landed = &f
	out.Lines = append(out.Lines, fmt.Sprintf("Landed: a %s %s, %s.", strings.ToLower(size.Name), strings.ToLower(sp.Name), WeighOut(f.Lb)))
	s.last = &f
	s.clearBite()
	s.Phase = PhaseDone
}

// nextMove picks the fish's next move: its profile's weights (a spent fish
// rests 70% of the time), never the same move three times running. Then the
// tell roll: does the angler read it coming?
func (s *Session) nextMove() {
	prof := s.cfg.Profiles[s.cfg.Species[s.bite.Species].Profile]
	w := map[string]float64{MoveRun: float64(prof.Run), MoveDive: float64(prof.Dive), MoveThrash: float64(prof.Thrash), MoveRest: float64(prof.Rest)}
	if s.streak >= 2 {
		delete(w, s.move)
	}
	next := pickWeighted(w, s.rng)
	if s.Spent && s.rng.Float64() < 0.7 {
		next = MoveRest
	}
	if next == s.move {
		s.streak++
	} else {
		s.move, s.streak = next, 1
	}
	chance := 35 + 10*skillcheck.Modifier(s.Cond.Survival) + s.Gear.Tell
	chance = min(90, chance)
	if s.Cond.Dark {
		chance /= 2
	}
	s.tell = s.rng.Intn(100) < chance
}

// Tells are what the line says about the fish's next move.
var tells = map[string]string{
	MoveRun:    "The line hums and tightens.",
	MoveDive:   "The rod tip bows, steady.",
	MoveThrash: "The water boils.",
	MoveRest:   "The line goes heavy and still.",
}

func fightLine(move, act string) string {
	what := map[string]string{
		MoveRun: "It runs", MoveDive: "It dives", MoveThrash: "It thrashes", MoveRest: "It rests",
	}[move]
	did := map[string]string{"reel": "you reel", "hold": "you hold", "give": "you give line"}[act]
	return what + ", and " + did + "."
}

// WeighOut writes a weight as pounds and ounces ("19 lb 6 oz").
func WeighOut(lb float64) string {
	whole := int(lb)
	oz := int((lb - float64(whole)) * 16)
	switch {
	case whole == 0:
		return fmt.Sprintf("%d oz", max(1, oz))
	case oz == 0:
		return fmt.Sprintf("%d lb", whole)
	}
	return fmt.Sprintf("%d lb %d oz", whole, oz)
}

// View is the public state for the client: never what's on the line until
// it's landed.
func (s *Session) View() map[string]any {
	v := map[string]any{
		"session_id": s.ID,
		"version":    s.Version,
		"spot":       s.Spot.Name,
		"water":      s.Spot.Water,
		"barehand":   s.Cond.Barehand,
		"bait":       s.Cond.Bait,
		"phase":      s.Phase,
		"float":      s.Float,
	}
	if b, ok := s.cfg.Baits[s.Cond.Bait]; ok {
		v["bait_name"] = b.Name
	}
	if s.Hint != "" {
		v["hint"] = s.Hint
	}
	if s.Phase == PhaseFight {
		v["tension"], v["line"] = s.Tension, s.Gear.Line
		v["distance"], v["max_distance"] = s.Distance, s.cfg.Rules.MaxDistance
		v["spent"] = s.Spent
		v["feel"] = PullFeel(power(s.cfg, *s.bite))
		if s.tell {
			v["tell"] = tells[s.move]
		}
	}
	if s.last != nil {
		sp, size := s.cfg.Species[s.last.Species], s.cfg.Sizes[SizeIndex(s.cfg, s.last.Size)]
		v["catch"] = map[string]any{
			"item_id": s.last.ItemID(), "species": sp.Name, "size": size.Name,
			"lb": s.last.Lb, "weight": WeighOut(s.last.Lb), "value": FishValue(s.cfg, sp, size),
		}
	}
	return v
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
