package mining

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand"
	"strings"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// ─── Veins: working a rock face as a minigame ────────────────────────────────
//
// A Session is one face, played move by move. The server lays out the hidden
// rock when the session starts, resolves every move against it, and only ever
// hands the client the public View: a cell shows what you've earned (a ring
// from a good sounding, what broke out of it), never what it holds. Each
// accepted move bumps Version; a move carrying an old version is refused, so a
// replayed or doubled request can't grant ore or cost time twice.
//
// The session decides what happened; the caller applies it (time, ore into the
// pack, cave-in damage, pick wear, exertion and XP when the face is done). Gem
// cells and prying wait for the gem generator (gem-system.md slice 1).

// Status is where a face stands.
type Status string

const (
	StatusActive    Status = "active"
	StatusExhausted Status = "exhausted" // out of swings
	StatusCollapsed Status = "collapsed" // the third crack
	StatusStopped   Status = "stopped"
)

// Cell kinds (hidden).
const (
	cellRock  = "rock"
	cellOre   = "ore"
	cellFault = "fault"
)

// MaxCracks is the crack that brings a face down.
const MaxCracks = 3

// Pick is the pick in hand: its tier, extra swings and how worn it is.
type Pick struct {
	Name   string
	Tier   int
	Swings int
	// Wear is its wear tier (degradation): 0 pristine, 1 worn (−1 swing),
	// 2 damaged (−2 swings, and hard rock takes two strikes whatever the
	// pick), 3 ruined (half the swings).
	Wear int
}

// Wear tiers a pick can be in.
const (
	WearDamaged = 2
	WearRuined  = 3
)

// Action is one move.
type Action struct {
	Kind string `json:"act"` // "sound" | "strike" | "stop"
	Cell int    `json:"cell"`
}

// Outcome is what a move did, for the caller to apply.
type Outcome struct {
	Minutes int
	Items   map[string]int // ore broken out, into the pack
	Damage  int            // a cave-in
	Wear    int            // wear points on the pick
	Ended   bool           // the face is done (status says how)
	Lines   []string
}

type cell struct {
	kind    string
	ore     string
	hard    bool
	cracked bool
	broken  bool
	read    bool // sounded successfully
	ring    int
	thud    bool // sounded and learned nothing
	hollow  bool
	tooHard bool
}

// Session is one face being worked.
type Session struct {
	ID     string
	SeamID string // the seam's id in the day's memory
	Node   string // the POI node the face is on
	Name   string
	Tier   types.MiningTier
	DC     int
	// Underground faces are always dark (see Dark).
	Underground bool

	Pick       Pick
	Athletics  int // effective skill values
	Perception int
	// Dark: no light to read the rock by, so there's no sounding. The caller
	// sets it before every move (a torch can burn out mid-face).
	Dark bool

	Version      int
	Status       Status
	Swings       int
	SwingsMax    int
	Soundings    int
	SoundingsMax int
	Cracks       int
	Worked       bool // a move has been made: the face counts as used
	Gained       int  // ore broken out so far (the caller adds it as it lands)
	W, H         int

	cells []cell
	cfg   *types.MiningConfig
	rng   *mrand.Rand
}

// ErrStale is returned for a move made against an old version of the face.
var ErrStale = errors.New("that move was already made")

// NewSession lays out a face of a seam (with its tier defaults filled in).
func NewSession(cfg *types.MiningConfig, seamID, node, name string, seam types.Seam, pick Pick, athletics, perception int, rng *mrand.Rand) (*Session, error) {
	tier, ok := Tier(cfg, seam.Tier)
	if !ok {
		return nil, fmt.Errorf("no mining tier %d", seam.Tier)
	}
	if tier.Grid[0] <= 0 || tier.Grid[1] <= 0 {
		return nil, fmt.Errorf("mining tier %d has no grid", seam.Tier)
	}
	if len(seam.Ores) == 0 {
		return nil, fmt.Errorf("this seam has no ore to find")
	}
	s := &Session{
		ID: newID(), SeamID: seamID, Node: node, Name: name, Tier: tier, DC: seam.DC, Underground: seam.Underground,
		Pick: pick, Athletics: athletics, Perception: perception, Status: StatusActive,
		W: tier.Grid[0], H: tier.Grid[1], cfg: cfg, rng: rng,
	}
	s.SwingsMax = swingsFor(athletics, pick)
	s.SoundingsMax = max(2, 3+skillcheck.Modifier(perception))
	s.Swings, s.Soundings = s.SwingsMax, s.SoundingsMax
	s.layout(seam)
	return s, nil
}

// swingsFor is how many swings you have at a face: 6 + Athletics + the pick's
// bonus (at least 3), less what the pick's wear costs.
func swingsFor(athletics int, pick Pick) int {
	n := max(3, 6+skillcheck.Modifier(athletics)+pick.Swings)
	switch pick.Wear {
	case 1:
		n--
	case WearDamaged:
		n -= 2
	case WearRuined:
		n /= 2
	}
	return max(n, 2)
}

// layout hides the ore veins, faults and hard rock.
func (s *Session) layout(seam types.Seam) {
	s.cells = make([]cell, s.W*s.H)
	for i := range s.cells {
		s.cells[i].kind = cellRock
	}
	for v := s.between(s.Tier.Veins); v > 0; v-- {
		ore := s.pickOre(seam.Ores)
		c := s.emptyCell()
		if c < 0 {
			break
		}
		for n := 2 + s.rng.Intn(3); n > 0 && c >= 0; n-- { // a run of 2–4 cells
			s.cells[c].kind, s.cells[c].ore = cellOre, ore
			c = s.emptyNeighbour(c)
		}
	}
	for f := s.between(s.Tier.Faults); f > 0; f-- {
		if c := s.emptyCell(); c >= 0 {
			s.cells[c].kind = cellFault
		}
	}
	for i := range s.cells {
		if s.cells[i].kind != cellFault && s.rng.Float64() < s.Tier.Hard {
			s.cells[i].hard = true
		}
	}
}

func (s *Session) between(r [2]int) int {
	if r[1] <= r[0] {
		return r[0]
	}
	return r[0] + s.rng.Intn(r[1]-r[0]+1)
}

func (s *Session) pickOre(ores []types.SeamOre) string {
	total := 0
	for _, o := range ores {
		total += max(o.Weight, 0)
	}
	if total <= 0 {
		return ores[0].Item
	}
	r := s.rng.Intn(total)
	for _, o := range ores {
		if r -= max(o.Weight, 0); r < 0 {
			return o.Item
		}
	}
	return ores[len(ores)-1].Item
}

// emptyCell is a random plain-rock cell (-1 when there's none).
func (s *Session) emptyCell() int {
	var free []int
	for i, c := range s.cells {
		if c.kind == cellRock {
			free = append(free, i)
		}
	}
	if len(free) == 0 {
		return -1
	}
	return free[s.rng.Intn(len(free))]
}

// emptyNeighbour is a random plain-rock cell beside c, up/down/left/right.
func (s *Session) emptyNeighbour(c int) int {
	x, y := c%s.W, c/s.W
	var free []int
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if nx >= 0 && ny >= 0 && nx < s.W && ny < s.H && s.cells[ny*s.W+nx].kind == cellRock {
			free = append(free, ny*s.W+nx)
		}
	}
	if len(free) == 0 {
		return -1
	}
	return free[s.rng.Intn(len(free))]
}

// around is the up-to-8 cells touching c.
func (s *Session) around(c int) []int {
	x, y := c%s.W, c/s.W
	var out []int
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if (dx != 0 || dy != 0) && nx >= 0 && ny >= 0 && nx < s.W && ny < s.H {
				out = append(out, ny*s.W+nx)
			}
		}
	}
	return out
}

// pickCuts reports whether the pick in hand can cut an ore.
func (s *Session) pickCuts(ore string) bool {
	need := 1
	if r, ok := s.cfg.Ores[ore]; ok && r.Pick > need {
		need = r.Pick
	}
	return s.Pick.Tier >= need
}

// Act resolves one move. version must be the session's current Version. Errors
// leave the face untouched; a nil error means the move happened.
func (s *Session) Act(a Action, version int, names func(string) string) (Outcome, error) {
	var out Outcome
	if s.Status != StatusActive {
		return out, fmt.Errorf("this face is done")
	}
	if version != s.Version {
		return out, ErrStale
	}
	if a.Kind != "stop" && (a.Cell < 0 || a.Cell >= len(s.cells)) {
		return out, fmt.Errorf("there's no rock there")
	}
	switch a.Kind {
	case "stop":
		s.Status = StatusStopped
		out.Lines = append(out.Lines, "You step back from the face with what you have.")
	case "sound":
		if err := s.sound(a.Cell, &out); err != nil {
			return out, err
		}
	case "strike":
		if err := s.strike(a.Cell, &out, names); err != nil {
			return out, err
		}
	default:
		return out, fmt.Errorf("unknown move %q", a.Kind)
	}
	if a.Kind != "stop" {
		s.Worked = true
	}
	if s.Status == StatusActive && s.Swings <= 0 {
		s.Status = StatusExhausted
		out.Lines = append(out.Lines, "Your arms are done. The face is finished for now.")
	}
	out.Ended = s.Status != StatusActive
	s.Version++
	return out, nil
}

func (s *Session) sound(i int, out *Outcome) error {
	c := &s.cells[i]
	switch {
	case s.Dark:
		return fmt.Errorf("it's too dark to read the rock: you need a light")
	case s.Soundings <= 0:
		return fmt.Errorf("you've no soundings left")
	case c.broken:
		return fmt.Errorf("that rock's already broken")
	case c.read:
		return fmt.Errorf("you already know how that one rings")
	}
	s.Soundings--
	out.Minutes += s.cfg.Rules.SoundMinutes
	r := skillcheck.Resolve(s.Perception, s.DC, s.rng)
	if !r.Success {
		c.thud = true
		out.Lines = append(out.Lines, fmt.Sprintf("Tap... a dull thud. (Perception %d vs %d)", r.Total, r.DC))
		return nil
	}
	c.read, c.thud = true, false
	for _, n := range s.around(i) {
		if s.cells[n].kind == cellOre {
			c.ring++
		}
	}
	line := fmt.Sprintf("Tap... it rings %d. (Perception %d vs %d)", c.ring, r.Total, r.DC)
	if c.kind == cellFault {
		c.hollow = true
		line += " It sounds hollow!"
	}
	if c.kind == cellOre && !s.pickCuts(c.ore) {
		c.tooHard = true
		line += " Too hard for your pick."
	}
	out.Lines = append(out.Lines, line)
	return nil
}

func (s *Session) strike(i int, out *Outcome, names func(string) string) error {
	c := &s.cells[i]
	if c.broken {
		return fmt.Errorf("that rock's already broken")
	}
	s.Swings--
	out.Minutes += s.cfg.Rules.SwingMinutes
	out.Wear += max(s.cfg.Rules.WearPerStrike, 1)
	// Hard rock takes two strikes with a tier-1 pick, or a damaged one.
	crackFirst := c.hard && !c.cracked && (s.Pick.Tier < 2 || s.Pick.Wear >= WearDamaged)
	switch {
	case c.kind == cellFault:
		c.broken = true
		out.Wear = max(s.cfg.Rules.WearPerFault, out.Wear)
		s.Cracks++
		if s.Cracks >= MaxCracks {
			s.caveIn(out)
			return nil
		}
		out.Lines = append(out.Lines, fmt.Sprintf("Your pick bites into a fault. The face cracks (%d of %d).", s.Cracks, MaxCracks))
		if s.Cracks == MaxCracks-1 {
			out.Lines = append(out.Lines, "The face groans. One more crack will bring it down.")
		}
	case c.kind == cellOre && !s.pickCuts(c.ore):
		c.tooHard = true
		out.Lines = append(out.Lines, fmt.Sprintf("Your pick skids off the %s. It's too hard for it.", strings.ToLower(names(c.ore))))
	case crackFirst:
		c.cracked = true
		out.Lines = append(out.Lines, "Hard rock. It cracks; one more strike.")
	case c.kind == cellOre:
		c.broken = true
		n := 1 + s.rng.Intn(2)
		if out.Items == nil {
			out.Items = map[string]int{}
		}
		out.Items[c.ore] += n
		out.Lines = append(out.Lines, fmt.Sprintf("The rock splits: %d × %s!", n, strings.ToLower(names(c.ore))))
	default:
		c.broken = true
		out.Lines = append(out.Lines, "Rock. Nothing in it.")
	}
	return nil
}

// caveIn brings the face down: the tier's d6s, halved by an Athletics check.
func (s *Session) caveIn(out *Outcome) {
	s.Status = StatusCollapsed
	dmg := 0
	for range max(s.Tier.CaveInDice, 1) {
		dmg += 1 + s.rng.Intn(6)
	}
	r := skillcheck.Resolve(s.Athletics, s.DC, s.rng)
	how := "full"
	if r.Success {
		dmg /= 2
		how = "halved"
	}
	out.Damage = dmg
	out.Lines = append(out.Lines, fmt.Sprintf("CAVE-IN! The face comes down around you. You take %d damage. (Athletics %d vs %d: %s)", dmg, r.Total, r.DC, how))
}

// View is the public state of the face: per cell, only what's been earned.
func (s *Session) View(names func(string) string) map[string]any {
	cells := make([]map[string]any, len(s.cells))
	for i, c := range s.cells {
		v := map[string]any{"state": "unknown"}
		switch {
		case c.broken:
			v["state"] = "broken"
			v["kind"] = c.kind
			if c.kind == cellOre {
				v["ore"], v["ore_name"] = c.ore, names(c.ore)
			}
		default:
			if c.read {
				v["state"], v["ring"] = "sounded", c.ring
			} else if c.thud {
				v["thud"] = true
			}
			if c.hollow {
				v["hollow"] = true
			}
			if c.tooHard {
				v["too_hard"] = true
			}
			if c.cracked {
				v["cracked"] = true
			}
			// Hard rock shows once you've read it or hit it.
			if c.hard && (c.read || c.cracked) {
				v["hard"] = true
			}
		}
		cells[i] = v
	}
	return map[string]any{
		"session_id": s.ID, "name": s.Name, "style": "veins", "version": s.Version,
		"status": s.Status, "tier": s.Tier.Tier, "dc": s.DC, "grid": []int{s.W, s.H},
		"swings": s.Swings, "swings_max": s.SwingsMax, "soundings": s.Soundings,
		"soundings_max": s.SoundingsMax, "cracks": s.Cracks, "cracks_max": MaxCracks,
		"pick_tier": s.Pick.Tier, "pick_name": s.Pick.Name, "pick_wear": s.Pick.Wear,
		"dark": s.Dark, "cells": cells,
	}
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
