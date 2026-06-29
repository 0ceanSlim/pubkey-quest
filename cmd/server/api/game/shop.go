package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"pubkey-quest/cmd/server/api/character"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/shop"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
	"pubkey-quest/cmd/server/world"
)

// parseIntervalToMinutes delegates to game/shop package
func parseIntervalToMinutes(interval string) int {
	return shop.ParseIntervalToMinutes(interval)
}

// getCharismaFromSession returns the player's EFFECTIVE charisma — base charisma
// with active ability-modifier effects folded in (effects.EffectiveStats, the
// same value checks roll against). This is what shop pricing keys off, so a CHA
// buff/debuff shifts buy and sell prices just like it shifts skill checks.
func getCharismaFromSession(npub, saveID string) int {
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		log.Printf("⚠️ Failed to get session for charisma lookup: %v", err)
		return 10 // Default charisma
	}

	eff := effects.EffectiveStats(&sess.SaveData)
	switch cha := eff["charisma"].(type) {
	case float64:
		return int(cha)
	case int:
		return cha
	}
	return 10 // Default charisma if not found
}

// calculateBuyPrice delegates to game/shop package
func calculateBuyPrice(basePrice int, shopConfig types.ShopConfig, charisma int) int {
	return shop.CalculateBuyPrice(basePrice, shopConfig, charisma)
}

// calculateSellPrice delegates to game/shop package
func calculateSellPrice(basePrice int, shopConfig types.ShopConfig, charisma int) int {
	return shop.CalculateSellPrice(basePrice, shopConfig, charisma)
}

// ShopDataResponse represents the shop data returned by GET /api/shop/{merchant_id}
// swagger:model ShopDataResponse
type ShopDataResponse struct {
	MerchantID           string                   `json:"merchant_id" example:"blacksmith-john"`
	MerchantName         string                   `json:"merchant_name" example:"John"`
	ShopType             string                   `json:"shop_type" example:"general"`
	BuysItems            bool                     `json:"buys_items" example:"true"`
	CurrentGold          int                      `json:"current_gold" example:"500"`
	MaxGold              int                      `json:"max_gold" example:"1000"`
	BuyPriceMultiplier   float64                  `json:"buy_price_multiplier" example:"1.2"`
	SellPriceMultiplier  float64                  `json:"sell_price_multiplier" example:"0.5"`
	Inventory            []map[string]interface{} `json:"inventory"`
	ItemRestockInterval  int                      `json:"item_restock_interval" example:"10"`
	GoldRestockInterval  int                      `json:"gold_restock_interval" example:"30"`
	TimeUntilItemRestock int                      `json:"time_until_item_restock" example:"5"`
	TimeUntilGoldRestock int                      `json:"time_until_gold_restock" example:"15"`
	JustRestocked        bool                     `json:"just_restocked" example:"false"`
}

// ShopTransactionResponse represents the response from buy/sell operations
// swagger:model ShopTransactionResponse
type ShopTransactionResponse struct {
	Success     bool   `json:"success" example:"true"`
	Message     string `json:"message" example:"Bought 1x Longsword for 15g"`
	GoldSpent   int    `json:"gold_spent,omitempty" example:"15"`
	GoldEarned  int    `json:"gold_earned,omitempty" example:"7"`
	NewGold     int    `json:"new_gold" example:"85"`
	ItemsBought int    `json:"items_bought,omitempty" example:"1"`
	ItemsSold   int    `json:"items_sold,omitempty" example:"1"`
	Error       string `json:"error,omitempty"`
}

// ShopHandler godoc
// @Summary      Shop operations
// @Description  GET /{merchant_id}: Get shop data with inventory and prices. POST /buy: Buy items. POST /sell: Sell items.
// @Tags         Shop
// @Accept       json
// @Produce      json
// @Param        merchant_id  path      string                  false  "Merchant ID (for GET)"
// @Param        npub         query     string                  false  "Nostr public key (for GET)"
// @Param        save_id      query     string                  false  "Save file ID (for GET)"
// @Param        transaction  body      types.ShopTransaction   false  "Transaction data (for POST)"
// @Success      200          {object}  ShopDataResponse        "Shop data (GET)"
// @Success      200          {object}  ShopTransactionResponse "Transaction result (POST)"
// @Failure      400          {object}  map[string]interface{}  "Invalid request"
// @Failure      404          {object}  map[string]interface{}  "Merchant or session not found"
// @Failure      405          {string}  string                  "Method not allowed"
// @Router       /shop/{merchant_id} [get]
// @Router       /shop/buy [post]
// @Router       /shop/sell [post]
func ShopHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/shop/"), "/")

	switch r.Method {
	case "GET":
		if len(pathParts) > 0 && pathParts[0] != "" {
			handleGetShop(w, r, pathParts[0])
		} else {
			http.Error(w, "Missing merchant ID", http.StatusBadRequest)
		}
	case "POST":
		if len(pathParts) > 0 {
			switch pathParts[0] {
			case "buy":
				handleBuyFromShop(w, r)
			case "sell":
				handleSellToShop(w, r)
			default:
				http.Error(w, "Invalid shop action", http.StatusBadRequest)
			}
		} else {
			http.Error(w, "Missing action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Get shop data including inventory with prices
func handleGetShop(w http.ResponseWriter, r *http.Request, merchantID string) {
	// Get npub and saveID from query parameters
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" {
		http.Error(w, "Missing npub parameter", http.StatusBadRequest)
		return
	}
	if saveID == "" {
		http.Error(w, "Missing save_id parameter", http.StatusBadRequest)
		return
	}

	log.Printf("📂 Loading shop data for merchant: %s (player: %s)", merchantID, npub[:12])

	// Get player charisma from in-memory session state
	playerCharisma := getCharismaFromSession(npub, saveID)

	// Get NPC data from database
	npcData, err := db.GetNPCByID(merchantID)
	if err != nil {
		log.Printf("❌ Error loading NPC: %v", err)
		http.Error(w, "Merchant not found", http.StatusNotFound)
		return
	}

	// Parse shop config - it's already in the ShopConfig field
	configJSON, err := json.Marshal(npcData.ShopConfig)
	if err != nil {
		log.Printf("❌ Error marshaling shop config: %v", err)
		http.Error(w, "Invalid shop configuration", http.StatusInternalServerError)
		return
	}

	var shopConfig types.ShopConfig
	if err := json.Unmarshal(configJSON, &shopConfig); err != nil {
		log.Printf("❌ Error parsing shop config: %v", err)
		http.Error(w, "Invalid shop configuration", http.StatusInternalServerError)
		return
	}

	// Initialize merchant inventory items for state manager
	initialInventory := make([]world.MerchantInventoryItem, 0)
	for _, invItem := range shopConfig.Inventory {
		initialInventory = append(initialInventory, world.MerchantInventoryItem{
			ItemID:       invItem.ItemID,
			CurrentStock: invItem.Stock,
			MaxStock:     invItem.MaxStock,
		})
	}

	// Parse intervals from JSON
	itemRestockInterval := 10 // Default: 10 minutes
	if shopConfig.ItemRestockInterval > 0 {
		itemRestockInterval = shopConfig.ItemRestockInterval
	}

	goldRestockInterval := 30 // Default: 30 minutes
	if shopConfig.GoldRestockInterval > 0 {
		goldRestockInterval = shopConfig.GoldRestockInterval
	}

	goldRegenInterval := 10 // Default: 10 minutes (1 game day)
	if shopConfig.GoldRegenInterval != "" {
		goldRegenInterval = parseIntervalToMinutes(shopConfig.GoldRegenInterval)
	}

	merchantManager := world.GetMerchantManager()
	merchantState, restocked := merchantManager.GetMerchantState(npub, merchantID, shopConfig.StartingGold, shopConfig.GoldRegenRate, initialInventory, itemRestockInterval, goldRestockInterval, goldRegenInterval)

	// Get item prices and current stock from merchant state
	itemsWithPrices := make([]map[string]any, 0)
	for _, invItem := range shopConfig.Inventory {
		item, err := db.GetItemByID(invItem.ItemID)
		if err != nil {
			log.Printf("⚠️ Item not found: %s", invItem.ItemID)
			continue
		}

		// Get current stock from merchant state
		currentStock := 0
		if stateItem, exists := merchantState.Inventory[invItem.ItemID]; exists {
			currentStock = stateItem.CurrentStock
		}

		// Calculate buy/sell prices with shop type and charisma modifiers
		basePrice := item.Value
		buyPrice := calculateBuyPrice(basePrice, shopConfig, playerCharisma)   // Player pays this (affected by shop type + charisma)
		sellPrice := calculateSellPrice(basePrice, shopConfig, playerCharisma) // Merchant pays player this (affected by charisma only)

		itemsWithPrices = append(itemsWithPrices, map[string]any{
			"item_id":     invItem.ItemID,
			"name":        item.Name,
			"description": item.Description,
			"type":        item.Type,
			"value":       basePrice,
			"buy_price":   buyPrice,     // What player pays to buy
			"sell_price":  sellPrice,    // What player gets when selling
			"stock":       currentStock, // Current stock from merchant state
			"max_stock":   invItem.MaxStock,
		})
	}

	// Calculate time until next restock
	timeUntilItemRestock := merchantManager.GetTimeUntilRestock(npub, merchantID)
	timeUntilGoldRestock := merchantManager.GetTimeUntilGoldRestock(npub, merchantID)

	response := map[string]any{
		"merchant_id":             merchantID,
		"merchant_name":           npcData.Name,
		"shop_type":               shopConfig.ShopType,
		"buys_items":              shopConfig.BuysItems,
		"current_gold":            merchantState.CurrentGold, // Current gold from state
		"max_gold":                shopConfig.MaxGold,
		"buy_price_multiplier":    shopConfig.BuyPriceMultiplier,
		"sell_price_multiplier":   shopConfig.SellPriceMultiplier,
		"inventory":               itemsWithPrices,
		"item_restock_interval":   itemRestockInterval,       // Minutes between item restocks
		"gold_restock_interval":   goldRestockInterval,       // Minutes between gold restocks
		"time_until_item_restock": int(timeUntilItemRestock), // Minutes until next item restock
		"time_until_gold_restock": int(timeUntilGoldRestock), // Minutes until next gold restock
		"just_restocked":          restocked,                 // Whether merchant just restocked items
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("✅ Loaded shop data for merchant: %s", merchantID)
}

// Buy items from shop
func handleBuyFromShop(w http.ResponseWriter, r *http.Request) {
	var transaction types.ShopTransaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Invalid transaction data",
		})
		return
	}

	log.Printf("🛒 Processing buy: %s buying %dx %s from %s", transaction.Npub, transaction.Quantity, transaction.ItemID, transaction.MerchantID)

	sessionMgr := session.GetSessionManager()

	// Get session from memory (not disk!)
	session, err := sessionMgr.GetSession(transaction.Npub, transaction.SaveID)
	if err != nil {
		log.Printf("❌ Session not found: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Session not found",
		})
		return
	}

	save := &session.SaveData

	// Get merchant data
	npcData, err := db.GetNPCByID(transaction.MerchantID)
	if err != nil {
		log.Printf("❌ Error loading merchant: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Merchant not found",
		})
		return
	}

	// Parse shop config
	configJSON, _ := json.Marshal(npcData.ShopConfig)
	var shopConfig types.ShopConfig
	json.Unmarshal(configJSON, &shopConfig)

	// Find item in shop inventory config
	var shopItem *types.ShopInventoryItem
	for i := range shopConfig.Inventory {
		if shopConfig.Inventory[i].ItemID == transaction.ItemID {
			shopItem = &shopConfig.Inventory[i]
			break
		}
	}

	if shopItem == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Item not in shop inventory",
		})
		return
	}

	// Get merchant state to check current stock
	initialInventory := make([]world.MerchantInventoryItem, 0)
	for _, invItem := range shopConfig.Inventory {
		initialInventory = append(initialInventory, world.MerchantInventoryItem{
			ItemID:       invItem.ItemID,
			CurrentStock: invItem.Stock,
			MaxStock:     invItem.MaxStock,
		})
	}

	itemRestockInterval := 10
	if shopConfig.ItemRestockInterval > 0 {
		itemRestockInterval = shopConfig.ItemRestockInterval
	}

	goldRestockInterval := 30
	if shopConfig.GoldRestockInterval > 0 {
		goldRestockInterval = shopConfig.GoldRestockInterval
	}

	goldRegenInterval := 10
	if shopConfig.GoldRegenInterval != "" {
		goldRegenInterval = parseIntervalToMinutes(shopConfig.GoldRegenInterval)
	}

	merchantManager := world.GetMerchantManager()
	merchantState, _ := merchantManager.GetMerchantState(transaction.Npub, transaction.MerchantID, shopConfig.StartingGold, shopConfig.GoldRegenRate, initialInventory, itemRestockInterval, goldRestockInterval, goldRegenInterval)

	// Check current stock from merchant state
	currentStock := 0
	if stateItem, exists := merchantState.Inventory[transaction.ItemID]; exists {
		currentStock = stateItem.CurrentStock
	}

	if currentStock < transaction.Quantity {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Not enough stock (available: %d)", currentStock),
		})
		return
	}

	// Get item data for price
	item, err := db.GetItemByID(transaction.ItemID)
	if err != nil {
		log.Printf("❌ Error loading item: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Item not found",
		})
		return
	}

	// Calculate total cost with shop type and charisma modifiers (from session state)
	playerCharisma := getCharismaFromSession(transaction.Npub, transaction.SaveID)
	buyPrice := calculateBuyPrice(item.Value, shopConfig, playerCharisma)
	totalCost := buyPrice * transaction.Quantity
	log.Printf("💰 Price calculation: base=%dg, shop_type=%s, CHA=%d, final_price=%dg",
		item.Value, shopConfig.ShopType, playerCharisma, buyPrice)

	// Check player gold (using existing helper function)
	playerGold := gameutil.GetGoldQuantity(save)

	if playerGold < totalCost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Not enough gold (need %d, have %d)", totalCost, playerGold),
		})
		return
	}

	// Try to add items to inventory first to see how many fit
	itemsAdded, err := addItemToInventory(save, transaction.ItemID, transaction.Quantity)
	if err != nil && itemsAdded == 0 {
		// No items could be added
		log.Printf("❌ Error adding item to inventory: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
			"message": "No room in inventory",
		})
		return
	}

	// Calculate actual cost for items that fit
	actualCost := buyPrice * itemsAdded

	// Deduct gold for items that were added
	if !gameutil.DeductGold(save, actualCost) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Failed to deduct gold",
		})
		return
	}

	// Update encumbrance effects after buying items
	status.UpdateEncumbrancePenaltyEffects(save)

	// Update session in memory (not disk!)
	if err := sessionMgr.UpdateSession(transaction.Npub, transaction.SaveID, session.SaveData); err != nil {
		log.Printf("❌ Failed to update session: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Failed to update session",
		})
		return
	}

	// Update merchant state (deduct stock, add gold from player)
	merchantManager.UpdateMerchantInventory(transaction.Npub, transaction.MerchantID, transaction.ItemID, -itemsAdded, actualCost)

	// Build response message
	var message string
	if itemsAdded < transaction.Quantity {
		message = fmt.Sprintf("Bought %dx %s for %dg (inventory full - %d didn't fit)", itemsAdded, item.Name, actualCost, transaction.Quantity-itemsAdded)
		log.Printf("⚠️ Partial buy: %s bought %dx %s for %dg (%d didn't fit - inventory full)", transaction.Npub, itemsAdded, transaction.ItemID, actualCost, transaction.Quantity-itemsAdded)
	} else {
		message = fmt.Sprintf("Bought %dx %s for %dg", itemsAdded, item.Name, actualCost)
		log.Printf("✅ Buy successful: %s bought %dx %s for %dg (IN MEMORY)", transaction.Npub, itemsAdded, transaction.ItemID, actualCost)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"message":      message,
		"gold_spent":   actualCost,
		"new_gold":     playerGold - actualCost,
		"items_bought": itemsAdded,
	})
}

// Sell items to shop
func handleSellToShop(w http.ResponseWriter, r *http.Request) {
	var transaction types.ShopTransaction
	if err := json.NewDecoder(r.Body).Decode(&transaction); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Invalid transaction data",
		})
		return
	}

	log.Printf("💰 Processing sell: %s selling %dx %s to %s", transaction.Npub, transaction.Quantity, transaction.ItemID, transaction.MerchantID)

	sessionMgr := session.GetSessionManager()

	// Get session from memory (not disk!)
	session, err := sessionMgr.GetSession(transaction.Npub, transaction.SaveID)
	if err != nil {
		log.Printf("❌ Session not found: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Session not found",
		})
		return
	}

	save := &session.SaveData

	// Get merchant data
	npcData, err := db.GetNPCByID(transaction.MerchantID)
	if err != nil {
		log.Printf("❌ Error loading merchant: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Merchant not found",
		})
		return
	}

	// Parse shop config
	configJSON, _ := json.Marshal(npcData.ShopConfig)
	var shopConfig types.ShopConfig
	json.Unmarshal(configJSON, &shopConfig)

	// Check if merchant buys items
	if !shopConfig.BuysItems {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "This merchant doesn't buy items",
		})
		return
	}

	// Specialty shops only buy items they stock
	if shopConfig.ShopType == "specialty" {
		itemInStock := false
		for _, invItem := range shopConfig.Inventory {
			if invItem.ItemID == transaction.ItemID {
				itemInStock = true
				break
			}
		}
		if !itemInStock {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "This specialty shop doesn't buy that type of item",
			})
			return
		}
	}

	// Get item data for price
	item, err := db.GetItemByID(transaction.ItemID)
	if err != nil {
		log.Printf("❌ Error loading item: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Item not found",
		})
		return
	}

	// Calculate total value with charisma modifier (from session state)
	playerCharisma := getCharismaFromSession(transaction.Npub, transaction.SaveID)
	sellPrice := calculateSellPrice(item.Value, shopConfig, playerCharisma)
	totalValue := sellPrice * transaction.Quantity
	log.Printf("💰 Sell price calculation: base=%dg, shop_type=%s, CHA=%d, merchant_pays=%dg",
		item.Value, shopConfig.ShopType, playerCharisma, sellPrice)

	// Get merchant state to check current gold
	initialInventory := make([]world.MerchantInventoryItem, 0)
	for _, invItem := range shopConfig.Inventory {
		initialInventory = append(initialInventory, world.MerchantInventoryItem{
			ItemID:       invItem.ItemID,
			CurrentStock: invItem.Stock,
			MaxStock:     invItem.MaxStock,
		})
	}

	itemRestockInterval := 10
	if shopConfig.ItemRestockInterval > 0 {
		itemRestockInterval = shopConfig.ItemRestockInterval
	}

	goldRestockInterval := 30
	if shopConfig.GoldRestockInterval > 0 {
		goldRestockInterval = shopConfig.GoldRestockInterval
	}

	goldRegenInterval := 10
	if shopConfig.GoldRegenInterval != "" {
		goldRegenInterval = parseIntervalToMinutes(shopConfig.GoldRegenInterval)
	}

	merchantManager := world.GetMerchantManager()
	merchantState, _ := merchantManager.GetMerchantState(transaction.Npub, transaction.MerchantID, shopConfig.StartingGold, shopConfig.GoldRegenRate, initialInventory, itemRestockInterval, goldRestockInterval, goldRegenInterval)

	// Check merchant gold from state
	merchantGold := merchantState.CurrentGold
	if merchantGold < totalValue {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Merchant doesn't have enough gold (needs %d, has %d)", totalValue, merchantGold),
		})
		return
	}

	// NOTE: Items are already removed from inventory when added to sell staging
	// Frontend removes items via remove_from_inventory action, so we don't remove them here
	log.Printf("ℹ️ Items already removed from inventory during sell staging")

	// Add gold to player (using existing helper function)
	if err := character.AddGoldToInventory(save.Inventory, totalValue); err != nil {
		log.Printf("❌ Error adding gold to inventory: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Failed to add gold",
		})
		return
	}

	playerGold := gameutil.GetGoldQuantity(save)

	// Update encumbrance effects after selling items
	status.UpdateEncumbrancePenaltyEffects(save)

	// Update session in memory (not disk!)
	if err := sessionMgr.UpdateSession(transaction.Npub, transaction.SaveID, session.SaveData); err != nil {
		log.Printf("❌ Failed to update session: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Failed to update session",
		})
		return
	}

	// Update merchant state (add stock, deduct gold paid to player)
	merchantManager.UpdateMerchantInventory(transaction.Npub, transaction.MerchantID, transaction.ItemID, transaction.Quantity, -totalValue)

	log.Printf("✅ Sell successful: %s sold %dx %s for %dg (IN MEMORY)", transaction.Npub, transaction.Quantity, transaction.ItemID, totalValue)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":     true,
		"message":     fmt.Sprintf("Sold %dx %s for %dg", transaction.Quantity, item.Name, totalValue),
		"gold_earned": totalValue,
		"new_gold":    playerGold + totalValue,
		"items_sold":  transaction.Quantity,
	})
}

// addItemToInventory delegates to game/inventory package
func addItemToInventory(save *SaveFile, itemID string, quantity int) (int, error) {
	return inventory.AddItemToInventory(save, itemID, quantity)
}
