# Spell learning

How casters gain spells after character creation. The starting lists stay the
D&D 5e reference (`game-data/systems/new-character/starting-spells.json`); this
covers everything after that.

Engine: `cmd/server/game/spellbook/` (known spells + slot counts at a level).

## Status

| Caster type | Classes | Rule | Status |
|---|---|---|---|
| Whole-list | cleric, druid (`spells_known` = 99 in `spell-progression.json`) | Know every class spell of each spell level they have slots for | ✅ **built** (derived, nothing stored) |
| Known | bard, sorcerer, warlock, paladin, ranger | Pick new spells at level-up, up to `spells_known[level]` | ⬜ to build |
| Spellbook | wizard | 2 free spells per level, plus copying scrolls | ⬜ to build |

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
- **Scrolls:** copy a wizard spell from a scroll into the spellbook. It costs
  time and gold (5e: 2 hours and 50 gp per spell level, rescaled to our gold)
  and uses the scroll up. Ties into scroll crafting (M5.5).
- **Storage:** appended to `KnownSpells`.

## Not in scope

- **Upcasting** (a level-1 spell in a level-2 slot for more effect) is a separate,
  smaller follow-up.
- **Cantrips known** growing with level (druid 2 → 3 at level 4): needs the
  known-casters pick flow first.
