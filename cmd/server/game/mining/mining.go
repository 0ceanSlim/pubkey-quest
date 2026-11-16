// Package mining is the rock-face side of gathering (docs/draft/mining-system.md):
// mining.json's tiers, a seam's effective numbers, and which faces have been
// worked today. The API layer rolls the work and applies its costs.
package mining

import (
	"fmt"

	"pubkey-quest/types"
)

// Tier is a tier's defaults (false when mining.json has no such tier).
func Tier(cfg *types.MiningConfig, n int) (types.MiningTier, bool) {
	for _, t := range cfg.Tiers {
		if t.Tier == n {
			return t, true
		}
	}
	return types.MiningTier{}, false
}

// Effective is a seam with its tier's defaults filled in.
func Effective(cfg *types.MiningConfig, s types.Seam) types.Seam {
	t, _ := Tier(cfg, s.Tier)
	if s.DC <= 0 {
		s.DC = t.DC
	}
	if s.FacesPerDay <= 0 {
		s.FacesPerDay = t.FacesPerDay
	}
	return s
}

// QuickLoot is what a quick roll at the seam yields on a success: its authored
// table, or else two rolls over its ores by weight.
func QuickLoot(s types.Seam) *types.POILootTable {
	if s.QuickLoot != nil {
		return s.QuickLoot
	}
	t := &types.POILootTable{Rolls: 2}
	for _, o := range s.Ores {
		t.Tiers = append(t.Tiers, types.POILootTier{
			Name: o.Item, Weight: o.Weight,
			Entries: []types.POILootEntry{{Item: o.Item, Quantity: []any{float64(o.Quantity[0]), float64(o.Quantity[1])}}},
		})
	}
	return t
}

// Describe is the quick-work choice's label for a seam.
func Describe(s types.Seam, minutes int) string {
	return fmt.Sprintf("Quick work (Athletics DC %d, %d minutes)", s.DC, minutes)
}

// FacesLeftText is "3 faces left today".
func FacesLeftText(left int) string {
	if left == 1 {
		return "1 face left today"
	}
	return fmt.Sprintf("%d faces left today", left)
}

// ─── Faces worked today (kept with the save: types.Limits) ─────────────────

// Memory is the faces worked at each seam today.
type Memory map[string]seamDay

type seamDay = types.SeamDay

// Left is how many faces the seam has left today.
func (m Memory) Left(id string, day, perDay int) int {
	d := m[id]
	if d.Day != day {
		return perDay
	}
	return max(perDay-d.Worked, 0)
}

// Work uses up one of the seam's faces for today.
func (m Memory) Work(id string, day int) {
	d := m[id]
	if d.Day != day {
		d = seamDay{Day: day}
	}
	d.Worked++
	m[id] = d
}
