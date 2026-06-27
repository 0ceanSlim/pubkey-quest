# UI / Inventory issue backlog

A running list of interaction bugs and UI-redesign needs, captured from
playtesting. These are mostly frontend (`src/`), cut across each other, and
overlap the M3 quest UI and the M6 presentation overhaul. Not yet scheduled —
this is the markup so we can pull items into a dedicated UI/UX track.

## Interaction model — the through-line
Click-to-act and drag-and-drop should **both** work everywhere they make sense,
and both should work on **mobile touch**. Right now each surface supports a
different subset, which is the root of most of these bugs.

## Inventory & containers
- **Ground items broken** — dropping and picking items up off the ground did not
  work last tested (`src/ui/groundItems.js`, the `drop_item` / `pickup_item`
  actions). Verify the full round-trip.
- **Component pouch can't be filled** — it's a container meant to hold spell
  component materials, but components can't be put into it. Want **both**: click
  -to-add (when the container is open) **and** drag-and-drop into it.
- **Containers in general are buggy** — needs a **container UI redesign** plus a
  general correctness fix pass (`src/systems/containers.js`, the
  `add_to_container` / `remove_from_container` actions).
- **Vault: click path broken** — vault works via drag-and-drop but **not** via
  clicking (the inverse of some other surfaces). Click-to-deposit/withdraw
  should work too.
- **Eating from a stack consumes the whole stack** — ate 1 of a 3× rations
  stack and all 3 disappeared. The `use_item` / consume path decrements the
  whole stack (or removes the slot) instead of one unit. Same area as the other
  consume-quantity bugs.

## Mobile / touch
- **Touch controls are inconsistent** across the board and need a dedicated pass.
- **Drag-and-drop must work on touch**, not just mouse — currently unreliable.

## UI redesign / richness passes
- **Quest tab — full redesign** (ties directly into the M3 quest-log UI: active
  quests + stage/objective progress, completed list, QP total).
- **Equipment UI — richer**: show the stats the equipped gear actually produces
  (AC, main/off-hand attack + damage, ranged + ammo) — overlaps M6.
- **Item info tabs — another pass** (the item detail / info panels).
- **General UI pass** — broad polish; much of it overlaps M6 §7.

## How this ties to the milestones
- The **quest tab** redesign is the visible half of M3 slice 4b's quest UI.
- **Equipment / item-info / general polish** are M6 (presentation overhaul).
- **Inventory/container/ground-item/vault correctness** and **mobile touch +
  drag-and-drop parity** are their own interaction-layer track — not currently a
  milestone; worth carving one out, since the click-vs-drag-vs-touch
  inconsistency is a single root cause behind several of these.
