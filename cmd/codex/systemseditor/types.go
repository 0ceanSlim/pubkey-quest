package systemseditor

import "pubkey-quest/types"

// Use shared types from types package
type Effect = types.EffectData
type Modifier = types.Modifier
type RemovalCondition = types.RemovalCondition

// Effect Type Definition (systems/effects.json) - codex-specific
type EffectTypeDefinition struct {
	ID             string `json:"id"`
	Property       string `json:"property"`
	Description    string `json:"description"`
	Category       string `json:"category"`        // "stat", "resource", "capacity"
	AllowsPeriodic bool   `json:"allows_periodic"` // Whether this type can have periodic modifiers
}

type EffectTypes struct {
	EffectTypes map[string]EffectTypeDefinition `json:"effect_types"`
}
