package fishing

import (
	"math"
	"math/rand"
	"slices"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// ─── Bites: what's biting, how often, and how big ────────────────────────────
//
// A bite is rolled per wait step:
//
//	chance = base_bite × spot.bite_rate × appetite × depletion × (1 + 5% × Survival mod)
//
// where appetite is the spot's species weights averaged over how much each likes
// the bait and how hungry it is at this hour. The species that bites is picked
// by the same weights; its size by the bands' shares tilted by the bait.

// Conditions is the world around a cast: the hour, whether it's the day's
// sunlit window (time-config sunlight_hours), the angler and the bait.
type Conditions struct {
	TimeOfDay int
	Day       [2]int // sunlit window, minutes since midnight
	Survival  int    // skill score (not modifier)
	Athletics int
	Bait      string // bait kind ("" = bare hook)
	Barehand  bool
	Dark      bool // night with no light and no darkvision: tells are halved
	Party     bool // no monster bites in a party
	Depletion float64
}

func (c Conditions) day() bool  { return types.InWindow(c.TimeOfDay, c.Day[0], c.Day[1]) }
func (c Conditions) dawn() bool { return near(c.TimeOfDay, c.Day[0]) }
func (c Conditions) dusk() bool { return near(c.TimeOfDay, c.Day[1]) }

// golden is the hour either side of sunrise and sunset.
func (c Conditions) golden() bool { return c.dawn() || c.dusk() }

func near(t, edge int) bool {
	d := t - edge
	if d < 0 {
		d = -d
	}
	return d <= 60 || 1440-d <= 60
}

// hunger is how readily a species bites at this hour.
func hunger(sp types.FishSpecies, c Conditions) float64 {
	m := 1.0
	switch sp.Bites {
	case "day":
		if !c.day() {
			return 0
		}
	case "night":
		if c.day() && !c.golden() {
			return 0
		}
	case "dawn_dusk":
		if !c.golden() {
			m = 0.3
		}
	case "dawn":
		if !c.dawn() {
			m = 0.3
		}
	}
	if c.golden() {
		m *= 1.5
	}
	return m
}

// liking is how much a species wants the bait: ×2.5 for one it likes, ×0.5
// otherwise; a bare hook (or bare hands) is the same to everything.
func liking(sp types.FishSpecies, bait string) float64 {
	if bait == "" {
		return 1
	}
	if slices.Contains(sp.Likes, bait) || slices.Contains(sp.Likes, "any") {
		return 2.5
	}
	return 0.5
}

// pool is the species that can bite at a spot right now, with their weights.
func pool(cfg *types.FishingConfig, spot types.FishingSpot, c Conditions) map[string]float64 {
	out := map[string]float64{}
	for sid, w := range spot.Species {
		sp, ok := cfg.Species[sid]
		if !ok || sp.Barehand || (c.Barehand && sp.Shy) {
			continue // shy fish are too quick for hands
		}
		if v := w * liking(sp, c.Bait) * hunger(sp, c); v > 0 {
			out[sid] = v
		}
	}
	if c.Barehand {
		for sid, w := range spot.Barehand {
			if sp, ok := cfg.Species[sid]; ok {
				if v := w * hunger(sp, c); v > 0 {
					out[sid] = v
				}
			}
		}
	}
	return out
}

// BiteChance is the chance of a bite on one wait step.
func BiteChance(cfg *types.FishingConfig, spot types.FishingSpot, c Conditions) float64 {
	weights := pool(cfg, spot, c)
	total, base := 0.0, 0.0
	for sid, w := range spot.Species {
		if sp, ok := cfg.Species[sid]; ok && !sp.Barehand {
			base += w
		}
	}
	for _, w := range weights {
		total += w
	}
	if base <= 0 {
		return 0
	}
	appetite := total / base
	if c.Bait == "" && !c.Barehand {
		appetite *= 0.4 // a bare hook: any bait beats it
	}
	if b, ok := cfg.Baits[c.Bait]; ok && b.Night > 0 && !c.day() {
		appetite *= b.Night
	}
	if c.Barehand {
		appetite *= 0.4
	}
	dep := c.Depletion
	if dep <= 0 {
		dep = 1
	}
	chance := cfg.Rules.BaseBite * spot.BiteRate * appetite * dep * (1 + 0.05*float64(skillcheck.Modifier(c.Survival)))
	return min(0.9, max(0, chance))
}

// pickWeighted draws a key by weight (keys sorted first, so a seeded rng is
// reproducible).
func pickWeighted(weights map[string]float64, rng *rand.Rand) string {
	keys := make([]string, 0, len(weights))
	total := 0.0
	for k, w := range weights {
		if w > 0 {
			keys = append(keys, k)
			total += w
		}
	}
	if total <= 0 {
		return ""
	}
	slices.Sort(keys)
	r := rng.Float64() * total
	for _, k := range keys {
		r -= weights[k]
		if r < 0 {
			return k
		}
	}
	return keys[len(keys)-1]
}

// Fish is a fish on the line, rolled at the bite and kept from the client until
// it's landed.
type Fish struct {
	Species string
	Size    string
	Lb      float64
}

// ItemID is the fish's item.
func (f Fish) ItemID() string { return types.FishItemID(f.Species, f.Size) }

// rollSize picks a size band: shares × tilt^step, up to maxSize (an index;
// -1 = no cap).
func rollSize(cfg *types.FishingConfig, tilt float64, maxSize int, rng *rand.Rand) int {
	mid := mediumIndex(cfg)
	weights := map[string]float64{}
	for i, s := range cfg.Sizes {
		if maxSize >= 0 && i > maxSize {
			break
		}
		weights[s.ID] = s.Share * math.Pow(tilt, float64(i-mid))
	}
	return SizeIndex(cfg, pickWeighted(weights, rng))
}

// rollFish rolls a whole fish of a species: its size band, then an exact weight
// inside the band.
func rollFish(cfg *types.FishingConfig, sid string, tilt float64, maxSize int, rng *rand.Rand) Fish {
	i := rollSize(cfg, tilt, maxSize, rng)
	lo, hi := band(cfg, i)
	lb := cfg.Species[sid].MaxLb * (lo + rng.Float64()*(hi-lo))
	return Fish{Species: sid, Size: cfg.Sizes[i].ID, Lb: math.Max(0.01, math.Round(lb*100)/100)}
}

// tilt is the bait's size tilt (0.6 on a bare hook).
func tilt(cfg *types.FishingConfig, bait string) float64 {
	if b, ok := cfg.Baits[bait]; ok && b.Tilt > 0 {
		return b.Tilt
	}
	return 0.6
}

// RollBite rolls what took the bait: a fish of the spot's pool, or nil when
// nothing in the water is biting right now.
func RollBite(cfg *types.FishingConfig, spot types.FishingSpot, c Conditions, rng *rand.Rand) *Fish {
	sid := pickWeighted(pool(cfg, spot, c), rng)
	if sid == "" {
		return nil
	}
	maxSize := -1
	if c.Barehand && !cfg.Species[sid].Barehand {
		maxSize = SizeIndex(cfg, "small") // hands only hold the small ones
	}
	f := rollFish(cfg, sid, tilt(cfg, c.Bait), maxSize, rng)
	return &f
}

// power is how hard a fish pulls: its species' power plus its size's.
func power(cfg *types.FishingConfig, f Fish) int {
	p := cfg.Species[f.Species].Power
	if i := SizeIndex(cfg, f.Size); i >= 0 {
		p += cfg.Sizes[i].Power
	}
	return max(0, p)
}

// PullFeel is the always-shown hint at a hooked fish's strength.
func PullFeel(p int) string {
	switch {
	case p <= 1:
		return "light"
	case p <= 3:
		return "firm"
	case p <= 6:
		return "heavy"
	}
	return "monstrous"
}
