package combat

import (
	"testing"

	"pubkey-quest/types"
)

// One attack per action (beta exploit audit C1): a main-hand attack spends
// the action, and a second one in the same turn is refused, however it went.
func TestOneAttackPerAction(t *testing.T) {
	cs := boardWith(goblinsAt(types.Position{X: 2, Y: 3}))
	cs.Monsters[0].CurrentHP, cs.Monsters[0].MaxHP = 999, 999
	save := &types.SaveFile{Inventory: map[string]interface{}{}, Stats: map[string]interface{}{"strength": 10}}
	if _, err := ProcessPlayerAttack(nil, cs, save, cs.Monsters[0].InstanceID, "unarmed", "main", false, nil); err != nil {
		t.Fatalf("first attack: %v", err)
	}
	if _, err := ProcessPlayerAttack(nil, cs, save, cs.Monsters[0].InstanceID, "unarmed", "main", false, nil); err == nil {
		t.Fatal("a second attack in one turn")
	}
	// Action Surge's extra action allows exactly one more.
	cs.Party[0].CombatState.ExtraActions = 1
	if _, err := ProcessPlayerAttack(nil, cs, save, cs.Monsters[0].InstanceID, "unarmed", "main", false, nil); err != nil {
		t.Fatalf("the surged attack: %v", err)
	}
}

// A failed flee spends the action, so it can't be retried in the same turn
// until it works (C3).
func TestFleeOncePerTurn(t *testing.T) {
	cs := boardWith(goblinsAt(types.Position{X: 7, Y: 3}))
	cs.Party[0].CombatState.ActionUsed = true
	if _, err := ProcessPlayerFlee(cs, &types.SaveFile{}); err == nil {
		t.Fatal("fled after the action was spent")
	}
}

// XP comes from the HP a monster loses, not a blow's overkill (C4).
func TestNoXPForOverkill(t *testing.T) {
	m := goblins(1)[0]
	if dealt := applyDamageToMonster(&m, 50); dealt != 7 || m.IsAlive {
		t.Fatalf("dealt %d to a 7 HP goblin (alive %v)", dealt, m.IsAlive)
	}
}
