# Spell learning

How casters gain spells after character creation. The starting lists stay the
D&D 5e reference (`game-data/systems/new-character/starting-spells.json`); this
covers everything after that.

Engine: `cmd/server/game/spellbook/` (known spells + slot counts at a level).

Companion draft: [spell-scroll-system.md](spell-scroll-system.md). Scribing a
scroll is gated on `spellbook.Knows`, so whatever this system teaches a caster
is exactly what they can scribe. For wizards, copying a scroll is also a way
to learn.

## Status

| Caster type | Classes | Rule | Status |
|---|---|---|---|
| Whole-list | cleric, druid (`spells_known` = 99 in `spell-progression.json`) | Know every class spell of each spell level they have slots for | ✅ **built** (derived, nothing stored) |
| Known | bard, sorcerer, warlock, paladin, ranger | Pick new spells at level-up, up to `spells_known[level]` | ✅ **built** (picks; swap still to do) |
| Spellbook | wizard | 2 free spells per level, plus copying scrolls | 🔶 level-up picks ✅ built; copying scrolls ⬜ (needs scroll scribing) |

**Picks as built** (`spellbook.Picks` / `PickOptions` / `Learn`):
- Open picks = `spells_known[level] − spells_known[1]` (the rise since level
  1) − the spells already picked (known spells off the class's creation list).
  The starting lists are longer than `spells_known[1]`, so counting the rise,
  not the total, is what keeps picks flowing.
- `GET /api/progression/spell-picks` and `POST /api/progression/learn-spell`.
- The picker is a "Learn spells" section in the allocate-points modal. The
  level-up popup says when picks are waiting.

Spell slots also grow with level now: `spellbook.SyncSlots` brings the save's
slot arrays up to the `spell-slots.json` row on load and on every level-up
(before this they stayed at the level-1 counts forever).

## Whole-list casters (built)

`spellbook.Known(save)` = `KnownSpells` (the creation snapshot) ∪ every spell
whose `classes` include the character's class and whose level is between 1 and
the highest slot level they have. Nothing new is stored. Hydration rule
(roadmap §4): it derives from class + XP.

New spells for these classes reach players with no save change: author the
spell with the class in `classes`.

## Known casters (to build)

- **When:** at level-up, if `spells_known[newLevel] > len(learned)`.
- **Choice:** any class spell of a level they have slots for, not already known.
  Picked in the level-up window, next to spend-a-point / take-a-feat.
- **Storage:** append to `KnownSpells`. A pick is a choice, so it can't be
  derived and has to be stored.
- **Endpoint:** `POST /api/progression/learn-spell {spell_id}`. It checks the
  pick count, class, slot level and that the spell isn't already known.
- **Swap (5e):** on level-up, may replace one known spell. Optional, later.
- **Open:** paladin and ranger are prepared casters in 5e, but
  `spell-progression.json` gives them counts. The data wins until we decide
  otherwise.

## Wizard (to build)

- **Level-up:** 2 picks per level from the wizard list (slot level limit as above).
- **Copying scrolls:** a wizard spell on a scroll in the inventory can be
  copied in if its level is one the wizard has slots for and it isn't known.
  It costs 5,000 gold and 2 hours per spell level (PHB: 50 gp + 2 h), has no
  roll, and consumes the scroll. Scrolls come from loot, shops, or other
  casters' scribing. Rules and endpoint: [spell-scroll-system.md](spell-scroll-system.md#wizard-copying).
- **Storage:** appended to `KnownSpells`. The copied spells don't depend on
  carrying the `spellbook` item: that item is a scroll container, and losing it
  on defeat never costs spells.

## Not in scope

- **Upcasting** (a level-1 spell in a level-2 slot for more effect) is a separate,
  smaller follow-up.
- **Cantrips known** growing with level (druid 2 → 3 at level 4): needs the
  known-casters pick flow first.

## Build order (both drafts)

1. Known-caster picks + wizard level-up picks (this doc). This makes
   `spellbook.Known` complete for every class.
2. Scroll scribing (spell-scroll-system.md), gated on `Knows`.
3. Wizard copying, which reuses the scroll items from step 2.
