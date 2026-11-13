package types

// Fishing config (docs/draft/fishing-system.md), mirrors
// game-data/systems/fishing.json. The migration expands species × sizes into
// fish-<species>-<size> items (plus cooked/salted fish by size and a trophy per
// species); the server's game/fishing engine reads the same file from the
// systems table.

// FishingConfig is fishing.json.
type FishingConfig struct {
	Sizes    []FishSize              `json:"sizes"`
	Profiles map[string]FishProfile  `json:"profiles"`
	Species  map[string]FishSpecies  `json:"species"`
	Baits    map[string]FishBait     `json:"baits"`
	Spots    map[string]FishingSpot  `json:"spots"`
	Prepared map[string]PreparedFish `json:"prepared"`
	Trophy   TrophyRules             `json:"trophy"`
	Rules    FishingRules            `json:"rules"`
}

// FishSize is one size band, a slice of a species' max weight. Sizes are listed
// smallest first; a size's step is its index minus the medium's.
type FishSize struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	MaxPct   float64 `json:"max_pct"`   // upper edge of the band, as a fraction of max_lb
	Share    float64 `json:"share"`     // how often this size bites
	Value    float64 `json:"value"`     // × the species' medium value
	Power    int     `json:"power"`     // + the species' power
	Stamina  int     `json:"stamina"`   // fight stamina
	RawHours int     `json:"raw_hours"` // fullness eaten raw (food-system.md §3)
}

// FishProfile weights the four fight moves.
type FishProfile struct {
	Run    int `json:"run"`
	Dive   int `json:"dive"`
	Thrash int `json:"thrash"`
	Rest   int `json:"rest"`
}

// FishSpecies is one kind of fish.
type FishSpecies struct {
	Name     string   `json:"name"`
	Water    []string `json:"water"`
	Bites    string   `json:"bites"` // any | day | night | dawn_dusk | dawn
	MaxLb    float64  `json:"max_lb"`
	Value    int      `json:"value"` // a medium one's value
	Power    int      `json:"power"`
	Profile  string   `json:"profile"`
	Shy      bool     `json:"shy,omitempty"`
	Likes    []string `json:"likes,omitempty"` // bait kinds; "any" likes everything
	Barehand bool     `json:"barehand_only,omitempty"`
	Rarity   string   `json:"rarity,omitempty"` // base rarity (huge goes one up)
	Desc     string   `json:"description"`
}

// FishBait is a bait or lure kind, named by an item's fishing.bait.
type FishBait struct {
	Name  string  `json:"name"`
	Tilt  float64 `json:"tilt"`           // size tilt (bigger than 1 favours bigger fish)
	Lure  bool    `json:"lure,omitempty"` // kept unless the line snaps
	Night float64 `json:"night,omitempty"`
}

// FishingSpot is a place to fish.
type FishingSpot struct {
	Name string `json:"name"`
	// District puts the spot in a town district (a Fish action there); a spot
	// without one is fished from a POI's fishing node.
	District   string             `json:"district,omitempty"`
	Water      string             `json:"water"`
	DC         int                `json:"dc"`
	BiteRate   float64            `json:"bite_rate"`
	Shallows   bool               `json:"shallows,omitempty"`
	Worms      bool               `json:"worms,omitempty"` // Dig for worms here
	DailyCatch int                `json:"daily_catch"`
	Species    map[string]float64 `json:"species"`
	Barehand   map[string]float64 `json:"barehand,omitempty"` // barehand-only species found here
	Finds      map[string]float64 `json:"finds,omitempty"`    // item id → weight per bite
	Shellfish  map[string]float64 `json:"shellfish,omitempty"`
	Monsters   []FishMonster      `json:"monsters,omitempty"`
}

// FishMonster is something nastier that takes the bait at a spot.
type FishMonster struct {
	Monster string  `json:"monster"`
	Weight  float64 `json:"weight"`
	When    string  `json:"when,omitempty"` // "" any | day | night
}

// PreparedFish is a cooked or salted fish by size.
type PreparedFish struct {
	Name       string         `json:"name"`
	Image      string         `json:"image"`
	ImageLarge string         `json:"image_large,omitempty"` // large and huge
	Shelf      int            `json:"shelf_life"`
	Hours      map[string]int `json:"hours"` // size → fullness
	Value      map[string]int `json:"value"` // size → gold
}

// TrophyRules are Finn's mounting terms.
type TrophyRules struct {
	Weight float64 `json:"weight"`
	Value  float64 `json:"value"` // × the huge fish's value
	Fee    float64 `json:"fee"`   // × the huge fish's value, paid to mount it
}

// FishingRules are the minigame's tunables.
type FishingRules struct {
	WaitMinutes  int     `json:"wait_minutes"`
	FightMinutes int     `json:"fight_minutes"`
	RecastMinute int     `json:"recast_minutes"`
	BaseBite     float64 `json:"base_bite"`
	MaxDistance  int     `json:"max_distance"`
	QuickMinutes int     `json:"quick_minutes"`
	NetMinutes   int     `json:"net_minutes"`
	WormMinutes  int     `json:"worm_minutes"`
	WormDC       int     `json:"worm_dc"`
	MonsterBite  float64 `json:"monster_bite"` // share of bites that are a monster, where a spot has any
	FindBite     float64 `json:"find_bite"`    // share of bites that are a find
}

// FishItemID is the generated item for a species at a size.
func FishItemID(species, size string) string { return "fish-" + species + "-" + size }

// TrophyItemID is a species' mounted trophy.
func TrophyItemID(species string) string { return "trophy-" + species }

// PreparedFishID is a cooked or salted fish of a size ("cooked", "small").
func PreparedFishID(kind, size string) string { return kind + "-fish-" + size }
