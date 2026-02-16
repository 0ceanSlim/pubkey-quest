package types

// ActiveEffect stores only runtime state for an active effect (template data is in database)
type ActiveEffect struct {
	EffectID          string  `json:"effect_id"`          // ID of effect template (e.g., "performance-high")
	EffectIndex       int     `json:"effect_index"`       // Index in effect's effects array (0 for single-effect templates)
	DurationRemaining float64 `json:"duration_remaining"` // Minutes remaining until effect expires
	TotalDuration     float64 `json:"total_duration"`     // Original duration when effect was applied (for progress calculation)
	DelayRemaining    float64 `json:"delay_remaining"`    // Minutes remaining before effect starts
	TickAccumulator   float64 `json:"tick_accumulator"`   // Time accumulated since last tick (for periodic effects)
	AppliedAt         int     `json:"applied_at"`         // Time of day (minutes) when effect was applied
}

// EnrichedEffect combines runtime state with template data for API responses
type EnrichedEffect struct {
	ActiveEffect
	Name          string         `json:"name"`           // Display name from template
	Description   string         `json:"description"`    // Effect description from template
	Category      string         `json:"category"`       // buff, debuff, modifier, system
	StatModifiers map[string]int `json:"stat_modifiers"` // Map of stat name to modifier value
	TickInterval  float64        `json:"tick_interval"`  // Minutes between ticks (for periodic effects)
}

// SaveFile represents the complete game state for a player
type SaveFile struct {
	D                   string                   `json:"d"`
	CreatedAt           string                   `json:"created_at"`
	Race                string                   `json:"race"`
	Class               string                   `json:"class"`
	Background          string                   `json:"background"`
	Alignment           string                   `json:"alignment"`
	Experience          int                      `json:"experience"`
	HP                  int                      `json:"hp"`
	MaxHP               int                      `json:"max_hp"`
	Mana                int                      `json:"mana"`
	MaxMana             int                      `json:"max_mana"`
	Fatigue             int                      `json:"fatigue"`             // Fatigue level (0-10+), penalties applied via effects
	Hunger              int                      `json:"hunger"`              // Hunger level (0-3: 0=Famished, 1=Hungry, 2=Satisfied, 3=Full), penalties applied via effects
	Stats               map[string]interface{}   `json:"stats"`
	Location            string                   `json:"location"`            // City ID or environment ID when traveling
	District            string                   `json:"district"`            // District key, or origin district ID when traveling (e.g., "kingdom-east")
	Building            string                   `json:"building"`            // Building ID or empty for outdoors
	TravelProgress      float64                  `json:"travel_progress,omitempty"` // 0.0-1.0 percentage through environment when traveling
	TravelStopped       bool                     `json:"travel_stopped,omitempty"`  // True when player has stopped moving in environment (time still flows)
	CurrentDay          int                      `json:"current_day"`
	TimeOfDay           int                      `json:"time_of_day"` // Minutes in current day (0-1439, where 720=noon, 0=midnight)
	Inventory           map[string]interface{}   `json:"inventory"`
	Vaults              []map[string]interface{} `json:"vaults"`
	KnownSpells         []string                 `json:"known_spells"`
	SpellSlots          map[string]interface{}   `json:"spell_slots"`
	LocationsDiscovered []string                 `json:"locations_discovered"`
	MusicTracksUnlocked []string                 `json:"music_tracks_unlocked"`
	ActiveEffects       []ActiveEffect           `json:"active_effects,omitempty"` // Compact format: only runtime state, template data in DB
	InternalID          string                   `json:"-"`                        // Not serialized, used internally for file naming
	InternalNpub        string                   `json:"-"`                        // Not serialized, used internally for directory structure
}
