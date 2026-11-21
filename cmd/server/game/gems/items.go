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
//   - ring-/necklace-<type>-<size>-<quality>: a cut gem set in gold;
//   - …-enchanted: that piece after an enchanter's work (enchant.go), at
//     every size its gem has an effect for.
//
// A natural type (pearls) has no rough gem: gem-pearl-<size>-<quality> is how
// it's found.
//
// One sprite per type serves every size and quality ("sprite"); "image" is the
// id's own path, which the server answers with the sprite (routes/sprites.go),
// and the client sizes it and makes it shine from the id (src/styles/gems.css).
func Items(c *types.GemsConfig) []map[string]any {
	var out []map[string]any
	for _, t := range SortedTypes(c) {
		for _, s := range c.Sizes {
			if !c.Types[t].Natural {
				out = append(out, gemItem(c, Gem{Type: t, Size: s.ID}))
			}
			for _, q := range c.Qualities {
				cut := Gem{Type: t, Size: s.ID, Quality: q.ID}
				out = append(out, gemItem(c, cut))
				for _, p := range types.GemPieces {
					if c.Jeweller.Pieces[p].Gold > 0 {
						cut.Piece = p
						out = append(out, pieceItem(c, cut))
						if Enchantable(c, cut) {
							out = append(out, enchantedItem(c, cut))
						}
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
// (small)", "Ruby Ring (small, fine)". A standard pearl is just "Pearl
// (small)": "Cut" means nothing on a pearl.
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
	case g.Cut() && c.Types[g.Type].Natural && g.Quality == "standard":
		return fmt.Sprintf("%s (%s)", t, size)
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

func gemTags(c *types.GemsConfig, g Gem) []string {
	tags := []string{"treasure", "gem", "gem-" + g.Type, "size-" + g.Size}
	if c.Types[g.Type].Natural {
		return append(tags, "natural", "quality-"+g.Quality)
	}
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
		"tags":      gemTags(c, g),
		"gem":       map[string]any{"type": g.Type, "size": g.Size, "quality": g.Quality},
		"generated": true,
	}
	if t.Natural {
		item["description"] = fmt.Sprintf("%s: %s.", t.Description, carats(s))
		item["sprite"] = "/res/img/items/gem-" + g.Type + ".png"
		item["notes"] = []string{qualityNote(qualityOf(c, g), "pearl"), "Found as it is: it needs no cutting. A jeweller sets it in a gold ring or necklace."}
		return item
	}
	if g.Cut() {
		q := qualityOf(c, g)
		item["description"] = fmt.Sprintf("%s, cut and polished: %s.", t.Description, carats(s))
		item["sprite"] = "/res/img/items/gem-" + g.Type + ".png"
		item["notes"] = []string{qualityNote(q, "cut"), "A jeweller sets it in a gold ring or necklace."}
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

// qualityNote is what a quality means for the price, against a standard one
// of its kind ("cut", "pearl").
func qualityNote(q types.GemQuality, kind string) string {
	switch {
	case q.Multiplier == 1:
		return "A standard " + kind + "."
	case q.Multiplier == 0.5:
		return fmt.Sprintf("%s: worth half a standard %s.", q.Name, kind)
	}
	return fmt.Sprintf("%s: worth %s× a standard %s.", q.Name, trimFloat(q.Multiplier), kind)
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
		"notes":       []string{pieceNote(c, g)},
		"gem":         map[string]any{"type": g.Type, "size": g.Size, "quality": g.Quality, "piece": g.Piece},
		"generated":   true,
	}
}

// enchantedItem is a piece after an enchanter's work: the same ring or
// necklace, worth its base fee more, wearing its gem's effect.
func enchantedItem(c *types.GemsConfig, plain Gem) map[string]any {
	g := plain
	g.Enchanted = true
	item := pieceItem(c, plain)
	value := Value(c, g)
	e, _ := Enchant(c, g.Type, g.Size)
	name := "Enchanted " + Name(c, plain)
	if e.Name != "" {
		name = fmt.Sprintf("%s: %s", e.Name, Name(c, plain))
	}
	item["id"] = g.ID()
	item["name"] = name
	item["description"] = strings.TrimSuffix(item["description"].(string), ".") + ", and enchanted: the stone holds a working."
	item["value"] = value
	item["rarity"] = Rarity(value)
	item["image"] = "/res/img/items/" + g.ID() + ".png"
	item["tags"] = append(item["tags"].([]string), "enchanted", "magic")
	gem := item["gem"].(map[string]any)
	gem["enchanted"] = true
	worn := wornFields(c, g)
	for k, v := range worn {
		item[k] = v
	}
	if _, byClass := worn["effects_when_worn_by_class"]; byClass {
		item["notes"] = []string{"Its working follows its wearer's calling: their class's ability and skill.",
			"A fighter would get " + PieceDescription(c, g, "fighter") + "."}
	} else {
		item["notes"] = []string{"While worn: " + PieceDescription(c, g, "") + "."}
	}
	return item
}

// pieceNote is a plain piece's note: what it's worth, and whether an
// enchanter could wake its stone.
func pieceNote(c *types.GemsConfig, g Gem) string {
	if Enchantable(c, g) {
		return "Worth its gold and its gem together. No magic in it yet: an enchanter could wake the stone."
	}
	return "Worth its gold and its gem together. No magic in it."
}
