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

### Ammunition (4/4 done — Batch 4)
- [x] `arrows` — value 5→100 (1gp per bundle of 20×100, was stale); description written; tags +`ammunition` (kept `equipment`)
- [x] `blowgun-needle` — value 2→100 (1gp per bundle of 50×100, was stale); description written; tags +`ammunition` (kept `equipment`)
- [x] `crossbow-bolts` — value 5→100 (1gp per bundle of 20×100, was stale); description written; tags +`ammunition` (kept `equipment`)
- [x] `sling-bullet` — value 0→4 (4cp per bundle of 20×1); description/tags already good (`ammunition, equipment`)

### Adventuring Gear (64/64 done — Batch 3)

**Containers** (`gear_slot` present → keep `equipment` tag; schema-container fields
present but no `gear_slot` → `container` tag only, per the validator's
equipment-requires-gear_slot rule discovered this batch, see Conventions)
- [x] `backpack` — value 1000→200 (2gp×100, was stale ×5 too high); rarity uncommon→common; `effects_when_worn:["backpack-capacity"]` confirmed wired (real effect file exists); tags already `container, equipment` (has `gear_slot: "bag"`)
- [x] `pouch` — value 0→50 (5sp×100); description written; tags already `container, equipment` (has `gear_slot: "bag"`)
- [x] `quiver` — value 0→100 (1gp×100); tags already `container, equipment` (has `gear_slot: "ammo"`)
- [x] `sack` — value 50→1 (1cp×100, was stale ×50 too high); description written; tags: `container` (no gear_slot — not wearable)
- [x] `chest` — value 5→500 (5gp raw, needed ×100); tags: `container`
- [x] `barrel` — value 200 already correct (2gp×100); description written; tags: `container`
- [x] `basket` — value 40 already correct (4sp×100); description written; tags: `container`
- [x] `case-map-and-scroll` — value 1→100 (1gp×100); rarity rare→common (report's flagged contradiction); tags: `container`
- [x] `component-pouch` — value 25→2500 (25gp×100, was stale raw-gp); tags: `container`
- [~] `spellbook` — value 5000 already correct (50gp×100); description/tags already good (`container, equipment`, has `gear_slot: "hands"`); `allowed_types:["spell-scroll"]` flagged — no matching item type exists in the catalog, proposal filed in item-mechanics-proposals.md rather than inventing a type

**Restraint / hazard items**
- [~] `caltrops` — value 0→100 (1gp×100, bag of 20); tags unchanged (`restraint, thrown`); bespoke `restrain:"1d3"` key left as-is, cross-referenced to the existing `net` restraint proposal (not duplicated)
- [x] `ball-bearings` — value 1→100 (1gp×100, bag of 1000, was stale); tags: `restraint` (unchanged)
- [x] `chain` — value 5→500 (5gp×100, was stale raw-cp-scale error); tags: `restraint` (unchanged)
- [x] `hunting-trap` — value 500 already correct (5gp×100); tags +`trap`

**Light sources / fuel**
- [x] `candle` — value 1 already correct (1cp); tags unchanged (`light-source`)
- [x] `torch` — value 1 already correct (1cp); tags unchanged (`light-source`)
- [x] `lamp` — value 50 already correct (5sp×100); tags unchanged (`light-source, oil-burning`)
- [x] `lantern-hooded` — value 500 already correct (5gp×100); tags unchanged (`light-source, oil-burning`)
- [x] `lantern-bullseye` — value 1000 already correct (10gp×100); tags unchanged (`light-source, directional, oil-burning`)
- [x] `oil` — value 10 already correct (1sp×100); tags +`fuel`
- [x] `tinderbox` — value 50 already correct (5sp×100); tags +`tool`

**Consumables / thrown**
- [x] `acid` — value 2500 already correct (25gp×100); description written; tags +`consumable` (kept thrown)
- [x] `alchemists-fire` — value 5000 already correct (50gp×100); tags +`consumable` (kept thrown)
- [x] `poison-basic` — value 10000 already correct (100gp×100); tags +`consumable` (kept poison)
- [x] `healers-kit` — value 500 already correct (5gp×100); tags +`consumable` (kept healing)
- [x] `soap` — value 2 already correct (2cp×100); tags +`consumable`

**Camping / utility gear**
- [x] `bedroll` — value 100 already correct (1gp×100); description written; tags +`camping`
- [x] `blanket` — value 50 already correct (5sp×100); description written; tags +`camping`
- [x] `waterskin` — value 20 already correct (2sp×100); description written; tags +`camping`
- [x] `mess-kit` — value 20 already correct (2sp×100); tags +`camping`
- [x] `pot-iron` — value 200 already correct (2gp×100); tags +`camping`
- [x] `block-and-tackle` — value 100 already correct (1gp×100); description written; tags +`tool`
- [x] `climbers-kit` — value 2500 already correct (25gp×100); tags unchanged (`pack`)
- [x] `crowbar` — value 200 already correct (2gp×100); tags +`tool`
- [x] `grappling-hook` — value 200 already correct (2gp×100); tags +`tool`
- [x] `hammer-sledge` — value 200 already correct (2gp×100); tags +`tool` (kept two-handed)
- [x] `ladder` — value 10 already correct (1sp×100); tags +`tool`
- [x] `piton` — value 5 already correct (5cp×100); tags +`tool`
- [x] `whetstone` — value 1 already correct (1cp×100); tags +`tool`
- [x] `rope` — value 100 already correct (1gp×100); tags +`tool`
- [x] `ram-portable` — value 400 already correct (4gp×100); added missing `damage_type: "bludgeoning"` (had `damage:"1d4"` with no type); tags +`tool`
- [x] `pole` — value 0→5 (5cp×100); description/tags already good (has `gear_slot: "hands"` from batch 1, kept `equipment` tag)

**Writing / office / misc**
- [x] `book` — value 2500 already correct (25gp×100); description written; tags +`writing`
- [x] `ink` — value 1000 already correct (10gp×100); tags +`writing`
- [x] `ink-pen` — value 2 already correct (2cp×100); tags +`writing`
- [x] `paper` — value 20 already correct (2sp×100); tags +`writing`
- [x] `parchment` — value 10 already correct (1sp×100); tags +`writing`; weight 0→0.01 (report-flagged zero-weight gap)
- [x] `sealing-wax` — value 50 already correct (5sp×100); tags +`writing`
- [x] `chalk` — value 1 already correct (1cp×100); tags +`writing`
- [x] `lock` — value 1000 already correct (10gp×100); tags +`tool`
- [x] `magnifying-glass` — value 10000 already correct (100gp×100); tags +`tool`
- [x] `mirror` — value 500 already correct (5gp×100); tags +`tool`
- [x] `nail` — value 1 already correct (homebrew, no direct PHB entry, priced by analogy to piton); tags +`tool`
- [x] `signet-ring` — value 500 already correct (5gp×100); tags +`jewelry`
- [x] `signal-whistle` — value 5→50 (5sp×100, was stale); tags +`tool`
- [x] `spyglass` — value 100000 already correct (1000gp×100); tags +`tool`
- [x] `bell` — value 100 already correct (1gp×100); rarity uncommon→common; description written; tags +`tool`
- [x] `hourglass` — value 2500 already correct (25gp×100); tags +`tool`
- [x] `bottle-glass` — value 200 already correct (2gp×100); description written; tags +`vessel`
- [x] `bucket` — value 5 already correct (5cp×100); tags +`vessel`
- [x] `flask` — value 2 already correct (2cp×100); tags +`vessel`
- [x] `jug-or-pitcher` — value 2 already correct (2cp×100); tags +`vessel`
- [x] `vial` — value 100 already correct (1gp×100); tags +`vessel`
- [x] `perfume-vial` — value 500 already correct (5gp×100); tags +`vessel`

**Related fix outside Adventuring Gear (explicitly called out in the task):**
- [x] `shovel` (Tools type) — value 2→200 (2gp raw, needed ×100); tags +`tool` (kept improvised-weapon)

### Musical Instrument (10/10 done — Batch 4)
- [x] `bagpipe` — rarity uncommon→common; added missing `performance` (base_success 55/charisma_modifier 6/difficulty hard, modeled between `viol` and `pan-flute`); tags empty→`instrument`; value 3000 already correct (30gp×100) — the one gap the report called out, now fully closed
- [x] `lute` — value 3500 already correct (35gp×100); tags +`instrument`
- [x] `lyre` — value 3000 already correct (30gp×100); tags +`instrument`
- [x] `flute` — value 200 already correct (2gp×100); description written; tags +`instrument`
- [x] `drum` — value 600 already correct (6gp×100); description written; tags +`instrument`
- [x] `horn` — value 300 already correct (3gp×100); tags +`instrument`
- [x] `pan-flute` — value 1200 already correct (12gp×100); tags +`instrument`
- [x] `viol` — value 3000 already correct (30gp×100); tags +`instrument`
- [x] `shawm` — value 200 already correct (2gp×100); tags +`instrument`
- [x] `dulcimer` — value 2500 already correct (25gp×100); tags +`instrument`

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

### Pack (7/7 done — Batch 4)
Value = sum of `contents` (item value × qty), verified every content ref resolves.
Descriptions/tags (`pack`) were already complete on all 7 — only `value` needed fixing.
- [x] `scholars-pack` — value 3902→4202 (sum: backpack 200 + book 2500 + ink 1000 +
  ink-pen 2 + lamp 50 + parchment×10=100 + tinderbox 50 + dagger 200 + rope 100)
- [x] `priests-pack` — value 2830→580 (sum: backpack 200 + blanket 50 + holy-water 150 +
  rations×2=100 + tinderbox 50 + candle×10=10 + waterskin 20)
- [x] `explorers-pack` — value 850→1000 (sum: backpack 200 + bedroll 100 +
  rations×10=500 + rope 100 + tinderbox 50 + torch×10=10 + waterskin 20 + mess-kit 20)
- [x] `entertainers-pack` — value 3300→2500 (sum: backpack 200 + bedroll 100 + bell 100 +
  lantern-bullseye 1000 + mirror 500 + oil×8=80 + rations×9=450 + tinderbox 50 +
  waterskin 20)
- [x] `dungeoneers-pack` — value 1100→1200 (sum: backpack 200 + caltrops 100 +
  crowbar 200 + oil×2=20 + rations×10=500 + rope 100 + tinderbox 50 + torch×10=10 +
  waterskin 20)
- [x] `diplomats-pack` — value 3402→2300 (sum: backpack 200 + case-map-and-scroll×2=200 +
  rations×2=100 + ink 1000 + ink-pen×5=10 + lamp 50 + oil×4=40 + paper×5=100 +
  parchment×5=50 + perfume-vial 500 + tinderbox 50)
- [x] `burglars-pack` — value 1626→1725 (sum: backpack 200 + ball-bearings 100 + rope 100 +
  bell 100 + candle×5=5 + crowbar 200 + hammer 100 + piton×10=50 + lantern-hooded 500 +
  oil×5=50 + rations×5=250 + tinderbox 50 + waterskin 20)

All 7 pack sprites still confirmed missing (ART gap, unchanged from report — noted in
Sprite section below).

### Tools (7/7 done — Batch 4), Gaming Set (4/4 done — Batch 4)
- [x] `thieves-kit` — value 2500 already correct (25gp×100); tags +`tool`
- [x] `navigator-kit` — value 2500 already correct (25gp×100); tags +`tool`
- [x] `herbalism-kit` — value 500 already correct (5gp×100); tags +`tool`
- [x] `hammer` — value 100 already correct (homebrew, by analogy to `piton`/`crowbar`
  tier — no standalone PHB "hammer" price outside artisan's-tools bundles); tags +`tool`
- [x] `crafting-kit` — value 2000 already correct (homebrew consolidated artisan's-tools
  kit, priced at the upper end of the PHB 5–30gp artisan's-tools range since it
  replaces ~9 separate tool types); tags +`tool`
- [x] `pick-miners` — value 200 already correct (homebrew "miner's pick", priced by
  analogy to `shovel` 2gp×100 — both excavation tools, no direct PHB pick price); tags
  +`tool` (kept `improvised-weapon`); missing sprite (ART, unchanged)
- [x] `shovel` — already done in Batch 3 (value 200, tags `improvised-weapon, tool`) —
  confirmed complete, no further action
- [x] `dice-set` — value 10 already correct (1sp×100); description written; tags
  +`gaming-set`
- [x] `playing-card-set` — value 500→50 (5sp×100, was stale ×10 too high); description
  written; tags +`gaming-set`
- [x] `dragonchess-set` — value 100 already correct (1gp×100); tags +`gaming-set`
- [x] `three-dragon-ante-set` — value 100 already correct (homebrew, priced by analogy
  to `dragonchess-set`'s 1gp tier); tags +`gaming-set`; missing sprite (ART, unchanged)

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

**Batch 4 (hard goods) sprite notes:** confirmed unchanged from report §5 — all 7 Packs
(`burglars-pack`, `diplomats-pack`, `dungeoneers-pack`, `entertainers-pack`,
`explorers-pack`, `priests-pack`, `scholars-pack`) plus `pick-miners` and
`playing-card-set` are still missing `{id}.png` sprites (validator confirms these as
pre-existing "Image file not found" warnings, all part of the report's original list
of 28). No sprite work performed — ART gap, not data.

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

**Batch 3 (this batch) — Adventuring Gear (all 64):**
Full type sweep in one batch (containers, restraint/hazard items, light sources/fuel,
consumables/thrown, camping/utility gear, writing/office/misc) since every item's
correct price had to be individually derived from its real D&D denomination (not a
blanket ×100 on the stored number) — the type-internal consistency benefit of doing it
as one pass outweighed splitting further. Also fixed `shovel` (Tools type, called out
explicitly in the task) since it was a one-line, unambiguous, in-scope correction.

**Pricing corrections found (both directions, verified per-item against real PHB
denomination, not just multiplying the stored value):**
- `backpack` 1000→200 (2gp — was stale ×5 too high)
- `sack` 50→1 (1cp — was stale ×50 too high)
- `chest` 5→500 (5gp raw, needed ×100)
- `component-pouch` 25→2500 (25gp raw, needed ×100)
- `chain` 5→500 (5gp raw, needed ×100 — NEW outlier found, not in the task's seed list)
- `case-map-and-scroll` 1→100 (1gp×100) + rarity rare→common
- `signal-whistle` 5→50 (5sp×100)
- `pole` 0→5 (5cp×100)
- `pouch` 0→50 (5sp×100)
- `quiver` 0→100 (1gp×100)
- `caltrops` 0→100 (1gp×100, bag of 20)
- `ball-bearings` 1→100 (1gp×100, bag of 1000 — NEW outlier found)
- `shovel` (Tools) 2→200 (2gp raw, needed ×100 — explicitly flagged in the task)

Every other gear item's stored value was independently checked against its real PHB
gp/sp/cp price and found already correct at the ×100/×10/×1 scale (see the
per-item checklist above for each one's specific D&D anchor) — nothing was left at a
stale value.

**Homebrew (no direct PHB entry), priced by analogy:** `nail` kept at 1 (1cp-equivalent,
analogous to `piton`/`chalk` bulk-fastener pricing — no PHB "nail" line item exists).

**Validator-driven tag-vocabulary correction (significant mid-batch finding):** the
codex validator enforces `Items with 'equipment' tag must have 'gear_slot' property`
(and the inverse: items **with** a `gear_slot` should carry `equipment`). Adding a blanket
`equipment` tag to every gear item (as an initial pass did) produced 60 new validator
errors, because most Adventuring Gear (sacks, tools, vessels, writing supplies, camping
gear) is **not equippable** — it has no `gear_slot` and shouldn't claim one. Corrected:
`equipment` is now reserved for items that actually carry `gear_slot`
(`backpack`/`pouch`/`quiver`/`spellbook`/`pole`, all pre-existing or fixed in earlier
batches); every other gear item instead got a real classification tag: `container`
(schema containers without a wearable slot), `tool`, `vessel` (glass/ceramic/metal
liquid-holders — distinct from schema `container`s since they lack `container_slots`),
`writing`, `camping`, `jewelry`, `fuel`, `consumable`, `restraint`, `trap`. Recorded as a
new ruling in `item-conventions.md`.

**Descriptions written (13):** `acid`, `barrel`, `basket`, `bedroll`, `bell`, `blanket`,
`block-and-tackle`, `book`, `bottle-glass`, `pouch`, `sack`, `waterskin` (+ `case-map-
and-scroll`/`component-pouch`/`spellbook` already had good descriptions, untouched).

**Rarity fixes:** `case-map-and-scroll` rare→common (report's flagged rarity/value
contradiction); `backpack` uncommon→common (mundane gear, no reason to sit above
common — same ruling as Batch 2 armor); `bell` uncommon→common (same reasoning,
found mid-batch, not in the report's original list).

**Other fixes:** `ram-portable` was missing `damage_type` despite having `damage:"1d4"`
— added `"bludgeoning"` (a real, already-used key on this schema, not invented).
`parchment` weight 0→0.01 (report-flagged zero-weight gap, given a token non-zero
value consistent with `paper`/other single-sheet items). Confirmed `backpack`'s
`effects_when_worn: ["backpack-capacity"]` is genuinely wired (real effect file at
`game-data/effects/backpack-capacity.json`, read by the equip pipeline) — no proposal
needed, it already works.

**`[~]` proposals filed (2 items, both cross-referenced to existing/new proposal
entries in `item-mechanics-proposals.md`, not duplicated mechanics):**
- `caltrops` — bespoke `restrain: "1d3"` key has no engine hook; cross-referenced to
  the existing `net` restraint proposal (same missing "on-hit/on-trigger restrain"
  mechanic).
- `spellbook` — `allowed_types: ["spell-scroll"]` points at an item `type` that doesn't
  exist in the catalog (no item has `type: "spell-scroll"`), so the container can never
  accept anything by type match today. New proposal filed: either add a real
  `Spell Scroll` item type with content, or otherwise resolve in a future spell-scroll
  content pass — not fixed by repointing at a wrong-but-existing type.

`--validate`: **0 errors**, 37 warnings (33 pre-existing unchanged: 28 sprite-not-found
+ 3 monster + 1 spell — none of this batch's doing) + 4 new informational-only
warnings (`Items with 'consumable' tag should have 'effects' property (not yet
enforced)` on `acid`/`alchemists-fire`/`healers-kit`/`poison-basic`/`soap` — the
validator itself says "not yet enforced"; these are genuinely single-use consumables
so the tag is correct, they just don't have a stat-effect payload like Potions/Food do).

**Adventuring Gear: 64/64 done (62 `[x]` + 2 `[~]` = `caltrops`, `spellbook`).**

**Batch 4 (this batch) — Hard Goods (Pack, Tools, Musical Instrument, Gaming Set,
Ammunition):**
Pack (7), Tools (7), Musical Instrument (10), Gaming Set (4), Ammunition (4) = 32 items.

**Pricing corrections found (both directions, per-item PHB derivation):**
- `arrows` 5→100 (1gp per 20-bundle×100, was stale)
- `crossbow-bolts` 5→100 (1gp per 20-bundle×100, was stale)
- `blowgun-needle` 2→100 (1gp per 50-bundle×100, was stale)
- `sling-bullet` 0→4 (4cp per 20-bundle×1, was the one zero-value straggler in this
  batch — flagged in the report as part of the original "37" but is Ammunition type,
  deferred to this batch per the progress doc's earlier note)
- `playing-card-set` 500→50 (5sp×100, was stale ×10 too high)
- All 7 Packs recomputed as sum-of-contents (see Pack section above for each
  breakdown) — `priests-pack` (2830→580), `entertainers-pack` (3300→2500),
  `diplomats-pack` (3402→2300) dropped significantly (their stored values were stale
  guesses, not sums); `scholars-pack` (3902→4202), `explorers-pack` (850→1000),
  `dungeoneers-pack` (1100→1200), `burglars-pack` (1626→1725) rose slightly to match
  the real sum now that every content item is correctly priced from prior batches.
  No discount applied — task guidance said keep it simple (sum of contents) absent a
  spotted discount, and none of the 7 packs' stored values suggested an intentional
  discount pattern (they were simply wrong in both directions).
- Everything else in this batch (`thieves-kit`, `navigator-kit`, `herbalism-kit`,
  `hammer`, `crafting-kit`, `pick-miners`, `shovel`, `dice-set`, `dragonchess-set`,
  `three-dragon-ante-set`, all 10 Musical Instruments) was already correctly priced —
  verified per-item against its real PHB gp/sp/cp denomination, nothing left stale.

**Homebrew prices invented/confirmed by analogy:** `hammer` (100, analogous to
`piton`/`crowbar` tier — no standalone PHB "hammer" price, only bundled artisan's-tools
sets); `crafting-kit` (2000, upper end of PHB artisan's-tools 5–30gp range since it
consolidates ~9 separate tool types into one homebrew kit); `pick-miners` (200, by
analogy to `shovel`'s 2gp — both excavation tools, no direct PHB pick price);
`three-dragon-ante-set` (100, by analogy to `dragonchess-set`'s 1gp tier — both
strategy/card games of similar complexity).

**Descriptions written (5):** `flute`, `drum`, `dice-set`, `playing-card-set`, `arrows`,
`crossbow-bolts`, `blowgun-needle` (7 total — all remaining "NEEDS DESCRIPTION" entries
in this batch's scope).

**Tags backfilled:** `instrument` added to all 10 Musical Instruments (all had empty
tags); `gaming-set` added to all 4 Gaming Sets (all had empty tags); `tool` added to 6
Tools that had empty tags (`thieves-kit`, `navigator-kit`, `herbalism-kit`, `hammer`,
`crafting-kit`) + backfilled onto `pick-miners` (kept existing `improvised-weapon`);
`ammunition` added to all 4 Ammunition items (kept existing `equipment`, which is
correct since all 4 carry `gear_slot: "ammo"`).

**Musical Instrument gap closed:** `bagpipe` was the one instrument missing
`performance` + carrying empty tags (per report §3.6) — added `performance`
(base_success 55, charisma_modifier 6, difficulty "hard", modeled between `viol`'s
"hard" tier and `pan-flute`'s mid-range success rate) and `instrument` tag; also fixed
rarity uncommon→common (same mundane-instrument-is-common ruling as armor/weapons).

**No `[~]` proposals this batch** — every item in Pack/Tools/Musical
Instrument/Gaming Set/Ammunition is fully expressible in the current schema; no
engine-mechanic gaps encountered.

`--validate`: **0 errors**, 37 warnings — identical set to Batch 3 (28 pre-existing
sprite-not-found including all 7 packs + `pick-miners` + `playing-card-set`, 5
informational consumable-effects warnings, 3 monster warnings, 1 spell warning). No
new warnings introduced by this batch's edits.

**Hard Goods: 32/32 done (all `[x]`, no `[~]`).**

**Overall catalog: 166/209 items refined so far (37 weapons + 33 armor + 64 gear + 32
hard goods); 43 remaining** (Arcane Focus 5, Druidic Focus 4, Holy Symbol 3, Potion 4,
Food 2, Spell Component 24, currency 1).
