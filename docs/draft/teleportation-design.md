# Teleportation — design + handoff

Status: design agreed with the maintainer; not built. Code references come from a read-only survey
(2026-09-29) and should be re-checked before editing.

## 1. Design (agreed)

### Homeward Stones
- One stone item **per starting city** (6 cities: kingdom, verdant, ironpeak, millhaven, marshlight,
  goldenhaven). The other three cities (dusthaven, frosthold, saltwind) are not starting cities and have
  no stone or waystone. Using a stone of city X teleports you to X.
- **Reusable**, not consumed. **Shared 1-week cooldown** across all stones (one use per cooldown).
- **Starting kit:** every character starts with the stone of their starting city, **free**. This is the
  only free stone. An NPC in that town carries the lore.
- **Every other stone costs 1000**, including a replacement of your origin stone.
- **Origin stone replacement:** sold only by the origin-town NPC, for 1000. The NPC refuses if the
  player already holds that stone in **inventory or vault**. Losing it (death, etc.) just means
  buying another.
- **Other starting cities' stones:** each has an **attunement quest** (given by an NPC in that city).
  Completing it unlocks the ability to buy that city's stone (1000, same hold-check refusal). Completing
  the quest gives no free stone. Losing it later just means rebuying, since the unlock stays.
- **Teleport guardrails:** not in combat; not inside a POI/encounter. **Does not advance the game clock.**
- **Death is unchanged** — returns to the starting city. No new save field.

### Waystone network
- Each **starting city** has a **waystone** (6 total; the non-starting cities have none). A Homeward
  Stone teleports you to that city's waystone.
- Waystones let you hop **waystone → waystone**, free of charge, on a **separate 1-week cooldown**.
  A player can therefore stone-hop to a city and then waystone-hop onward once per week. Then both
  cooldowns are running.
- **Nobody starts with waystone access.** Every player unlocks the network through one **fixed,
  ordered quest chain of 6 quests** at levels **5, 7, 9, 11, 13, 15**, one per waystone city.
  The chain is the same for everyone, whatever city they started in. The first quest (level 5) unlocks
  "hop to the kingdom from any waystone"; each later quest unlocks that quest's waystone as a
  destination. The order of cities in the chain is a story decision (see §5).
- **No attunement save state.** Unlocks are derived from `QuestsCompleted`.

### Two separate quest tracks
| Track | Purpose | Level range | Count per player |
|---|---|---|---|
| **Stone quests** (attunement) | Unlock the ability to *buy* another starting city's stone | ~3–8, simple, low requirement | 5 (all starting cities except your own) |
| **Waystone chain** | Unlock waystone hops, in a fixed order | 5, 7, 9, 11, 13, 15 | 6 (all six waystones) |

### Ferrymen
- Random **travel event**, a **shortcut**: pay gold, skip part of the route.
- Light downsides only: small extra cost, delay/detour, minor encounter. Any item loss is **capped
  below a low gold value** and light. Nothing that punishes heavily.

## 2. Save impact
None new. Everything derives from existing fields:
- Cooldowns → two `ActiveEffect`s (`homeward-cooldown`, `waystone-cooldown`, `timer: 10080` minutes).
- Unlocks → `QuestsCompleted`.
- Stones → ordinary inventory items.
Complies with the hydration rule in CLAUDE.md / roadmap §4.

## 3. Build plan (suggested order)

1. **Cooldown effects + item.** Add effect defs and 6 stone items. Add a `teleport` handling path.
2. **Teleport action.** A shared helper used by both stone and waystone hops. Validates state,
   moves the player, records discovery.
3. **Starting kit.** Grant the origin stone at character creation.
4. **Shop gating.** Requirement + "already holds" refusal; origin-town NPC and other-city NPCs sell stones.
5. **Waystones.** Model as a city feature; add the hop UI.
6. **Quest content.** Stone quests (one per starting city, levels ~3–8, simple) + the 6-quest
   waystone chain at levels 5, 7, 9, 11, 13, 15. Level gates use the `level` requirement type.
7. **Ferryman encounter.** Needs cost deduction and a relocation node.
8. **Tests** in `tests/` (cooldown logic, hold-check, unlock derivation, teleport guardrails).

## 4. Implementation notes from the code survey

### Items and effects
- Use path: `POST /api/game/action` `type:"use_item"` → `cmd/server/game/inventory/inventory.go:69`
  `HandleUseItemAction`. It **requires the `consumable` tag** and removes one from the stack
  (:83–152), so reusable items need a branch before that check, or a dedicated `teleport` action in
  `processActionSwitch` (`cmd/server/api/game/actions.go:398`). A dedicated action is cleaner: it must
  be added to `combatBlockedActions` (:182–190). It also needs the **session** (to reject an active
  POI), and `HandleUseItemAction` only receives `(state, params)`.
- Effects persist in `SaveFile.ActiveEffects` (`types/save.go:53`); timers are minutes.
  `effects.ApplyEffect` (`cmd/server/game/effects/effects.go:120`), `HasActiveEffect` (:604).
- **Gotcha:** an effect with zero modifiers appends nothing (:148). The cooldown effect needs a dummy
  `constant` modifier (e.g. `stat:"teleport_cooldown", value:0`).
- **Bug to fix first:** `TickEffects` (~effects.go:375) keeps an effect whose remaining time is exactly
  0, treating it as permanent. A 10080-minute cooldown can hit 0 exactly. Fix the condition, or check
  `DurationRemaining > 0` when testing the cooldown.
- After adding item/effect JSON, run `go run ./cmd/codex --migrate`.

### Teleport action
- Player location fields to set: `Location=<city>`, `District="center"`, `Building=""`, `Room=""`,
  `TravelProgress=0`, `TravelStopped=false` (`cmd/server/game/travel/travel.go`).
- Discovery, `events.Record(LocationDiscovered)` and music-unlock logic live in the unexported
  `processArrival` (:477) and need an `EnvironmentData`. **Extract the shared block** into a function
  the teleport can call.
- Refuse when `session.ActiveCombat != nil` or `session.ActivePOI != nil`. Combat is blocked via
  `combatBlockedActions`, but `ActivePOI` is not blocked server-side today.
- **No clock advance.** `processGameAction` (`actions.go:192–263`) derives elapsed minutes from
  time-of-day/day changes, so leaving the clock untouched avoids triggering travel rolls.
- Valid destinations are the 6 starting-city ids with a waystone, drawn from `LocationsDiscovered`. `LocationsDiscovered[0]` is the home
  city (used by death respawn), so **don't reorder it**.

### Death
- `ApplyDeath` (`cmd/server/api/game/combat.go:1238`) already returns to the starting city. No change.
- `stripInventoryForDeath` (:1417) keeps only the **top 3 units by cost** and drops the rest. A 1000-gold
  stone will usually survive, but not guaranteed. This matches the "buy another" rule, but confirm
  it's intended.

### Shop and NPC gating (net-new work)
- Shops have no per-item condition field (`ShopInventoryItem`, `types/types.go:95–108`). Add
  `requirements []POIRequirement`, evaluate with `requirement.Evaluate` + `buildQuestContext(save)`,
  and apply in both `handleGetShop` (hide) and `handleBuyFromShop` (refuse) in
  `cmd/server/api/game/shop.go`.
- The requirement evaluator (`cmd/server/game/requirement/requirement.go`) supports
  `quest_completed` and `item`, but has **no negation, no vault check, no "effect active"**. Add:
  "player holds item X in inventory or vault" (negated). The current `PlayerHasItem`
  (`gameutil/inventory.go:149`) checks general slots and backpack **only**, not vaults. Vault state
  is `state.Vaults`.
- NPC dialogue gating (`npc/helpers.go:30`) does not use the requirement package. It has no way to
  grant a specific item; a new action (e.g. `sell_item`) or the shop route above is needed. Shop
  route is probably simpler.

### Waystones
- Model as a **building type** in each of the 6 starting cities (`districts.<key>.buildings[]`) or a district feature.
  `building.GetBuildingType` already resolves types. `special_features` on buildings is free text and
  unused by the server.
- Existing flavor to reuse: encounter `ancient-elven-waystone.json`, POI `shrine-of-the-four-winds`.

### Quests
- Completion lands in `QuestsCompleted` (`cmd/server/game/quest/objective.go:79–100`). **Repeatable
  quests (daily/weekly) never enter it**, so unlock quests must be normal (non-repeatable) quests.
- Quests live in `game-data/quests-drafts/{main,side,class,race,daily,weekly}/`. Run
  `--check-schema` after editing.
- Rewards are paid per stage (`quest.GrantReward`).
- **No mage guild NPC exists.** `kingdom.json:64–72` has a `mage_tower` building open 480–1200 with no
  NPC. The chain's mage would be a new file `game-data/npcs/kingdom/<id>.json` with a schedule in
  `mage_tower`. NPC directory must equal `primary_home` and filename must equal `id`.

### Ferryman encounter
- Add a JSON in `game-data/systems/encounters-draft/` with `trigger:"travel"` and `valid_locations`
  of the chosen environments. Template: `suspicious-peddler.json`. Choice node: pay / decline.
- **Gold cost is not enforced.** `poi.Resolve` (`cmd/server/game/poi/walker.go:59`) never reads
  `node.Cost` (only validated in `cmd/codex/validation/schema.go:308`). Add deduction using
  `gameutil.DeductGold` / `GetGoldQuantity`. The `{"type":"item","id":"gold-piece"}` requirement
  checks presence, not quantity.
- **No relocation node exists.** Add a `travel_to` / progress-jump node in `Resolve`, or handle it in
  `stepPOI` (`cmd/server/api/game/poi.go:54`), the only place with both session and state. It must clear
  `ActivePOI`. A shortcut can be a `TravelProgress` jump rather than a full relocation.
- Scheduler: `cmd/server/api/game/encounter_scheduler.go` (`maybeFireEncounter`, cooldown, one-shot
  flags).

## 5. Open decisions
1. **Ferryman item-loss cap** — exact gold ceiling and how "light" the loss is.
2. **Ferryman environments** — where it can appear.
3. **Mid-route use** — may stones/waystones be used while travelling in an environment? Suggested: yes,
   and it cancels the trip.
4. **Death vs. stone** — is the top-3 death strip acceptable, or should stones be exempt?
5. ~~Waystone chain city order~~ — **decided:** Kingdom, Millhaven, Verdant, Goldenhaven, Marshlight,
   Ironpeak (levels 5–15). To be revisited later; the free stone always takes you home regardless of
   when your home waystone unlocks.
6. **Your own city's chain quest** — does a player still do the chain quest for their own starting city
   (yes by default, since nobody has waystone access), and does that city's stone quest simply not
   exist for them?
7. **Who gives the chain** — the Kingdom mage (mage tower, no NPC yet) or the waystone-city NPCs.
8. **Stone lore NPCs** — who carries it in each of the 6 starting cities.
9. **Waystone location** — a dedicated building in each starting city, or part of the town square.
10. **Race-shared starting cities** — several races share a start (Human, Half-Elf, Half-Orc and
   Tiefling all start in kingdom; Halfling and Gnome in millhaven), so one city's NPC issues that
   stone to several races. Confirm that's fine.

## 6. Quest design (draft — for workshopping)

### Lore spine
The world already hints at this: the `ancient-elven-waystone` encounter, the `shrine-of-the-four-winds`
POI, and the Elf race quest `elven-memory` ("the ancient waystones of the forest are fading… corruption
from the Shadowmere Wetlands seeks to extinguish them"). So: **the waystones are an ancient elven-built
network, dormant, and being snuffed out by a corruption rising from the Shadowmere.** Homeward Stones are
the *portable* form — a fragment bonded to a person and their home. The chain is the player re-awakening
the network, one waystone at a time, ending at the source of the corruption. (Check overlap with main
quests `the-rising-shadow` / `the-shadows-source` before locking this.)

### Who gives what
Every starting city already has a racial **vault keeper** NPC — the natural stone-giver:

| City | Keeper | Race |
|---|---|---|
| Kingdom | Marcus Goldvault (royal-custodian) | Human |
| Verdant | Silvanthir Moonbough (warden-of-roots) | Elf |
| Ironpeak | Thorin Ironledger (vaultwright) | Dwarf |
| Millhaven | Rosie Lockwood (keywarden) | Halfling |
| Marshlight | Grommash Ironhide (hoardkeeper) | Orc |
| Goldenhaven | Zarithax the Golden (scalekeeper) | Dragonborn |

Keepers *hold things for people*, so "you can't buy a stone you already hold in inventory or vault" fits
them. Each keeper: (a) issues the free origin stone with lore, (b) sells replacements (1000), (c) gives
that city's **stone quest**, (d) sells that city's stone to those who completed it.
The **waystone chain** is given by a new **Archmage in the Kingdom mage tower** (the building exists, no
NPC yet), who researches the network and sends you city to city; each keeper helps with the local step.

### Level gates
Levels attach to the **destination city**, not the player, so the quests are authored once per city.
Order (agreed: Ironpeak last); the chain level is always stone level + 2, with the chain being harder.

| # | City | Stone quest level | Chain level |
|---|---|---|---|
| 1 | Kingdom | 3 | 5 |
| 2 | Millhaven | 4 | 7 |
| 3 | Verdant | 5 | 9 |
| 4 | Goldenhaven | 6 | 11 |
| 5 | Marshlight | 7 | 13 |
| 6 | Ironpeak | 8 | 15 |

### Stone quests (one per city, short, 2–3 stages, no new engine)
| City | Quest | Shape | Danger |
|---|---|---|---|
| Kingdom | The Crown's Ledger | talk → fetch a missing ledger page → Persuasion/Investigation check. Bureaucracy, no combat. | none |
| Millhaven | Keys Go Missing | find Rosie's three lost keys around the village/plains. Cozy; sibling of `the-missing-mule`. | trivial |
| Verdant | Root and Song | gather herbs → Nature check to attune a sapling → talk. Reuses `elven-memory` pieces. | none |
| Goldenhaven | The Golden Toll | deliver crated goods through the customs house → Persuasion/Insight check to win the scalekeeper's trust. | low |
| Marshlight | Prove Your Mettle | slay a swamp beast in the wetlands → Survival check → Grommash judges you. First real fight. | moderate |
| Ironpeak | A Stone Worth Carving | fetch ore from the mine → slay a few kobolds → deliver to Thorin. | moderate–high |

### Waystone chain (6 quests, level 5–15, fixed order, given by the Archmage)
| Lvl | City | Quest | Beat |
|---|---|---|---|
| 5 | Kingdom | The Dormant Spire | Archmage shows the kingdom waystone; Arcana check to wake it. **Reward: hop to the Kingdom from any waystone.** |
| 7 | Millhaven | The Windmill Stone | a fragment buried under the old mill; investigate, fetch a missing part. |
| 9 | Verdant | The Elders' Song | the elven heart of the network. Elves get extra elder dialogue; everyone does the same quest. (`elven-memory` is separate.) |
| 11 | Goldenhaven | The Harbor Beacon | the stone is in the harbor; smugglers/customs stand in the way. |
| 13 | Marshlight | The Source of the Fade | the wetlands where the corruption rises; first real confrontation with it. |
| 15 | Ironpeak | Beneath the Flooded Mine | finale: the deepest stone lies in the flooded depths (same place as `sword-of-the-ancestors`); strongest encounter in the arc. |

Rule (confirmed): a **hop needs you to be standing in one of the six waystone cities** (source is
always allowed); quest completion unlocks a **destination**.

### Per-race view
All 10 races follow the same authored quests. What differs is the free stone's lore, where the player's
home sits in the chain, and small race-aware branches.

| Race | Home / keeper | Home waystone unlocks at | Stone quests (need) | Race hooks |
|---|---|---|---|---|
| **Human** | Kingdom / Marcus | 5 | Millhaven, Verdant, Goldenhaven, Ironpeak, Marshlight | Crown-sealed stone, "registered in the royal ledger". Most straightforward path — the baseline. |
| **Half-Elf** | Kingdom / Marcus | 5 | same as Human | Extra dialogue with the Verdant elders (entry fee is only 2 there). |
| **Half-Orc** | Kingdom / Marcus | 5 | same as Human | Welcome at Marshlight (entry fee 0): Grommash treats the quest as kinship, softer opening. |
| **Tiefling** | Kingdom / Marcus | 5 | same as Human | Also welcome at Marshlight (fee 0); distrust flavor in Verdant/Goldenhaven (fees 5 / 4). |
| **Elf** | Verdant / Silvanthir | 9 | Kingdom, Millhaven, Goldenhaven, Ironpeak, Marshlight | Heartwood seed-stone. Network heirs: extra Verdant-elder dialogue in the chain quest. `elven-memory` stays a separate quest. |
| **Dwarf** | Ironpeak / Thorin | 15 | Kingdom, Millhaven, Verdant, Goldenhaven, Marshlight | Clan-marked vaultstone. `sword-of-the-ancestors` touches the same flooded mine as the level-15 finale. Last home waystone in the chain. |
| **Halfling** | Millhaven / Rosie | 7 | Kingdom, Verdant, Goldenhaven, Ironpeak, Marshlight | Key-stone on your own key ring. Cozy start, early home unlock. |
| **Gnome** | Millhaven / Rosie | 7 | same as Halfling | Tinker angle: an Arcana/Investigation option in waystone quests. |
| **Orc** | Marshlight / Grommash | 13 | Kingdom, Millhaven, Verdant, Goldenhaven, Ironpeak | War-trophy stone from the clan hoard. Late home waystone; highest friction elsewhere (Goldenhaven fee 10, Verdant 5), so their stone quests there carry distrust dialogue. |
| **Dragonborn** | Goldenhaven / Zarithax | 11 | Kingdom, Millhaven, Verdant, Ironpeak, Marshlight | Golden ember-scale from the hoard (matches `ember_vault`). Mid-chain home. |

Note the skew: 4 of 10 races start in the Kingdom, so the most-played path has its home unlocked first.

### Authoring scope
6 stone quests + 6 chain quests = **12 quests**, plus about 10 race-aware dialogue branches, plus
the 2 existing race quests wired in. Not 60.

### Engineering constraints on race-aware content
- NPC dialogue gating (`npc/helpers.go:30`) supports only `not_native`, `registered`, `not_registered`,
  `gold` — **no `race` requirement**. Race branches need that extended, or the branch must live in POI/quest
  nodes, where the `race` requirement type already exists.
- Race quests already use `requirements: [{type:"race", values:[...]}]`.
- Quest gate for buying stones: `quest_completed` on the shop item (see §4).
