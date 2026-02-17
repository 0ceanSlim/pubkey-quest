package systemseditor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/cmd/codex/config"
)

type Editor struct {
	// Core data
	EffectTypes EffectTypes      // systems/effects.json
	Effects     map[string]Effect // effects/*.json (keyed by ID)

	// Character generation (from systems/new-character/)
	BaseHP            interface{} // base-hp.json
	StartingGold      interface{} // starting-gold.json
	GenerationWeights interface{} // generation-weights.json
	Introductions     interface{} // introductions.json
	StartingLocations interface{} // starting-locations.json

	// Other systems
	Advancement  interface{} // advancement.json
	Combat       interface{} // combat.json
	Encumbrance  interface{} // encumbrance.json
	Skills       interface{} // skills.json
	TravelConfig interface{} // travel-config.json

	Config *config.Config
}

func NewEditor(cfg *config.Config) *Editor {
	return &Editor{
		Effects: make(map[string]Effect),
		Config:  cfg,
	}
}

func (e *Editor) LoadAll() error {
	// 1. Load effect types (systems/effects.json)
	effectTypesPath := "game-data/systems/effects.json"
	effectTypesData, err := os.ReadFile(effectTypesPath)
	if err != nil {
		return fmt.Errorf("failed to read effect types: %w", err)
	}
	if err := json.Unmarshal(effectTypesData, &e.EffectTypes); err != nil {
		return fmt.Errorf("failed to parse effect types: %w", err)
	}

	// 2. Load all individual effects (effects/*.json)
	files, err := filepath.Glob("game-data/effects/*.json")
	if err != nil {
		return fmt.Errorf("failed to glob effects directory: %w", err)
	}
	for _, file := range files {
		var effect Effect
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read effect file %s: %w", file, err)
		}
		if err := json.Unmarshal(data, &effect); err != nil {
			return fmt.Errorf("failed to parse effect file %s: %w", file, err)
		}
		e.Effects[effect.ID] = effect
	}

	// 3. Load character generation files
	if err := e.loadJSONFile("game-data/systems/new-character/base-hp.json", &e.BaseHP); err != nil {
		fmt.Printf("⚠️  Warning: failed to load base-hp.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/new-character/starting-gold.json", &e.StartingGold); err != nil {
		fmt.Printf("⚠️  Warning: failed to load starting-gold.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/new-character/generation-weights.json", &e.GenerationWeights); err != nil {
		fmt.Printf("⚠️  Warning: failed to load generation-weights.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/new-character/introductions.json", &e.Introductions); err != nil {
		fmt.Printf("⚠️  Warning: failed to load introductions.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/new-character/starting-locations.json", &e.StartingLocations); err != nil {
		fmt.Printf("⚠️  Warning: failed to load starting-locations.json: %v\n", err)
	}

	// 4. Load other system files
	if err := e.loadJSONFile("game-data/systems/advancement.json", &e.Advancement); err != nil {
		fmt.Printf("⚠️  Warning: failed to load advancement.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/combat.json", &e.Combat); err != nil {
		fmt.Printf("⚠️  Warning: failed to load combat.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/encumbrance.json", &e.Encumbrance); err != nil {
		fmt.Printf("⚠️  Warning: failed to load encumbrance.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/skills.json", &e.Skills); err != nil {
		fmt.Printf("⚠️  Warning: failed to load skills.json: %v\n", err)
	}
	if err := e.loadJSONFile("game-data/systems/travel-config.json", &e.TravelConfig); err != nil {
		fmt.Printf("⚠️  Warning: failed to load travel-config.json: %v\n", err)
	}

	return nil
}

func (e *Editor) loadJSONFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func (e *Editor) SaveEffect(effect Effect) error {
	filePath := fmt.Sprintf("game-data/effects/%s.json", effect.ID)
	data, err := json.MarshalIndent(effect, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal effect: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write effect file: %w", err)
	}
	return nil
}

func (e *Editor) SaveEffectTypes(types EffectTypes) error {
	filePath := "game-data/systems/effects.json"
	data, err := json.MarshalIndent(types, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal effect types: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write effect types file: %w", err)
	}
	return nil
}

func (e *Editor) DeleteEffect(effectID string) error {
	filePath := fmt.Sprintf("game-data/effects/%s.json", effectID)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete effect file: %w", err)
	}
	delete(e.Effects, effectID)
	return nil
}

func (e *Editor) ValidateEffect(effect Effect) error {
	// Check that all effect.Modifiers[].Stat references valid EffectTypeDefinition
	for _, modifier := range effect.Modifiers {
		if _, exists := e.EffectTypes.EffectTypes[modifier.Stat]; !exists {
			return fmt.Errorf("invalid effect stat '%s' (not defined in systems/effects.json)", modifier.Stat)
		}
	}

	// Basic validation
	if effect.ID == "" {
		return fmt.Errorf("effect ID cannot be empty")
	}
	if effect.Name == "" {
		return fmt.Errorf("effect name cannot be empty")
	}
	if strings.Contains(effect.ID, " ") || strings.Contains(effect.ID, "/") || strings.Contains(effect.ID, "\\") {
		return fmt.Errorf("effect ID cannot contain spaces or path separators")
	}

	return nil
}
