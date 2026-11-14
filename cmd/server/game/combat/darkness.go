package combat

import (
	"strings"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/effects"
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
// darkvision (by race, or an effect's "darkvision" stat: Owl's Eye), and no
// light source held in a hand.
func inDarkness(cs *types.CombatSession, save *types.SaveFile) bool {
	if save == nil || !types.IsNight(save.TimeOfDay) || !outdoors(cs) {
		return false
	}
	if gametime.HasDarkvision(save.Race) || effects.ModifierTotal(save, "darkvision") > 0 {
		return false
	}
	return !holdsLight(save)
}

// BlindAtNight reports whether the player is out in the dark: night, no
// darkvision and no light shining. Fishing reads the water by it.
func BlindAtNight(save *types.SaveFile) bool {
	if save == nil || !types.IsNight(save.TimeOfDay) {
		return false
	}
	if gametime.HasDarkvision(save.Race) || effects.ModifierTotal(save, "darkvision") > 0 {
		return false
	}
	return !holdsLight(save)
}

// InTheDark reports whether the player can't see to work: underground (always
// dark) or out at night, with no darkvision and no light shining. Mining reads
// the rock by it, and a deep mine's "light" requirement turns on it.
func InTheDark(save *types.SaveFile, underground bool) bool {
	if save == nil || (!underground && !types.IsNight(save.TimeOfDay)) {
		return false
	}
	if gametime.HasDarkvision(save.Race) || effects.ModifierTotal(save, "darkvision") > 0 {
		return false
	}
	return !holdsLight(save)
}

// holdsLight reports whether a light is shining: a lit torch, lamp or lantern
// in a hand, or a light worn at the neck (the Amulet of Light, always on). A
// light that burns (a "light" block) only counts while it's lit.
func holdsLight(save *types.SaveFile) bool {
	gear, _ := save.Inventory["gear_slots"].(map[string]interface{})
	for _, name := range []string{"mainhand", "offhand", "neck"} {
		id := gaminventory.GetEquippedItemID(save.Inventory, name)
		if id == "" {
			continue
		}
		item, err := loadItemProps(serverdb.GetDB(), id)
		if err != nil || !hasTag(item["tags"], "light-source") {
			continue
		}
		slot, _ := gear[name].(map[string]interface{})
		if condition.LightOf(id) == nil || condition.Shining(slot) {
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
