# Playtest notes — fresh start, 2026-10-08

Running list while the maintainer plays from a new character. Investigate only when asked.

## Blockers

1. **Game screen loads empty after the intro.** No log, inventory, equipment, nav buttons,
   building or people lists, and no other game data. HP/mana read `0/0`, XP `0/100`, clock
   `--:--`, and the character name shows as "--Character --".
   - Console: `inventoryInteractions.js:950 Uncaught ReferenceError: initializeInventoryInteractions is not defined`
   - Suspect: the `ef1a97a` cleanup ("delete the dead drag layer…") removed a function that
     `inventoryInteractions.js` still calls at module load. The uncaught error likely stops
     the rest of game init.
   - **Fixed:** confirmed. `initializeInventoryInteractions` was live, not dead — it closes
     the item context menu on outside clicks and blocks the browser's right-click menu over
     item slots. Restored. The other `ef1a97a` deletions checked clean (no other
     still-called function was removed).

## UI

2. **Character-select card: unreadable stats.** The stat labels and values
   (STR/DEX/CON/INT/WIS/CHA) are grey text on the yellow selected-card background. The
   TOTAL box is fine.
   - Seen on the "Start New Adventure" card: Human Druid · Farmer, Lawful Neutral.
   - **Fixed:** `renderStats` treated the gold card's dark-text flag as "muted". It now has
     a `gold` tone with dark stats; the TOTAL inset keeps light text.

3. **Combat started from an encounter shows no combat buttons** (tavern brawler: failed
   the Resolve check → fight). Reload shows them.
   - Cause (likely): NPC dialogue hides the whole `#action-buttons` bar behind its option
     strip. A fight that starts while a conversation is open (or before the bar is shown
     again) renders its buttons into the hidden bar.
   - **Fixed:** `enterCombatMode` closes an open NPC dialogue and un-hides the bar before
     drawing the combat buttons.

## Data check (asked: did Codex break game data / migration?)

No. A from-scratch `--migrate` succeeds and `--validate` reports 0 errors (the warnings are
known missing art / consumable effects). `--check-schema` reports 29 reference blocks and
7 NPC-needs-creating warnings, identical to the commit before the last game-data change
(`ef37549^`). Those are the pre-existing M7 broken refs, not new. All Go tests, including
`tests/content`, pass against the fresh DB.

## Fine

- The intro flow went through without problems.
