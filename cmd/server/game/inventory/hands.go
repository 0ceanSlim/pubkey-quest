package inventory

import (
	"log"

	"pubkey-quest/types"
)

// NormalizeHands repairs hands that break the two-handed rule. A two-handed
// weapon is held in both hands — stored as the same item in the mainhand and
// offhand slots, the way equipping one does it. Older starting gear could leave
// a bow in the main hand with a staff in the off hand; on load the two-handed
// weapon keeps both hands and the other item goes to the pack. When there's no
// room in the pack nothing changes. Reports whether it changed anything.
func NormalizeHands(save *types.SaveFile) bool {
	gear, ok := save.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return false
	}
	main, _ := gear["mainhand"].(map[string]interface{})
	off, _ := gear["offhand"].(map[string]interface{})
	mainID, _ := main["item"].(string)
	offID, _ := off["item"].(string)

	twoHander, twoSlot, otherSlot := "", "", ""
	switch {
	case mainID != "" && mainID != offID && isTwoHandedWeapon(mainID):
		twoHander, twoSlot, otherSlot = mainID, "mainhand", "offhand"
	case offID != "" && offID != mainID && isTwoHandedWeapon(offID):
		twoHander, twoSlot, otherSlot = offID, "offhand", "mainhand"
	default:
		return false
	}
	if other, _ := gear[otherSlot].(map[string]interface{}); other != nil {
		if id, _ := other["item"].(string); id != "" {
			if !stashInPack(save, other) {
				log.Printf("⚠️ %s held with %s, but the pack is full — hands left as they are", twoHander, id)
				return false
			}
			log.Printf("🔧 %s needs both hands: moved %s to the pack", twoHander, id)
		}
	}
	held, _ := gear[twoSlot].(map[string]interface{})
	gear[otherSlot] = map[string]interface{}{"item": twoHander, "quantity": held["quantity"]}
	return true
}

// stashInPack puts a slot's contents (item, quantity, and a container's own
// contents) into the first empty general slot, then the backpack.
func stashInPack(save *types.SaveFile, slot map[string]interface{}) bool {
	place := func(slots []interface{}) bool {
		for i, raw := range slots {
			s, ok := raw.(map[string]interface{})
			if raw != nil && !ok {
				continue
			}
			if s != nil {
				if id, _ := s["item"].(string); id != "" {
					continue
				}
			}
			moved := map[string]interface{}{}
			for k, v := range slot {
				moved[k] = v
			}
			if s != nil {
				if idx, ok := s["slot"]; ok {
					moved["slot"] = idx
				}
			}
			slots[i] = moved
			return true
		}
		return false
	}
	if general, ok := save.Inventory["general_slots"].([]interface{}); ok && place(general) {
		return true
	}
	if gear, ok := save.Inventory["gear_slots"].(map[string]interface{}); ok {
		if bag, ok := gear["bag"].(map[string]interface{}); ok {
			if contents, ok := bag["contents"].([]interface{}); ok && place(contents) {
				return true
			}
		}
	}
	return false
}
