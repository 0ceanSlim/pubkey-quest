package db

import "fmt"

// MonsterRef is a lightweight monster reference for biome encounter rolling —
// just what the roller needs to pick a fight and hand the id to StartCombat.
type MonsterRef struct {
	ID   string
	Name string
	CR   float64
}

// GetMonstersByBiome returns the monsters whose environment tags include the
// given biome (an environment_type like "forest" or "arctic"). The biome list
// lives inside each monster's JSON rather than a column, so this scans the
// table and filters in Go — the monster set is small.
func GetMonstersByBiome(biome string) ([]MonsterRef, error) {
	if biome == "" {
		return nil, nil
	}
	rows, err := db.Query(`SELECT id, name, challenge_rating, stats FROM monsters`)
	if err != nil {
		return nil, fmt.Errorf("failed to query monsters: %v", err)
	}
	defer rows.Close()

	var refs []MonsterRef
	for rows.Next() {
		var id, name, statsJSON string
		var cr float64
		if err := rows.Scan(&id, &name, &cr, &statsJSON); err != nil {
			continue
		}
		var m struct {
			Environment []string `json:"environment"`
		}
		if err := parseJSON(statsJSON, &m); err != nil {
			continue
		}
		for _, e := range m.Environment {
			if e == biome {
				refs = append(refs, MonsterRef{ID: id, Name: name, CR: cr})
				break
			}
		}
	}
	return refs, nil
}

// GetEnvironmentType returns an environment's biome (its environment_type),
// e.g. "forest" for darkwood-forest — the key used to pick the travel-encounter
// monster pool.
func GetEnvironmentType(envID string) (string, error) {
	var propsJSON string
	err := db.QueryRow(`SELECT properties FROM locations WHERE id = ?`, envID).Scan(&propsJSON)
	if err != nil {
		return "", fmt.Errorf("environment not found: %s", envID)
	}
	var props struct {
		EnvironmentType string `json:"environment_type"`
	}
	if err := parseJSON(propsJSON, &props); err != nil {
		return "", fmt.Errorf("failed to parse environment %s: %v", envID, err)
	}
	return props.EnvironmentType, nil
}
