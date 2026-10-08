package character_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// A roster should read as several different people, not several rolls of the
// same dice — and, above all, slot 1 must keep generating exactly what it
// always did, or every character already in play changes underneath its player.

// testWeights mirrors the shape of game-data/systems/new-character/
// generation-weights.json closely enough to exercise the rules, including a
// deliberately small background pool (the case where exclusions can run out).
func testWeights() *types.WeightData {
	races := []string{"Human", "Elf", "Dwarf", "Halfling", "Orc", "Gnome", "Tiefling", "Dragonborn"}
	raceWeights := make([]int, len(races))
	for i := range raceWeights {
		raceWeights[i] = 10
	}

	classes := []string{"Fighter", "Wizard", "Rogue", "Cleric", "Bard", "Druid"}
	classByRace := map[string]map[string]int{}
	for _, r := range races {
		pool := map[string]int{}
		for _, c := range classes {
			pool[c] = 10
		}
		classByRace[r] = pool
	}

	// Two backgrounds per class: with three exclusions this pool must fall back
	// rather than fail.
	bgByClass := map[string]map[string]int{}
	for _, c := range classes {
		bgByClass[c] = map[string]int{c + "-Origin": 10, c + "-Exile": 10}
	}

	alignments := []string{"Lawful Good", "Neutral Good", "Chaotic Good", "Lawful Neutral", "True Neutral"}
	alignWeights := make([]int, len(alignments))
	for i := range alignWeights {
		alignWeights[i] = 10
	}

	return &types.WeightData{
		Races:                    races,
		RaceWeights:              raceWeights,
		ClassWeightsByRace:       classByRace,
		BackgroundWeightsByClass: bgByClass,
		Alignments:               alignments,
		AlignmentWeights:         alignWeights,
	}
}

const testKey = "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"

// The one that matters most: slot 1 is the old behaviour, untouched.
func TestSlotOneMatchesTheOriginalGeneration(t *testing.T) {
	w := testWeights()

	original := character.GenerateCharacter(testKey, w)
	slotOne := character.GenerateCharacterForSlot(testKey, w, 1, character.Exclusions{})

	if slotOne.Race != original.Race || slotOne.Class != original.Class ||
		slotOne.Background != original.Background || slotOne.Alignment != original.Alignment {
		t.Errorf("slot 1 drifted from the original generation:\n  original = %+v\n  slot 1   = %+v", original, slotOne)
	}
	for stat, want := range original.Stats {
		if slotOne.Stats[stat] != want {
			t.Errorf("slot 1 stat %s = %d, want %d", stat, slotOne.Stats[stat], want)
		}
	}
}

func TestRosterCharactersDiverge(t *testing.T) {
	roster := character.GenerateRoster(testKey, testWeights())

	if len(roster) != character.DerivedSlots {
		t.Fatalf("roster has %d entries, want %d", len(roster), character.DerivedSlots)
	}

	seenRace := map[string]int{}
	seenClass := map[string]int{}
	for i, c := range roster {
		if c.Race == "" || c.Class == "" || c.Background == "" {
			t.Errorf("slot %d is incomplete: %+v", i+1, c)
		}
		if prev, dup := seenRace[c.Race]; dup {
			t.Errorf("slot %d repeats the race of slot %d (%s)", i+1, prev, c.Race)
		}
		if prev, dup := seenClass[c.Class]; dup {
			t.Errorf("slot %d repeats the class of slot %d (%s)", i+1, prev, c.Class)
		}
		seenRace[c.Race] = i + 1
		seenClass[c.Class] = i + 1
	}
}

// Same npub, same roster, every time — the saves screen shows a locked slot's
// character long before it is unlocked, so the promise has to hold.
func TestRosterIsDeterministic(t *testing.T) {
	w := testWeights()
	first := character.GenerateRoster(testKey, w)
	second := character.GenerateRoster(testKey, w)

	for i := range first {
		if first[i].Race != second[i].Race || first[i].Class != second[i].Class ||
			first[i].Background != second[i].Background || first[i].Alignment != second[i].Alignment {
			t.Errorf("slot %d differs between runs:\n  %+v\n  %+v", i+1, first[i], second[i])
		}
	}
}

func TestDifferentPlayersGetDifferentRosters(t *testing.T) {
	w := testWeights()
	mine := character.GenerateRoster(testKey, w)
	theirs := character.GenerateRoster("32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245", w)

	identical := true
	for i := range mine {
		if mine[i].Race != theirs[i].Race || mine[i].Class != theirs[i].Class {
			identical = false
			break
		}
	}
	if identical {
		t.Error("two different npubs produced the same roster")
	}
}

// Backgrounds are scoped to a class and the smallest pools are tiny, so
// exclusions must degrade to a repeat rather than to an empty choice.
func TestExhaustedPoolFallsBackInsteadOfFailing(t *testing.T) {
	w := testWeights()

	// Exclude every background the chosen class has.
	c := character.GenerateCharacterForSlot(testKey, w, 1, character.Exclusions{})
	all := []string{}
	for bg := range w.BackgroundWeightsByClass[c.Class] {
		all = append(all, bg)
	}

	got := character.GenerateCharacterForSlot(testKey, w, 1, character.Exclusions{Backgrounds: all})
	if got.Background == "" {
		t.Error("an exhausted background pool produced no background; it should ignore exclusions instead")
	}
}

// Exclusions are respected when the pool has room.
func TestExclusionsAreHonoured(t *testing.T) {
	w := testWeights()
	first := character.GenerateCharacterForSlot(testKey, w, 1, character.Exclusions{})

	second := character.GenerateCharacterForSlot(testKey, w, 2, character.Exclusions{
		Races:   []string{first.Race},
		Classes: []string{first.Class},
	})

	if second.Race == first.Race {
		t.Errorf("slot 2 used the excluded race %s", first.Race)
	}
	if second.Class == first.Class {
		t.Errorf("slot 2 used the excluded class %s", first.Class)
	}
}
