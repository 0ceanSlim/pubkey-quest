package migration

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Scroll items (docs/draft/spell-scroll-system.md): one per spell up to the
// scribing table's max level, so every spell a caster knows can be scribed,
// looted and read. A hand-written game-data/items/scroll-of-<spell>.json wins;
// the rest are generated here, valued at their tier's scribing gold, with one
// sprite per tradition.

const scrollScribingPath = "game-data/magic/scroll-scribing.json"

type scribingTable struct {
	Tiers         map[string]struct{ Gold int `json:"gold"` } `json:"tiers"`
	MaxSpellLevel int                                         `json:"max_spell_level"`
	Traditions    map[string]struct {
		Name    string   `json:"name"`
		Station string   `json:"station"`
		Classes []string `json:"classes"`
		Image   string   `json:"image"`
	} `json:"traditions"`
}

// ScrollItemID is the item id of a spell's scroll.
func ScrollItemID(spellID string) string { return "scroll-of-" + spellID }

// generateScrollItems inserts the scroll items no hand-written file provides.
func generateScrollItems() error {
	raw, err := os.ReadFile(scrollScribingPath)
	if err != nil {
		return err
	}
	var tbl scribingTable
	if err := json.Unmarshal(raw, &tbl); err != nil {
		return fmt.Errorf("scroll-scribing.json: %v", err)
	}
	// Tradition order for the sprite: arcane, divine, nature (a spell on several
	// lists takes the first that has one of its classes).
	order := make([]string, 0, len(tbl.Traditions))
	for k := range tbl.Traditions {
		order = append(order, k)
	}
	sort.Strings(order)

	made := 0
	err = filepath.WalkDir("game-data/magic/spells", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var sp map[string]interface{}
		if json.Unmarshal(data, &sp) != nil {
			return nil
		}
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		level := 0
		if v, ok := sp["level"].(float64); ok {
			level = int(v)
		}
		if level > tbl.MaxSpellLevel {
			return nil
		}
		itemID := ScrollItemID(id)
		var exists int
		if database.QueryRow("SELECT COUNT(1) FROM items WHERE id = ?", itemID).Scan(&exists); exists > 0 {
			return nil // hand-written
		}
		name, _ := sp["name"].(string)
		if name == "" {
			name = id
		}
		image := ""
		classes, _ := sp["classes"].([]interface{})
		for _, t := range order {
			for _, c := range classes {
				cs, _ := c.(string)
				for _, tc := range tbl.Traditions[t].Classes {
					if strings.EqualFold(cs, tc) && image == "" {
						image = tbl.Traditions[t].Image
					}
				}
			}
		}
		mana := 0
		if v, ok := sp["mana_cost"].(float64); ok {
			mana = int(v)
		}
		kind := "cantrip"
		if level > 0 {
			kind = "level " + strconv.Itoa(level) + " spell"
		}
		item := map[string]interface{}{
			"id":          itemID,
			"name":        "Scroll of " + name,
			"description": fmt.Sprintf("A single-use scroll inscribed with the %s %s. Reading it casts the spell; it still costs mana and is consumed.", name, kind),
			"type":        "Spell Scroll",
			"spell_id":    id,
			"value":       tbl.Tiers[strconv.Itoa(level)].Gold,
			"rarity":      "common",
			"stack":       5,
			"weight":      0.1,
			"image":       image,
			"tags":        []string{"consumable", "spell-scroll"},
			"notes": []string{
				fmt.Sprintf("Read it to cast %s (%d mana); consumed on use. Any caster can read it.", name, mana),
				"Casters who know the spell can scribe more at a station of their tradition.",
			},
			"generated": true,
		}
		props, _ := json.Marshal(item)
		tags, _ := json.Marshal(item["tags"])
		if _, err := database.Exec(`INSERT INTO items (id, name, description, item_type, properties, tags, rarity) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			itemID, item["name"], item["description"], "Spell Scroll", string(props), string(tags), "common"); err != nil {
			return fmt.Errorf("insert %s: %v", itemID, err)
		}
		made++
		return nil
	})
	if err != nil {
		return err
	}
	log.Printf("Generated %d scroll items", made)
	return nil
}
