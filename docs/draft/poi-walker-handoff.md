# Handoff: finish the POI node-walker (exploration)

State as of commit `035dfa6`. M3 is ~90% done. This doc is the pickup point for
the **next session**: wire the already-built walker engine into a playable
exploration loop.

## What's already done (don't rebuild)
- **Quest system, fully wired + UI** — availability, accept, objective tracking,
  rewards, in-world start (talk to givers), collapsible journal + in-scene detail
  modal. (commits up to `46f5411`)
- **Biome travel encounters** (organic combat) + post-encounter cooldown.
- **POI discovery** (`5b8bafe`) — `maybeDiscoverPOIs` in `actions.go` rolls a POI's
  `discovery.chance` when travel progress crosses its `position`; on a hit it joins
  `LocationsDiscovered` + fires `LocationDiscovered` (XP + explore objectives).
  Perception path uses a passive check.
- **Checks/effects foundation** (`c41c6b8`):
  - `cmd/server/game/skillcheck` — `Resolve` = d20 + `Modifier(skill)` vs DC,
    `Modifier = floor((skill-10)/2)`; `Passive` = 10+mod. Tested.
  - `effects.EffectiveStats(save)` = base stats + active ability modifiers (NOT
    AbilityIncreases — already in save.Stats). Wired into the requirement gate,
    `/api/skills`, and discovery. Design: `docs/draft/checks-design.md`.
- **Walker engine core** (`035dfa6`): `cmd/server/game/poi/walker.go` —
  `Resolve(node, nodeID, save, Deps) StepResult`. Handles narrative / choice
  (gated by `requirement.Evaluate`) / check (→ `skillcheck.Resolve` with
  `Deps.Ctx.SkillValue`, success/failure branch) / reward / transaction / loot
  (guaranteed) / damage / effect / monster (sets `StepResult.Combat`) /
  exit-terminal. Side-effects injected via `Deps{Ctx, Rng, GrantReward,
  ApplyEffect, AddItem}`. Tested per node type in `tests/poi/`.

## What to build next (the task)
1. **POI session state.** Add `ActivePOI *poi.Session` to `session.GameSession`
   (next to `ActiveCombat`, `session/types.go`). `poi.Session{POIID, CurrentNode,
   ValidNexts []string}`. (session→poi is safe; poi doesn't import session.)
2. **Endpoints** `cmd/server/api/game/poi.go` + routes in `api/routes.go`:
   - `POST /api/poi/enter {npub, save_id, poi_id}` → load `db.GetPOIByID`, verify
     it's in `LocationsDiscovered`, build `Deps`, set CurrentNode = `StartNode`,
     `poi.Resolve` it, store ValidNexts (res.Next + each choice.Next), return the
     StepResult.
   - `POST /api/poi/advance {npub, save_id, next}` → validate `next` ∈
     session.ActivePOI.ValidNexts (anti-skip), set CurrentNode = next, Resolve,
     update ValidNexts; on `Terminal`, clear ActivePOI.
   - **Build `Deps`** at this layer (where everything's importable):
     `Ctx = buildQuestContext(state)` (already effective-stats aware),
     `GrantReward = func(s,r){ quest.GrantReward(s,r,advancement) }`,
     `ApplyEffect = func(s,id){ effects.ApplyEffect(s,id) }`,
     `AddItem = func(s,id,q){ inventory.AddItemToInventory(s,id,q) }`,
     `Rng = rand.New(rand.NewSource(time.Now().UnixNano()))`.
   - On a **passing check node**, also `events.Record(save, events.SkillCheckPassed,
     node.Skill, 1)` so quest `check` objectives advance. (Walker returns the
     check result; decide whether the handler or walker fires it — handler is
     simplest, keep walker pure.)
3. **Combat bridge** (the tricky bit). When `StepResult.Combat != ""`:
   `combat.StartCombat(db, state, npub, monsterID, state.Location, advancement)`,
   set `sess.ActiveCombat`, flag `response.Data["combat_started"]=true` +
   `["combat"]` (mirror `maybeRollTravelEncounter` in actions.go), and stash the
   POI **resume node** (`StepResult.Next`) on the POI session. On combat victory
   (where combat ends server-side), if an ActivePOI is mid-fight, auto-advance it
   to the resume node. Front end already enters combat on `combat_started`.
4. **Exploration UI** (`src/`): reuse the dialogue overlay pattern
   (`locationDisplay.js showNPCDialogue`) — render node `text` + `outcome[]` +
   `choices[]` (buttons → `/api/poi/advance {next: choice.next}`) or a Continue
   (→ res.Next). An **Explore** affordance: discovered POIs in the current
   environment need an entry point — simplest is a list/button when travelling or
   on the location view; POIs are ids in `LocationsDiscovered`. The in-scene
   modal pattern (`game/quest-modal.html` + `window.closeQuestModal`) is the model
   for drawing over the scene.

## Key references
- POI data: `db.GetPOIByID`, `db.GetPOIsByEnvironment`; 15 POIs in
  `game-data/locations/poi-draft/`. `POIData.StartNode` + `.Nodes` (map). Some
  reference unbuilt monsters/items (rat-king-boss, silver-signet-ring) — fine,
  loot/monster just won't find them.
- Design: `docs/poi-quest-design.md` (discovery/cooldown/node semantics),
  `docs/poi-quest-schema.md`, `docs/draft/checks-design.md`.
- Combat entry pattern: `maybeRollTravelEncounter` (actions.go) → StartCombat +
  ActiveCombat + `combat_started`; client `tickManager` + `combatSystem`
  (`eventBus.on('combat:started', enterCombatMode)`).
- Per-save `POIState` (already on `types.SaveFile`: POIID, LastDay/Minute,
  Passed/Looted/Cleared) is for the 3-in-game-day cooldown + looted/cleared
  flags — wire when persisting POI outcomes (a refinement after basic walk works).

## After the walker
- **Authored encounter scheduler** — the 9 vignettes (`encounters-draft/`) run
  through the SAME walker, triggered by context on the travel/location tick.
- Small leftovers: `fetch`→item objective (blocked on the broken item-pickup, see
  `docs/draft/ui-inventory-issues.md`), quest wait-stages, daily/weekly reset roll.
- UI/inventory backlog (`docs/draft/ui-inventory-issues.md`): ground items,
  containers, vault click, mobile drag-drop, eat-a-stack consume bug.

## Running it (Windows / this repo)
- DB: `go run ./cmd/codex --migrate` (server refuses to start without `www/game.db`).
- Server: built binary at `$TEMP/pq-server.exe` (more reliable than `go run`),
  port **8584**. Restart = kill PID on 8584 → start binary in background → poll.
- Frontend: `npm run build` (→ `www/dist/`, gitignored). **`game.js` is not
  content-hashed → hard-refresh (Ctrl+Shift+R) after a rebuild** or the browser
  serves the stale bundle (this bit us twice).
- Tests: `go test ./tests/...`. Validation: `go run ./cmd/codex --check-schema` /
  `--validate` / `--check-connections`.
- Commits: no AI attribution (user preference).
