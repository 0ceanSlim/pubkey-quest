# Spell scrolls

Casters scribe single-use scrolls of spells they know; any caster can read one.
For wizards, scrolls are also how they learn: a scroll kept in their
spellbook can be committed to memory, adding the spell for good.

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
| **Scribe** a scroll at a station | ✅ built: `game/spells/scribe.go`, `GET /api/scrolls/scribable`, `POST /api/scrolls/scribe`, the 📜 Scribe panel (`src/ui/scrollScribe.js`), `scroll-scribing.json`, `crafting_stations` on 5 buildings |
| **Commit to memory** (wizards: a scroll inside their spellbook) | ✅ built: `game/spells/memorize.go`, `POST /api/progression/memorize-scroll` (with `dry_run` for the confirm prompt), the container menu's "Commit to memory", the save's `spells_memorized` |
| Scroll items for every spell + sprites | ✅ generated at migrate (`cmd/codex/migration/scrolls.go`, 104 items); 8 school sprites |
| Scrolls in loot tables | ✅ `random-scroll` entries, weighted by spell level: 9 caster monsters, 7 treasure nodes |
| Scrolls in shops | ✅ 4 basics each at the court arcanist (Mage Tower) and the temple almoner (Dawnflame) |

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
4. **The `spellbook` item holds scrolls; memory holds spells.** The item is a
   20-slot scroll container (`allowed_types: ["spell-scroll"]`) for anyone. A
   wizard commits a scroll in it to memory with the container's "Commit to
   memory" action, and the spell goes into the save's `spells_memorized`. It
   does **not** depend on carrying the book: losing the book on defeat never
   costs a wizard their spells.

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
| `scribe_nature` | druid, ranger | The Green Mortar, Verdant (`green_mortar`): the herbalist who sells druid and ranger reagents. Thalindra's Grove is an NPC's private quarters, not a station |
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
- **Sprites**: one per school of magic (`scroll-<school>.png`: abjuration,
  conjuration, divination, enchantment, evocation, illusion, necromancy,
  transmutation), the same rolled scroll with the school's ribbon, seal and
  sigil. The 3 hand-written scrolls keep their own art. Not one per spell.
- **Loot**: a loot entry `{"item": "random-scroll", "scroll": {"max_level": 1,
  "classes": ["cleric"], "schools": ["necromancy"]}}` rolls a random scroll
  matching the filter (every field optional). Within it, a spell's chance follows
  its level: `loot_level_weights` in scroll-scribing.json (cantrip 8, level 1 6,
  level 2 2, level 3 1), so common spells drop often and higher ones rarely.
  Both loot engines resolve it (monster loot in `combat/loot.go`, POI loot in
  `poi/walker.go`), and the validators accept it.
  - **Monsters** (a "scroll" tier, chance per loot roll): apprentice wizard 15%
    (wizard, ≤1) · acolyte 6% (cleric, ≤1) · cultist 5% (necromancy/enchantment,
    ≤1) · cult fanatic 8% (cleric, ≤2) · druid 8% (druid, ≤2) · kobold shaman 6%
    (sorcerer/wizard, ≤1) · satyr 4% (enchantment/illusion, ≤1) · wight and
    wraith 4% (necromancy, ≤2).
  - **Treasure nodes**: Crypt of the Ancients sarcophagus 15% (necromancy/
    abjuration, ≤2) · Mournful Catacombs ossuary 15% (necromancy, ≤2) and inner
    sanctum 15% (cleric, ≤2) · Sunken Temple altar 20% (cleric, ≤2) · Abandoned
    Watchtower leader's desk 15% (≤1) · Whispering Caverns hidden nook 12% (≤1) ·
    Forgotten Battlefield commander 12% (abjuration/evocation, ≤1).
- **Shops**: the court arcanist at the Mage Tower stocks fire bolt, magic
  missile, shield and mage armor; the temple almoner at the Dawnflame stocks
  sacred flame, cure wounds, bless and healing word (2 each, weekly restock).

## Using a scroll (built; rules unchanged)

Any class with mana can read any scroll: it costs the spell's mana and is
consumed, skipping the known, prepared and component gates. 5e limits scrolls
to your class list; this game stays looser on purpose. Non-casters (no mana)
can't use scrolls until mana potions exist.

## Wizard copying: Commit to memory (built)

Part of [spell-learning.md](spell-learning.md) (Wizard). The scroll side:

- **Where:** a scroll inside a `spellbook` container. Its menu offers "Commit to
  memory" to wizards (`memorize.classes` in scroll-scribing.json).
- **What:** a wizard spell of a level the wizard has slots for, not already
  known. Cantrips can't be memorized.
- **Cost:** 5,000 gold and 2 hours per spell level (PHB: 50 gp + 2 h per
  level), and the scroll is used up. No roll (PHB has none). The menu asks the
  server first (`dry_run`) and shows the real cost before you confirm.
- **Storage:** the spell goes into the save's `spells_memorized` (codec key
  `sm`). A memorized spell is a choice, so it's stored, and it's kept apart from
  `KnownSpells` so it doesn't use up a level-up pick.

So the wizard's loop is: find a scroll (loot or a shop), put it in the
spellbook, commit it to memory, and from then on scribe more of it.

## Build

1. **Engine:** `game/spells/scribe.go` (`Scribable`, `Scribe`; it sits beside
   casting to reuse its component and focus rules) on top of `game/spellbook`.
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
   - 8 school sprites.
4. **UI:** a "Scribe scrolls" panel at a station building (like the shop
   panels), showing chance, cost, time and missing items.
5. **Wizard copy:** `POST /api/progression/memorize-scroll {container_slot,
   container_slot_type, from_container_slot, dry_run}`, plus "Commit to memory"
   in the spellbook container's menu (wizards only).

**Tests:**
- A level-3 druid can scribe barkskin but not moonbeam without its component.
- A wizard can't scribe what they haven't copied.
- Failure spends the materials.
- Committing a scroll to memory teaches the spell, spends the gold and the
  scroll, and leaves level-up picks alone.
- Random scrolls honour the filter and favour low levels.

## Not in scope

- General item crafting (beta).
- Upcasting.
- Batch scribing.
- NPC scribes for hire.
- Scroll variants (empowered, quickcast).
