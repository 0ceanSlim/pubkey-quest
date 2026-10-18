package effects

import (
	"reflect"
	"testing"
)

func TestWornEffectIDsByRace(t *testing.T) {
	props := map[string]interface{}{
		"effects_when_worn":         []interface{}{"ancestral-might"},
		"effects_when_worn_by_race": map[string]interface{}{"Dwarf": []interface{}{"clans-endurance"}},
	}
	if got := WornEffectIDs(props, "dwarf"); !reflect.DeepEqual(got, []string{"ancestral-might", "clans-endurance"}) {
		t.Fatalf("dwarf: %v", got)
	}
	if got := WornEffectIDs(props, "Elf"); !reflect.DeepEqual(got, []string{"ancestral-might"}) {
		t.Fatalf("elf: %v", got)
	}
	if got := WornEffectIDs(map[string]interface{}{}, "Dwarf"); len(got) != 0 {
		t.Fatalf("no worn effects: %v", got)
	}
}
