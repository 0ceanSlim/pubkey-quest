# Item Mechanics Proposals

Items whose real-world/D&D role needs an engine mechanic that doesn't exist yet. Data
is shaped as close to correct as the current schema allows; this doc records what's
still missing and a minimal way to model it. Flag such items `[~]` in
`item-refinement-progress.md`, not `[x]`.

---

## Restraint / entangle mechanic (`net`, `caltrops`)

**Items affected:** `net` (Martial Ranged Weapon), `caltrops` (Adventuring Gear, already
carries a bespoke `restrain: "1d3"` key per the report).

**Missing mechanic:** neither weapon nor consumable-terrain restraint exists in the
effects/combat engine today. In 5e:
- **Net**: on a hit, a Large-or-smaller creature is Restrained until it uses an action
  to make a Str check (escape DC 10) or someone else uses an action to free it (or 5
  slashing damage destroys the net, AC 10). No damage on hit.
- **Caltrops**: a creature entering/starting turn in the area must make a DC 15 Dex
  save or stop moving and take 1 piercing damage (not 1d3 — the existing `restrain`
  value is actually the wrong die entirely per 5e SRD, but that's a separate balance
  question from the mechanic gap).

**Data shape chosen for `net`** (this batch): kept `type: "Martial Ranged Weapons"`,
`damage` field present but set to `"0"` is what the schema currently requires (UNIV
`damage` on this type) — **left `damage: "0"` as the least-wrong placeholder** since
removing the key entirely would violate the type's universal schema and the report
flagged `damage:"0"` as a units bug, not as "delete this key." Fixed the actual bugs
(dropped bogus ammo ref, fixed the range_long unit error). The `restraint` tag +
descriptive `notes` are the current best-effort encoding of "this doesn't deal damage,
it restrains."

**Minimal engine hook proposal:** add a generic `on_hit_effect` (or reuse the existing
`effects` array shape from Potions) to weapon items, e.g.:
```json
"on_hit_effect": { "condition": "restrained", "escape_dc": 10, "escape_check": "str", "duration": null }
```
applied by the combat engine on a successful attack roll instead of rolling `damage`.
This would let `net` express "0 damage, apply restrained" without a fake damage number,
and would let `caltrops` (already terrain/thrown, not a wielded weapon) express its
save-or-restrain as a proper effect rather than the bespoke `restrain` key. Both items
could then share one `condition: "restrained"` status effect definition in the effects
system (`game-data/effects/`), consistent with how other statuses are data-driven.

**Status:** `net` flagged `[~]` in progress log. `caltrops` priced (0→100, 1gp bag of 20
per PHB) in the Adventuring Gear batch but its `restrain: "1d3"` bespoke key was left
as-is (schema shape unchanged) — same missing mechanic as `net`, cross-referenced here
rather than duplicated. Both items would share one `condition: "restrained"` effect once
the `on_hit_effect`/save-or-restrain hook above is built.

---

## `spellbook.allowed_types` points at a non-existent item type

**Item affected:** `spellbook` (Adventuring Gear, container).

**Issue:** `allowed_types: ["spell-scroll"]` — no item in the catalog has
`type: "spell-scroll"` (real types are `Potion`, `Spell Component`, `Ammunition`, etc.,
Title Case). As shipped, this container can **never accept anything by type match**,
because `allowed_types` is matched against the literal `type` string
(`cmd/server/game/inventory/containers.go:122`). This is a real functional gap, not just
a naming nit — a spellbook currently can't hold anything.

**Not fixed this batch** (per the propose-don't-mangle rule: inventing a new item `type`
value is a schema decision, not a data-authoring one). Two ways to close it, either is
minimal:
1. **Add a real `Spell Scroll` item type** (Title Case, matching convention) with its own
   scroll items (`scroll-fireball`, etc.) — the "correct" 5e-flavored answer, but is new
   content, not just a rename.
2. **Repoint `spellbook.allowed_types` at an existing type** that already models
   "a single spell recorded on a medium" — none currently exists; `Spell Component`
   is the closest existing type but represents ingredients, not written spells, so this
   would be a lossy fit.

**Recommendation:** wait for a real `Spell Scroll` / consumable-scroll content pass (the
spell-refiner's domain) rather than repoint `spellbook` at a wrong-but-existing type.
Left `allowed_types` untouched this batch; `spellbook` flagged `[~]` in the progress log
for this reason (fully priced/described/tagged otherwise — only this cross-type wiring
is outstanding).

---
