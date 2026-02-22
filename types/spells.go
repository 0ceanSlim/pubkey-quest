package types

// SpellPrepTask tracks an in-progress spell preparation in session memory.
// Prep state is NEVER written to the save file — lost on session clear.
type SpellPrepTask struct {
	SpellID         string `json:"spell_id"`
	SlotLevel       string `json:"slot_level"`       // "cantrips", "level_1", "level_2", etc.
	SlotIndex       int    `json:"slot_index"`
	ReadyAtAbsolute int    `json:"ready_at_absolute"` // CurrentDay*1440 + TimeOfDay at completion
}
