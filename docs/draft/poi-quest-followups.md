# POI / Encounter / Quest Follow-ups

Tracks work deferred after the 2026-05-06 schema canonicalisation pass.

The correctness sweep (skill names, requirement-field shapes, NPC dialogue
field names) is **complete**. Reference validation (`go run ./cmd/codex --check-schema`)
is **wired**. Everything below is the remaining work.

Run `go run ./cmd/codex --check-schema` to regenerate the broken-ref list at any time.

---

## 1. Broken references (116 errors across 35 files)

These are concrete and fixable. Each falls into one of three buckets:

- **Remap** — the referenced thing exists under a different ID; just rename the ref.
- **Create** — a real new item / monster / NPC / location is needed; author it in `game-data/`.
- **Cut** — the content path the ref enables isn't worth keeping; rewrite the node to use something existing.

The choice between create vs. cut is a design call (see §2). Below is a triage to drive that decision.

### 1a. Items (missing 38 unique IDs, ~70 occurrences)

**Almost certainly remaps to existing items:**

| Broken ref                  | Likely target               | Note                              |
|-----------------------------|-----------------------------|-----------------------------------|
| `chainmail`                 | `chainmail-set` / `-hauberk`| Pick body piece vs. full set      |
| `potion-of-healing`         | `healing`                   | Canonical potion id is `healing`  |
| `potion-of-water-breathing` | (none)                      | No water-breathing potion exists  |
| `rope-hempen-50-feet`       | `rope`                      | Already canonical                 |
| `silver-ring`               | `signet-ring`               | Or create distinct silver variant |
| `silver-signet-ring`        | `signet-ring`               | Same                              |
| `flask-of-oil`              | (none — `oil` exists?)      | Confirm exact id                  |
| `leather-cap`               | (none)                      | No helmet — drop or create        |
| `ring` (generic)            | specific ring id            | Replace placeholder               |

**Plot/loot items that need to be created** (likely scope: 1-line JSON entries with low value, just exist as collectibles):

`amulet-of-light`, `ancestral-blade`, `ancestral-blade-restored`, `bandit-stash`, `cloak-of-protection`, `cult-manifesto`, `eye-of-the-abyss`, `ghost-lily`, `golden-viper-statue`, `guild-dead-drop-letter`, `guild-signet-ring`, `heavy-crowbar`, `heavy-iron-key`, `hermit-stash`, `hunters-secret-cache`, `incense-of-sight`, `iron-ore`, `marsh-willow-bark`, `moon-essence-shard`, `owlbear-feather-enchanted`, `owlbear-fur`, `purified-gem`, `rare-gems`, `rat-tail-trophy`, `shadow-fragment`, `silver-key`, `silver-moss`, `soul-gem-fragment`, `spices`, `vial-of-pure-water`, `waystone-fragment`, `worthless-trinket`, `ale-mug`, `poison-vial`

Many of these are *intended* to be quest-bound trash items (`worthless-trinket`, `ale-mug`, `rat-tail-trophy`). The cleanest pattern is probably a single shared item file per quest's bespoke loot, or a `game-data/items/quest-items/` subdirectory.

### 1b. Effects (5 unique IDs)

`blood-lust`, `divine-grace`, `fatigue`, `mapped-terrain`, `wind-walker`

- `fatigue` exists already as `fatigue-accumulation` / `fatigued` / `tired` / `very-tired` — refs need to be remapped to a specific tier.
- `blood-lust`, `divine-grace`, `mapped-terrain`, `wind-walker` are net-new and need authoring under `game-data/effects/` with proper `modifiers[]` and `removal.timer` fields. See `docs/poi-quest-design.md` §3 patterns.

### 1c. Monsters (10 unique IDs)

`cave-bear`, `corrupted-dryad`, `dire-wolf`, `kobold-shaman`, `kobold-warrior`, `rat-king-boss`, `restless-spirit`, `shadow-stalker`, `shadow-tentacle`, `will-o-wisp`

- `kobold-warrior` / `kobold-shaman` could be variants of existing `kobold` (rename) or distinct stat blocks (create).
- `dire-wolf` likely a tougher `wolf` variant.
- `cave-bear` is a real D&D 5e creature; consider creating.
- `rat-king-boss`, `shadow-stalker`, `shadow-tentacle`, `corrupted-dryad`, `restless-spirit`, `will-o-wisp` are bespoke — needed for specific quests/POIs.

### 1d. NPCs (7 unique IDs that are real, plus 2 IDs that are mistaken POI refs)

**Mistaken POI refs (objective.type=`talk` pointing at a POI/landmark):**
- `ancient-elven-waystone` — should be `objective.type=explore`, target = the waystone POI/location
- `shadowmere-hermit` — should be `objective.type=talk`, target = the hermit NPC embedded in the POI (currently un-named)

**Real NPCs that need authoring** (likely under `game-data/npcs/<location>/`):
- `oracle-seraphina` (Goldenhaven Temple — referenced 6× in main quest)
- `mayor-thomas` (Goldenhaven Center)
- `barnaby-merchant` (already embedded in `wandering-merchant` encounter — needs to either also live in goldenhaven NPCs or quests need to reference the encounter NPC differently)
- `thistle-goldworthy` (mentor figure for `sword-of-the-ancestors` race quest)
- `tavern-owner-bob` (rats-of-goldenhaven side quest)
- `high-priest-lawrence` (paladin class quest)
- `exiled-orc` (mountain survey daily — could just be a generic NPC in cragspire env)
- `town-square-board` (used as objective target for daily reset; this should probably be a `location` or `bulletin_board` start_condition type, not a "talk" NPC)

### 1e. Locations (6 unique IDs)

- `goldenhaven-east`, `goldenhaven-center`, `kingdom-center` — these are **districts within a city**, not separate locations. The current schema only knows top-level locations (cities + environments). Two options:
  1. Use the city ID and treat district as flavor text.
  2. Extend `EncounterTrigger`/`POIData` to support district refs (would need a districts model).
- `goldenhaven-temple` — same, this is a building inside Goldenhaven.
- `mountain-pass` — should likely be `cragspire-mountains` or a new sub-region.
- `shadow-alley-den` — being referenced as a `location`; it's a POI, not a location. Either fix the ref to use `parent_environment` or extend POIs to be valid `start_condition.location` targets.

### 1f. Locations referenced as POI/location targets in objectives (8 IDs)

`flooded-mine-depths`, `mountain-pass-overlook`, `sewer-drainage-chamber`, `sewer-junction-a`, `sewer-tunnels-entry`, `shadowmere-mirror-pool` — these are sub-areas inside larger POIs that don't exist. Either:
- Create new POIs for each (heavyweight), or
- Treat them as named *nodes* inside an existing POI and rewrite the objectives as "reach node X in POI Y" (requires extending QuestObjective with a node target).

### 1g. Invalid skill in objective (1 occurrence)

`elven-memory.json` stage 7 objective 1 uses `skill: "intelligence"` for a `check`-type objective. `intelligence` is a stat, not one of the 8 derived skills. Replace with `medicine` (the INT-driven skill) or restructure the objective.

---

## 2. Deferred: design-quality pass

Per-file design audit. None of this is required for the schema/refs to be valid — it's about the content being *good*.

For each POI / encounter / quest:

- **Encounters**
  - Confirm `cooldown_minutes` is set on every `repeatable: true` encounter (default 360 friendly / 720+ hostile, see design doc §5).
  - Confirm every encounter has at least one no-requirement walk-away path.
  - Vary skill checks across the file set — currently `perception` is heavily over-used.
  - Sanity-check DCs against the 8d20 derived-skill ranges (see design doc §7 formulas).
- **POIs**
  - Confirm `discovery.chance` is in a sane range (0.05–0.4) and not all the same value.
  - Confirm utility-gated paths (rope, crowbar, thieves-kit, keys) are distributed across files, not concentrated in one or two.
  - Confirm every POI has at least one terminal node beyond the obvious "exit", to give branching weight.
  - Loot tables: convert flat loot into weighted tiers where the encounter is high-value (boss POIs, dungeons).
  - Audit DCs against intended difficulty band.
- **Quests**
  - Quests with one-objective stages should be merged unless the wait-stage matters.
  - Multi-stage quests should accumulate `total_qp` correctly (`total_qp` ≥ `step_count`).
  - Difficulty labels (`Novice`, `Apprentice`, etc.) should be normalised to a fixed vocabulary.
  - `recommended.combat_danger` should match the toughest monster the quest can spawn.
  - `prerequisites` chains should be sanity-checked for cycles or unreachable quests.

---

## 3. Deferred: content additions

Net-new content not covered by Gemini's drafts:

- **Failure-recovery paths**: many POIs have linear "fail = damage, succeed = loot" branches with no second-chance recovery. Consider patterns like "spend gold to bribe past", "wait 8h and retry with bonus", "spend a willpower point".
- **Class/race flavor branches**: only a handful of POIs have class-gated paths. The schema supports it widely; designer pass should add more (Druid in nature POIs, Cleric in sacred POIs, etc.).
- **Embedded NPC schedules**: POI NPCs like Whispering Jack and Viper have no `schedule[]`. Adding even a 2-slot schedule (working / sleeping) would make them feel like real inhabitants.
- **Quest hand-offs**: most current quests don't trigger follow-up quests via prerequisites. Build proper chains: `the-rising-shadow` → `the-shadows-source` already exists but isn't using `prerequisites`.
- **Daily/weekly variety**: only 2 dailies, 0 weeklies. Need a pool of ~6 of each so the random-shuffle feels fresh.
- **Alignment-gated content**: only 2 files use alignment requirements. Design doc §4 implies it should be more pervasive.

---

## 4. Tooling / runtime work (already noted in `project_poi_quest_schema.md`)

Not part of the content cleanup, but listed here for completeness:

- POI / encounter / quest runtime loaders (none yet — `cmd/server/db/migration.go` doesn't know about these directories).
- World-state persistence (POI cooldowns, encounter `LastFired`).
- Encounter scheduling tied to travel ticks.
- Move directories out of `-draft` once runtime systems land.
