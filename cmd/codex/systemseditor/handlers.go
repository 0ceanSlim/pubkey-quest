package systemseditor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"pubkey-quest/cmd/codex/staging"
	"github.com/gorilla/mux"
)

// Serve HTML page
func (e *Editor) HandlePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "cmd/codex/html/systems-editor.html")
}

// Get all systems data
func (e *Editor) HandleGetSystemsData(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"effects":             e.Effects,
		"effect_types":        e.EffectTypes,
		"base_hp":             e.BaseHP,
		"starting_gold":       e.StartingGold,
		"generation_weights":  e.GenerationWeights,
		"introductions":       e.Introductions,
		"starting_locations":  e.StartingLocations,
		"advancement":         e.Advancement,
		"combat":              e.Combat,
		"encumbrance":         e.Encumbrance,
		"skills":              e.Skills,
		"travel_config":       e.TravelConfig,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Get all effects
func (e *Editor) HandleGetEffects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e.Effects)
}

// Get effect types
func (e *Editor) HandleGetEffectTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e.EffectTypes)
}

// Save individual effect
func (e *Editor) HandleSaveEffect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	effectID := vars["id"]

	var effect Effect
	if err := json.NewDecoder(r.Body).Decode(&effect); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Ensure ID matches URL
	effect.ID = effectID

	// Validate effect references valid effect types
	if err := e.ValidateEffect(effect); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Detect mode
	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := fmt.Sprintf("game-data/effects/%s.json", effectID)
	gitPath := strings.ReplaceAll(filePath, "\\", "/")
	newContent, _ := json.MarshalIndent(effect, "", "  ")

	if mode == staging.ModeDirect {
		if err := e.SaveEffect(effect); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		e.Effects[effectID] = effect
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "saved",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		// Read old content if file exists
		oldContent, _ := os.ReadFile(filePath)

		// Determine change type
		changeType := staging.ChangeUpdate
		if len(oldContent) == 0 {
			changeType = staging.ChangeCreate
		}

		session.AddChange(staging.Change{
			Type:       changeType,
			FilePath:   gitPath,
			OldContent: oldContent,
			NewContent: newContent,
			Timestamp:  time.Now(),
		})

		e.Effects[effectID] = effect
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}

// Create new effect
func (e *Editor) HandleCreateEffect(w http.ResponseWriter, r *http.Request) {
	var effect Effect
	if err := json.NewDecoder(r.Body).Decode(&effect); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Validate
	if err := e.ValidateEffect(effect); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if already exists
	if _, exists := e.Effects[effect.ID]; exists {
		http.Error(w, "Effect already exists", http.StatusConflict)
		return
	}

	// Detect mode
	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := fmt.Sprintf("game-data/effects/%s.json", effect.ID)
	gitPath := strings.ReplaceAll(filePath, "\\", "/")
	newContent, _ := json.MarshalIndent(effect, "", "  ")

	if mode == staging.ModeDirect {
		if err := e.SaveEffect(effect); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		e.Effects[effect.ID] = effect
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "created",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		session.AddChange(staging.Change{
			Type:       staging.ChangeCreate,
			FilePath:   gitPath,
			NewContent: newContent,
			Timestamp:  time.Now(),
		})

		e.Effects[effect.ID] = effect
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}

// Delete effect
func (e *Editor) HandleDeleteEffect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	effectID := vars["id"]

	// Check if exists
	if _, exists := e.Effects[effectID]; !exists {
		http.Error(w, "Effect not found", http.StatusNotFound)
		return
	}

	// Detect mode
	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := fmt.Sprintf("game-data/effects/%s.json", effectID)
	gitPath := strings.ReplaceAll(filePath, "\\", "/")

	if mode == staging.ModeDirect {
		if err := e.DeleteEffect(effectID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "deleted",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		oldContent, _ := os.ReadFile(filePath)

		session.AddChange(staging.Change{
			Type:       staging.ChangeDelete,
			FilePath:   gitPath,
			OldContent: oldContent,
			Timestamp:  time.Now(),
		})

		delete(e.Effects, effectID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}

// Update effect types
func (e *Editor) HandleSaveEffectTypes(w http.ResponseWriter, r *http.Request) {
	var types EffectTypes
	if err := json.NewDecoder(r.Body).Decode(&types); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Detect mode
	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := "game-data/systems/effects.json"
	gitPath := strings.ReplaceAll(filePath, "\\", "/")
	newContent, _ := json.MarshalIndent(types, "", "  ")

	if mode == staging.ModeDirect {
		if err := e.SaveEffectTypes(types); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		e.EffectTypes = types
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "saved",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		oldContent, _ := os.ReadFile(filePath)

		session.AddChange(staging.Change{
			Type:       staging.ChangeUpdate,
			FilePath:   gitPath,
			OldContent: oldContent,
			NewContent: newContent,
			Timestamp:  time.Now(),
		})

		e.EffectTypes = types
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}

// Save skills config
func (e *Editor) HandleSaveSkills(w http.ResponseWriter, r *http.Request) {
	var newData json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := "game-data/systems/skills.json"
	gitPath := strings.ReplaceAll(filePath, "\\", "/")
	newContent, _ := json.MarshalIndent(newData, "", "  ")

	if mode == staging.ModeDirect {
		if err := os.WriteFile(filePath, newContent, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
			return
		}
		json.Unmarshal(newData, &e.Skills)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "saved",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		oldContent, _ := os.ReadFile(filePath)

		session.AddChange(staging.Change{
			Type:       staging.ChangeUpdate,
			FilePath:   gitPath,
			OldContent: oldContent,
			NewContent: newContent,
			Timestamp:  time.Now(),
		})

		json.Unmarshal(newData, &e.Skills)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}

// Save travel config
func (e *Editor) HandleSaveTravelConfig(w http.ResponseWriter, r *http.Request) {
	var newData json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := "game-data/systems/travel-config.json"
	gitPath := strings.ReplaceAll(filePath, "\\", "/")
	newContent, _ := json.MarshalIndent(newData, "", "  ")

	if mode == staging.ModeDirect {
		if err := os.WriteFile(filePath, newContent, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
			return
		}
		json.Unmarshal(newData, &e.TravelConfig)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "saved",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		oldContent, _ := os.ReadFile(filePath)

		session.AddChange(staging.Change{
			Type:       staging.ChangeUpdate,
			FilePath:   gitPath,
			OldContent: oldContent,
			NewContent: newContent,
			Timestamp:  time.Now(),
		})

		json.Unmarshal(newData, &e.TravelConfig)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}

// Save encumbrance config (base_weight_multiplier etc.)
func (e *Editor) HandleSaveEncumbrance(w http.ResponseWriter, r *http.Request) {
	var newData json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&newData); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	mode := staging.DetectMode(r, e.Config)
	sessionID := r.Header.Get("X-Session-ID")

	filePath := "game-data/systems/encumbrance.json"
	gitPath := strings.ReplaceAll(filePath, "\\", "/")
	newContent, _ := json.MarshalIndent(newData, "", "  ")

	if mode == staging.ModeDirect {
		if err := os.WriteFile(filePath, newContent, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
			return
		}
		json.Unmarshal(newData, &e.Encumbrance)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "saved",
			"mode":   "direct",
		})
	} else {
		session := staging.Manager.GetSession(sessionID)
		if session == nil {
			http.Error(w, "Invalid session", http.StatusBadRequest)
			return
		}

		oldContent, _ := os.ReadFile(filePath)

		session.AddChange(staging.Change{
			Type:       staging.ChangeUpdate,
			FilePath:   gitPath,
			OldContent: oldContent,
			NewContent: newContent,
			Timestamp:  time.Now(),
		})

		json.Unmarshal(newData, &e.Encumbrance)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "staged",
			"mode":    "staging",
			"changes": len(session.Changes),
		})
	}
}
