package data

import (
	"encoding/json"
	"fmt"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// LoadFeats reads all selectable feats from the migrated feats table.
func LoadFeats() ([]types.Feat, error) {
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := database.Query("SELECT id, name, COALESCE(description, ''), COALESCE(properties, '') FROM feats ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query feats: %v", err)
	}
	defer rows.Close()

	var feats []types.Feat
	for rows.Next() {
		var id, name, description, props string
		if err := rows.Scan(&id, &name, &description, &props); err != nil {
			continue
		}
		feat := types.Feat{ID: id, Name: name, Description: description}
		if props != "" {
			var full types.Feat
			if json.Unmarshal([]byte(props), &full) == nil {
				feat.Prerequisite = full.Prerequisite
				feat.StatGrant = full.StatGrant
				feat.HPPerLevel = full.HPPerLevel
				feat.Effects = full.Effects
			}
		}
		feats = append(feats, feat)
	}
	return feats, nil
}

// LoadFeatByID returns a single feat by id, or an error if it doesn't exist.
func LoadFeatByID(id string) (*types.Feat, error) {
	feats, err := LoadFeats()
	if err != nil {
		return nil, err
	}
	for i := range feats {
		if feats[i].ID == id {
			return &feats[i], nil
		}
	}
	return nil, fmt.Errorf("unknown feat %q", id)
}
