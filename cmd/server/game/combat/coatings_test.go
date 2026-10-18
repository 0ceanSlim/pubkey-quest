package combat

import (
	"os"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// A coating's extra damage lasts its number of hits, then wears off.
func TestCoatingWearsOffAfterItsHits(t *testing.T) {
	cs := boardWith(goblins(1))
	me := &cs.Party[0]
	me.CombatState.SpellBuffs = []types.SpellBuff{{Name: "Poisoned Blade", HitDice: "1d1", HitType: "poison", Hits: 3}}
	for i := 1; i <= 3; i++ {
		if extra, _ := buffHitDamage(cs, me, &cs.Monsters[0], true, false, false, "slashing"); extra != 1 {
			t.Fatalf("hit %d: extra %d, want 1", i, extra)
		}
	}
	if extra, _ := buffHitDamage(cs, me, &cs.Monsters[0], true, false, false, "slashing"); extra != 0 {
		t.Fatalf("fourth hit: extra %d, want 0 (worn off)", extra)
	}
	if len(me.CombatState.SpellBuffs) != 0 {
		t.Fatalf("the spent coating should be gone: %+v", me.CombatState.SpellBuffs)
	}
}

// A coating applied out of combat becomes a fight buff when the fight starts,
// once, and the effect is used up.
func TestTakeCoatings(t *testing.T) {
	// The test environment moves to the repo root; other tests here use paths
	// relative to this package, so come back afterwards.
	if wd, err := os.Getwd(); err == nil {
		t.Cleanup(func() { os.Chdir(wd) })
	}
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
	save := &types.SaveFile{ActiveEffects: []types.ActiveEffect{
		{EffectID: "honed-edge", DurationRemaining: 200},
		{EffectID: "honed-edge", DurationRemaining: 200}, // honed twice
		{EffectID: "poisoned-blade", DurationRemaining: 200},
	}}
	buffs := takeCoatings(save)
	if len(buffs) != 2 {
		t.Fatalf("got %d buffs, want 2 (one honed edge, one poison): %+v", len(buffs), buffs)
	}
	for _, b := range buffs {
		if b.SpellID == "poisoned-blade" && (b.HitDice != "1d4" || b.Hits != 3) {
			t.Errorf("poison buff: %+v", b)
		}
		if b.SpellID == "honed-edge" && b.HitBonus != 1 {
			t.Errorf("honed buff: %+v", b)
		}
	}
	for _, ae := range save.ActiveEffects {
		if ae.EffectID == "poisoned-blade" || ae.EffectID == "honed-edge" {
			t.Fatalf("coating effects should be used up, still have %s", ae.EffectID)
		}
	}
}
