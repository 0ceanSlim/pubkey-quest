// Package gems is gems (docs/draft/gem-system.md): the items gems.json
// generates, what each is worth, and the rolls that find them, at a kill (by
// challenge rating), at a rock face (by tier) and at the jeweller's wheel (the
// quality, which no uncut gem carries), and the pearls shellfish hold.
package gems

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// LoadFile reads gems.json from disk (Codex: migration, validation, wiki).
func LoadFile(path string) (*types.GemsConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg types.GemsConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("gems.json: %v", err)
	}
	return &cfg, nil
}

var (
	cfgOnce sync.Once
	cfg     *types.GemsConfig
)

// Config is gems.json, read once from the systems table (the server). It's
// never nil; without the table there are no gems to find.
func Config() *types.GemsConfig {
	cfgOnce.Do(func() {
		cfg = &types.GemsConfig{}
		database := serverdb.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'gems'").Scan(&props) != nil {
			return
		}
		if err := json.Unmarshal([]byte(props), cfg); err != nil {
			cfg = &types.GemsConfig{}
		}
	})
	return cfg
}

// SetConfig replaces the loaded config (tests).
func SetConfig(c *types.GemsConfig) {
	cfgOnce.Do(func() {})
	cfg = c
}

// SortedTypes is the gem types, most valuable first.
func SortedTypes(c *types.GemsConfig) []string {
	ids := make([]string, 0, len(c.Types))
	for id := range c.Types {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := c.Types[ids[i]], c.Types[ids[j]]
		if a.BaseValue != b.BaseValue {
			return a.BaseValue > b.BaseValue
		}
		return ids[i] < ids[j]
	})
	return ids
}

// SizeIndex is a size's place in the bands (0 = tiny), or -1.
func SizeIndex(c *types.GemsConfig, size string) int {
	for i, s := range c.Sizes {
		if s.ID == size {
			return i
		}
	}
	return -1
}

// QualityIndex is a quality's place (0 = worst), or -1.
func QualityIndex(c *types.GemsConfig, quality string) int {
	for i, q := range c.Qualities {
		if q.ID == quality {
			return i
		}
	}
	return -1
}

// Gem is what an item id says about a gem or a piece set with one.
type Gem struct {
	Type    string
	Size    string
	Quality string // "" while uncut
	Piece   string // "ring" / "necklace" for jewelry
}

// Cut reports whether it's finished: cut, or a natural gem (a pearl), which
// is found with its quality.
func (g Gem) Cut() bool { return g.Quality != "" }

// Parse reads a generated gem or piece id ("gem-ruby-small-uncut",
// "gem-ruby-small-fine", "ring-ruby-small-fine", "gem-black-pearl-tiny-fine").
// A type id may hold dashes; sizes and qualities don't.
func Parse(c *types.GemsConfig, id string) (Gem, bool) {
	parts := strings.Split(id, "-")
	n := len(parts)
	if n < 4 {
		return Gem{}, false
	}
	g := Gem{Type: strings.Join(parts[1:n-2], "-"), Size: parts[n-2]}
	t, ok := c.Types[g.Type]
	if !ok || SizeIndex(c, g.Size) < 0 {
		return Gem{}, false
	}
	switch {
	case parts[0] == "gem" && parts[n-1] == "uncut":
		if t.Natural {
			return Gem{}, false
		}
	case parts[0] == "gem":
		g.Quality = parts[n-1]
	case c.Jeweller.Pieces[parts[0]].Gold > 0:
		g.Piece, g.Quality = parts[0], parts[n-1]
	default:
		return Gem{}, false
	}
	if g.Quality != "" && QualityIndex(c, g.Quality) < 0 {
		return Gem{}, false
	}
	return g, true
}

// ID is the item id for a gem or piece.
func (g Gem) ID() string {
	switch {
	case g.Piece != "":
		return types.GemPieceID(g.Piece, g.Type, g.Size, g.Quality)
	case g.Quality != "":
		return types.CutGemID(g.Type, g.Size, g.Quality)
	}
	return types.UncutGemID(g.Type, g.Size)
}

// Value is what a gem is worth in gold: base × size, × quality once cut (an
// uncut gem sells at a standard cut's price). A piece adds its gold.
func Value(c *types.GemsConfig, g Gem) int {
	t := c.Types[g.Type]
	v := float64(t.BaseValue)
	if i := SizeIndex(c, g.Size); i >= 0 {
		v *= c.Sizes[i].Multiplier
	}
	if i := QualityIndex(c, g.Quality); i >= 0 {
		v *= c.Qualities[i].Multiplier
	}
	gold := max(1, int(math.Round(v)))
	if g.Piece != "" {
		gold += c.Jeweller.Pieces[g.Piece].Gold
	}
	return gold
}

// Rarity follows value, onto the game's five rarities.
func Rarity(value int) string {
	switch {
	case value >= 1_000_000:
		return "mythic"
	case value >= 100_000:
		return "legendary"
	case value >= 10_000:
		return "rare"
	case value >= 1_000:
		return "uncommon"
	}
	return "common"
}

// CutFee is the jeweller's fee to cut a gem of a size.
func CutFee(c *types.GemsConfig, size string) int {
	if i := SizeIndex(c, size); i >= 0 {
		return c.Sizes[i].CutFee
	}
	return 0
}

// SetFee is the jeweller's price to set a cut gem in a piece: the piece's gold
// plus labour on the gem's value.
func SetFee(c *types.GemsConfig, cut Gem, piece string) (gold, labour int) {
	p := c.Jeweller.Pieces[piece]
	return p.Gold, max(1, int(math.Round(float64(Value(c, cut))*c.Jeweller.Labour)))
}

// ─── Rolls ───────────────────────────────────────────────────────────────────

// Rng is what a roll draws from: a seeded *rand.Rand (a rock face's layout,
// tests), or Global, which is safe to share between sessions.
type Rng interface{ Float64() float64 }

type globalRng struct{}

func (globalRng) Float64() float64 { return rand.Float64() }

// Global rolls from math/rand's shared source.
var Global Rng = globalRng{}

// At reads a curve at x: log-linear between points, flat past the ends.
func At(curve types.Curve, x float64) float64 {
	if len(curve) == 0 {
		return 1
	}
	if x <= curve[0][0] {
		return curve[0][1]
	}
	for i := 1; i < len(curve); i++ {
		a, b := curve[i-1], curve[i]
		if x <= b[0] {
			if b[0] == a[0] || a[1] <= 0 || b[1] <= 0 {
				return b[1]
			}
			f := (x - a[0]) / (b[0] - a[0])
			return math.Exp(math.Log(a[1]) + f*(math.Log(b[1])-math.Log(a[1])))
		}
	}
	return curve[len(curve)-1][1]
}

// TypeWeights are the type roll's weights at a tilt: drop_rate ×
// tilt^log10(base_value/1000). A leaning (type → weight) replaces drop_rate.
// Natural gems are never in it.
func TypeWeights(c *types.GemsConfig, tilt float64, leaning map[string]int) map[string]float64 {
	out := map[string]float64{}
	for id, t := range c.Types {
		if t.Natural {
			continue
		}
		w := t.DropRate
		if len(leaning) > 0 {
			w = float64(leaning[id])
		}
		if w <= 0 {
			continue
		}
		out[id] = w * math.Pow(tilt, math.Log10(float64(max(t.BaseValue, 1))/1000))
	}
	return out
}

// SizeWeights are the size roll's weights at a tilt, up to maxSize (an index;
// -1 for all): share × tilt^log10(multiplier).
func SizeWeights(c *types.GemsConfig, tilt float64, maxSize int) []float64 {
	out := make([]float64, len(c.Sizes))
	for i, s := range c.Sizes {
		if maxSize >= 0 && i > maxSize {
			break
		}
		out[i] = s.DropRate * math.Pow(tilt, math.Log10(s.Multiplier))
	}
	return out
}

func pickType(w map[string]float64, rng Rng) string {
	ids := make([]string, 0, len(w))
	total := 0.0
	for id, x := range w {
		ids = append(ids, id)
		total += x
	}
	if total <= 0 {
		return ""
	}
	sort.Strings(ids) // a seeded roll is repeatable
	r := rng.Float64() * total
	for _, id := range ids {
		if r -= w[id]; r < 0 {
			return id
		}
	}
	return ids[len(ids)-1]
}

func pickIndex(w []float64, rng Rng) int {
	total := 0.0
	for _, x := range w {
		total += x
	}
	if total <= 0 {
		return -1
	}
	r := rng.Float64() * total
	for i, x := range w {
		if r -= x; r < 0 {
			return i
		}
	}
	return len(w) - 1
}

// roll is one uncut gem at the given tilts.
func roll(c *types.GemsConfig, typeTilt, sizeTilt float64, leaning map[string]int, maxSize int, rng Rng) (Gem, bool) {
	t := pickType(TypeWeights(c, typeTilt, leaning), rng)
	s := pickIndex(SizeWeights(c, sizeTilt, maxSize), rng)
	if t == "" || s < 0 {
		return Gem{}, false
	}
	return Gem{Type: t, Size: c.Sizes[s].ID}, true
}

// KillChance is a monster's chance of carrying a gem: its own gem_chance if
// it sets one, none for the creature types that carry none, else the CR curve.
func KillChance(c *types.GemsConfig, m *types.MonsterData) float64 {
	if m.GemChance != nil {
		return *m.GemChance
	}
	for _, t := range c.Drops.NoGemTypes {
		if strings.EqualFold(t, m.Type) {
			return 0
		}
	}
	if len(c.Drops.Chance) == 0 {
		return 0
	}
	return At(c.Drops.Chance, m.ChallengeRating)
}

// ForKill rolls a slain monster's gem: an uncut gem id, or "" for none.
func ForKill(c *types.GemsConfig, m *types.MonsterData, rng Rng) string {
	if len(c.Types) == 0 || rng.Float64() >= KillChance(c, m) {
		return ""
	}
	g, ok := roll(c, At(c.Drops.TypeTilt, m.ChallengeRating), At(c.Drops.SizeTilt, m.ChallengeRating), nil, -1, rng)
	if !ok {
		return ""
	}
	return g.ID()
}

// ForFace rolls a gem cell at a rock face of a tier: its type (from the seam's
// leaning, if it has one) and size, no bigger than maxSize (-1 for any).
func ForFace(c *types.GemsConfig, tier int, leaning map[string]int, maxSize int, rng Rng) (Gem, bool) {
	if len(c.Types) == 0 {
		return Gem{}, false
	}
	return roll(c, At(c.Mining.TypeTilt, float64(tier)), At(c.Mining.SizeTilt, float64(tier)), leaning, maxSize, rng)
}

// ForShellfish rolls what's inside a shellfish (an item id): a pearl, with its
// quality, or false for nothing (or a shellfish gems.json doesn't list).
func ForShellfish(c *types.GemsConfig, itemID string, rng Rng) (Gem, bool) {
	sf, ok := c.Pearls.Shellfish[itemID]
	if !ok || rng.Float64() >= sf.Chance {
		return Gem{}, false
	}
	w := map[string]float64{}
	for t, n := range sf.Types {
		if n > 0 {
			w[t] = float64(n)
		}
	}
	t := pickType(w, rng)
	s := pickIndex(SizeWeights(c, sf.SizeTilt, SizeIndex(c, sf.MaxSize)), rng)
	if t == "" || s < 0 {
		return Gem{}, false
	}
	return Gem{Type: t, Size: c.Sizes[s].ID, Quality: RollQuality(c, rng)}, true
}

// IsShellfish reports whether an item can hold a pearl.
func IsShellfish(c *types.GemsConfig, itemID string) bool {
	_, ok := c.Pearls.Shellfish[itemID]
	return ok
}

// RollQuality is the quality a gem shows when it's cut.
func RollQuality(c *types.GemsConfig, rng Rng) string {
	w := make([]float64, len(c.Qualities))
	for i, q := range c.Qualities {
		w[i] = q.DropRate
	}
	if i := pickIndex(w, rng); i >= 0 {
		return c.Qualities[i].ID
	}
	return ""
}
