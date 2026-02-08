package systemseditor

// Effect Type Definition (systems/effects.json)
type EffectTypeDefinition struct {
	ID          string `json:"id"`
	Property    string `json:"property"`
	Description string `json:"description"`
}

type EffectTypes struct {
	EffectTypes map[string]EffectTypeDefinition `json:"effect_types"`
}

// Individual Effect (effects/*.json)
type Effect struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Effects     []EffectDetail `json:"effects"`
	Message     string         `json:"message,omitempty"`
	Category    string         `json:"category,omitempty"` // "buff", "debuff", "system"
	Silent      *bool          `json:"silent,omitempty"`   // Pointer to distinguish false from unset
	Icon        string         `json:"icon,omitempty"`
	Color       string         `json:"color,omitempty"`
}

type EffectDetail struct {
	Type         string `json:"type"`                    // References EffectTypeDefinition.ID
	Value        int    `json:"value"`
	Duration     int    `json:"duration"`
	Delay        int    `json:"delay"`
	TickInterval int    `json:"tick_interval,omitempty"`
}
