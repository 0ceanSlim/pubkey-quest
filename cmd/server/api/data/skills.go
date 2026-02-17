package data

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"

	"pubkey-quest/cmd/server/db"
)

// SkillDefinition represents a skill's configuration from game-data
type SkillDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Ratio       map[string]float64 `json:"ratio"`
}

// SkillResponse includes the definition plus a calculated value
type SkillResponse struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Ratio       map[string]float64 `json:"ratio"`
	Value       int                `json:"value"`
}

// LoadSkillDefinitions reads skill definitions from the systems table
func LoadSkillDefinitions() (map[string]SkillDefinition, error) {
	database := db.GetDB()
	if database == nil {
		return nil, nil
	}

	var propsJSON string
	err := database.QueryRow("SELECT properties FROM systems WHERE id = 'skills'").Scan(&propsJSON)
	if err != nil {
		return nil, err
	}

	var skills map[string]SkillDefinition
	if err := json.Unmarshal([]byte(propsJSON), &skills); err != nil {
		return nil, err
	}

	return skills, nil
}

// CalculateSkillValue computes a skill value from character stats and ratios.
// Handles case-insensitive stat lookups since save files use "Strength" but
// skills.json uses "strength".
func CalculateSkillValue(stats map[string]interface{}, ratio map[string]float64) int {
	// Build case-insensitive lookup from stats map
	normalizedStats := make(map[string]interface{}, len(stats))
	for k, v := range stats {
		normalizedStats[strings.ToLower(k)] = v
	}

	var value float64
	for stat, weight := range ratio {
		statVal := 10.0 // default
		if v, ok := normalizedStats[strings.ToLower(stat)]; ok {
			switch n := v.(type) {
			case float64:
				statVal = n
			case int:
				statVal = float64(n)
			case json.Number:
				if f, err := n.Float64(); err == nil {
					statVal = f
				}
			}
		}
		value += statVal * weight
	}
	return int(math.Round(value))
}

// SkillsDefinitionsHandler returns skill definitions without character context
// GET /api/skills/definitions
func SkillsDefinitionsHandler(w http.ResponseWriter, r *http.Request) {
	skills, err := LoadSkillDefinitions()
	if err != nil {
		log.Printf("❌ Failed to load skill definitions: %v", err)
		http.Error(w, "Failed to load skill definitions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": skills,
	})
}
