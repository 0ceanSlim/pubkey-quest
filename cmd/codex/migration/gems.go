package migration

import (
	"encoding/json"
	"fmt"
	"log"

	"pubkey-quest/cmd/server/game/gems"
)

// Gem items (docs/draft/gem-system.md §7): gems.json's types × sizes become
// rough gems, × qualities cut gems, and each cut gem a ring and a necklace. A
// hand-written game-data/items file of the same id wins.

const gemsPath = "game-data/systems/gems.json"

// generateGemItems inserts the gem items no hand-written file provides.
func generateGemItems() error {
	cfg, err := gems.LoadFile(gemsPath)
	if err != nil {
		return err
	}
	made, err := insertGeneratedItems(gems.Items(cfg))
	if err != nil {
		return err
	}
	log.Printf("Generated %d gem items", made)
	return nil
}

// generateGemEffects inserts the enchanted pieces' worn effects
// (docs/draft/enchanting.md). A hand-written game-data/effects file of the
// same id wins.
func generateGemEffects() error {
	cfg, err := gems.LoadFile(gemsPath)
	if err != nil {
		return err
	}
	made := 0
	for _, def := range gems.Effects(cfg) {
		id, _ := def["id"].(string)
		var exists int
		if database.QueryRow("SELECT COUNT(1) FROM effects WHERE id = ?", id).Scan(&exists); exists > 0 {
			continue // hand-written
		}
		props, _ := json.Marshal(def)
		if _, err := database.Exec(`INSERT INTO effects (id, name, description, source_type, properties) VALUES (?, ?, ?, ?, ?)`,
			id, def["name"], def["description"], def["source_type"], string(props)); err != nil {
			return fmt.Errorf("insert %s: %v", id, err)
		}
		made++
	}
	log.Printf("Generated %d gem effects", made)
	return nil
}
