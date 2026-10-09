# Gems, jewelry & enchanting — brainstorm

Status: **DESIGN — first decisions made** (2026-10-08). Source data:
`game-data/systems/gems.json` (not read by any code yet). Only one gem item exists:
`rough-gem` (placeholder, 500 copper), the Ember Vault rite's price.

Scope note: the roadmap keeps **general item crafting** and **magic items** out of alpha.
Gems as loot plus a sell value are alpha-sized; cutting, jewelry and enchanting are beta.

### Decided (2026-10-08)
- **Crafting is done by NPCs, D&D-style.** You bring the materials plus a fee; you don't
  craft yourself. That keeps it to "find the right person", and stat-derived skills stay
  untouched.
- **Jeweller NPC** cuts gems for a fee. **Hidden quality:** an uncut gem's quality is
  unknown until the jeweller cuts it.
- **Smelter NPC** melts gold coins into bars for a fee. **No silver.**
- **Jeweller crafts the finished piece:** bring a gold bar + a cut gem + the fee and get the
  ring or necklace back. There is **no separate "set the gem" step.**
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
A gem's rarity follows its final value (type × size × quality), on bands of its own:

| Rarity | Value (copper) | e.g. |
|---|---|---|
| common | < 1,000 | tiny or small basic and common gems |
| uncommon | 1,000 – 9,999 | medium basic, small rare |
| rare | 10,000 – 99,999 | medium common/uncommon, small diamond |
| very rare | 100,000 – 999,999 | medium rare, large uncommon |
| legendary | 1,000,000+ | large rare, huge anything |

The draft's ×100 for huge makes a huge flawless diamond 200,000,000c (2,000,000 gp). It may
want flattening (huge ×25?) when prices are tuned.

## 2. Where gems come from

- **Monster kills: one shared gem table, overridable per monster.** No monster JSON has a gem
  entry today (31 checked), so a shared table is the base:
  - Every kill rolls it at a **25% base chance**, adjusted by the draft's CR scaling (×1.2 at
    CR 1–2 up to ×5 at CR 16+) on the *type* roll, so tougher monsters drop better gems, not
    more of them.
  - An optional `gem_chance` on a monster overrides the 25%: 0 for beasts that shouldn't carry
    gems, higher for hoarders like dragons and kobolds.
  - Rolled per kill, so a pack of goblins can drop several. Gems land on the fight's loot
    pile with everything else.
- **Mining POIs** (Ironvein Seam etc.), mostly uncut basic/common gems.
- **Geodes:** an item you crack open (`dwarven-geode-cache` encounter) for a random gem.
- **Treasure:** strongboxes, boss hoards, quest rewards (a quest can reward a specific gem).

## 3. The NPC crafting chain (beta)

| Step | NPC | You bring | You get |
|---|---|---|---|
| Melt | **Smelter** (Ironpeak forge; Goldenhaven's Mountain Ore Exchange) | N gold coins + fee | gold bar |
| Cut | **Jeweller** (Goldenhaven, the gem trade) | uncut gem + fee (by type and size) | cut gem, **quality revealed**; ~40% of the carats lost |
| Craft | **Jeweller** | gold bar + cut gem + fee | gold ring or necklace with that gem (unenchanted) |
| Enchant | **Enchanter** (open: Verdant's elves? the Kingdom?) | that ring or necklace + fee | the magic item (§4) |

- Each is an ordinary NPC dialogue action, like the vault rite: `requirements` + `consume_items`
  + a fee, then the result is added. No new UI beyond the dialogue strip.
- Plain gold rings and necklaces (no gem) can be crafted too. They're valuable but have no
  magic.
- **Cutting never fails.** The quality roll is the risk. Enchanting is where quality matters
  (§4).

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
| common | amber | **Stillness:** lightning resistance |
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

### Huge = an effect while worn (decided)
A huge gem's piece carries a full effect from the effects system (`effects_when_worn`), e.g.
an ember aura or a regeneration tick. It's applied on equip and removed on unequip like any
worn item.

### Quality = ease of enchanting (decided)
Quality sets the enchanter's **success chance and fee**. Failure costs the fee only; the
piece survives.

- **Flawed:** ~60%.
- **Standard:** ~80%.
- **Fine:** ~95% at half the fee.
- **Flawless:** 100%, and a second minor enchant.

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
| plain gold ring/necklace, gold bar | fixed | 3 |

- **A — generated variant items:** ~1,500 generated item ids.
  - Every one is an ordinary item, so stacking, weight, price, death, loot, saves, shops
    and the vault all work untouched. That's the real win.
  - The cost is a very large items table and a shop and inventory UI that must group
    variants well.
  - The flawless bonus enchant multiplies the enchanted set further, unless that roll is
    made deterministic (e.g. flawless always adds the gem's minor effect).
- **B — instance properties on the slot** (`{item, qty, p: {t, ct, q}}`).
  - Five definitions: uncut gem, cut gem, ring, necklace, gold bar.
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
  is exactly what the vault rite already does. Under A the jeweller, smelter and enchanter
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

1. **Per kill or per encounter?** Proposal: per kill, 25% each.
2. **`gem_chance` override per monster**: worth having from the start, or add it when a
   monster needs one?
3. **Where the enchanter lives**, and the coins-per-bar ratio.

## 9. Build slices (Option A)

1. **Generator:** migration expands `gems.json` into uncut and cut gem items, with value,
   rarity-from-value, tags, and sprite path + size scale. Replaces `rough-gem`.
2. **Gem drops:** the shared table on every kill (plus mining POIs).
3. **Tag requirements:** dialogue `requirements.items` / `consume_items` accept a tag. The
   Ember Vault rite asks for any 2 uncut gems.
4. **Jeweller (cut) + smelter (bars)** NPCs and dialogue actions.
5. **Jeweller crafting** of gold rings and necklaces.
6. **Enchanter** and the enchanted pieces with their worn effects (beta).
