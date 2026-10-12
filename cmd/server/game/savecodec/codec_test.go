package savecodec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"pubkey-quest/types"
)

func loadFixtures(t *testing.T) map[string]*types.SaveFile {
	t.Helper()
	paths, _ := filepath.Glob("testdata/save*.json")
	if len(paths) == 0 {
		t.Fatal("no fixtures")
	}
	out := map[string]*types.SaveFile{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var s types.SaveFile
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[p] = &s
	}
	return out
}

// itemsOf lists every (place, item, qty) in an inventory, ignoring empties.
func itemsOf(inv map[string]any) []string {
	var out []string
	add := func(place string, m map[string]any) {
		if id, _ := m["item"].(string); id != "" {
			out = append(out, fmt.Sprintf("%s:%s×%d", place, id, toInt(m["quantity"])))
		}
		if contents, ok := m["contents"].([]any); ok {
			for i, raw := range contents {
				c, _ := raw.(map[string]any)
				if id, _ := c["item"].(string); id != "" {
					out = append(out, fmt.Sprintf("%s/%d:%s×%d", place, i, id, toInt(c["quantity"])))
				}
			}
			out = append(out, fmt.Sprintf("%s:cap%d", place, len(contents)))
		}
	}
	if gear, ok := inv["gear_slots"].(map[string]any); ok {
		for name, raw := range gear {
			m, _ := raw.(map[string]any)
			add("gear."+name, m)
		}
	}
	if gen, ok := inv["general_slots"].([]any); ok {
		for i, raw := range gen {
			m, _ := raw.(map[string]any)
			add(fmt.Sprintf("gen.%d", i), m)
		}
		out = append(out, fmt.Sprintf("gen:cap%d", len(gen)))
	}
	sort.Strings(out)
	return out
}

func spellsOf(ss map[string]any) []string {
	var out []string
	for level, raw := range ss {
		list, _ := raw.([]any)
		for i, e := range list {
			m, _ := e.(map[string]any)
			sp, _ := m["spell"].(string)
			out = append(out, fmt.Sprintf("%s/%d:%s×%d", level, i, sp, toInt(m["quantity"])))
		}
	}
	sort.Strings(out)
	return out
}

func TestRoundTripKeepsEverythingThatMatters(t *testing.T) {
	for name, orig := range loadFixtures(t) {
		data, err := Encode(orig)
		if err != nil {
			t.Fatalf("%s encode: %v", name, err)
		}
		got, err := Decode(data)
		if err != nil {
			t.Fatalf("%s decode: %v", name, err)
		}

		// Scalars and lists come back exactly.
		checks := []struct {
			field     string
			want, got any
		}{
			{"name", orig.D, got.D}, {"race", orig.Race, got.Race}, {"class", orig.Class, got.Class},
			{"xp", orig.Experience, got.Experience}, {"hp", orig.HP, got.HP}, {"mana", orig.Mana, got.Mana},
			{"place", [4]string{orig.Location, orig.District, orig.Building, orig.Room}, [4]string{got.Location, got.District, got.Building, got.Room}},
			{"time", [2]int{orig.CurrentDay, orig.TimeOfDay}, [2]int{got.CurrentDay, got.TimeOfDay}},
			{"spells", orig.KnownSpells, got.KnownSpells},
			{"discovered", orig.LocationsDiscovered, got.LocationsDiscovered},
			{"effects", orig.ActiveEffects, got.ActiveEffects},
			{"quests", orig.QuestsActive, got.QuestsActive},
			{"rentals", orig.Rentals, got.Rentals},
		}
		for _, c := range checks {
			if !reflect.DeepEqual(c.want, c.got) && !bothEmpty(c.want, c.got) {
				t.Errorf("%s %s: want %v, got %v", name, c.field, c.want, c.got)
			}
		}
		for _, a := range abilityOrder {
			if toInt(orig.Stats[a]) != toInt(got.Stats[a]) {
				t.Errorf("%s stat %s: %v vs %v", name, a, orig.Stats[a], got.Stats[a])
			}
		}
		if w, g := itemsOf(orig.Inventory), itemsOf(got.Inventory); !reflect.DeepEqual(w, g) {
			t.Errorf("%s inventory differs:\n want %v\n  got %v", name, w, g)
		}
		if w, g := spellsOf(orig.SpellSlots), spellsOf(got.SpellSlots); !reflect.DeepEqual(w, g) {
			t.Errorf("%s spell slots differ:\n want %v\n  got %v", name, w, g)
		}

		// Encoding the decoded save gives the same bytes: the format is stable.
		again, _ := Encode(got)
		if string(again) != string(data) {
			t.Errorf("%s: re-encode differs\n%s\n%s", name, data, again)
		}
		pretty, _ := json.MarshalIndent(orig, "", "  ")
		t.Logf("%s: %d bytes on disk → %d bytes compact", name, len(pretty), len(data))
		if len(data) > 16<<10 {
			t.Errorf("%s: %d bytes is over the 16 KB budget", name, len(data))
		}
	}
}

func TestDecodeRejectsUnknownVersions(t *testing.T) {
	if _, err := Decode([]byte(`{"v":99,"n":"x"}`)); err == nil {
		t.Error("a save from the future should be refused, not misread")
	}
	if _, err := Decode([]byte(`{"n":"x"}`)); err == nil {
		t.Error("a save without a version should be refused")
	}
}

func TestUnknownFieldsSurvive(t *testing.T) {
	s := &types.SaveFile{
		D:     "Odd",
		Stats: map[string]any{"Strength": 10.0, "Luck": 3.0},
		Inventory: map[string]any{
			"gear_slots":    map[string]any{"mainhand": map[string]any{"item": "dagger", "quantity": 1.0, "durability": 7.0}},
			"general_slots": []any{},
			"pockets":       []any{"lint"},
		},
	}
	data, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stats["Luck"] != 3.0 {
		t.Errorf("extra stat lost: %v", got.Stats)
	}
	mh := got.Inventory["gear_slots"].(map[string]any)["mainhand"].(map[string]any)
	if mh["durability"] != 7.0 {
		t.Errorf("extra slot field lost: %v", mh)
	}
	if _, ok := got.Inventory["pockets"]; !ok {
		t.Error("extra inventory key lost")
	}
}

// bothEmpty treats a nil list and an empty one as the same.
func bothEmpty(a, b any) bool {
	ra, rb := reflect.ValueOf(a), reflect.ValueOf(b)
	return ra.Kind() == reflect.Slice && rb.Kind() == reflect.Slice && ra.Len() == 0 && rb.Len() == 0
}
