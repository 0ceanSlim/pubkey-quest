// Package survival is the wild's weather (docs/draft/survival-gear.md): the
// Frozen Wastes' cold and the Sunscorch Desert's heat, from survival.json.
// Out in either, every hour on the clock is a Constitution save; a failure
// costs fatigue. Gear, resistance, a drink or a campfire keep it off. The
// clock's hour marks drive it, so it needs no timer of its own: a spell of
// time that crosses three marks is three saves.
package survival

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

var (
	cfgMu sync.Mutex
	cfg   *types.SurvivalConfig
)

// Config is survival.json (empty when there's no database yet).
func Config() *types.SurvivalConfig {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if cfg != nil {
		return cfg
	}
	database := serverdb.GetDB()
	if database == nil {
		return &types.SurvivalConfig{}
	}
	var props string
	if database.QueryRow("SELECT properties FROM systems WHERE id = 'survival'").Scan(&props) != nil {
		return &types.SurvivalConfig{}
	}
	c := &types.SurvivalConfig{}
	if json.Unmarshal([]byte(props), c) != nil {
		return &types.SurvivalConfig{}
	}
	cfg = c
	return cfg
}

// SetConfig replaces the loaded config (tests).
func SetConfig(c *types.SurvivalConfig) {
	cfgMu.Lock()
	cfg = c
	cfgMu.Unlock()
}

// Roll is the d20 (tests replace it).
var Roll = func() int { return rand.Intn(20) + 1 }

// Pass runs the hourly saves for awake time passing from minute from to
// minute to (absolute: day×1440 + time of day), and shows or clears the
// weather's effect for where you are now.
func Pass(state *types.SaveFile, from, to int) []types.EffectMessage {
	return expose(state, from, to, false)
}

// Sleep runs them for a night in the wild. A fire (one of the cold's fire
// items, yours or shared by the party) keeps the cold off all night; without
// one the saves keep coming while you sleep.
func Sleep(state *types.SaveFile, from, to int, fire bool) []types.EffectMessage {
	return expose(state, from, to, fire || HasFire(state))
}

// HasFire reports whether you carry something to light a camp's fire with.
func HasFire(state *types.SaveFile) bool {
	return carriesAny(state, Config().Cold.Fire)
}

func expose(state *types.SaveFile, from, to int, fire bool) []types.EffectMessage {
	c := Config()
	var msgs []types.EffectMessage
	for h := from/60 + 1; h*60 <= to; h++ {
		mark := h * 60
		if !bites(c.Heat, state.Location, mark%1440) && state.Limits != nil {
			state.Limits.Dry = 0 // out of the heat: you cool off
		}
		for _, e := range []*types.Exposure{&c.Cold, &c.Heat} {
			if !bites(*e, state.Location, mark%1440) {
				continue
			}
			if m := check(state, e, mark, fire); m != "" {
				msgs = append(msgs, types.EffectMessage{Message: m, Color: "yellow", Category: "survival"})
			}
		}
	}
	show(state, c, to)
	return msgs
}

// bites reports whether a weather is on at a place and time of day.
func bites(e types.Exposure, location string, timeOfDay int) bool {
	if e.DC <= 0 || !slices.Contains(e.Environments, location) {
		return false
	}
	if e.From == e.To {
		return true
	}
	return types.InWindow(timeOfDay, e.From*60, e.To*60)
}

// Protected reports whether something keeps a weather off you entirely:
// gear for it, resistance to it, or (asleep) a fire.
func Protected(state *types.SaveFile, e *types.Exposure, fire bool) bool {
	return carriesAny(state, e.Protect) || (fire && len(e.Fire) > 0) || resists(state, e.Resist)
}

// watered reports whether a drink is still keeping the heat off at minute at.
func watered(state *types.SaveFile, at int) bool {
	l := state.Limits
	return l != nil && l.Drank > 0 && at >= l.Drank && at-l.Drank < effects.WateredMinutes()
}

// check is one hour's save against a weather. Returns the log line, if any.
func check(state *types.SaveFile, e *types.Exposure, mark int, fire bool) string {
	if Protected(state, e, fire) {
		return ""
	}
	var drank string
	if e.Water {
		if watered(state, mark) {
			return ""
		}
		if drank = autoDrink(state, mark); drank != "" {
			return drank
		}
	}
	dc := e.DC
	if e.Water {
		l := state.Lim()
		dc += e.DCPerDryHour * l.Dry
		l.Dry++
	}
	adv := 0
	if carriesAny(state, e.Advantage) {
		adv++
	}
	if wearsArmor(state, e.Disadvantage) {
		adv--
	}
	roll := Roll()
	switch {
	case adv > 0:
		roll = max(roll, Roll())
	case adv < 0:
		roll = min(roll, Roll())
	}
	total := roll + abilityMod(effects.Ability(state, "constitution"))
	if total >= dc {
		return ""
	}
	status.Exert(state, &state.Lim().Exposure, max(e.Fatigue, 1), 1) // Unbroken halves it
	return fmt.Sprintf("%s (Constitution %d against %d)", e.Fail, total, dc)
}

// autoDrink drinks from a waterskin you carry, as a heat save comes due.
// Returns the log line, or "" with nothing to drink.
func autoDrink(state *types.SaveFile, mark int) string {
	for _, slots := range gameutil.CarriedSlotArrays(state) {
		for _, raw := range slots {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := slot["item"].(string)
			if c := condition.ChargesOf(id); c == nil || c.Refill != "water" || condition.ChargesLeft(slot) == 0 {
				continue
			}
			left, _ := condition.UseCharge(slot)
			effects.Drink(state, state.CurrentDay*1440+state.TimeOfDay-mark)
			name := condition.ItemName(id)
			if left == 0 {
				return fmt.Sprintf("💧 The heat is fierce. You drain the last of your %s.", strings.ToLower(name))
			}
			return fmt.Sprintf("💧 The heat is fierce. You take a swig from your %s (%d left).", strings.ToLower(name), left)
		}
	}
	return ""
}

// show puts a weather's effect on you while you're out in it unprotected, and
// takes it off otherwise, so the effects bar says what the clock is doing.
func show(state *types.SaveFile, c *types.SurvivalConfig, now int) {
	for _, e := range []*types.Exposure{&c.Cold, &c.Heat} {
		if e.Effect == "" {
			continue
		}
		on := bites(*e, state.Location, now%1440) && !Protected(state, e, false) && !(e.Water && watered(state, now))
		switch has := effects.HasActiveEffect(state, e.Effect); {
		case on && !has:
			_ = effects.ApplyEffect(state, e.Effect)
		case !on && has:
			effects.RemoveEffect(state, e.Effect)
		}
	}
}

// abilityMod is an ability score's modifier, rounding down.
func abilityMod(score int) int {
	return (score+10)/2 - 10
}

// carriesAny reports whether you carry any of the items.
func carriesAny(state *types.SaveFile, ids []string) bool {
	for _, id := range ids {
		if gameutil.PlayerHasItem(state, id) {
			return true
		}
	}
	return false
}

// resists reports whether an effect on you resists a damage type (a worn
// sapphire's Rimeheart, an enchanted diamond).
func resists(state *types.SaveFile, damage string) bool {
	if damage == "" {
		return false
	}
	seen := map[string]bool{}
	for _, a := range state.ActiveEffects {
		if seen[a.EffectID] {
			continue
		}
		seen[a.EffectID] = true
		if d, err := effects.LoadEffectData(a.EffectID); err == nil && d.CombatBuff != nil &&
			slices.ContainsFunc(d.CombatBuff.Resist, func(r string) bool { return strings.EqualFold(r, damage) }) {
			return true
		}
	}
	return false
}

// wearsArmor reports whether your chest armour is one of the item types.
func wearsArmor(state *types.SaveFile, itemTypes []string) bool {
	if len(itemTypes) == 0 {
		return false
	}
	gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
	slot, _ := gear["chest"].(map[string]interface{})
	id, _ := slot["item"].(string)
	if id == "" || serverdb.GetDB() == nil {
		return false
	}
	item, err := serverdb.GetItemByID(id)
	return err == nil && item != nil && slices.ContainsFunc(itemTypes, func(t string) bool { return strings.EqualFold(t, item.Type) })
}
