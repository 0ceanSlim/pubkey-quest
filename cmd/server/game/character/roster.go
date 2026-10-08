package character

import (
	"fmt"
	"slices"

	"pubkey-quest/types"
)

// A player's roster of characters.
//
// Characters are derived from the npub, so without further work every slot
// would hand back the same person. Instead each derived slot is generated from
// the pool with the previous slots' defining traits removed, so a roster is
// five different people rather than five rolls of the same dice:
//
//	slot 1  derived from the npub, as it always was
//	slot 2  derived, minus slot 1's race / class / background
//	slot 3  derived, minus slots 1–2's
//	slot 4  derived, minus slots 1–3's
//	slot 5  the player's own build — the reward for reaching level 20
//
// Alignment is deliberately not excluded: it is flavour, and sharing it does
// not make two characters feel alike. It still varies per slot, because the
// seed does.
//
// Every derived slot is a pure function of the npub and the slots before it, so
// the whole roster can be shown before any of it is unlocked — and none of it
// needs storing (see the hydration rule, roadmap §4).

const (
	// DerivedSlots is how many slots are generated. The slot after them is the
	// player-authored build.
	DerivedSlots = 4
	// CustomSlot is the player-authored slot.
	CustomSlot = 5
)

// Exclusions are the traits already spoken for by earlier slots.
type Exclusions struct {
	Races       []string
	Classes     []string
	Backgrounds []string
}

// slotKey returns the seed material for a slot. Slot 1 uses the player's key
// unchanged, so every character already in play keeps the identity it was born
// with; later slots derive from a slot-tagged key, which re-rolls every trait
// that isn't pinned by an exclusion — alignment and stats included, since they
// hang off the same key.
func slotKey(hexKey string, slot int) string {
	if slot <= 1 {
		return hexKey
	}
	return fmt.Sprintf("%s:slot%d", hexKey, slot)
}

// GenerateCharacterForSlot derives the character for one roster slot, avoiding
// the traits earlier slots already used.
func GenerateCharacterForSlot(hexKey string, weightData *types.WeightData, slot int, exclude Exclusions) types.Character {
	key := slotKey(hexKey, slot)

	race := chooseExcluding(weightData.Races, weightData.RaceWeights, exclude.Races, key, "race")
	classOptions, classWeights := weightedPairs(weightData.ClassWeightsByRace[race])
	class := chooseExcluding(classOptions, classWeights, exclude.Classes, key, "class_"+race)
	bgOptions, bgWeights := weightedPairs(weightData.BackgroundWeightsByClass[class])
	background := chooseExcluding(bgOptions, bgWeights, exclude.Backgrounds, key, "background_"+class)

	return types.Character{
		Race:       race,
		Class:      class,
		Background: background,
		Alignment:  GenerateAlignment(key, weightData),
		Stats:      GenerateStats(key, class),
	}
}

// GenerateRoster derives every slot a player can be handed, in order, each one
// avoiding the traits of the ones before it. The custom slot is not included —
// there is nothing to derive for it.
func GenerateRoster(hexKey string, weightData *types.WeightData) []types.Character {
	roster := make([]types.Character, 0, DerivedSlots)
	var exclude Exclusions

	for slot := 1; slot <= DerivedSlots; slot++ {
		c := GenerateCharacterForSlot(hexKey, weightData, slot, exclude)
		roster = append(roster, c)
		exclude.Races = append(exclude.Races, c.Race)
		exclude.Classes = append(exclude.Classes, c.Class)
		exclude.Backgrounds = append(exclude.Backgrounds, c.Background)
	}
	return roster
}

// chooseExcluding picks from the pool with the excluded options removed.
//
// If removing them would leave nothing to choose from, the exclusions are
// ignored for that trait: a repeated background beats a character that cannot
// be generated. Backgrounds are the pool where this can bite — they are scoped
// to a class, and the smallest has five.
func chooseExcluding(options []string, weights []int, exclude []string, key, context string) string {
	if len(options) == 0 {
		return ""
	}

	filteredOptions := make([]string, 0, len(options))
	filteredWeights := make([]int, 0, len(weights))
	for i, opt := range options {
		if slices.Contains(exclude, opt) {
			continue
		}
		filteredOptions = append(filteredOptions, opt)
		if i < len(weights) {
			filteredWeights = append(filteredWeights, weights[i])
		}
	}

	if len(filteredOptions) == 0 {
		filteredOptions, filteredWeights = options, weights
	}
	return DeterministicWeightedChoice(filteredOptions, filteredWeights, CreateDeterministicSeed(key, context))
}

// weightedPairs flattens a name→weight map into the parallel slices the chooser
// takes. Order does not matter: DeterministicWeightedChoice sorts internally,
// which is what keeps map iteration from making generation non-deterministic.
func weightedPairs(byName map[string]int) ([]string, []int) {
	options := make([]string, 0, len(byName))
	weights := make([]int, 0, len(byName))
	for name, weight := range byName {
		options = append(options, name)
		weights = append(weights, weight)
	}
	return options, weights
}
