package wiki

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// Wiki-side views of the game-data JSON. They decode only what the pages show and
// stay lenient (any) where the data mixes numbers and strings.

type item struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Type            string          `json:"type"`
	Rarity          string          `json:"rarity"`
	Image           string          `json:"image"`
	Value           int             `json:"value"`
	Weight          float64         `json:"weight"`
	Stack           int64           `json:"stack"`
	Tags            []string        `json:"tags"`
	Notes           []string        `json:"notes"`
	GearSlot        string          `json:"gear_slot"`
	Damage          string          `json:"damage"`
	DamageType      string          `json:"damage_type"`
	Range           any             `json:"range"`
	RangeLong       any             `json:"range_long"`
	AC              any             `json:"ac"`
	Heal            string          `json:"heal"`
	Cures           []string        `json:"cures"`
	Set             string          `json:"set"`
	Ammunition      string          `json:"ammunition"`
	SpellID         string          `json:"spell_id"`
	Provides        string          `json:"provides"`
	ContainerSlots  int             `json:"container_slots"`
	AllowedTypes    []string        `json:"allowed_types"`
	SkillBonus      map[string]int  `json:"skill_bonus"`
	EffectsWhenWorn []string        `json:"effects_when_worn"`
	Contents        json.RawMessage `json:"contents"`
}

type packEntry struct {
	id  string
	qty int
}

// contents decodes a pack's [[id, qty], …] list.
func (it *item) contents() []packEntry {
	var raw [][]any
	if len(it.Contents) == 0 || json.Unmarshal(it.Contents, &raw) != nil {
		return nil
	}
	var out []packEntry
	for _, r := range raw {
		if len(r) == 0 {
			continue
		}
		id, _ := r[0].(string)
		qty := 1
		if len(r) > 1 {
			if f, ok := r[1].(float64); ok {
				qty = int(f)
			}
		}
		out = append(out, packEntry{id, qty})
	}
	return out
}

type monster struct {
	ID                    string         `json:"id"`
	Name                  string         `json:"name"`
	CR                    float64        `json:"challenge_rating"`
	XP                    int            `json:"xp"`
	Type                  string         `json:"type"`
	Size                  string         `json:"size"`
	AC                    int            `json:"armor_class"`
	HP                    int            `json:"hit_points"`
	HPDice                string         `json:"hp_dice"`
	Alignment             string         `json:"alignment"`
	Environment           []string       `json:"environment"`
	Speed                 map[string]int `json:"speed"`
	Stats                 map[string]int `json:"stats"`
	Saves                 map[string]int `json:"saving_throws"`
	Skills                map[string]int `json:"skills"`
	Senses                map[string]int `json:"senses"`
	DamageResistances     []string       `json:"damage_resistances"`
	DamageImmunities      []string       `json:"damage_immunities"`
	DamageVulnerabilities []string       `json:"damage_vulnerabilities"`
	ConditionImmunities   []string       `json:"condition_immunities"`
	Actions               []action       `json:"actions"`
	BonusActions          []action       `json:"bonus_actions"`
	Reactions             []action       `json:"reactions"`
	LegendaryActions      []action       `json:"legendary_actions"`
	Special               []action       `json:"special_abilities"`
	Loot                  *lootTable     `json:"loot_table"`
}

type action struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	AttackBonus *int   `json:"attack_bonus"`
	Attacks     int    `json:"attacks"`
	Reach       *int   `json:"reach"`
	Range       *int   `json:"range"`
	RangeLong   *int   `json:"range_long"`
	Recharge    string `json:"recharge"`
	Hit         *struct {
		Dice    string `json:"dice"`
		Mod     int    `json:"mod"`
		Type    string `json:"type"`
		Special any    `json:"special"` // prose, or {ability, dc, effect} for a save rider
	} `json:"hit"`
}

type lootTable struct {
	Guaranteed []lootEntry `json:"guaranteed"`
	Rolls      int         `json:"rolls"`
	Tiers      []struct {
		Name    string      `json:"name"`
		Weight  int         `json:"weight"`
		Entries []lootEntry `json:"entries"`
	} `json:"tiers"`
}

type lootEntry struct {
	Item     string `json:"item"`
	Weight   int    `json:"weight"`
	Quantity []int  `json:"quantity"`
}

func (m *monster) lootItems() []string {
	if m.Loot == nil {
		return nil
	}
	var ids []string
	for _, e := range m.Loot.Guaranteed {
		ids = append(ids, e.Item)
	}
	for _, t := range m.Loot.Tiers {
		for _, e := range t.Entries {
			if e.Item != "nothing" {
				ids = append(ids, e.Item)
			}
		}
	}
	return ids
}

type spell struct {
	ID            string   `json:"-"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Level         int      `json:"level"`
	School        string   `json:"school"`
	CastingTime   string   `json:"casting_time"`
	Range         any      `json:"range"`
	RangeLong     any      `json:"range_long"`
	Duration      string   `json:"duration"`
	Concentration bool     `json:"concentration"`
	Damage        any      `json:"damage"`
	DamageType    any      `json:"damage_type"`
	Heal          any      `json:"heal"`
	SpellAttack   any      `json:"spell_attack"`
	SaveType      any      `json:"save_type"`
	Classes       []string `json:"classes"`
	ManaCost      int      `json:"mana_cost"`
	PrepTime      int      `json:"prep_time"`
	Notes         []string `json:"notes"`
	Material      *struct {
		Required []struct {
			Component string `json:"component"`
			Quantity  int    `json:"quantity"`
		} `json:"required"`
		FocusProvided string `json:"focus_provided"`
	} `json:"material_component"`
}

// Spell JSON carries no id — the filename is the id.
func (s *spell) setFile(path string) {
	s.ID = strings.TrimSuffix(filepath.Base(path), ".json")
}

type city struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        string              `json:"type"`
	Image       string              `json:"image"`
	Description string              `json:"description"`
	Races       []string            `json:"starting_races"`
	EntryFees   map[string]int      `json:"racial_entry_fees"`
	Districts   map[string]district `json:"districts"`
}

type district struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Connections map[string]string `json:"connections"`
	Buildings   []building        `json:"buildings"`
}

type building struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Open        any    `json:"open"`
	Close       any    `json:"close"`
}

func (b building) private() bool {
	f, ok := b.Open.(float64)
	return ok && f < 0
}

func (b building) hours() string {
	switch o := b.Open.(type) {
	case string:
		if o == "always" {
			return "Always open"
		}
		return titleCase(o)
	case float64:
		if o < 0 {
			return "Private"
		}
		if c, ok := b.Close.(float64); ok {
			return clock(int(o)) + "–" + clock(int(c))
		}
		return "Always open"
	}
	return ""
}

type environment struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	EnvType     string   `json:"environment_type"`
	Description string   `json:"description"`
	Connects    []string `json:"connects"`
	TravelTime  int      `json:"travel_time"`
	Difficulty  string   `json:"travel_difficulty"`
}

type npc struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Race        string `json:"race"`
	Description string `json:"description"`
	PrimaryHome string `json:"primary_home"`
	Schedule    []struct {
		Start    int    `json:"start"`
		End      int    `json:"end"`
		Location string `json:"location"`
		State    string `json:"state"`
	} `json:"schedule"`
	Shop *struct {
		Type      string `json:"shop_type"`
		BuysItems bool   `json:"buys_items"`
		Inventory []struct {
			ItemID   string `json:"item_id"`
			MaxStock int    `json:"max_stock"`
		} `json:"inventory"`
	} `json:"shop_config"`
}

type effect struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Removal     struct {
		Type  string `json:"type"`
		Timer int    `json:"timer"`
	} `json:"removal"`
	Modifiers []struct {
		Stat         string `json:"stat"`
		Value        int    `json:"value"`
		Type         string `json:"type"`
		TickInterval int    `json:"tick_interval"`
	} `json:"modifiers"`
	Visible bool `json:"visible"`
}

type feat struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Prerequisite string   `json:"prerequisite"`
	Effects      []string `json:"effects"`
}

type ability struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Class        string `json:"class"`
	UnlockLevel  int    `json:"unlock_level"`
	ResourceCost int    `json:"resource_cost"`
	ResourceType string `json:"resource_type"`
	Cooldown     string `json:"cooldown"`
	Description  string `json:"description"`
	Tiers        []struct {
		MinLevel int    `json:"min_level"`
		MaxLevel int    `json:"max_level"`
		Summary  string `json:"summary"`
	} `json:"scaling_tiers"`
}
