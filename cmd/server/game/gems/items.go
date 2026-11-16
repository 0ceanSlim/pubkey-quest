package gems

import (
	"fmt"
	"strings"

	"pubkey-quest/types"
)

// Items expands gems.json into its generated items, each an item JSON object
// (as a game-data/items file would be) with "generated": true:
//   - gem-<type>-<size>-uncut: a rough gem, quality unknown until it's cut;
//   - gem-<type>-<size>-<quality>: the cut gem;
//   - ring-/necklace-<type>-<size>-<quality>: a cut gem set in gold.
//
// One sprite per type serves every size and quality ("sprite"); "image" is the
// id's own path, which the server answers with the sprite (routes/sprites.go),
// and the client sizes it and makes it shine from the id (src/styles/gems.css).
func Items(c *types.GemsConfig) []map[string]any {
	var out []map[string]any
	for _, t := range SortedTypes(c) {
		for _, s := range c.Sizes {
			out = append(out, gemItem(c, Gem{Type: t, Size: s.ID}))
			for _, q := range c.Qualities {
				cut := Gem{Type: t, Size: s.ID, Quality: q.ID}
				out = append(out, gemItem(c, cut))
				for _, p := range types.GemPieces {
					if c.Jeweller.Pieces[p].Gold > 0 {
						cut.Piece = p
						out = append(out, pieceItem(c, cut))
					}
				}
			}
		}
	}
	return out
}

// Names.

func sizeOf(c *types.GemsConfig, g Gem) types.GemSize {
	return c.Sizes[max(SizeIndex(c, g.Size), 0)]
}

func qualityOf(c *types.GemsConfig, g Gem) types.GemQuality {
	if i := QualityIndex(c, g.Quality); i >= 0 {
		return c.Qualities[i]
	}
	return types.GemQuality{}
}

// Name is a gem's or piece's display name: "Rough Ruby (small)", "Fine Ruby
// (small)", "Ruby Ring (small, fine)".
func Name(c *types.GemsConfig, g Gem) string {
	t := c.Types[g.Type].Name
	size := strings.ToLower(sizeOf(c, g).Name)
	switch {
	case g.Piece != "":
		qual := ""
		if g.Quality != "standard" {
			qual = ", " + strings.ToLower(qualityOf(c, g).Name)
		}
		return fmt.Sprintf("%s %s (%s%s)", t, c.Jeweller.Pieces[g.Piece].Name, size, qual)
	case g.Cut():
		return fmt.Sprintf("%s %s (%s)", qualityOf(c, g).Name, t, size)
	}
	return fmt.Sprintf("Rough %s (%s)", t, size)
}

// carats is a size band as words: "about 2–5 carats".
func carats(s types.GemSize) string {
	return fmt.Sprintf("about %s–%s carats", trimFloat(s.Carats[0]), trimFloat(s.Carats[1]))
}

func trimFloat(f float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.1f", f), "0"), ".")
}

func gemTags(g Gem) []string {
	tags := []string{"treasure", "gem", "gem-" + g.Type, "size-" + g.Size}
	if g.Cut() {
		return append(tags, "cut", "quality-"+g.Quality)
	}
	return append(tags, "uncut")
}

func gemItem(c *types.GemsConfig, g Gem) map[string]any {
	t, s := c.Types[g.Type], sizeOf(c, g)
	value := Value(c, g)
	item := map[string]any{
		"id":        g.ID(),
		"name":      Name(c, g),
		"type":      "Treasure",
		"value":     value,
		"rarity":    Rarity(value),
		"stack":     10,
		"weight":    0,
		"image":     "/res/img/items/" + g.ID() + ".png",
		"tags":      gemTags(g),
		"gem":       map[string]any{"type": g.Type, "size": g.Size, "quality": g.Quality},
		"generated": true,
	}
	if g.Cut() {
		q := qualityOf(c, g)
		item["description"] = fmt.Sprintf("%s, cut and polished: %s.", t.Description, carats(s))
		item["sprite"] = "/res/img/items/gem-" + g.Type + ".png"
		item["notes"] = []string{qualityNote(q), "A jeweller sets it in a gold ring or necklace."}
		return item
	}
	item["description"] = fmt.Sprintf("%s, still in its rough skin: %s.", t.Description, carats(s))
	item["sprite"] = "/res/img/items/gem-" + g.Type + "-uncut.png"
	item["notes"] = []string{
		fmt.Sprintf("Rough: its quality won't show until a jeweller cuts it (%d gold).", s.CutFee),
		"Sells as a standard cut gem would.",
	}
	return item
}

// qualityNote is what a quality means for the price.
func qualityNote(q types.GemQuality) string {
	switch {
	case q.Multiplier == 1:
		return "A standard cut."
	case q.Multiplier == 0.5:
		return q.Name + ": worth half a standard cut."
	}
	return fmt.Sprintf("%s: worth %s× a standard cut.", q.Name, trimFloat(q.Multiplier))
}

func pieceItem(c *types.GemsConfig, g Gem) map[string]any {
	p, t, s := c.Jeweller.Pieces[g.Piece], c.Types[g.Type], sizeOf(c, g)
	value := Value(c, g)
	gem := strings.ToLower(s.Name)
	if g.Quality != "standard" {
		gem += " " + strings.ToLower(qualityOf(c, g).Name)
	}
	gem += " " + strings.ToLower(t.Name)
	return map[string]any{
		"id":          g.ID(),
		"name":        Name(c, g),
		"description": fmt.Sprintf("A %s of %d gold, set with a %s.", strings.ToLower(p.Name), p.Gold, gem),
		"type":        "Adventuring Gear",
		"value":       value,
		"rarity":      Rarity(value),
		"stack":       1,
		"weight":      p.Weight,
		"gear_slot":   p.GearSlot,
		"image":       "/res/img/items/" + g.ID() + ".png",
		"sprite":      fmt.Sprintf("/res/img/items/%s-%s.png", g.Piece, g.Type),
		"tags":        []string{"equipment", "jewelry", "valuable", "gem-" + g.Type, "size-" + g.Size, "quality-" + g.Quality},
		"notes":       []string{"Worth its gold and its gem together. No magic in it."},
		"gem":         map[string]any{"type": g.Type, "size": g.Size, "quality": g.Quality, "piece": g.Piece},
		"generated":   true,
	}
}
