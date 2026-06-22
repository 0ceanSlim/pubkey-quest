package npc

import (
	"pubkey-quest/types"
	"strings"
)

// GetCurrentScheduleSlot finds the active schedule slot for a given time of day
// timeOfDay: minutes from midnight (0-1439)
// Returns: the active slot, or nil if no schedule exists
func GetCurrentScheduleSlot(schedule []types.NPCScheduleSlot, timeOfDay int) *types.NPCScheduleSlot {
	if len(schedule) == 0 {
		return nil // No schedule defined, NPC uses default behavior
	}

	// Normalize time to 0-1439 range
	normalizedTime := timeOfDay % 1440
	if normalizedTime < 0 {
		normalizedTime += 1440
	}

	for i := range schedule {
		slot := &schedule[i]

		// Handle slots that wrap around midnight
		if slot.End < slot.Start {
			// e.g., 22:00 (1320) to 06:00 (360)
			if normalizedTime >= slot.Start || normalizedTime < slot.End {
				return slot
			}
		} else {
			// Normal slot within same day
			if normalizedTime >= slot.Start && normalizedTime < slot.End {
				return slot
			}
		}
	}

	// Shouldn't happen if schedule covers full 24h, but fallback to first slot
	return &schedule[0]
}

// DetermineLocationType determines if a location ID is a building or district
// Buildings use underscores (e.g., "kingdom_general_store", "aurelia_home")
// Districts use hyphens (e.g., "kingdom-center", "city-east")
func DetermineLocationType(locationID string) string {
	if strings.Contains(locationID, "_") {
		return "building"
	}
	return "district"
}

// ResolveNPCSchedule returns current schedule state for an NPC
func ResolveNPCSchedule(npc *types.NPCData, timeOfDay int) *types.NPCScheduleInfo {
	currentSlot := GetCurrentScheduleSlot(npc.Schedule, timeOfDay)

	if currentSlot == nil {
		// No schedule - NPC is always available (fallback for NPCs without schedules)
		return &types.NPCScheduleInfo{
			CurrentSlot:       nil,
			IsAvailable:       true,
			Location:          "",
			State:             "working",
			AvailableDialogue: getAllDialogueKeys(npc.Dialogue),
			AvailableActions:  getAllActions(npc),
		}
	}

	return &types.NPCScheduleInfo{
		CurrentSlot:       currentSlot,
		IsAvailable:       len(currentSlot.AvailableActions) > 0 || len(currentSlot.DialogueOptions) > 0,
		Location:          currentSlot.Location,
		Room:              currentSlot.Room,
		State:             currentSlot.State,
		AvailableDialogue: currentSlot.DialogueOptions,
		AvailableActions:  currentSlot.AvailableActions,
	}
}

// getAllDialogueKeys extracts all dialogue keys (for backward compat when no schedule)
func getAllDialogueKeys(dialogue map[string]interface{}) []string {
	if dialogue == nil {
		return []string{}
	}
	keys := make([]string, 0, len(dialogue))
	for k := range dialogue {
		keys = append(keys, k)
	}
	return keys
}

// getAllActions extracts all possible actions from NPC config (for backward compat)
func getAllActions(npc *types.NPCData) []string {
	actions := []string{}
	if npc.ShopConfig != nil {
		actions = append(actions, "open_shop", "open_sell")
	}
	if npc.StorageConfig != nil {
		actions = append(actions, "open_storage", "register_storage")
	}
	if npc.InnConfig != nil {
		actions = append(actions, "rent_room")
	}
	return actions
}

// GetDayOfWeek calculates day of week from current day (0 = Sunday, 6 = Saturday)
// Assumes all adventures start on Sunday (day 0)
func GetDayOfWeek(currentDay int) int {
	return currentDay % 7
}
