# Spell scrolls

Casters scribe single-use scrolls of spells they know; any caster can read one.
For wizards, scrolls are also how they learn: copying a scroll into the
spellbook adds the spell for good.

Companion draft: [spell-learning.md](spell-learning.md), which covers what a
caster knows at each level. Scribing is gated on that, so the two systems use
one source of truth.

> Rewritten 2026-10-18 to fit the spellbook package (`game/spellbook`). The
> earlier version hand-gated recipes by class, character level and raw INT. All
> of that now derives from what the character knows.

## Status

| Part | Status |
|---|---|
| **Use** a scroll (`spells.CastFromScroll`: any caster, costs mana, consumed, skips known/prepared/component gates) | ✅ built (`a5314e5`). 3 scroll items exist |
| **Scribe** a scroll at a station | ⬜ to build |
| **Copy** a scroll into a wizard's spellbook | ⬜ to build (see spell-learning.md → Wizard) |
| Scroll items for every spell + sprites | ⬜ to build |

## The four rules

1. **You can scribe what you know.** A spell is scribable when
   `spellbook.Knows(save, spell)` is true and its level is at most
   `spellbook.MaxSpellLevel(class, level)`. Nothing new is stored, and there is
   no recipe list. A level-3 druid can scribe barkskin and pass without trace
   because they know them, not because a recipe file says level 3.
2. **The recipe comes from the spell.** No recipe files. Scribing costs:
   - 1 **parchment** (consumed);
   - the spell's own `material_component.required` items, if it has any
     (consumed, as when casting: a focus that "provides unlimited" a component
     covers it, the same rule as casting);
   - an **ink pen** and **ink** carried (not consumed for now; ink becomes a
     uses-item when the uses/durability system lands);
   - **gold** and **time** by spell level (table below).
3. **The roll uses your casting ability**: WIS for druids, clerics and rangers,
   INT for wizards, CHA for sorcerers, bards, warlocks and paladins (the class's
   casting ability, `character.SpellcastingAbility`). The Crafting skill and the
   Crafting Kit stay with physical crafting (gear, repairs, later), so the game
   has two separate crafting models instead of two overlapping ones.
4. **"Spellbook" is the item; "known spells" is the list.** The `spellbook`
   item stays a 20-slot scroll container (`allowed_types: ["spell-scroll"]`). A
   wizard's copied spells go into `KnownSpells` and do **not** depend on carrying
   the book. Losing the book on defeat must never cost a wizard their spells.

## Cost, time and success

Gold uses the game's internal scale (PHB prices in copper as gold, the item
rule). Times follow the spell-prep idea: scribing takes game time.

| Spell level | Gold | Time | Base success |
|---|---|---|---|
| Cantrip | 1,500 | 1 hour | 85% |
| 1 | 2,500 | 2 hours | 85% |
| 2 | 10,000 | 4 hours | 75% |
| 3 | 25,000 | 8 hours | 65% |

- **Success** = base + 5% × casting-ability modifier, capped at 95% and floored
  at 25%. A druid with WIS 16 (+3) scribes barkskin at 90%.
- **Failure**: the parchment, the components and the gold are spent; no scroll.
  The time passes either way.
- **Tuning note:** level 2 is well under Xanathar's (250 gp → 25,000) on
  purpose, for alpha pacing. The balance pass (M8) revisits all four rows.
- Gold and time are a tuning table in data (`game-data/magic/scroll-scribing.json`),
  not per-spell fields.

## Stations

Scribing happens at a station of the spell's **tradition**. Stations are data
on the building (`crafting_stations: ["scribe_divine"]` in the city JSON), so
adding one is a data edit.

| Tradition | Classes | Stations (current world) |
|---|---|---|
| `scribe_divine` | cleric, paladin | Temple of the Dawnflame, Goldenhaven (`dawnflame_temple`, Sanctuary) · Shrine of the Sacred Well, Dusthaven (`oasis_shrine`) · Shrine to the Sea Mother, Saltwind (`sea_shrine`) |
| `scribe_nature` | druid, ranger | Thalindra's Grove, Verdant (`thalindra_grove`) |
| `scribe_arcane` | wizard, sorcerer, warlock, bard | Mage Tower, Kingdom (`mage_tower`) |

Gaps to fill in M7 content: a second arcane station (only the Kingdom has one)
and a second nature station. A druid grove POI in the wilds would fit.

## Scroll items

One item per spell: `scroll-of-<spell-id>`, type `Spell Scroll`, tags
`consumable` + `spell-scroll`, `spell_id` set, stack 5, weight 0.1.

- **Generated, not hand-written.** `codex --migrate` creates the scroll item for
  every spell of level 0–3 that has none. A hand-written
  `game-data/items/scroll-of-*.json` overrides the generated one (the 3
  existing scrolls stay as they are).
- **Value** = the scribing gold for its level, so shops and loot price them
  consistently.
- **Sprites**: one per tradition (divine, nature, arcane), plus the 3 existing
  ones. Not one per spell.
- **Loot**: scrolls join the tiered loot tables as rare drops from caster
  monsters and chests. That gives non-scribing classes scrolls, and gives
  wizards something to copy.

## Using a scroll (built; rules unchanged)

Any class with mana can read any scroll: it costs the spell's mana and is
consumed, skipping the known, prepared and component gates. 5e limits scrolls
to your class list; this game stays looser on purpose. Non-casters (no mana)
can't use scrolls until mana potions exist.

## Wizard copying

Part of [spell-learning.md](spell-learning.md) (Wizard). The scroll side:

- Copying needs the scroll in the inventory, a wizard spell of a level the
  wizard has slots for, that isn't already known.
- It costs 5,000 gold and 2 hours per spell level (PHB: 50 gp + 2 h per level)
  and consumes the scroll. No roll (PHB has none).
- The spell is appended to `KnownSpells`, and the save records it: a copied
  spell is a choice, so it can't be derived.

So the wizard's loop is: find a scroll (loot or a shop), copy it, and from then
on scribe more of it.

## Build

1. **Engine:** a `game/scroll` service (`Scribable(save)`,
   `Scribe(save, spellID, station)`) on top of `game/spellbook`.
   - Validate: known, slot level, station tradition, parchment, components,
     ink and pen, gold.
   - Then: roll, consume, advance time, add the scroll.
2. **API:** `GET /api/scrolls/scribable` lists each spell with its cost, its
   chance and what's missing. `POST /api/scrolls/scribe {spell_id}` scribes it.
   Responses use the standard `{success, message, data}`.
3. **Data:**
   - `scroll-scribing.json`: the cost table above.
   - `crafting_stations` on the 5 buildings above.
   - The migration step that generates the scroll items.
   - 3 tradition sprites.
4. **UI:** a "Scribe scrolls" panel at a station building (like the shop
   panels), showing chance, cost, time and missing items.
5. **Wizard copy:** `POST /api/progression/copy-scroll {item_id}`, plus a
   "Copy into spellbook" action on a scroll in the inventory (wizards only).

**Tests:**
- A level-3 druid can scribe barkskin but not moonbeam without its component.
- A wizard can't scribe what they haven't copied.
- Failure spends the materials.
- Copying a scroll adds it to `KnownSpells` and consumes the scroll.

## Not in scope

- General item crafting (beta).
- Upcasting.
- Batch scribing.
- NPC scribes for hire.
- Scroll variants (empowered, quickcast).
