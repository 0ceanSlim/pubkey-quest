package npc

import (
	"pubkey-quest/types"
	"strings"
)

// GetCurrentScheduleSlot finds the active schedule slot on a day (the save's
// CurrentDay) at a time of day (minutes from midnight, 0-1439). An entry for
// this day of the week ("days") wins over the everyday entries.
// Returns nil if no schedule exists.
func GetCurrentScheduleSlot(schedule []types.NPCScheduleSlot, day, timeOfDay int) *types.NPCScheduleSlot {
	if len(schedule) == 0 {
		return nil // No schedule defined, NPC uses default behavior
	}

	// Normalize time to 0-1439 range
	t := ((timeOfDay % 1440) + 1440) % 1440
	today := strings.ToLower(types.Weekday(day))
	yesterday := strings.ToLower(types.Weekday(day - 1))

	// This day's own entries first. One that wraps past midnight belongs to
	// the day it started: its early-morning part is checked against yesterday.
	for i := range schedule {
		slot := &schedule[i]
		if len(slot.Days) == 0 {
			continue
		}
		if slot.End < slot.Start {
			if (t >= slot.Start && hasDay(slot.Days, today)) || (t < slot.End && hasDay(slot.Days, yesterday)) {
				return slot
			}
		} else if t >= slot.Start && t < slot.End && hasDay(slot.Days, today) {
			return slot
		}
	}

	for i := range schedule {
		slot := &schedule[i]
		if len(slot.Days) > 0 {
			continue
		}
		if types.InWindow(t, slot.Start, slot.End) {
			return slot
		}
	}

	// Shouldn't happen if schedule covers full 24h, but fallback to first slot
	return &schedule[0]
}

func hasDay(days []string, day string) bool {
	for _, d := range days {
		if strings.EqualFold(d, day) {
			return true
		}
	}
	return false
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
func ResolveNPCSchedule(npc *types.NPCData, day, timeOfDay int) *types.NPCScheduleInfo {
	currentSlot := GetCurrentScheduleSlot(npc.Schedule, day, timeOfDay)

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
