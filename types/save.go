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

	// Schema v2 fields (populated by M2/M3; zero-valued on v1 saves). Per §4
	// these hold only player decisions and non-derivable outcomes.
	Room            string          `json:"room,omitempty"`             // Current room within a building (M2)
	QuestsCompleted []string        `json:"quests_completed,omitempty"` // Completed quest IDs; quest points/availability derive from this list
	QuestsActive    []QuestProgress `json:"quests_active,omitempty"`    // In-progress quests
	POIStates       []POIState      `json:"poi_states,omitempty"`       // Per-POI interaction state
	Rentals         []Rental        `json:"rentals,omitempty"`          // Paid rooms held until expiry (M2)
	AbilityIncreases map[string]int `json:"ability_increases,omitempty"` // Points spent per ability via level-up allocation (player choice). Unspent count derives — see character.UnspentAbilityPoints
	FeatsChosen     []string        `json:"feats_chosen,omitempty"`     // Selected feat IDs (reserved; activated in the feats milestone — docs/draft/feats-progression.md)
	SchemaVersion   int             `json:"schema_version,omitempty"`   // Save schema version (see CurrentSchemaVersion)

	InternalID          string                   `json:"-"`                        // Not serialized, used internally for file naming
	InternalNpub        string                   `json:"-"`                        // Not serialized, used internally for directory structure
}

// CurrentSchemaVersion is the save schema version this build writes. The load
// path stamps older saves up to this value (see the migration shim in the
// session package).
const CurrentSchemaVersion = 2

// QuestProgress is one in-progress quest. ObjectiveCounts indexes the current
// stage's objectives (e.g. slay 3 of 5). Completed quests live in
// SaveFile.QuestsCompleted as plain IDs.
type QuestProgress struct {
	QuestID         string `json:"quest_id"`
	Stage           int    `json:"stage"`
	ObjectiveCounts []int  `json:"objective_counts,omitempty"`
}

// POIState is the per-save runtime state of a discovered POI. "Fresh again"
// derives from cooldown math against LastDay/LastMinute.
type POIState struct {
	POIID      string `json:"poi_id"`
	LastDay    int    `json:"last_day"`
	LastMinute int    `json:"last_minute"`
	Passed     bool   `json:"passed,omitempty"`
	Looted     bool   `json:"looted,omitempty"`
	Cleared    bool   `json:"cleared,omitempty"`
}

// Rental is a paid room the player holds until it expires (in-game day/minute).
type Rental struct {
	Building   string `json:"building"`
	ExpiresDay int    `json:"expires_day"`
	ExpiresMin int    `json:"expires_minute"`
}
