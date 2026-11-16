package types

import "fmt"

// Gems config (docs/draft/gem-system.md), mirrors game-data/systems/gems.json.
// Gems are generated items, not files: every type × size becomes an uncut gem,
// every type × size × quality a cut one, and each cut gem a ring and a necklace
// (game/gems.Items, run by --migrate). A natural type (pearls) has no uncut gem.

// GemsConfig is gems.json.
type GemsConfig struct {
	Types     map[string]GemType `json:"types"`
	Sizes     []GemSize          `json:"sizes"`     // smallest first
	Qualities []GemQuality       `json:"qualities"` // worst first
	Drops     GemDrops           `json:"drops"`
	Mining    GemMining          `json:"mining"`
	Jeweller  GemJeweller        `json:"jeweller"`
	Pearls    GemPearls          `json:"pearls"`
}

// GemType is one kind of gem.
type GemType struct {
	Name        string  `json:"name"`
	BaseValue   int     `json:"base_value"` // a medium, standard gem, in gold
	DropRate    float64 `json:"drop_rate"`  // weight in the type roll
	Description string  `json:"description"`
	// Natural gems (pearls) are found finished: no rough form, a quality
	// from the moment they're found, and never in a kill's or a rock's roll.
	Natural bool `json:"natural,omitempty"`
}

// GemSize is a size band.
type GemSize struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Multiplier float64    `json:"multiplier"` // of the type's base value
	DropRate   float64    `json:"drop_rate"`  // base share in the size roll
	Carats     [2]float64 `json:"carats"`     // the band, rough
	CutFee     int        `json:"cut_fee"`    // the jeweller's fee to cut one
}

// GemQuality is how good a cut gem turned out.
type GemQuality struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Multiplier float64 `json:"multiplier"`
	DropRate   float64 `json:"drop_rate"` // chance at the cut
}

// Curve is [x, value] points, interpolated log-linearly and held flat past
// the ends.
type Curve [][2]float64

// GemDrops is the per-kill roll, scaled by the monster's CR.
type GemDrops struct {
	Chance   Curve `json:"gem_chance"`
	TypeTilt Curve `json:"type_tilt"`
	SizeTilt Curve `json:"size_tilt"`
	// NoGemTypes are creature types that carry no gems unless the monster
	// sets its own gem_chance (beasts, plants, oozes).
	NoGemTypes []string `json:"no_gem_types"`
}

// GemMining is the same roll at a rock face, scaled by the face's tier.
type GemMining struct {
	TypeTilt Curve `json:"type_tilt"`
	SizeTilt Curve `json:"size_tilt"`
	// GeodeShare is the chance a gem cell holds a geode instead.
	GeodeShare float64 `json:"geode_share"`
	// QuickChance is a quick roll's chance of a gem on a success, no bigger
	// than QuickMaxSize.
	QuickChance  float64 `json:"quick_chance"`
	QuickMaxSize string  `json:"quick_max_size"`
	// Cracking a geode with a pick: the tier its gem rolls at, the time and
	// the pick wear.
	GeodeTier    int `json:"geode_tier"`
	CrackMinutes int `json:"crack_minutes"`
	CrackWear    int `json:"crack_wear"`
}

// GemPearls is where pearls come from: shellfish, by item id ("oyster").
type GemPearls struct {
	Shellfish map[string]GemShellfish `json:"shellfish"`
}

// GemShellfish is one shellfish's pearls: the chance one holds a pearl,
// which (weights by type), how big (a size tilt, as GemDrops') and the
// largest size it can be.
type GemShellfish struct {
	Chance   float64        `json:"chance"`
	Types    map[string]int `json:"types"`
	SizeTilt float64        `json:"size_tilt"`
	MaxSize  string         `json:"max_size"`
}

// GemJeweller is the jeweller's trade: cutting, and setting a cut gem in gold.
type GemJeweller struct {
	NPC    string              `json:"npc"`
	Labour float64             `json:"labour"` // of the cut gem's value, to set it
	Pieces map[string]GemPiece `json:"pieces"` // "ring", "necklace"
}

// GemPiece is a piece of jewelry a cut gem is set in.
type GemPiece struct {
	Name     string  `json:"name"`
	Gold     int     `json:"gold"` // coins it takes (the piece is worth them)
	Weight   float64 `json:"weight"`
	GearSlot string  `json:"gear_slot"`
}

// Gem pieces, in the order the jeweller offers them.
var GemPieces = []string{"ring", "necklace"}

// UncutGemID is a rough gem's item id.
func UncutGemID(gemType, size string) string {
	return fmt.Sprintf("gem-%s-%s-uncut", gemType, size)
}

// CutGemID is a cut gem's item id.
func CutGemID(gemType, size, quality string) string {
	return fmt.Sprintf("gem-%s-%s-%s", gemType, size, quality)
}

// GemPieceID is a ring's or necklace's item id.
func GemPieceID(piece, gemType, size, quality string) string {
	return fmt.Sprintf("%s-%s-%s-%s", piece, gemType, size, quality)
}

// GeodeID is the geode item.
const GeodeID = "geode"
