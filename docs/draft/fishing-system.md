# Fish and fishing — draft

Status: **DRAFT, decided 2026-10-22, nothing built.** Fishing is in M5.5 scope ("gathering
nodes (mine/fish)" plus the minigames). Mining gets its own draft on the same session pattern.
**Build order:** after degradation, the food workshop and the potions brainstorm, since fish
plug into all three.

### Decided (2026-10-22)
- **The draft's recommendations stand** (§12): a turn-based fight, the cosmetic exact weight,
  the ~22 species, no fishing XP, monster bites in, the net not used up, normal fatigue, and
  Finn as the only trophy mounter.
- **No hand line.** Instead, **barehand fishing** is unlocked by a short side quest,
  *Barbarian Fishing* (an Old School RuneScape callback). Afterwards you can fish barehanded
  anywhere, under a steep cap. Two species can only be caught that way (§5).
- **No legendary fish.** Huge is special enough. Trophies stay.
- **Food and potions are left open.** Cooking, salting, hunger and the magic-fish
  ingredients wait for the food workshop and the potions brainstorm. This draft only supplies
  the raw fish.
- **Driftwood is out.** The find is a dry branch that is simply an unlit `torch`.
- **Completion and highscores live server-side, in a separate system.** Fishing only reports
  catches to the event recorder (§8).

What exists today:
- `fishing-rod` (100 gold): gates the one fishing POI.
- `tackle-box` (300 gold): a 6-slot container for `fishing` and `fish` items.
- `net` (100 gold): a combat throwable that restrains, used up on the throw.
- `raw-fish` (15 gold, stack 20) and `large-fish` (60 gold, stack 10).
- `still-waters-fishing` (Shadowmere): a rod-gated `check` (Survival DC 9) followed by a tiered `loot` node.
- Finn Brackwater's Net & Tackle in Saltwind. The rod is also sold in Marshlight and Verdant.
- Six aquatic monsters (giant crab, kuo-toa, merfolk, merrow, sahuagin, bullywug), some of which drop `raw-fish`.

Related: [gem-system.md](gem-system.md) (size bands and generated variant items, reused here),
[degradation-system.md](degradation-system.md) (fish spoil and stack 1; cooking and salting),
[../development/locks.md](../development/locks.md) (the Tumblers session engine the minigame
copies), and the food workshop (sets hunger values; still to come).

---

## 1. The shape in one paragraph

You find water: a town pier, a POI, a hole cut in the ice. You pick a bait, cast, and
**watch the float**. Strike when it dives. Then you **fight the fish** in short turns,
managing line tension against distance while the fish runs, dives, thrashes and tires.
Land it and you see what you caught: a **species and a size**, unique the way gems are
(a *large river pike, 19 lb 6 oz*). Then you sell it fresh, cut it into bait, hand it in,
or, if it's huge, have Finn mount it as a trophy (cooking waits for the food workshop). Once
you've learned it, you can also wade in and grab fish barehanded. It all runs on the server;
the client only animates.

## 2. What a fish is: species × size

| Axis | Values | Drives |
|---|---|---|
| **Species** | ~22, by water type (§3) | base value, power, fight behaviour, bait it likes, when it bites |
| **Size** | tiny · small · medium · large · huge, from weight | value ×, power +, stamina, carry weight, portions, sprite scale |

Fish are **not** stack-sized trade goods: degradation already makes every fish stack 1 with
its own freshness. That fits a catch being *this* fish.

### Size from weight (the gem idea, adapted)
Every species has a `max_lb`. A size band is a slice of it:

| Size | Weight (of max) | Base share | Value × | Power | Stamina |
|---|---|---|---|---|---|
| tiny | < 10% | 22% | 0.2 | −2 | 2 |
| small | 10–25% | 38% | 0.5 | −1 | 3 |
| medium | 25–50% | 28% | 1 | ±0 | 5 |
| large | 50–80% | 10% | 2.5 | +1 | 8 |
| huge | 80–100% | 2% | 8 | +3 | 12 |

What a fish is worth as food (hunger, portions, cooking) is the food workshop's call.

- **Shares are what bites, not what you land.** Big fish fight harder (§5), so what you
  land skews smaller, and the better you get, the less it does. Bait tilts the shares
  (§4), with the gem formula: `weight = share × tilt ^ step` (tiny −2 … huge +2).
- **Carry weight is the band's midpoint**, so encumbrance stays a property of the item.
  A huge cod weighs about 54 lb. A huge fish caught far from town is a real decision: it
  spoils in 12 hours (degradation), it's heavy, and it's worth 8× a medium one.
- **The exact weight is a slot field** (`lb`, written when you land the fish), shown in
  the tooltip and on the catch card. It's cosmetic: price and weight come from the band. The
  slot already carries fish state (`t`, freshness), so this costs one short key in the save.
  Hydration: it can't be derived, so it may be stored. It also gives highscores a number to
  rank (§8).

### Option A again: generated items
As with gems, `game-data/systems/fishing.json` is expanded into items at migration:
`fish-{species}-{size}`, so 24 × 5 = **120 ids**, each with its value, weight, rarity-from-value
and tags (`fish`, `fish-pike`, `size-large`, `water-river`). Nothing is hand-authored. Quests
and shops address fish by tag ("any trout", "a large or huge fish"), which needs gem slice 3
(tag requirements) anyway.

**Retiring the placeholders:** `raw-fish` → `fish-mackerel-small` and `large-fish` →
`fish-cod-large`, both in monster loot and in Finn's stock. Old saves get a one-line id alias
at load, or wait for the next schema wipe.

## 3. Species and waters

Five water types, 22 rod species plus 2 barehand-only ones (§5). Every spot (§6) has one
water type and can re-weight its species.

| Species | Water | Bites | Max lb | Base (medium) | Power | Fights like | Likes |
|---|---|---|---|---|---|---|---|
| minnow | river, lake | any | 0.2 | 1 | 0 | rester | anything (the bait fish) |
| sunfish | lake | day | 1.5 | 10 | 1 | balanced | worms |
| river perch | river, lake | day | 3 | 20 | 1 | balanced | worms |
| brook trout | river (streams) | dawn/dusk | 6 | 40 | 2 | jumper | fly, worms |
| grayling | river (mountain) | day | 4 | 45 | 2 | runner, **shy** | fly |
| river pike | river, lake | dawn/dusk | 30 | 60 | 4 | runner | spoon, cut bait |
| carp | lake, swamp | day | 40 | 25 | 3 | diver | worms |
| sturgeon | river (Goldenhaven only) | night | 120 | 150 | 6 | diver | cut bait, worms |
| mudcat | swamp, river | night | 60 | 35 | 4 | diver | stink bait, cut bait |
| bog eel | swamp | night | 15 | 45 | 3 | twister | cut bait, stink bait |
| marsh gar | swamp | day | 50 | 40 | 4 | runner, jumper | spoon, cut bait |
| quipper | swamp, river | any | 0.5 | 5 | 1 | thrasher | anything (steals bait) |
| glowfin | swamp (Shadowmere) | night | 2 | 120 | 2 | **shy** | glow lure |
| mackerel | sea | day | 4 | 20 | 2 | runner | spoon, cut bait |
| flounder | sea (shallows) | day | 10 | 35 | 2 | rester | worms |
| sea bass | sea | dawn/dusk | 20 | 55 | 3 | balanced | cut bait, spoon |
| cod | sea (deep) | any | 60 | 45 | 3 | diver | cut bait, worms |
| red snapper | sea (cliffs) | day | 35 | 70 | 4 | diver | cut bait |
| goldscale | sea (Suncrest) | dawn | 100 | 200 | 6 | jumper | spoon |
| ice cod | ice | any | 8 | 30 | 2 | diver | cut bait |
| arctic char | ice | any | 20 | 60 | 3 | balanced | cut bait, spoon |
| frostfin | ice (deep holes) | night | 5 | 150 | 3 | **shy** | glow lure |
| **leaping salmon** | river (falls, rapids) | dawn/dusk | 30 | 80 | — | barehand only | — |
| **rock grouper** | sea (shallows, under rocks) | any | 40 | 90 | — | barehand only | — |

- **Bites:** day 06:00–20:00, night 20:00–06:00 (`sunlight_hours`). Dawn/dusk is an hour either
  side of each edge, the golden hour: every species bites ×1.5 then, and dawn/dusk species
  only then (×0.3 otherwise).
- **Fights like** names a behaviour profile (§5).
- **Shy** fish never take the float under. You have to strike on the twitch (§5).
- **Glowfin and frostfin** are the magic fish: worth the most and night-only. Whether they
  become potion ingredients is for the potions brainstorm. Here they're only fish.
- **Quippers** are the nuisance: they bite fast, steal bait and are worth almost nothing.
  On a quipper-heavy spot, cheap bait is the smart choice.
- Values are first-pass and tunable. A medium fish sells for about a ration's worth before
  the shop's cut, so fishing feeds you and pays a little; trophies and the magic fish pay real
  money.

### Not fish: shellfish and finds
Tide pools and net hauls also turn up these (simple items, no sizes):

| Catch | Where | Use |
|---|---|---|
| shore crab | sea shallows | sells; food use is the food workshop's call |
| mussels | sea shallows | cut bait; food use is the food workshop's call |
| oyster | sea shallows | **open it:** 4% a tiny uncut pearl, 0.5% a small one (gem system ids) |
| dry branch | river, lake, swamp | it *is* an unlit `torch` (the find just says how you got it) |
| old boot | any | junk; sells for 1 gold; Finn has a story about it |
| message in a bottle | sea, rare | a lore note, sometimes revealing a POI on the map (discovery) |
| sunken purse | any, rare | 5–50 gold |

## 4. Gear and bait

### Rods (the line)
Rods get a `fishing` block instead of a `skill_bonus`, so they don't leak into every Survival
check:

| Item | Price | `line` (snaps at) | `cast` (start distance) | `tell` | Notes |
|---|---|---|---|---|---|
| **fishing rod** | 100 | 10 | 6 | +0 | unchanged price |
| **Brackwater rod** (new) | 1,500 | 13 | 7 | +10% | Finn's own make; reward for his quest line (§9) and later on sale from him |

A huge goldscale or sturgeon (power 9) will usually snap a plain rod's line. That's
deliberate: gear sets the ceiling and the fight decides the rest.

### Bait (used up) and lures (kept)
| Bait | Kind | Source | Tilt | Good for |
|---|---|---|---|---|
| none (bare hook) | — | — | 0.6 | bites ×0.4; whatever's hungry |
| worms | bait | buy (2 gold) or **Dig for worms** at river/swamp/lake spots (Survival DC 8, 10 min) | 1.0 | freshwater generalist |
| cut bait | bait | **Cut into bait:** any tiny or small fish → 2 | 1.3 | predators, sea fish |
| stink bait | bait | **spoiled food** (once degradation adds it) | 1.1 | mudcat, eels |
| fly | lure | Finn, Verdant (30 gold) | 1.0 | trout, grayling |
| spoon | lure | Finn (40 gold) | 1.2 | pike, gar, mackerel, goldscale |
| glow lure | lure | Marshlight (150 gold) | 1.0 | glowfin, frostfin; night bites ×1.3 |

- **Bait** is used up when a fish takes it, steals it, or comes off the hook. **Lures**
  are lost only when the line snaps.
- **Liking:** a species that likes your bait bites ×2.5. One that doesn't still can, at
  ×0.5. Any bait beats a bare hook.
- Bait carries the `fishing` tag, so it lives in the tackle box. Cut bait spoils like fish.
- This gives scraps a job: tiny fish become bait and spoiled food becomes stink bait. The
  food workshop may add more baits (bread dough for carp, say).

### The net (its second use)
At a spot with `shallows`, **Haul the net** (30 minutes, one Survival check vs the spot DC):
- **Success:** 1–4 tiny or small fish from the shallow species, plus a shellfish roll.
- **Failure:** nothing.
- **No minigame, and nothing medium or bigger.**

Fishing with it doesn't use it up. You're dragging it, not throwing it at a crab. In combat it
stays a consumable throwable, as it is now.

### Spear and trident (later)
Finn already sells both. **Spearfishing** fits as a second style on the same session engine
(a strike-timing fight in clear shallows, using the weapon's attack bonus). It's parked, like
the Sweet Spot and Wards lock styles.

## 5. The minigame: Cast & Reel

**Server-authoritative and turn-based, built like Tumblers.** A real-time reflex game can't
be judged fairly by the server over the network, and anything decided by the client is
trivially cheatable. So every decision is a discrete move. The client makes it feel alive
(bobbing float, bending rod, splashes, a tension bar that shakes), but the server rolls
everything.

This replaces the roadmap's "skin over a check" idea. Tumblers proved that a real session
minigame is about the same amount of work and much better.

### Session protocol
Everything goes through `POST /api/game/action`, under the session lock, versioned like
`lock_act`:

- **`fish_begin {spot, bait, barehand?}`**: you're at the spot and carry a rod and the bait
  (or none), or you fish barehanded (below). Stored as `GameSession.FishSession` (memory
  only).
- **`fish_act {session_id, version, act}`**: `act` is one of `wait`, `strike`, `reel`, `hold`,
  `give`, `recast` or `quit`. A stale version gets `data.stale: true` and changes nothing.
- **`fish_quick {spot, bait}`**: the quick roll (below).
- **Any other game action** abandons the session for free.
- **The public view** never includes the hooked fish's species or size until it's landed.
  The fish is rolled at the bite and held server-side, the same way a gem's quality waits for
  the cut.

### Phase 1: watch the float
Each `wait` step takes **5 minutes** of game time. The server rolls for a bite:

```
bite chance per step = 25% × spot.bite_rate × bait liking × time-of-day
                       × depletion × (1 + 5% × Survival modifier)
```

The species is picked from the spot's weights × bait liking × whether it's biting now. The
size comes from the shares × the bait's tilt. Then the float plays a short sequence, one
state per step:

| Float | Strike | Wait |
|---|---|---|
| **still** | nothing there; you pull the bait out (recast costs 2 min) | keep waiting |
| **twitch** (nibble) | 25% you hook it; otherwise it spooks (30% the bait's gone) | next step: 60% it dives, 40% it drifts off |
| **dive** (bite) | **hooked** → the fight | 50% it holds one more step, else it drops the bait and leaves |

- **Shy** species never dive. They twitch twice and leave, so on a shy-fish spot you strike
  on the second twitch.
- With Survival +3 or better you sometimes get a hint ("a delicate, testing touch: a shy
  biter"), at 15% per point above +2.
- **Monster bites (§6)** happen here: the float goes under hard, and *striking* hands off to
  combat.

### Phase 2: the fight
State: **tension T** (snaps at the rod's `line`), **distance D** (starts at the rod's `cast`;
land the fish at 0; the line runs out at 12), and the fish's **stamina S**.

Each turn costs **1 minute**:
1. The server has already picked the fish's move from its behaviour profile.
2. The view shows the **tell** if the tell roll succeeded (`35% + 10% × Survival mod + rod
   tell`, at most 90%), plus the **pull feel**, which is always shown: light, firm, heavy or
   monstrous, from its power. That's a fair hint at the size, the way Inspect is for locks.
3. You pick **reel**, **hold** or **give line**.
4. It resolves: `T = max(0, T − 3 + pull + strain)`, `D = D + run + reel`. Stamina drops.

| Fish move (tell) | Pull | Distance | Special |
|---|---|---|---|
| **Run** ("the line hums and tightens") | power + 2 | +1 | reeling gains nothing |
| **Dive** ("the rod tip bows, steady") | power | 0 | reeling only gains 1 |
| **Thrash** ("the water boils") | power + 1 | 0 | if T ends ≤ 2, **the hook is thrown** |
| **Rest** ("the line goes heavy and still") | 0 | 0 | it gets 1 stamina back unless you reel or hold |

| Your move | Strain | Distance | Tires it |
|---|---|---|---|
| **Reel** | +2 | −2 | 1 |
| **Hold** | +1 | 0 | 2 |
| **Give line** | −3 | +1 | 0 |

- **Snap:** T reaches `line`. The fish is gone, along with the bait and any lure.
- **Thrown:** thrashing on a slack line, or two turns at T = 0. The fish is gone, and so is
  the bait; a lure is kept.
- **Run out:** D reaches 12, and it snaps.
- **Spent:** at S = 0 the fish's pull halves and it rests 70% of the time. Reel it in.
- **Landed:** D reaches 0. The catch card shows the species, size, exact weight and value.

**Behaviour profiles** are weights over the four moves:

| Profile | Run / Dive / Thrash / Rest |
|---|---|
| balanced | 3 / 2 / 2 / 3 |
| runner | 5 / 1 / 2 / 2 |
| diver | 1 / 5 / 1 / 3 |
| jumper | 2 / 1 / 5 / 2 |
| twister | 1 / 3 / 4 / 2 |
| thrasher | 1 / 1 / 6 / 2 |
| rester | 1 / 2 / 1 / 6 |

A move never repeats more than twice in a row, so a fish is readable, not random noise.

**A worked turn:**
- The setup: a medium pike (power 4) on a fishing rod (line 10), T = 2.
- It runs and you reel: T = 2 − 3 + 6 + 2 = 7.
- It runs again and you reel again: T = 12, and the line snaps.
- Give line instead on that second run: T = 7 − 3 + 6 − 3 = 7, and the distance grows by 2.
- Then it rests and you reel: T = 6, D − 2.

You read the tell, give against runs, and reel on rests. A typical medium fish takes 6–12
turns, and a huge one 15–25.

### Quick fish (the other way)
Like a lock's quick attempt: one Survival check against the spot's DC, costing the average
wait time (about 20 minutes).
- **Success:** a fish, species rolled normally, **size capped at medium** with tilt 0.5.
- **Failure:** nothing, and a 50% chance the bait is gone.

That's there for players who don't want the minigame. Trophies need the fight (or bare
hands, below).

### Barehand fishing (after *Barbarian Fishing*)
Once you've finished the side quest (§9), every spot offers **Fish barehanded**. You need no
rod and no bait. You wade in, watch the water and grab. The unlock is the quest's completion,
so nothing new goes in the save.

- **Watching:** the same `wait` steps as the float, but you read a **shadow** in the water:
  *still*, *circling* (the twitch), *close* (the dive). Shy species are too quick to grab.
- **The grab:** `strike` on *close* is an **Athletics** check (the barbarian way) against the
  spot's DC + 4. Each size step above small adds +2.
  - **Success:** it's yours.
  - **Failure:** it's gone.
  - **No fight phase:** your hands are the line.
- **The cap:** bites come at ×0.4, and any rod species you catch is **tiny or small only**.
  It's a way to eat when you've lost your rod, not a way to farm.
- **The exception: two species exist only for bare hands,** and they use their full size
  range:
  - **leaping salmon:** river falls and rapids at dawn and dusk. You catch it mid-leap.
  - **rock grouper:** sea shallows, wedged under rocks. Tide pools are its home.
- Both are worth more than their rod-caught neighbours. Since "caught every species" is a
  completion goal (§8), they make the quest worth doing.

### Time, the clock, survival
- **Time:** a POI walk holds the clock, so fishing advances it itself, like `PickMinutes`.
  A normal catch costs 20–45 game minutes.
- **Survival:** hunger and fatigue run normally. Fishing is not rest, so it doesn't freeze
  fatigue the way wait/rest does.
- **Night:** fishing outdoors at night with no darkvision and no light works, but tells are
  halved. Night fishing wants a lantern, which ties into degradation's lights.
- **Settings:** a "fishing animation speed" option alongside text speed.

## 6. Where: spots

A **spot** is defined once in `fishing.json` → `spots`, and used from a POI node or a town
district.

```json
"saltwind-harbor": {
  "name": "Saltwind Harbor",
  "water": "sea",
  "dc": 10,
  "bite_rate": 1.0,
  "shallows": false,
  "daily_catch": 12,
  "species": { "mackerel": 35, "cod": 25, "sea-bass": 20, "flounder": 10, "red-snapper": 5 },
  "finds": { "old-boot": 2, "torch": 4, "message-in-a-bottle": 0.3 },
  "monsters": [ { "monster": "giant-crab", "weight": 1, "when": "night" } ]
}
```

- **In a POI:** a new node type, `{ "type": "fishing", "spot": "still-waters", "next": "bank" }`,
  next to `lock`. The node's choices offer **Fish** (the minigame), **Quick fish**, **Fish
  barehanded** (after the quest), **Haul the net** (if the spot has shallows and you carry a
  net) and **Dig for worms** (river, lake and swamp spots). The tool gating works as now.
- **In a town:** a district gets `"fishing_spot": "saltwind-harbor"`, which shows a Fish action
  there (schedule-free; piers don't close).
- **Depletion:** after `daily_catch` fish landed at a spot today, bites halve; after twice that,
  it's fished out until the next in-game day. This is memory-only per-day state, like a
  picked-open lock.
- **Monster bites:** a hooked monster skips the fight and starts combat through the POI
  combat handoff. They're off while you're in a party (parties get no random encounters yet).

### First spots (content)
| Spot | Where | Water | Character |
|---|---|---|---|
| Saltwind Harbor | Saltwind (town) | sea | the beginner pier, Finn next door; cod, mackerel |
| Goldenhaven Wharf | Goldenhaven river port (town) | river | perch, pike, carp; **sturgeon at night** |
| Marshlight Platforms | Marshlight (town) | swamp | mudcat and eels at night, quippers by day |
| Still Waters (exists) | Shadowmere POI | swamp | moves to a `fishing` node; **glowfin** at night |
| Tide Pools (new) | Suncrest POI | sea, shallows | net hauls, oysters, flounder, crabs (giant ones too); **rock grouper** barehanded |
| Gull Rock (new) | Suncrest POI | sea | cliffs over deep water: snapper, **goldscale** at dawn, merrow at night |
| Cragspire Falls (new) | Cragspire POI | river | a mountain stream under a falls: trout, grayling, pike; **leaping salmon** barehanded; where *Barbarian Fishing* is taught |
| Darkwood Brook (new) | Darkwood POI | river | trout, perch, worms to dig |
| Millpond (new) | Windswept Hills POI, near Millhaven | lake | sunfish, carp; the gentle family spot |
| Ice Hole (new) | Frozen Wastes POI | ice | cutting the hole needs a pick (`pick-miners`, an Athletics check, once per day); char, ice cod, **frostfin** |

That covers every water region except the desert. Dusthaven's well fish are sacred to the
well-keepers, and fishing there is a lore line, not a spot.

## 7. What fish are for

- **Food:** for the food workshop. It decides hunger, cooking and salting. Until then, fish
  stay what they are today: edible raw.
- **Sell:** at value × size, less the shop's cut, less the freshness tier (degradation). Fish
  buyers: Finn, the Fisherman's Inn kitchen in Saltwind, the galley at Goldenhaven's tavern,
  and the Marshlight swamp trader. General stores buy anything, at their usual worse rate.
- **Bait:** tiny and small fish become cut bait.
- **Quests:** fetch objectives by tag ("3 fresh fish", "a large trout"). The event recorder
  gets `fish_caught` so a quest can say "catch" rather than "have". Fish orders are a natural
  **vault-keeper daily** ("the Fisherman's Inn needs 4 fish by sundown").
- **Trophies:** Finn mounts a **huge** fish for a fee (25% of its value) and turns it into
  `trophy-{species}`.
  - A trophy doesn't spoil, weighs a fixed 6 lb, and sells for 1.5× the fish.
  - That's the way to bank a lucky catch made far from town before it rots: bring it in
    fresh. The 12-hour clock is the challenge.
  - 24 generated ids.

## 8. Completion and highscores (server-side, a separate system)

The save holds what play needs. A catch log isn't needed for play, so it doesn't go in the
save: that would break the hydration rule's spirit and grow every save event. Completion and
highscores are **official-server records**: a new server-side store, with its own draft. The
event recorder already names "the badge/stat tracker" as a future consumer on the official
server, so that's where it plugs in.

**What fishing reports:** one `fish_caught` event per landed fish, carrying species, size,
`lb`, spot and barehand-or-rod. Today's `Event` is only `Target` + `Count`, so it needs an
optional detail field.

**What the store would keep from it:**

| Record | Kind | From |
|---|---|---|
| every species caught (24) | completion badge, "caught one of every fish" | the set of species |
| biggest of each species | highscore | max `lb` per species |
| fish landed | highscore | a count |

Trust: only catches the official server saw count, so an edited save can't post a record.
Modded servers don't feed the official board.

## 9. Quests

### Finn's line, *The One That Got Away*
Finn's description already says he's "nearly caught" every fish.

1. **Land your first fish:** he gives you worms and a story.
2. **A large fish from three different waters:** the **Brackwater rod**. Its real use is line 13,
   which is what a huge goldscale or sturgeon (power 9) needs.

### *Barbarian Fishing* (side quest, the barehand unlock)
An Old School RuneScape callback, kept short.

- **The giver:** a northern barbarian, working name **Ottar Bare-Hand**, camped at Cragspire
  Falls (a POI `npc_interaction`; home `ironpeak`). He laughs at your rod.
- **The steps:**
  1. Find Ottar at the falls.
  2. Put the rod down and grab a leaping salmon at the falls. Barehand fishing is open for
     this step at this spot only, and the cap applies.
  3. Hand him the salmon. He teaches you, and from then on you can fish barehanded anywhere.
- **The reward** is the skill itself, plus a little quest XP. Completion is the unlock, so
  there's no save field (hydration).
- **The hand-in consumes the salmon.** Your first salmon goes to Ottar.

## 10. Data and code

| Piece | Where |
|---|---|
| Species, sizes, profiles, baits, spots | `game-data/systems/fishing.json` (new) |
| Generated fish and trophy items | migration, from `fishing.json` (like the gem generator) |
| Rod, net and bait fields | `fishing` block on items: `{line, cast, tell}`, `{net: true}`, `{bait: "cut"}` |
| Engine | `cmd/server/game/fishing/`: spot resolution, bite/species/size rolls, fight session, the barehand grab, depletion; seeded-rng tests in the package |
| API | `cmd/server/api/game/fishing.go`: `fish_begin`, `fish_act`, `fish_quick`, `haul_net`, `dig_worms`, `cut_bait` |
| POI node | `POIStepFishing = "fishing"` in `types/poi.go`; walker hand-off like `lock` |
| Session | `GameSession.FishSession`, plus per-day spot depletion (memory only) |
| Save | only the `lb` slot field. No new `SaveFile` fields; the barehand unlock is a quest completion |
| Events | `events.FishCaught` (Target = species), plus a detail field for the stat store |
| Frontend | `src/ui/fishing.js` + `src/styles/fishing.css`: a win95 window like lockpicking (float or shadow, rod, tension and distance bars, tell line, catch card) |
| Validation | `--validate`: spot species, baits and monster ids exist; barehand-only species are never in a rod pool; profiles sum > 0. `--check-schema`: `fishing` nodes name a real spot |

## 11. Build slices

1. **Data + generator:** `fishing.json`, generated `fish-*` and `trophy-*` items, the
   validator. Retire `raw-fish` and `large-fish` (aliases, monster loot, Finn's stock).
2. **Engine:** bite, species and size rolls; the fight session; quick fish; the barehand grab;
   depletion. Tests first: a seeded fight, the snap and thrown rules, the size skew of landed
   vs bitten, the barehand cap.
3. **API + POI node:** the actions, the `fishing` node, town district spots, the event.
4. **The minigame window:** float, shadow, fight, catch card, settings speed.
5. **Gear and bait:** Brackwater rod, worms (buy and dig), cut bait, fly and spoon, glow lure,
   the net haul. Stink bait waits for degradation's spoiled food.
6. **Spots:** the ten in §6, with Still Waters migrated.
7. **Uses and quests:** fish buyers, Finn's mounting service, innkeeper fish dailies, *The One
   That Got Away*, *Barbarian Fishing* (Ottar + Cragspire Falls).
8. **Art:** 24 species sprites, scaled by size like gems; bait and lure sprites; one trophy
   plaque frame composited with the species sprite; minigame art; Ottar's portrait.

**Dependencies:**
- **Degradation goes in first,** for spoiling fish, stack 1, sell-by-freshness and spoiled
  food as stink bait.
- **The food workshop and the potions brainstorm** decide fish as food and as ingredients.
- **Tag requirements** (gem slice 3) are needed before tag quests.
- **Completion and highscores** wait for the server-side stat store (§8). Fishing only emits
  the event.

## 12. Decisions

Taken 2026-10-22:

1. **The fight is turn-based tension and distance,** server-authoritative.
2. **The exact weight (`lb`) is a cosmetic slot field.** It also feeds highscores.
3. **The species list stands** (22 rod species), plus the 2 barehand-only ones.
4. **No XP for fishing.** XP comes from quests and dailies.
5. **Monster bites are in,** rare and night-leaning, and off in a party.
6. **The net isn't used up when fishing.**
7. **Fatigue runs normally while fishing.**
8. **Finn is the only trophy mounter** for alpha.
9. **No hand line;** barehand fishing comes from *Barbarian Fishing* instead.
10. **No legendary fish.**
11. **Food and potion uses are left to their own sessions.**
12. **The dry branch is an unlit torch;** driftwood is dropped.
13. **Completion and highscores live server-side,** in their own system.

Still open:
- **Barehand numbers:** an Athletics grab at DC + 4 (+2 per size step), bites ×0.4, rod
  species capped at small, and the barehand-only species at their full range. OK?
- **Ottar:** name, and the falls as his home.
