package effects

import (
	"encoding/json"
	"log"
	"strings"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// WornEffectIDs lists the effects an equipped item grants its wearer: its
// effects_when_worn, plus any effects_when_worn_by_race entry for the wearer's
// race (the Axe of the Ancestors' dwarf-only fury) and effects_when_worn_by_class
// entry for their class (alexandrite follows the class), matched
// case-insensitively. SyncWorn uses this, so equip, unequip and load agree.
func WornEffectIDs(properties map[string]interface{}, race, class string) []string {
	var ids []string
	add := func(raw interface{}) {
		list, _ := raw.([]interface{})
		for _, v := range list {
			if s, ok := v.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
	}
	add(properties["effects_when_worn"])
	for key, who := range map[string]string{"effects_when_worn_by_race": race, "effects_when_worn_by_class": class} {
		if by, ok := properties[key].(map[string]interface{}); ok {
			for k, list := range by {
				if strings.EqualFold(k, who) {
					add(list)
				}
			}
		}
	}
	return ids
}

// SyncWorn makes the worn effects (removal "equipment") match the gear: one
// instance of each effect per equipped piece that grants it, and none for a
// piece that's gone. Every gear change runs it, however the piece came off —
// a swap, a drag, a sale — so a worn effect can't outlive its piece or stack
// by swapping. Two pieces granting the same effect (a ruby ring on each hand)
// give it twice: abilities and skills add, combat riders count once (combat).
func SyncWorn(state *types.SaveFile) {
	if state == nil || db.GetDB() == nil {
		return
	}
	syncWorn(state, wornWanted(state), LoadEffectData)
}

// wornWanted counts the worn effects the equipped gear grants.
func wornWanted(state *types.SaveFile) map[string]int {
	want := map[string]int{}
	gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
	held := func(slot string) string {
		m, _ := gear[slot].(map[string]interface{})
		id, _ := m["item"].(string)
		return id
	}
	for slot := range gear {
		id := held(slot)
		if id == "" {
			continue
		}
		props := itemProperties(id)
		if props == nil {
			continue
		}
		// A two-handed weapon fills both hands under one id: it's one piece.
		if slot == "offhand" && id == held("mainhand") && hasTag(props, "two-handed") {
			continue
		}
		for _, e := range WornEffectIDs(props, state.Race, state.Class) {
			want[e]++
		}
	}
	return want
}

// syncWorn reconciles the active worn effects with want.
func syncWorn(state *types.SaveFile, want map[string]int, load func(string) (*types.EffectData, error)) {
	have := map[string]int{}
	worn := map[string]bool{}
	for _, ae := range state.ActiveEffects {
		if _, seen := worn[ae.EffectID]; !seen {
			data, err := load(ae.EffectID)
			worn[ae.EffectID] = err == nil && data.Removal.Type == "equipment"
		}
		if worn[ae.EffectID] {
			have[ae.EffectID] = Instances(state, ae.EffectID)
		}
	}
	for id, n := range have {
		for ; n > want[id]; n-- {
			removeInstance(state, id)
		}
	}
	for id, n := range want {
		for i := have[id]; i < n; i++ {
			if err := ApplyEffect(state, id); err != nil {
				log.Printf("⚠️ Failed to apply worn effect '%s': %v", id, err)
				break
			}
		}
	}
}

// Instances is how many times an effect is on: its entries for one modifier
// (each application adds one entry per modifier).
func Instances(state *types.SaveFile, effectID string) int {
	perIndex := map[int]int{}
	n := 0
	for _, ae := range state.ActiveEffects {
		if ae.EffectID == effectID {
			perIndex[ae.EffectIndex]++
			n = max(n, perIndex[ae.EffectIndex])
		}
	}
	return n
}

// removeInstance takes one application of an effect off: the last entry for
// each of its modifiers.
func removeInstance(state *types.SaveFile, effectID string) {
	drop := map[int]int{} // modifier index → position of its last entry
	for i, ae := range state.ActiveEffects {
		if ae.EffectID == effectID {
			drop[ae.EffectIndex] = i
		}
	}
	out := state.ActiveEffects[:0]
	for i, ae := range state.ActiveEffects {
		if ae.EffectID == effectID && drop[ae.EffectIndex] == i {
			continue
		}
		out = append(out, ae)
	}
	state.ActiveEffects = out
}

// itemProperties reads an item's properties, or nil.
func itemProperties(itemID string) map[string]interface{} {
	var raw string
	if db.GetDB().QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&raw) != nil {
		return nil
	}
	var props map[string]interface{}
	if json.Unmarshal([]byte(raw), &props) != nil {
		return nil
	}
	return props
}

func hasTag(props map[string]interface{}, tag string) bool {
	tags, _ := props["tags"].([]interface{})
	for _, t := range tags {
		if s, _ := t.(string); s == tag {
			return true
		}
	}
	return false
}
