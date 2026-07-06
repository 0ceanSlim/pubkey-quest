# Item Conventions

Reusable rulings from the item-refiner passes. Read this before every batch; append
whenever a new reusable decision is made. Source audit: `docs/draft/item-report.md`.

---

## Pricing

**Rule:** this world has no copper/silver — **1 game gold = 1 D&D copper piece**. So an
item that exists in D&D 5e is priced at its **full PHB cost converted to copper**:
**gp × 100, sp × 10, cp × 1** → written as an integer to `value`. **Watch the PHB unit** —
many simple weapons and cheap gear are priced in sp/cp, not gp (club 1sp→10,
quarterstaff 2sp→20, javelin 5sp→50, sling 1sp→10, dart 5cp→5, torch 1cp→1), while a
1gp item → 100. Do NOT round everything to gp. Verified anchors already in the catalog:
`breastplate` 400gp→40000, `spyglass` 1000gp→100000, `healing` potion 50gp→5000,
`bedroll`/`rope`/`hammer` 1gp→100.

**For non-D&D (homebrew) items:** price by analogy to the nearest D&D item/tier; keep
`value` an integer. Set-bundle items ≈ sum of pieces (or slight discount).

### Weapon anchors used (PHB cost → copper: gp×100, sp×10, cp×1)

**Simple Melee Weapons**
| id | PHB gp | value |
|---|---|---|
| club | 1 sp | 10 |
| dagger | 2 gp | 200 |
| greatclub | 2 sp | 20 |
| handaxe | 5 gp | 500 |
| javelin | 5 sp | 50 |
| light-hammer | 2 gp | 200 |
| mace | 5 gp | 500 |
| quarterstaff | 2 sp | 20 |
| sickle | 1 gp | 100 |
| spear | 1 gp | 100 |

**Martial Melee Weapons**
| id | PHB gp | value |
|---|---|---|
| battleaxe | 10 | 1000 |
| flail | 10 | 1000 |
| glaive | 20 | 2000 |
| greataxe | 30 | 3000 |
| greatsword | 50 | 5000 |
| halberd | 20 | 2000 |
| lance | 10 | 1000 |
| longsword | 15 | 1500 |
| maul | 10 | 1000 |
| morningstar | 15 | 1500 |
| pike | 5 | 500 |
| rapier | 25 | 2500 |
| scimitar | 25 | 2500 |
| shortsword | 10 | 1000 |
| trident | 5 | 500 |
| war-pick | 5 | 500 |
| warhammer | 15 | 1500 |
| whip | 2 | 200 |

**Simple Ranged Weapons**
| id | PHB gp | value |
|---|---|---|
| crossbow-light | 25 gp | 2500 |
| dart | 5 cp | 5 |
| shortbow | 25 gp | 2500 |
| sling | 1 sp | 10 |

**Martial Ranged Weapons**
| id | PHB gp | value |
|---|---|---|
| blowgun | 10 | 1000 |
| crossbow-hand | 75 | 7500 |
| crossbow-heavy | 50 | 5000 |
| longbow | 50 | 5000 |
| net | 1 | 100 |

**Note on stale non-zero prices found mid-pass:** `mace` (5), `morningstar` (15),
`war-pick` (5), `blowgun` (1), `crossbow-hand` (150) were NOT `value: 0` (so missed by
the report's zero-value filter) but were still wrong — they held the **raw PHB gp
number** (or a partial ×10) instead of ×100. Fixed alongside the zero-value weapons in
the same batch since they're in the same type groups and the bug is identical in kind.
**Takeaway: when pricing a type group, verify every item's value against the ×100 rule,
not just the ones already flagged as zero** — stale non-multiplied values can hide in
the "non-zero" set.

---

## Rarity

Vocab: `common / uncommon / rare / legendary`. Keep monotonic with value within a
category. Basic PHB weapons (all Simple/Martial Melee/Ranged priced this batch) are
`common` — D&D doesn't assign special rarity to mundane weapons, and their value tier
(100–7500) sits at the bottom of the catalog's range alongside other common gear, so
`common` is correct as-is. Only bump rarity for a weapon if it's later reskinned as a
magic/masterwork item.

---

## Tags

Tag vocabulary observed/used (weapon-relevant): `equipment`, `weapon`, `simple-melee`,
`martial-melee`, `simple-ranged`, `martial-ranged`, `finesse`, `light`, `heavy`,
`two-handed`, `versatile`, `thrown`, `reach`, `loading`, `ammunition`, `topple`,
`improvised-weapon`, `restraint`.

**Decision:** every weapon gets `equipment` + `weapon` + a category tag
(`simple-melee`/`martial-melee`/`simple-ranged`/`martial-ranged`) + its D&D weapon
properties as tags (finesse/light/heavy/two-handed/versatile/thrown/reach/loading/
ammunition). This was inferred from existing partial tagging (e.g. `crossbow-heavy` had
`ammunition, loading, two-handed, heavy, equipment` but no `weapon`/category tag) — the
category + `weapon` tags were the gap, so this batch backfills them onto every weapon
touched (including previously-priced ones like `mace`, `morningstar`, `crossbow-hand`,
`blowgun`, `crossbow-light`, `crossbow-heavy`, `shortbow`, `longsword` for consistency
within the type group).

Existing hyphen-vs-underscore drift noted in report (`spell_component` vs `armor-set`)
does not affect weapons — no weapon tag has an underscore variant, hyphenated multi-word
tags (`simple-melee`, `two-handed`) are the convention for this concept.

---

## Armor pricing (Batch 2)

**PHB gp anchors → ×100 copper**, applied across Light/Medium/Heavy Armor + Armor Set:

| Base armor | PHB gp | value |
|---|---|---|
| Padded | 5 | 500 |
| Leather | 10 | 1000 |
| Studded Leather | 45 | 4500 |
| Hide | 10 | 1000 |
| Chain Shirt | 50 | 5000 |
| Scale Mail | 50 | 5000 |
| Breastplate | 400 | 40000 |
| Half Plate | 750 | 75000 |
| Ring Mail | 30 | 3000 |
| Chain Mail | 75 | 7500 |
| Splint | 200 | 20000 |
| Plate | 1500 | 150000 |
| Shield | 10 | 1000 |

**Piece-split rule:** this catalog splits full armors into a chest piece (`-cuirass`/
`-vest`/`-gambeson`/`-hauberk`) + leg piece (`-greaves`/`-leggings`/`-chausses`/
`-chaps`) plus a virtual `-set` bundle (weight 0, `contents: [[piece,1],[piece,1]]`).
**Set value = sum of its pieces.** The chest/leg split ratio already established by
`plate-set` (cuirass 90000 / greaves 60000 = 60/40 of 150000) turned out to hold
**exactly** for every multi-piece armor already in the catalog once each was checked
against the ×100 total — so no re-splitting was needed except chain mail (see below).
Confirmed 60/40 chest/leg splits: `padded-set` (300/200 of 500), `leather-set`
(600/400 of 1000), `studded-leather-set` (2700/1800 of 4500), `hide-set` (600/400 of
1000), `scalemail-set` (3000/2000 of 5000), `ringmail-set` (1800/1200 of 3000),
`splint-set` (12000/8000 of 20000), `halfplate-set` (45000/30000 of 75000),
`plate-set` (90000/60000 of 150000).

**Stale value fixed:** `chainmail-hauberk`/`chainmail-chausses`/`chainmail-set` held a
raw ×1.6-inflated number (12000 for the set instead of 7500 = 75gp×100) — same
"stale non-×100 value hiding in the non-zero set" pattern as Batch 1's weapons.
Corrected to the 60/40 split of 7500: hauberk 4500, chausses 3000.

**Standalone mundane armors** (no `-set` bundle, sold individually): `breastplate`
40000, `chain-shirt` 5000, `shield` 1000 — all already correct at ×100, no set to
sum against.

## Armor rarity ↔ value (Batch 2 — resolves report §6 contradiction)

**Ruling:** rarity in this catalog tracks *magical/exotic status*, not price tier —
confirmed by existing precedent (`spyglass` 100000 common, `supreme-healing` 50000
common). All PHB armor is **mundane** (D&D 5e assigns armor no rarity at all; rarity
only applies to *magic* armor). So **every mundane armor piece/set is `common`**,
regardless of value. This directly resolves the flagged `breastplate` common@40000 vs
`halfplate-cuirass` rare@45000 contradiction — both are common now, and it also
fixes the same value-tier-as-rarity mistake that had been applied to
`chainmail-*`/`scalemail-*`/`splint-*` (all `uncommon`) and `halfplate-*`/`plate-*`
(all `rare`). **Downgraded to `common` this batch:** `chainmail-hauberk`,
`chainmail-chausses`, `chainmail-set`, `scalemail-cuirass`, `scalemail-greaves`,
`scalemail-set`, `splint-cuirass`, `splint-greaves`, `splint-set`,
`halfplate-cuirass`, `halfplate-greaves`, `halfplate-set`, `plate-cuirass`,
`plate-greaves`, `plate-set`. Rarity should only bump above `common` for armor that's
explicitly a *magic item* variant (none currently exist in the catalog) — future
magic-armor content should follow D&D's real rarity assignment when added, not proxy
off price.

## Armor tags

Every armor piece gets `equipment` + a weight-class tag: `light` / `medium` / `heavy`.
`shield` (type `Heavy Armor` but not body armor) gets `equipment` + `shield` instead
of `heavy`, since it's a distinct piece of gear, not a torso/leg armor material.
Armor Set bundles keep their existing `pack, armor-set` tags (unchanged, already
consistent). Fixed 3 armor pieces that only had a bare `equipment` tag with no
weight-class tag: `breastplate`, `chain-shirt` → `+medium`; `shield` → `+shield`.

## gear_slot

Confirmed by reading `cmd/server/game/inventory/equipment.go:99-148`: `gear_slot:
"hands"` is **not dead / not a bug** — it's live logic that dynamically resolves to
`mainhand` if free, else `offhand` (shields always resolve to `offhand`). This is the
correct mechanism for one-handed/light weapons that can go in either hand. **No fix
needed for the 33 `gear_slot: "hands"` items** — leave as-is.

Valid resolved equip slots (`types.EquipmentSlots`): `neck, head, ammo, mainhand,
chest, offhand, ring1, legs, ring2, gloves, boots, bag`. `gear_slot` raw values seen on
weapons: `hands` (resolves dynamically), `mainhand` (two-handed weapons — pins to
mainhand explicitly), `ammo` (wrong on held weapons — see dart/javelin below).

**`dart`**: was `gear_slot: "ammo"` — wrong, it's a held thrown weapon (same shape as
`handaxe`/`light-hammer`, both `gear_slot: "hands"`). Fixed to `"hands"`.

**`javelin`**: same bug, not explicitly named in the task but identical pattern in the
same batch (Simple Melee Weapons) — `gear_slot: "ammo"` fixed to `"hands"` too, for
consistency (a thrown javelin is held until thrown, not ammunition loaded into a
launcher).

---

## dart / net / blowgun dispositions

- **`dart`**: fixed `gear_slot` "ammo"→"hands"; dropped bogus `ammunition: "arrows"`
  key (a dart isn't loaded with arrows — it IS the thrown munition); priced 5cp×100=5;
  tags rebuilt to `weapon, equipment, simple-ranged, finesse, thrown`. No engine
  proposal needed — fully expressible in current schema.
- **`blowgun`**: `stack: 25` was wrong for a held weapon (copied from an ammo
  pattern) — fixed to `stack: 1`. Kept `ammunition: "blowgun-needle"` (valid ref, a
  held weapon legitimately declares what ammo it fires — same pattern as
  `longbow`/`shortbow`/`crossbow-*`). Kept the `ammunition`/`loading` tags (these
  describe the weapon's own properties, distinct from the stray `stack`). No engine
  proposal needed.
- **`net`**: kept `type: "Martial Ranged Weapons"` (not asked to change type/schema
  shape). Fixed `damage: "0"` → dropped (a net deals no damage in 5e; rather than a
  fake "0" numeric, left `damage` absent is not allowed by UNIV schema for this type —
  see note below) — **see proposal below, flagged `[~]`**. Fixed `range_long: "320"`
  (units bug) → `"3"` (matches other short-range thrown weapons' long-range scale).
  Dropped bogus `ammunition: "arrows"` (a net needs no ammo). Priced 1gp×100=100.
  Restraint/entangle mechanic proposed in `item-mechanics-proposals.md` — the net's
  core function (restrain on hit, escape DC, Str check to burst free) has no engine
  hook yet, same gap as `caltrops.restrain`.

---

## Weapon type group tags (backfilled this batch)

Applied uniformly to every weapon touched in the zero-value pricing pass:
- **Simple Melee Weapons** → `weapon, equipment, simple-melee` + properties
- **Martial Melee Weapons** → `weapon, equipment, martial-melee` + properties
- **Simple Ranged Weapons** → `weapon, equipment, simple-ranged` + properties
- **Martial Ranged Weapons** → `weapon, equipment, martial-ranged` + properties

Properties tag vocabulary drawn straight from PHB weapon property text (finesse,
light, heavy, two-handed, versatile, thrown, reach, loading, ammunition, topple).

---

## Adventuring Gear pricing (Batch 3)

**Rule reminder — derive per-item, don't just multiply the stored value.** Adventuring
Gear is a mix of already-correct (×100/×10/×1 already applied) and stale (raw PHB
number, or a wrong scale) values. Every item's `value` was checked against its **real
PHB denomination** (gp/sp/cp) independently — not assumed from what was already
stored. Most of the 64 were already correct; the actual corrections found:

| id | PHB price | correct value | was |
|---|---|---|---|
| `backpack` | 2 gp | 200 | 1000 (stale ×5) |
| `sack` | 1 cp | 1 | 50 (stale ×50) |
| `chest` | 5 gp | 500 | 5 (raw gp, un-multiplied) |
| `component-pouch` | 25 gp | 2500 | 25 (raw gp, un-multiplied) |
| `chain` (10 ft) | 5 gp | 500 | 5 (raw gp, un-multiplied) |
| `case-map-and-scroll` | 1 gp | 100 | 1 (raw gp, un-multiplied) |
| `signal-whistle` | 5 sp | 50 | 5 (raw sp, un-multiplied) |
| `pole` | 5 cp | 5 | 0 |
| `pouch` | 5 sp | 50 | 0 |
| `quiver` | 1 gp | 100 | 0 |
| `caltrops` | 1 gp (bag of 20) | 100 | 0 |
| `ball-bearings` | 1 gp (bag of 1000) | 100 | 1 (raw gp, un-multiplied) |
| `shovel` (Tools type, not Gear, but same bug) | 2 gp | 200 | 2 (raw gp) |

**Takeaway confirmed again:** stale un-multiplied or wrong-scale values hide among
non-zero entries just as often in Adventuring Gear as they did in weapons/armor —
`chest`, `component-pouch`, `chain`, `case-map-and-scroll`, `signal-whistle`, `ball-
bearings` were all non-zero but wrong. Always check the real PHB denomination per
item, never trust "non-zero = already priced."

**Homebrew (no PHB entry) pricing by analogy:** `nail` — no PHB "nail" line item exists;
priced at 1 (1cp-equivalent), analogous to other small bulk fasteners/consumables
(`piton` 5cp/unit, `chalk` 1cp/piece) already in the catalog. Kept as-is (was already 1).

**Everything else already correct at ×100/×10/×1** (confirmed per item, not
listed exhaustively here — see the per-item checklist in
`item-refinement-progress.md` for each one's specific D&D anchor): `acid` 2500 (25gp),
`alchemists-fire` 5000 (50gp), `barrel` 200 (2gp), `basket` 40 (4sp), `bedroll` 100
(1gp), `bell` 100 (1gp), `blanket` 50 (5sp), `block-and-tackle` 100 (1gp), `book` 2500
(25gp), `bottle-glass` 200 (2gp), `bucket` 5 (5cp), `climbers-kit` 2500 (25gp),
`crowbar` 200 (2gp), `flask` 2 (2cp), `grappling-hook` 200 (2gp), `hammer-sledge` 200
(2gp), `healers-kit` 500 (5gp), `hourglass` 2500 (25gp), `hunting-trap` 500 (5gp),
`ink` 1000 (10gp), `ink-pen` 2 (2cp), `jug-or-pitcher` 2 (2cp), `ladder` 10 (1sp),
`lamp` 50 (5sp), `lantern-bullseye` 1000 (10gp), `lantern-hooded` 500 (5gp), `lock`
1000 (10gp), `magnifying-glass` 10000 (100gp), `mess-kit` 20 (2sp), `mirror` 500 (5gp),
`oil` 10 (1sp), `paper` 20 (2sp), `parchment` 10 (1sp), `perfume-vial` 500 (5gp),
`piton` 5 (5cp), `poison-basic` 10000 (100gp), `pot-iron` 200 (2gp), `ram-portable` 400
(4gp), `rope` 100 (1gp), `sealing-wax` 50 (5sp), `signet-ring` 500 (5gp), `soap` 2
(2cp), `spellbook` 5000 (50gp), `spyglass` 100000 (1000gp), `tinderbox` 50 (5sp),
`torch` 1 (1cp), `vial` 100 (1gp), `waterskin` 20 (2sp), `whetstone` 1 (1cp), `candle`
1 (1cp), `chalk` 1 (1cp).

## Rarity — mundane gear (Batch 3)

Same ruling as Batch 2 armor: rarity tracks magical/exotic status, not price tier.
Fixed 2 mundane-gear rarity errors this batch: `case-map-and-scroll` rare→common
(report-flagged, value/rarity contradiction), `backpack` uncommon→common (no reason
to sit above common — mundane gear). Also found and fixed mid-batch (not in the
report's original list): `bell` uncommon→common.

## Tags — the `equipment` tag is reserved for actually-equippable items (important, validator-enforced)

**Discovered this batch via the validator, not the report:** `cmd/codex/validation/
validation.go` enforces a hard rule — *"Items with 'equipment' tag must have
'gear_slot' property"* (and the inverse warns if an item **has** `gear_slot` but lacks
the `equipment` tag). Valid `gear_slot` values: `hands, mainhand, offhand, chest, head,
legs, gloves, boots, neck, ring, ammo, bag`.

**Ruling:** do **not** use `equipment` as a generic "this is gear you own" tag. Reserve
it strictly for items that carry a real `gear_slot` (i.e. can actually be worn/wielded/
socketed) — `backpack`, `pouch`, `quiver`, `spellbook`, `pole` (all pre-existing or
fixed in earlier batches), plus all weapons/armor/foci which already have `gear_slot`.
For everything else (tools, vessels, writing supplies, camping gear, restraint
items, consumables with no equip slot), give a **real classification tag instead**,
never `equipment`. New tag vocabulary introduced this batch for the previously-
untagged bulk of Adventuring Gear:
- `tool` — hand tools / utility devices with no wearable slot (`crowbar`, `lock`,
  `whetstone`, `spyglass`, `mirror`, `hourglass`, `bell`, `signal-whistle`, `ladder`,
  `piton`, `rope`, `ram-portable`, `hammer-sledge`, `grappling-hook`,
  `block-and-tackle`, `magnifying-glass`, `nail`, `tinderbox`).
- `vessel` — glass/ceramic/metal liquid-holding containers that are **not** schema
  containers (no `container_slots`/`allowed_types`): `bottle-glass`, `bucket`,
  `flask`, `jug-or-pitcher`, `vial`, `perfume-vial`. Kept distinct from the `container`
  tag, which is reserved for items with real `container_slots` (`backpack`, `sack`,
  `chest`, `barrel`, `basket`, `case-map-and-scroll`, `component-pouch`, `pouch`,
  `quiver`, `spellbook`).
- `writing` — office/scribing supplies: `book`, `ink`, `ink-pen`, `paper`, `parchment`,
  `sealing-wax`, `chalk`.
- `camping` — bedroll/cookware/travel-comfort gear: `bedroll`, `blanket`, `waterskin`,
  `mess-kit`, `pot-iron`.
- `jewelry` — `signet-ring` (not equippable via a modeled ring slot yet, so no
  `gear_slot`; flavor-only for now).
- `fuel` — `oil` (burns in lamps/lanterns).
- Existing tags reused as-is: `consumable` (added to `acid`, `alchemists-fire`,
  `healers-kit`, `poison-basic`, `soap` — all single-use), `restraint` (`ball-bearings`,
  `chain`, already-tagged `caltrops`), `trap` (`hunting-trap`, new), `container`
  (all schema containers, confirmed/backfilled), `light-source`/`oil-burning`/
  `directional` (unchanged, already complete per the report), `thrown`/`poison`/
  `healing`/`pack`/`two-handed`/`improvised-weapon` (all pre-existing, unchanged).

**Container items keep `container` only, not `equipment`** — none of the 10 schema
containers in this catalog (`backpack`/`pouch`/`quiver` excepted, which are wearable)
have a `gear_slot`, so adding `equipment` to `sack`/`chest`/`barrel`/`basket`/
`case-map-and-scroll`/`component-pouch` would fail validation. Only wearable
containers (`gear_slot: "bag"` or `"ammo"`) get both `container` + `equipment`.

## Adventuring Gear content fixes (Batch 3)

- **`ram-portable`** was missing `damage_type` despite having `damage: "1d4"` (report
  flagged this pattern generally for `pick-miners`/`ram-portable` in §3.4). Added
  `"bludgeoning"` — a real, already-used key on this schema (not invented), matching
  how every other improvised/weapon-shaped item in the catalog pairs `damage` +
  `damage_type`.
- **`parchment`** weighed `0` (report-flagged zero-weight gap, §1). Fixed to `0.01`
  (a token non-zero weight consistent with `paper`'s 0.01).
- **`backpack.effects_when_worn: ["backpack-capacity"]`** — confirmed genuinely wired,
  not a dead key: `game-data/effects/backpack-capacity.json` exists and grants
  `weight_capacity +25` on equip via the equip-effects pipeline
  (`cmd/server/game/inventory/equipment.go`). No proposal needed. (Minor aside, not
  fixed: the item's own `notes` claim "+35 lbs" but the effect grants +25 — a
  pre-existing inconsistency in flavor text vs. the real effect value, out of scope
  for a pricing/tagging pass; flagged here for whoever next touches this item.)

---

## Pack pricing (Batch 4) — value = sum of contents

**Rule:** a Pack's `value` = Σ(content item `value` × qty) across every `[item-id, qty]`
entry in its `contents` list. No discount applied vs. buying pieces separately — task
guidance was to keep it simple unless a pack's stored value clearly signaled an
intentional discount, and none did (every pack's original value was simply wrong, in
both directions, relative to the real sum). **Recompute this whenever a content item's
own price changes** — pack values are derivative, not independently authored.

All 7 packs recomputed once every content item was correctly priced (post Batches 1–3):
`scholars-pack` 4202, `priests-pack` 580, `explorers-pack` 1000, `entertainers-pack`
2500, `dungeoneers-pack` 1200, `diplomats-pack` 2300, `burglars-pack` 1725. Three of
the seven (`priests-pack`, `entertainers-pack`, `diplomats-pack`) had stored values
far *above* the real sum (stale over-estimates); the other four were close but still
off by a small amount — none were correct as stored.

## Ammunition pricing (Batch 4) — value = PHB bundle price, independent of `stack`

**Rule:** Ammunition `value` is the PHB price for **one full bundle** (arrows/bolts/
sling-bullets sold 20 to a bundle at 1gp, blowgun needles 50 to a bundle at 1gp,
converted ×100/×10/×1 as usual). The item's own `stack` field (the inventory UI's max
stack size, e.g. `arrows.stack: 25`) is a **separate, unrelated inventory-mechanic
number** — do not try to make `value` scale with `stack`; they serve different
purposes and don't need to match the real-world bundle count exactly.

| id | PHB price (bundle) | value |
|---|---|---|
| arrows | 1 gp / 20 | 100 |
| crossbow-bolts | 1 gp / 20 | 100 |
| sling-bullet | 4 cp / 20 | 4 |
| blowgun-needle | 1 gp / 50 | 100 |

All 4 kept `gear_slot: "ammo"` (UNIV for the type) and got an `ammunition` tag added
alongside the pre-existing `equipment` tag (correct per the equipment/gear_slot
validator rule — ammo items do carry a real `gear_slot`).

## Musical Instrument / Gaming Set / Tools tags (Batch 4)

New tag vocabulary for these three previously-untagged types (all had empty `tags: []`
across the board except `pick-miners`/`shovel` which had `improvised-weapon`):
- **`instrument`** — all 10 Musical Instruments (`bagpipe`, `lute`, `lyre`, `flute`,
  `drum`, `horn`, `pan-flute`, `viol`, `shawm`, `dulcimer`).
- **`gaming-set`** — all 4 Gaming Sets (`dice-set`, `playing-card-set`,
  `dragonchess-set`, `three-dragon-ante-set`).
- **`tool`** — reused from the Batch 3 Adventuring Gear vocabulary, extended to cover
  the Tools type proper: `thieves-kit`, `navigator-kit`, `herbalism-kit`, `hammer`,
  `crafting-kit`, `pick-miners` (kept its pre-existing `improvised-weapon`), `shovel`
  (already tagged in Batch 3).

None of these three types carry a `gear_slot`, so none get `equipment` — consistent
with the Batch 3 ruling that `equipment` is reserved for items with a real gear_slot.

## Musical Instrument gap: `bagpipe.performance` (Batch 4, closes report §3.6/§6)

`bagpipe` was the one instrument missing the UNIV `performance` object. Modeled by
analogy to the two nearest "hard difficulty" instruments already in the catalog
(`viol`: base_success 40/charisma_modifier 7; `shawm`: base_success 50/
charisma_modifier 6) — bagpipes are a loud, technically demanding reed instrument, so
placed at base_success 55/charisma_modifier 6/difficulty "hard", between the two.
Also fixed rarity `uncommon`→`common` (mundane instrument, same ruling as armor/
weapons: rarity tracks magical status not price/rarity-flavor).

## Homebrew pricing by analogy (Batch 4)

No direct PHB line item exists for these; priced by analogy to the nearest comparable
item already in the catalog, at the same tier:
- **`hammer`** (Tools) — 100 (1gp-equivalent), analogous to `piton`/`crowbar`-tier
  simple hand tools; PHB only prices hammers bundled inside artisan's-tools sets, never
  standalone.
- **`crafting-kit`** (Tools) — 2000 (20gp-equivalent), placed at the upper end of the
  PHB artisan's-tools range (5–30gp) since this homebrew kit consolidates ~9 separate
  real-world tool types (smith's, carpenter's, mason's, leatherworker's, weaver's,
  woodcarver's, glassblower's, potter's, +misc) into one item.
- **`pick-miners`** (Tools) — 200 (2gp-equivalent), by analogy to `shovel`'s 2gp — both
  are excavation tools with improvised-weapon `damage`, no direct PHB "pick" price
  exists outside masonry/miner's-tools artisan bundles.
- **`three-dragon-ante-set`** (Gaming Set) — 100 (1gp-equivalent), by analogy to
  `dragonchess-set`'s 1gp — both are complex strategy/card games at the same PHB
  gaming-set tier (distinct from the cheaper `dice-set`/`playing-card-set` tier).
