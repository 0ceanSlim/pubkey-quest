# Potions, herbs and teas — draft

Brainstorm started 2026-10-30. The last undesigned piece of the pre-beta item systems
(degradation → food → **potions** → fishing → mining → gems).

### Decided so far
- **Mana potions are flat tiers:** a mana draught restores 2, a greater one 4 (healing-style),
  strong at low level and a top-up later. Max mana is casting mod + caster level (~4 at
  level 1, ~20 at 17); spells cost 1–5.
- **New families: cures and utility.** Blade poisons and combat buffs are out of this pass.
- **Every potion must touch a system that exists**, and the design has to be extensible and
  reuse what's built (condition, effects, `cures`, commissions, loot tables).
- **Mana potions are required**: mana is the caster's whole budget (the game's spell points),
  so unlike 5e (spell slots, no restoring potions) there must be a way to buy it back.
- **Drinking is a bonus action** (2024 5e). Today it costs the action, and digging an item out
  of the backpack costs the bonus action; the new rule: a drink is the bonus action, pulling
  it from the pack costs the action. Not built yet (`combat/cast.go`, the use-item path, and
  every potion's "In combat it uses your action" note change with it).
- **Feather fall is cut.**
- **The list comes first** (§4, drafted 2026-10-31); how each is made, found or bought is
  decided after. §2 stays a proposal until then.

### Drafted (2026-10-31): items, effects and sprites in the repo
10 items, 5 effects, 1 stat, 2 hooks. Placed in shops and loot the same day (below).

| Item | Does | Sprite |
|---|---|---|
| `mana-draught` (common, 5,000) | +2 mana | healing's slim vial, blue |
| `greater-mana-draught` (uncommon, 10,000) | +4 mana | greater healing's round flask, blue |
| `superior-mana-draught` (rare, 20,000) | +8 mana | superior's big flask, indigo |
| `owls-eye` (uncommon, 3,000) | `darkvision` effect, 8 h | antidote's flask, amber |
| `climbers-draught` (common, 1,500) | `climbing`: +5 Athletics, 1 h | slim vial, brown |
| `keen-eye-tonic` (common, 1,500) | `keen-eye`: +5 Perception, 1 h | slim vial, yellow |
| `mending-tea` (40) | `mending`: 1 HP / 30 min, 3 h | clay mug, red-brown |
| `meditation-tea` (60) | `meditation`: 1 mana / hour, 3 h | clay mug, silver |
| `settling-tea` (30) | `cures: ["food-poisoning"]` | clay mug, mint green |
| `stamina-tea` (25) | fatigue −2 | clay mug, earthy brown |

- Teas are type `Potion` tagged `tea`, with `spoils.shelf_life` 1440 (stale at 12 h, spoiled
  at a day). They have no `food` block, so food poisoning doesn't stop you drinking them.
- Hooks: `"food-poisoning"` in `inventory.cureEffectIDs`; `combat/darkness.go` counts an
  effect's `darkvision` stat (registered in `systems/effects.json`) like racial darkvision.
- Mana from a draught works mid-fight with no bridge: mana lives on the save, as casting spends it.
- Sprites: the bottles are the healing/antidote sprites with only the liquid recoloured; the
  teas are one PixelLab clay mug (bitforge) with the tea and sprig recoloured. Review sheet:
  `docs/draft/sprites-potions-review.png`. Alternates (corked clay flasks) are in
  `_candidates/tea-flask-test/`.
- Tests: `tests/food/potions_test.go` (draughts, settling tea, Owl's Eye).

### Where they come from (2026-10-31, first pass)
Bought and found only; brewing stays undecided (§2).

- **Teas at taverns**, two per town, default restock:
  Verdant mending + meditation · Kingdom meditation + settling · Goldenhaven settling + stamina ·
  Ironpeak stamina + mending · Marshlight settling + mending · Millhaven stamina + settling ·
  Frosthold stamina + mending · Saltwind settling + stamina · Dusthaven stamina + meditation.
  The Frosthold, Saltwind and Dusthaven innkeepers had no counter; each got a small tea shop
  (`buy_tea` dialogue, `open_shop` on their working shifts).
- **Bottles in shops**, 1 a day restock: Court Arcanist (mana ×3, greater mana ×1) ·
  Shadowmere hermit (mana ×2, Owl's Eye ×1) · Viper (Owl's Eye ×2) · Ironpeak supply master
  (climber's ×3, Owl's Eye ×2) · Frosthold fur trader (climber's ×2) · Verdant bowyer and
  Saltwind tackle keeper (keen-eye ×2 each). Superior mana is never sold.
- **Drops** (rare tier): mana from apprentice wizards, cultists, acolytes, druids and cult
  fanatics (greater from fanatics and druids); greater + superior from wights and vampire
  spawn. Owl's Eye from cultists, bandits, hobgoblins and goblin bosses; climber's from
  goblins, goblin bosses and orcs; keen-eye from gnolls and lizardfolk. Teas never drop
  (they spoil).

## 1. What exists today

| Thing | State |
|---|---|
| Healing ×4 (2d4+2, 4d4+4, 8d4+8, 10d4+20; 5,000–50,000 gold) | Plain healing is in nearly every general store; greater/superior only at the Goldenhaven temple. Monsters drop them |
| Antidote (5,000) | `cures: ["poisoned"]`; Verdant herbalist and the Shadowmere hermit |
| Water breathing (3,000) | `water-breathing` effect; Mother Muriel's quest |
| Mana | Item use already handles a `mana` effect type, and a `mana-regeneration` effect exists (+1 per 30 min for 5 h). No item uses either |
| Herbs | `marsh-willow-bark`, `silver-moss`: spell components (aid, lesser restoration), tagged `herb` |
| Herbalism kit (500), vial (100) | Sold in Verdant; the kit does nothing yet |
| Cures with no potion | `food-poisoning` (degradation), `drunk` (the tavern) |
| Making things today | **Scribing** (stations on buildings, cost table, success roll); **cook/salt** transforms (tinderbox in the wild, innkeeper in town); **commissions** planned for mining (dialogue action + `consume_items` + fee, no engine) |

## 2. How potion-making slots in

Three models were on the table:

| | A. NPC brews for you (commission) | B. Players brew (kit + station + roll) | C. Class-restricted brewing |
|---|---|---|---|
| Engine | None: a dialogue action with `requirements` + `consume_items` + fee, as mining's smith commissions | A scribing-style panel, recipe table, roll, new endpoints | B plus a class gate |
| Fits | Living towns (only on shift), food (innkeeper cooks), mining (smith commissions) | Scribing | Scribing (caster-only by spell knowledge) |
| Risk | Town-bound | Drifts toward general crafting, which is out of alpha | Locks fighters and rogues out of basic cures. Medicine (INT 0.6 / WIS 0.4) already leans to casters without a lock |

**Proposal: A + field teas.**
- **Bottled potions** (healing, mana, antidote, utility) keep, and come from shops, loot, or
  an **NPC commission**: bring the herbs and a fee and the brewer bottles it on the spot,
  cheaper than buying. Each brewer has a family: the herbalist (cures, healing), the court
  arcanist (mana), the hermit (rare utility), the sea priestess (water breathing).
- **Teas** are the field-made tier. Anyone with the **herbalism kit**, a fire (the cooking
  rule: tinderbox in the wild, the hearth at an inn) and a waterskin charge steeps one herb
  into a tea. It's a 1→1 transform like `cook`, with no recipe system and no class lock.
  Teas spoil (`spoils`, about a day), so they're made where they're drunk. This gives the
  kit its job.
- **Later, if wanted (beta extends):** a Medicine roll for a stronger tea, or player-bottled
  potions at a station on the scribing model.

## 3. The long list

Hook: ✅ works today as data · 🔧 small code hook · ❌ needs a system that doesn't exist.

| # | Candidate | What it does | Hook |
|---|---|---|---|
| | **Restoratives** | | |
| 1 | Healing ×4 | HP (keep) | ✅ |
| 2 | Mana draught | +2 mana | ✅ `mana` effect type |
| 3 | Greater mana draught | +4 mana | ✅ |
| 4 | Meditation tea | `mana-regeneration` out of combat | ✅ |
| 5 | Mending tea | `regeneration`, HP over time | ✅ |
| 6 | Potion of vitality (5e) | Cures fatigue and disease, heals | ✅ / 🔧 |
| | **Cures** | | |
| 7 | Antidote | Ends poisoned (keep) | ✅ |
| 8 | Settling tea | Ends food poisoning | 🔧 one line in `cureEffectIDs` |
| 9 | Clear-head draught | Ends drunk | 🔧 same |
| 10 | Elixir of health (5e) | Ends poisoned, blinded, deafened, disease | 🔧 |
| 11 | Cleansing draught | Ends `cursed` | 🔧 |
| | **Survival** | | |
| 12 | Stamina tonic | Fatigue −N (rations already use a fatigue effect) | ✅ |
| 13 | Hunger-staving draught | Hours of fullness, no food | ✅ `food.hours` |
| 14 | Warming draught | Cold exposure | ❌ no cold system |
| 15 | Cooling draught | Desert dehydration | ❌ undesigned |
| | **Utility** | | |
| 16 | Owl's Eye | Darkvision for some hours: no night-combat disadvantage | 🔧 one check in `combat/darkness.go` |
| 17 | Water breathing | (keep) | ✅ |
| 18 | Climber's draught (5e climbing) | Bonus to Athletics checks in POIs | ✅ `skill_athletics` (drafted) |
| 19 | Keen-eye tonic | Bonus to Perception (finds, traps) | ✅ `skill_perception` (drafted) |
| 20 | Silver tongue | Bonus to Influence | ✅ `skill_influence` |
| 21 | Shadowstep draught | `pass-without-trace`, stealth | ✅ effect exists |
| 22 | Wayfarer's tonic | Faster travel | ✅ `travel_speed` stat |
| 23 | Beast-bane incense | Fewer road encounters for a day | 🔧 travel encounter roll (`ambush_ward` only stops ambushes) |
| 24 | ~~Feather-fall draught~~ | Cut (2026-10-31): falls are too rare | — |
| 25 | Speak with animals | (effect exists) | ✅, no real use yet |
| | **Out of this pass** (combat buffs, poisons) | | |
| 26 | Heroism, giant strength, barkskin, resistance, speed, invisibility | Combat | mixed |
| 27 | Blade poisons and oils | Weapon coatings | ✅ `poisoned-blade` |
| 28 | Mind reading, love philter, fire breath | 5e oddities | ❌ |

## 4. The narrowed list (proposal)

The criteria: fills a gap a player feels today, is cheap to hook, and each potion has its own job.

| Potion | Kind | Job | Made by |
|---|---|---|---|
| Healing ×4 | bottled | HP | buy / loot; herbalist brews healing + greater |
| **Mana draught** (+2) | bottled | mana | court arcanist; loot from casters |
| **Greater mana draught** (+4) | bottled | mana | court arcanist, rarer |
| Antidote | bottled | poison | herbalist, hermit |
| **Owl's Eye** | bottled | darkvision at night | hermit |
| Water breathing | bottled | (keep) | sea priestess; Mother Muriel |
| **Settling tea** | tea | food poisoning | anyone with the kit |
| **Stamina tea** | tea | fatigue | anyone with the kit |
| **Meditation tea** | tea | slow mana out of combat | anyone with the kit |
| **Mending tea** | tea | slow HP out of combat | anyone with the kit |

That's 10 lines (13 items with the healing tiers), with 6 new items. Every one is data except
two 🔧 hooks: settling (cure map) and Owl's Eye (darkness check).

On the bench for beta (extends): clear-head draught, skill tonics (needs skill modifiers in
effects: climber's, keen-eye, silver tongue), beast-bane incense, potion of vitality.

## 5. Herbs (first pass)

A small set, each one feeding a tea or a commission. The two that exist stay spell components
too, so casters and brewers share one herb economy.

| Herb | Where | Feeds |
|---|---|---|
| Marsh-willow bark *(exists)* | Shadowmere | Mending tea; healing commissions |
| Silver moss *(exists)* | Shadowmere | Meditation tea; mana commissions |
| Mint (common) | plains, highway, every herbalist | Settling tea |
| Wakeroot | hills, mountains | Stamina tea |
| Owlcap (a mushroom) | Darkwood, at night | Owl's Eye commission |
| Sea kelp | coast (fishing later) | Water breathing commission |
| Bloodmoss | forest, wetlands | Healing commissions |

- **Sources now:** herbalist shops; existing POI loot nodes (the walker's yield tables);
  monster drops. **Later:** fishing (kelp, fish oil) and mining (sulfur, salt) feed in with
  no change here. A Forage action waits on the shared "spot" model fishing and mining use.
- **Shelf life is per-item data** (`spoils`): dried herbs keep; a fresh one (owlcap) can wilt
  if wanted. Teas spoil in about a day.

## 6. Ties to other systems (room to grow, no redesign)

- **Condition:** teas and fresh herbs use `spoils`; the herbalism kit could carry charges (its
  reagents run out, refill at a herbalist) like ink and the whetstone.
- **Effects:** every potion is `heal`, `mana`, `cures` or `apply_effect`.
- **Gems** (gem draft): malachite (herbalism) and azurite (mana regeneration).
- **Quests:** recipe or commission unlocks as quest rewards (derived from the completed-quest
  list, so nothing new in the save); vault-keeper dailies ("bring the herbalist 5 mint").
- **Living towns:** brewers only commission on shift (`npc.OnDuty`).
- **Vials:** a commission takes a vial; drinking a bottled potion leaves the empty vial.

## 7. Open

1. The slot-in model: A + field teas (§2), or something else?
2. The narrowed list (§4): add or cut?
3. Empty vials back on drinking: yes, or do they break?
4. Commission prices: herbs + fee should land well under the shop price (say 40–60%).
