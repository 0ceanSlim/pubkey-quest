package types

// Mining config (docs/draft/mining-system.md), mirrors
// game-data/systems/mining.json. A seam's faces live on POI "mine" nodes
// (POIStep.Seam); anything a seam leaves out falls back to its tier here.

// MiningConfig is mining.json.
type MiningConfig struct {
	Tiers []MiningTier         `json:"tiers"` // tier 1 first
	Ores  map[string]MiningOre `json:"ores"`  // item id → what it takes to cut
	Rules MiningRules          `json:"rules"`
}

// MiningOre is an ore's tier and the pick tier that cuts it.
type MiningOre struct {
	Tier int `json:"tier"`
	Pick int `json:"pick"`
}

// MiningTier is what a face of a tier is like by default.
type MiningTier struct {
	Tier        int     `json:"tier"`
	DC          int     `json:"dc"`
	FacesPerDay int     `json:"faces_per_day"`
	Grid        [2]int  `json:"grid"`   // Veins: columns × rows
	Veins       [2]int  `json:"veins"`  // Veins: ore veins, [min, max]
	Gems        [2]int  `json:"gems"`   // Veins: gem cells
	Faults      [2]int  `json:"faults"` // Veins: fault cells
	Hard        float64 `json:"hard"`   // Veins: share of hard rock
	CaveInDice  int     `json:"cave_in_dice"`
	XP          int     `json:"xp"`       // base XP for a face worked well
	Exertion    int     `json:"exertion"` // work toward fatigue per face (rules.exertion_per_fatigue)
}

// MiningRules are the times and costs shared by every face.
type MiningRules struct {
	SwingMinutes       int `json:"swing_minutes"`
	SoundMinutes       int `json:"sound_minutes"`
	QuickMinutes       int `json:"quick_minutes"`
	ExertionPerFatigue int `json:"exertion_per_fatigue"`
	// Pick wear (degradation): per strike, per fault struck, per quick roll.
	WearPerStrike int `json:"wear_per_strike"`
	WearPerFault  int `json:"wear_per_fault"`
	QuickWear     int `json:"quick_wear"`
}

// Seam is a mine node's rock face. Zero fields take the tier's defaults.
type Seam struct {
	Tier        int `json:"tier"`
	DC          int `json:"dc,omitempty"`
	FacesPerDay int `json:"faces_per_day,omitempty"`
	// Underground: always dark, so reading the rock takes a light (or
	// darkvision). A surface face is only dark at night.
	Underground bool      `json:"underground,omitempty"`
	Ores        []SeamOre `json:"ores"`
	// QuickLoot is what a quick roll yields on a success. Left out, it's one
	// roll per ore weight, twice.
	QuickLoot *POILootTable `json:"quick_loot,omitempty"`
	// Gems is the seam's gem leaning (gems.json mining): which types its gem
	// cells hold, by weight, in place of the usual drop rates.
	Gems *SeamGems `json:"gems,omitempty"`
}

// SeamGems is a seam's gem leaning.
type SeamGems struct {
	Types map[string]int `json:"types"`
	// Cells overrides the tier's count of gem cells ([min, max]); a deep mine
	// whose gems are its draw has more.
	Cells [2]int `json:"cells,omitempty"`
}

// SeamOre is one ore a face can hold.
type SeamOre struct {
	Item     string `json:"item"`
	Weight   int    `json:"weight"`
	Quantity [2]int `json:"quantity"`
}
