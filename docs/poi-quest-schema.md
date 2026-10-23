# POI / Encounter / Quest Schema Reference

This is the authoritative schema reference for points of interest, encounters, and quests. It mirrors the canonical Go types in `types/poi.go` and `types/quests.go`. If JSON disagrees with this doc, the Go types are right - update the doc.

A draft JSON file can be validated with:

```
go run ./cmd/schemacheck
```

Strict mode (`DisallowUnknownFields`) - any unknown field is an error.

---

## 1. Shared types

### POIRequirement

Used in `POIData.Requirements`, `POIChoice.Requirements`, `POIStep.Requirements`, `EncounterData.Requirements`, and `QuestData.Requirements`.

| Field         | Type       | When                                      |
|---------------|------------|-------------------------------------------|
| `type`        | string     | always (see table below)                  |
| `id`          | string     | for `item`, `skill`, `stat`, `quest_completed`, `quest_active`, `quest_stage` |
| `min`         | int        | numeric thresholds (`skill`, `stat`, `level`, `quest_points`, `item` quantity, `quest_stage`) |
| `max`         | int        | optional upper bound for `quest_stage` (`0` = stage 0 only; leave out for no bound) |
| `values`      | string[]   | enum lists (`class`, `race`, `alignment`, `background`, `deity`) |
| `consumed`    | bool       | for `item` requirements that get spent    |
| `description` | string     | optional human-readable reason            |

| Type             | Shape                                                                                  |
|------------------|----------------------------------------------------------------------------------------|
| `skill`          | `{type:"skill", id:"<skill-id>", min:<dc>}`                                            |
| `stat`           | `{type:"stat", id:"<stat-id>", min:<value>}` (str/dex/con/int/wis/cha)                 |
| `level`          | `{type:"level", min:<n>}`                                                              |
| `quest_points`   | `{type:"quest_points", min:<n>}`                                                       |
| `item`           | `{type:"item", id:"<item-id>", min?:<qty>, consumed?:bool}`                            |
| `class`          | `{type:"class", values:["Fighter","Rogue"]}`                                           |
| `race`           | `{type:"race", values:["Elf","Half-Elf"]}`                                             |
| `alignment`      | `{type:"alignment", values:["good","neutral_good","lawful_good","chaotic_good"]}`      |
| `background`     | `{type:"background", values:["Sage"]}`                                                 |
| `quest_completed`| `{type:"quest_completed", id:"<quest-id>"}`                                            |
| `quest_active`   | `{type:"quest_active", id:"<quest-id>"}` — in progress (not done)                      |
| `quest_stage`    | `{type:"quest_stage", id:"<quest-id>", min:<stage>, max?:<stage>}` — active at a stage index (0-based) in range; a completed quest passes any `min` and fails any `max` |

`quest_stage` is how a quest reaches into a place that also exists on its own: a POI anyone can visit gets a choice gated on the quest's stage (the quest's part of it), and its waypoints, checks and fights feed the stage's objectives.

Skill IDs (canonical, in `game-data/systems/skills.json`): `athletics`, `crafting`, `influence`, `medicine`, `perception`, `resolve`, `survival`, `thieving`. Anything else is wrong.

Alignment string values: `lawful_good`, `neutral_good`, `chaotic_good`, `lawful_neutral`, `true_neutral`, `chaotic_neutral`, `lawful_evil`, `neutral_evil`, `chaotic_evil`, plus the broad shortcuts `good`, `evil`, `lawful`, `chaotic`, `neutral`.

### POIReward

Used as `POIStep.Reward`, `QuestStage.Rewards`, and inside `POIStep.Cost`-paired transactions.

```
{
  "xp":            int,           // optional
  "gold":          int,           // optional
  "quest_points":  int,           // optional
  "items":         [{ "id": "...", "quantity": N }],  // optional
  "effect":        { "id": "...", "amount": N, "duration_minutes": N }  // optional
}
```

### POICost

Resource cost paid at a `transaction` node. At least one of `gold` / `items` must be present.

```
{
  "gold":  int,
  "items": [{ "id": "...", "quantity": N }],
  "theft": bool   // optional: the gold is taken, not paid (a pickpocket)
}
```

A `theft` cost takes what the player carries, up to `gold`, and never fails: an empty purse loses
nothing ("Your purse was empty: they got nothing.") and the walk goes on to `next`. Gold only.

### POIDamage

```
{ "type": "bludgeoning|piercing|slashing|fire|cold|poison|necrotic|...", "amount": int }
```

### POIEffect

```
{ "id": "<effect-id>", "amount": int?, "duration_minutes": int? }
```

`id` references an entry in `game-data/effects/`.

### POILootTable

```
{
  "guaranteed": [LootEntry, ...],   // always drops
  "rolls":      int,                // how many tier-rolls to perform
  "tiers": [
    {
      "name":    "common|uncommon|rare|...",
      "weight":  int,                // relative weight of this tier
      "entries": [LootEntry, ...]    // weighted bucket; one entry rolled per `rolls`
    }
  ]
}
```

`LootEntry`:
```
{ "item": "<item-id>", "quantity": N | [min, max], "weight": N }
```
`weight` only applies inside a tier's `entries`. `quantity` is either a fixed integer or a 2-element `[min, max]` array.

---

## 2. POI (Point of Interest)

A discoverable, revisitable location attached to an environment.

```
{
  "id":                   "<kebab-case-id>",
  "name":                 "Display Name",
  "category":             "dungeon | landmark | utility | settlement",
  "parent_environment":   "<environment-id> | <city>-<district>",   // a road, or a city district (a hidden city place)
  "position":             0.0..1.0,                 // position along the environment's path
  "description":          "...",
  "discovery": {
    "chance":  0.0..1.0,
    "skill":   "<skill-id>",                        // optional - boosts/replaces flat chance
    "dc":      int,                                 // required if skill is set
    "message": "..."                                // shown when discovered
  },
  "requirements":         [POIRequirement, ...],    // optional, gates entry
  "start_node":           "<node-id>",
  "nodes":                { "<node-id>": POIStep, ... },
  "npcs":                 [NPCData, ...]            // optional, embedded
}
```

**NPC embedding rule.** NPCs that exist *only* at this POI live inline in `npcs`. Settlement-wide NPCs (the kingdom barkeep, etc.) live in `game-data/npcs/<location>/` and are not embedded. The split exists so the visual codex editor can ship a POI as a single unit.

**Road POIs and city POIs.** A POI on a road (`parent_environment` is an environment) is found while travelling past its `position`. A POI in a city (`parent_environment` is a district id like `goldenhaven-east`) is a hidden place: a player who spends a while outdoors in that district gets one roll per visit to notice it (its `discovery` chance or passive skill check), and once found it's listed and entered from that district. Either kind is entered only from where it is.

### POIStep (a node in the graph)

```
{
  "type":         POIStepType,
  "text":         "...",                  // narrative text shown to the player
  "next":         "<node-id>",            // for narrative/monster/loot/damage/effect/reward/transaction
  "is_terminal":  bool,                   // true ends the POI run

  // choice
  "choices": [
    { "label": "...", "next": "<node-id>", "requirements": [POIRequirement, ...] }
  ],

  // check / passive_check
  "skill":        "<skill-id>",
  "dc":           int,
  "success_text": "...", "success_next": "<node-id>",
  "failure_text": "...", "failure_next": "<node-id>",

  // monster
  "monster_id":   "<monster-id>",
  "count":        int,                    // default 1
  "surprise":     bool,                   // player gets first round
  "arena":        ["....~~..", ...],      // optional authored battlefield (combat-terrain arena_key)

  // loot
  "loot_table":   POILootTable,

  // damage
  "damage":       POIDamage,

  // effect
  "effect":       POIEffect,

  // reward / transaction
  "reward":       POIReward,
  "cost":         POICost,                // transaction only

  // npc_interaction
  "npc_ids":      ["<npc-id>", ...],

  // gating — when unmet the node does nothing and goes to failure_next
  // (shown failure_text), or ends the walk when there's no failure_next
  "requirements": [POIRequirement, ...],

  // any node
  "waypoint":     "<spot-id>"             // reaching this node counts for quest explore objectives
}
```

A `transaction` takes its `cost` all or nothing; when the player can't pay it goes to `failure_next` (or ends) without granting the reward.

### Node type summary

| Type             | Purpose                                                  | Required fields                          | Outgoing |
|------------------|----------------------------------------------------------|------------------------------------------|----------|
| `narrative`      | Pure text, single transition                             | `text`                                   | `next` or `is_terminal` |
| `choice`         | Player picks a branch                                    | `text`, `choices`                        | per-choice `next` |
| `check`          | Active skill roll (player sees it)                       | `skill`, `dc`                            | `success_next`, `failure_next` |
| `passive_check`  | Hidden skill roll                                        | `skill`, `dc`                            | `success_next`, `failure_next` |
| `monster`        | Combat encounter                                         | `monster_id`                             | `next` (on victory) |
| `loot`           | Distribute items from a loot table                       | `loot_table`                             | `next` or `is_terminal` |
| `damage`         | Apply damage to player                                   | `damage`                                 | `next` |
| `effect`         | Apply a status effect                                    | `effect`                                 | `next` or `is_terminal` |
| `reward`         | Grant XP / gold / items / effect                         | `reward`                                 | `next` or `is_terminal` |
| `transaction`    | Spend resources, optionally receive reward               | `cost` (and usually `reward`)            | `next` or `is_terminal` |
| `exit`           | Explicit terminal node                                   | -                                        | `is_terminal: true` |
| `npc_interaction`| Hand off to NPC dialogue system                          | `npc_ids`                                | `is_terminal: true` |
| `lock`           | A lock: its key, a pick (minigame or quick attempt), or walk away | `lock` (`{key, dc, jam_on_miss}`, see `docs/development/locks.md`) | `success_next` (opened), `failure_next` (walk away; empty leaves) |

---

## 3. Encounter

An encounter shares the POI step graph but fires probabilistically when the player enters a context, rather than being discovered on a map. Encounters are typically one-and-done or repeatable with a cooldown; POIs are permanent and revisitable.

```
{
  "id":               "<kebab-case-id>",
  "name":             "Display Name",
  "description":      "...",
  "trigger":          "travel | location | building | building_type",
  "valid_locations":  ["<location-or-environment-id>", ...],   // for location/building triggers
  "building_types":   ["tavern", "inn", "shop"],               // for building_type trigger only
  "chance":           0.0..1.0,                                // probability per check
  "repeatable":       bool,
  "cooldown_minutes": int,                                     // optional
  "requirements":     [POIRequirement, ...],
  "start_node":       "<node-id>",
  "nodes":            { "<node-id>": POIStep, ... },
  "npcs":             [NPCData, ...]
}
```

### Trigger semantics

| Trigger          | When it can fire                                                | What `valid_locations` / `building_types` mean |
|------------------|-----------------------------------------------------------------|------------------------------------------------|
| `travel`         | While the player is travelling between locations                | Environment IDs the encounter is eligible in   |
| `location`       | On entering a specific location (city/district/environment)     | Location IDs                                   |
| `building`       | On entering a specific building                                 | Building IDs                                   |
| `building_type`  | On entering any building of a given type (e.g. all taverns)     | `building_types` lists the type IDs            |

POIs vs encounters at a glance:

| Aspect             | POI                                | Encounter                       |
|--------------------|------------------------------------|---------------------------------|
| Discovered once?   | Yes (added to `locations_discovered`) | No, fires on trigger            |
| Revisitable?       | Yes                                | Only if `repeatable: true`      |
| Triggered by       | Travel discovery roll              | Context entry + chance roll     |
| Lives in           | `game-data/locations/poi/`         | `game-data/systems/encounters/` |

---

## 4. Quest

Quests live in `game-data/quests/<category>/<quest-id>.json`.

```
{
  "id":             "<kebab-case-id>",
  "name":           "Display Name",
  "category":       "main | side | class | race | daily | weekly",
  "difficulty":     "Novice | Moderate | Hard | ...",
  "step_count":     int,
  "total_qp":       int,                                  // total quest points awarded
  "is_randomized":  bool,                                 // dailies/weeklies that re-roll stages
  "description":    "...",
  "start_condition": {
    "type":       "talk | explore | item | bulletin_board",
    "target":     "<target-id>",
    "location":   "<location-id>",                        // optional
    "start_hint": "Player-facing hint shown in the quest log."
  },
  "requirements":  [POIRequirement, ...],                 // class/race/level/quest_points gating
  "prerequisites": ["<quest-id>", ...],                   // must be completed first
  "recommended": {
    "recommended_stats": ["Strength 14", "Resolve 12"],
    "combat_danger":     "Low | Moderate | High"
  },
  "sites":  { "<site-id>": QuestSite, ... },              // optional, places that exist only for this quest
  "stages": [QuestStage, ...]
}
```

### QuestStage

```
{
  "id":           "<kebab-case-id>",
  "description":  "...",
  "wait_minutes": int,                          // narrative pause before stage activates
  "objectives":   [QuestObjective, ...],
  "rewards":      POIReward,                    // optional, usually only on the final stage
  "unlocks_poi":  "<poi-id>",                   // optional, reveals a POI on completion (derived, not saved)
  "encounter":    QuestStageEncounter,          // optional, a fight waiting at a place while the stage is active
  "site":         "<site-id>"                   // optional, opens one of the quest's sites during this stage
}
```

### QuestSite

A place that exists only for a quest: the sewers behind a tavern's cellar wall. It is **not** a POI. Nothing about it is discovered or saved. Its entrance shows at its spot only while the player is on a stage whose `site` names it, and it plays as a node walk with the same nodes as a POI. Use a POI with a `quest_stage`-gated branch instead when the place should also be visitable without the quest.

```
{
  "name":        "The Old Sewers",
  "description": "...",
  "location":    "<city-id>",                    // where the entrance is; empty district/building/room match anywhere
  "district":    "<district>",
  "building":    "<building-id>",
  "room":        "<room-id>",
  "label":       "The breach in the wall",       // entrance button (defaults to name)
  "start_node":  "<node-id>",
  "nodes":       { "<node-id>": POIStep, ... }
}
```

Waypoints inside a site are valid `explore` targets for its quest.

### Quest dialogue

`dialogue` holds the giver's `offer`, `active` and `completed` conversations, plus `with`: what *other* NPCs say while the quest is in progress (`{"<npc-id>": QuestDialogueTree}`), e.g. the merchant the giver sends you to. A `talk` objective ticks the moment the player greets the NPC, so gate a `with`/`active` node on the stage *after* the talk (see `the-rising-shadow.json`).

**Turn-ins.** A `talk` objective on a quest's *last* stage (the last one the character does; class-limited stages after it don't count) is the turn-in. Greeting the NPC doesn't meet it. Once every other objective on that stage is done (fetch counts what's carried right then), the NPC's menu offers **📜 Turn in: <quest>**, and choosing it finishes the quest (`events.TurnedIn`). The conversation closes and the rewards screen (the last stage's `rewards` plus `total_qp`) comes up at once, carrying the giver's closing line: the opening text of the quest's `completed` conversation, or a plain thank-you. (The `completed` conversation is still there to talk through afterwards.) A last stage without a `talk` objective (slay, check) finishes as soon as it's done and still gets the rewards screen. Vault-keeper bounties turn in to any keeper (`vault-keeper`).

An `npc_interaction` node ends the walk and offers to talk to its `npc_ids` right there, which is how NPCs who live at a POI (Thistle at the Wayward Wagon) are reached. See `quests-drafts/side/rats-of-goldenhaven.json`.

### QuestObjective

| Type      | Required fields                          |
|-----------|------------------------------------------|
| `talk`    | `target` (npc-id)                        |
| `fetch`   | `target` (item-id), `count` — counts what the player carries; `consume: true` hands the items over when the stage completes |
| `explore` | `target` (location-id, poi-id, or a POI/site `waypoint`) |
| `slay`    | `target` (monster-id), `count`           |
| `check`   | `skill` (skill-id), `value` (DC)         |

`description` is required on every objective.

---

## 5. NPC embedding (inside POI/encounter `npcs`)

Embedded NPCs use the canonical `NPCData` shape (`types/types.go`):

```
{
  "id":             "<kebab-case-id>",
  "name":           "Display Name",
  "title":          "Role / Job title",          // optional
  "race":           "Human | Elf | ...",         // optional
  "description":    "...",                       // optional
  "greeting":       { "first_time": "...", "returning": "..." },
  "dialogue":       { "<dialogue-key>": { ... free-form } },
  "schedule":       [NPCScheduleSlot, ...],      // usually omitted for POI-bound NPCs
  "shop_config":    { ... free-form },
  "storage_config": { ... free-form },
  "inn_config":     { ... free-form }
}
```

Greeting is a map (not a bare string). `dialogue` is intentionally free-form so the codex can iterate without schema churn.

---

## 6. Reference files

When in doubt, read these - they're the authoritative templates:

- `game-data/locations/poi-draft/abandoned-watchtower.json` - dungeon POI with combat, checks, branching loot.
- `game-data/locations/poi-draft/wayward-wagon.json` - utility POI with a single embedded NPC.
- `game-data/systems/encounters-draft/city-pickpocket.json` - encounter with passive check, alignment/class gating, transaction node.
- `game-data/quests-drafts/template.json` - blank quest template.
- `game-data/quests-drafts/side/rats-of-goldenhaven.json` - multi-stage quest with check/explore/slay objectives and a final reward.
