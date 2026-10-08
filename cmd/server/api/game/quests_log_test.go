package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// The journal's quest log against the migrated quest drafts: every quest a
// character could see lands in exactly one status, and locked quests say why.

func TestQuestLogStatusesAndReasons(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}

	save := &types.SaveFile{
		Race: "Human", Class: "Fighter", Alignment: "neutral",
		HP: 20, MaxHP: 20,
		Stats: map[string]interface{}{
			"strength": 14, "dexterity": 12, "constitution": 14,
			"intelligence": 10, "wisdom": 10, "charisma": 10,
		},
		Inventory: map[string]interface{}{},
	}
	log := buildQuestLog(save, buildQuestContext(save))

	seen := map[string]string{}
	check := func(list []questView, want string) {
		for _, q := range list {
			if q.Status != want {
				t.Errorf("%s listed under %s but status %q", q.ID, want, q.Status)
			}
			if prev, dup := seen[q.ID]; dup {
				t.Errorf("%s appears as both %s and %s", q.ID, prev, want)
			}
			seen[q.ID] = want
		}
	}
	check(log.Active, "active")
	check(log.Available, "available")
	check(log.Locked, "locked")
	check(log.Completed, "completed")

	// A human fighter can't take the paladin's or the dwarf's quest — they show
	// as locked, with the unmet gate spelled out.
	for _, id := range []string{"paladins-vow", "sword-of-the-ancestors"} {
		if seen[id] != "locked" {
			t.Errorf("%s: status %q, want locked", id, seen[id])
		}
	}
	for _, q := range log.Locked {
		unmet := 0
		for _, r := range q.Requirements {
			if r.Description == "" {
				t.Errorf("%s: requirement with no description", q.ID)
			}
			if !r.Met {
				unmet++
			}
		}
		if unmet == 0 {
			t.Errorf("%s is locked but every requirement reads as met", q.ID)
		}
	}

	// A sequel waits on its prerequisite and says so.
	if seen["the-shadows-source"] == "locked" {
		for _, q := range log.Locked {
			if q.ID != "the-shadows-source" {
				continue
			}
			if len(q.Requirements) == 0 || q.Requirements[0].Description != "Complete The Rising Shadow" || q.Requirements[0].Met {
				t.Errorf("the-shadows-source should lead with an unmet 'Complete The Rising Shadow', got %+v", q.Requirements)
			}
		}
	}
}

func TestDescribeRequirementFallbacks(t *testing.T) {
	cases := map[string]types.POIRequirement{
		"Athletics 10":           {Type: "skill", ID: "athletics", Min: 10},
		"Level 3":                {Type: "level", Min: 3},
		"5 Quest Points":         {Type: "quest_points", Min: 5},
		"Class: Paladin or Monk": {Type: "class", Values: []string{"paladin", "monk"}},
		"Authored wins":          {Type: "level", Min: 9, Description: "Authored wins"},
	}
	for want, req := range cases {
		if got := describeRequirement(req); got != want {
			t.Errorf("describeRequirement(%+v) = %q, want %q", req, got, want)
		}
	}
}
