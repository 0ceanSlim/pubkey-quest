package types

// EffectData represents the complete effect definition (from JSON files)
type EffectData struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	SourceType  string           `json:"source_type"` // "system_ticker", "system_status", "applied"
	Category    string           `json:"category"`    // "buff", "debuff", "status"
	Removal     RemovalCondition `json:"removal"`
	SystemCheck *SystemCheck     `json:"system_check,omitempty"` // For system_status effects: when to activate
	Modifiers   []Modifier       `json:"modifiers"`
	Message     string           `json:"message,omitempty"`
	Visible     bool             `json:"visible"`
}

// RemovalCondition describes how an effect is removed
type RemovalCondition struct {
	Type      string `json:"type"`                // "permanent", "timed", "conditional", "action", "equipment", "hybrid"
	Timer     int    `json:"timer,omitempty"`     // Duration in minutes (for timed/hybrid)
	Condition string `json:"condition,omitempty"` // Condition string (for conditional)
	Action    string `json:"action,omitempty"`    // Action ID (for action/hybrid)
}

// Modifier describes a single stat/resource modification
type Modifier struct {
	Stat         string `json:"stat"`                    // "hp", "strength", etc.
	Value        int    `json:"value"`                   // Amount to modify
	Type         string `json:"type"`                    // "instant" or "periodic"
	Delay        int    `json:"delay,omitempty"`         // Minutes before this modifier activates
	TickInterval int    `json:"tick_interval,omitempty"` // For periodic: minutes between applications
}

// SystemCheck defines when a system status effect should be active
type SystemCheck struct {
	Stat     string `json:"stat"`     // "hunger", "fatigue", "weight_percent", "hp_percent", "mana_percent"
	Operator string `json:"operator"` // "==", "!=", "<", "<=", ">", ">="
	Value    int    `json:"value"`    // The threshold value
}
