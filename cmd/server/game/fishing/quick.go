package fishing

import (
	"math/rand"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// ─── The other ways: quick fish, the net, digging worms ──────────────────────

// QuickResult is a one-roll outcome.
type QuickResult struct {
	Fish     *Fish
	Items    map[string]int // net hauls and dug worms: item id → count
	BaitUsed bool
	Check    skillcheck.Result
	Minutes  int
}

// Quick is the quick roll, like a lock's quick attempt: one Survival check
// against the spot's DC. Success lands a fish of the usual species, at most
// medium (tilt 0.5); failure lands nothing and loses the bait half the time.
// Trophies need the fight.
func Quick(cfg *types.FishingConfig, spot types.FishingSpot, c Conditions, rng *rand.Rand) QuickResult {
	res := QuickResult{Minutes: cfg.Rules.QuickMinutes, Check: skillcheck.Resolve(c.Survival, spot.DC, rng)}
	sid := pickWeighted(pool(cfg, spot, c), rng)
	if !res.Check.Success || sid == "" {
		res.BaitUsed = c.Bait != "" && rng.Intn(2) == 0
		return res
	}
	f := rollFish(cfg, sid, 0.5, SizeIndex(cfg, "medium"), rng)
	res.Fish = &f
	res.BaitUsed = c.Bait != "" && !cfg.Baits[c.Bait].Lure
	return res
}

// Haul drags the net through a spot's shallows: one Survival check against the
// spot's DC. Success brings up 1–4 tiny or small fish of the shallows' species
// and maybe a shellfish; failure brings up weed. The net isn't used up.
func Haul(cfg *types.FishingConfig, spot types.FishingSpot, c Conditions, rng *rand.Rand) QuickResult {
	res := QuickResult{Minutes: cfg.Rules.NetMinutes, Items: map[string]int{}, Check: skillcheck.Resolve(c.Survival, spot.DC, rng)}
	if !res.Check.Success {
		return res
	}
	c.Bait = ""
	weights := pool(cfg, spot, c)
	for i, n := 0, 1+rng.Intn(4); i < n; i++ {
		if sid := pickWeighted(weights, rng); sid != "" {
			f := rollFish(cfg, sid, 1, SizeIndex(cfg, "small"), rng)
			res.Items[f.ItemID()]++
		}
	}
	if len(spot.Shellfish) > 0 && rng.Float64() < 0.6 {
		res.Items[pickWeighted(spot.Shellfish, rng)]++
	}
	return res
}

// DigWorms turns the bank over for worms: a Survival check against worm_dc
// finds 2–5.
func DigWorms(cfg *types.FishingConfig, c Conditions, rng *rand.Rand) QuickResult {
	res := QuickResult{Minutes: cfg.Rules.WormMinutes, Items: map[string]int{}, Check: skillcheck.Resolve(c.Survival, cfg.Rules.WormDC, rng)}
	if res.Check.Success {
		res.Items["worms"] = 2 + rng.Intn(4)
	}
	return res
}

// ─── Depletion: a spot fished hard bites less ────────────────────────────────

// Memory is the fish landed per spot today (session memory, like picked locks).
type Memory map[string]spotDay

type spotDay struct {
	Day    int
	Landed int
}

// Landed counts a fish landed at a spot today.
func (m Memory) Landed(spot string, day int) {
	d := m[spot]
	if d.Day != day {
		d = spotDay{Day: day}
	}
	d.Landed++
	m[spot] = d
}

// Factor is the spot's bite multiplier today: 1, halved past daily_catch, and
// 0 (fished out) past twice that.
func (m Memory) Factor(spot string, day, daily int) float64 {
	d := m[spot]
	if d.Day != day || daily <= 0 {
		return 1
	}
	switch {
	case d.Landed >= 2*daily:
		return 0
	case d.Landed >= daily:
		return 0.5
	}
	return 1
}
