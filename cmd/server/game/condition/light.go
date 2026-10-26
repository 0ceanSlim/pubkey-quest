package condition

import (
	"encoding/json"
	"fmt"
	"sync"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// Lights (degradation §6): a torch, candle, lamp or lantern is unlit until a
// tinderbox lights it. Lit, it sheds light from your off hand and burns every
// game minute. A torch or candle is spent once it goes out; a lamp or lantern
// keeps the oil left in it, and runs dry until refilled with oil.
//
// Item data: "light": {"burn": 60, "spent": true} (torch, candle) or
// "light": {"burn": 360, "fuel": "oil"} (lamps, lanterns). Slot state: "l" is
// set while lit, "b" is the minutes of burn left (a lamp's oil, kept when it's
// out; missing means a full lamp or a fresh torch).

// KeyLit is set (1) on a slot whose light is burning; KeyShut on a lantern
// whose shutter is closed (it burns on, but sheds no light and gives nothing away).
const (
	KeyLit  = "l"
	KeyShut = "s"
)

func init() { StateKeys = append(StateKeys, KeyLit, KeyShut) }

// LightInfo is an item's "light" block.
type LightInfo struct {
	Burn  int    `json:"burn"`  // minutes one lighting (a torch) or one flask of fuel (a lamp) lasts
	Spent bool   `json:"spent"` // used up once it goes out (torches, candles)
	Fuel  string `json:"fuel"`  // what refills it ("oil")
	// Beam is the Perception a long beam adds at night (the bullseye lantern).
	Beam int `json:"beam,omitempty"`
	// Shutter: it can be closed while lit (the hooded lantern), to hide the light.
	Shutter bool `json:"shutter,omitempty"`
}

var (
	lightMu    sync.RWMutex
	lightCache = map[string]*LightInfo{}
)

// LightOf returns an item's light block, or nil if it isn't a light that burns
// (magic lights, the Amulet of Light, have none: they're always on).
func LightOf(itemID string) *LightInfo {
	lightMu.RLock()
	l, ok := lightCache[itemID]
	lightMu.RUnlock()
	if ok {
		return l
	}
	if item, err := db.GetItemByID(itemID); err == nil && item != nil {
		var props struct {
			Light *LightInfo `json:"light"`
		}
		if json.Unmarshal([]byte(item.Properties), &props) == nil && props.Light != nil && props.Light.Burn > 0 {
			l = props.Light
		}
	}
	lightMu.Lock()
	lightCache[itemID] = l
	lightMu.Unlock()
	return l
}

// IsLit reports whether a slot's light is burning.
func IsLit(slot map[string]interface{}) bool {
	_, ok := slot[KeyLit]
	return ok
}

// IsShut reports whether a lit lantern's shutter is closed.
func IsShut(slot map[string]interface{}) bool {
	_, ok := slot[KeyShut]
	return ok
}

// Shining reports whether a slot's light is giving light: lit, shutter open.
func Shining(slot map[string]interface{}) bool {
	return IsLit(slot) && !IsShut(slot)
}

// HeldLight is the light in the off hand (nil slot when there's none).
func HeldLight(save *types.SaveFile) (string, map[string]interface{}, *LightInfo) {
	gear, _ := save.Inventory["gear_slots"].(map[string]interface{})
	slot, _ := gear["offhand"].(map[string]interface{})
	id, _ := slot["item"].(string)
	if l := LightOf(id); l != nil {
		return id, slot, l
	}
	return "", nil, nil
}

// BurnLeft is the minutes a light has left: what's burning, or a lamp's oil.
// A light with no record is fresh: a whole torch, a full lamp.
func BurnLeft(slot map[string]interface{}) int {
	id, _ := slot["item"].(string)
	l := LightOf(id)
	if l == nil {
		return 0
	}
	if b, ok := intField(slot, KeyBurn); ok {
		return b
	}
	return l.Burn
}

// Ignite lights a light that's in a slot holding just it (quantity 1).
func Ignite(slot map[string]interface{}) error {
	id, _ := slot["item"].(string)
	l := LightOf(id)
	if l == nil {
		return fmt.Errorf("that doesn't light")
	}
	if IsLit(slot) {
		return fmt.Errorf("it's already lit")
	}
	left := BurnLeft(slot)
	if left <= 0 {
		return fmt.Errorf("the %s is dry: refill it with %s", ItemName(id), l.Fuel)
	}
	slot[KeyLit] = 1
	slot[KeyBurn] = left
	return nil
}

// Douse puts a light out. It reports whether the item is kept: a torch or
// candle is spent (the caller empties the slot); a lamp keeps its oil.
func Douse(slot map[string]interface{}) (kept bool) {
	id, _ := slot["item"].(string)
	l := LightOf(id)
	delete(slot, KeyLit)
	delete(slot, KeyShut)
	if l != nil && l.Spent {
		return false
	}
	return true
}

// Refill tops a lamp up to full (one flask of its fuel).
func Refill(slot map[string]interface{}) {
	id, _ := slot["item"].(string)
	if l := LightOf(id); l != nil {
		slot[KeyBurn] = l.Burn
	}
}

// NeedsFuel reports whether a slot holds a fuelled light that isn't full.
func NeedsFuel(slot map[string]interface{}, fuel string) bool {
	id, _ := slot["item"].(string)
	l := LightOf(id)
	return l != nil && l.Fuel == fuel && BurnLeft(slot) < l.Burn
}

// BurnHeld burns the lit light in the off hand down by minutes. A torch or
// candle that burns out is gone; a lamp runs dry and stays, unlit. The
// messages say what went out.
func BurnHeld(save *types.SaveFile, minutes int) []string {
	gear, _ := save.Inventory["gear_slots"].(map[string]interface{})
	slot, _ := gear["offhand"].(map[string]interface{})
	if slot == nil || !IsLit(slot) || minutes <= 0 {
		return nil
	}
	id, _ := slot["item"].(string)
	left := BurnLeft(slot) - minutes
	if left > 0 {
		slot[KeyBurn] = left
		return nil
	}
	name := ItemName(id)
	if !Douse(slot) {
		gear["offhand"] = map[string]interface{}{"item": nil, "quantity": 0}
		return []string{fmt.Sprintf("Your %s burns out.", name)}
	}
	slot[KeyBurn] = 0
	return []string{fmt.Sprintf("Your %s runs dry. Refill it with oil.", name)}
}

// ResetLightCache forgets the cached light data (tests).
func ResetLightCache() {
	lightMu.Lock()
	lightCache = map[string]*LightInfo{}
	lightMu.Unlock()
}
