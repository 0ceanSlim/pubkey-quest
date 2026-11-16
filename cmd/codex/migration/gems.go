package migration

import (
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
