// Package spells provides spell preparation and management logic.
package spells

import (
	"fmt"
	"log"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// SlotLevelToSpellLevel maps slot level strings to spell levels.
var SlotLevelToSpellLevel = map[string]int{
	"cantrips": 0,
	"level_1":  1,
	"level_2":  2,
	"level_3":  3,
	"level_4":  4,
	"level_5":  5,
	"level_6":  6,
	"level_7":  7,
	"level_8":  8,
	"level_9":  9,
}

// SpellLevelToSlotLevel maps spell level ints back to slot level strings.
var SpellLevelToSlotLevel = map[int]string{
	0: "cantrips",
	1: "level_1",
	2: "level_2",
	3: "level_3",
	4: "level_4",
	5: "level_5",
	6: "level_6",
	7: "level_7",
	8: "level_8",
	9: "level_9",
}

// PrepMinutes returns how long (in minutes) a spell takes to prepare.
// Cantrips (level 0) are instant (0 minutes). Leveled spells take level×60 minutes.
func PrepMinutes(spellLevel int) int {
	if spellLevel <= 0 {
		return 0
	}
	return spellLevel * 60
}

// AbsoluteMinutes converts (currentDay, timeOfDay) into an absolute minute counter.
func AbsoluteMinutes(currentDay, timeOfDay int) int {
	return currentDay*1440 + timeOfDay
}

// SetSpellInSlot writes a spell ID into a specific slot in the SpellSlots map.
// SpellSlots format: {"level_1": [{"slot": 0, "spell": null, "quantity": 0}, ...]}
func SetSpellInSlot(slots map[string]interface{}, slotLevel string, slotIndex int, spellID string) bool {
	raw, ok := slots[slotLevel]
	if !ok {
		return false
	}
	slotList, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, entry := range slotList {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		idx := -1
		switch v := m["slot"].(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		}
		if idx == slotIndex {
			m["spell"] = spellID
			return true
		}
	}
	return false
}

// ClearSpellInSlot sets a spell slot to null.
func ClearSpellInSlot(slots map[string]interface{}, slotLevel string, slotIndex int) bool {
	raw, ok := slots[slotLevel]
	if !ok {
		return false
	}
	slotList, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, entry := range slotList {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		idx := -1
		switch v := m["slot"].(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		}
		if idx == slotIndex {
			m["spell"] = nil
			return true
		}
	}
	return false
}

// HasSlot returns true if the given slotLevel+slotIndex exists in SpellSlots.
func HasSlot(slots map[string]interface{}, slotLevel string, slotIndex int) bool {
	raw, ok := slots[slotLevel]
	if !ok {
		return false
	}
	slotList, ok := raw.([]interface{})
	if !ok {
		return false
	}
	for _, entry := range slotList {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		switch v := m["slot"].(type) {
		case float64:
			if int(v) == slotIndex {
				return true
			}
		case int:
			if v == slotIndex {
				return true
			}
		}
	}
	return false
}

// FindPrepTask returns the index of a prep task matching the given slot, or -1.
func FindPrepTask(queue []types.SpellPrepTask, slotLevel string, slotIndex int) int {
	for i, t := range queue {
		if t.SlotLevel == slotLevel && t.SlotIndex == slotIndex {
			return i
		}
	}
	return -1
}

// RemovePrepTask removes a prep task at the given index from the queue (order-preserving).
func RemovePrepTask(queue []types.SpellPrepTask, idx int) []types.SpellPrepTask {
	return append(queue[:idx], queue[idx+1:]...)
}

// ResolvePrepTimers checks all pending prep tasks against current game time and completes
// any that are ready. Completed tasks are slotted directly into the save's SpellSlots.
// Returns log messages for any spells that finished preparing.
// Should be called on every time advancement (from AdvanceTime).
func ResolvePrepTimers(sess *session.GameSession) []string {
	if len(sess.PrepQueue) == 0 {
		return nil
	}

	currentAbsolute := AbsoluteMinutes(sess.SaveData.CurrentDay, sess.SaveData.TimeOfDay)
	spellSlots := sess.SaveData.SpellSlots

	var messages []string
	var remaining []types.SpellPrepTask

	for _, task := range sess.PrepQueue {
		if task.ReadyAtAbsolute <= currentAbsolute {
			// Spell is ready — slot it in
			ok := SetSpellInSlot(spellSlots, task.SlotLevel, task.SlotIndex, task.SpellID)
			if ok {
				msg := fmt.Sprintf("You have finished preparing %s.", task.SpellID)
				messages = append(messages, msg)
				log.Printf("🔮 Spell prepared: %s → %s[%d]", task.SpellID, task.SlotLevel, task.SlotIndex)
			} else {
				log.Printf("⚠️ Failed to slot prepared spell %s into %s[%d]", task.SpellID, task.SlotLevel, task.SlotIndex)
			}
		} else {
			remaining = append(remaining, task)
		}
	}

	sess.PrepQueue = remaining
	return messages
}

// MinutesRemaining returns how many in-game minutes remain until a task completes.
// Returns 0 if the task is already ready.
func MinutesRemaining(task types.SpellPrepTask, currentDay, timeOfDay int) int {
	currentAbsolute := AbsoluteMinutes(currentDay, timeOfDay)
	diff := task.ReadyAtAbsolute - currentAbsolute
	if diff < 0 {
		return 0
	}
	return diff
}
