package combat

import (
	"math/rand"
	"testing"

	"pubkey-quest/types"
)

func TestRollAwarenessExtremes(t *testing.T) {
	g := goblinData()
	g.Senses.PassivePerception = 9
	monsters := []*types.MonsterData{&g}
	rng := rand.New(rand.NewSource(1))

	// A ghost with eagle eyes: always sees them, never seen.
	for i := 0; i < 20; i++ {
		if got := RollAwareness(rng, 30, 40, monsters).Opening; got != OpeningUnaware {
			t.Fatalf("stealthy + perceptive: got %s, want unaware", got)
		}
	}
	// Clumsy and oblivious: always ambushed.
	for i := 0; i < 20; i++ {
		if got := RollAwareness(rng, -30, -10, monsters).Opening; got != OpeningAmbush {
			t.Fatalf("loud + oblivious: got %s, want ambush", got)
		}
	}
	// Loud but perceptive: both sides see each other.
	for i := 0; i < 20; i++ {
		if got := RollAwareness(rng, -30, 40, monsters).Opening; got != OpeningNormal {
			t.Fatalf("loud + perceptive: got %s, want normal", got)
		}
	}
}

func TestSurprisedMonstersLoseTheirFirstTurn(t *testing.T) {
	ms := goblins(1)
	cs := boardWith(ms)
	cs.Monsters[0].Surprised = true
	cs.Monsters[0].Pos = types.Position{X: 2, Y: 3}

	log := runMonsterTurn(nil, cs, &types.SaveFile{}, &cs.Monsters[0], false)
	if len(log) != 1 || cs.Monsters[0].Surprised {
		t.Fatalf("expected one 'caught off guard' line and the flag cleared, got %v", log)
	}
	if cs.Monsters[0].Pos != (types.Position{X: 2, Y: 3}) {
		t.Error("a surprised monster moved")
	}
}

func TestPartyFirstKeepsOrderWithinSides(t *testing.T) {
	in := []types.InitiativeEntry{{ID: "a", Type: "monster"}, {ID: "p", Type: "player"}, {ID: "b", Type: "monster"}}
	out := partyFirst(in)
	if out[0].ID != "p" || out[1].ID != "a" || out[2].ID != "b" {
		t.Errorf("partyFirst = %v", out)
	}
}

func TestReadTheSigns(t *testing.T) {
	g := goblinData()
	monsters := []*types.MonsterData{&g}
	rng := rand.New(rand.NewSource(1))
	ambush := AwarenessRoll{Opening: OpeningAmbush}

	for i := 0; i < 20; i++ {
		if got := ReadTheSigns(rng, ambush, 40, monsters).Opening; got != OpeningNormal {
			t.Fatalf("a master tracker should always spot the ambush, got %v", got)
		}
		if got := ReadTheSigns(rng, ambush, -40, monsters).Opening; got != OpeningAmbush {
			t.Fatalf("a hopeless roll should leave the ambush, got %v", got)
		}
	}
	// Only ambushes are rolled against; other openings pass through.
	if got := ReadTheSigns(rng, AwarenessRoll{Opening: OpeningUnaware}, -40, monsters).Opening; got != OpeningUnaware {
		t.Fatalf("an unaware opening must pass through, got %v", got)
	}
}
