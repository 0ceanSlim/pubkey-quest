# Gems, jewelry & enchanting — brainstorm

Status: **DESIGN — first decisions made** (2026-10-08). Source data:
`game-data/systems/gems.json` (not read by any code yet). Only one gem item exists:
`rough-gem` (placeholder, 500 gold), the Ember Vault rite's price.

Scope note: the roadmap keeps **general item crafting** and **magic items** out of alpha.
Gems as loot plus a sell value are alpha-sized; cutting, jewelry and enchanting are beta.

### Decided (2026-10-08)
- **Crafting is done by NPCs, D&D-style.** You bring the materials plus a fee; you don't
  craft yourself. That keeps it to "find the right person", and stat-derived skills stay
  untouched.
- **Jeweller NPC** cuts gems for a fee. **Hidden quality:** an uncut gem's quality is
  unknown until the jeweller cuts it.
- **No smelter, no bars, no silver.** The jeweller works straight from gold coins.
- **Jeweller crafts the finished piece:** bring the coins + a cut gem + the fee and get the
  ring or necklace back. There is **no separate "set the gem" step** (pricing in §3).
- **Several enchanters,** each with their own pieces, success rates and fees (§4).
- **Only rings and necklaces** are enchanted. No staffs, wands, foci, socketing or charges,
  and nothing that degrades (so no food-spoilage affinity).
- **Affinities** (§4) are good as drafted, minus amber's food preservation.
- **Size caps power; huge gems grant a full effect while worn.**
- **Set bonuses** aren't a separate mechanic: two ruby pieces simply stack their effects.
- **Class affinity** is dropped as too complicated.
- **Rarity comes from value**, like every other item (§1).
- **Sprites** as drafted: one per type (cut and uncut), scaled by size, shimmer by quality.
- **Option A:** generated variant items from `gems.json` (§7).
- **No stopgap.** `rough-gem` is a placeholder, not a real item. It isn't seeded into loot or
  shops; it gets replaced when the generator lands, and the vault rite moves to "any 2
  uncut gems" by tag.
- **Huge stays ×100**, made rare enough to earn it: **0.5%** of gem drops (§1).
- **Drops roll per kill, with a per-monster `gem_chance` override** (§2).

---

## 1. What a gem is: four axes

| Axis | Values | What it drives |
|---|---|---|
| **Type** | 20 types, quartz (1,000) … diamond (500,000) | base value; the **affinity** (what it enchants into, §4) |
| **Size** | derived from **weight in carats** | value multiplier; **power** of an enchant (§4); sprite scale (§6) |
| **Quality** | flawed · standard · fine · flawless | value multiplier; **ease of enchanting** (§4) |
| **State** | uncut · cut | uncut is a trade good; cut is needed to set into jewelry (§3) |

### Size from weight (your idea, adopted)
Store the gem's **carats**; the size class is a band of that, not a separate field:

| Size | Carats | Value × | Drop share |
|---|---|---|---|
| tiny | < 0.5 | 0.01 | 44% |
| small | 0.5–2 | 0.1 | 33% |
| medium | 2–5 | 1 | 18.5% |
| large | 5–15 | 10 | 4% |
| huge | 15+ | 100 | **0.5%** |

**Why ×100 is fine at 0.5%:** a drop's expected size multiplier is Σ share × multiplier =
0.0044 + 0.033 + 0.185 + 0.4 + 0.5 ≈ **1.1×** the medium price. Huge gems carry about half
of that expected value but turn up once in 200 gems. With a 25% gem chance per kill, that's
one huge gem in ~800 kills, and a huge *diamond* (1% of types) one in ~80,000.

Gems are light: carry weight can be carats ÷ 2,000 lb, effectively nothing. That's fine,
because a gem's weight matters for value and enchanting, not for encumbrance.

### Quality
| Quality | Value × | Enchant ease (§4) | Drop share |
|---|---|---|---|
| flawed | 0.5 | hard: lower success, may crack | 30% |
| standard | 1 | normal | 50% |
| fine | 2 | easier, cheaper | 15% |
| flawless | 4 | always succeeds, and a bonus roll | 5% |

**Hidden quality (decided).** An uncut gem's quality is unknown until the jeweller cuts
it. Uncut gems are a gamble: sell them rough at the standard-quality price, or pay to cut
and hope.

**Quality is rolled at the moment of cutting, not stored on the uncut gem.** To the player
it's the same "hidden until cut", but nothing secret sits in the save. That matters
because saves become public Nostr events (roadmap §4): a quality stored on the uncut gem
would be readable by anyone. The player can still reload a save to re-cut. That's the same
accepted revert freedom as everywhere else, and the fee makes it a loss.

### Rarity from value (decided)
A gem's rarity follows its final value (type × size × quality), mapped onto the game's
five item rarities. Values are in **gold**: the game counts everything in one coin, the gold
piece, worth what D&D calls a copper.

| Rarity | Value (gold) | Standard-quality examples |
|---|---|---|
| 5–10k | < 1,000 | tiny or small basic and common gems, tiny rare gems |
| 50,000 | 1,000 – 9,999 | medium basic gems, small amethyst/pearl/amber |
| 100,000 | 10,000 – 99,999 | medium common and uncommon gems, large basic, small ruby/sapphire/emerald |
| 500,000 | 100,000 – 999,999 | medium ruby/sapphire/emerald/diamond, large topaz, huge basic |
| mythic | 1,000,000+ | large rare, huge common and up |

Quality shifts a gem up or down: a flawless medium amethyst is 40,000 (rare), a flawed one
5,000 (uncommon).

**No type tiers** (decided): a gem's type only contributes its `base_value` and drop
weight; rarity comes from the final value alone. The draft's `tier` field was removed from
`gems.json`.

**Top rarity is `mythic` everywhere** (decided): the codex validator and item editor were
changed from `mythical` to match the game UI.

Huge is ×100 by decision, so a huge flawless diamond is 500,000 × 100 × 4 = **200,000,000
gold**. That's deliberately astronomical at a 0.5% size roll × 1% type roll × 5% flawless.

## 2. Where gems come from

- **Monster kills: one shared gem table, overridable per monster.** No monster JSON has a gem
  entry today (31 checked), so a shared table is the base:
  - Every kill rolls it at a **25% base chance**, adjusted by the draft's CR scaling (×1.2 at
    CR 1–2 up to ×5 at CR 16+) on the *type* roll — shifting weight toward higher-`base_value` types — so tougher
    monsters drop better gems, not more of them.
  - **Decided:** an optional `gem_chance` on a monster overrides the 25%: 0 for beasts that
    shouldn't carry gems, higher for hoarders like dragons and kobolds.
  - **Decided:** rolled per kill, so a pack of goblins can drop several. Gems land on the fight's loot
    pile with everything else.
- **Mining POIs** (Ironvein Seam etc.), mostly uncut basic/common gems.
- **Geodes:** an item you crack open (`dwarven-geode-cache` encounter) for a random gem.
- **Treasure:** strongboxes, boss hoards, quest rewards (a quest can reward a specific gem).

## 3. The NPC crafting chain (beta)

| Step | NPC | You bring | You get |
|---|---|---|---|
| Cut | **Jeweller** (Goldenhaven, the gem trade) | uncut gem + cutting fee | cut gem, **quality revealed**; ~40% of the carats lost |
| Craft | **Jeweller** | gold coins + cut gem + labour fee | gold ring or necklace with that gem (unenchanted) |
| Enchant | **An enchanter** (several; §4) | that ring or necklace + their fee | the magic item |

Each step is an ordinary NPC dialogue action, like the vault rite: `requirements` +
`consume_items` + a fee, then the result is added. No new UI beyond the dialogue strip.

### Jeweller pricing (proposal from the maintainer's figures)
| Piece | Gold it takes | Labour | Piece is worth |
|---|---|---|---|
| ring | **1,000** | 10% of the cut gem's value | 1,000 + gem value |
| necklace | **2,000** | 10% of the cut gem's value | 2,000 + gem value |

- The coins become the piece's gold, so their value isn't lost: a piece is worth its gold
  plus its gem. Only the labour fee is spent, and it scales with what the piece is worth.
- **Cutting fee:** a flat fee by size (e.g. tiny 50 … huge 5,000). Since quality is unknown
  until cut, the fee can't follow the cut value.
- **Weights:** ring 0.02 lb, necklace 0.05 lb. A thousand coins melted into a ring weigh far
  less than the coins did, which is the point of the jeweller.
- **Plain gold** rings and necklaces (no gem) cost the gold plus a flat labour fee.

## 4. Enchanting: what each gem does

### Affinities: every type has its own identity
| Base value | Gem | Affinity (stat / element / utility) |
|---|---|---|
| 1,000 | quartz | **Focus:** +spell attack / perception |
| 1,000 | malachite | **Verdance:** poison resistance, herbalism |
| 1,000 | azurite | **Tide:** mana regeneration |
| 1,000 | obsidian | **Shade:** stealth, necrotic edge |
| 1,000 | turquoise | **Wayfarer:** less travel fatigue, safer roads |
| 5,000 | onyx | **Ward:** +AC, necrotic resistance |
| 5,000 | moonstone | **Night:** darkvision, more mana at night |
| 5,000 | citrine | **Dawn:** radiant damage, light |
| 5,000 | jasper | **Endurance:** stamina/rage pool, fatigue resistance |
| 5,000 | bloodstone | **Vitality:** HP regen, healing received |
| 10,000 | pearl | **Grace:** CHA, water breathing |
| 10,000 | amber | **Stillness:** lightning resistance |
| 10,000 | amethyst | **Clarity:** WIS, resist charm/fright |
| 50,000 | topaz | **Swiftness:** DEX, lightning damage |
| 50,000 | garnet | **Vigor:** STR, max HP |
| 50,000 | alexandrite | **Shifting:** adapts to the wearer's class's main stat |
| 100,000 | ruby | **Flame:** STR, fire damage |
| 100,000 | sapphire | **Frost & mind:** INT, max mana, cold damage |
| 100,000 | emerald | **Growth:** WIS, healing power, nature spells |
| 500,000 | diamond | **Purity:** CON, resist all; takes any affinity at full power |

### Size = power (this gives size a role even for flat +stat items)
An enchant's **tier is capped by the gem's size**:

| Size | Enchant capacity | Example (ruby) |
|---|---|---|
| tiny | trinket: +1 to one skill | +1 Athletics |
| small | lesser: minor resistance or utility | fire resistance 10% |
| medium | standard: **+1 stat** | +1 STR |
| large | greater: +2 stat, or +1 and a rider | +1 STR, +1d4 fire on hit |
| huge | **that gem's unique effect** (one of 20, authored) | "Emberheart": an ember aura |

### Every jewelry effect is a worn effect (decided)
Enchanted rings and necklaces grant their effect the same way any worn item does: through
`effects_when_worn`, applied on equip and removed on unequip. There's no separate jewelry
mechanism.

- **Tiny → large:** the effect follows the gem's affinity and the size tier in the table
  above. Each affinity × tier is one effect definition in `game-data/effects/`, so 4 tiers ×
  20 gems = 80 effects. The generator can produce these from a small table (stat or
  resistance, and amount per tier).
- **Huge:** each gem type has **one unique effect, 20 in all, authored by hand** by the
  maintainer. Each stays in line with its gem's affinity but is clearly better than anything
  the regular tiers give. The generator points huge pieces at them by id
  (`jewel-{type}-huge`); until one is written, its placeholder falls back to that gem's
  large-tier effect.

### Quality = ease of enchanting (decided)
Quality shifts the **success chance and fee**. Failure costs the fee only; the piece
survives.

- **Flawed:** −20% success.
- **Standard:** base.
- **Fine:** +15%, at a lower fee.
- **Flawless:** always succeeds, and adds a second minor enchant.

### Enchanters differ (decided)
Several enchanter NPCs, each with an `enchant_config` on their NPC JSON:

| Field | Meaning |
|---|---|
| `pieces` | what they'll work on: ring, necklace or both |
| `affinities` | which gem affinities they know (all, or a specialty) |
| `max_size` | the largest size they can handle (a hedge-witch can't touch huge gems) |
| `base_success` | their base chance before quality |
| `fee_multiplier` | their price relative to the base fee (base fee scales with enchant tier) |

Example roster, placements open:

| Enchanter | Where | Character |
|---|---|---|
| Marsh hedge-witch | Marshlight | cheap, 60% base, small gems only, any affinity |
| Elven enchantress | Verdant | 90% base, expensive, nature and clarity specialties |
| Dwarven runesmith | Ironpeak | 80%, mid price, ward / vigor / endurance only, any size |
| Goldenhaven arcanist | Goldenhaven | 85%, priciest, any affinity, the only one for huge gems |

### Stacking (decided)
Two pieces with the same gem simply add their effects: ruby ring + ruby necklace = +2 STR.
There is no special set-bonus mechanic.

### Dropped
Class affinity, enchanted staffs/wands/foci, socketing weapons or armor, rechargeable
charges, anything that degrades.

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

## 7. Option A vs B, re-assessed with the decisions

What actually varies per piece now:

| Thing | Varies by | Combinations |
|---|---|---|
| uncut gem | type × size *(quality is rolled at cutting, not stored)* | 20 × 5 = **100** |
| cut gem | type × size × quality | 20 × 5 × 4 = **400** |
| unenchanted piece | ring/necklace × gem type × size × quality *(quality is still needed for enchanting)* | 2 × 400 = **800** |
| enchanted piece | ring/necklace × gem type × size *(+ flawless bonus roll)* | 2 × 100 = **200+** |
| plain gold ring/necklace | fixed | 2 |

- **A — generated variant items:** ~1,500 generated item ids.
  - Every one is an ordinary item, so stacking, weight, price, death, loot, saves, shops
    and the vault all work untouched. That's the real win.
  - The cost is a very large items table and a shop and inventory UI that must group
    variants well.
  - The flawless bonus enchant multiplies the enchanted set further, unless that roll is
    made deterministic (e.g. flawless always adds the gem's minor effect).
- **B — instance properties on the slot** (`{item, qty, p: {t, ct, q}}`).
  - Four definitions: uncut gem, cut gem, ring, necklace.
  - The props say which gem, how big and how good, and an enchanted piece's effect is
    computed from them.
  - The cost is one shared helper set (`ItemValue`, `ItemWeight`, `SameStack`,
    `ItemRarity`, `WornEffects`) threaded through the eight systems that touch slots:
    stacking, moving, weight, shop pricing, death keep-3, loot placement, vault, tooltips.
  - Equip would derive `effects_when_worn` from the props instead of the item definition.

**What changed since the brainstorm:** with NPC crafting and only rings and necklaces to
enchant, A comes to ~1,500 ids. That's large, but every one is generated and each is a
real, finite thing a player can hold. B's work stayed the same size. Two things now favour A:
- **Nothing in this design needs a continuous value.** Size works in five bands,
  quality in four tiers.
- **Every NPC step is a dialogue action that consumes item ids and grants item ids.** That
  is exactly what the vault rite already does. Under A the jeweller and the enchanters
  need **no new engine**, just generated data and dialogue nodes.

**Recommendation: A, generated from `gems.json` at migration.** It keeps all of this in
data, fits the existing dialogue-action and requirements machinery, and leaves the save
format untouched (id + quantity).

Three guard-rails make A comfortable:
1. **The generator owns the ids:** `gem-{type}-{size}-uncut`, `gem-{type}-{size}-{quality}`,
   `{ring|necklace}-{type}-{size}-{quality}` and `…-enchanted`. Nothing is hand-authored, and
   the grammar is regular, so a later move to B is a mechanical id → props migration.
2. **Tags carry the axes** (`gem`, `gem-ruby`, `uncut`, `size-large`, `quality-fine`), so
   requirements like "any 2 uncut gems" and UI grouping work by tag.
3. **The UI groups by tag:** one "Gems" family in shops and tooltips, so 1,500 ids never
   show as a 1,500-row list.

Choose B instead if magic items later need per-piece rolls that the id grammar can't hold,
such as random enchant strengths or durability. Those were ruled out today.

## 8. Still open

1. **Jeweller figures:** 1,000 / 2,000 gold + 10% labour, and the cutting fee table — OK?
2. **Enchanter roster:** which ones, where, and their numbers.

## 9. Build slices (Option A)

1. **Generator:** migration expands `gems.json` into uncut and cut gem items, with value,
   rarity-from-value, tags, and sprite path + size scale. Replaces `rough-gem`.
2. **Gem drops:** the shared table on every kill (plus mining POIs).
3. **Tag requirements:** dialogue `requirements.items` / `consume_items` accept a tag. The
   Ember Vault rite asks for any 2 uncut gems.
4. **Jeweller cutting:** NPC + `cut_gem` dialogue action (quality rolled at the cut).
5. **Jeweller crafting:** gold rings and necklaces from coins + cut gem + labour.
6. **Enchanters:** `enchant_config` NPCs, and the enchanted pieces with their worn effects (beta).
