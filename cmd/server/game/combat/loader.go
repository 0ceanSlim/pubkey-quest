package combat

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"pubkey-quest/types"
)

// LoadMonsterByID loads a full monster stat block from the database.
// The stats column stores the entire monster JSON (set by the migration tool).
func LoadMonsterByID(db *sql.DB, id string) (*types.MonsterData, error) {
	var statsJSON string
	err := db.QueryRow("SELECT stats FROM monsters WHERE id = ?", id).Scan(&statsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("monster not found: %s", id)
		}
		return nil, fmt.Errorf("failed to query monster %s: %v", id, err)
	}

	var monster types.MonsterData
	if err := json.Unmarshal([]byte(statsJSON), &monster); err != nil {
		return nil, fmt.Errorf("failed to parse monster %s: %v", id, err)
	}

	return &monster, nil
}
