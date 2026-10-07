# World Overview, Lore & Map Description

Status: draft. Lore is a proposal built from hooks already in game data — nothing here is
binding until it lands in JSON. The realm has no name yet ("The Royal Kingdom" is a working
name); see §6.

Sources: `game-data/locations/{cities,environments,poi-draft}/`, `game-data/npcs/`,
`game-data/quests-drafts/`, `game-data/systems/encounters-draft/`, `docs/draft/teleportation-design.md`.

---

## 1. How the world is connected

The capital (the Royal Kingdom) is the hub: each of its four outer districts exits onto a
different environment. Everything else is one great loop plus three spurs.

```
                 Frosthold ════ Frozen Wastes (24h) ════ Ironpeak
                     ║                                       ║
             Greywind Plains (10h)               Cragspire Mountains (20h)
                     ║                                       ║
 Millhaven ═ Windswept Hills (8h) ═ KINGDOM ═ Merchant's Highway (8h) ═ Goldenhaven ═ Sunscorch Desert (16h) ═ Dusthaven
                                       ║
                              Darkwood Forest (20h)
                                       ║
       Marshlight ═ Shadowmere Wetlands (11h) ═ Verdant
                                                   ║
                                     Suncrest Coastlands (8h)
                                                   ║
                                               Saltwind
```

| Route | Connects | Travel time | Difficulty | Terrain |
|---|---|---|---|---|
| Greywind Plains | Kingdom (N) ↔ Frosthold (S) | 600 min / 10h | easy | grassland |
| Frozen Wastes | Frosthold (E) ↔ Ironpeak (W) | 1440 min / 24h | very hard | arctic |
| Cragspire Mountains | Ironpeak (S) ↔ Goldenhaven (N) | 1200 min / 20h | very hard | mountain |
| Merchant's Highway | Kingdom (E) ↔ Goldenhaven (W) | 480 min / 8h | easy | road |
| Sunscorch Desert | Goldenhaven (E) ↔ Dusthaven (W) | 960 min / 16h | very hard | desert |
| Windswept Hills | Kingdom (W) ↔ Millhaven (E) | 480 min / 8h | easy | hills |
| Darkwood Forest | Kingdom (S) ↔ Verdant (N) | 1200 min / 20h | moderate | forest |
| Shadowmere Wetlands | Verdant (W) ↔ Marshlight (E) | 672 min / ~11h | hard | swamp |
| Suncrest Coastlands | Verdant (S) ↔ Saltwind (N) | 480 min / 8h | moderate | coast |

**Shape of play**
- **The Northern Loop** (Kingdom → Frosthold → Ironpeak → Goldenhaven → Kingdom, ~62h) is the
  only cycle. Its near side (Greywind, Highway) is easy; its far side (Wastes, Cragspire) is the
  hardest travel in the game.
- **Three dead-end spurs:** Millhaven (west, gentlest trip), Dusthaven (far east, edge of the
  world), and the **southern cluster** where Verdant acts as a second hub for Marshlight and Saltwind.
- **Waystones** (per teleportation design) exist only in the six starting cities.

## 2. Settlements

| Settlement | Size | Founding people | Vault | Role |
|---|---|---|---|---|
| The Royal Kingdom | capital | Humans, Half-Elves, Half-Orcs, Tieflings | Vault of Crowns | Hub, crown seat, Mage Tower, Grand Forge |
| Verdant | city | Elves | Glade of Safekeeping | Learning, nature magic, southern hub |
| Goldenhaven | city | Dragonborn | Ember Vault | Trade capital, river port, Merchant Princes |
| Ironpeak | town | Dwarves | Stonevault | Mining, ore |
| Millhaven | village | Halflings, Gnomes | Burrowlock | Farming, windmills |
| Marshlight | village | Orcs | Warhoard | Stilt village of the outcast clans |
| Frosthold | town | — (frontier) | — | Northern bastion, the Great Hearth |
| Dusthaven | village | — (frontier) | — | Oasis, last stop before the deep desert |
| Saltwind | village | — (frontier) | — | Cliffside fishing village, the only true seaport |

**Who is welcome where** (story only, not a mechanic): Orcs are met with suspicion nearly
everywhere except Marshlight; Tieflings get cold looks in Verdant, Frosthold and Millhaven but
are at home in Marshlight; Elves are least welcome in Ironpeak (an old elf–dwarf grudge);
Goldenhaven is grasping toward everyone but its own Dragonborn.

## 3. Points of interest

| Environment | POIs (position along route) |
|---|---|
| Merchant's Highway | Abandoned Watchtower (0.2, dungeon) · Wayward Wagon (0.4) · Forgotten Battlefield (0.7) · Crypt of the Ancients (0.85, dungeon) |
| Windswept Hills | Sentinel's Perch (0.5) · Buried Strongbox (0.6) · Shrine of the Four Winds (0.7) |
| Cragspire Mountains | Ironvein Seam (0.45) · Desperate Outcast (0.6) · Flooded Mine (0.65, dungeon) |
| Darkwood Forest | Old Hunter's Shack (0.2) |
| Shadowmere Wetlands | Shadowmere Hermit (0.25) · Still Waters (0.35) · Whispering Caverns (0.35, dungeon) · Raven Shrine (0.5) · Sunken Temple (0.8, dungeon) · Mournful Catacombs (0.9, dungeon) |
| Goldenhaven (east district) | Shadow Alley Den |
| **Empty** | Greywind Plains, Frozen Wastes, Sunscorch Desert, Suncrest Coastlands |

Content is lopsided: Shadowmere holds a third of all POIs; the whole far side of the
Northern Loop has only the Cragspire three. The empty routes are the obvious M7 targets.

## 4. Lore

### The Sixfold Compact
Long ago six peoples — the crown's folk, the elves, the dwarves, the small folk of the hills,
the dragonborn, and (last and grudgingly) the orcs — sealed a pact of peace. Each kept its wealth
in its own vault under its own law: the Vault of Crowns, the Glade of Safekeeping, the
Stonevault, the Burrowlock, the Ember Vault and the Warhoard.

The elves sang the **waystones** into being to bind the six seats together — the *Ritual of
Eternal Song* (`elven-memory` quest) is the founding song itself. The stones are fading now,
which is why every adventurer must re-attune them one by one (the level 5–15 waystone chain).

### The war before the Compact
The Forgotten Battlefield and the Crypt of the Ancients lie on the Merchant's Highway, between
the Kingdom and Goldenhaven. That road was once a front line between the Crown and the dragonborn
merchant princes. Peace paved it into a trade road, but Goldenhaven still drives the hardest
bargains in the realm with every people but its own — the princes never forgot who won the war
and who won the peace.

### The river and the river-princes
Goldenhaven stands where a river spills down out of the Cragspire Mountains. Ore from Ironpeak
and the mountain mines rides the river on barges to the Eastern Port, where the Merchant Princes
tax it, trade it, and send it west along the Highway. Whoever holds the river mouth holds the
wealth of the mountains — the root of Goldenhaven's power and of Ironpeak's resentment.

### The outcasts
The orcs signed the Compact last and were given the worst land — the black water of Shadowmere.
Marshlight became a refuge for anyone the other cities squeezed out, which is why Tieflings and
Half-Orcs are welcomed there as kin. An orc without a clan has nowhere at all: the Desperate Outcast in the
Cragspire is what that looks like.

### The rising shadow
Oracle Seraphina of Goldenhaven sees *a dark cloud moving from the mountains*. The elves feel
their waystones being drained from the direction of Shadowmere. The Sunken Temple is slowly
sinking into the mire. These are one thing: something beneath the swamp feeds on the waystone
song, and as the network weakens it breaks through wherever the old wards were thinnest — the
abandoned Flooded Mine, the kobold warrens, the bandit-cultists in the Highway watchtower.
The main quest walks that path: Goldenhaven → Cragspire → Shadowmere.

### The frontier towns
- **Frosthold's Great Hearth never goes out** because it is a Compact flame; if it ever went
  cold, the north would fall outside the pact.
- **Dusthaven's sacred well** and **Saltwind's Sea Mother** are older, pre-Compact faiths. The
  waystones never reached those towns, and their people like it that way.
- **The Raven Queen's shrine** in Shadowmere is older than all of it — the dead keep their own counsel.

### A Nostr note (optional flavor)
Every soul in the realm is born with a **true sigil** that cannot be forged. It is why a
character is fixed by their key, why the waystones know you, and why a written save is a vow
signed in your own hand.

## 5. Map description

Use this as the brief for any map — the in-app SVG draft, a PixelLab piece, or a hand-drawn one.

**Orientation & extent.** North up. Landscape rectangle (about 16:9); the southern sea runs
along the bottom edge.

**Labels.** Don't bake names into the art. Generate it text-free and overlay labels in the
app, so names (including the realm's) can change without redrawing.

**Layout (relative positions)**
- **Centre:** the Royal Kingdom, the largest marker, at the crossing of four roads.
- **North:** Frosthold, straight north across the open Greywind Plains.
- **North-east:** Ironpeak, reached from Frosthold by a long eastward crossing of the Frozen
  Wastes. The snowfield spans the whole northern band between them.
- **East:** Goldenhaven, directly east of the Kingdom along the straight Merchant's Highway, and
  directly south of Ironpeak through the Cragspire Mountains. The mountains fill the space between
  Ironpeak and Goldenhaven.
- **The river:** rises in the Cragspire, runs south down the mountains, and leaves the foothills
  on Goldenhaven's east side — the Eastern Port sits there. Below Goldenhaven it bends
  south-west across the lowlands, past the edge of the Darkwood, through Verdant, and on south to
  empty into the sea near Saltwind.
- **Frosthold** sits well up inside the Frozen Wastes, not on the edge of the plains — the
  Greywind Plains are a long climb north before the snow begins.
- **Far east:** Dusthaven, south-east of Goldenhaven across the Sunscorch Desert; the desert
  covers the whole eastern edge of the map.
- **West:** Millhaven, across the rolling Windswept Hills.
- **South:** Darkwood Forest directly south of the Kingdom, with Verdant on its southern edge.
  West of Verdant, the Shadowmere Wetlands with Marshlight at the far western side. South of
  Verdant, the Suncrest Coastlands run down to Saltwind on the sea cliffs.

**Terrain palette**
| Region | Look |
|---|---|
| Frozen Wastes | white snowfield, ice-blue shadow |
| Greywind Plains | pale gold grassland, lone standing stones |
| Cragspire | grey-brown peaks with snow caps, river source |
| Windswept Hills | soft green rolling hills, wildflowers |
| Darkwood | dense dark-green canopy |
| Verdant | bright terraced gardens fading into forest |
| Shadowmere | murky grey-green marsh, willows, faint glowing fungus |
| Sunscorch | golden dunes, a few ruins |
| Suncrest / sea | sandy cliffs, blue sea along the south |

**Markers & routes**
- Starting cities (with waystones): red markers. Frontier towns: brown markers.
- Route style by difficulty: easy = solid road, moderate = dashed, hard/very hard = dotted.
- Label each route with its name and travel time (e.g. *Merchant's Highway · 8h*).
- POIs as small diamonds at their `position` along the route (0 = first `connects` entry); dark
  diamonds for dungeons.

**Art prompt (pixel-art version)**
> Top-down fantasy world map, 16-bit pixel art, landscape 16:9, parchment border, no text or
> labels anywhere. A central walled capital where four roads meet. North: a long road across
> golden grassland climbing into a vast white snowfield; deep inside the snow, a stone town around
> a great bonfire. The snowfield stretches east to a dwarven mining town in grey snow-capped
> peaks. A river rises in those mountains and runs south past the east side of a golden trade
> city at the mountains' foot, then bends south-west through the lowlands to an elven garden city,
> and on south to the sea. A straight paved highway links the capital east to the trade city.
> Beyond the trade city, golden desert dunes with a small oasis village on the far east edge.
> West: green rolling hills with a windmill village. South: dark dense forest between the capital
> and the elven city; west of the elven city a murky swamp with a stilt village; south of it
> sandy cliffs, a fishing village, and blue sea along the bottom edge.

### The world map asset

`www/res/img/map/world-map.png` — 1536×1024 (3:2), text-free, generated from the brief above.
Labels are meant to be overlaid in the UI. Approximate settlement centres, as fractions of
width/height (eyeballed — fine-tune when wiring the overlay):

| Settlement | x | y |
|---|---|---|
| Frosthold | 0.40 | 0.11 |
| Ironpeak | 0.83 | 0.12 |
| Royal Kingdom | 0.41 | 0.38 |
| Goldenhaven | 0.69 | 0.39 |
| Dusthaven | 0.92 | 0.44 |
| Millhaven | 0.14 | 0.35 |
| Verdant | 0.55 | 0.64 |
| Marshlight | 0.18 | 0.64 |
| Saltwind | 0.47 | 0.87 |

## 6. World map feature (idea, not built)

The map image doubles as an in-game object.

- **Where it appears:** the destination chooser for Homeward Stones / waystone hops
  (teleportation design), and when a player uses a **World Map** item.
- **Recharting quest:** a short quest from a cartographer NPC — the old charts are lost or out of
  date, so the player gathers **map pieces** from around the realm (natural fits: one per region,
  dropped in POIs or given by NPCs), returns them, and the region is "recharted". Reward: the
  World Map item.
- **Replacement:** after the quest, the same NPC sells the map for **100 gold** (lost on death,
  dropped, etc.). Same pattern as Homeward Stone rebuys: unlock derives from `QuestsCompleted`,
  no new save field.
- **Open questions:** which city the cartographer lives in (Kingdom's Mage Tower or Goldenhaven's
  Merchant Princes Guild both fit); how many pieces; whether the map before completion shows
  partial/fogged regions (only the pieces you hold) or nothing; whether teleport destination UI
  requires owning the map or always shows it.

## 7. Open threads

- **Realm name.** Candidates: *Sixfold* / *the Sixfold Realm*, *Concord*, *Veyl*, *Aldermark*,
  *Sigilmark* / *the Sigiled Lands*.
- **Goldenhaven Temple** (Oracle Seraphina's seat) — referenced by the main quest, not yet built.
- **River** — exists in lore and map only; no game data models it yet (possible future
  ferryman / barge travel event, see teleportation design §Ferrymen).
- **Empty routes** — Greywind, Frozen Wastes, Sunscorch, Suncrest have no POIs.
