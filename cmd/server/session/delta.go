package session

import (
	"strconv"

	"pubkey-quest/types"
)

// Delta represents changes between states for surgical frontend updates.
// Only fields that changed are included (omitempty ensures minimal payload).
type Delta struct {
	Character *CharacterDelta `json:"character,omitempty"`
	NPCs      *NPCDelta       `json:"npcs,omitempty"`
	Buildings *BuildingDelta  `json:"buildings,omitempty"`
	Inventory *InventoryDelta `json:"inventory,omitempty"`
	Equipment *EquipmentDelta `json:"equipment,omitempty"`
	Location  *LocationDelta  `json:"location,omitempty"`
	Effects   *EffectsDelta   `json:"effects,omitempty"`
	ShowReady *ShowReadyDelta `json:"show_ready,omitempty"`
}

// ShowReadyDelta holds changes to show readiness state
type ShowReadyDelta struct {
	IsReady    bool   `json:"is_ready"`              // Whether a show is ready to perform
	BuildingID string `json:"building_id,omitempty"` // Building where the show is (if ready)
}

// CharacterDelta holds changes to character stats
type CharacterDelta struct {
	HP         *int `json:"hp,omitempty"`
	MaxHP      *int `json:"max_hp,omitempty"`
	Mana       *int `json:"mana,omitempty"`
	MaxMana    *int `json:"max_mana,omitempty"`
	Fatigue    *int `json:"fatigue,omitempty"`
	Hunger     *int `json:"hunger,omitempty"`
	Gold       *int `json:"gold,omitempty"`
	XP         *int `json:"xp,omitempty"`
	TimeOfDay  *int `json:"time_of_day,omitempty"`
	CurrentDay *int `json:"current_day,omitempty"`
}

// NPCDelta holds changes to NPCs at current location
type NPCDelta struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

// BuildingDelta holds changes to building states (open/closed)
type BuildingDelta struct {
	StateChanged map[string]bool `json:"state_changed,omitempty"` // building_id -> isOpen
}

// InventoryDelta holds changes to inventory slots
type InventoryDelta struct {
	GeneralSlots  map[int]InventorySlotDelta `json:"general_slots,omitempty"`
	BackpackSlots map[int]InventorySlotDelta `json:"backpack_slots,omitempty"`
}

// InventorySlotDelta holds changes to a single inventory slot
type InventorySlotDelta struct {
	ItemID   *string `json:"item_id,omitempty"`
	Quantity *int    `json:"quantity,omitempty"`
	Made     *int    `json:"t,omitempty"`     // a food's made-time (its freshness bar)
	Wear     *int    `json:"w,omitempty"`     // wear taken (its wear bar)
	Burn     *int    `json:"b,omitempty"`     // a lamp's oil left
	Uses     *int    `json:"u,omitempty"`     // charges spent
	Empty    bool    `json:"empty,omitempty"` // True if slot was cleared
}

// EquipmentDelta holds changes to equipment slots
type EquipmentDelta struct {
	Changed map[string]*string `json:"changed,omitempty"` // slot_name -> item_id (nil for empty)
}

// LocationDelta holds changes to location state
type LocationDelta struct {
	City     *string `json:"city,omitempty"`
	District *string `json:"district,omitempty"`
	Building *string `json:"building,omitempty"`
}

// EffectsDelta holds changes to active effects
type EffectsDelta struct {
	Added   []string `json:"added,omitempty"`   // Effect IDs added
	Removed []string `json:"removed,omitempty"` // Effect IDs removed
	Updated []string `json:"updated,omitempty"` // Effect IDs with duration changes
}

// CreateSnapshot creates a snapshot from current session state
func CreateSnapshot(save *types.SaveFile, npcs []string, buildings map[string]bool) *SessionSnapshot {
	snapshot := &SessionSnapshot{
		HP:             save.HP,
		MaxHP:          save.MaxHP,
		Mana:           save.Mana,
		MaxMana:        save.MaxMana,
		Fatigue:        save.Fatigue,
		Hunger:         save.Hunger,
		TimeOfDay:      save.TimeOfDay,
		CurrentDay:     save.CurrentDay,
		City:           save.Location,
		District:       save.District,
		Building:       save.Building,
		NPCs:           make([]string, len(npcs)),
		Buildings:      make(map[string]bool),
		EquipmentSlots: make(map[string]string),
		ActiveEffects:  make([]string, 0),
	}

	// Copy NPCs
	copy(snapshot.NPCs, npcs)

	// Copy buildings
	for id, isOpen := range buildings {
		snapshot.Buildings[id] = isOpen
	}

	// Extract gold from stats if present
	if save.Stats != nil {
		if gold, ok := save.Stats["gold"].(float64); ok {
			snapshot.Gold = int(gold)
		}
	}

	// Copy inventory general slots
	if save.Inventory != nil {
		if generalSlots, ok := save.Inventory["general_slots"].([]interface{}); ok {
			for i, slot := range generalSlots {
				if i >= 4 {
					break
				}
				if slotMap, ok := slot.(map[string]interface{}); ok {
					if itemID, ok := slotMap["item"].(string); ok {
						snapshot.GeneralSlots[i].ItemID = itemID
						snapshot.GeneralSlots[i].Quantity = slotInt(slotMap, "quantity")
						snapshot.GeneralSlots[i].Made = slotInt(slotMap, "t")
						snapshot.GeneralSlots[i].Wear = slotInt(slotMap, "w")
						snapshot.GeneralSlots[i].Burn = slotInt(slotMap, "b")
						snapshot.GeneralSlots[i].Uses = slotInt(slotMap, "u")
					}
				}
			}
		}

		// Copy backpack slots
		if gearSlots, ok := save.Inventory["gear_slots"].(map[string]interface{}); ok {
			if bag, ok := gearSlots["bag"].(map[string]interface{}); ok {
				if contents, ok := bag["contents"].([]interface{}); ok {
					for i, slot := range contents {
						if i >= 20 {
							break
						}
						if slotMap, ok := slot.(map[string]interface{}); ok {
							if itemID, ok := slotMap["item"].(string); ok {
								snapshot.BackpackSlots[i].ItemID = itemID
								snapshot.BackpackSlots[i].Quantity = slotInt(slotMap, "quantity")
								snapshot.BackpackSlots[i].Made = slotInt(slotMap, "t")
								snapshot.BackpackSlots[i].Wear = slotInt(slotMap, "w")
								snapshot.BackpackSlots[i].Burn = slotInt(slotMap, "b")
								snapshot.BackpackSlots[i].Uses = slotInt(slotMap, "u")
							}
						}
					}
				}
			}

			// Copy equipment slots
			equipmentSlotNames := []string{"neck", "head", "ammo", "mainhand", "chest", "offhand", "ring1", "legs", "ring2", "gloves", "boots", "bag"}
			for _, slotName := range equipmentSlotNames {
				if slot, ok := gearSlots[slotName].(map[string]interface{}); ok {
					if itemID, ok := slot["item"].(string); ok && itemID != "" {
						snapshot.EquipmentSlots[slotName] = itemID
					}
				}
			}
		}
	}

	// Copy active effects
	for _, effect := range save.ActiveEffects {
		snapshot.ActiveEffects = append(snapshot.ActiveEffects, effect.EffectID)
	}

	return snapshot
}

// CalculateDelta compares two snapshots and returns only the changes
func CalculateDelta(old, new *SessionSnapshot) *Delta {
	if old == nil || new == nil {
		return nil
	}

	delta := &Delta{}

	// Character stats delta
	charDelta := &CharacterDelta{}
	hasCharChanges := false

	if old.HP != new.HP {
		charDelta.HP = &new.HP
		hasCharChanges = true
	}
	if old.MaxHP != new.MaxHP {
		charDelta.MaxHP = &new.MaxHP
		hasCharChanges = true
	}
	if old.Mana != new.Mana {
		charDelta.Mana = &new.Mana
		hasCharChanges = true
	}
	if old.MaxMana != new.MaxMana {
		charDelta.MaxMana = &new.MaxMana
		hasCharChanges = true
	}
	if old.Fatigue != new.Fatigue {
		charDelta.Fatigue = &new.Fatigue
		hasCharChanges = true
	}
	if old.Hunger != new.Hunger {
		charDelta.Hunger = &new.Hunger
		hasCharChanges = true
	}
	if old.Gold != new.Gold {
		charDelta.Gold = &new.Gold
		hasCharChanges = true
	}
	if old.XP != new.XP {
		charDelta.XP = &new.XP
		hasCharChanges = true
	}
	if old.TimeOfDay != new.TimeOfDay {
		charDelta.TimeOfDay = &new.TimeOfDay
		hasCharChanges = true
	}
	if old.CurrentDay != new.CurrentDay {
		charDelta.CurrentDay = &new.CurrentDay
		hasCharChanges = true
	}

	if hasCharChanges {
		delta.Character = charDelta
	}

	// NPCs delta
	added, removed := diffStringSlices(old.NPCs, new.NPCs)
	if len(added) > 0 || len(removed) > 0 {
		delta.NPCs = &NPCDelta{
			Added:   added,
			Removed: removed,
		}
	}

	// Buildings delta
	changedBuildings := make(map[string]bool)
	for buildingID, newOpen := range new.Buildings {
		oldOpen, exists := old.Buildings[buildingID]
		if !exists || oldOpen != newOpen {
			changedBuildings[buildingID] = newOpen
		}
	}
	// Check for buildings that existed in old but not in new (now nil/unknown)
	for buildingID := range old.Buildings {
		if _, exists := new.Buildings[buildingID]; !exists {
			// Building removed from view - don't include in delta
		}
	}
	if len(changedBuildings) > 0 {
		delta.Buildings = &BuildingDelta{StateChanged: changedBuildings}
	}

	// Inventory general slots delta
	generalChanges := make(map[int]InventorySlotDelta)
	for i := 0; i < 4; i++ {
		oldSlot := old.GeneralSlots[i]
		newSlot := new.GeneralSlots[i]

		if oldSlot != newSlot {
			slotDelta := InventorySlotDelta{}
			if newSlot.ItemID == "" {
				slotDelta.Empty = true
			} else {
				slotDelta.ItemID = &newSlot.ItemID
				slotDelta.Quantity = &newSlot.Quantity
				slotDelta.Made, slotDelta.Wear = nonZero(newSlot.Made), nonZero(newSlot.Wear)
				slotDelta.Burn, slotDelta.Uses = nonZero(newSlot.Burn), nonZero(newSlot.Uses)
			}
			generalChanges[i] = slotDelta
		}
	}
	if len(generalChanges) > 0 {
		if delta.Inventory == nil {
			delta.Inventory = &InventoryDelta{}
		}
		delta.Inventory.GeneralSlots = generalChanges
	}

	// Backpack slots delta
	backpackChanges := make(map[int]InventorySlotDelta)
	for i := 0; i < 20; i++ {
		oldSlot := old.BackpackSlots[i]
		newSlot := new.BackpackSlots[i]

		if oldSlot != newSlot {
			slotDelta := InventorySlotDelta{}
			if newSlot.ItemID == "" {
				slotDelta.Empty = true
			} else {
				slotDelta.ItemID = &newSlot.ItemID
				slotDelta.Quantity = &newSlot.Quantity
				slotDelta.Made, slotDelta.Wear = nonZero(newSlot.Made), nonZero(newSlot.Wear)
				slotDelta.Burn, slotDelta.Uses = nonZero(newSlot.Burn), nonZero(newSlot.Uses)
			}
			backpackChanges[i] = slotDelta
		}
	}
	if len(backpackChanges) > 0 {
		if delta.Inventory == nil {
			delta.Inventory = &InventoryDelta{}
		}
		delta.Inventory.BackpackSlots = backpackChanges
	}

	// Equipment delta
	equipmentChanges := make(map[string]*string)
	for slotName, newItemID := range new.EquipmentSlots {
		oldItemID, exists := old.EquipmentSlots[slotName]
		if !exists || oldItemID != newItemID {
			id := newItemID // Copy to avoid pointer to loop variable
			equipmentChanges[slotName] = &id
		}
	}
	// Check for equipment that was removed
	for slotName := range old.EquipmentSlots {
		if _, exists := new.EquipmentSlots[slotName]; !exists {
			equipmentChanges[slotName] = nil
		}
	}
	if len(equipmentChanges) > 0 {
		delta.Equipment = &EquipmentDelta{Changed: equipmentChanges}
	}

	// Location delta
	if old.City != new.City || old.District != new.District || old.Building != new.Building {
		delta.Location = &LocationDelta{}
		if old.City != new.City {
			delta.Location.City = &new.City
		}
		if old.District != new.District {
			delta.Location.District = &new.District
		}
		if old.Building != new.Building {
			delta.Location.Building = &new.Building
		}
	}

	// Effects delta
	addedEffects, removedEffects := diffStringSlices(old.ActiveEffects, new.ActiveEffects)
	if len(addedEffects) > 0 || len(removedEffects) > 0 {
		delta.Effects = &EffectsDelta{
			Added:   addedEffects,
			Removed: removedEffects,
		}
	}

	// Show ready delta (for Play Show button visibility)
	if old.ShowReady != new.ShowReady || old.ShowReadyBuilding != new.ShowReadyBuilding {
		delta.ShowReady = &ShowReadyDelta{
			IsReady:    new.ShowReady,
			BuildingID: new.ShowReadyBuilding,
		}
	}

	return delta
}

// diffStringSlices returns added and removed elements between two slices
func diffStringSlices(old, new []string) (added, removed []string) {
	oldMap := make(map[string]bool)
	newMap := make(map[string]bool)

	for _, item := range old {
		oldMap[item] = true
	}
	for _, item := range new {
		newMap[item] = true
	}

	for item := range newMap {
		if !oldMap[item] {
			added = append(added, item)
		}
	}

	for item := range oldMap {
		if !newMap[item] {
			removed = append(removed, item)
		}
	}

	return
}

// IsEmpty returns true if the delta contains no changes
func (d *Delta) IsEmpty() bool {
	if d == nil {
		return true
	}
	return d.Character == nil &&
		d.NPCs == nil &&
		d.Buildings == nil &&
		d.Inventory == nil &&
		d.Equipment == nil &&
		d.Location == nil &&
		d.Effects == nil &&
		d.ShowReady == nil
}

// GetNPCs returns NPC delta info for the DeltaProvider interface
func (d *Delta) GetNPCs() *types.NPCDeltaInfo {
	if d == nil || d.NPCs == nil {
		return nil
	}
	return &types.NPCDeltaInfo{
		Added:   d.NPCs.Added,
		Removed: d.NPCs.Removed,
	}
}

// ToMap converts a Delta to a map[string]any for JSON serialization
func (d *Delta) ToMap() map[string]any {
	if d == nil || d.IsEmpty() {
		return nil
	}

	result := make(map[string]any)

	if d.Character != nil {
		char := make(map[string]any)
		if d.Character.HP != nil {
			char["hp"] = *d.Character.HP
		}
		if d.Character.MaxHP != nil {
			char["max_hp"] = *d.Character.MaxHP
		}
		if d.Character.Mana != nil {
			char["mana"] = *d.Character.Mana
		}
		if d.Character.MaxMana != nil {
			char["max_mana"] = *d.Character.MaxMana
		}
		if d.Character.Fatigue != nil {
			char["fatigue"] = *d.Character.Fatigue
		}
		if d.Character.Hunger != nil {
			char["hunger"] = *d.Character.Hunger
		}
		if d.Character.Gold != nil {
			char["gold"] = *d.Character.Gold
		}
		if d.Character.XP != nil {
			char["xp"] = *d.Character.XP
		}
		if d.Character.TimeOfDay != nil {
			char["time_of_day"] = *d.Character.TimeOfDay
		}
		if d.Character.CurrentDay != nil {
			char["current_day"] = *d.Character.CurrentDay
		}
		if len(char) > 0 {
			result["character"] = char
		}
	}

	if d.NPCs != nil {
		npcs := make(map[string]any)
		if len(d.NPCs.Added) > 0 {
			npcs["added"] = d.NPCs.Added
		}
		if len(d.NPCs.Removed) > 0 {
			npcs["removed"] = d.NPCs.Removed
		}
		if len(npcs) > 0 {
			result["npcs"] = npcs
		}
	}

	if d.Buildings != nil && len(d.Buildings.StateChanged) > 0 {
		result["buildings"] = map[string]any{
			"state_changed": d.Buildings.StateChanged,
		}
	}

	if d.Location != nil {
		loc := make(map[string]any)
		if d.Location.City != nil {
			loc["city"] = *d.Location.City
		}
		if d.Location.District != nil {
			loc["district"] = *d.Location.District
		}
		if d.Location.Building != nil {
			loc["building"] = *d.Location.Building
		}
		if len(loc) > 0 {
			result["location"] = loc
		}
	}

	if d.Effects != nil {
		effects := make(map[string]any)
		if len(d.Effects.Added) > 0 {
			effects["added"] = d.Effects.Added
		}
		if len(d.Effects.Removed) > 0 {
			effects["removed"] = d.Effects.Removed
		}
		if len(d.Effects.Updated) > 0 {
			effects["updated"] = d.Effects.Updated
		}
		if len(effects) > 0 {
			result["effects"] = effects
		}
	}

	// Inventory and equipment deltas are more complex - use struct directly
	if d.Inventory != nil {
		inv := make(map[string]any)
		if d.Inventory.GeneralSlots != nil && len(d.Inventory.GeneralSlots) > 0 {
			general := make(map[string]any)
			for idx, slot := range d.Inventory.GeneralSlots {
				slotData := make(map[string]any)
				if slot.Empty {
					slotData["empty"] = true
				} else {
					if slot.ItemID != nil {
						slotData["item_id"] = *slot.ItemID
					}
					if slot.Quantity != nil {
						slotData["quantity"] = *slot.Quantity
					}
					for key, v := range map[string]*int{"t": slot.Made, "w": slot.Wear, "b": slot.Burn, "u": slot.Uses} {
						if v != nil {
							slotData[key] = *v
						}
					}
				}
				general[strconv.Itoa(idx)] = slotData
			}
			inv["general_slots"] = general
		}
		if d.Inventory.BackpackSlots != nil && len(d.Inventory.BackpackSlots) > 0 {
			backpack := make(map[string]any)
			for idx, slot := range d.Inventory.BackpackSlots {
				slotData := make(map[string]any)
				if slot.Empty {
					slotData["empty"] = true
				} else {
					if slot.ItemID != nil {
						slotData["item_id"] = *slot.ItemID
					}
					if slot.Quantity != nil {
						slotData["quantity"] = *slot.Quantity
					}
					for key, v := range map[string]*int{"t": slot.Made, "w": slot.Wear, "b": slot.Burn, "u": slot.Uses} {
						if v != nil {
							slotData[key] = *v
						}
					}
				}
				backpack[strconv.Itoa(idx)] = slotData
			}
			inv["backpack_slots"] = backpack
		}
		if len(inv) > 0 {
			result["inventory"] = inv
		}
	}

	if d.Equipment != nil && len(d.Equipment.Changed) > 0 {
		result["equipment"] = map[string]any{
			"changed": d.Equipment.Changed,
		}
	}

	if d.ShowReady != nil {
		showReady := map[string]any{
			"is_ready": d.ShowReady.IsReady,
		}
		if d.ShowReady.BuildingID != "" {
			showReady["building_id"] = d.ShowReady.BuildingID
		}
		result["show_ready"] = showReady
	}

	return result
}

// slotInt reads a numeric slot field, whether it came from JSON (float64) or
// was set in Go (int).
func slotInt(slot map[string]interface{}, key string) int {
	switch v := slot[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// nonZero points at n, or is nil for 0 (a condition field the slot doesn't have).
func nonZero(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}
