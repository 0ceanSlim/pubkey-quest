package combat

import (
	"strings"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gametime"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// Light and dark in a fight. The clock is frozen during combat, so the save's
// time of day is the fight's.

// outdoors reports whether a fight is out in the world (an environment, or a
// POI out in one) rather than in a lit town.
func outdoors(cs *types.CombatSession) bool {
	if cs == nil || cs.EnvironmentID == "" {
		return false
	}
	biome, err := serverdb.GetEnvironmentType(cs.EnvironmentID)
	return err == nil && biome != ""
}

// inDarkness reports whether the player fights blind: night, outdoors, no
// darkvision, and no light source held in a hand.
func inDarkness(cs *types.CombatSession, save *types.SaveFile) bool {
	if save == nil || !types.IsNight(save.TimeOfDay) || !outdoors(cs) {
		return false
	}
	if gametime.HasDarkvision(save.Race) {
		return false
	}
	return !holdsLight(save)
}

// holdsLight reports whether a light source (torch, lantern, lamp…) is
// equipped in either hand.
func holdsLight(save *types.SaveFile) bool {
	for _, slot := range []string{"mainhand", "offhand"} {
		id := gaminventory.GetEquippedItemID(save.Inventory, slot)
		if id == "" {
			continue
		}
		if item, err := loadItemProps(serverdb.GetDB(), id); err == nil && hasTag(item["tags"], "light-source") {
			return true
		}
	}
	return false
}

// sunlightHampers reports whether a monster with Sunlight Sensitivity is out
// in daylight.
func sunlightHampers(cs *types.CombatSession, m *types.MonsterInstance, save *types.SaveFile) bool {
	if m == nil || save == nil || !hasSunlightSensitivity(m) {
		return false
	}
	return outdoors(cs) && gametime.IsSunlit(save.TimeOfDay)
}

func hasSunlightSensitivity(m *types.MonsterInstance) bool {
	for _, a := range m.Data.SpecialAbilities {
		if strings.EqualFold(a.Name, "Sunlight Sensitivity") {
			return true
		}
	}
	return false
}
