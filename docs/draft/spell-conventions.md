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

## Prep time (per-spell, unique — `prep_time` in minutes)

Prep time is now a per-spell field the refiner tunes (the engine falls back to
`level × 60` only when unset — `spells.PrepMinutesForSpell`). Make each leveled spell's
value distinct.

- **Cantrips: omit `prep_time`** — always instant.
- **Leveled spells:** anchor on `level × 60` (L1 60, L2 120, L3 180) and spread by nature:
  - ~0.5× (L1 30–45): quick/simple/low-commitment — minor utility, fast detections/buffs
  - ~1× (L1 ~60): standard spells
  - ~1.5–2× (L1 75–120): complex/powerful/long-duration/ritual — summons, all-day buffs
    (mage-armor), find-familiar, hard control
- Round to 5/15-min steps.

**Three cost axes, balanced + focus-aware:** total cost = `mana_cost` + `prep_time` +
component. Don't triple-tax one spell. A real always-consumed rune already IS a cost →
keep mana/prep modest; a focus-provided or component-free spell carries its cost in
mana + prep. Weigh whether a component is really a cost for the intended caster given
the focus that class routinely holds.

**Batch 2 backfill complete.** All 12 already-refined L1 spells now have `prep_time`.

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

## Level 1 mana rulings (batch 2)

- **mana 3 (push up):** Burning Hands (AoE fire cone), Thunderwave (AoE + push), Inflict Wounds (3d10 biggest L1 damage), Armor of Agathys (1hr no-conc warlock self-buff), Bless (3-target conc), Bane (3-target conc debuff), Charm Person (hour-long hard control), Arms of Hadar (AoE necrotic)
- **mana 2 (base):** Magic Missile (reliable auto-hit, not exceptional), Witch Bolt (single-target conc sustained), Command (1-round only, expires immediately), False Life (minor utility buff)
- **Ritual/out-of-combat L1:** stay mana 2 or lower (not cast in combat normally)

## Level 1 component-cost decisions (batch 2)

**Received a component (selective):**
- `burning-hands`: ash×1 (15gp) — fire substance spell, cheapest fire rune, L1 tier. No focus provides ash — always a cost.
- `witch-bolt`: iron-filings×1 (20gp) — lightning theme; no focus provides iron-filings — always a cost.
- `inflict-wounds`: spirit-dust×1 (75gp) — necrotic/dark substance; no focus provides spirit-dust. Cost is mid-tier but the spell does 3d10 (highest L1 damage) so it's warranted.
- `arms-of-hadar`: spirit-dust×1 (75gp) — dark energy invocation; AoE necrotic warlock signature. No focus provides spirit-dust.
- `bane`: spirit-dust×1 (75gp) — curse/dark theme; 3-target conc hard debuff is signature. No focus provides spirit-dust.
- `bless`: blessed-incense×1 (100gp) — divine substance spell; signature cleric/paladin support. Amulet provides it free (clerics/paladins routinely carry amulet). Kept to one component only.

**Left free (most L1 spells):**
- `magic-missile`: generic reliable force — not substance-themed
- `thunderwave`: thunder/sonic — no matching rune in the rune list
- `false-life`: minor necromantic utility — not substance-requiring
- `armor-of-agathys`: cold/frost — no matching cold rune in the rune list
- `charm-person`: basic enchantment — not substance-themed
- `command`: single-word control — not substance-themed

**Key correction from pre-existing stubs:**
- `burning-hands` had sulfur×2 (500gp×2 = Fireball-tier) — removed completely wrong.
- `arms-of-hadar` had void-crystal×1 (8000gp) + demon-ichor×1 (600gp) — capstone/legendary components on a L1 AoE!
- `bane` had ether-essence+arcane-powder (arcane domain, not dark/curse)
- `inflict-wounds` had holy-water+blessed-incense (divine healing components on a NECROTIC spell!)
- `witch-bolt` had sulfur+arcane-powder (fire+arcane domain for a lightning spell)
- `bless` had blessed-incense×2+sacred-oil (doubled up, over-costed)
- `magic-missile` had arcane-powder×3 (too many per cast)
- `charm-person` had two mid-tier components (not warranted for basic enchantment)
- `armor-of-agathys` had mana-crystals×2+quartz-dust (200+50 each = over-priced)
- `thunderwave` had bone-dust+tree-sap (necrotic/nature components — totally wrong theme)

**Rule codified:** Pre-existing stubs had components on almost everything with mis-matched domains and wrong cost tiers. The correct approach: few components, right domain, L1 = cheap runes (ash 15, iron-filings 20, spider-silk/bark-shavings 25, pollen 30) or spirit-dust 75 for especially signature/powerful L1 dark spells. Mid-tier (100+) only if the spell is truly a signature and the class routinely uses the focus that provides it free.

## Level 1 component-cost decisions (batch 3)

**Received a component (selective):**
- `mage-armor`: mana-crystals×1 (200gp, staff=free) — core arcane protection, all-day buff is signature for wizards/sorcerers. Reduced from stub's ×3. With staff equipped, free per cast.
- `protection-from-evil`: salt×1 (10gp) — protection domain's cheapest rune; no focus provides salt so it's always a small cost. Fits the "salt circle" thematic perfectly.
- `shield`: mana-crystals×1 (200gp, staff=free) — kept from stub. Reaction arcane force — warranted for a signature defensive spell. With staff, free.

**Left free (batch 3 spells):**
- `cure-wounds`: basic healing touch — not substance-themed
- `detect-evil`: divine detection — not substance-themed
- `detect-magic`: divination utility — removed stub's quartz-dust×2 (not warranted)
- `detect-poison`: divination utility — not substance-themed
- `divine-favor`: prayer empowerment — removed stub's starlight-essence×1 (LEGENDARY 10000gp on L1 paladin buff — catastrophically wrong)
- `healing-word`: basic healing prayer — removed stub's sacred-oil+holy-water (150+125gp — over-costed on a minor bonus-action heal)
- `heroism`: courage prayer — not substance-themed
- `shield-of-faith`: faith prayer — not substance-themed; a small +2 AC buff doesn't warrant a rune

**Key correction from pre-existing stubs (batch 3):**
- `divine-favor` had starlight-essence×1 (10000gp LEGENDARY on L1 paladin buff!) — entirely wrong.
- `cure-wounds` had bark-shavings×2+pollen (two components, mismatched — both nature not healing themed)
- `detect-magic` had quartz-dust×2 (illusion/scrying domain, not appropriate for basic divination)
- `mage-armor` had mana-crystals×3 (triple-stack per cast even with focus; reduced to ×1)
- `healing-word` had sacred-oil×1+holy-water×1 (both components, 125+150=275gp on a bonus-action heal)

## Level 1 prep_time rulings (batch 2 backfill)

All distinct values; logic: quick utility/fast spells at low end, powerful/complex/long-duration at high end.

| Spell | prep_time | Rationale |
|-------|-----------|-----------|
| `command` | 30 | Single-word, 1-round only — simplest possible control |
| `magic-missile` | 35 | Classic fast evocation, no component, reliable but not exceptional |
| `false-life` | 40 | Minor necromantic utility, quick self-buff |
| `burning-hands` | 45 | Fast instinctive AoE; cheapest rune cost (ash 15gp) means no triple-tax |
| `witch-bolt` | 55 | Conc sustained lightning; iron-filings cost already a tax — keep prep modest |
| `inflict-wounds` | 60 | Standard melee attack; spirit-dust cost already a tax |
| `arms-of-hadar` | 75 | AoE invocation; spirit-dust cost means we trim prep |
| `charm-person` | 75 | Hard control, 1-hour; no component so cost lives in mana+prep |
| `thunderwave` | 75 | AoE push, powerful; no component, mana 3 carries the cost |
| `armor-of-agathys` | 90 | Strong 1-hr no-conc self-buff; warlock signature |
| `bless` | 90 | 3-target conc buff; incense=free for amulet so prep is the main cost |
| `bane` | 105 | 3-target conc hard debuff; complex multi-creature coordination |

## Level 1 prep_time rulings (batch 3)

| Spell | prep_time | Rationale |
|-------|-----------|-----------|
| `healing-word` | 30 | Bonus action, simplest heal, no component |
| `command` | 30 | (batch 2 — same tier) |
| `detect-magic` | 35 | Fast detection utility, also a ritual |
| `detect-poison` | 35 | Fast detection utility, also a ritual |
| `detect-evil` | 40 | Fast detection, slightly more complex aura |
| `divine-favor` | 45 | Bonus action combat buff; no component, modest mana |
| `shield` | 45 | Reaction prep; mana-crystals=free with staff |
| `cure-wounds` | 50 | Touch heal, simple but single target; no component |
| `shield-of-faith` | 50 | Bonus action, modest +2 AC; no component |
| `heroism` | 60 | Conc touch buff, standard; no component |
| `protection-from-evil` | 65 | Ward ritual feel; salt cost (always-consumed) means trim prep slightly |
| `mage-armor` | 120 | 8-hour all-day no-conc buff — highest L1 prep; mana-crystals=free with staff |

## Level 1 component-cost decisions (batch 4)

**Received a component (selective):**
- `animal-friendship`: pollen×1 (30gp) — charm/nature domain; sprig-of-mistletoe provides it free for druids/rangers. The 24-hour beast charm is signature enough for one cheap nature rune.
- `goodberry`: pollen×1 (30gp) — nature/transmutation; sprig-of-mistletoe provides it free. Reduced from stub's tree-sap+pollen (double-taxing). One nature rune for a healing-utility transmutation.
- `sleep` (Exhausting Hex): spirit-dust×1 (75gp) — hex/curse domain; no focus provides spirit-dust so it's always a cost. The 1-hour no-concentration exhaustion debuff is the most powerful L1 control spell — the rune cost is warranted alongside mana 3. Replaced stub's ether-essence×2+mana-crystals×1 (focus-provided orb/staff = not a real cost for wizards).

**Left free (batch 4 spells):**
- `identify` (Analyze Weakness): 1-minute cast utility divination — removed stub's quartz-dust×2+ether-essence×1 (two mid-tier components on a basic identification spell is wrong)
- `disguise-self`: self-illusion utility — not substance-themed
- `silent-image`: illusory image — not substance-themed
- `comprehend-languages`: language utility — not substance-themed
- `color-spray`: light flash AoE — not substance-themed (flash of light doesn't burn a physical rune)
- `speak-with-animals`: communication ritual — removed stub's pollen×1 (stub had wrong focus note claiming no focus provides it; sprig-of-mistletoe does, but the spell is a utility ritual not substance-themed enough to warrant it)
- `purify-food`: cleansing ritual — not substance-themed

**Key corrections from pre-existing stubs (batch 4):**
- `identify` had quartz-dust×2+ether-essence×1 (250gp per cast on a 1-min utility) — removed entirely
- `sleep` (Exhausting Hex) had ether-essence×2+mana-crystals×1 — both focus-provided (orb/staff), meaning they cost nothing for a wizard/sorcerer anyway — replaced with spirit-dust×1 which IS always consumed
- `goodberry` had tree-sap+pollen (double nature cost removed tree-sap; kept pollen only)
- `speak-with-animals` had pollen with incorrect "No focus provides this component" note — sprig-of-mistletoe DOES provide pollen; removed entirely since not substance-themed enough

## Level 1 prep_time rulings (batch 4)

| Spell | prep_time | Rationale |
|-------|-----------|-----------|
| `purify-food` | 25 | Simplest ritual utility — pure cleansing, no combat use |
| `comprehend-languages` | 30 | Pure language utility; ritual-eligible; tie with healing-word/command acceptable at this tier |
| `identify` | 40 | Quick detection buff; 1-min cast is out-of-combat only |
| `disguise-self` | 45 | Simple 1-hr self-illusion; tie with shield/divine-favor acceptable |
| `animal-friendship` | 50 | 24-hr beast charm; pollen=free for druids; tie with shield-of-faith acceptable |
| `silent-image` | 55 | Conc illusion utility; 10 min duration; tie with speak-with-animals acceptable |
| `speak-with-animals` | 55 | Ritual utility; homebrew tactical element; tie with silent-image acceptable |
| `color-spray` | 60 | AoE control (hp-threshold); mana 3 carries main cost; tie with inflict-wounds acceptable |
| `goodberry` | 65 | Nature transmutation, creates lasting items; pollen=free for druids |
| `sleep` | 80 | Hardest control in L1 (1hr no-conc exhaustion); spirit-dust cost + mana 3; unique between 75 and 90 |

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
