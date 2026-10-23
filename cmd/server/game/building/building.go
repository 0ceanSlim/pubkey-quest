package building

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// BuildingHours represents open/close times for a building
type BuildingHours struct {
	Open  interface{} `json:"open"`  // int (minutes 0-1439) or "always"
	Close interface{} `json:"close"` // int (minutes 0-1439) or null
}

// IsBuildingOpen checks if a building is accessible at the given time
// Returns: (isOpen bool, openMinutes int, closeMinutes int, err)
func IsBuildingOpen(db *sql.DB, locationID, buildingID string, timeOfDay int) (bool, int, int, error) {
	// Get location data from database
	var propertiesJSON string
	err := db.QueryRow("SELECT properties FROM locations WHERE id = ?", locationID).Scan(&propertiesJSON)
	if err != nil {
		return false, 0, 0, fmt.Errorf("location not found: %s", locationID)
	}

	var locationData map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &locationData); err != nil {
		return false, 0, 0, fmt.Errorf("failed to parse location data: %v", err)
	}

	// Navigate to building in districts
	districts, ok := locationData["districts"].(map[string]interface{})
	if !ok {
		return false, 0, 0, fmt.Errorf("location has no districts")
	}

	// Search all districts for the building
	for _, districtData := range districts {
		district, ok := districtData.(map[string]interface{})
		if !ok {
			continue
		}

		buildings, ok := district["buildings"].([]interface{})
		if !ok {
			continue
		}

		for _, buildingData := range buildings {
			building, ok := buildingData.(map[string]interface{})
			if !ok {
				continue
			}

			if building["id"] == buildingID {
				// Found the building, check hours
				hours := BuildingHours{
					Open:  building["open"],
					Close: building["close"],
				}

				// Check if always open
				if openStr, ok := hours.Open.(string); ok && openStr == "always" {
					return true, 0, 1440, nil
				}

				// Check if private/never accessible (open: -1)
				if openFloat, ok := hours.Open.(float64); ok && openFloat < 0 {
					return false, -1, -1, nil
				}

				// Get numeric minutes (0-1439)
				openMinutes, ok := hours.Open.(float64)
				if !ok {
					return false, 0, 0, fmt.Errorf("invalid open time format")
				}

				closeMinutes := 1440.0 // Default to end of day
				if hours.Close != nil {
					closeMinutes, ok = hours.Close.(float64)
					if !ok {
						return false, 0, 0, fmt.Errorf("invalid close time format")
					}
				}

				// Current time is already in minutes (0-1439)
				currentTimeMinutes := timeOfDay

				// Check if building is open
				open := int(openMinutes)
				close := int(closeMinutes)

				// Handle wrapping (e.g., 1020 minutes [5 PM] to 480 minutes [8 AM] for overnight)
				if close < open {
					// Overnight hours - open if current time >= open OR current time < close
					if currentTimeMinutes >= open || currentTimeMinutes < close {
						return true, open, close, nil
					}
				} else {
					// Normal hours - open if current time >= open AND current time < close
					if currentTimeMinutes >= open && currentTimeMinutes < close {
						return true, open, close, nil
					}
				}

				return false, open, close, nil
			}
		}
	}

	return false, 0, 0, fmt.Errorf("building not found: %s", buildingID)
}

// districtBuildings returns the raw building entries of one district of a
// location (nil when the location has no such district or no buildings).
func districtBuildings(db *sql.DB, locationID, districtID string) ([]interface{}, error) {
	// Get location data from database
	var propertiesJSON string
	err := db.QueryRow("SELECT properties FROM locations WHERE id = ?", locationID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("location not found: %s", locationID)
	}

	var locationData map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &locationData); err != nil {
		return nil, fmt.Errorf("failed to parse location data: %v", err)
	}

	// Navigate to the specific district
	districts, ok := locationData["districts"].(map[string]interface{})
	if !ok {
		return nil, nil // No districts
	}

	// Extract district key from districtID (e.g., "kingdom-center" -> "center")
	districtKey := districtID
	if idx := len(locationID) + 1; idx < len(districtID) {
		districtKey = districtID[idx:]
	}

	districtData, ok := districts[districtKey].(map[string]interface{})
	if !ok {
		return nil, nil // District not found
	}

	buildings, _ := districtData["buildings"].([]interface{})
	return buildings, nil
}

// DistrictBuildingNames returns map[buildingID]name for one district.
func DistrictBuildingNames(db *sql.DB, locationID, districtID string) (map[string]string, error) {
	buildings, err := districtBuildings(db, locationID, districtID)
	names := make(map[string]string, len(buildings))
	for _, b := range buildings {
		if m, ok := b.(map[string]interface{}); ok {
			id, _ := m["id"].(string)
			name, _ := m["name"].(string)
			if name == "" {
				name = id
			}
			names[id] = name
		}
	}
	return names, err
}

// GetAllBuildingStatesForDistrict returns the open/closed state of all buildings in a district
// Returns map[buildingID]isOpen
func GetAllBuildingStatesForDistrict(db *sql.DB, locationID, districtID string, timeOfDay int) (map[string]bool, error) {
	result := make(map[string]bool)

	buildings, err := districtBuildings(db, locationID, districtID)
	if err != nil {
		return result, err
	}

	// Check each building
	for _, buildingData := range buildings {
		building, ok := buildingData.(map[string]interface{})
		if !ok {
			continue
		}

		buildingID, ok := building["id"].(string)
		if !ok {
			continue
		}

		hours := BuildingHours{
			Open:  building["open"],
			Close: building["close"],
		}

		// Check if always open
		if openStr, ok := hours.Open.(string); ok && openStr == "always" {
			result[buildingID] = true
			continue
		}

		// Check if private/never accessible (open: -1)
		if openFloat, ok := hours.Open.(float64); ok && openFloat < 0 {
			result[buildingID] = false
			continue
		}

		// Get numeric minutes (0-1439)
		openMinutes, ok := hours.Open.(float64)
		if !ok {
			result[buildingID] = false
			continue
		}

		closeMinutes := 1440.0
		if hours.Close != nil {
			closeMinutes, _ = hours.Close.(float64)
		}

		open := int(openMinutes)
		close := int(closeMinutes)

		// Handle wrapping (overnight hours)
		if close < open {
			result[buildingID] = timeOfDay >= open || timeOfDay < close
		} else {
			result[buildingID] = timeOfDay >= open && timeOfDay < close
		}
	}

	return result, nil
}
