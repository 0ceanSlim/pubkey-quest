# Item condition: spoilage, wear, burn time and charges

Status: **all decisions taken** (2026-10-20). **Built** (2026-10-25/26): every step of §9, with the
food content from [food-system.md](food-system.md). §9 records what was built, §10 what was decided.

One system for every item that changes as you use it or as time passes:

| Kind | What changes it | What happens at the end |
|---|---|---|
| **Food** spoils | game time | it goes stale, then spoiled (an emergency meal with a food-poisoning risk) |
| **Weapons** wear | dealing damage | lower tiers cut its damage by a percentage; it **never breaks** |
| **Armor** wears | taking damage | lower tiers cut its AC; it **never breaks** |
| **Light sources** burn | time while lit | a torch or candle burns out and is gone; a lamp or lantern runs dry until refilled with oil |
| **Charges** run down | each use | a waterskin empties until refilled; a bottle of ink is used up |

Related drafts: [fishing-system.md](fishing-system.md), potions and herbs (to brainstorm, see §8), camping and the
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
| `b` | lit lights | burn minutes left |
| `u` | charges | uses left |

- **Food stores a timestamp, not a countdown.** Freshness is derived: `now − t` against its shelf
  life. Nothing ticks, nothing is written as time passes, and no delta churn. This is the
  hydration-friendly half: we store the one fact (when it was made) and derive the rest.
- **Wear, burn and charges are counters**, because they change with use, not with the clock.
- **A missing field means new**: pristine, full, or (for food) "made the first time we see it".
  Every existing save just works.

### Stacking: anything with a condition doesn't stack
One rule, no merging maths: **an item that carries condition state stacks 1.**

| Stacks | Doesn't stack (stack 1) |
|---|---|
| Rations (never spoil) | Fresh or stale food that spoils: fish, bread, cheese, berries, meat… |
| **Spoiled food** (one item, no state, §3) | |
| Unlit torches and candles (no state until lit) | A **lit** torch, candle, lamp or lantern |
| Salt, ore, hides, arrows, everything without condition | Weapons and armor (already 1) |
| | Charge items: waterskin, ink, whetstone |

- This means data changes on the items: raw fish (today 20), large fish (10) and any new
  spoiling food go to `stack: 1`. Torch (5) and candle (10) **keep** their stacks, because a
  stack is always unlit.
- **Once food spoils it stacks again.** Spoiled food is all the same: whatever it was, it becomes
  the one `spoiled-food` item (§3), which has no state and stacks.
- Existing saves: a stack of 5 torches is 5 unlit torches, which is exactly what the new rule
  wants. Existing fish and whetstone stacks split into singles on load, into empty slots
  (`inventory.SplitStacks`; with no room the rest stay stacked), each keeping the stack's state.
- **Goodberry stacks (decided).** The spell makes 10 berries at once; they're all made in the
  same minute, share one `t`, and all lose their magic together after 24 hours (the 5e rule),
  when the whole stack disappears rather than going stale (magic berries don't rot into spoiled
  food). A second cast makes a separate stack with its own `t`.

### Where state must travel (every move keeps the slot's fields)
- **Inventory moves:** move, split, containers. (No merges to handle, by the rule above.)
- **Ground items:** drop and pickup.
- **Vault:** today it's `[]VaultEntry{item_id, quantity}`, which can't hold state. **Decided:**
  vault entries get an optional state, one entry per stateful item (they don't stack, so there's
  nothing to split). Banking a worn sword keeps its wear, so the vault can't be used to repair.
- **Shops:** what you buy is new (and lights are bought unlit). Selling pays less by tier (§4, §5).
- **Party loot and trades:** carry the fields.

### The countdown bar
Everything that counts down shows **the same bar** on its inventory slot and in its tooltip, so
there's one thing to learn:
- **Food:** time until it spoils. The bar is green while fresh and amber when stale. Tooltip:
  "Fresh · 2 days left", "Stale · 9 hours left". When it runs out the item turns into spoiled food,
  which has no bar.
- **Lit light:** burn time left. Tooltip: "Lit · 42 minutes left".
- **Wear and charges** use the same bar as a fill level rather than a clock: "Worn · 61%",
  "3 of 5 drinks".
- The frontend computes food's bar from `t` and the clock it already has (`smoothClock.js`), so
  it moves smoothly without server deltas. The server sends `b` with the tick deltas only while a
  light is lit.

## 3. Food and spoilage

Item field: `"spoils": {"shelf_life": 2880}` (minutes). Food without it never spoils and keeps
stacking: rations and beer. Salted and dried foods do spoil, just slowly, so they're stack 1
like other spoiling food.

| Tier | Age vs. shelf life | Eating it |
|---|---|---|
| **Fresh** | under 50% | full nourishment (and any meal bonus) |
| **Stale** | 50–100% | half the hunger restored, no bonus |
| **Spoiled** | past it | it becomes **Spoiled Food** (below) |

**All spoiled food is the same item.** When a fish, a loaf or a cheese passes its shelf life it is
replaced by one `spoiled-food`: a plain stacking item (stack 10) with no state, whatever it used to
be. It stays in your bag as the emergency meal; you can eat it when you're starving, or drop it.

- **Eating it:** restores 1 hunger, then a Constitution save (DC 10) or *food poisoning*.
- **Food poisoning** (a new effect, `food-poisoning`): −1 CON and no hunger recovery for 4 hours.
  It's a natural fit for the antidote and for the potions brainstorm (a cure-ailment draught).
- Sells for nothing. One new sprite.
- **When the swap happens:** in the time-advance path (`gametime.AdvanceTime`), food whose
  `t + shelf_life` has passed is swapped for spoiled food and merged into any spoiled stack. It
  only writes when something actually spoils, so it stays cheap, and the frontend's bar already
  shows it running out.

### More food (content, works with gathering, fishing and camping)
A starting list only. **The food workshop has settled the foods:** see
[food-system.md](food-system.md) (hours of fullness, the full list, cooking at a fire, inn meals).

| Food | Shelf life | Source | Notes |
|---|---|---|---|
| Rations | never | shops | unchanged: dull, reliable, fatigue −3; **still stacks** |
| Bread | 3 days | general stores, inns | cheap |
| Cheese | 10 days | general stores | |
| Apples | 7 days | shops, forest gathering | |
| Wild berries | 1 day | forest and hill gathering | goodberry stays the spell |
| Mushrooms | 2 days | swamp and forest gathering | some poisonous: a Survival check at gathering, the herbalism kit helps |
| Raw fish | 12 hours | fishing, coast and swamp | already exists |
| Cooked fish | 2 days | cooking raw fish | more hunger than raw |
| Salted fish | 30 days | raw fish + salt (salt exists) | preserving: spoils eventually, stack 1 |
| Raw game meat | 12 hours | hunting beasts (boar, deer, bear loot) | |
| Cooked meat | 2 days | cooking | |
| Jerky | 60 days | meat + salt, dried | the long-trip food: spoils eventually, stack 1 |
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
  Flat +1 to start, tune later.
- Monster gear doesn't track.
- **Decided:** these rates stand. A martial weapon at 300 wears out of Pristine in about 75 hits,
  roughly 15–25 fights.

**Tiers** (wear left as a share of max). Weapons never break, armor never breaks. **Decided:**
this table stands.

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
- The tier shows on the item, in its tooltip and as the countdown bar (§2) on equipped slots.
- **Selling** pays the tier's share of value (Worn 75%, Damaged 50%, Ruined 25%).

## 5. Repair

### In the field
**Decided:** field repair only brings gear back up to **Worn**; only a smith makes it new.
- **Whetstone** (exists: reusable, today it gives a pre-fight +1 "Honed Edge"). It also repairs
  **edged** weapons (slashing or piercing) up to Worn.
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

## 6. Light sources: unlit, lit, and the tinderbox

Every light (torch, candle, lamp, bullseye lantern, hooded lantern) has two states:

| | Unlit | Lit |
|---|---|---|
| Stacks | torches and candles stack (5 and 10); lamps and lanterns are 1 anyway | 1 |
| Sprite | **new unlit sprite** (`<id>-unlit.png`) | today's sprite (they're all drawn lit) |
| Gives light | no | yes, while held in the off hand |
| Burns | no | yes, every game minute it's lit (`b` counts down) |

- **The tinderbox lights and puts out.** "Light" and "Put out" are context actions on a light in
  your off hand, and both need a tinderbox in your inventory (the tinderbox isn't used up). No
  tinderbox, no light, so it becomes a real thing to pack. Shops that sell torches should sell
  tinderboxes.
- **Lighting a torch or candle** takes one from the unlit stack and makes it a single lit item.
- **Torches and candles are spent once lit.** When one goes out, whether it burns down or you put
  it out, it's consumed. There are no half-burned torches to track, so the unlit stack stays
  clean.
- **Lamps and lanterns are kept.** Putting one out keeps the oil left in it (`b` stays on the
  slot). At 0 it **runs dry** and stays as an empty lamp until refilled: "Refill" uses a flask of
  **oil** (oil exists, and this is its use).
- **It burns whenever it's lit.** You choose when to light it, so there's no "only in the dark"
  rule: lighting is the decision, and the cost is the burn time.
- 5e burn times: torch 1 hour, candle 1 hour, lamp 6 hours per flask of oil, lanterns 6 hours per
  flask.
- Engine: lit lights tick in the same time-advance path as effects (`gametime.AdvanceTime` →
  effects tick).

**Using tools on items (decided 2026-10-25): the "Use on…" picker.** A tool's default action is
"Use on…" (the tinderbox's reads "Light…"). It opens a small box listing only the items you carry
that it works on, and the server decides what's listed. Pick one and it happens. It works from the
other side too: right-click a torch and "Light" shows if you carry a tinderbox. That one pattern
covers every tool coming in this draft: tinderbox → lights, oil → lamps, whetstone → edged weapons,
crafting kit → armor, salt → raw fish and meat. It works on phones too, and it never offers a pair
that does nothing (OSRS's cursor "use item on item" was considered and passed over for that).

**Lighting a light that isn't in your hand (decided):** it goes into your off hand, lit. Whatever
was there goes back to your bag; if the bag is full it doesn't light, and says why. A lit light
only burns, and only lights, in the off hand. Taking it out of your hand puts it out (a torch or
candle is spent; a lamp keeps its oil).

**The three oil lights differ (decided 2026-10-25), and lights give you away:**
- **A lit light gives you away at night in the wild.** It shows on the noticing checks when an encounter starts and when you sneak away: −5 Stealth. That's the trade-off: a light keeps the dark off in a fight, but things see you coming. (`api/game/light_rules.go`, applied through the travel skill context.)
- **Lamp** (50 gold): the plain light.
- **Bullseye lantern** (1000 gold): its long beam gives +2 Perception at night on the road, both for spotting ambushes and for finding places (`"beam": 2`).
- **Hooded lantern** (500 gold): its shutter closes while lit (`"shutter": true`, slot `s`). It burns on but sheds no light: no −5 Stealth, but you fight in the dark. "Close shutter" / "Open shutter" are on the lantern in your hand.
- **Road POI discovery's passive check** now uses the travel context too, so a spyglass and the bullseye count there.

**Sprites (done):** unlit torch, candle, lamp, bullseye lantern and hooded lantern are `<id>.png`; the lit art is `<id>-lit.png`. They're
today's sprites with the flame and glow taken out pixel by pixel, so they match exactly.

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
  A cure for food poisoning belongs here.
- **Fishing**: produces raw fish (spoils fast, so cook or salt it).
- **Camping**: the fire that cooking needs.

## 9. Build order

**Progress (2026-10-25):**
- **Steps 1–3 are built for food.**
  - The `game/condition` package: slot keys, `ItemInfo` cache, `CanMerge`, the spoil pass, `SellShare`.
  - `t` travels through add, split, stack (refused), containers, drop and pickup (`GroundDrop.State`), the vault (`VaultEntry.State`, codec `[item, qty, state]`), the sell counter (`SellStagedState`) and loot (one per slot, stamped at first sight).
  - Inventory deltas carry `t`.
  - The countdown bar and tooltip line: `src/ui/conditionBar.js`.
  - Hours of fullness: `status.Feed`, `food: {hours, risky_dc}`, stale = half, refused when full or food-poisoned.
  - `spoiled-food`, `food-poisoning`, `meal-hearty`, the goodberry's 24-hour vanish, and breakfast as `WakeWellFed`.
  - Tests: `tests/food`, plus the codec vault round trip.
- **Carried since:** equip and unequip (step 4), and the three items you keep on death (2026-10-26: a light you held goes out).
- **Step 4 (lights) is built:**
  - `condition/light.go`: `l`/`b`, `Ignite`, `Douse`, `Refill`, `BurnHeld` in the time advance and overnight.
  - Lit-only light in `holdsLight`.
  - Equip and unequip now carry condition: a lit light leaving the hand goes out (torch spent, lamp keeps its oil).
  - The "Use on…" picker: `use_targets` / `use_on` (`inventory/tools.go`, `src/ui/toolUse.js`), with Light / Put out / Refill on the items themselves.
  - Light and oil bars on slots.
  - Tests: `tests/food/light_test.go`.
- **Sprites done (2026-10-25):**
  - The 5 unlit lights were edited pixel by pixel from the lit art; PixelLab image-to-image kept the flames.
  - Spoiled food is PixelLab pixflux.
  - Naming: `<id>.png` is now the **unlit** sprite (shops, vault, bag, gear cards). The old lit art is `<id>-lit.png`, shown only for a burning light (`itemSrc` in `src/ui/conditionBar.js`).
- **Step 5 (wear) is built:**
  - The codex migration bakes `durability_max` into each item (`types.DurabilityFor`, from the `condition.json` defaults).
  - `condition/wear.go`: tiers, `AddWear`, the damage and AC cuts, the sell share.
  - Combat: weapons wear on a damaging hit (2 on a crit) and lose damage by tier; armour wears on hits taken (body, then legs, then shield) and loses AC (`combat/wear.go`). Two-handers keep their wear on the main hand.
  - Wear bars and tooltips on gear and bag slots. Tests: `tests/food/wear_test.go`, `tests/combat/wear_test.go`.
- **Step 6 (repair) is built:**
  - `condition.json` gains `materials` (anything not listed is metal) and `repair` (field repair's limit, the whetstone and kit, the five smiths).
  - `condition/repair.go`: `FieldRepair` (back to 74% left; half the way on a failed kit check) and `SmithQuote`.
  - The whetstone's "Sharpen…" (`use_on: sharpen`: slashing or piercing melee weapons, 10 minutes) and the crafting kit's "Mend…" (`use_on: mend`: armour, shields, blunt and ranged weapons, 30 minutes, Crafting DC 12) go through the same picker (`inventory/repair.go`).
  - Smiths get a "🔨 Repair my gear" option: a window of your worn gear with each price, and `smith_repair` mends one (`api/game/repair.go`, `src/ui/smithRepair.js`). The bowyer's owlbear-fur discount uses up one fur.
  - Tests: `tests/food/repair_test.go`.
- **Step 7 (charges) is built:**
  - `"charges": {"max", "refill", "noun"}` on the item; the slot's `u` counts uses spent (`condition/charges.go`). `gameutil.SpendCharge` spends from the part-used one first; anything without a refill is used up on its last charge.
  - Ink: 10 scrolls (scribing spends one per attempt). Whetstone: 10 sharpenings, then it's worn smooth; honing stays free. Waterskin: 5 drinks, Drink and Fill, with water in every town and at the coast, swamp and forest (`condition.json` `water`, `api/game/water.go`). Drinking has no effect yet: it waits for the desert's dehydration, which is still undesigned.
  - The charges bar and tooltip ("3 of 5 drinks left").
  - `ConsumeItem` now clears condition from the slots it empties.
- **Step 8 (content) is built:**
  - New foods: bread, cheese, apple, raw and cooked meat, jerky, cooked and salted fish (small and large). A raw food's `cook` and `salt` name what it becomes.
  - Cook (`inventory/cooking.go`): in the wild, stopped, with a tinderbox, 30 minutes, every raw food at once. Salt…: one salt per food at the same fire.
  - Innkeepers (any NPC who rents rooms) offer "Order a hot meal" (Hearty Stew, 30 gold, 8 hours, +1 CON for 4 hours; one meal buff at a time) and "Cook my food" (2 gold each), from `game-data/systems/cooking.json` (`api/game/inn_kitchen.go`).
  - Raw meat drops from wolves, dire wolves, cave bears, owlbears, hyenas, apes, giant crabs and pteranodons (there are no boars or deer yet). Bread, cheese, apples, jerky and salt are stocked in general stores; barkeeps sell bread.
  - Tests: `tests/food/charges_test.go`, `tests/food/cooking_test.go`, `api/game/water_test.go`, `api/game/inn_kitchen_test.go`.
- **Cleanup (2026-10-26):** the vault window draws every condition bar; the tick's slot deltas carry `w`, `b` and `u` as well as `t`, and an emptied slot drops all its bars.
- **Left for later:** the waterskin's drink does nothing until the desert's dehydration is designed; death keeps the 3 most valuable items by their new price, not their worn one.

1. **Condition core** (`game/condition`):
   - Read and write slot state; freshness from `t`, tier from `w`.
   - The stacking rule: stateful items stack 1 (item data changes for fish); split old fish
     stacks on load.
   - Moves, ground, vault entries with state, shops and loot carry the fields.
   - Tests: round-trip through the codec, missing fields mean new, vault keeps state.
2. **The countdown bar**: one slot and tooltip component for food, lit lights, wear and charges.
3. **Spoilage**: `spoils` on fish and goodberries (goodberry's 24-hour vanish); eating by tier;
   the swap to the stacking `spoiled-food` item (+ sprite); the `food-poisoning` effect.
4. **Lights**: unlit/lit states, Light and Put out with the tinderbox, burn ticking, torches and
   candles consumed when they go out, lamp and lantern oil refills; the 5 unlit sprites.
5. **Wear**:
   - `condition.json` category defaults.
   - Wear on player hits and hits taken; tier penalties in damage and AC.
   - Sell price by tier.
6. **Repair**: whetstone and crafting kit field repair; the smith Repair service with gold and
   material costs per NPC.
7. **Charges**: waterskin, ink and whetstone.
8. **Content**: the new foods and their sources, cooking and preserving (with camping).

## 10. Decisions

Taken (2026-10-20):

1. **Vault:** vault entries keep condition state.
2. **Lights:** replaced by explicit lighting (§6). Lights have unlit and lit states and new unlit
   sprites; the tinderbox lights and puts out; a lit light burns whenever it's lit; torches and
   candles are consumed once they go out; unlit ones stack.
3. **Tier penalties:** the §4 table stands.
4. **Wear rates:** stand as drafted.
5. **Spoiled food:** stays in the bag as an emergency meal with the food-poisoning risk. All
   spoiled food is one item, `spoiled-food`, which stacks.
6. **Field repair:** up to Worn only.
7. **Stacking (new):** anything with a condition doesn't stack. Rations stay a stacking food that
   never spoils.
8. **Countdown bar (new):** one bar on the item for spoilage, burn time, wear and charges.
9. **Goodberry stacks:** one cast is one stack sharing a `t`; all of it vanishes after 24 hours.
10. **Salted and dried foods** (salted fish, jerky) spoil eventually and don't stack. Only rations
    and beer never spoil.
11. **Food list:** workshopped separately, all the foods together in one go.

Nothing open in this draft. The food workshop is done ([food-system.md](food-system.md)); next, build in the §9 order.
