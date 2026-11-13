package migration

import (
	"encoding/json"
	"fmt"
	"log"

	"pubkey-quest/cmd/server/game/fishing"
)

// Fish items (docs/draft/fishing-system.md §2): fishing.json's species × sizes
// become fish-<species>-<size>, with cooked- and salted-fish-<size> and a
// trophy-<species> each. A hand-written game-data/items file of the same id wins.

const fishingPath = "game-data/systems/fishing.json"

// generateFishItems inserts the fish items no hand-written file provides.
func generateFishItems() error {
	cfg, err := fishing.LoadFile(fishingPath)
	if err != nil {
		return err
	}
	made := 0
	for _, item := range fishing.Items(cfg) {
		id, _ := item["id"].(string)
		var exists int
		if database.QueryRow("SELECT COUNT(1) FROM items WHERE id = ?", id).Scan(&exists); exists > 0 {
			continue // hand-written
		}
		props, _ := json.Marshal(item)
		tags, _ := json.Marshal(item["tags"])
		if _, err := database.Exec(`INSERT INTO items (id, name, description, item_type, properties, tags, rarity) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, item["name"], item["description"], item["type"], string(props), string(tags), item["rarity"]); err != nil {
			return fmt.Errorf("insert %s: %v", id, err)
		}
		made++
	}
	log.Printf("Generated %d fish items", made)
	return nil
}
