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
