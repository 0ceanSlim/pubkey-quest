# POI / Encounter / Quest Design Notes

Schema reference lives in `docs/poi-quest-schema.md`. This doc is the rationale and runtime behaviour - the *why* and *when*, not the *what*.

---

## 1. Why two systems (POIs and encounters)?

POIs and encounters share the same node graph, but their trigger and persistence semantics differ enough that one combined type would lie about both.

| | **POI** | **Encounter** |
|---|---|---|
| Discovery | Rolled while travelling, against `discovery.chance` (optionally with a perception check) | Triggered when player enters a context (travel tick, location, building, or building type) |
| Persistence | Once discovered, lives in `locations_discovered`. Always revisitable. | Transient. Fires on a chance roll; cooldown if repeatable. |
| World map | Becomes a stable map feature | Never a map feature |
| Use case | Dungeons, shrines, hermit huts, ruined towers, ongoing quest hubs | Roadside thieves, drunken brawlers, traveling merchants, one-shot vignettes |

So: if it's a place the player should be able to come back to, it's a POI. If it's an event that happens *to* the player, it's an encounter.

## 2. Discovery & encounter rolls

POIs and encounters both roll on a **time-check tick**. Default tick is 30 in-game minutes (matches the existing travel system). Both reuse the same RNG plumbing.

**POI discovery** (during travel through an environment):
1. For each undiscovered POI on the current environment, compare `position` to the player's current travel progress.
2. If within proximity, roll `rand() < discovery.chance`. If `discovery.skill` + `dc` are set, a passing skill check guarantees discovery; failing falls back to the flat chance.
3. On success, the POI is added to `locations_discovered` and the discovery message is shown.

**Encounter rolls** (every tick in any context):
1. Gather all encounters whose `trigger` matches the current context and whose `valid_locations` / `building_types` matches.
2. Filter by `requirements`.
3. Filter by repeatability: if `repeatable=false` and seen, skip; if repeatable and within `cooldown_minutes`, skip.
4. Roll `rand() < chance` for each candidate. First success fires.

To prevent encounter spam from rapid-fire ticks (entering and leaving a tavern), encounters track `LastFired` per (player, encounter) and respect `cooldown_minutes`.

### Deterministic shuffle (future)

If the random feel becomes too repetitive, switch to a deterministic shuffle: `seed = hash(npub + save_id)`, draw from a shuffled deck of valid encounters per context, advancing only on actual fires. This avoids both "same encounter every time" and pathological streaks. Not implemented yet - flat chance is the v1.

## 3. POI persistence (within a save)

When the player runs through a POI's nodes, side effects mark the POI as "interacted" in session state.

- **Combat nodes**: cleared monsters stay dead until the cooldown elapses. Re-entering shows narrative text confirming the area is quiet.
- **Loot nodes**: looted containers stay empty.
- **Skill checks**: a passed check unlocks the success branch for the duration of the cooldown - no re-rolling. A failed check stays failed (one shot per cooldown).
- **Default cooldown**: 3 in-game days (4320 minutes). After it elapses, all interacted flags clear and the POI is "fresh" again - monsters respawn, loot re-rolls, checks re-roll.

The POI itself is always in `locations_discovered` once discovered; only the *internal* state resets.

## 4. Utility-gated paths

POIs (and individual nodes within them) can be gated by `requirements`. The intended pattern is a Metroidvania-style loop: tools and items become keys to new content.

| Item                   | Use case                  | Mechanic                                  |
|------------------------|---------------------------|-------------------------------------------|
| `rope-hempen-50-feet`  | Pits, shafts              | Unlocks vertical access nodes             |
| `crowbar`              | Jammed doors/crates       | Bonus or auto-pass on Strength checks     |
| `thieves-kit`          | Locked gates              | Required for `thieving` checks            |
| `hammer` + `pitons`    | Cliffs                    | Climbs without needing high `athletics`   |
| `shovel`               | Buried mounds             | Triggers a hidden `loot` node             |
| `silver-key`/`iron-key`| Specific doors            | Bypasses the check entirely               |

Two requirement modes worth noting:
- `consumed: true` - the item is destroyed on use (key in a one-time door).
- `consumed: false` (default) - the item just needs to be present (rope, crowbar).

A choice's `requirements` are *all* required (AND). For OR semantics, author multiple choice entries pointing at the same `next`.

## 5. Encounter authoring guidelines

- Keep the start node small. Either a `passive_check` that splits on player perception, or a `choice` with 3-5 distinct paths (combat, social, escape, special-class).
- Always include at least one path that requires no requirements - a "default" walk-away or fight-anyway option.
- Use `transaction` nodes (cost + reward) instead of fake "loot" nodes when the player is paying for something. The schema has it for a reason.
- Set `cooldown_minutes` for any `repeatable: true` encounter so it doesn't fire every tick. 360 (6 hours) is a reasonable default for friendly merchants; 720+ (12 hours) for confrontational ones.

## 6. Quest design

Quest pacing builds on three gates:

1. **Prerequisites** - `prerequisites: ["<quest-id>", ...]` chains quests strictly. The quest doesn't appear in the log until prerequisites are completed.
2. **Quest-point gate** - `requirements: [{type:"quest_points", min:N}]` walls off high-tier content behind reputation/experience accumulated from prior quest completions.
3. **Adventure-day gate** - `requirements: [{type:"level", min:N}]` or future `min_days` style gating ensures world-shaking quests don't land on day-one characters.

### Wait stages

`wait_minutes > 0` makes a stage "Pending" until the in-game clock advances by that amount. Use this for narrative pauses ("the Oracle needs a day to translate the runes"). The save file holds `ready_at_day` and `ready_at_minute` so the wait survives reloads.

### Stage rewards

By convention only the final stage carries `rewards`. Multi-stage quests build up to one payout. If you find yourself wanting per-stage payouts, that's a sign the "quest" should probably be several smaller quests with prerequisites.

### Randomized dailies/weeklies

`is_randomized: true` marks quests whose stages may be re-rolled. The `last_rolled_day` / `last_rolled_week` fields on `PlayerQuestProgress` track when the current roll happened.

Dailies and weeklies reset on **real time**, not the in-game clock: a daily resets once per real-world day (at a fixed reset hour, server time), a weekly once per real calendar week. At 144× an in-game day is only ~10 real minutes, so game-time resets would be trivially farmable — the real-time cadence is what creates the "check in tomorrow" rhythm. Consequently `last_rolled_day` / `last_rolled_week` hold **real-world** day/week indices (e.g. days since epoch), while wait-stage fields (`ready_at_day` / `ready_at_minute`) stay on the in-game clock.

## 7. Skill catalogue

Authoritative list lives in `game-data/systems/skills.json`. The eight derived skills:

| Skill        | Formula                                                  |
|--------------|----------------------------------------------------------|
| `athletics`  | STR×0.5  +  CON×0.35  +  DEX×0.15                        |
| `crafting`   | INT×0.4  +  DEX×0.3   +  STR×0.15  +  WIS×0.15           |
| `influence`  | CHA×0.7  +  INT×0.3                                      |
| `medicine`   | INT×0.6  +  WIS×0.4                                      |
| `perception` | WIS×0.65 +  DEX×0.35                                     |
| `resolve`    | CON×0.5  +  WIS×0.3   +  INT×0.2                         |
| `survival`   | WIS×0.6  +  CON×0.25  +  DEX×0.15                        |
| `thieving`   | DEX×0.7  +  WIS×0.3                                      |

Use these only - `investigation`, `religion`, `intimidation`, `arcana` etc. don't exist. Map them to the closest derived skill (intimidation→`resolve` or `influence`, religion→`resolve`, investigation→`perception`, arcana→`medicine`).

## 8. Implementation status

What's wired:

- Types in `types/poi.go` and `types/quests.go` are canonical.
- `cmd/schemacheck` validates draft files against the types in strict mode.
- `cmd/schemafix` and `cmd/schemafmt` are one-shot tools that fixed the Gemini drafts; they remain in the repo as long as the `-draft` directories exist, then can be deleted.

What's not wired yet:

- The runtime POI/encounter loaders (will live alongside `cmd/server/db/migration.go`).
- World-state persistence for POI cooldowns (planned for `cmd/server/session/`).
- Quest-progress tracking on the save file (already shaped via `PlayerQuestProgress` in `types/quests.go`).
- Encounter scheduling tied to travel ticks.

The drafts in `game-data/locations/poi-draft/`, `game-data/systems/encounters-draft/`, and `game-data/quests-drafts/` are content-ready; they move out of `-draft` once the runtime systems land.
