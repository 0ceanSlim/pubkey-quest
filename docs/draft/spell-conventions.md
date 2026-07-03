# Spell Refinement Conventions

Accumulated rulings made during the refinement process. Read this at the start of every
run to keep decisions consistent across batches.

---

## Mana cost bands

| Level  | Base mana | Push up for…                          | Push down for…       |
|--------|-----------|---------------------------------------|----------------------|
| 0 (cantrip) | 1    | AoE, 1d10 damage, double-effect (damage + debuff), sustained buff (conc) | Minor utility/flavor |
| 1      | 2–3       | Hard control, AoE, healing, big single-target damage | Ritual, minor buff |
| 2      | 3–4       | (TBD)                                 |                      |
| 3      | 4–5       | (TBD)                                 |                      |

**Cantrip rulings settled (batch 1):**
- mana 1: most single-target damage cantrips, all pure utility/flavor, stabilization
- mana 2: Eldritch Blast (1d10 force, warlock signature), Fire Bolt (1d10 fire), Arcane
  Weapon (1 min conc damage buff), Vicious Mockery (damage + debuff double-effect),
  Blinding Flash (AoE con save), Word of Radiance (AoE con save)

---

## Component cost (homebrew rune-like — NOT D&D materials)

Components are a **per-cast resource cost** on *certain* spells, like RuneScape runes —
NOT D&D's material list. Most spells and most cantrips are free (`required: []`). Give a
cost only to signature / impactful / substance-themed spells, and scale the component's
`value` (cost tier) to the spell's power. Valid ids = the game's `spell_component` items
only — enumerate live: `game-data/items/*.json` with tag `spell_component` (24 today,
list grows) — never invent one. Scarcity is the point; don't decorate every spell.

**Domain map** (theme → components, cheap → pricey by value):
- arcane/force → arcane-powder(75), ether-essence(100), mana-crystals(200)
- fire → ash(15), elemental-sparks(150), sulfur(500), dragon-scale(500)
- lightning → iron-filings(20)
- nature/heal → bark-shavings(25), pollen(30), tree-sap(50)
- poison/decay → mushroom-spores(45), tree-sap(50)
- holy/divine → blessed-incense(100), sacred-oil(125), holy-water(150), starlight-essence(10000)
- necrotic/dark → spirit-dust(75), bone-dust(200), demon-ichor(600), void-crystal(8000)
- illusion/light → quartz-dust(50)
- protection → salt(10)
- binding → spider-silk(25)
- rebirth → phoenix-feather(5000)

**Focus = unlimited runes** (elemental-staff mechanic): the 12 focus-provided ids
(wand→arcane-powder, staff→mana-crystals, wooden-staff→bark-shavings, amulet→blessed-incense,
emblem→sacred-oil, orb→ether-essence, rod→sulfur, yew-wand→tree-sap, crystal→quartz-dust,
reliquary→holy-water, sprig-of-mistletoe→pollen, totem→bone-dust) are free with the matching
focus. The other 12 (ash, demon-ichor, dragon-scale, elemental-sparks, iron-filings,
mushroom-spores, phoenix-feather, salt, spider-silk, spirit-dust, starlight-essence,
void-crystal) always cost. The three single-stack legendaries (phoenix-feather 5000,
void-crystal 8000, starlight-essence 10000) are for capstone spells ONLY.

**Batch 1 (cantrips):** mostly free — correct under this framing. Shillelagh keeps
`pollen` (focus: sprig-of-mistletoe). Re-audit any cantrip cost against the cost-tier rule
on a later pass; cantrips should almost never carry a rune cost.

---

## Schema conventions

- **Always include explicit `null` for `heal` and `effect`** even if not in original
  stub — keeps shape consistent.
- **material_component:** always include block even if empty: `{ "required": [], "focus_provided": "" }`.
- **Never set both `spell_attack` and `save_type`** — pick one primary shape. If a spell
  needs both (e.g. attack then save on hit), set `spell_attack` as primary and document
  the save in `notes`.
- **`action_cost`:** must be a valid enum string. Confirmed values seen: `"action"`,
  `"bonus_action"`, `"reaction"`. Avoid `"bonus action"` with a space — use
  `"bonus_action"` (underscore). **NOTE:** check whether the validator enforces this
  (shillelagh previously had `"bonus action"` space form — now fixed to `"bonus_action"`).
- **`range` field:** is a numeric string representing grid cells (5 ft per cell in D&D).
  60 ft → range "4", 30 ft → range "2", 10 ft → range "1", touch/self → range "0".
  `range_long` should equal range when there's no extended range; for thrown weapons
  (produce-flame) range is self but range_long is the throw distance.

---

## Homebrew content in the library

Several spell FILES contain homebrew cantrips (non-D&D-5e spells). Treat them as
first-class game content — do not revert to D&D originals. Key homebrew cantrips:

| File | D&D Name | Game Name | Notes |
|------|----------|-----------|-------|
| `dancing-lights.json` | Dancing Lights | Blinding Flash | AoE blind, con save |
| `druidcraft.json` | Druidcraft | Druidcraft | Kept as utility (removed invalid combat save) |
| `light.json` | Light | Revealing Light | Detection utility, 1 hour |
| `mage-hand.json` | Mage Hand | Spectral Strike | Ranged attack + push |
| `mending.json` | Mending | Repair Armor | Out-of-combat repair, 1 min cast |
| `minor-illusion.json` | Minor Illusion | Combat Illusion | Wis save, debuff |
| `prestidigitation.json` | Prestidigitation | Arcane Weapon | Conc buff cantrip |
| `identify.json` | Identify | Analyze Weakness | (level 1, check next batch) |
| `sleep.json` | Sleep | Exhausting Hex | (level 1, check next batch) |

---

## Combat vs. out-of-combat cast time

- Spells with `casting_time: "1 minute"` or longer are out-of-combat only.
- Keep `action_cost: "action"` for these (cost if somehow triggered in combat context),
  but note the out-of-combat nature in `notes`.
- Ritual spells (level 1+): will carry `casting_time: "10 minutes"` and a "ritual"
  tag when applicable.

---

## Conditions needing engine support

These secondary effects cannot be fully modeled in the current schema. They are noted
in `notes[]` and tracked in `docs/draft/spell-mechanics-proposals.md`:

- `blinded` — AoE/save, disadvantage both ways
- `no_healing` — on-hit, prevents hp recovery
- `no_reactions` — on-hit, suppresses reactions
- `speed_reduction` — on-hit, reduces movement
- `advantage_next_attack_vs_target` — personal, self-buff
- `debuff_next_attack` — on-save-fail, target has disadvantage
- `push_N_ft` / `pull_N_ft` — forced movement, on-hit
- `spellcasting_attack_override` — use spell stat for weapon attacks

---

## Class list conventions

Using lowercase class names. Known valid values (matches engine):
`wizard`, `sorcerer`, `warlock`, `cleric`, `druid`, `bard`, `ranger`, `paladin`,
`artificer`, `fighter`

When D&D 5e lists a class the game doesn't have yet (e.g. `artificer`), include it
anyway — the engine ignores unknown classes gracefully and having it prepares for future
class additions.

---
