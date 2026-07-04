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

### Armor Set (10/10 done — Batch 2)
- [x] `chainmail-set` — value 12000→7500 (stale, was ×1.6 too high; 75gp×100); rarity uncommon→common; contents/set_bonus unchanged
- [x] `halfplate-set` — value 75000 already correct (750gp×100 = cuirass 45000+greaves 30000); rarity rare→common
- [x] `hide-set` — value 1000 already correct (10gp×100 = cuirass 600+chaps 400); rarity already common
- [x] `leather-set` — value 1000 already correct (10gp×100 = vest 600+leggings 400); rarity already common
- [x] `padded-set` — value 500 already correct (5gp×100 = gambeson 300+leggings 200); rarity already common
- [x] `plate-set` — value 150000 already correct (1500gp×100 = cuirass 90000+greaves 60000); rarity rare→common
- [x] `ringmail-set` — value 3000 already correct (30gp×100 = hauberk 1800+chausses 1200); rarity already common
- [x] `scalemail-set` — value 5000 already correct (50gp×100 = cuirass 3000+greaves 2000); rarity uncommon→common
- [x] `splint-set` — value 20000 already correct (200gp×100 = cuirass 12000+greaves 8000); rarity uncommon→common
- [x] `studded-leather-set` — value 4500 already correct (45gp×100 = vest 2700+leggings 1800); rarity already common

### Heavy Armor (9/9 done — Batch 2), Medium Armor (8/8 done — Batch 2), Light Armor (6/6 done — Batch 2)
- [x] `chainmail-chausses` — value 4800→3000 (stale; 60/40 split of corrected 7500 set total); rarity uncommon→common
- [x] `chainmail-hauberk` — value 7200→4500 (stale; 60/40 split of corrected 7500 set total); rarity uncommon→common
- [x] `plate-cuirass` — value 90000 already correct (60% of 150000); rarity rare→common
- [x] `plate-greaves` — value 60000 already correct (40% of 150000); rarity rare→common
- [x] `ringmail-chausses` — value 1200 already correct; rarity already common
- [x] `ringmail-hauberk` — value 1800 already correct; rarity already common
- [x] `shield` — value 1000 already correct (10gp×100); description written; tags +shield
- [x] `splint-cuirass` — value 12000 already correct (60% of 20000); rarity uncommon→common
- [x] `splint-greaves` — value 8000 already correct (40% of 20000); rarity uncommon→common; `set` backref already present (fixed in an earlier pass, confirmed symmetric)
- [x] `breastplate` — value 40000 already correct (400gp×100); rarity already common; description written; tags +medium
- [x] `chain-shirt` — value 5000 already correct (50gp×100); rarity already common; description written; tags +medium
- [x] `halfplate-cuirass` — value 45000 already correct (60% of 75000); rarity rare→common
- [x] `halfplate-greaves` — value 30000 already correct (40% of 75000); rarity rare→common
- [x] `hide-chaps` — value 400 already correct (40% of 1000); rarity already common
- [x] `hide-cuirass` — value 600 already correct (60% of 1000); rarity already common
- [x] `scalemail-cuirass` — value 3000 already correct (60% of 5000); rarity uncommon→common
- [x] `scalemail-greaves` — value 2000 already correct (40% of 5000); rarity uncommon→common
- [x] `leather-leggings` — value 400 already correct; rarity already common
- [x] `leather-vest` — value 600 already correct; rarity already common
- [x] `padded-gambeson` — value 300 already correct; rarity already common
- [x] `padded-leggings` — value 200 already correct; rarity already common
- [x] `studded-leather-leggings` — value 1800 already correct; rarity already common
- [x] `studded-leather-vest` — value 2700 already correct; rarity already common

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

No sprite work done or needed in Batch 1 (weapons) — all weapon sprites already exist.

**Batch 2 (armor) sprite notes:** confirmed unchanged from report §5 — the 10 Armor Set
`image` paths point at real, differently-named shared sprites (`chainmail-set` →
`chain-mail.png`, `halfplate-set` → `half-plate.png`, `hide-set` → `hide.png`,
`leather-set` → `leather.png`, `padded-set` → `padded.png`, `plate-set` → `plate.png`,
`ringmail-set` → `ring-mail.png`, `scalemail-set` → `scale-mail.png`, `splint-set` →
`splint.png`, `studded-leather-set` → `studded-leather.png`) and render fine — left
untouched per instructions (target exists, only the `{id}.png` convention is broken,
which is an ART decision not a data fix). Validator still flags these 10 plus 3 armor
*piece* sprites that are genuinely missing on disk: `halfplate-greaves.png`,
`hide-chaps.png`, `ringmail-hauberk.png` — these are real asset gaps (not path bugs;
no `{id}.png` exists for them anywhere), noted for the maintainer, not fabricated.

---

## Batch log

**Batch 1 — ALL zero-value weapons + stale non-zero weapon prices:**
Simple Melee (10), Simple Ranged (4), Martial Melee (18), Martial Ranged (5) = 37
weapon items priced/described/tagged. `--validate`: 0 errors, 32 pre-existing warnings
(unchanged — all sprite/monster/spell, none newly introduced). See
`docs/draft/item-conventions.md` for pricing anchors and `docs/draft/
item-mechanics-proposals.md` for the `net` restraint proposal.

**Weapons: 37/37 done (32 `[x]` + 1 `[~]` = `net`).**

**Batch 2 (this batch) — Armor (Light/Medium/Heavy/Armor Set):**
Light Armor (6), Medium Armor (8), Heavy Armor (9), Armor Set (10) = 33 armor items.
Fixed one stale-value cluster (`chainmail-hauberk`/`chainmail-chausses`/
`chainmail-set`, ×1.6 too high — same "stale non-×100 value hides in the non-zero
set" pattern as Batch 1). Reconciled the rarity↔value contradiction flagged in report
§6: all mundane PHB armor set to `common` (rarity tracks magic status, not price tier,
per existing `spyglass`/`supreme-healing` precedent) — downgraded 15 armor
pieces/sets from `uncommon`/`rare` to `common`. Wrote 3 missing descriptions
(`breastplate`, `chain-shirt`, `shield`) and backfilled tags on those same 3 (only
had a bare `equipment` tag, now `+medium`/`+medium`/`+shield`). Confirmed the
`splint-greaves` set-backref asymmetry from report §4 was already fixed (present,
symmetric) — no action needed. Verified all 10 set-bundle values equal the sum of
their 2 pieces, and the plate-established 60/40 chest/leg split ratio holds exactly
across every multi-piece armor. `--validate`: 0 errors, 32 warnings (same
pre-existing set as Batch 1 — no new warnings from this batch's edits).

**Armor: 33/33 done (all `[x]`, no `[~]` — fully expressible in current schema).**
**Overall catalog: 70/209 items refined so far (37 weapons + 33 armor); 139
remaining** (Ammunition, Adventuring Gear, Musical Instrument, Pack, Tools, Gaming
Set, foci×3, Potion, Food, Spell Component, currency).
