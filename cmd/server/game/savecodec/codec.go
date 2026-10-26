// Package savecodec turns a save into the compact, versioned form that goes
// into a signed Nostr event (docs/draft/relay-saves-plan.md §4), and back.
//
// The in-memory shape stays types.SaveFile. The wire form is short keys and
// tuples, with nothing that can be derived (max HP / mana come from class,
// level and stats on load — character.Hydrate) and no empty slots: capacities
// are recorded instead, and the empties are rebuilt on decode.
//
// Every wire form carries "v". Decode reads any version this build knows,
// migrating older ones forward step by step (migrations.go), so a save written
// years ago still loads. Never change what a version means once it ships —
// add a new version and a migration instead.
//
// Key map (v1):
//
//	v   format version            n   character name (the old "d")
//	ca  created_at                r/c/bg/al  race / class / background / alignment
//	xp  experience                hp/mp  current HP / mana
//	f/h fatigue / hunger          st  ability scores [STR,DEX,CON,INT,WIS,CHA]
//	ai  ability increases (same order)
//	pos [location, district, building, room]
//	tr  [travel progress, stopped 0|1]          t   [day, minute of day]
//	gr  gear: {slot: [item, qty, cap?, contents?, extras?]}
//	gn  general-slot count        gs  general: [[slot, item, qty, cap?, contents?, extras?]]
//	    contents: [[index, item, qty, extras?]]
//	ss  spell slots: {level: [[slot, spell, qty]]}
//	sp  known spells              ld  locations discovered      mu  music unlocked
//	fx  effects: [[id, index, remaining, total, delay, tick, applied_at]]
//	qc  quests completed          qa  quests active: [[id, stage, [counts]]]
//	pi  POI state: [[id, day, minute, flags 1=passed 2=looted 4=cleared]]
//	rn  rentals: [[building, day, minute]]
//	ft  feats                     rq  repeatable quests {id: period}
//	vt  vault: [[item, qty, state?]]      vk  vault keepers             pt  portrait URL
//	x*  anything this build didn't recognise, kept verbatim (stx, ix, …)
package savecodec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// Version is the wire format this build writes.
const Version = 1

// abilityOrder fixes where each ability score sits in st / ai.
var abilityOrder = []string{"Strength", "Dexterity", "Constitution", "Intelligence", "Wisdom", "Charisma"}

// standardGear are the gear slots every inventory has; empty ones aren't
// written and are rebuilt on decode.
var standardGear = []string{"ammo", "bag", "boots", "chest", "gloves", "head", "legs", "mainhand", "neck", "offhand", "ring1", "ring2"}

// wire is the v1 compact form.
type wire struct {
	V  int    `json:"v"`
	N  string `json:"n"`
	CA string `json:"ca,omitempty"`
	R  string `json:"r,omitempty"`
	C  string `json:"c,omitempty"`
	BG string `json:"bg,omitempty"`
	AL string `json:"al,omitempty"`
	XP int    `json:"xp"`
	HP int    `json:"hp"`
	MP int    `json:"mp"`
	F  int    `json:"f,omitempty"`
	H  int    `json:"h,omitempty"`

	St  []int          `json:"st"`
	Stx map[string]any `json:"stx,omitempty"`
	AI  []int          `json:"ai,omitempty"`
	AIx map[string]int `json:"aix,omitempty"`

	Pos [4]string `json:"pos"`
	Tr  []float64 `json:"tr,omitempty"`
	T   [2]int    `json:"t"`

	Gr map[string][]any `json:"gr,omitempty"`
	Gn int              `json:"gn,omitempty"`
	Gs [][]any          `json:"gs,omitempty"`
	Ix map[string]any   `json:"ix,omitempty"` // other inventory keys

	SS map[string][][]any `json:"ss,omitempty"`
	Sp []string           `json:"sp,omitempty"`
	LD []string           `json:"ld,omitempty"`
	Mu []string           `json:"mu,omitempty"`
	Fx [][]any            `json:"fx,omitempty"`

	QC []string        `json:"qc,omitempty"`
	QA [][]any         `json:"qa,omitempty"`
	PI [][]any         `json:"pi,omitempty"`
	RN [][]any         `json:"rn,omitempty"`
	FT []string        `json:"ft,omitempty"`
	RQ map[string]int  `json:"rq,omitempty"`
	VT [][]any         `json:"vt,omitempty"`
	VK []string        `json:"vk,omitempty"`
	PT string          `json:"pt,omitempty"`
	DG []string        `json:"dg,omitempty"` // dialogue gifts already given
	SM []string        `json:"sm,omitempty"` // spells a wizard memorized from scrolls
}

// ─── Encode ──────────────────────────────────────────────────────────────────

// Encode writes a save in the current compact form.
func Encode(s *types.SaveFile) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("savecodec: nil save")
	}
	w := wire{
		V: Version, N: s.D, CA: s.CreatedAt, R: s.Race, C: s.Class, BG: s.Background, AL: s.Alignment,
		XP: s.Experience, HP: s.HP, MP: s.Mana, F: s.Fatigue, H: s.Hunger,
		Pos: [4]string{s.Location, s.District, s.Building, s.Room},
		T:   [2]int{s.CurrentDay, s.TimeOfDay},
		Sp:  s.KnownSpells, LD: s.LocationsDiscovered, Mu: s.MusicTracksUnlocked,
		QC: s.QuestsCompleted, FT: s.FeatsChosen, RQ: s.RepeatableQuests, VK: s.VaultKeepers, PT: s.Portrait,
		DG: s.DialogueGifts, SM: s.SpellsMemorized,
	}
	w.St, w.Stx = encodeAbilities(s.Stats)
	if len(s.AbilityIncreases) > 0 {
		ints := map[string]any{}
		for k, v := range s.AbilityIncreases {
			ints[k] = v
		}
		ai, extra := encodeAbilities(ints)
		if !allZero(ai) {
			w.AI = ai
		}
		for k, v := range extra {
			if w.AIx == nil {
				w.AIx = map[string]int{}
			}
			w.AIx[k] = toInt(v)
		}
	}
	if s.TravelProgress != 0 || s.TravelStopped {
		stopped := 0.0
		if s.TravelStopped {
			stopped = 1
		}
		w.Tr = []float64{s.TravelProgress, stopped}
	}
	if err := encodeInventory(&w, s.Inventory); err != nil {
		return nil, err
	}
	w.SS = encodeSpellSlots(s.SpellSlots)
	for _, e := range s.ActiveEffects {
		w.Fx = append(w.Fx, []any{e.EffectID, e.EffectIndex, e.DurationRemaining, e.TotalDuration, e.DelayRemaining, e.TickAccumulator, e.AppliedAt})
	}
	for _, q := range s.QuestsActive {
		row := []any{q.QuestID, q.Stage}
		if len(q.ObjectiveCounts) > 0 {
			row = append(row, q.ObjectiveCounts)
		}
		w.QA = append(w.QA, row)
	}
	for _, p := range s.POIStates {
		flags := 0
		if p.Passed {
			flags |= 1
		}
		if p.Looted {
			flags |= 2
		}
		if p.Cleared {
			flags |= 4
		}
		w.PI = append(w.PI, []any{p.POIID, p.LastDay, p.LastMinute, flags})
	}
	for _, r := range s.Rentals {
		w.RN = append(w.RN, []any{r.Building, r.ExpiresDay, r.ExpiresMin})
	}
	for _, v := range s.Vault {
		if len(v.State) > 0 {
			w.VT = append(w.VT, []any{v.ItemID, v.Quantity, v.State})
		} else {
			w.VT = append(w.VT, []any{v.ItemID, v.Quantity})
		}
	}
	return json.Marshal(w)
}

// encodeAbilities packs the six ability scores into the fixed order; anything
// else in the map is kept aside.
func encodeAbilities(m map[string]any) ([]int, map[string]any) {
	out := make([]int, len(abilityOrder))
	var extra map[string]any
	for k, v := range m {
		idx := -1
		for i, name := range abilityOrder {
			if strings.EqualFold(k, name) {
				idx = i
			}
		}
		if idx < 0 {
			if extra == nil {
				extra = map[string]any{}
			}
			extra[k] = v
			continue
		}
		out[idx] = toInt(v)
	}
	return out, extra
}

// encodeInventory packs gear and general slots, dropping empties.
func encodeInventory(w *wire, inv map[string]any) error {
	for k, v := range inv {
		switch k {
		case "gear_slots":
			gear, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("savecodec: gear_slots is %T", v)
			}
			for name, raw := range gear {
				slot, _ := raw.(map[string]any)
				if t := encodeSlot(slot); t != nil {
					if w.Gr == nil {
						w.Gr = map[string][]any{}
					}
					w.Gr[name] = t
				} else if !isStandardGear(name) {
					// A non-standard empty gear slot still has to come back.
					if w.Gr == nil {
						w.Gr = map[string][]any{}
					}
					w.Gr[name] = []any{nil, 0}
				}
			}
		case "general_slots":
			list, ok := v.([]any)
			if !ok {
				return fmt.Errorf("savecodec: general_slots is %T", v)
			}
			w.Gn = len(list)
			for i, raw := range list {
				slot, _ := raw.(map[string]any)
				idx := i
				if n, ok := slot["slot"]; ok {
					idx = toInt(n)
				}
				if t := encodeSlot(slot); t != nil {
					w.Gs = append(w.Gs, append([]any{idx}, t...))
				}
			}
		default:
			if w.Ix == nil {
				w.Ix = map[string]any{}
			}
			w.Ix[k] = v
		}
	}
	return nil
}

// encodeSlot packs one slot: [item, qty, cap, contents, extras], trailing
// empties trimmed. Nil for an empty slot.
func encodeSlot(slot map[string]any) []any {
	if slot == nil {
		return nil
	}
	item := itemID(slot["item"])
	rawContents, hasContents := slot["contents"].([]any)
	if item == "" && !hasContents {
		return nil
	}
	t := []any{nullable(item), toInt(slot["quantity"])}
	var extras map[string]any
	for k, v := range slot {
		switch k {
		case "item", "quantity", "slot", "contents":
		default:
			if extras == nil {
				extras = map[string]any{}
			}
			extras[k] = v
		}
	}
	if hasContents {
		contents := [][]any{} // [] even when empty, so an empty container keeps its list
		for i, raw := range rawContents {
			c, _ := raw.(map[string]any)
			ci := itemID(c["item"])
			if ci == "" {
				continue
			}
			idx := i
			if n, ok := c["slot"]; ok {
				idx = toInt(n)
			}
			row := []any{idx, ci, toInt(c["quantity"])}
			var cx map[string]any
			for k, v := range c {
				switch k {
				case "item", "quantity", "slot":
				default:
					if cx == nil {
						cx = map[string]any{}
					}
					cx[k] = v
				}
			}
			if cx != nil {
				row = append(row, cx)
			}
			contents = append(contents, row)
		}
		t = append(t, len(rawContents), contents)
	}
	if extras != nil {
		if !hasContents {
			t = append(t, 0, nil)
		}
		t = append(t, extras)
	}
	return t
}

// encodeSpellSlots packs prepared spell slots per level.
func encodeSpellSlots(ss map[string]any) map[string][][]any {
	if len(ss) == 0 {
		return nil
	}
	out := map[string][][]any{}
	for level, raw := range ss {
		list, _ := raw.([]any)
		rows := [][]any{}
		for i, e := range list {
			m, _ := e.(map[string]any)
			idx := i
			if n, ok := m["slot"]; ok {
				idx = toInt(n)
			}
			rows = append(rows, []any{idx, nullable(itemID(m["spell"])), toInt(m["quantity"])})
		}
		out[level] = rows
	}
	return out
}

// ─── Decode ──────────────────────────────────────────────────────────────────

// Decode reads any known version of the compact form, migrating it forward.
// MaxHP/MaxMana come back zero: run character.Hydrate after decoding.
func Decode(data []byte) (*types.SaveFile, error) {
	var probe struct {
		V int `json:"v"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("savecodec: %w", err)
	}
	if probe.V < 1 || probe.V > Version {
		return nil, fmt.Errorf("savecodec: unknown save version %d (this build reads 1–%d)", probe.V, Version)
	}
	var err error
	for v := probe.V; v < Version; v++ {
		if data, err = migrations[v](data); err != nil {
			return nil, fmt.Errorf("savecodec: migrating v%d→v%d: %w", v, v+1, err)
		}
	}
	return decodeCurrent(data)
}

func decodeCurrent(data []byte) (*types.SaveFile, error) {
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("savecodec: %w", err)
	}
	s := &types.SaveFile{
		D: w.N, CreatedAt: w.CA, Race: w.R, Class: w.C, Background: w.BG, Alignment: w.AL,
		Experience: w.XP, HP: w.HP, Mana: w.MP, Fatigue: w.F, Hunger: w.H,
		Location: w.Pos[0], District: w.Pos[1], Building: w.Pos[2], Room: w.Pos[3],
		CurrentDay: w.T[0], TimeOfDay: w.T[1],
		KnownSpells: w.Sp, LocationsDiscovered: w.LD, MusicTracksUnlocked: w.Mu,
		QuestsCompleted: w.QC, FeatsChosen: w.FT, RepeatableQuests: w.RQ, VaultKeepers: w.VK, Portrait: w.PT,
		DialogueGifts: w.DG, SpellsMemorized: w.SM,
		SchemaVersion: types.CurrentSchemaVersion,
	}
	// These three are written as lists (never omitted) on disk and to the
	// client: keep them non-nil.
	if s.KnownSpells == nil {
		s.KnownSpells = []string{}
	}
	if s.LocationsDiscovered == nil {
		s.LocationsDiscovered = []string{}
	}
	if s.MusicTracksUnlocked == nil {
		s.MusicTracksUnlocked = []string{}
	}
	s.Stats = map[string]any{}
	for i, name := range abilityOrder {
		if i < len(w.St) {
			s.Stats[name] = float64(w.St[i])
		}
	}
	for k, v := range w.Stx {
		s.Stats[k] = v
	}
	if len(w.AI) > 0 || len(w.AIx) > 0 {
		s.AbilityIncreases = map[string]int{}
		for i, name := range abilityOrder {
			if i < len(w.AI) && w.AI[i] != 0 {
				s.AbilityIncreases[name] = w.AI[i]
			}
		}
		for k, v := range w.AIx {
			s.AbilityIncreases[k] = v
		}
	}
	if len(w.Tr) == 2 {
		s.TravelProgress, s.TravelStopped = w.Tr[0], w.Tr[1] != 0
	}
	s.Inventory = decodeInventory(&w)
	s.SpellSlots = decodeSpellSlots(w.SS)
	for _, r := range w.Fx {
		if len(r) < 7 {
			return nil, fmt.Errorf("savecodec: short effect row %v", r)
		}
		s.ActiveEffects = append(s.ActiveEffects, types.ActiveEffect{
			EffectID: str(r[0]), EffectIndex: toInt(r[1]), DurationRemaining: toFloat(r[2]), TotalDuration: toFloat(r[3]),
			DelayRemaining: toFloat(r[4]), TickAccumulator: toFloat(r[5]), AppliedAt: toInt(r[6]),
		})
	}
	for _, r := range w.QA {
		q := types.QuestProgress{QuestID: str(r[0]), Stage: toInt(r[1])}
		if len(r) > 2 {
			if counts, ok := r[2].([]any); ok {
				for _, c := range counts {
					q.ObjectiveCounts = append(q.ObjectiveCounts, toInt(c))
				}
			}
		}
		s.QuestsActive = append(s.QuestsActive, q)
	}
	for _, r := range w.PI {
		flags := toInt(r[3])
		s.POIStates = append(s.POIStates, types.POIState{
			POIID: str(r[0]), LastDay: toInt(r[1]), LastMinute: toInt(r[2]),
			Passed: flags&1 != 0, Looted: flags&2 != 0, Cleared: flags&4 != 0,
		})
	}
	for _, r := range w.RN {
		s.Rentals = append(s.Rentals, types.Rental{Building: str(r[0]), ExpiresDay: toInt(r[1]), ExpiresMin: toInt(r[2])})
	}
	for _, r := range w.VT {
		e := types.VaultEntry{ItemID: str(r[0]), Quantity: toInt(r[1])}
		if len(r) > 2 {
			e.State, _ = r[2].(map[string]any)
		}
		s.Vault = append(s.Vault, e)
	}
	return s, nil
}

// decodeInventory rebuilds the full inventory, empties included. Numbers come
// back as float64, the way the rest of the server reads JSON-loaded maps.
func decodeInventory(w *wire) map[string]any {
	inv := map[string]any{}
	gear := map[string]any{}
	for _, name := range standardGear {
		gear[name] = map[string]any{"item": nil, "quantity": float64(0)}
	}
	for name, t := range w.Gr {
		gear[name] = decodeSlot(t, -1)
	}
	inv["gear_slots"] = gear
	general := make([]any, w.Gn)
	for i := range general {
		general[i] = map[string]any{"item": nil, "quantity": float64(0), "slot": float64(i)}
	}
	for _, row := range w.Gs {
		idx := toInt(row[0])
		slot := decodeSlot(row[1:], idx)
		if idx >= 0 && idx < len(general) {
			general[idx] = slot
		} else {
			general = append(general, slot)
		}
	}
	inv["general_slots"] = general
	for k, v := range w.Ix {
		inv[k] = v
	}
	return inv
}

// decodeSlot rebuilds one slot from [item, qty, cap, contents, extras].
// slotIndex < 0 leaves "slot" off (gear slots don't carry one).
func decodeSlot(t []any, slotIndex int) map[string]any {
	slot := map[string]any{"item": nil, "quantity": float64(0)}
	if slotIndex >= 0 {
		slot["slot"] = float64(slotIndex)
	}
	if len(t) > 0 {
		if s := str(t[0]); s != "" {
			slot["item"] = s
		}
	}
	if len(t) > 1 {
		slot["quantity"] = float64(toInt(t[1]))
	}
	if len(t) > 3 {
		capacity := toInt(t[2])
		rows, _ := t[3].([]any)
		if capacity > 0 || rows != nil {
			contents := make([]any, capacity)
			for i := range contents {
				contents[i] = map[string]any{"item": nil, "quantity": float64(0), "slot": float64(i)}
			}
			for _, raw := range rows {
				r, _ := raw.([]any)
				if len(r) < 3 {
					continue
				}
				i := toInt(r[0])
				c := map[string]any{"item": str(r[1]), "quantity": float64(toInt(r[2])), "slot": float64(i)}
				if len(r) > 3 {
					if x, ok := r[3].(map[string]any); ok {
						for k, v := range x {
							c[k] = v
						}
					}
				}
				if i >= 0 && i < len(contents) {
					contents[i] = c
				} else {
					contents = append(contents, c)
				}
			}
			slot["contents"] = contents
		}
	}
	if len(t) > 4 {
		if x, ok := t[4].(map[string]any); ok {
			for k, v := range x {
				slot[k] = v
			}
		}
	}
	return slot
}

func decodeSpellSlots(ss map[string][][]any) map[string]any {
	out := map[string]any{}
	levels := make([]string, 0, len(ss))
	for l := range ss {
		levels = append(levels, l)
	}
	sort.Strings(levels)
	for _, level := range levels {
		var list []any
		for _, r := range ss[level] {
			e := map[string]any{"slot": float64(toInt(r[0])), "quantity": float64(toInt(r[2])), "spell": nil}
			if s := str(r[1]); s != "" {
				e["spell"] = s
			}
			list = append(list, e)
		}
		if list == nil {
			list = []any{}
		}
		out[level] = list
	}
	return out
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func isStandardGear(name string) bool {
	for _, g := range standardGear {
		if g == name {
			return true
		}
	}
	return false
}

func allZero(v []int) bool {
	for _, n := range v {
		if n != 0 {
			return false
		}
	}
	return true
}

func itemID(v any) string {
	s, _ := v.(string)
	return s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}
