// Package shop handles shop-related game logic including pricing calculations
package shop

import (
	"log"
	"strconv"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// ParseIntervalToMinutes converts interval strings to minutes
// Supports named intervals ("daily", "hourly", "weekly") or direct numeric strings ("60")
func ParseIntervalToMinutes(interval string) int {
	switch interval {
	case "daily":
		return 10 // 10 minutes real-time = 1 game day
	case "hourly":
		return 1 // 1 minute real-time
	case "weekly":
		return 70 // 70 minutes real-time = 1 game week
	default:
		// Try parsing as direct minutes (e.g., "60" for 60 minutes)
		if minutes, err := strconv.Atoi(interval); err == nil && minutes > 0 {
			return minutes
		}
		return 10 // Default to daily if parsing fails
	}
}

// CalculateBuyPrice calculates the price a player pays when buying from a merchant
// Uses pricing rules from database (shop-pricing.json)
func CalculateBuyPrice(basePrice int, shopConfig types.ShopConfig, charisma int) int {
	// Get pricing rules from database
	rules, err := db.GetShopPricingRules()
	if err != nil {
		log.Printf("⚠️ Failed to load shop pricing rules, using defaults: %v", err)
		// Fallback to hard-coded defaults if database fails
		shopBaseMult := 1.625
		charismaRate := 0.0625
		if shopConfig.ShopType == "specialty" {
			shopBaseMult = 1.675
		}
		charismaDiscount := float64(charisma-10) * charismaRate
		finalMultiplier := shopBaseMult - charismaDiscount
		if finalMultiplier < 0.5 {
			finalMultiplier = 0.5
		}
		result := int(float64(basePrice)*finalMultiplier + 0.5)
		if result < 1 {
			result = 1
		}
		return result
	}

	// Get appropriate pricing based on shop type
	var shopBaseMult float64
	var charismaRate float64
	charismaBase := rules.CharismaBase
	if charismaBase == 0 {
		charismaBase = 10 // Default if not set
	}

	if shopConfig.ShopType == "specialty" {
		shopBaseMult = rules.BuyPricing.Specialty.BaseMultiplier
		charismaRate = rules.BuyPricing.Specialty.CharismaRate
	} else {
		shopBaseMult = rules.BuyPricing.General.BaseMultiplier
		charismaRate = rules.BuyPricing.General.CharismaRate
	}

	// Formula: base_value × (base_multiplier - (CHA - charisma_base) × charisma_rate)
	charismaDiscount := float64(charisma-charismaBase) * charismaRate
	finalMultiplier := shopBaseMult - charismaDiscount

	// Ensure multiplier doesn't go below a minimum (prevent negative/zero prices)
	if finalMultiplier < 0.5 {
		finalMultiplier = 0.5
	}

	// Calculate final price
	finalPrice := float64(basePrice) * finalMultiplier

	// Round to nearest int, minimum 1 gold
	result := int(finalPrice + 0.5)
	if result < 1 {
		result = 1
	}

	return result
}

// CalculateSellPrice calculates the price a merchant pays when buying from the
// player (shop-pricing.json), never more than the item would cost to buy back
// at any shop: with Charisma buffed past 20 the formulas cross, and buying
// and reselling (at one shop, or across a general store and a specialist)
// made gold.
func CalculateSellPrice(basePrice int, shopConfig types.ShopConfig, charisma int) int {
	sell := rawSellPrice(basePrice, shopConfig, charisma)
	for _, kind := range []string{"general", "specialty"} {
		if buy := CalculateBuyPrice(basePrice, types.ShopConfig{ShopType: kind}, charisma); sell >= buy {
			sell = max(buy-1, 0)
		}
	}
	return sell
}

// rawSellPrice is the sell formula alone.
func rawSellPrice(basePrice int, shopConfig types.ShopConfig, charisma int) int {
	// Get pricing rules from database
	rules, err := db.GetShopPricingRules()
	if err != nil {
		log.Printf("⚠️ Failed to load shop pricing rules, using defaults: %v", err)
		// Fallback to hard-coded defaults
		var baseMult float64
		var charismaRate float64
		if shopConfig.ShopType == "specialty" {
			baseMult = 0.5
			charismaRate = 0.05
		} else {
			baseMult = 0.3875
			charismaRate = 0.05625
		}
		finalMultiplier := baseMult + float64(charisma-10)*charismaRate
		if finalMultiplier < 0 {
			finalMultiplier = 0
		}
		result := int(float64(basePrice)*finalMultiplier + 0.5)
		if result < 0 {
			result = 0
		}
		return result
	}

	// Get appropriate pricing based on shop type
	var baseMult float64
	var charismaRate float64
	charismaBase := rules.CharismaBase
	if charismaBase == 0 {
		charismaBase = 10 // Default if not set
	}

	if shopConfig.ShopType == "specialty" {
		baseMult = rules.SellPricing.Specialty.BaseMultiplier
		charismaRate = rules.SellPricing.Specialty.CharismaRate
	} else {
		baseMult = rules.SellPricing.General.BaseMultiplier
		charismaRate = rules.SellPricing.General.CharismaRate
	}

	// Formula: base_value × (base_multiplier + (CHA - charisma_base) × charisma_rate)
	charismaBonus := float64(charisma-charismaBase) * charismaRate
	finalMultiplier := baseMult + charismaBonus

	// Ensure multiplier doesn't go below zero
	if finalMultiplier < 0 {
		finalMultiplier = 0
	}

	// Calculate final price
	finalPrice := float64(basePrice) * finalMultiplier

	// Round to nearest int, minimum 0 gold
	result := int(finalPrice + 0.5)
	if result < 0 {
		result = 0
	}

	return result
}
