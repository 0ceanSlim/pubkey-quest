package combat

import (
	"testing"

	"pubkey-quest/types"
)

func cc(name string) []types.CombatCondition { return []types.CombatCondition{{Name: name}} }

func TestConditionAttackAdvantage(t *testing.T) {
	none := []types.CombatCondition{}
	// Target restrained → attackers get advantage (+1).
	if got := ConditionAttackAdvantage(none, cc("restrained")); got != 1 {
		t.Errorf("restrained target: got %d, want +1", got)
	}
	// Attacker poisoned → its attacks are at disadvantage (−1).
	if got := ConditionAttackAdvantage(cc("poisoned"), none); got != -1 {
		t.Errorf("poisoned attacker: got %d, want -1", got)
	}
	// Poisoned attacker vs restrained target → advantage and disadvantage cancel (0).
	if got := ConditionAttackAdvantage(cc("poisoned"), cc("restrained")); got != 0 {
		t.Errorf("cancel: got %d, want 0", got)
	}
	// Blinded is both: attackers vs a blinded creature have advantage; a blinded
	// creature attacks at disadvantage.
	if got := ConditionAttackAdvantage(none, cc("blinded")); got != 1 {
		t.Errorf("blinded target: got %d, want +1", got)
	}
	if got := ConditionAttackAdvantage(cc("blinded"), none); got != -1 {
		t.Errorf("blinded attacker: got %d, want -1", got)
	}
	// Outlined (faerie-fire) grants attackers advantage but doesn't hurt its own attacks.
	if got := ConditionAttackAdvantage(none, cc("outlined")); got != 1 {
		t.Errorf("outlined target: got %d, want +1", got)
	}
	if got := ConditionAttackAdvantage(cc("outlined"), none); got != 0 {
		t.Errorf("outlined attacker: got %d, want 0", got)
	}
}

func TestIsIncapacitated(t *testing.T) {
	for _, n := range []string{"stunned", "paralyzed", "unconscious"} {
		if !IsIncapacitated(cc(n)) {
			t.Errorf("%s should incapacitate", n)
		}
	}
	for _, n := range []string{"poisoned", "restrained", "blinded", "outlined", "charmed"} {
		if IsIncapacitated(cc(n)) {
			t.Errorf("%s should NOT incapacitate", n)
		}
	}
	if IsIncapacitated(nil) {
		t.Error("no conditions should not incapacitate")
	}
}

func TestApplyRemoveCondition(t *testing.T) {
	var conds []types.CombatCondition
	ApplyCondition(&conds, types.CombatCondition{Name: "restrained", DurationRounds: 5})
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	// Re-applying the same condition refreshes it in place (no duplicate).
	ApplyCondition(&conds, types.CombatCondition{Name: "restrained", DurationRounds: 10})
	if len(conds) != 1 || conds[0].DurationRounds != 10 {
		t.Errorf("re-apply should replace in place: %+v", conds)
	}
	ApplyCondition(&conds, types.CombatCondition{Name: "poisoned"})
	if len(conds) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(conds))
	}
	RemoveCondition(&conds, "restrained")
	if len(conds) != 1 || !HasCondition(conds, "poisoned") {
		t.Errorf("remove failed: %+v", conds)
	}
}

func TestTickConditions_SaveEndsIt(t *testing.T) {
	conds := []types.CombatCondition{{Name: "restrained", DurationRounds: 10, SaveDC: 12, SaveStat: "strength"}}
	log := TickCreatureConditions("Goblin", &conds, func(string) int { return 20 }) // always passes
	if len(conds) != 0 {
		t.Errorf("a passing save should end the condition, left %+v", conds)
	}
	if len(log) == 0 {
		t.Error("expected a log line for shaking it off")
	}
}

func TestTickConditions_DurationExpires(t *testing.T) {
	// Failing every save, the timed duration counts down and ends at 0.
	conds := []types.CombatCondition{{Name: "restrained", DurationRounds: 2, SaveDC: 12, SaveStat: "strength"}}
	TickCreatureConditions("Goblin", &conds, func(string) int { return 1 }) // fails
	if len(conds) != 1 || conds[0].DurationRounds != 1 {
		t.Fatalf("expected duration 1 remaining, got %+v", conds)
	}
	TickCreatureConditions("Goblin", &conds, func(string) int { return 1 })
	if len(conds) != 0 {
		t.Errorf("duration should reach 0 and end, left %+v", conds)
	}
}
