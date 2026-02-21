package combat

import (
	"pubkey-quest/types"
)

// RollLoot executes all rolls on a monster's loot table and returns the resulting drops.
// Guaranteed items always appear. Tier rolls use weighted random selection.
func RollLoot(table types.LootTable) []types.LootDrop {
	var drops []types.LootDrop

	// Always add guaranteed drops
	for _, g := range table.Guaranteed {
		qty := RollRange(g.Quantity[0], g.Quantity[1])
		if qty > 0 {
			drops = append(drops, types.LootDrop{Item: g.Item, Quantity: qty})
		}
	}

	// Perform the configured number of tier rolls
	rolls := table.Rolls
	if rolls <= 0 {
		rolls = 1
	}

	for i := 0; i < rolls; i++ {
		drop := rollOneTier(table.Tiers)
		if drop != nil {
			drops = appendOrStack(drops, *drop)
		}
	}

	return drops
}

// rollOneTier picks a tier by weight, then picks an entry within that tier by weight.
// Returns nil if the result is "nothing".
func rollOneTier(tiers []types.LootTier) *types.LootDrop {
	if len(tiers) == 0 {
		return nil
	}

	tier := selectWeightedTier(tiers)
	if tier == nil {
		return nil
	}

	entry := selectWeightedEntry(tier.Entries)
	if entry == nil || entry.Item == "" || entry.Item == "nothing" {
		return nil
	}

	qty := 1
	if entry.Quantity != nil {
		qty = RollRange(entry.Quantity[0], entry.Quantity[1])
	}
	if qty <= 0 {
		return nil
	}

	return &types.LootDrop{Item: entry.Item, Quantity: qty}
}

// selectWeightedTier picks a LootTier using weighted random selection.
func selectWeightedTier(tiers []types.LootTier) *types.LootTier {
	total := 0
	for _, t := range tiers {
		total += t.Weight
	}
	if total == 0 {
		return nil
	}

	roll := RollRange(1, total)
	cumulative := 0
	for i := range tiers {
		cumulative += tiers[i].Weight
		if roll <= cumulative {
			return &tiers[i]
		}
	}
	return &tiers[len(tiers)-1]
}

// selectWeightedEntry picks a LootEntry using weighted random selection.
func selectWeightedEntry(entries []types.LootEntry) *types.LootEntry {
	total := 0
	for _, e := range entries {
		total += e.Weight
	}
	if total == 0 {
		return nil
	}

	roll := RollRange(1, total)
	cumulative := 0
	for i := range entries {
		cumulative += entries[i].Weight
		if roll <= cumulative {
			return &entries[i]
		}
	}
	return &entries[len(entries)-1]
}

// appendOrStack adds a drop to the list, stacking onto an existing entry of the same item.
func appendOrStack(drops []types.LootDrop, drop types.LootDrop) []types.LootDrop {
	for i := range drops {
		if drops[i].Item == drop.Item {
			drops[i].Quantity += drop.Quantity
			return drops
		}
	}
	return append(drops, drop)
}
