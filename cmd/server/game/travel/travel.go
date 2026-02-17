package travel

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// EnvironmentData holds parsed environment data from the database
type EnvironmentData struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	EnvironmentType  string   `json:"environment_type"`
	Connects         []string `json:"connects"`
	TravelTime       int      `json:"travel_time"` // Total minutes to traverse
	TravelDifficulty string   `json:"travel_difficulty"`
}

// TravelEndpoints holds parsed origin and destination info
type TravelEndpoints struct {
	OriginCity      string
	OriginDistrict  string
	DestCity        string
	DestDistrict    string
	OriginConnectID string // Full connect ID e.g. "kingdom-east"
	DestConnectID   string // Full connect ID e.g. "goldenhaven-west"
}

// TravelUpdate is returned by MaybeAdvanceTravelProgress when something notable happens
type TravelUpdate struct {
	Arrived         bool
	DestCity        string
	DestCityName    string
	DestDistrict    string
	NewlyDiscovered bool
	MusicUnlocked   []string
	TravelProgress  float64
}

// MusicConfig holds music track data for unlock checks
type MusicConfig struct {
	Tracks []MusicTrack `json:"tracks"`
}

// MusicTrack represents a single music track
type MusicTrack struct {
	Title      string  `json:"title"`
	File       string  `json:"file"`
	UnlocksAt  *string `json:"unlocks_at"`
	AutoUnlock bool    `json:"auto_unlock"`
}

// TravelConfig holds travel system configuration loaded from game-data/systems/travel-config.json
type TravelConfig struct {
	SkillScaling *types.SkillScaling `json:"skill_scaling"`
}

// Cached travel config (loaded once on first use)
var cachedTravelConfig *TravelConfig

// loadTravelConfig loads travel configuration from the systems DB table
func loadTravelConfig() *TravelConfig {
	if cachedTravelConfig != nil {
		return cachedTravelConfig
	}

	database := db.GetDB()
	if database == nil {
		return nil
	}

	var propsJSON string
	err := database.QueryRow("SELECT properties FROM systems WHERE id = 'travel-config'").Scan(&propsJSON)
	if err != nil {
		log.Printf("⚠️ No travel-config in systems table: %v", err)
		return nil
	}

	var config TravelConfig
	if err := json.Unmarshal([]byte(propsJSON), &config); err != nil {
		log.Printf("⚠️ Failed to parse travel config: %v", err)
		return nil
	}

	cachedTravelConfig = &config
	return cachedTravelConfig
}

// skillRatio holds a skill's stat weights (loaded from DB to avoid import cycle)
type skillRatio struct {
	Ratio map[string]float64 `json:"ratio"`
}

// Cached skill ratios for travel package
var cachedSkillRatios map[string]skillRatio

// getSkillRatio returns the stat ratio for a skill
func getSkillRatio(skillID string) (map[string]float64, bool) {
	if cachedSkillRatios == nil {
		database := db.GetDB()
		if database == nil {
			return nil, false
		}

		var propsJSON string
		err := database.QueryRow("SELECT properties FROM systems WHERE id = 'skills'").Scan(&propsJSON)
		if err != nil {
			return nil, false
		}

		var skills map[string]skillRatio
		if err := json.Unmarshal([]byte(propsJSON), &skills); err != nil {
			return nil, false
		}
		cachedSkillRatios = skills
	}

	skill, ok := cachedSkillRatios[skillID]
	if !ok {
		return nil, false
	}
	return skill.Ratio, true
}

// calculateSkillValue computes a skill value from character stats and ratios
func calculateSkillValue(stats map[string]interface{}, ratio map[string]float64) int {
	// Normalize stat keys to lowercase (save files use "Strength", ratios use "strength")
	normalizedStats := make(map[string]interface{}, len(stats))
	for k, v := range stats {
		normalizedStats[strings.ToLower(k)] = v
	}

	var value float64
	for stat, weight := range ratio {
		statVal := 10.0
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

// getTravelSpeedMultiplier calculates the travel speed multiplier based on Athletics skill
func getTravelSpeedMultiplier(stats map[string]interface{}) float64 {
	config := loadTravelConfig()
	if config == nil || config.SkillScaling == nil {
		return 1.0
	}

	scaling := config.SkillScaling
	ratio, ok := getSkillRatio(scaling.Skill)
	if !ok {
		return 1.0
	}

	skillValue := calculateSkillValue(stats, ratio)

	// No bonus at or below base level
	if skillValue <= scaling.BaseLevel {
		return 1.0
	}

	// Calculate bonus levels (capped)
	bonusLevels := skillValue - scaling.BaseLevel
	if bonusLevels > scaling.MaxBonusLevels {
		bonusLevels = scaling.MaxBonusLevels
	}

	// Higher skill = faster travel (multiplier > 1.0)
	return 1.0 + float64(bonusLevels)*scaling.BonusPerLevel
}

// GetEnvironmentData looks up location in DB and returns environment data if it's an environment
func GetEnvironmentData(locationID string) *EnvironmentData {
	database := db.GetDB()
	if database == nil {
		return nil
	}

	var locationType, propertiesJSON string
	err := database.QueryRow(
		"SELECT location_type, properties FROM locations WHERE id = ?",
		locationID,
	).Scan(&locationType, &propertiesJSON)

	if err != nil || locationType != "environment" {
		return nil
	}

	var env EnvironmentData
	if err := json.Unmarshal([]byte(propertiesJSON), &env); err != nil {
		log.Printf("❌ Failed to parse environment data for %s: %v", locationID, err)
		return nil
	}

	return &env
}

// GetTravelEndpoints parses connects[] to determine origin and destination
// originDistrict is the full district ID stored in state.District (e.g., "kingdom-east")
func GetTravelEndpoints(env *EnvironmentData, originDistrict string) (*TravelEndpoints, error) {
	if len(env.Connects) != 2 {
		return nil, fmt.Errorf("environment %s has %d connections, expected 2", env.ID, len(env.Connects))
	}

	var originIdx int
	if env.Connects[0] == originDistrict {
		originIdx = 0
	} else if env.Connects[1] == originDistrict {
		originIdx = 1
	} else {
		return nil, fmt.Errorf("origin district %s not found in connects %v", originDistrict, env.Connects)
	}

	destIdx := 1 - originIdx

	originCity, originDist := parseCityDistrict(env.Connects[originIdx])
	destCity, destDist := parseCityDistrict(env.Connects[destIdx])

	return &TravelEndpoints{
		OriginCity:      originCity,
		OriginDistrict:  originDist,
		DestCity:        destCity,
		DestDistrict:    destDist,
		OriginConnectID: env.Connects[originIdx],
		DestConnectID:   env.Connects[destIdx],
	}, nil
}

// parseCityDistrict splits "kingdom-east" into city="kingdom", district="east"
func parseCityDistrict(connectID string) (string, string) {
	lastHyphen := strings.LastIndex(connectID, "-")
	if lastHyphen == -1 {
		return connectID, "center"
	}
	return connectID[:lastHyphen], connectID[lastHyphen+1:]
}

// HandleStartTravel begins travel through an environment
func HandleStartTravel(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	envID, ok := params["environment_id"].(string)
	if !ok || envID == "" {
		return nil, fmt.Errorf("missing environment_id parameter")
	}

	// Validate player is in a city (not already traveling)
	if GetEnvironmentData(state.Location) != nil {
		return &types.GameActionResponse{
			Success: false,
			Message: "You are already traveling",
			Color:   "red",
		}, nil
	}

	// Get environment data
	env := GetEnvironmentData(envID)
	if env == nil {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	// Build origin district ID from current location
	originDistrict := state.Location + "-" + state.District

	// Validate this environment connects to player's current district
	if !slices.Contains(env.Connects, originDistrict) {
		return &types.GameActionResponse{
			Success: false,
			Message: "This environment is not accessible from your current location",
			Color:   "red",
		}, nil
	}

	// Get destination info for the response
	endpoints, err := GetTravelEndpoints(env, originDistrict)
	if err != nil {
		return nil, fmt.Errorf("failed to determine travel endpoints: %v", err)
	}

	// Look up destination city name
	destCityName := lookupCityName(endpoints.DestCity)

	// Set travel state
	state.Location = envID
	state.District = originDistrict // Store full origin district ID
	state.TravelProgress = 0.0
	state.TravelStopped = false
	state.Building = ""

	log.Printf("🚶 Started travel through %s: %s → %s (travel_time: %d min)",
		env.Name, endpoints.OriginCity, endpoints.DestCity, env.TravelTime)

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("You begin your journey through %s toward %s.", env.Name, destCityName),
		Color:   "yellow",
		Data: map[string]interface{}{
			"environment_name": env.Name,
			"description":      env.Description,
			"travel_time":      env.TravelTime,
			"dest_city":        endpoints.DestCity,
			"dest_city_name":   destCityName,
			"origin_city":      endpoints.OriginCity,
			"travel_progress":  0.0,
		},
	}, nil
}

// HandleTurnBack reverses travel direction
func HandleTurnBack(state *types.SaveFile, _ map[string]interface{}) (*types.GameActionResponse, error) {
	env := GetEnvironmentData(state.Location)
	if env == nil {
		return &types.GameActionResponse{
			Success: false,
			Message: "Not currently traveling",
		}, nil
	}

	// Swap direction: set district to the OTHER endpoint, invert progress
	endpoints, err := GetTravelEndpoints(env, state.District)
	if err != nil {
		return nil, fmt.Errorf("failed to determine travel endpoints: %v", err)
	}

	state.District = endpoints.DestConnectID // Now heading back to what was the destination
	state.TravelProgress = 1.0 - state.TravelProgress
	state.TravelStopped = false // Resume moving in new direction

	destCityName := lookupCityName(endpoints.DestCity)

	log.Printf("🔄 Travel reversed in %s, now heading toward %s (progress: %.1f%%)",
		env.Name, destCityName, state.TravelProgress*100)

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("You turn back toward %s.", destCityName),
		Color:   "yellow",
		Data: map[string]interface{}{
			"travel_progress": state.TravelProgress,
		},
	}, nil
}

// HandleStopTravel stops movement in an environment (time keeps flowing)
func HandleStopTravel(state *types.SaveFile, _ map[string]interface{}) (*types.GameActionResponse, error) {
	env := GetEnvironmentData(state.Location)
	if env == nil {
		return &types.GameActionResponse{
			Success: false,
			Message: "Not currently traveling",
		}, nil
	}

	state.TravelStopped = true

	log.Printf("⏸️ Travel stopped in %s (progress: %.1f%%)", env.Name, state.TravelProgress*100)

	return &types.GameActionResponse{
		Success: true,
		Message: "You stop to rest.",
		Color:   "yellow",
		Data: map[string]interface{}{
			"travel_progress": state.TravelProgress,
			"travel_stopped":  true,
		},
	}, nil
}

// HandleResumeTravel resumes movement in an environment
func HandleResumeTravel(state *types.SaveFile, _ map[string]interface{}) (*types.GameActionResponse, error) {
	env := GetEnvironmentData(state.Location)
	if env == nil {
		return &types.GameActionResponse{
			Success: false,
			Message: "Not currently traveling",
		}, nil
	}

	state.TravelStopped = false

	// Get destination name for message
	endpoints, err := GetTravelEndpoints(env, state.District)
	if err != nil {
		return nil, fmt.Errorf("failed to determine travel endpoints: %v", err)
	}
	destCityName := lookupCityName(endpoints.DestCity)

	log.Printf("▶️ Travel resumed in %s toward %s (progress: %.1f%%)",
		env.Name, destCityName, state.TravelProgress*100)

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("You continue toward %s.", destCityName),
		Color:   "yellow",
		Data: map[string]interface{}{
			"travel_progress": state.TravelProgress,
			"travel_stopped":  false,
		},
	}, nil
}

// MaybeAdvanceTravelProgress checks if the player is in an environment and advances
// travel progress proportionally to elapsed minutes. Called after any time advancement.
// Skips progress if player has stopped (TravelStopped=true) - time still flows but no movement.
// Returns a TravelUpdate if arrival occurred, nil otherwise.
func MaybeAdvanceTravelProgress(state *types.SaveFile, minutesElapsed int) *TravelUpdate {
	if minutesElapsed <= 0 {
		return nil
	}

	env := GetEnvironmentData(state.Location)
	if env == nil {
		return nil // Not in an environment
	}

	if env.TravelTime <= 0 {
		return nil
	}

	// If stopped, don't advance progress (but time still flows for hunger/fatigue)
	if state.TravelStopped {
		return &TravelUpdate{
			Arrived:        false,
			TravelProgress: state.TravelProgress,
		}
	}

	// Advance progress proportionally (Athletics skill scaling applied)
	progressIncrement := float64(minutesElapsed) / float64(env.TravelTime)
	progressIncrement *= getTravelSpeedMultiplier(state.Stats)
	state.TravelProgress += progressIncrement

	// Check for arrival
	if state.TravelProgress >= 1.0 {
		return processArrival(state, env)
	}

	return &TravelUpdate{
		Arrived:        false,
		TravelProgress: state.TravelProgress,
	}
}

// processArrival handles arriving at the destination city
func processArrival(state *types.SaveFile, env *EnvironmentData) *TravelUpdate {
	endpoints, err := GetTravelEndpoints(env, state.District)
	if err != nil {
		log.Printf("❌ Failed to determine destination on arrival: %v", err)
		return nil
	}

	destCity := endpoints.DestCity
	destDistrict := endpoints.DestDistrict
	destCityName := lookupCityName(destCity)

	// Update state to destination city
	state.Location = destCity
	state.District = destDistrict
	state.TravelProgress = 0
	state.TravelStopped = false
	state.Building = ""

	// Check if city is newly discovered
	newlyDiscovered := false
	if !slices.Contains(state.LocationsDiscovered, destCity) {
		state.LocationsDiscovered = append(state.LocationsDiscovered, destCity)
		newlyDiscovered = true
		log.Printf("🗺️ New location discovered: %s", destCityName)
	}

	// Check for music unlocks
	musicUnlocked := checkMusicUnlocks(state, destCity)

	log.Printf("🏙️ Arrived at %s (district: %s, discovered: %v, music: %v)",
		destCityName, destDistrict, newlyDiscovered, musicUnlocked)

	return &TravelUpdate{
		Arrived:         true,
		DestCity:        destCity,
		DestCityName:    destCityName,
		DestDistrict:    destDistrict,
		NewlyDiscovered: newlyDiscovered,
		MusicUnlocked:   musicUnlocked,
		TravelProgress:  0,
	}
}

// lookupCityName gets the display name for a city ID from the database
func lookupCityName(cityID string) string {
	database := db.GetDB()
	if database == nil {
		return cityID
	}
	var name string
	err := database.QueryRow("SELECT name FROM locations WHERE id = ?", cityID).Scan(&name)
	if err != nil {
		return cityID
	}
	return name
}

// checkMusicUnlocks checks music.json for tracks that unlock at this city
func checkMusicUnlocks(state *types.SaveFile, cityID string) []string {
	database := db.GetDB()
	if database == nil {
		return nil
	}

	var dataJSON string
	err := database.QueryRow("SELECT data FROM music_tracks WHERE id = 'music'").Scan(&dataJSON)
	if err != nil {
		return nil
	}

	var config MusicConfig
	if err := json.Unmarshal([]byte(dataJSON), &config); err != nil {
		return nil
	}

	var unlocked []string
	for _, track := range config.Tracks {
		if track.UnlocksAt != nil && *track.UnlocksAt == cityID {
			if !slices.Contains(state.MusicTracksUnlocked, track.Title) {
				state.MusicTracksUnlocked = append(state.MusicTracksUnlocked, track.Title)
				unlocked = append(unlocked, track.Title)
				log.Printf("🎵 Music track unlocked: %s", track.Title)
			}
		}
	}

	return unlocked
}

// IsInEnvironment checks if the player's current location is an environment
func IsInEnvironment(state *types.SaveFile) bool {
	return GetEnvironmentData(state.Location) != nil
}
