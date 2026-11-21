package combat

import (
	"os"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

func wornDB(t *testing.T) {
	t.Helper()
	if serverdb.GetDB() != nil {
		return // opened by an earlier test
	}
	if wd, err := os.Getwd(); err == nil {
		t.Cleanup(func() { os.Chdir(wd) })
	}
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
}

// Worn riders ride every fight and aren't used up; the same gem's rider
// counts once, the stronger one winning (docs/draft/enchanting.md §1.5).
func TestWornBuffsCountOncePerGem(t *testing.T) {
	wornDB(t)
	save := &types.SaveFile{ActiveEffects: []types.ActiveEffect{
		{EffectID: "jewel-ruby-large"}, {EffectID: "jewel-ruby-large"}, {EffectID: "jewel-ruby-huge"},
		{EffectID: "jewel-diamond-huge-flawed"}, {EffectID: "jewel-diamond-large-flawless"},
		{EffectID: "jewel-topaz-huge"},
	}}
	buffs := wornBuffs(save)
	if len(buffs) != 3 {
		t.Fatalf("want one ruby, one diamond, one topaz rider: %+v", buffs)
	}
	for _, b := range buffs {
		switch b.SpellID {
		case "jewel-ruby":
			if b.Name != "Emberheart" || b.HitDice != "1d6" {
				t.Errorf("the huge ruby's rider wins: %+v", b)
			}
		case "jewel-diamond":
			if len(b.Resist) != 5 {
				t.Errorf("the flawless large diamond (5 resists) beats the flawed huge one (2): %+v", b)
			}
		}
	}
	if len(takeCoatings(save)) != 0 || len(save.ActiveEffects) != 6 {
		t.Fatalf("worn buffs aren't coatings, and aren't used up: %+v", save.ActiveEffects)
	}
}

// A black pearl's ward turns only the fight's first attack.
func TestSanctuaryOnce(t *testing.T) {
	cs := boardWith(goblins(1))
	me := &cs.Party[0]
	me.CombatState.SpellBuffs = []types.SpellBuff{{SpellID: "jewel-black-pearl", Name: "Abyssal Pearl", Sanctuary: true, SanctuaryOnce: true, SaveDC: 99}}
	if _, _, lost, _ := attackDefense(cs, me, &cs.Monsters[0], false); !lost {
		t.Fatal("the first attack should be turned")
	}
	if _, _, lost, _ := attackDefense(cs, me, &cs.Monsters[0], false); lost {
		t.Fatal("only the first attack is turned")
	}
	if len(me.CombatState.SpellBuffs) != 0 {
		t.Fatalf("the spent ward should be gone: %+v", me.CombatState.SpellBuffs)
	}
}

// Heart of Change grows a class pool by 10%, at least 1, and a pool that
// starts full starts at its new max.
func TestGrowResourcePool(t *testing.T) {
	stamina := &types.ResourcePool{Current: 10, Max: 10}
	growResourcePool(stamina, 10)
	if stamina.Max != 11 || stamina.Current != 11 {
		t.Fatalf("stamina: %+v", stamina)
	}
	rage := &types.ResourcePool{Current: 0, Max: 100}
	growResourcePool(rage, 10)
	if rage.Max != 110 || rage.Current != 0 {
		t.Fatalf("rage: %+v", rage)
	}
	growResourcePool(nil, 10)
}

// Wellspring's mana at a fight's start stops at the max.
func TestFightStartMana(t *testing.T) {
	wornDB(t)
	save := &types.SaveFile{Mana: 4, MaxMana: 5, ActiveEffects: []types.ActiveEffect{{EffectID: "jewel-azurite-huge", EffectIndex: 1}}}
	if n := fightStartMana(save); n != 1 || save.Mana != 5 {
		t.Fatalf("added %d, mana %d", n, save.Mana)
	}
}
