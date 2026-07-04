# Item Refinement Progress

Batch-by-batch checklist. Status: `[ ]` todo / `[x]` done / `[~]` needs-mechanic (see
`docs/draft/item-mechanics-proposals.md`). Grouped by type per
`docs/draft/item-report.md` §1. 209 items total.

Read `docs/draft/item-conventions.md` before every batch; append to it whenever a new
reusable ruling is made.

---

## Simple Melee Weapons (10/10 done — Batch 1)

- [x] `club` — value 0→100 (1gp×100); tags: +weapon/equipment/simple-melee (kept light)
- [x] `dagger` — value 0→200 (2gp×100); tags: +weapon/equipment/simple-melee (kept finesse/light/thrown)
- [x] `greatclub` — value 0→200 (2gp×100); description written; tags: +weapon/equipment/simple-melee (kept two-handed)
- [x] `handaxe` — value 0→500 (5gp×100); description written; tags: +weapon/equipment/simple-melee (kept light/thrown)
- [x] `javelin` — value 0→500 (5gp×100); description written; **gear_slot "ammo"→"hands"** (held thrown weapon, same bug pattern as dart); tags: +weapon/equipment/simple-melee (kept thrown)
- [x] `light-hammer` — value 0→200 (2gp×100); description written; tags: +weapon/equipment/simple-melee (kept light/thrown)
- [x] `mace` — value 5→500 (was stale raw-gp, not ×100; fixed); description written; tags: +weapon/equipment/simple-melee
- [x] `quarterstaff` — value 0→200 (2gp×100); description written; tags: +weapon/equipment/simple-melee (kept versatile)
- [x] `sickle` — value 0→100 (1gp×100); description written; tags: +weapon/equipment/simple-melee (kept light)
- [x] `spear` — value 0→100 (1gp×100); description written; tags: +weapon/equipment/simple-melee (kept thrown/versatile)

## Simple Ranged Weapons (4/4 done — Batch 1)

- [x] `crossbow-light` — value 0→2500 (25gp×100); tags: +weapon/equipment/simple-ranged (kept ammunition/loading/two-handed)
- [x] `dart` — value 0→5 (5cp×100); description written; **gear_slot "ammo"→"hands"**; dropped bogus `ammunition: "arrows"` key; tags rebuilt: weapon/equipment/simple-ranged/finesse/thrown
- [x] `shortbow` — value 0→2500 (25gp×100); tags: +weapon/equipment/simple-ranged (kept ammunition/two-handed)
- [x] `sling` — value 0→100 (1gp×100); description written; tags: +weapon/equipment/simple-ranged (kept ammunition)

## Martial Melee Weapons (18/18 done — Batch 1)

- [x] `battleaxe` — value 0→1000 (10gp×100); description written; tags: +weapon/equipment/martial-melee (kept versatile)
- [x] `flail` — value 0→1000 (10gp×100); description written; tags: +weapon/equipment/martial-melee
- [x] `glaive` — value 0→2000 (20gp×100); description written; tags: +weapon/equipment/martial-melee (kept heavy/reach/two-handed)
- [x] `greataxe` — value 0→3000 (30gp×100); description written; tags: +weapon/equipment/martial-melee (kept heavy/two-handed)
- [x] `greatsword` — value 0→5000 (50gp×100); description written; tags: +weapon/equipment/martial-melee (kept heavy/two-handed)
- [x] `halberd` — value 0→2000 (20gp×100); description written; tags: +weapon/equipment/martial-melee (kept heavy/reach/two-handed)
- [x] `lance` — value 0→1000 (10gp×100); description written; tags: +weapon/equipment/martial-melee (kept reach/topple/heavy/two-handed)
- [x] `longsword` — value 0→1500 (15gp×100); description already good (kept); tags: +weapon/equipment/martial-melee (kept versatile)
- [x] `maul` — value 0→1000 (10gp×100); description written; tags: +weapon/equipment/martial-melee (kept heavy/two-handed)
- [x] `morningstar` — value 15→1500 (was stale raw-gp; fixed); description written; tags: +weapon/equipment/martial-melee
- [x] `pike` — value 0→500 (5gp×100); description written; tags: +weapon/equipment/martial-melee (kept heavy/reach/two-handed)
- [x] `rapier` — value 0→2500 (25gp×100); description written; tags: +weapon/equipment/martial-melee (kept finesse)
- [x] `scimitar` — value 0→2500 (25gp×100); description written; tags: +weapon/equipment/martial-melee (kept finesse/light)
- [x] `shortsword` — value 0→1000 (10gp×100); description written; tags: +weapon/equipment/martial-melee (kept finesse/light)
- [x] `trident` — value 0→500 (5gp×100); description written; tags: +weapon/equipment/martial-melee (kept thrown/versatile)
- [x] `war-pick` — value 5→500 (was stale raw-gp; fixed); description written; tags: +weapon/equipment/martial-melee
- [x] `warhammer` — value 0→1500 (15gp×100); description written; tags: +weapon/equipment/martial-melee (kept versatile)
- [x] `whip` — value 0→200 (2gp×100); description written; tags: +weapon/equipment/martial-melee (kept finesse/reach)

## Martial Ranged Weapons (5/5 done — Batch 1)

- [x] `blowgun` — value 1→1000 (10gp×100, was stale raw-gp); description written; **stack 25→1** (held weapon, not stackable ammo); tags: +weapon/equipment/martial-ranged (kept ammunition/loading)
- [x] `crossbow-hand` — value 150→7500 (75gp×100, was stale/wrong scale); tags: +weapon/equipment/martial-ranged (kept ammunition/loading/light)
- [x] `crossbow-heavy` — value 0→5000 (50gp×100); tags: +weapon/equipment/martial-ranged (kept ammunition/loading/two-handed/heavy)
- [x] `longbow` — value 0→5000 (50gp×100); description written; tags: +weapon/equipment/martial-ranged (kept ammunition/heavy/two-handed)
- [~] `net` — value 0→100 (1gp×100); description written; added missing `gear_slot: "hands"`; fixed `range_long` "320"→"3" (units bug); dropped bogus `ammunition: "arrows"`; kept `damage: "0"` (schema-required field, see proposal) + `restraint` tag; **restraint/entangle mechanic has no engine hook — proposal filed in item-mechanics-proposals.md**; tags: +weapon/equipment/martial-ranged (kept thrown/restraint)

---

## Remaining type groups (not started)

### Ammunition (0/4)
- [ ] `arrows` — value 5 may be stale (5gp per PHB is for 20 arrows = should check ×100 scale: 1gp/20 = 5cp/arrow bundle; verify against sling-bullet/blowgun-needle/crossbow-bolts together as one batch)
- [ ] `blowgun-needle`
- [ ] `crossbow-bolts`
- [ ] `sling-bullet` — currently `value: 0`, needs pricing too (flagged in report as part of the "37" but is Ammunition not Weapons — do in the Ammunition batch)

### Adventuring Gear (0/64)
Includes containers (`pouch`, `quiver`, `backpack`, `barrel`, `chest`, `sack`,
`case-map-and-scroll`), `caltrops` (restraint — cross-ref net's proposal), `spellbook`
(needs the `allowed_types: ["spell-scroll"]` type decision), `pole` (already has a good
description, improvised-weapon tags present), and the bulk of "NEEDS DESCRIPTION" +
empty-tags items live here.

### Musical Instrument (0/10)
`bagpipe` missing `performance` field + has empty tags — the one gap the report called out.

### Armor Set (0/10)
10 image-path basename mismatches to note (not fix — sprites exist, just flag); rarity/
value coherence check per report §6 (`breastplate` vs `halfplate-cuirass` tier issue
lives in Medium/Heavy Armor, not here, but sets bundle-price off their pieces).

### Heavy Armor (0/9), Medium Armor (0/8), Light Armor (0/6)
`splint-greaves` needs `"set": "splint-greaves"` back-reference (report §4, asymmetric
link) — schema fix, do in the armor batch. Rarity/value tier reconciliation
(`breastplate` common@40k vs `halfplate-cuirass` rare@45k) — balance judgment call for
that batch.

### Pack (0/7)
Bundle-price check against contents; sprite gaps already noted in report (all 7 packs
missing sprites — ART, not data).

### Tools (0/7), Gaming Set (0/4)
Mostly base-field-only; `pick-miners` (Tools, thrown-weapon-like fields) needs a look —
missing sprite too (ART).

### Arcane Focus (0/5), Druidic Focus (0/4), Holy Symbol (0/3)
`substitues`→`substitutes` typo fix (4 Arcane Focus items) + decide fate of the
(all-empty) substitutes/provides-adjacent key across all 12 foci — one convention
across Arcane+Druidic+Holy.

### Potion (0/4), Food (0/2)
`healing` sprite missing (ART); `greater-healing`/`superior-healing`/`supreme-healing`
value tier check (50000 for supreme — flagged as priciest "common" outlier in report §6).

### Spell Component (0/24)
Cross-reference with spell-refiner per report §4 (11 unused components) before any
value/tag changes — coordinate, don't just tag in isolation.

### currency (0/1)
`gold-piece` — the type-casing one-off (`currency` vs Title Case) and the 1e12 stack
sentinel; likely leave as-is with a documented rationale, not a "fix."

---

## Sprite / asset gaps (maintainer follow-up, not data work)

No sprite work done or needed this batch — all weapon sprites already exist
(confirmed: `battleaxe.png`, `dagger.png`, etc. all present, validator raised zero new
image warnings for any item touched). The 28 missing sprites + 10 armor-set basename
mismatches from the report are unchanged and remain ART/asset backlog for the
maintainer (see report §5). Will re-flag per-type as those batches come up.

---

## Batch log

**Batch 1 (this batch) — ALL zero-value weapons + stale non-zero weapon prices:**
Simple Melee (10), Simple Ranged (4), Martial Melee (18), Martial Ranged (5) = 37
weapon items priced/described/tagged. `--validate`: 0 errors, 32 pre-existing warnings
(unchanged — all sprite/monster/spell, none newly introduced). See
`docs/draft/item-conventions.md` for pricing anchors and `docs/draft/
item-mechanics-proposals.md` for the `net` restraint proposal.

**Weapons: 37/37 done (32 `[x]` + 1 `[~]` = `net`).**
**Overall catalog: 37/209 items refined this batch; 172 remaining** (Ammunition,
Adventuring Gear, Armor×3 tiers + Armor Set, Musical Instrument, Pack, Tools, Gaming
Set, foci×3, Potion, Food, Spell Component, currency).
