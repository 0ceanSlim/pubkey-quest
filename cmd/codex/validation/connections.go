package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// World connectivity validation: every city district exit and every environment
// `connects` endpoint must agree — no dangling targets, no one-way links, no
// district claimed by two environments. This catches exactly the class of bug
// where an environment claims districts that point somewhere else (or don't
// exist), e.g. a "crossing" environment wired to the wrong/missing districts.

const (
	citiesGlob   = "game-data/locations/cities/*.json"
	envsGlob     = "game-data/locations/environments/*.json"
	monstersGlob = "game-data/monsters/*.json"
)

// indoorTags are monster habitats handled by the POI/dungeon system rather than
// travel environments, so they don't need a matching environment_type.
var indoorTags = map[string]bool{"cave": true, "dungeon": true, "graveyard": true}

// ConnectionResult holds the outcome of a connectivity check.
type ConnectionResult struct {
	Errors        []string
	Warnings      []string
	Districts     int
	Environments  int
}

type connCityFile struct {
	ID        string                  `json:"id"`
	Districts map[string]connDistrict `json:"districts"`
}

type connDistrict struct {
	ID          string            `json:"id"`
	Connections map[string]string `json:"connections"`
	ExitToEnv   string            `json:"exit_to_environment"`
}

type connEnvFile struct {
	ID       string   `json:"id"`
	EnvType  string   `json:"environment_type"`
	Connects []string `json:"connects"`
}

// ValidateConnections cross-checks city district connections against environment
// `connects` endpoints and reports inconsistencies.
func ValidateConnections() (ConnectionResult, error) {
	var res ConnectionResult

	cityPaths, _ := filepath.Glob(citiesGlob)
	envPaths, _ := filepath.Glob(envsGlob)

	districtIDs := map[string]bool{}        // every district id that exists
	districtExits := map[string][]string{}  // districtID -> connection target values
	districtExitEnv := map[string]string{}  // districtID -> exit_to_environment (if set)
	envIDs := map[string]bool{}
	envConnects := map[string][]string{} // envID -> endpoint district ids
	envTypes := map[string]bool{}        // distinct environment_type values

	for _, p := range cityPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			return res, err
		}
		var c connCityFile
		if err := json.Unmarshal(data, &c); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: invalid JSON: %v", filepath.Base(p), err))
			continue
		}
		for _, d := range c.Districts {
			if d.ID == "" {
				continue
			}
			districtIDs[d.ID] = true
			for _, target := range d.Connections {
				districtExits[d.ID] = append(districtExits[d.ID], target)
			}
			if d.ExitToEnv != "" {
				districtExitEnv[d.ID] = d.ExitToEnv
			}
		}
	}
	for _, p := range envPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			return res, err
		}
		var e connEnvFile
		if err := json.Unmarshal(data, &e); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: invalid JSON: %v", filepath.Base(p), err))
			continue
		}
		envIDs[e.ID] = true
		envConnects[e.ID] = e.Connects
		if e.EnvType != "" {
			envTypes[e.EnvType] = true
		}
	}
	res.Districts = len(districtIDs)
	res.Environments = len(envIDs)

	// 1. Dangling district connection targets (point at no known district or environment).
	for dID, targets := range districtExits {
		for _, t := range targets {
			if !districtIDs[t] && !envIDs[t] {
				res.Errors = append(res.Errors, fmt.Sprintf("district %q connects to %q — not a known district or environment (dangling)", dID, t))
			}
		}
	}

	// 2. Environment endpoints + bidirectionality + double-booking.
	endpointOwner := map[string]string{} // districtID -> envID that claims it
	for envID, connects := range envConnects {
		if len(connects) != 2 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("environment %q has %d connect endpoints (expected 2)", envID, len(connects)))
		}
		for _, dID := range connects {
			if !districtIDs[dID] {
				res.Errors = append(res.Errors, fmt.Sprintf("environment %q connects to %q — not a known district", envID, dID))
				continue
			}
			// The endpoint district must actually connect back to this environment.
			exitsBack := false
			for _, t := range districtExits[dID] {
				if t == envID {
					exitsBack = true
					break
				}
			}
			if !exitsBack {
				res.Errors = append(res.Errors, fmt.Sprintf("environment %q claims endpoint %q, but that district has no connection to %q (one-way/broken)", envID, dID, envID))
			}
			// A district may belong to only one environment.
			if other, ok := endpointOwner[dID]; ok && other != envID {
				res.Errors = append(res.Errors, fmt.Sprintf("district %q is an endpoint of both %q and %q (double-booked)", dID, other, envID))
			} else {
				endpointOwner[dID] = envID
			}
		}
	}

	// 3. Reverse: any district that exits to an environment must be listed in that
	//    environment's connects (otherwise the env doesn't know about that mouth).
	for dID, targets := range districtExits {
		for _, t := range targets {
			if !envIDs[t] {
				continue
			}
			listed := false
			for _, c := range envConnects[t] {
				if c == dID {
					listed = true
					break
				}
			}
			if !listed {
				res.Errors = append(res.Errors, fmt.Sprintf("district %q exits to environment %q, but %q does not list it in connects (missing endpoint)", dID, t, t))
			}
		}
	}

	// 4. exit_to_environment should agree with an actual connection (light check).
	for dID, env := range districtExitEnv {
		has := false
		for _, t := range districtExits[dID] {
			if t == env {
				has = true
				break
			}
		}
		if !has {
			res.Warnings = append(res.Warnings, fmt.Sprintf("district %q has exit_to_environment %q but no connection pointing there", dID, env))
		}
	}

	// 5. Encounter coverage: every travel biome (environment_type) should have at
	//    least one monster, and every monster habitat tag should map to a known
	//    biome or the indoor/POI pool — catches drift like "coastal" vs "coast"
	//    and empty pools (an environment no monster can spawn in).
	monsterPaths, _ := filepath.Glob(monstersGlob)
	tagCount := map[string]int{}
	for _, p := range monsterPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var m struct {
			Environment []string `json:"environment"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		for _, t := range m.Environment {
			tagCount[t]++
		}
	}
	for biome := range envTypes {
		if tagCount[biome] == 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("environment_type %q has no monsters tagged for it (empty encounter pool)", biome))
		}
	}
	for tag := range tagCount {
		if !envTypes[tag] && !indoorTags[tag] {
			res.Warnings = append(res.Warnings, fmt.Sprintf("monster habitat tag %q maps to no environment_type or indoor pool (orphan/typo?)", tag))
		}
	}

	sort.Strings(res.Errors)
	sort.Strings(res.Warnings)
	return res, nil
}
