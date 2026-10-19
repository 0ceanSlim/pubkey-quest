# Item condition: spoilage, wear, burn time and charges

Status: **draft for review** (2026-10-19). Nothing built.

One system for every item that changes as you use it or as time passes:

| Kind | What changes it | What happens at the end |
|---|---|---|
| **Food** spoils | game time | it goes stale, then spoiled (eat at your own risk) |
| **Weapons** wear | dealing damage | lower tiers cut its damage by a percentage; it **never breaks** |
| **Armor** wears | taking damage | lower tiers cut its AC; it **never breaks** |
| **Light sources** burn | time while held | a torch or candle burns out; a lamp runs dry until refilled with oil |
| **Charges** run down | each use | a waterskin empties until refilled; a bottle of ink is used up |

Related drafts: fishing (to draft), potions and herbs (to brainstorm, see §8), camping and the
desert's dehydration (item workshop), [spell-scroll-system.md](spell-scroll-system.md) (ink).

## 1. Why now

Hunger is a real thing to manage (0–3, one step every 4 hours, so about three meals a day), but
food is rations, beer, raw and large fish and goodberries. Rations never spoil and are sold
everywhere, so hunger is a shopping chore, not a choice. Meanwhile gear never wears, so smiths
only sell, ore and hides only sell, and the whetstone is a pre-fight +1.

Condition gives a reason to:
- carry the right food;
- cook and preserve;
- fish and gather;
- visit a smith;
- pick a repair material over gold.

## 2. Storage and the hydration rule

Condition **can't be derived** from game data, so it lives on the item's inventory slot, the
same map that already holds `item`, `quantity` and a container's `contents`. The save codec
already keeps any extra slot field (`savecodec.encodeSlot` extras), so no schema version bump is
needed. Short keys keep saves small:

| Field | Kind | Meaning |
|---|---|---|
| `t` | food | the absolute game minute it was made (`CurrentDay*1440 + TimeOfDay`) |
| `w` | weapons, armor | wear points taken so far (0 = pristine) |
| `b` | light | burn minutes left |
| `u` | charges | uses left |

- **Food stores a timestamp, not a countdown.** Freshness is derived: `now − t` against its shelf
  life. Nothing ticks, nothing is written as time passes, and no delta churn. This is the
  hydration-friendly half: we store the one fact (when it was made) and derive the rest.
- **Wear, burn and charges are counters**, because they change with use, not with the clock.
- **A missing field means new**: pristine, full, or (for food) "made the first time we see it".
  Every existing save just works.

### Stacking
- Weapons, armor and lights stack 1, so there's nothing to merge.
- Food stacks. When two stacks merge, the stack takes the **count-weighted average** of their
  timestamps (Don't Starve's rule). It's fair, it's one number, and a fresh fish added to old
  ones freshens the stack a little.
- Items with charges merge only when both are full (two full waterskins), otherwise they stay
  apart.

### Where state must travel (every move keeps the slot's fields)
- **Inventory moves:** move, stack, split, containers.
- **Ground items:** drop and pickup.
- **Vault:** today it's `[]VaultEntry{item_id, quantity}`, which can't hold state. **Decision
  needed:** give `VaultEntry` an optional state (and split entries by it), or let the vault keep
  only plain items (food deposits keep `t`, while gear deposits must be pristine or get refused).
  Recommendation: add state to vault entries. Losing a worn sword's condition by banking it would
  be an exploit (bank and withdraw to repair).
- **Shops:** what you buy is new. Selling pays less by tier (§4, §5).
- **Party loot and trades:** carry the fields.

## 3. Food and spoilage

Item field: `"spoils": {"shelf_life": 2880}` (minutes). Food without it never spoils: rations,
salted and dried foods, beer.

| Tier | Age vs. shelf life | Eating it |
|---|---|---|
| **Fresh** | under 50% | full nourishment (and any meal bonus) |
| **Stale** | 50–100% | half the hunger restored, no bonus |
| **Spoiled** | past it | hunger restored, but a Constitution save (DC 10) or *food poisoning*: −1 CON and no hunger recovery for 4 hours |

Spoiled food is still food, so starving players aren't stuck, but it's a gamble. Tooltips show the
tier and time left ("Fresh · 2 days left").

### More food (content, works with gathering, fishing and camping)
| Food | Shelf life | Source | Notes |
|---|---|---|---|
| Rations | never | shops | unchanged: dull, reliable, fatigue −3 |
| Bread | 3 days | general stores, inns | cheap |
| Cheese | 10 days | general stores | |
| Apples | 7 days | shops, forest gathering | |
| Wild berries | 1 day | forest and hill gathering | goodberry stays the spell |
| Mushrooms | 2 days | swamp and forest gathering | some poisonous: a Survival check at gathering, the herbalism kit helps |
| Raw fish | 12 hours | fishing, coast and swamp | already exists |
| Cooked fish | 2 days | cooking raw fish | more hunger than raw |
| Salted fish | 30 days | raw fish + salt (salt exists) | preserving |
| Raw game meat | 12 hours | hunting beasts (boar, deer, bear loot) | |
| Cooked meat | 2 days | cooking | |
| Jerky | 60 days | meat + salt, dried | the long-trip food |
| Hearty stew | eaten on the spot | inns | a "well fed" buff (e.g. +1 CON for 4 hours) |

**Cooking and preserving** need a fire: a camp (tinderbox, the camping workshop group) or an inn
kitchen. That gives the camp set, tinderbox and salt real jobs.

## 4. Weapons and armor wear

Item field: `"durability": {"max": 300}`. Most items take a **category default** from
`game-data/systems/condition.json` (simple weapon 200, martial 300, light armor 250, medium 300,
heavy 400, shield 300), so 200 item files don't need editing. Magic and quest items can opt out
(`"durability": false`).

**Wear accrues:**
- Weapon: +1 per hit that deals damage, +2 on a crit. Thrown weapons and ammunition don't wear
  (they're spent).
- Armor: +1 per hit that damages you, to one worn piece (body first, then legs, then shield).
  Weight it by the hit's size? Recommendation: flat +1 to start, tune later.
- Monster gear doesn't track.

**Tiers** (wear left as a share of max). Weapons never break, armor never breaks:

| Tier | Durability left | Weapon damage | Armor AC |
|---|---|---|---|
| Pristine | 75–100% | full | full |
| Worn | 50–75% | −10% | −1 |
| Damaged | 25–50% | −25% | −2 |
| Ruined | 0–25% | −50% | half its AC bonus |

- Weapon damage is cut after the roll (floor, min 1), so the dice animation still shows the
  roll and the log names it ("9 slashing, −25% damaged = 6").
- A ruined weapon still swings and ruined armor still protects a little: "mostly unusable", never
  gone.
- The tier shows on the item, in its tooltip and as a small bar on equipped slots.
- **Selling** pays the tier's share of value (Worn 75%, Damaged 50%, Ruined 25%).

## 5. Repair

### In the field
- **Whetstone** (exists: reusable, today it gives a pre-fight +1 "Honed Edge"). It also repairs
  **edged** weapons (slashing or piercing) up to **Worn**, never past it: the field can't make a
  blade new.
  - It takes 10 minutes and has charges (`u`, about 10 sharpenings), then it's worn smooth.
  - The +1 hone stays.
- **Crafting kit** (exists, no use today): field repair of **armor and blunt weapons** up to
  Worn, with a Crafting check for how much (pass: up to Worn; fail: half that). 30 minutes.

### At a smith (back to pristine)
A new NPC service, a "Repair" option and shop tab, shows each worn item, the cost, and what's
missing. The cost is gold scaled by the item's value × wear fraction, sometimes plus a material.
Different smiths, different deals, which gives ore and hides a use:

| Smith | Repairs | Cost |
|---|---|---|
| Master smith (Kingdom) | metal weapons, metal armor | gold only, priciest |
| Forgemistress (Ironpeak) | metal weapons, metal armor | cheap gold **plus iron ore** (1 per 25% of wear) |
| War-gear trader (Marshlight) | everything, rough work | cheap, but only back to Worn |
| Master bowyer (Verdant) | bows, leather and padded armor | gold, or **owlbear fur / hides** for a discount |
| Fur trader (Frosthold) | hide and leather armor | gold plus **hides** |

Repair costs and materials are data (`condition.json` `repair` per NPC), so smiths can be added
without code.

## 6. Light sources

- Torch, candle, lamp (exist, held in the off hand). They burn **only while equipped**, ticking
  with game time (the `b` field), so you can stash a torch to save it.
  - **Decision needed:** burn only at night and in dark places, or whenever equipped?
    Recommendation: only when it's dark, since that's when the light matters. Simpler to
    understand, and still a cost.
- 5e numbers: torch 1 hour, candle 1 hour, lamp 6 hours per flask of **oil** (oil exists, and
  this is its use).
- At 0: a torch or candle **burns out** (it's consumed, the one exception to "never breaks",
  since they're fuel), and a lamp **runs dry** and stays until refilled ("Refill" uses a flask
  of oil).
- Engine: lights tick in the same time-advance path as effects (`gametime.AdvanceTime` →
  effects tick), only for the equipped off-hand item.

## 7. Charges

Item field `"charges": {"max": 5, "refill": "water"}`, stored as `u`.
- **Waterskin:** 5 drinks; refill at a well, an inn or a river (towns, coast, swamp, forest
  roads). Drinking cures the desert's *dehydrated* (item workshop).
- **Ink:** a bottle scribes about 10 scrolls (the scroll draft already says ink becomes a uses
  item), then it's used up.
- **Whetstone:** about 10 sharpenings (§5).
- **Healer's kit:** stays a stack of one-use kits (it's already that).

## 8. Out of scope here, next drafts

- **Potions, herbs and preparing** (your brainstorm): consolidate the existing potions, add the
  missing families (**mana potions**), herb gathering, the herbalism kit, brewing. Potions could
  use the same spoilage rules (`spoils`) for fresh herbal draughts, while bottled potions keep.
- **Fishing**: produces raw fish (spoils fast, so cook or salt it).
- **Camping**: the fire that cooking needs.

## 9. Build order

1. **Condition core** (`game/condition`):
   - Read and write slot state; freshness from `t`, tier from `w`.
   - Merge and split rules in inventory add, move and stack; ground, vault (per the decision),
     shops and loot carry it.
   - Tests: round-trip through the codec, merges, missing fields mean new.
2. **Spoilage**: `spoils` on fish and goodberries; eating by tier; food poisoning effect;
   tooltips.
3. **Wear**:
   - `condition.json` category defaults.
   - Wear on player hits and hits taken; tier penalties in damage and AC.
   - Equipped-slot bar; sell price by tier.
4. **Repair**: whetstone and crafting kit field repair; the smith Repair service with gold and
   material costs per NPC.
5. **Lights and charges**: burn ticking, oil refills; waterskin, ink and whetstone charges.
6. **Content**: the new foods and their sources, cooking and preserving (with camping).

## 10. Decisions for you

1. **Vault and condition:** store state on vault entries (recommended), or refuse worn gear?
2. **Light burn:** only in the dark (recommended), or whenever equipped?
3. **Tier penalties:** the table in §4 (−10/−25/−50% damage, −1/−2/half AC). Too harsh, too
   soft?
4. **Wear rates:** a martial weapon at max 300 wears through Pristine in about 75 hits,
   roughly 15–25 fights. Faster or slower?
5. **Spoiled food:** edible with a food-poisoning risk (recommended), or useless?
6. **Field repair cap:** whetstone and kit can only bring gear back to Worn (recommended), or all
   the way?
7. **Food list:** which of §3's foods to add first. Fish and berries with gathering? Bread and
   cheese in shops?
