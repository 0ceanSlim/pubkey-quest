# Mining: seams, deep mines, ore and the Veins minigame

Status: **DRAFT** (2026-10-22). Nothing built. Roadmap slot: M5.5 "mining/fishing minigames",
plus beta work for the smith commissions. Fishing has its own draft
([fishing-system.md](fishing-system.md)) on the same session pattern; §9 lists what they share.

## What exists today

| Piece | State |
|---|---|
| `ironvein-seam` (Cragspire, utility POI) | `[Pickaxe]` choice → Athletics DC 10 → loot (1–2 iron ore, 60% 1–3 copper ore) → back to the seam. **Free and endless**: no time cost, no fatigue, no daily cap. |
| `flooded-mine` (Cragspire, dungeon POI) | Rope-gated shaft, zombies, a shovel stash. No mining in it. |
| Items | `pick-miners` (200, tool, improvised weapon), `iron-ore` (40), `copper-ore` (25), `heartstone-ore` (quest item). Ores only sell. |
| Gems | `docs/draft/gem-system.md` is decided but **the generator isn't built**, so there are no gem items to mine yet. |
| Minigame engine | Tumblers (`cmd/server/game/locks/`): a server-held session with a versioned action protocol. The pattern to copy. |
| Degradation | `docs/draft/degradation-system.md` is decided, not built: wear counters, never-break tiers, the Forgemistress repairs metal for gold + iron ore, lit lights burn down. |

## Decided (2026-10-22)

- **The minigame is a rock-face puzzle** (Veins, §3), held and resolved by the server like
  Tumblers. A quick roll stays for players who skip it.
- **Seams deplete per in-game day, in session memory only** (like picked locks). Each face
  also costs game time and fatigue.
- **Ore has four uses:** selling and quests, smith repair material, NPC smith commissions
  (no player crafting), and it feeds **pickaxe tiers and wear**.
- **Mining happens in surface seams and deep mines.** No town mine or mining shifts.

Standing rules this draft keeps: no per-skill XP or professions (skills stay stat-derived);
gear never breaks; nothing secret or derivable goes in the save; gold is the only currency.

---

## 1. The loop

```
find a seam (discovery) → work a face (Veins or quick) → ore + maybe gems land in the pack
   → haul it home (it's heavy: encumbrance is the brake on a trip)
   → sell it · repair gear with it · commission a better pick or deep-ore gear
   → the better pick opens harder ore → deeper mines → richer gems and rarer ore
```

Progression is **gear and character, not a mining level**: the pick's tier, Athletics (how
many swings), Perception (how well you read the rock), a light source, and knowing where the
good seams are.

## 2. Where you mine

### Surface seams (utility POIs)
Small, quick, mostly tier 1–2. Each has **3 faces a day**. Daylight is enough; at night you
need a lit light (§5). Proposed set, one character per region:

| Seam | Environment | Tier | Ore | Gem leaning |
|---|---|---|---|---|
| Ironvein Seam *(exists)* | Cragspire Mountains | 1 | iron, copper | quartz, jasper |
| Redcliff Cut | Windswept Hills | 1 | copper, iron | amethyst, citrine |
| Glasswind Scarp | Sunscorch Desert | 2 | iron, copper | obsidian, turquoise, topaz (rare) |
| Rimewall Outcrop | Frozen Wastes | 2 | iron, deepiron (rare) | moonstone, quartz, sapphire (rare) |
| Mossback Quarry | Darkwood Forest | 1 | copper | malachite, amber, emerald (rare) |
| Bloodfen Ledge | Shadowmere Wetlands | 1 | iron | bloodstone, malachite |

Pearls are left to fishing on purpose.

### Deep mines (dungeon POIs with levels)
Multi-node walks where **depth is the difficulty dial**. Each level has its own faces (one
working each a day), tougher hazards and better ore. The deepest level is the only place
mithral and adamantine turn up.

| Level | Tier | Needs | Hazards | Monsters (existing) |
|---|---|---|---|---|
| 1 Adit | 2 | light | loose rock | swarm-of-rats, kobold |
| 2 Galleries | 3 | light, rope | bad air, flooding | kobold-shaman, giant-spider, skeleton, zombie |
| 3 Deeps | 4 | light, rope, tier-3 pick to mine | cave-ins, gas, dark | grimlock, gray-ooze, ghoul, cave-bear, wight |

- **The Flooded Mine** grows into the first one: today's shaft bottom becomes level 1, the
  water passage leads to level 2 (the Potion of Water Breathing finally has a dig-site use).
- Two more proposed: **Rimehollow Delve** (Frozen Wastes, moonstone and sapphire, cold
  hazards) and **The Glass Pits** (Sunscorch, obsidian and topaz, heat and gas).
- Hazards are ordinary POI nodes (checks, damage, effects, fatigue); only the faces are new.
  Bad air: a Resolve check or +2 fatigue. Flooding: water breathing, an Athletics check, or
  turn back. Gas near a naked flame: a Perception check to notice it, else fire damage.
- A cave-in in a deep mine can route to a different `failure_next`: the way you came is
  blocked and you take the long way out.

## 3. Veins — the minigame

### The face
A hidden grid of rock, set by the face's tier:

| Tier | Grid | Ore veins | Gem cells | Faults | Hard rock |
|---|---|---|---|---|---|
| 1 | 5×4 | 1–2 | 0–1 | 1–2 | 10% |
| 2 | 6×5 | 2 | 0–1 | 2–3 | 20% |
| 3 | 7×5 | 2–3 | 0–2 | 3–4 | 30% |
| 4 | 8×6 | 3 | 1–2 | 4–5 | 40% |

Cell kinds: **rock** (nothing), **ore** (one ore type), **gem** (one uncut gem, type and size
rolled at layout), **fault** (a crack in the face), **hard** (rock or ore that takes two
strikes with a weak pick). Ore is laid out in **veins**: short connected runs of 2–4 cells,
so a find tells you where to look next. That is what makes reading the rock pay.

### The moves
| Move | Cost | What it does |
|---|---|---|
| **Sound** a cell | 1 minute, one sounding | A Perception check vs the face DC. Success: the cell's **ring**, the number of ore and gem cells around it (8 neighbours), plus "hollow" if the cell itself is a fault and "too hard" if it's ore your pick can't cut. Failure: a dull thud, nothing learned. |
| **Strike** a cell | 5 minutes, one swing, +1 pick wear | Breaks it. Ore: 1–2 chunks straight into your pack. Rock: nothing. Hard: cracks it (a second strike breaks it; one strike with a tier-2+ pick). Fault: **+1 crack** and +3 pick wear. Gem, struck blind: it **chips a size down** (tiny is lost). |
| **Pry** an exposed gem | 5 minutes, one swing | Once any neighbour of a gem cell is broken, the gem shows as a glint. Prying it frees it whole. |
| **Stop** | free | Leave the face with what you have. |

- **Swings:** 6 + Athletics modifier + the pick's bonus, minimum 3. Run out and your arms
  are done: the face ends, no penalty.
- **Soundings:** 3 + Perception modifier, minimum 2.
- **Cracks:** the third crack brings the face down (a cave-in). You take the tier's damage
  (1d6 per tier, an Athletics check vs the DC halves it), the face ends, and it counts as
  worked. Two cracks shows a warning.
- **Glint (Perception +2 and up):** a successful sounding next to a gem sometimes adds
  "something glitters" (15% per point above +1, at most 60%), mirroring the Tumblers glint.
- Any other game action abandons the face for free. Nothing is lost, because everything you
  break out is granted the moment you break it. A full pack sends it to the ground store.

What the player is doing: sound a few cells, follow the numbers to a vein, strike along it,
steer around hollow cells, and clear the rock beside a glint before prying instead of
smashing it. Good Perception reads the face; good Athletics works more of it; a better pick
gets through hard rock and opens deep ore.

### The quick roll
The lock-style quick option: one Athletics check vs the face DC, 30 minutes, uses a face.
Success rolls the seam's authored `quick_loot` table (today's Ironvein table becomes this),
failure finds nothing. Quick yields are tuned to about **60% of a decent Veins run** and
never include gems above small or ore above tier 2, so the game is worth playing but never
required. A settings toggle (like text speed) can make quick the default button.

### Protocol (mirrors locks)
Everything goes through `POST /api/game/action`, under the session lock.

- `mine_begin {node, style: "veins"}`: the server checks you're at the face, it has faces
  left today, and you carry a pick. It lays out the hidden face and stores it as
  `GameSession.MineSession`. Replies with `data.mine` (the public view).
- `mine_act {session_id, version, act: "sound"|"strike"|"pry"|"stop", cell}`: same version
  rule as `lock_act`. A stale version gets `data.stale: true` and changes nothing; invalid
  moves are refused unchanged. The server applies each consequence exactly once (time, wear,
  items, damage). Replies with `data.mine` and `data.lines`, plus `data.step` once the face
  ends (back to the seam's choices).
- **Public view:** `session_id, name, style, version, status, tier, grid, swings,
  swings_max, soundings, cracks, cracks_max`, and per cell only what you've earned:
  `unknown | sounded{ring, hollow?, too_hard?} | broken{kind} | glint`. The layout is never
  in the view.
- **Status:** `active`, then `exhausted` (out of swings), `collapsed`, `stopped` or
  `abandoned`.
- The face is rolled fresh from a session rng each time. Reloading a save and re-rolling a
  face is the same accepted freedom as re-picking a lock; the daily cap and the time cost
  make it not worth much.

## 4. Seam data

### `game-data/systems/mining.json` (globals)
- Tier table (§3): grid, vein/gem/fault/hard counts, default DC, cave-in dice.
- Times: `swing_minutes` 5, `sound_minutes` 1, `quick_minutes` 30, fatigue per face 1.
- The gem curve by tier (§6).
- Ore definitions: which tier each ore is and what pick tier cuts it.

### A face node in a POI
A new POI step type, `mine`, beside `lock`:

```json
"face": {
  "type": "mine",
  "text": "The ore vein runs deep into the rock.",
  "seam": {
    "tier": 1,
    "dc": 10,
    "faces_per_day": 3,
    "ores": [
      { "item": "iron-ore",   "weight": 60, "quantity": [1, 2] },
      { "item": "copper-ore", "weight": 40, "quantity": [1, 3] }
    ],
    "gems": { "chance": 0.35, "types": { "quartz": 3, "jasper": 2 } },
    "extras": [ { "item": "geode", "chance": 0.1 } ],
    "quick_loot": { "...": "a normal POILootTable" }
  },
  "next": "seam",
  "failure_next": "seam"
}
```

- The node shows `#work` (Veins) and `#quick` choices, the way a lock node shows `#pick`
  and `#quick`. Both are hidden when the seam is spent for the day ("The face is worked
  out. Come back tomorrow.") or you carry no pick.
- Any field missing from `seam` falls back to the tier defaults, so a basic seam is a few
  lines.
- `faces_per_day` is per node, so a deep mine's levels count separately.
- **Daily state:** `GameSession.Seams[poi#node] = {day, worked}`, memory only, reset at the
  in-game day rollover. Never saved.
- `--check-schema` validates `mine` nodes: ore ids exist and carry the `ore` tag, gem types
  exist in `gems.json`, tier is 1–4, quantities are sane.

## 5. Tools and light

### Pick tiers
Picks get a `mining` block (`{"tier": 1, "swings": 0}`):

| Pick | Tier | Swings | How you get one |
|---|---|---|---|
| Pickaxe (`pick-miners`, exists) | 1 | +0 | shops, 200 |
| Dwarven pick | 2 | +2, one-strike hard rock | commission at Ironpeak (§7) |
| Deepiron pick | 3 | +3, cuts mithral and adamantine | commission at Ironpeak (§7) |

The war pick is a weapon and doesn't mine. Picks stay improvised weapons in combat.

### Pick wear
Under the degradation system, picks take a `tool` durability default (proposal: 200; the
dwarven 300, the deepiron 400). Wear: +1 per strike, +3 per fault struck. Picks never break.

| Tier | Effect on a face |
|---|---|
| Pristine | full |
| Worn | −1 swing |
| Damaged | −2 swings; hard rock takes an extra strike even with a good pick |
| Ruined | half swings |

Repairs follow the degradation draft: the crafting kit gets a pick back to Worn in the field;
a smith makes it pristine, and the Forgemistress takes iron ore as part of the price.

### Light
- Surface seams: daylight is enough. At night, or anywhere underground, a **lit light in the
  off hand** is needed (degradation §6 lights).
- With no light, Sound is unavailable (you can't read the rock), and deep-mine levels refuse
  entry or turn you back.
- Light burns while you mine: a long session underground costs torch time.
- Until degradation lands, "has a light" is a plain item check on the lantern and torch ids.

## 6. Gems from mining

Mining uses the `gems.json` machinery (gem draft §2) with **tier in place of CR**:

| Face tier | 1 | 2 | 3 | 4 |
|---|---|---|---|---|
| `type_tilt` | 0.5 | 1.0 | 1.6 | 2.5 |
| `size_tilt` | 0.15 | 0.4 | 0.8 | 1.5 |

- A seam's `gems.types` replaces the global type weights with its own leaning (Glasswind
  favours obsidian and turquoise), so a region feels like a region. The tilt still applies.
- Mined gems are **uncut**, quality hidden until the jeweller cuts them (unchanged).
- **Geodes** (new item, from seams and deep mines): crack one open with a pick (a use-item
  action, 10 minutes, +2 wear) for one uncut gem rolled at tier 2. Gem draft §2 already
  wants them, and the parked `dwarven-geode-cache` encounter can drop them once it's back.
- **Dependency:** gem cells need gem slice 1 (the generator). Until it lands, gem cells are
  left out of the layout and geodes aren't seeded.

## 7. Ore and what it's for

### The roster
| Ore | Tier | Value | Weight | Found | Uses |
|---|---|---|---|---|---|
| Copper ore *(exists)* | 1 | 25 | 1 | surface | sell; dwarven-pick commission (the head is bronze-banded); lantern and tool repair at the general smiths |
| Iron ore *(exists)* | 1 | 40 | 1 | surface, deep 1–2 | sell; Forgemistress repair (1 per 25% wear); every commission |
| Deepiron ore *(new)* | 3 | 300 | 2 | deep 2–3, rare in Rimewall | sell; deepiron pick; the Forgemistress takes it for heavy-armor repair at a discount |
| Mithral ore *(new)* | 4 | 2,500 | 1 | deep 3 only | mithral armor commission (beta) |
| Adamantine ore *(new)* | 4 | 5,000 | 3 | deep 3 only | adamantine armor commission (beta) |
| Heartstone ore *(exists)* | – | – | 3 | its quest | quest item, unchanged |

Weight is the real limiter: 30 iron ore is 30 lb, so a mining trip is a hauling trip and
encumbrance (and the vault) decide how long you stay out.

### Smith commissions (NPC crafting)
Same model as the jeweller: you bring materials and a fee, the NPC makes the item. Each one is
a dialogue action with `requirements` + `consume_items` + a fee, like the vault rite, so no
new engine.

| Commission | Smith | Materials | Fee | Phase |
|---|---|---|---|---|
| Dwarven pick | Forgemistress (the Anvilheart, Ironpeak) | 10 iron ore + 5 copper ore | 400 | alpha |
| Deepiron pick | Forgemistress | 8 deepiron ore + the dwarven pick | 2,000 | alpha |
| Metal weapon or armor from the shop list | Forgemistress, Grand Forge (Kingdom) | iron ore by weight (1 per lb) | 70% of shop price | beta |
| Mithral armor (5e: no Stealth disadvantage, no STR requirement) | Forgemistress (Ironpeak) | mithral ore by the armor's weight + the base armor | 5e-scaled | beta |
| Adamantine armor (5e: crits against you become normal hits) | Forgemistress | adamantine ore by weight + the base armor | 5e-scaled | beta |

- The deepiron pick consumes the dwarven pick (an upgrade, not a second pick).
- Mithral and adamantine armor are 5e magic items, which is why they wait for beta with the
  rest of the magic items.
- Commission time: alpha hands the item over on the spot. A "come back in 2 days" version
  can reuse the quest wait-stage clock later.

### Quests
- Fetch objectives already count items you obtain, so "bring 10 iron ore" works today.
- Natural hooks: an Ironpeak supply order (repeatable daily, the innkeeper roll), a missing
  miner in the Flooded Mine, the Forgemistress's first commission as a tutorial for the
  dwarven pick, a jeweller asking for a specific uncut gem type.
- A new `mine` event (one per face worked, with the tier) would let quests ask for "work 3
  faces in the Deeps" without counting items.

## 8. Economy targets (first pass, tune in play)

| Tier | Where | Avg per Veins face (gold) | Quick roll | 3 faces + walk (game time) |
|---|---|---|---|---|
| 1 | surface | ~150 | ~90 | ~2 h |
| 2 | surface, deep 1 | ~350 | ~200 | ~2.5 h |
| 3 | deep 2 | ~900 + rare deepiron | – | a dungeon trip |
| 4 | deep 3 | ~2,500 + mithral and adamantine chances | – | a dungeon trip |

Gems sit on top: rare and swingy, exactly as the gem draft intends. A day at a tier-1 seam
earns a few hundred gold: worth doing, not better than adventuring. Deep mines pay like
dungeons because they are dungeons.

## 9. Shared with fishing

Fishing (`fishing-system.md`) has its own engine package and `GameSession.FishSession`, so
mining gets `cmd/server/game/mining/` and `GameSession.MineSession` the same way. What the two
share:
- **The versioned session protocol** (`begin`/`act`, stale version, exactly-once
  consequences), copied from locks. Mining is the third copy, so it's the point to lift the
  version/stale/abandon plumbing into a small shared helper that locks, fishing and mining
  all use.
- **Per-day state in session memory.** The two differ on purpose: a fishing spot goes soft
  (bites halve after `daily_catch`), while a seam has a hard face count. Both live in one
  memory-only map keyed by POI node and reset at the day rollover.
- **The node pattern:** a POI step type next to `lock`, with a minigame choice and a quick
  choice, and the shared quick-default setting.
- **The countdown bar** for tool wear (degradation).

## 10. Frontend

- `src/ui/mining.js` + `src/styles/mining.css`, built on the lockpicking window (title bar,
  message panel, status strip and action row are shared).
- The face is a grid of pixel-art tiles: unknown rock, sounded (the ring number painted on),
  hollow, broken, ore by type, glint, hard-cracked.
- Tool toggle: **Tap** (sound) and **Swing** (strike), so a click on a tile is one move;
  Pry appears on a glint. Works the same with mouse and touch.
- Status strip: swings left, soundings left, cracks as a three-pip meter, pick tier and wear.
- Each break flies its item into a small tray that mirrors what went into the pack.
- **Art needed:** ~8 rock tiles, a crack overlay, ore chunks for the three new ores, two pick
  sprites, a geode sprite.

## 11. Still open

1. **Numbers:** the tier table, swings and soundings formulas, cave-in damage, the economy
   targets. All are data; settle them in play.
2. **Copper's second use:** dwarven pick + light repairs, or something better?
3. **Deep-mine roster:** grow the Flooded Mine, plus Rimehollow and the Glass Pits, or
   different places?
4. **Mithral and adamantine pricing** and which armors they apply to.
5. **Party mining:** each member works their own face from a shared daily count, or one face
   per party? (The party-quests work is next on the list anyway.)
6. **Discovery XP only?** No XP per face is the proposal (it keeps mining from becoming the
   best way to level).

## 12. Build slices

1. **Seam core:** the `mine` node type, `mining.json`, the daily store, the quick roll, time
   and fatigue. Ironvein moves onto it. *This alone fixes the free endless seam.*
2. **Veins:** the server session (layout, sound/strike/pry, exactly-once grants) with seeded
   tests next to the locks tests, then the overlay.
3. **Pick tiers + light:** the `mining` block on picks, hard rock, tier-gated ore, no-light
   rules.
4. **Content:** the five new surface seams, the three new ores, geodes (once gems land).
5. **Commissions:** the dwarven and deepiron picks at the Forgemistress.
6. **Deep mines:** the Flooded Mine levels, then the other two.
7. **With degradation:** pick wear and repair; lights burning while you mine.
8. **Beta:** gear commissions, mithral and adamantine armor.

Each slice that players notice gets an entry in `www/res/updates.json`.
