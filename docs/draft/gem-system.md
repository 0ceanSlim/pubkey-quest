# Gems, jewelry & enchanting — brainstorm

Status: **BRAINSTORM, for the maintainer to pick from** (2026-10-08). Source data:
`game-data/systems/gems.json` (not read by any code yet). Only one gem item exists:
`rough-gem` (placeholder, 500 copper), the Ember Vault rite's price.

Scope note: the roadmap keeps **general item crafting** and **magic items** out of alpha.
Gems as loot plus a sell value are alpha-sized; cutting, jewelry and enchanting are beta.
That ordering is how the slices at the bottom are cut.

---

## 1. What a gem is: four axes

| Axis | Values | What it drives |
|---|---|---|
| **Type** | 20 types in 5 tiers (quartz … diamond) | base value; the **affinity** (what it enchants into, §4) |
| **Size** | derived from **weight in carats** | value multiplier; **power** of an enchant (§4); sprite scale (§6) |
| **Quality** | flawed · standard · fine · flawless | value multiplier; **ease of enchanting** (§4) |
| **State** | uncut · cut | uncut is a trade good; cut is needed to set into jewelry (§3) |

### Size from weight (your idea, adopted)
Store the gem's **carats**; the size class is a band of that, not a separate field:

| Size | Carats | Value × | Drop share |
|---|---|---|---|
| tiny | < 0.5 | 0.01 | 42% |
| small | 0.5–2 | 0.1 | 32% |
| medium | 2–5 | 1 | 18% |
| large | 5–15 | 10 | 6% |
| huge | 15+ | 100 | 2% |

Gems are light: carry weight can be carats ÷ 2,000 lb, effectively nothing. That's fine,
because a gem's weight matters for value and enchanting, not for encumbrance.

### Quality
| Quality | Value × | Enchant ease (§4) | Drop share |
|---|---|---|---|
| flawed | 0.5 | hard: lower success, may crack | 30% |
| standard | 1 | normal | 50% |
| fine | 2 | easier, cheaper | 15% |
| flawless | 4 | always succeeds, and a bonus roll | 5% |

**Idea: hidden quality.** An uncut gem's quality is *unknown* until it's cut or appraised
(a jeweller, or an INT / Wisdom check). Uncut gems are then a gamble you can sell rough or
cut and hope. It also gives cutting a reason to exist beyond "required for jewelry".

## 2. Where gems come from

- **Monster drops:** one shared gem roll on every combat loot roll, using the draft's
  CR scaling (×1.2 at CR 1–2 up to ×5 at CR 16+). The draft's 50% base drop rate is likely
  too generous for the economy; ~15–25% feels closer, with large/huge gated behind CR.
- **Mining POIs** (Ironvein Seam etc.), mostly uncut basic/common gems.
- **Geodes:** an item you crack open (`dwarven-geode-cache` encounter) for a random gem.
- **Treasure:** strongboxes, boss hoards, quest rewards (a quest can reward a specific
  flawless gem).

## 3. Cutting and jewelry (OSRS-style chain, beta)

1. **Melt gold:** coins → gold bars at a furnace (Ironpeak forge, Goldenhaven smelter).
   Also silver, since cheaper silver jewelry suits basic gems.
2. **Cut:** an uncut gem → cut, at a jeweller (fee) or with a chisel (skill check). This
   reveals quality; a bad roll can drop it a grade or shatter a flawed gem. Cutting loses
   ~40% of the carats, which is realistic and offset by the cut value multiplier.
3. **Craft:** bar + mould → ring / amulet / bracelet / circlet (unset jewelry).
4. **Set:** jewelry + cut gem → set jewelry. Sells well, even unenchanted.
5. **Enchant:** set jewelry → magic item (§4).

Skills stay stat-derived (no crafting XP, per the hydration rule). Success is a d20 check
on the relevant stat (DEX for cutting and setting), and quality adjusts the DC.

## 4. Enchanting: what each gem does

### Affinities: every type has its own identity
| Tier | Gem | Affinity (stat / element / utility) |
|---|---|---|
| basic | quartz | **Focus:** +spell attack / perception |
| basic | malachite | **Verdance:** poison resistance, herbalism |
| basic | azurite | **Tide:** mana regeneration |
| basic | obsidian | **Shade:** stealth, necrotic edge |
| basic | turquoise | **Wayfarer:** less travel fatigue, safer roads |
| common | onyx | **Ward:** +AC, necrotic resistance |
| common | moonstone | **Night:** darkvision, more mana at night |
| common | citrine | **Dawn:** radiant damage, light |
| common | jasper | **Endurance:** stamina/rage pool, fatigue resistance |
| common | bloodstone | **Vitality:** HP regen, healing received |
| common | pearl | **Grace:** CHA, water breathing |
| common | amber | **Stillness:** lightning resistance, food keeps longer |
| common | amethyst | **Clarity:** WIS, resist charm/fright |
| uncommon | topaz | **Swiftness:** DEX, lightning damage |
| uncommon | garnet | **Vigor:** STR, max HP |
| uncommon | alexandrite | **Shifting:** adapts to the wearer's class's main stat |
| rare | ruby | **Flame:** STR, fire damage |
| rare | sapphire | **Frost & mind:** INT, max mana, cold damage |
| rare | emerald | **Growth:** WIS, healing power, nature spells |
| legendary | diamond | **Purity:** CON, resist all; takes any affinity at full power |

### Size = power (this gives size a role even for flat +stat items)
An enchant's **tier is capped by the gem's size**:

| Size | Enchant capacity | Example (ruby) |
|---|---|---|
| tiny | trinket: +1 to one skill | +1 Athletics |
| small | lesser: minor resistance or utility | fire resistance 10% |
| medium | standard: **+1 stat** | +1 STR |
| large | greater: +2 stat, or +1 and a rider | +1 STR, +1d4 fire on hit |
| huge | legendary: a unique effect | "Emberheart": burning aura |

For **magic combat items** (staffs, wands, foci) size scales continuously instead: spell
damage +2% per carat, capped by tier. A huge sapphire staff is a real caster upgrade.

### Quality = ease (your idea, adopted)
Quality sets the **enchant success chance and cost**, and **flawless** gives a bonus:

- **Flawed:** ~60% success; a failure may crack the gem.
- **Standard:** ~80%.
- **Fine:** ~95% at half the cost.
- **Flawless:** 100%, plus a second minor enchant roll.

### More unique ideas
- **Set resonance:** wearing two items with the same gem type (ruby ring + ruby amulet)
  adds a small set bonus.
- **Class affinity:** sapphire/emerald enchants are stronger on casters, ruby/garnet on
  martials. Alexandrite always matches.
- **Gem charges:** consumable enchants (a "ruby of flame" that charges fire bolts). The
  size sets the charge count; it can be recharged at an enchanter.
- **Socketing weapons/armor** later uses the same affinity table, at half power.

## 5. Non-magic uses (alpha-friendly)

- **Sell** at jewellers/traders, at value × size × quality.
- **Rites and tribute:** vault rites ("any 2 uncut gems"), temple offerings, bribes.
- **Quest turn-ins:** "bring me a fine amethyst".
- **Collection badges:** "find every gem type" or "a huge flawless diamond" — natural
  NIP-58 badges for the Badges tab.

## 6. Sprites

One sprite per type, plus an uncut variant per type, so 40 images. The **UI scales the
sprite by size** (tiny 0.5× … huge 1.1×) and adds a shimmer overlay by quality. No extra
art per size or quality.

## 7. Option A vs B, re-weighed with the axes above

- **A — generated variant items.** Every combination becomes a real item id. With
  type × size × quality × state that's **20 × 5 × 4 × 2 = 800 items**, and continuous
  carats are impossible (only bands). Enchanted jewelry has the same problem: a ring's power
  depends on which gem went in, so the jewelry would need its own combinatorial ids.
- **B — item instances with properties.** One definition per gem type; the inventory slot
  carries `{item, qty, props: {ct, q, cut}}`. It's the foundation every later idea needs:
  set jewelry carries its gem, enchanted items carry their rolled enchant, and durability
  or charges later need it too.

**Re-assessed recommendation: B, done narrowly.**
- One shared helper set (`ItemValue(slot)`, `ItemWeight(slot)`, `SameStack(a, b)`), used by
  the eight systems that touch slots: add/stack/split, move, weight, shop pricing, death
  keep-3, loot placement, vault transfer, and tooltips.
- Items without `props` behave exactly as today.
- Save impact: props are real (non-derivable) data, kept short-keyed for the size budget.

**A is still the faster first step if only loot and selling are wanted in alpha:**
- Generate type × size only (uncut, standard quality: 100 items).
- Move to B when cutting and jewelry arrive.

## 8. Decisions for the maintainer

1. **A or B** (or A-now, B-later)?
2. **Hidden quality** for uncut gems, revealed by cutting or appraisal?
3. **Drop rate** (draft says 50%; suggest 15–25%) and CR gating for large/huge?
4. **Which of §4** — affinities, size = power tiers, quality = ease, set resonance,
   class affinity, charges?
5. **Silver** as a second, cheaper metal?
6. **Unblock now:** add `rough-gem` to early monster loot and/or stock it at the Mountain
   Ore Exchange (Goldenhaven north) so the Ember Vault rite is reachable today?
