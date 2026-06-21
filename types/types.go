package types

type RegistryEntry struct {
	Npub      string    `json:"npub"`
	PubKey    string    `json:"pubkey"`
	Character Character `json:"character"`
}

type RaceClassWeight struct {
	Race   string
	Class  string
	Weight int
}

type RaceBackgroundWeight struct {
	Race       string
	Background string
	Weight     int
}

type Character struct {
	Race       string         `json:"race"`
	Class      string         `json:"class"`
	Background string         `json:"background"`
	Alignment  string         `json:"alignment"`
	Stats      map[string]int `json:"stats"`
}

type WeightData struct {
	Races                    []string                  `json:"Races"`
	RaceWeights              []int                     `json:"RaceWeights"`
	ClassWeightsByRace       map[string]map[string]int `json:"classWeightsByRace"`
	BackgroundWeightsByClass map[string]map[string]int `json:"backgroundWeightsByClass"`
	Alignments               []string                  `json:"Alignments"`
	AlignmentWeights         []int                     `json:"AlignmentWeights"`
}

// Weighted option structure
type WeightedOption struct {
	Name   string
	Weight int
}

// NPCScheduleSlot represents a time period in an NPC's schedule
type NPCScheduleSlot struct {
	Start            int      `json:"start"`              // Minutes from midnight (0-1439)
	End              int      `json:"end"`                // Minutes from midnight (0-1439, wraps to next day if < start)
	Location         string   `json:"location"`           // Building ID or district ID (backend determines type)
	State            string   `json:"state"`              // "sleeping", "working", "traveling", "home"
	DialogueOptions  []string `json:"dialogue_options"`   // Which dialogue nodes are available
	AvailableActions []string `json:"available_actions"`  // Which actions can be performed (open_shop, etc.)
}

// NPCData represents the full NPC structure from database
type NPCData struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Title         string                 `json:"title,omitempty"`
	Race          string                 `json:"race,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Greeting      map[string]string      `json:"greeting,omitempty"`
	Dialogue      map[string]interface{} `json:"dialogue,omitempty"`
	Schedule      []NPCScheduleSlot      `json:"schedule,omitempty"`      // NPC schedule (location is per time slot)
	ShopConfig    map[string]interface{} `json:"shop_config,omitempty"`
	StorageConfig map[string]interface{} `json:"storage_config,omitempty"`
	InnConfig     map[string]interface{} `json:"inn_config,omitempty"`

	// PrimaryHome is the canonical "home" location ID for an external NPC
	// (a city, environment, or POI ID). Required for NPCs in game-data/npcs/;
	// omitted for NPCs embedded inline in encounters. The on-disk filename
	// for an external NPC must be <PrimaryHome>/<ID>.json.
	PrimaryHome string `json:"primary_home,omitempty"`
	// SecondaryHomes are additional location IDs where this NPC may appear
	// (e.g. a city merchant who visits a POI on certain days). Schedule
	// entries remain the runtime authority — this field is informational
	// metadata for Codex grouping/UX.
	SecondaryHomes []string `json:"secondary_homes,omitempty"`
}

// NPCScheduleInfo represents the resolved schedule state for an NPC at a given time
type NPCScheduleInfo struct {
	CurrentSlot       *NPCScheduleSlot `json:"current_slot"`
	IsAvailable       bool             `json:"is_available"`
	Location          string           `json:"location"`            // Current location (building or district)
	State             string           `json:"state"`
	AvailableDialogue []string         `json:"available_dialogue"`
	AvailableActions  []string         `json:"available_actions"`
}

// Shop-related types

// ShopInventoryItem represents an item in a shop's inventory
type ShopInventoryItem struct {
	ItemID          string `json:"item_id"`
	Stock           int    `json:"stock"`
	MaxStock        int    `json:"max_stock"`
	RestockRate     int    `json:"restock_rate"`
	RestockInterval string `json:"restock_interval"`
}

// ShopConfig represents the static configuration from NPC JSON
type ShopConfig struct {
	ShopType            string              `json:"shop_type"`
	BuysItems           bool                `json:"buys_items"`
	BuyPriceMultiplier  float64             `json:"buy_price_multiplier"`  // What merchant pays player (deprecated - not used)
	SellPriceMultiplier float64             `json:"sell_price_multiplier"` // What player pays merchant (deprecated - not used)
	StartingGold        int                 `json:"starting_gold"`
	MaxGold             int                 `json:"max_gold"` // Not enforced - merchants can accumulate unlimited gold
	GoldRegenRate       int                 `json:"gold_regen_rate"`       // Gold restored per gradual regen interval
	GoldRegenInterval   string              `json:"gold_regen_interval"`   // "daily" (10min), "hourly" (1min), "weekly" (70min), or direct minutes
	ItemRestockInterval int                 `json:"item_restock_interval"` // Minutes between item restocks (default 10)
	GoldRestockInterval int                 `json:"gold_restock_interval"` // Minutes between gold restocks (default 30)
	Inventory           []ShopInventoryItem `json:"inventory"`
}

// ShopState represents the runtime state of a shop (stored in save file)
type ShopState struct {
	MerchantID  string         `json:"merchant_id"`
	CurrentGold int            `json:"current_gold"`
	LastRegen   string         `json:"last_regen"` // ISO timestamp
	Stock       map[string]int `json:"stock"`      // item_id -> current quantity
}

// ShopTransaction represents a buy or sell transaction
type ShopTransaction struct {
	Npub       string `json:"npub"`
	SaveID     string `json:"save_id"`
	MerchantID string `json:"merchant_id"`
	ItemID     string `json:"item_id"`
	Quantity   int    `json:"quantity"`
	Action     string `json:"action"` // "buy" or "sell"
}