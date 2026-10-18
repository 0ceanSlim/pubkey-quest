package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/types"
)

// spellConditions are the condition names the combat engine knows
// (game/combat/conditions.go). A spell naming anything else would land a
// condition that does nothing.
var spellConditions = map[string]bool{
	"blinded": true, "poisoned": true, "frightened": true, "prone": true, "restrained": true,
	"grappled": true, "stunned": true, "paralyzed": true, "unconscious": true, "outlined": true,
	"charmed": true, "distracted": true, "exhausted": true, "laughing": true, "baned": true,
	"dueled": true, "burning": true, "moonlit": true, "beset": true,
}

var abilityNames = map[string]bool{
	"strength": true, "dexterity": true, "constitution": true,
	"intelligence": true, "wisdom": true, "charisma": true,
}

// validateSpellMechanics checks the data-driven mechanics blocks on a spell:
// combat_buff, applies_condition and aoe must use known keys, conditions and
// shapes (an aoe needs a save_type),
// applies_effect and creates_item must point at real files, and a reaction
// must say what triggers it.
func validateSpellMechanics(spell map[string]interface{}, filename string) []Issue {
	var issues []Issue
	bad := func(field, format string, a ...interface{}) {
		issues = append(issues, Issue{Type: "error", Category: "spells", File: filename, Field: field, Message: fmt.Sprintf(format, a...)})
	}

	if raw, ok := spell["combat_buff"]; ok && raw != nil {
		var b types.SpellBuff
		if err := strictDecode(raw, &b); err != nil {
			bad("combat_buff", "combat_buff: %v", err)
		} else {
			switch b.Target {
			case "", "self", "ally", "party", "enemy":
			default:
				bad("combat_buff.target", "unknown target %q (self, ally, party, enemy)", b.Target)
			}
			if b.HitCondition != nil {
				issues = append(issues, checkSpellCondition(*b.HitCondition, "combat_buff.hit_condition", filename)...)
			}
		}
	}
	if raw, ok := spell["applies_condition"]; ok && raw != nil {
		var c types.SpellCondition
		if err := strictDecode(raw, &c); err != nil {
			bad("applies_condition", "applies_condition: %v", err)
		} else {
			issues = append(issues, checkSpellCondition(c, "applies_condition", filename)...)
		}
	}
	if raw, ok := spell["aoe"]; ok && raw != nil {
		var a types.SpellArea
		if err := strictDecode(raw, &a); err != nil {
			bad("aoe", "aoe: %v", err)
		} else {
			switch a.Shape {
			case "sphere", "self_radius", "cone", "line":
			default:
				bad("aoe.shape", "unknown shape %q (sphere, self_radius, cone, line)", a.Shape)
			}
			if a.Radius < 1 {
				bad("aoe.radius", "radius must be at least 1 cell (5 ft)")
			}
			// Each creature caught makes its own save, so an area needs one.
			if st, _ := spell["save_type"].(string); st == "" {
				bad("aoe", "an aoe spell needs a save_type: everyone caught saves against it")
			}
		}
	}
	if id, ok := spell["applies_effect"].(string); ok && id != "" {
		if _, err := os.Stat(filepath.Join("game-data", "effects", id+".json")); err != nil {
			bad("applies_effect", "applies_effect %q has no game-data/effects/%s.json", id, id)
		}
	}
	if mk, ok := spell["creates_item"].(map[string]interface{}); ok {
		id, _ := mk["item"].(string)
		if _, err := os.Stat(filepath.Join("game-data", "items", id+".json")); id == "" || err != nil {
			bad("creates_item", "creates_item %q has no game-data/items/%s.json", id, id)
		}
	}
	if ac, _ := spell["action_cost"].(string); ac == "reaction" {
		switch t, _ := spell["reaction_trigger"].(string); t {
		case "hit", "damaged":
		case "":
			// A reaction with no trigger only works out of combat (feather fall).
			if spell["applies_effect"] == nil {
				bad("reaction_trigger", "a reaction spell needs reaction_trigger (\"hit\" or \"damaged\") or an out-of-combat applies_effect")
			}
		default:
			bad("reaction_trigger", "unknown reaction_trigger %q (\"hit\" or \"damaged\")", t)
		}
	}
	return issues
}

func checkSpellCondition(c types.SpellCondition, field, filename string) []Issue {
	var issues []Issue
	bad := func(format string, a ...interface{}) {
		issues = append(issues, Issue{Type: "error", Category: "spells", File: filename, Field: field, Message: fmt.Sprintf(format, a...)})
	}
	if !spellConditions[strings.ToLower(c.Condition)] {
		bad("unknown condition %q", c.Condition)
	}
	for _, stat := range []string{c.SaveEachTurn, c.Save} {
		if stat != "" && !abilityNames[strings.ToLower(stat)] {
			bad("%q isn't an ability score", stat)
		}
	}
	if (c.DamageDice == "") != (c.DamageType == "") {
		bad("damage_dice and damage_type go together")
	}
	return issues
}

// strictDecode re-decodes a JSON value into v, refusing unknown keys.
func strictDecode(raw interface{}, v interface{}) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
