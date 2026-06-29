# UI / Inventory issue backlog

A running list of interaction bugs and UI-redesign needs, captured from
playtesting. These are mostly frontend (`src/`), cut across each other, and
overlap the M3 quest UI and the M6 presentation overhaul. Not yet scheduled —
this is the markup so we can pull items into a dedicated UI/UX track.

## Interaction model — the through-line
Click-to-act and drag-and-drop should **both** work everywhere they make sense,
and both should work on **mobile touch**. Right now each surface supports a
different subset, which is the root of most of these bugs.

## Resolved in the 2026-06 playtest pass
- ✅ **Eating a stack consumed the whole stack** — fixed; `use_item` now
  decrements one unit (root cause: split stacks store `quantity` as int, the
  consume reader only handled float64 → saw 0 → wiped the slot).
- ✅ **Combat loot / death items "vanished"** — fixed; the inventory grid rendered
  by an `item.slot` field that server-placed items (loot, death-kept) lack, so it
  silently skipped them. Render now falls back to the array index.
- ✅ **Death only kept the backpack** — fixed; valuation read the wrong field
  (`cost` vs canonical `value`) so everything scored 0; the backpack also now
  competes for the top-3 like any other item.
- ✅ **Item-info panel** — redesigned (rarity-aware header + glow, sectioned
  stats, lore, tag descriptions on hover/click, sized to the scene). Further
  polish optional.
- ✅ **Level number + XP bar froze at level 1** — fixed; level is derived from XP
  client- and server-side.

## Inventory & containers
- **Ground items broken** — dropping and picking items up off the ground did not
  work last tested (`src/ui/groundItems.js`, the `drop_item` / `pickup_item`
  actions). Verify the full round-trip.
- **Component pouch can't be filled** — it's a container meant to hold spell
  component materials, but components can't be put into it. Want **both**: click
  -to-add (when the container is open) **and** drag-and-drop into it.
- **Drag onto a closed container** — dragging an item over a container that sits
  in a general slot should drop the item *into* that container, without opening
  it first.
- **Open-container modal redesign** — the open-container view should use the same
  win95 inventory-square grid as the inventory tab (it currently looks foreign);
  same cells, same click/drag behavior, just rendered in the scene modal.
- **Containers in general are buggy** — correctness fix pass alongside the
  redesign (`src/systems/containers.js`, the `add_to_container` /
  `remove_from_container` actions).
- **Vault: click path broken** — vault works via drag-and-drop but **not** via
  clicking (the inverse of some other surfaces). Click-to-deposit/withdraw
  should work too.
- **Trading (shop buy/sell) needs the same revisit** — the transaction surface
  wants the inventory-square styling + click/drag parity, just like containers.

## Mobile / touch
- **Touch controls are inconsistent** across the board and need a dedicated pass.
- **Drag-and-drop must work on touch**, not just mouse — currently unreliable.

## UI redesign / richness passes
- ✅ **Quest tab** — journal redesign shipped (collapsible categories, status
  dots, in-scene detail modal).
- ✅ **Item info panel** — redesigned (see Resolved above); revisit only if needed.
- **NPC dialogue → scene modal, animated** — move NPC speech out of the corner
  toast into a JRPG-style box over the scene's lower third (portrait + name plate),
  with the text **typed out / animated** (typewriter). Option buttons keep their
  current bottom-strip home. One renderer reused for POI/encounter nodes. This is
  the M6 dialogue item (roadmap §M6 / §7) — the speech-box + typewriter is the
  concrete spec.
- **Equipment UI — richer**: show the stats the equipped gear actually produces
  (AC, main/off-hand attack + damage, ranged + ammo) — overlaps M6.
- **Inventory rarity** ✅ added (inset glow per rarity; outer glow for
  legendary/mythic).
- **General UI pass** — broad polish; much of it overlaps M6 §7.

## How this ties to the milestones
- The **quest tab** redesign is the visible half of M3 slice 4b's quest UI.
- **Equipment / item-info / general polish** are M6 (presentation overhaul).
- **Inventory/container/ground-item/vault correctness** and **mobile touch +
  drag-and-drop parity** are their own interaction-layer track — not currently a
  milestone; worth carving one out, since the click-vs-drag-vs-touch
  inconsistency is a single root cause behind several of these.
