package savecodec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// ParseAny reads a save from whatever the caller has: a compact save, a
// signed Nostr save event (its content), or a server disk save file. It
// returns the save and the compact bytes it corresponds to.
func ParseAny(data []byte) (*types.SaveFile, []byte, error) {
	var probe struct {
		V       *int   `json:"v"`
		Kind    *int   `json:"kind"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, nil, fmt.Errorf("not JSON: %w", err)
	}
	switch {
	case probe.Kind != nil:
		s, err := Decode([]byte(probe.Content))
		return s, []byte(probe.Content), err
	case probe.V != nil:
		s, err := Decode(data)
		return s, data, err
	}
	var disk types.SaveFile
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, nil, err
	}
	compact, err := Encode(&disk)
	if err != nil {
		return nil, nil, err
	}
	s, err := Decode(compact)
	return s, compact, err
}

// Describe renders a save as a readable character sheet — the "encode and
// explain are two halves of one feature" rule (roadmap §4). level turns XP
// into a level (from the advancement table); nil shows XP only.
func Describe(s *types.SaveFile, level func(xp int) int) string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	lv := ""
	if level != nil {
		lv = fmt.Sprintf("Level %d ", level(s.Experience))
	}
	line("%s — %s%s %s (%s, %s)", s.D, lv, s.Race, s.Class, s.Background, s.Alignment)
	line("  XP %d · HP %d · mana %d · fatigue %d · hunger %d", s.Experience, s.HP, s.Mana, s.Fatigue, s.Hunger)
	var stats []string
	for _, a := range abilityOrder {
		stats = append(stats, fmt.Sprintf("%s %d", a[:3], toInt(s.Stats[a])))
	}
	line("  %s", strings.Join(stats, " · "))
	where := []string{s.Location}
	for _, p := range []string{s.District, s.Building, s.Room} {
		if p != "" {
			where = append(where, p)
		}
	}
	line("  At %s, day %d %02d:%02d%s", strings.Join(where, " / "), s.CurrentDay, s.TimeOfDay/60, s.TimeOfDay%60,
		map[bool]string{true: fmt.Sprintf(" — %.0f%% along the road", s.TravelProgress*100), false: ""}[s.TravelProgress > 0])

	gold := 0
	var gear, carried []string
	walk := func(place string, m map[string]any, into *[]string) {
		id, _ := m["item"].(string)
		if id == "gold-piece" {
			gold += toInt(m["quantity"])
		} else if id != "" {
			q := toInt(m["quantity"])
			label := id
			if q > 1 {
				label = fmt.Sprintf("%s ×%d", id, q)
			}
			*into = append(*into, place+label)
		}
		if contents, ok := m["contents"].([]any); ok {
			for _, raw := range contents {
				c, _ := raw.(map[string]any)
				cid, _ := c["item"].(string)
				if cid == "gold-piece" {
					gold += toInt(c["quantity"])
				} else if cid != "" {
					*into = append(*into, fmt.Sprintf("  in %s: %s ×%d", id, cid, toInt(c["quantity"])))
				}
			}
		}
	}
	if g, ok := s.Inventory["gear_slots"].(map[string]any); ok {
		names := make([]string, 0, len(g))
		for n := range g {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			m, _ := g[n].(map[string]any)
			walk(n+": ", m, &gear)
		}
	}
	if list, ok := s.Inventory["general_slots"].([]any); ok {
		for _, raw := range list {
			m, _ := raw.(map[string]any)
			walk("", m, &carried)
		}
	}
	line("  Gold %d", gold)
	if len(gear) > 0 {
		line("  Worn / held:")
		for _, g := range gear {
			line("    %s", g)
		}
	}
	if len(carried) > 0 {
		line("  Carried:")
		for _, c := range carried {
			line("    %s", c)
		}
	}
	if len(s.KnownSpells) > 0 {
		line("  Spells known: %s", strings.Join(s.KnownSpells, ", "))
	}
	for _, q := range s.QuestsActive {
		line("  Quest: %s (stage %d) %v", q.QuestID, q.Stage, q.ObjectiveCounts)
	}
	if len(s.QuestsCompleted) > 0 {
		line("  Quests done: %s", strings.Join(s.QuestsCompleted, ", "))
	}
	if len(s.LocationsDiscovered) > 0 {
		line("  Discovered: %s", strings.Join(s.LocationsDiscovered, ", "))
	}
	if len(s.DialogueGifts) > 0 {
		line("  Gifts received: %s", strings.Join(s.DialogueGifts, ", "))
	}
	if len(s.Vault) > 0 {
		var v []string
		for _, e := range s.Vault {
			v = append(v, fmt.Sprintf("%s ×%d", e.ItemID, e.Quantity))
		}
		line("  Vault: %s", strings.Join(v, ", "))
	}
	for _, e := range s.ActiveEffects {
		line("  Effect: %s (%.0f min left)", e.EffectID, e.DurationRemaining)
	}
	return b.String()
}
