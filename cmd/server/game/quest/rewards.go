package quest

import (
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// GrantReward applies a quest stage reward to the save: XP, gold, items, and an
// optional effect, each through the existing system that owns it.
//
// Quest points are deliberately NOT granted here — they derive from the
// completed-quest list (the §4 hydration rule, see QuestPoints), so a reward's
// quest_points field is informational only and never stored.
func GrantReward(save *types.SaveFile, reward *types.POIReward, advancement []types.AdvancementEntry) {
	if reward == nil {
		return
	}
	if reward.XP > 0 {
		character.GrantXP(save, reward.XP, advancement)
	}
	if reward.Gold > 0 {
		_ = gameutil.AddGoldToInventory(save.Inventory, reward.Gold)
	}
	for _, item := range reward.Items {
		qty := item.Quantity
		if qty <= 0 {
			qty = 1
		}
		_, _ = inventory.AddItemToInventory(save, item.ID, qty)
	}
	if reward.Effect != nil && reward.Effect.ID != "" {
		_ = effects.ApplyEffect(save, reward.Effect.ID)
	}
}
