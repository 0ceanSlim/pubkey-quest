package world

import (
	"log"
	"sync"
	"time"
)

// MerchantInventoryItem represents current stock for a single item
type MerchantInventoryItem struct {
	ItemID       string `json:"item_id"`
	CurrentStock int    `json:"current_stock"`
	MaxStock     int    `json:"max_stock"`

	// Optional per-item restock: RestockRate units come back every
	// RestockMinutes (real minutes; 10 = one game day). 0 = the item refills
	// with the whole shop on the shop's item_restock_interval instead.
	RestockRate    int       `json:"restock_rate,omitempty"`
	RestockMinutes int       `json:"restock_minutes,omitempty"`
	LastRestock    time.Time `json:"last_restock,omitempty"`
}

// tickItemRestock adds back stock for an item on its own timer.
func tickItemRestock(item *MerchantInventoryItem, now time.Time) {
	if item.RestockMinutes <= 0 {
		return
	}
	periods := int(now.Sub(item.LastRestock).Minutes()) / item.RestockMinutes
	if periods <= 0 {
		return
	}
	rate := item.RestockRate
	if rate <= 0 {
		rate = 1
	}
	item.CurrentStock = min(item.MaxStock, item.CurrentStock+periods*rate)
	item.LastRestock = item.LastRestock.Add(time.Duration(periods*item.RestockMinutes) * time.Minute)
}

// MerchantState represents the current state of a merchant for a specific player
type MerchantState struct {
	MerchantID            string                            `json:"merchant_id"`
	CurrentGold           int                               `json:"current_gold"`
	StartingGold          int                               `json:"starting_gold"`  // For gold regen cap
	GoldRegenRate         int                               `json:"gold_regen_rate"` // Gold restored per interval
	Inventory             map[string]*MerchantInventoryItem `json:"inventory"`       // item_id -> stock info
	LastItemRestock       time.Time                         `json:"last_item_restock"`
	LastGoldRestock       time.Time                         `json:"last_gold_restock"`
	LastGoldRegen         time.Time                         `json:"last_gold_regen"` // Track gradual gold regen separately
	ItemRestockInterval   int                               `json:"item_restock_interval"` // Minutes (default 10)
	GoldRestockInterval   int                               `json:"gold_restock_interval"` // Minutes (default 30)
	GoldRegenInterval     int                               `json:"gold_regen_interval"`   // Minutes (default 10 for "daily")
}

// MerchantStateManager manages merchant states per player
type MerchantStateManager struct {
	// Per-player merchant states: playerNpub -> merchantID -> MerchantState
	states map[string]map[string]*MerchantState
	mu     sync.RWMutex

	// Future: Global item tracking (specific items shared across all players)
	// globalItems map[string]int // item_id -> global_stock
}

var merchantManager *MerchantStateManager
var merchantOnce sync.Once

// GetMerchantManager returns the singleton merchant state manager
func GetMerchantManager() *MerchantStateManager {
	merchantOnce.Do(func() {
		merchantManager = &MerchantStateManager{
			states: make(map[string]map[string]*MerchantState),
		}
		log.Println("🏪 Merchant State Manager initialized")
	})
	return merchantManager
}

// GetMerchantState gets or initializes merchant state for a player
// Returns the state and whether a restock occurred
func (m *MerchantStateManager) GetMerchantState(npub, merchantID string, initialGold int, goldRegenRate int, initialInventory []MerchantInventoryItem, itemRestockInterval int, goldRestockInterval int, goldRegenInterval int) (*MerchantState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize player's merchant map if needed
	if m.states[npub] == nil {
		m.states[npub] = make(map[string]*MerchantState)
	}

	// Check if merchant state exists for this player
	state, exists := m.states[npub][merchantID]
	restocked := false

	if !exists {
		// First time seeing this merchant for this player - initialize
		log.Printf("🆕 Initializing merchant state: %s for player %s", merchantID, npub[:12])
		state = &MerchantState{
			MerchantID:          merchantID,
			CurrentGold:         initialGold,
			StartingGold:        initialGold,
			GoldRegenRate:       goldRegenRate,
			Inventory:           make(map[string]*MerchantInventoryItem),
			LastItemRestock:     time.Now(),
			LastGoldRestock:     time.Now(),
			LastGoldRegen:       time.Now(),
			ItemRestockInterval: itemRestockInterval,
			GoldRestockInterval: goldRestockInterval,
			GoldRegenInterval:   goldRegenInterval,
		}

		// Copy initial inventory
		for _, item := range initialInventory {
			copied := item
			copied.LastRestock = time.Now()
			state.Inventory[item.ItemID] = &copied
		}

		m.states[npub][merchantID] = state
	} else {
		// Check if gradual gold regen is due (based on gold_regen_interval, typically 10 min for "daily")
		minutesSinceGoldRegen := time.Since(state.LastGoldRegen).Minutes()
		if minutesSinceGoldRegen >= float64(state.GoldRegenInterval) {
			// Only regenerate if below starting gold
			if state.CurrentGold < state.StartingGold {
				oldGold := state.CurrentGold
				state.CurrentGold += state.GoldRegenRate
				// Cap at starting gold
				if state.CurrentGold > state.StartingGold {
					state.CurrentGold = state.StartingGold
				}
				state.LastGoldRegen = time.Now()
				log.Printf("💰 Gold regenerated (gradual): %s for player %s (%d -> %d, +%d)",
					merchantID, npub[:12], oldGold, state.CurrentGold, state.CurrentGold-oldGold)
			}
		}

		// Check if full gold restock is due (default 30 minutes)
		minutesSinceGoldRestock := time.Since(state.LastGoldRestock).Minutes()
		if minutesSinceGoldRestock >= float64(state.GoldRestockInterval) {
			if state.CurrentGold < state.StartingGold {
				oldGold := state.CurrentGold
				state.CurrentGold = state.StartingGold
				state.LastGoldRestock = time.Now()
				log.Printf("💰 Gold restocked (full): %s for player %s (%d -> %d)",
					merchantID, npub[:12], oldGold, state.CurrentGold)
			}
		}

		// Items with their own restock timer tick independently.
		now := time.Now()
		for _, item := range state.Inventory {
			tickItemRestock(item, now)
		}

		// Check if inventory restock is due (default 10 minutes)
		minutesSinceItemRestock := time.Since(state.LastItemRestock).Minutes()
		if minutesSinceItemRestock >= float64(state.ItemRestockInterval) {
			log.Printf("🔄 Restocking merchant items: %s for player %s (%.0f min since last restock)", merchantID, npub[:12], minutesSinceItemRestock)
			m.restockMerchant(state, initialInventory)
			restocked = true
		}
	}

	return state, restocked
}

// restockMerchant restores merchant inventory only (gold regen handled separately)
// Only restocks items that are below max stock
func (m *MerchantStateManager) restockMerchant(state *MerchantState, restoreInventory []MerchantInventoryItem) {
	restockedItems := 0

	// Restore inventory to max stock (only if below max)
	for _, item := range restoreInventory {
		if invItem, exists := state.Inventory[item.ItemID]; exists {
			// Items on their own timer are left to tickItemRestock.
			if invItem.RestockMinutes > 0 {
				continue
			}
			// Only restock if below max stock
			if invItem.CurrentStock < invItem.MaxStock {
				invItem.CurrentStock = invItem.MaxStock
				restockedItems++
			}
		} else {
			// Item was added to shop config since last time - add it
			added := item
			added.LastRestock = time.Now()
			state.Inventory[item.ItemID] = &added
			restockedItems++
		}
	}

	state.LastItemRestock = time.Now()
	log.Printf("✅ Merchant inventory restocked: %s (%d items)", state.MerchantID, restockedItems)
}

// UpdateMerchantInventory updates stock and gold after a transaction
func (m *MerchantStateManager) UpdateMerchantInventory(npub, merchantID, itemID string, quantityChange int, goldChange int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[npub][merchantID]
	if !exists {
		log.Printf("❌ Merchant state not found: %s for player %s", merchantID, npub[:12])
		return nil // Silently fail - will reinitialize next time
	}

	// Update stock
	if invItem, exists := state.Inventory[itemID]; exists {
		invItem.CurrentStock += quantityChange
		if invItem.CurrentStock < 0 {
			invItem.CurrentStock = 0
		}
	}

	// Update gold
	state.CurrentGold += goldChange

	log.Printf("💰 Merchant updated: %s | Item: %s (%+d) | Gold: %d (%+d)", merchantID, itemID, quantityChange, state.CurrentGold, goldChange)
	return nil
}

// GetTimeUntilRestock returns minutes until next item restock
func (m *MerchantStateManager) GetTimeUntilRestock(npub, merchantID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.states[npub] == nil || m.states[npub][merchantID] == nil {
		return 0
	}

	state := m.states[npub][merchantID]
	elapsed := time.Since(state.LastItemRestock).Minutes()
	remaining := float64(state.ItemRestockInterval) - elapsed

	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetTimeUntilGoldRestock returns minutes until next gold restock
func (m *MerchantStateManager) GetTimeUntilGoldRestock(npub, merchantID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.states[npub] == nil || m.states[npub][merchantID] == nil {
		return 0
	}

	state := m.states[npub][merchantID]
	elapsed := time.Since(state.LastGoldRestock).Minutes()
	remaining := float64(state.GoldRestockInterval) - elapsed

	if remaining < 0 {
		return 0
	}
	return remaining
}

// CleanupPlayerStates removes merchant states for a player (called when player logs out)
func (m *MerchantStateManager) CleanupPlayerStates(npub string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, npub)
	log.Printf("🧹 Cleaned up merchant states for player: %s", npub[:12])
}

// GetAllMerchantStatesForPlayer returns all merchant states for debugging
func (m *MerchantStateManager) GetAllMerchantStatesForPlayer(npub string) map[string]*MerchantState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.states[npub] == nil {
		return nil
	}

	// Return a copy to prevent external modification
	stateCopy := make(map[string]*MerchantState)
	for k, v := range m.states[npub] {
		stateCopy[k] = v
	}
	return stateCopy
}
