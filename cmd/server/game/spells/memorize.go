package spells

import (
	"database/sql"
	"fmt"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/spellbook"
	"pubkey-quest/types"
)

// ─── Committing a scroll to memory (wizards) ────────────────────────────────
//
// A wizard with a spell scroll inside a spellbook container can commit it to
// memory: they learn the spell for good (SpellsMemorized), the scroll is used
// up, and it costs gold and time per spell level (scroll-scribing.json
// "memorize"). No roll. Only spells on the wizard's list, of a level they have
// slots for, not already known; cantrips can't be memorized.

// SpellbookItem is the container a scroll must sit in to be memorized.
const SpellbookItem = "spellbook"

// MemorizeResult is what committing a scroll cost.
type MemorizeResult struct {
	SpellID string `json:"spell_id"`
	Name    string `json:"name"`
	Gold    int    `json:"gold"`
	Minutes int    `json:"minutes"`
}

// CanMemorize reports whether the class commits scrolls to memory.
func CanMemorize(db *sql.DB, class string) bool {
	for _, c := range scribingTable(db).Memorize.Classes {
		if strings.EqualFold(c, class) {
			return true
		}
	}
	return false
}

// Memorize commits the scroll at contents[fromSlot] of the spellbook at
// (containerSlot, slotType) to memory. slotType is "general" (a general slot)
// or "equipment" (the book worn in a gear slot). dryRun checks everything and
// reports the cost without spending anything (the confirm prompt).
func Memorize(db *sql.DB, save *types.SaveFile, containerSlot int, slotType string, fromSlot int, dryRun bool) (*MemorizeResult, error) {
	if !CanMemorize(db, save.Class) {
		return nil, fmt.Errorf("only wizards can commit spells to memory")
	}
	book := findSpellbook(save, containerSlot, slotType)
	if book == nil {
		return nil, fmt.Errorf("the scroll has to be in your spellbook")
	}
	contents, _ := book["contents"].([]interface{})
	entry := slotAt(contents, fromSlot)
	if entry == nil {
		return nil, fmt.Errorf("there's no scroll there")
	}
	itemID, _ := entry["item"].(string)
	spellID, err := scrollSpell(db, itemID)
	if err != nil {
		return nil, err
	}

	level := spellbook.SpellLevel(spellID)
	switch {
	case level < 1:
		return nil, fmt.Errorf("cantrips can't be committed to memory")
	case !spellbook.OnClassList(save.Class, spellID):
		return nil, fmt.Errorf("that isn't a %s spell", strings.ToLower(save.Class))
	case level > spellbook.MaxSpellLevel(save.Class, spellbook.Level(save)):
		return nil, fmt.Errorf("you need level-%d spell slots to learn that", level)
	case spellbook.Knows(save, spellID):
		return nil, fmt.Errorf("you already know that spell")
	}
	cfg := scribingTable(db).Memorize
	res := &MemorizeResult{SpellID: spellID, Gold: cfg.GoldPerLevel * level, Minutes: cfg.MinutesPerLevel * level}
	if spell, err := gamedata.LoadSpellByID(db, spellID); err == nil {
		res.Name = stringField(spell, "name")
	}
	if res.Name == "" {
		res.Name = spellID
	}
	if gameutil.GetGoldQuantity(save) < res.Gold {
		return nil, fmt.Errorf("committing %s to memory costs %d gold", res.Name, res.Gold)
	}
	if dryRun {
		return res, nil
	}

	// Spend: the scroll, the gold, the time.
	decrementSlotQuantity(entry)
	gameutil.DeductGold(save, res.Gold)
	gametime.AdvanceTime(save, res.Minutes, true)
	save.SpellsMemorized = append(save.SpellsMemorized, spellID)
	return res, nil
}

// scrollSpell is the spell a scroll item carries.
func scrollSpell(db *sql.DB, itemID string) (string, error) {
	if itemID == "" {
		return "", fmt.Errorf("there's no scroll there")
	}
	item, err := gamedata.LoadItemByID(db, itemID)
	if err != nil {
		return "", fmt.Errorf("unknown item %s", itemID)
	}
	if spellID, _ := item["spell_id"].(string); spellID != "" {
		return spellID, nil
	}
	return "", fmt.Errorf("that isn't a spell scroll")
}

// findSpellbook finds the spellbook container the client named.
func findSpellbook(save *types.SaveFile, containerSlot int, slotType string) map[string]interface{} {
	isBook := func(m map[string]interface{}) bool {
		id, _ := m["item"].(string)
		return id == SpellbookItem
	}
	if slotType == "general" {
		gen, _ := save.Inventory["general_slots"].([]interface{})
		if m := slotAt(gen, containerSlot); m != nil && isBook(m) {
			return m
		}
		return nil
	}
	gear, _ := save.Inventory["gear_slots"].(map[string]interface{})
	for _, raw := range gear {
		if m, ok := raw.(map[string]interface{}); ok && isBook(m) {
			return m
		}
	}
	return nil
}

// slotAt is the slot whose "slot" number is n (or the n-th entry when slots
// carry no number).
func slotAt(slots []interface{}, n int) map[string]interface{} {
	for i, raw := range slots {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		num := i
		switch v := m["slot"].(type) {
		case float64:
			num = int(v)
		case int:
			num = v
		}
		if num == n {
			return m
		}
	}
	return nil
}

func decrementSlotQuantity(slot map[string]interface{}) {
	q := 1
	switch v := slot["quantity"].(type) {
	case float64:
		q = int(v)
	case int:
		q = v
	}
	if q <= 1 {
		slot["item"] = nil
		slot["quantity"] = 0
		return
	}
	slot["quantity"] = q - 1
}
