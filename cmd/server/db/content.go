package db

import (
	"fmt"

	"pubkey-quest/types"
)

// Narrative-content read layer (M3): quests, POIs, and encounters. The full
// definition is stored as JSON in each table's properties column and
// unmarshalled into the canonical types here, so callers (the quest engine,
// POI walker, encounter scheduler) work with real structs rather than maps.

// GetQuestByID loads one quest's full definition.
func GetQuestByID(id string) (*types.QuestData, error) {
	var propertiesJSON string
	err := db.QueryRow(`SELECT properties FROM quests WHERE id = ?`, id).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("quest not found: %s", id)
	}
	var quest types.QuestData
	if err := parseJSON(propertiesJSON, &quest); err != nil {
		return nil, fmt.Errorf("failed to parse quest %s: %v", id, err)
	}
	return &quest, nil
}

// GetAllQuests loads every quest definition — for availability scans and the log.
func GetAllQuests() ([]types.QuestData, error) {
	return queryQuests(`SELECT properties FROM quests ORDER BY id`)
}

// GetQuestsByCategory loads quests in one category (e.g. the daily/weekly pools).
func GetQuestsByCategory(category string) ([]types.QuestData, error) {
	return queryQuests(`SELECT properties FROM quests WHERE category = ? ORDER BY id`, category)
}

func queryQuests(query string, args ...interface{}) ([]types.QuestData, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query quests: %v", err)
	}
	defer rows.Close()

	var quests []types.QuestData
	for rows.Next() {
		var propertiesJSON string
		if err := rows.Scan(&propertiesJSON); err != nil {
			continue
		}
		var q types.QuestData
		if err := parseJSON(propertiesJSON, &q); err != nil {
			continue
		}
		quests = append(quests, q)
	}
	return quests, nil
}

// GetPOIByID loads one POI's full node graph.
func GetPOIByID(id string) (*types.POIData, error) {
	var propertiesJSON string
	err := db.QueryRow(`SELECT properties FROM pois WHERE id = ?`, id).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("POI not found: %s", id)
	}
	var poi types.POIData
	if err := parseJSON(propertiesJSON, &poi); err != nil {
		return nil, fmt.Errorf("failed to parse POI %s: %v", id, err)
	}
	return &poi, nil
}

// GetPOIsByEnvironment loads the POIs that can be discovered while travelling a
// given environment — the discovery roll's candidate set.
func GetPOIsByEnvironment(environment string) ([]types.POIData, error) {
	rows, err := db.Query(`SELECT properties FROM pois WHERE parent_environment = ? ORDER BY position`, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to query POIs: %v", err)
	}
	defer rows.Close()

	var pois []types.POIData
	for rows.Next() {
		var propertiesJSON string
		if err := rows.Scan(&propertiesJSON); err != nil {
			continue
		}
		var p types.POIData
		if err := parseJSON(propertiesJSON, &p); err != nil {
			continue
		}
		pois = append(pois, p)
	}
	return pois, nil
}

// GetEncounterByID loads one encounter's full node graph.
func GetEncounterByID(id string) (*types.EncounterData, error) {
	var propertiesJSON string
	err := db.QueryRow(`SELECT properties FROM encounters WHERE id = ?`, id).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("encounter not found: %s", id)
	}
	var enc types.EncounterData
	if err := parseJSON(propertiesJSON, &enc); err != nil {
		return nil, fmt.Errorf("failed to parse encounter %s: %v", id, err)
	}
	return &enc, nil
}

// GetEncountersByTrigger loads the encounters that can fire in a given trigger
// context — the scheduler's candidate set before requirement/cooldown/chance
// filtering.
func GetEncountersByTrigger(trigger string) ([]types.EncounterData, error) {
	rows, err := db.Query(`SELECT properties FROM encounters WHERE trigger = ? ORDER BY id`, trigger)
	if err != nil {
		return nil, fmt.Errorf("failed to query encounters: %v", err)
	}
	defer rows.Close()

	var encounters []types.EncounterData
	for rows.Next() {
		var propertiesJSON string
		if err := rows.Scan(&propertiesJSON); err != nil {
			continue
		}
		var e types.EncounterData
		if err := parseJSON(propertiesJSON, &e); err != nil {
			continue
		}
		encounters = append(encounters, e)
	}
	return encounters, nil
}
