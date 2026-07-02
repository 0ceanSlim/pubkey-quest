# UI / Inventory issue backlog

A running list of interaction bugs and UI-redesign needs, captured from
playtesting. Most of the interaction-layer track landed in the 2026-07
equipment/inventory hardening pass (an off-milestone interlude before M4).

## Interaction model — the through-line
Click-to-act and drag-and-drop should **both** work everywhere they make sense,
and both should work on **mobile touch**. This was the root cause of most of the
old bugs, and is now addressed by a single delegated pointer-events core
(`src/systems/slotInteractions.js`).

## Resolved in the 2026-07 equipment/inventory hardening pass
- ✅ **Unified pointer-events interaction core** (`src/systems/slotInteractions.js`)
  — one delegated listener set drives click, drag-to-move, drop, and long-press
  across the general grid, backpack, equipment, vault, and open containers, for
  **mouse and touch**. Replaces the per-surface HTML5-drag bindings (which never
  fired on touch) and the three separate drop handlers. Root-cause fix.
- ✅ **Vault click path** — click-to-deposit/withdraw works.
- ✅ **Vault away-from-home** — deposits resolve the vault from the current
  building (worked only at the home vault before).
- ✅ **Containers fill (incl. component pouch)** — click-to-add (also when open)
  and drag-in (open or closed); `allowed_types` is now actually enforced (it was
  read as a string but the JSON is an array). Fixed the container int-slot bug
  (add-after-remove) and the no-room item-loss bug (emptying an unequipped bag).
- ✅ **Drag onto a closed container** — routes the item into it.
- ✅ **Open-container modal** — restyled to match the vault (clean overlay,
  vault-square slots) and migrated onto the core.
- ✅ **Quiver → ammo slot** — equips like the backpack and keeps its ammo on
  equip/unequip; combat draws rounds from the equipped quiver (weapon-matched).
- ✅ **Equipment UI — richer** — the Equipment tab shows AC / main+off-hand
  attack+damage / ranged + ammo (quiver-aware), reusing the combat engine's math;
  equipment slots resized + rarity glow applied like inventory.
- ✅ **Hover label** — cursor-following, leads with the color-coded default action.
- ✅ **Backend test net** — `tests/inventory/` covers equip/unequip, move/stack/
  split, container add/remove, vault round-trip, drop, use, quiver ammo, plus the
  regressions above; combat ammo has white-box tests.

## Resolved in the 2026-06 playtest pass
- ✅ **Eating a stack consumed the whole stack** — `use_item` now decrements one
  unit (int-vs-float64 quantity read).
- ✅ **Combat loot / death items "vanished"** — render fell back to the array
  index for server-placed items that lack an `item.slot` field.
- ✅ **Death only kept the backpack** — valuation read the wrong field; the
  backpack now competes for the top-3 like any other item.
- ✅ **Item-info panel** — redesigned (rarity-aware header + glow, sectioned
  stats, lore, tag descriptions, sized to the scene).
- ✅ **Level number + XP bar froze at level 1** — level derived from XP.
- ✅ **Quest tab** — journal redesign (collapsible categories, status dots,
  in-scene detail modal).
- ✅ **Inventory rarity** — inset glow per rarity; outer glow for legendary/mythic.

## Still open
- **Ground items** — `handlePickupItemAction` is a TODO stub (returns success but
  never adds the item), so drop→pickup doesn't round-trip. Implement pickup + move
  the ground UI onto the core. (`src/ui/groundItems.js`)
- **Trading (shop buy/sell)** — still on the old handlers; folds into the **shop
  revamp** (revise shop inventories + rebuild the buy/sell UI on the shared
  slot-grid with click/drag parity).
- **Manage a container while equipped** — an equipped quiver can't be opened /
  added-to in place; you fill it in a general slot then equip. Nice-to-have.
- **Settings knobs (deferred)** — toggle the hover label; mobile tap-vs-long-press
  for the context menu.
- **Dead code cleanup** — the old per-slot bind*/handleDrop*/handleLeftClick in
  `inventoryInteractions.js` are unused (bindInventoryEvents is a no-op) and can
  be deleted.

## Known correctness bugs (not UI)
- **Delta-engine sync** — the snapshot delta (`cmd/server/session/delta.go`) only
  diffs HP + top-level slots (not container contents), and the POI/combat
  endpoints are separate from the action handler's snapshot diff. Result: POI loot
  appears only on the *next* action, and a stale HP diff can show a phantom
  full-heal in combat. Needs a snapshot/delta extension (or those endpoints must
  emit inventory deltas).

## M6 (presentation) — not this track
- **NPC dialogue → scene modal, animated** — JRPG-style speech box over the
  scene's lower third (portrait + name plate), text typed out. One renderer reused
  for POI/encounter nodes. (roadmap §M6 / §7)
- **General UI polish** — broad pass, overlaps M6 §7.
