# Pubkey Quest — Release Roadmap (Pre-Alpha → Alpha → Beta → 1.0)

**Written:** 2026-06-11. **Last status update:** 2026-07-06.
**Progress (pre-alpha → alpha):** M0 ✅ · M1 ✅ (progression) · M2 ✅ (rooms) · M3 ✅ (quest/POI/encounter runtime + daily roll) · **M4 ✅ (spells & items as real systems)** · **M5 ✅ (combat completion: abilities, conditions, saves, interaction matrix)** · **M5.5 ⬜ (core-gameplay completion — feats, spell scrolls, drop/pickup, gathering & minigames — RESCOPED into alpha 2026-07-13)** · **M6 ⬜ ← active (presentation)** · M7 ⬜ (content) · M8 ⬜ (release eng). Recent extras done off-milestone: the **MILL login layer** replaced the bespoke Nostr auth (grain v0.8) + an in-game bug/access reporter; a **full item-data refinement pass** (all 209 items priced [game gold = D&D copper: gp×100/sp×10/cp×1], described, tagged — via the item-reporter/item-refiner agents, `docs/draft/item-*`); plus the earlier item `value` economy + effect-aware pricing, death keep-3, inventory rarity, item-info redesign, debug/settings cleanup, and the **2026-07 equipment/inventory hardening pass** (unified pointer-events core, container/vault/quiver fixes, equipment stat panel, backend inventory test net, `docs/draft/ui-inventory-issues.md`). **Deferred backlog:** progression polish (level-up popup, spend-confirm), wait-stages, delta-engine sync (POI-loot lag / phantom heal), ground-item pickup stub, shop revamp (inventories + UI), mobile touch QA.
**How to read this:** §1 is an honest inventory of what exists. §3 is the detailed pre-alpha → alpha plan. §4 is the Nostr save & trust architecture that constrains everything else. §7 is the UI/UX critique. §8 is the content/liveness strategy.

Parallel infra track (not gameplay): Grain client upgrade + login replaced with the mill library — ✅ **landed (2026-07)**: grain v0.8, the **MILL (Multi-Interface Login Layer)** Web Component handling every signing method client-side (NIP-07/46/55, key, read-only, new identity), server only ever sees the hex pubkey. Remaining constraint is beta-stability (beta = saves become precious).

**Core architectural premise (see §4):** saves are player-signed Nostr events on relays — portable across clients, owned by the player. That means: saves are *deliberate* (Pokemon/Fallout-style, no background autosave), the save schema must be hydration-first and byte-frugal from day one, and cheat-prevention is an official-server concern (event-ID validation), not a save-format concern. A modded ecosystem is a feature, not a threat.

**Two standing design decisions:**
- **Scene images are flavor art, period.** No hotspots, no click-the-painting interactivity — interaction lives in the button UI and room navigation. (Recorded so nobody re-proposes it.)
- **Buildings have rooms.** Entering a building puts you in its default room; you move between rooms or back out. NPCs occupy rooms, not buildings — see M2.

---

## 1. Where the game actually is

(`CLAUDE.md` has since been refreshed to describe the architecture accurately and defers status to this file.) Real state:

### Implemented end-to-end — functional, none of it polished

Every system below works and is architecturally sound, but **all of it carries a polish/debug debt**: edge cases are undefined or silently wrong at every step. Live examples: holding a spellbook in combat shows it as an attack option that does nothing when clicked; equip/unequip during combat has no defined rules; and that pattern repeats across the board. The plan treats this as real scheduled work (see the polish discipline note in §3 and the interaction-matrix task in M5), not a footnote.

| System | Evidence |
|---|---|
| Auth (NIP-07 + Amber), deterministic char gen | `cmd/server/auth/`, `api/character/` |
| Session architecture: in-memory authoritative state, delta updates, 417ms tick, smooth clock | `cmd/server/session/`, `src/systems/tickManager.js`, `deltaApplier.js` — the "Option 4 delta architecture" draft actually shipped |
| World sim: time, day cycle, NPC schedules, building open/close | `game/gametime/`, `game/npc/schedule.go`, `game/building/` |
| Travel: start/stop/resume/turn back, progress %, arrival discovery, music unlock, travel fatigue | `game/travel/` |
| Survival loop: hunger, fatigue, encumbrance — all data-driven through the effect system | `game/status/`, 28 effect JSONs |
| Inventory: equip/unequip, containers, move/stack/split, weight | `game/inventory/` — ⚠️ **ground drop/pickup is NOT real**: drop destroys the item (no ground store), pickup is a no-op that lies. Fixed in M5.5 |
| Economy: shops w/ pricing + stock, vaults (racial storage w/ rites), inn rooms/sleep, bard shows | `game/shop/`, `world/merchant.go`, `game/vault/`, `game/npc/housing.go`, `entertainment.go` |
| Combat core: initiative, two-phase turns, 0–6 range, full weapon properties, ammo, opportunity attacks, disengage, dash/charge, Hold&Ready/Dodge, flee, death saves, loot, defeat penalty | `game/combat/`, `combat-overlay.html`, `combatSystem.js` — feels okay, as you said |
| Spell **prep + casting (M4)**: prep queue (per-spell timers), shared shape-driven casting engine (mana + rune components, focus, concentration), combat + out-of-combat cast, combat spell/item panels | `game/spells/{prep,cast}.go`, `game/combat/cast.go`, `api/game/{spells,combat}.go`, 4 prep endpoints + `/api/combat/{cast,use-item}` |
| Skills: 8 derived skills computed server-side | `api/game/skills.go`, `game-data/systems/skills.json` |
| Content tooling: Codex (item editor, char-gen tables editor, systems editor, pixellab, migration, validation, `--check-schema/--format-schema`) | `cmd/codex/` — the schemacheck/fmt/fix consolidation is done in the working tree. **Big gaps remain**: no editors for locations, monsters, spells, or NPCs (the "character editor" edits char-gen tables, not creatures; all of those are hand-edited JSON today — rooms will make locations worse), and no NPC-schedule→building cross-validation — see the Codex track in §3 |
| POI/Encounter/Quest **schema**: canonical types, strict validation, 36 draft content files | `types/poi.go`, `types/quests.go`, `docs/poi-quest-{schema,design}.md` |
| **Progression (M1)**: level-up, ability-point earn/spend, re-derived MaxHP/MaxMana, save ritual, derived level/XP display | `game/character/`, progression endpoints |
| **Building rooms (M2)**: per-room NPC placement, inn flow, in-building bar rework, connectivity validation | `game/building/`, `api/game/rooms.go`, `locationDisplay.js` |
| **Event recorder (M1)**: central gameplay-event stream (kill/fetch/explore/talk/check/…) feeding quest objectives + discovery XP | `game/events/`, `game/quest/objective.go` |
| **Quest / POI / Encounter RUNTIME (M3)**: quest engine, POI node-walker (combat-bridged), authored-encounter scheduler (3 triggers), discovery, daily/weekly roll, quest-tracker chip | `game/quest/`, `game/poi/`, `api/game/{poi,encounter_scheduler,quests}.go`, `poiExplore.js`, `questTracker.js` |
| **Economy (M3-era)**: canonical item `value`, shop pricing off effective charisma (effects move prices) | `game/shop/pricing.go`, `api/game/shop.go` |

### Half-baked (exists but doesn't actually work end-to-end)

1. ✅ **RESOLVED (M1) — Level-up works.** Progression (level-up + ability-point earn/spend, re-derived MaxHP/MaxMana, journaling, save ritual) shipped; level is derived from XP (hydration-correct) and the top-bar number + XP bar track it. *Feats are slotted but not yet active (post-M5).*
2. ✅ **RESOLVED (M4) — Spell casting is real.** Shared shape-driven engine (`game/spells/cast.go`): known+prepared+mana+components gates; attack / auto-hit / save / heal / buff resolved from the spell JSON; focus-provided rune components; concentration (CON save on damage). Combat cast (`POST /api/combat/cast`) + out-of-combat `cast_spell`; frontend combat spell panel + spells-tab cast. All 84 spells hand-refined (mana, per-spell prep time, homebrew rune components). *Remaining:* combat-integration tests + the M5 conditions the save-shape spells need.
3. ✅ **RESOLVED (M4) — Items in combat.** `POST /api/combat/use-item` drinks potions/consumables mid-fight (loose general slots + general-slot pouches), bridged onto the combat HP pool; combat use-item UI shipped.
4. ✅ **RESOLVED (M5) — Martial class abilities work.** `POST /api/combat/ability` spends a combat-scoped resource pool (Stamina/Rage/Ki/Cunning); two abilities per martial class are wired to real combat state, launched from an in-combat ability chooser. *Feats still slotted-but-inactive (post-M5).*
5. ✅ **RESOLVED (M3, 2026-06-29) — Quests / POIs / Encounters now run end-to-end.** Quest engine (availability, talk-to-NPC + innkeeper-daily start, event-fed objective tracking, stage rewards, derived QP), real quest-log journal + over-scene tracker chip, POI node-walker (playable, combat-bridged), authored-encounter scheduler (all 3 triggers, 9 vignettes), daily/weekly roll (schema v3). Save now carries quest fields (`QuestsActive`/`QuestsCompleted`/`POIStates`/`RepeatableQuests`). *Still open:* the 116 broken content refs (`docs/draft/poi-quest-followups.md`) are an M7 content job, not engine.
6. **NPC dialogue presentation** — backend dialogue trees are genuinely good (greetings w/ first-time/returning/native-race, requirements, branching). But the *speech text renders as a transient toast in the top-right corner* (`locationDisplay.js:862` → `showMessage(msg,'warning')`) and options are 7px buttons in the bottom 125px strip. The single most-authored content in the game is shown in its least readable surface.
7. ✅ **RESOLVED (M1) — Save ritual** (win95 save modal, "last saved" awareness, deliberate save) + session journaling for crash recovery shipped.
8. **Tracking — partial.** The central **event stream now exists** (`game/events/`) and feeds quest objectives + discovery XP. Still missing: persisted statistics/counters on the save (kills, quests done, gold earned, deaths) and the badges system (button still hidden until badges exist); on the official server this stream is also the action audit trail (§4).

### Missing entirely
- ~~Quest/POI/encounter runtime~~ ✅ **DONE (M3)**
- ~~Building rooms~~ ✅ **DONE (M2)** — NPCs placed per-room; inn flow; in-building bar rework
- ~~Encounter triggering~~ ✅ **DONE (M3)** — biome + authored encounters fire on context; debug-only path retired
- **Codex content tooling** — no editors for locations (buildings live inside city JSON, so rooms raise the stakes), monsters, spells, or NPCs; no validation that NPC schedule slots point at real buildings/rooms; no derived world-map view (§3 Codex track)
- ~~Conditions, saving throws vs. spells, monster difficulty scaling~~ ✅ **DONE (M5)** — the 6 alpha conditions + saves both directions + deadly-fight guardrail. *Still deferred:* stealth/surprise, the other ~9 D&D conditions, and full CR scaling math (beta)
- ~~Spell casting + items in combat~~ ✅ **DONE (M4)** · ~~class **abilities** in combat~~ ✅ **DONE (M5)** — 2 wired abilities per martial class off a combat resource pool
- ~~Spell scrolls/crafting (deferred to beta)~~ → **RESCOPED into alpha (M5.5)** — scroll crafting + use; general item-crafting stays beta (`docs/draft/spell-scroll-system.md`)
- **Feats** — schema slot reserved (`FeatsChosen`), point-accounting works, but no selection UI and no effects → **RESCOPED into alpha (M5.5)** with a bounded MVP feat set (`docs/draft/feats-progression.md`)
- **Gathering-node POIs + mining/fishing/lockpicking minigames** — drafted (`draft_enviornment.txt`, `checks-design.md`) → **RESCOPED into alpha (M5.5)**; per-skill XP/professions stay out (skills are stat-derived)
- The official-server validation layer (§4): event-ID chain tracking, save plausibility checks. Note `POST /api/session/update` accepts a raw full-state overwrite today — fine solo, must be gated before strangers arrive

### Content inventory (what alpha has to work with)

| Content | Count | Notes |
|---|---|---|
| Items | 209 | ✅ **fully refined** — all priced (game gold = D&D copper), described, tagged (item-refiner pass); artisan tools merged into 4 skill-kits; + ~38 quest items to author (followups §1a) |
| Spells | 84 | ✅ **fully refined** — per-spell mana, prep time, homebrew rune components; casting engine live (M4) |
| Monsters | **31** | all combat-ready; ~10 more needed by quest drafts |
| Effects | 28 | + 4 net-new needed by drafts |
| Cities | 9 | **only 5 have art** (missing: dusthaven, frosthold, goldenhaven⚠️, saltwind) |
| Environments | 9 | **only 2 have art** (darkwood-forest, merchants-highway) |
| NPCs | 27 files | + ~7 quest NPCs to author (oracle-seraphina, mayor-thomas, …) |
| POI drafts | 15 | ✅ runtime live (walker); still in `-draft` until M7 ref-fix |
| Encounter drafts | 9 | ✅ runtime live (scheduler, all 3 triggers) |
| Quest drafts | 11 | 2 main / 3 side / 2 class / 2 race / 2 daily / **0 weekly**; engine live; dailies innkeeper-given + roll. **Author weeklies** to exercise the weekly pool |

⚠️ Goldenhaven is the main-quest hub (Oracle, temple, mayor) and has no city art.

---

## 2. Release definitions (what the words mean for this game)

**Pre-alpha (now, M0–M3 done / M4–M8 remaining):** core loop coming together — quest/POI/encounter runtime live, organic combat entry, progression + rooms working; content still in `-draft` dirs; spells/abilities/items-in-combat not yet real (M4/M5); wipes still possible (schema-break — most recent: save v3).

**Alpha:** *feature-complete core loop, content-light.* A stranger with a Nostr key can: log in → get a character → take the intro → learn time/saving/vault/trading from the spotlight tutorial → explore towns → enter buildings and walk their rooms → talk to NPCs (readably) → craft a spell scroll → pick up a quest from an NPC or board → travel → discover POIs including **gathering nodes (mine/fish) and locked chests (pick)** → hit a random encounter → fight with weapons *and* spells *and* scrolls *and* items *and* class abilities → win/lose meaningfully → drop/pick up loot → level up and get stronger (**spend ability points or take a feat**) → buy/sell/rest/bank → complete the quest → see progress tracked → **save deliberately and trust the save round-trips**. Placeholder art acceptable. Save wipes announced but rare (schema-break only). Invite-only testers. Bug-report button (already exists) is the feedback channel.

**Beta:** *content-complete-ish, persistence-trustworthy.* Main quest act 1 + side content per town; dailies/weeklies live; Grain + mill landed; **saves live on relays per §4** (size-budgeted, event-ID validated on the official server); balance pass; no planned wipes; open invite.

**1.0:** act 1 polished with story set pieces and final art, audio pass, Nostr-issued badges, mobile-presentable, performance hardened, public launch.

---

## 3. Pre-alpha → Alpha: the detailed plan

Nine milestones. M0–M5 are systems, M6 is presentation, M7 is content, M8 is release engineering. Sizes assume solo dev + AI pairing: **S** ≈ ≤3 days, **M** ≈ 1 week, **L** ≈ 2–3 weeks.

Recommended order: **M0 → M1 → M2 → M3 → M4 → M5 → (M6 ‖ M7) → M8.** M1 before M3 because the quest engine writes to the save schema M1 defines; M2 (rooms) before M7 because quest NPCs get authored with room placements. M6 and M7 can interleave once systems are stable. The **Codex track** (after M8 below) runs in parallel the whole way — land C1/C2 and the C4 monster/NPC editors before M7 at the latest, so the content pass has real tooling.

**Polish discipline (standing rule):** every system that "works" still needs its edge cases defined and its bugs swept — the spellbook-attack no-op is the canonical example. Budget ~20% of each milestone for polishing and debugging the systems it touches, keep a running `bugs/` issue label, and treat "undefined interaction" (what *should* happen when X meets Y?) as a design task, not just a bug. M8 ends with a dedicated bug-bash.

### M0 — Land the in-flight work, stabilize the base (S) — ✅ DONE

**✅ Done (2026-06-21)** — the month of in-flight work landed as 7 logical commits on `main` (`a2d7704`…`57a93f4`); tree clean, shape-check green, CLAUDE.md rewritten.

- [x] Commit the POI/quest schema work + codex validation consolidation (logical commits: types, codex, content drafts, NPC data, docs)
- [x] Delete `nul` file at repo root; `go mod tidy` (dropped unused `kr/text`, `rogpeppe/go-internal`)
- [x] `go run ./cmd/codex --check-schema` green on shape; broken-ref list is the M7 content backlog (`docs/draft/poi-quest-followups.md`)
- [x] Rewrite `CLAUDE.md` to match reality (`cmd/server`/`cmd/codex`/`types/` layout, the DB-built-by-codex-first flow, codex workflow) — note: `CLAUDE.md` is gitignored, so it lives locally
- [x] Finalize the NPC schedule model **including room references** — decided: optional `room` per schedule slot, absent = building's default room (implemented in M2); `primary_home` externalization now committed
- [x] Remove the dead commented `<!-- OLD SCRIPTS -->` blocks (all 7 view files, not just game/game-intro)

**Done when:** clean `git status`, schema check green, CLAUDE.md trustworthy. ✅ met.

> Side task landed alongside M0: artisan/profession **tools consolidated into 4 skill-linked kits** (Crafting/Thieves/Herbalism/Navigator); instruments + gaming sets kept individual. Design: `docs/draft/tools.txt`.

### M1 — Progression that actually works (M) — ✅ DONE

The "level up don't work right" fix, plus the save-schema groundwork everything else needs.

> **🚧 In progress (2026-06-21).** **Pillar A (level-up) ✅** and the **save-schema v2 core ✅** are done & committed (`977f3a5`…`e5a295e`). `ApplyLevelUp` landed as **derived** max stats (`character/derive.go` + `Hydrate` on load) plus one central **`GrantXP`** path — not a stored-and-grown `levelup.go`. New spell-slot rows + spells-known on level-up defer to **M4** (they only matter once casting exists). Persist-omit of MaxHP/MaxMana deferred to the §4 serializer (derived-authoritative already holds). **Remaining:** only the event recorder, deferred to M3 (save-ritual UI + session journaling have since landed ✅).

**Level-up application**
- [x] `game/character/levelup.go`: `ApplyLevelUp(save, newLevel)` — HP gain = fixed (half hit die + CON mod, per the planning formula), mana growth for casters (INT/WIS mod + level scaling), class-resource max growth for martials, new spell-slot rows from `spell-slots.json`, cantrip/spells-known additions from class tables
- [x] Apply on `POST /api/combat/end` when `level_up_pending` (and from any future XP source — make it a generic `CheckAndApplyLevelUps(save)` called wherever XP is granted)
- [x] Level-up response payload: old→new HP/mana/slots/abilities so the frontend can present it
- [x] **Level-up moment in UI** (deferred from combat Phase 10): modal over the scene — "Level 3! +6 HP, new spell slot" — this is a reward beat, make it feel good even with simple presentation
- [x] Audit proficiency-bonus scaling already used in `resolveAttackBonus` against `advancement.json` for levels 1–5
- [x] `kill_bonus_xp` field on MonsterData — implemented as a per-monster kill reward (not cut); POI/dungeon-step source hooks into the M3 node walker

**Save schema v2 — designed as the future Nostr event payload (§4 rules apply now)**

One migration, all at once, so alpha saves survive. Every field must pass the hydration test: *if the server can derive it from game data + other save fields, it does not go in the save.*

- [x] Add to `types.SaveFile`: `Quests` — two compact lists that **are stored in the save** (they're the only way to know on login what a character has done): *completed* = an array of quest IDs, nothing else; *in-progress* = tuples of `[quest_id, stage_index, objective_counts…]` (step-4 compaction later turns the ID into an index int). Quest points, availability, and log display all *derive from* these lists. Also: `POIState` (per-POI: last-interacted day/minute + passed/looted/cleared flags — "fresh again" derives from cooldown math), `Room string` (position state for M2 — joins the existing Location/District/Building trio), `Rentals` (compact `[building, expires_day, expires_minute]` — today rentals are session-only in `GetRentedRooms()` and a paid room evaporates on reload; it's a paid, non-derivable outcome, so it belongs in the save and doubles as M2's room-unlock state), `SchemaVersion int`
- [x] **Ability-point allocation (new M1 scope):** `AbilityIncreases map[string]int` — points spent per ability, the only stored record of allocation (base stays a creation snapshot, never re-derived from the editable generation tables). `FeatsChosen []string` reserved (empty until the feats milestone). Unspent points and feat counts *derive* — see [feats-progression.md](draft/feats-progression.md). Cadence 2/4/6/8/10/12/14/16/18/19/20 (11 points), per-stat cap 20, total cap 100 (matches the about-page). Per-level `AbilityPoints` in `advancement.json`; `GET/POST /api/progression/{ability-points,spend-point}` + guide endpoint; spend UI (`ability-allocate-modal.html`, opened from the level-up modal + progression guide). XP level-multiplier centralized in `character.BonusXP` so it reaches all sources. Logic + tests in `cmd/server/game/character/` + `tests/character/`.
- [x] **Explicitly do NOT add**: `Level` (derives from XP + advancement — the canonical example of the rule), `QuestPoints` (the *number* stays out because it's `sum(total_qp)` over the stored completed-quest list — to be unambiguous: the **list** is in the save, the **aggregate** never is), max-derived stats, anything the official server can recompute from game data + the lists the save does hold
- [ ] Encounter `LastFired` anti-spam state stays **session-only** (worst case after a reload an encounter can re-fire early — acceptable, free bytes)
- [ ] Lifetime statistics (kills, deaths, gold earned…) do **not** live in the save by default — they're tracked server-side from the event recorder and later published as separate server-issued Nostr events (badges/stats, §4). Only add an in-save counter if a *gameplay mechanic* gates on it
- [x] `LoadSession` migration shim: v1 saves get zero-valued new fields (forward-compatible loads)
- [ ] **Central event recorder** (deferred — build at M3 start with its first consumer): `game/events/record.go` — `Record(save, EventKindMonsterKilled, target, n)` feeds quest objectives (M3), dailies, and badge/stat tracking; on the official server this same stream is the **action audit trail** (§4). Call sites: combat kill, item pickup, location discovery, NPC talk, shop transactions, sleep. Build it once, not three times.

**Deliberate saves + session resilience (two different things — don't conflate them)**
- [x] *Saves are player actions*: explicit, Pokemon/Fallout-style, blocked in combat (already 409s). No background autosave — in the Nostr model a save is the player signing an event, which can't and shouldn't happen silently
- [x] Save UX as a ritual: win95 save modal replaces the browser `confirm()` (`www/views/game/save-modal.html` + `src/systems/saveRitual.js`), with a "last saved: N min ago" indicator (Settings + modal, `data-last-saved`) and Ctrl/Cmd+S. *Still optional polish:* contextual save *prompts* at inn rest / arrival.
- [x] *Session resilience is a server concern*: `cmd/server/session/journal.go` snapshots active sessions to `data/sessions/` every 2 min; restored on load when newer than the last deliberate save (`RecoverJournaledSave`); removed on save/reload/clean-quit so a lingering journal means a crash. Never signed, never leaves the server. Tested in `tests/session/`.

**Done when:** killing things levels you up with visible stat growth ✅; the save file passes the hydration audit (nothing derivable stored) ◐ (derived-authoritative via Hydrate; literal persist-omit deferred to the §4 serializer); a server crash restores the session ✅ (journaling); quitting without saving reverts to the last deliberate save ✅ (save-ritual modal + last-saved indicator) — and that's correct behavior.

### M2 — Buildings get rooms; NPCs live in them (M) — ✅ DONE

> **✅ Done (2026-06-26).** Room engine + navigation, NPC room placement, the spatial inn flow (rent → unlock → night-gated **Sleep**), and the room UI all shipped; rooms authored for every alpha inn/tavern. Landed alongside: a world-connectivity validator (`--check-connections`), the Frozen Wastes crossing fix, an environment-type reconciliation, and a building-placement pass — innkeepers (with homes + day-cycle schedules) for the 3 NPC-less towns, plus Verdant and Ironpeak de-clustered by district theme. The only unmet "done-when" bits are *demonstrative content* (an NPC standing in a back room, one key-locked door) — engine-supported, folded into M7.

The structural change to how interiors work: entering a building puts you in its **default room**; you navigate to other rooms and back; NPCs are present *in a room*, not smeared across the building. This also quietly fixes the "People column lists everyone in the building" overload.

**Schema (backward-compatible by construction)**
- [ ] Building objects (inside city district JSON) gain optional `rooms: [{id, name, description?}]` + `default_room` — a building with no `rooms` behaves exactly as today (one implicit default room), so **zero migration for the ~90% of buildings that don't need rooms** (shops, stalls)
- [ ] **Rooms can be locked**, mirroring building open/close: optional `access` on a room — `hours` (time window, like buildings), `key` (item requirement — reuse the `POIRequirement` evaluator, same engine that gates POI paths; `silver-key`/`iron-key` from the quest drafts now have an indoor use), or `state` (unlocked by an action/flag — the canonical case: an inn's guest room is locked until you rent it, then it's *yours*). No `access` = always open
- [ ] NPC schedule slots gain optional `room` (slot `location` already holds the building ID, e.g. `ember_vault`); no `room` = present in the default room
- [ ] Tooling: rooms land *inside city JSON*, which Codex can't edit yet — either pull the location editor (Codex track C1) forward, or hand-edit JSON for M2's tavern/inn floor and lean on validation (C2) to catch bad refs. Don't let the editor block the engine work

**Server**
- [ ] `Room string` on the save (added in M1's schema v2) — set to `default_room` by `enter_building`, cleared by `exit_building`
- [ ] New action `move_to_room` — validates the room exists **and is accessible** (hours window / key in inventory / state like an active rental), with a clear locked message otherwise
- [ ] `GET /api/npcs/at-location` gains `room` param; NPC visibility = schedule slot matches building **and** room
- [ ] **Room-scoped actions**: the inn flow becomes spatial — rent from the innkeeper → the locked guest room unlocks for you → walk into it and the **Sleep** option appears there (and only there). Rental state moves from session-only to the save (M1's `Rentals`) so a paid room survives reload. Same pattern later for vault interiors, tavern back rooms, night-only rooms

**UI**
- [ ] Inside a building, the "Buildings" column becomes **Rooms** (+ Exit); "People" shows current-room occupants only. Locked rooms render visibly locked (🔒) rather than hidden — a visible locked door is an affordance and a tease; unlocking gives feedback
- [ ] Scene header line shows `Building — Room` (district line already exists); scene image can stay per-building for alpha (per-room art is post-alpha flavor, scenes are just flavor by design)

**Content floor for alpha**
- [ ] Taverns + inns get 2–3 rooms each (common room / kitchen or cellar / guest hall); temples get sanctuary + back room where quests need it (Goldenhaven temple, M7); everything else stays single-room implicit

**Done when:** you can walk into a tavern's common room, find the barkeep there but the cook in the kitchen, see a locked guest room, rent it from the innkeeper, watch it unlock, walk into *your* room, and sleep in it; a keyed door refuses you without the key; no existing building breaks.

### M3 — Quest / POI / Encounter runtime (L — the big one) — ✅ COMPLETE (2026-06-29)

**Status:** done. Shipped: the shared **requirement evaluator** + **event recorder**; content **loaders/migration** (quests/pois/encounters tables); the **quest engine** (availability, talk-to-NPC start, objective tracking via events, stage rewards, QP derived); the **POI node-walker** wired into a playable loop (enter/advance endpoints, anti-skip, combat bridge both ways, discovery + travel-bar markers, exploration overlay); the **authored-encounter scheduler** (travel / location / building_type triggers, all 9 vignettes, through the same walker); biome **organic combat** (retired the debug-only path); the over-scene **quest-tracker chip**; and the **daily/weekly roll** (innkeeper-given, one-per-period rotation, repeatable, schema v3). **Deferred (out of done-when):** wait-stages (no quest content uses them) and POIState cooldown/looted persistence (POIs re-walkable for now). The checkboxes below are retained for historical detail; all but those two are done.

Everything was designed (`docs/poi-quest-design.md`) and content existed; this was pure engine work.

**Loaders + migration**
- [ ] Extend `cmd/codex/migration` + server `db/migration.go` to load `pois`, `encounters`, `quests` tables from the (still `-draft`) dirs; keep strict schema check as a migration pre-gate
- [ ] Move dirs out of `-draft` only at M7 when refs are fixed

**Quest engine** (`game/quest/`)
- [ ] Availability: prerequisites + requirements (uses skills, level, quest_points — *derived* at check time — items, class/race, alignment; the `POIRequirement` evaluator; share it with POIs/encounters/dialogue)
- [ ] Start conditions: `talk` (hook into `handleTalkToNPCAction` — NPC offers quest in dialogue), `explore`, `item`, `bulletin_board`
- [ ] Stage state machine: objective tracker fed by the M1 event recorder (`talk`/`fetch`/`explore`/`slay`/`check`), wait-stages (`ready_at_day/minute`, in-game clock), stage completion → next stage / rewards / `unlocks_poi`
- [ ] Rewards: gold/items/XP/effects via existing inventory + effects + (M1) XP path; quest points accrue implicitly via the completed-quest list
- [ ] Daily/weekly roll: pool per category; resets are **real-time** — daily at a fixed real-world hour, weekly per real calendar week (*not* in-game days, which last ~10 real minutes at 144×). `last_rolled_day`/`last_rolled_week` hold real-world day/week indices; wait-stages stay on the in-game clock; `is_randomized` re-roll
- [ ] Endpoints: `GET /api/quests/log`, `POST /api/quests/accept`, `POST /api/quests/abandon` (objective progress flows through existing action responses/deltas, not polling)

**POI runtime** (`game/poi/`)
- [ ] Node walker: `narrative`, `choice` (requirements + consumed items), `skill_check` (derived skills vs DC, d20-style roll server-side), `combat` (bridges into the existing combat engine; on victory resume next node), `loot` (weighted tables → ground/inventory), `transaction`, terminal nodes
- [ ] Per-save POI state: interacted flags, passed/failed checks, cleared combat, looted containers, 3-in-game-day cooldown reset — in the compact `POIState` from M1
- [ ] Discovery: travel-tick roll vs `discovery.chance` (+ optional perception check), adds to `locations_discovered`, map/travel UI entry
- [ ] POI entry UI: reuse the dialogue surface (M6) — POIs are "dialogue with a place"; same node renderer covers both

**Encounter scheduler**
- [ ] Hook the 30-min time-check tick (travel system already ticks): gather candidates by trigger context (travel/location/building/building-type) → requirements filter → repeatable/cooldown filter → chance roll → first success fires (building-level granularity; rooms don't need their own triggers in v1)
- [ ] Encounters run through the same node walker; combat nodes finally give **organic combat entry** (retire the debug-only path)
- [ ] `LastFired` per (session, encounter) in session memory (not the save — see M1)

**Quest log UI (functional, not pretty yet)**
- [ ] Replace placeholder `tabs/questlog.html`: active quests w/ current stage + objective progress (2/3), completed list, QP total (derived, returned by the API)
- [ ] **Active-objective tracker chip** over the scene (one line: "◆ Speak with Oracle Seraphina — Goldenhaven Temple") — biggest bang-for-buck quest UX element

**Done when:** ✅ MET — dailies are acceptable (from **innkeepers**, not a board — user preference); a side quest can be taken from an NPC, progressed by killing/fetching, turned in for gold+XP; a POI is discovered while traveling and runs start-to-finish including its combat node; wolves (biome) and authored vignettes can jump you on the highway.

### M4 — Spells and items as real systems (L) — ✅ DONE (2026-07-02)

> **✅ Done.** Shipped the shared **casting engine** (`game/spells/cast.go` — shape-driven from the spell JSON: attack / auto-hit / save / heal / buff, mana + material components, focus-provided runes, concentration), **combat cast** (`POST /api/combat/cast`) + **use-item** (`/api/combat/use-item`), **out-of-combat casting** (replaced the `cast_spell` stub), and the frontend combat spell/item choosers + spells-tab cast. The full **84-spell library was hand-refined** (per-spell mana, prep time, and a homebrew rune-component economy driven by the spell-refiner agent) and **all 209 items priced/described/tagged** (item-refiner agent). Committed + pushed (`ef6535c`…`39a1d0c`). **Resource model:** mana + material components (no per-cast slot consumption); prepared + known are the gates. **Deferred to M5:** the 47 spell + 3 item `[~]` mechanic proposals (conditions / summons / on-hit riders — `docs/draft/{spell,item}-mechanics-proposals.md`), plus combat-integration tests. Checkboxes below retained for historical detail.

The "whole system that needs implementation."

**Casting engine** (`game/spells/cast.go`) — shared by combat and exploration
- [ ] Validation: spell known + slotted + prepped (slot not used), components/focus (`provides` data already exists on focus items), mana cost
- [ ] Resolution by spell shape, driven by the spell JSON you already have: attack-roll spells (fire-bolt), save spells (DC = 8 + prof + casting mod; needs monster saves — M5 overlap), auto-hit (magic-missile), heal, buff/effect application (reuse effect system: `bless` → ActiveEffect), utility (light, mage-armor)
- [ ] Slot consumption + upcast (cast at higher slot), cantrips free, mana cost per your mana model
- [ ] Concentration: one concentrating effect at a time; broken on damage (CON save) — data flag already exists on all 84 spells
- [ ] Replace the `cast_spell` stub for out-of-combat utility/heal casting

**Combat integration**
- [ ] `POST /api/combat/cast` (or extend `/action` with `action_type`): target selection, range from spell data (0–6 mapping per plan §13), AoE = all monsters for v1
- [ ] Monster response turn reuses existing flow; spell lines into the staggered combat log + dice/flair animations (the combat UI patterns are already there)
- [ ] Combat UI: spell panel listing *prepped, affordable* spells with mana/slot cost — flat buttons first, polish in M6
- [ ] Holding a spellbook/focus finally *means* something: it's what you give up a weapon hand for — see the M5 interaction matrix for how it renders in the attack UI

**Consumables & items in combat** (plan §14)
- [ ] `POST /api/combat/use-item`: potions/food/throwables from quick slots; uses an action; routes through the same `use_item` effect path
- [ ] Quick-slot access rule: general slots only (4) usable in combat, backpack not — makes the 4 general slots a real loadout decision

**Scope guard:** implement the ~25 spells of levels 0–2 across the starting classes *well* (that covers alpha's level band, ~1–5); stub higher-level spells with "not yet implemented" rather than half-supporting all 84. Scrolls/crafting stay in draft for beta.

**Done when:** a wizard can clear a wolf with fire-bolt + magic missile, concentrate on bless, drink a potion mid-fight, and cast light in a dark POI node.

### M5 — Combat completion: abilities, conditions, saves, and the interaction matrix (M) — ✅ DONE (2026-07-07)

> **✅ Done.** Conditions engine (`game/combat/conditions.go` — the 6 alpha conditions
> plus outlined/charmed/unconscious, with advantage/incapacitation/speed hooks and
> save-to-end/duration ticks at end of turn), the generic **saving-throw resolver**
> (player + monster; drives M4 save spells + monster on-hit riders authored in
> `hit.special`), the **interaction matrix** (non-weapons strike Unarmed; a broad set
> of out-of-combat actions blocked mid-fight), **class abilities** (`POST /api/combat/ability`
> — a memory-only Rage/Stamina/Ki/Cunning pool + 2 wired abilities per martial class),
> and the **difficulty guardrail** (`encounter.Difficulty` → deadly-fight warning + badge;
> random encounters were already CR/level/biome-gated). Remaining spell riders
> (color-spray/sleep/charm-person) landed too. Committed (`3dc5204`…`7f5cf7d`). Checkboxes
> below retained for detail.

- [x] **Combat interaction matrix** — enforced server-side (`processGameAction` blocks
  equip/unequip/move/stack/drop/pickup/use_item/cast_spell/vault/rest/travel/npc while
  `ActiveCombat != nil`), and non-weapons in hand (spellbook/torch/tools) fall back to
  Unarmed in both the engine (`loadWeaponItem`/`isWeaponItem`) and the attack-label UI —
  fixes the spellbook-attack no-op. *Deferred (not alpha-blocking):* in-combat weapon-swap
  as an action and improvised-weapon 1d4 — equipping is simply blocked mid-fight for now.
- [x] **Class abilities** (plan §12): `POST /api/combat/ability` spends a combat-scoped
  resource pool (Rage/Stamina/Ki/Cunning, seeded from class+level, regen per turn / per hit
  taken / per crit). Two per class, all wired to real state: barbarian rage (dmg%+resist) /
  intimidating-roar (frighten); fighter second-wind (heal) / action-surge (extra action);
  monk flurry (extra strikes) / patient-defense (dodge); rogue sneak-attack (bonus dice) /
  shadow-step (disengage+dodge). Unwired abilities return a graceful "not usable yet".
- [x] **Conditions** (plan §15): all 6 alpha conditions (poisoned, prone, frightened,
  restrained, blinded, stunned) as combat-scoped conditions with advantage/disadvantage +
  incapacitation hooks; plus outlined/charmed/unconscious for the control spells.
- [x] **Saving throws** (plan §23): generic resolver (`playerSaveTotal`/`monsterSaveTotal`)
  — drives M4 save spells both directions and monster on-hit riders (wolf knockdown → prone,
  ghoul → paralyzed, giant-spider → restrained/poisoned) read from authored `hit.special`.
- [x] **Difficulty guardrails** (plan §22 minimal): `encounter.Difficulty(cr, level)` rates
  each fight; StartCombat warns in-log + the overlay shows a ⚠ Tough / ☠ Deadly badge for
  authored/POI fights that outclass the player. Random encounters stay CR/level/biome-gated.
- [x] **Defeat outcome: keep the current mechanic** (keep top-3 items by value, warp away, full HP/mana restore) — decided after weighing save-revert. It self-balances: random-encounter deaths usually land well *after* the last save, so accepting the loss beats re-grinding; static/boss fights *will* be reloaded past, and that's accepted single-player freedom (§4); post-defeat, the retry friction is the walk back — players who can't win yet will adventure elsewhere and return stronger, which is exactly the loop we want. The vault is the balancing valve: gold is an inventory item, vault contents are untouchable by death, so informed players bank before adventuring and carry lean loadouts — intended play, taught explicitly by the M8 tutorial. **Open workshop item** (not a blocker): whether carried gold should get a flat-% return instead of riding the top-3-by-unit-cost rule — as written, a gold stack flattens to 1gp units and effectively always vanishes

**Done when:** every class has a button that isn't "attack," save spells work both directions, conditions visibly modify fights, and nothing clickable in combat is a no-op.

### M5.5 — Core-gameplay completion (L) — **RESCOPE 2026-07-13: pulled into alpha**

> **Why this exists:** "alpha is done when core gameplay is in." Feats, spell scrolls,
> the drop/pickup loop, and gathering/minigame POIs are *mechanics the player interacts
> with*, not content or polish — so they move into alpha. A three-agent audit (2026-07-13)
> confirmed the systems below are the only core-gameplay gaps left; everything else is
> M6 polish, M7 content, or explicitly-deferred (magic items, general crafting, professions).
> Can run alongside M6/M7. Feats and scrolls are each **L**; the loose stubs are **M**;
> gathering v1 is mostly content.

- [ ] **Loose core stubs (do first — small, and one is a data-loss bug):**
  - [ ] **Ground drop/pickup loop.** Today drop (`inventory.go:337` `HandleDropItemAction`) just nulls the slot — **the item is destroyed**, there is no ground store — and pickup (`actions.go:562` `handlePickupItemAction`) is a pure no-op that lies (`Success:true`, changes nothing). Build a per-location session-scoped ground store (`world/ground.go`, already stubbed "future" in `world/doc.go`); drop writes to it, pickup reads it + finds a free slot. Combat loot is a separate path (unaffected). *(M — fix the destroy-on-drop bug even sooner.)*
  - [ ] **Effect `removal.type:"action"` lifecycle.** `types/effects.go` declares `action` removal (+ `RemovalCondition.Action` = rest/sleep/consume-antidote) but nothing reads it (`effects.go` only honors `timed`/`hybrid`). An authored "cure with antidote" / "lasts until you rest" effect never clears. Wire an action→removal hook at the rest/sleep/use-item sites. Latent now; blocks any cure/poison content. *(M)*
- [ ] **Feats** (design: `docs/draft/feats-progression.md` + `feats.json`, ~23 designed). Accounting already works (`FeatsChosen []string` reserved; `abilitypoints.go` derives unspent as `cadence − feats − allocated`). Build: (1) `feats.json` → structured executable JSONs under `game-data/systems/feats/` + a `feats` DB table (`migrateFeats`, clone of `migrateAbilities`) — real design work, since drafted effects are prose; (2) **half-feat stat-choice** needs a save-schema decision (`FeatsChosen []string` can't encode "Resilient: CON") — resolve under §4 hydration before coding; (3) selection endpoint `POST /api/progression/choose-feat` + `GET /api/progression/feats` (near-clone of the ability-point handler) and a feat-pick UI at the level-up moment (spend-point **or** take-feat, explicit trade); (4) a **feat-effect application layer** (new — the core engine work): fold stat-grant feats into an effective-stats/`DeriveMaxHP` read, apply passive combat feats as permanent ActiveEffects or `hasFeat()` reads at the calc sites. **MVP set that ships in alpha:** the ~7 clean stat feats + Tough + War Caster + Elemental Adept; feats needing systems we don't have (surprise/traps/reactions/per-rest-pools/skill+armor proficiency) are excluded from the migrated set until those land. *(L, long-tailed — bound it by scoping the shipped feat set.)*
- [ ] **Spell scrolls** (design: `docs/draft/spell-scroll-system.md`). Craft single-use scrolls (gold + components, INT-scaled success, gated by class/level/INT/location — no save tracking, recipe visibility is a pure function of stats); any magic class can then **use** a scroll to cast the spell (still costs mana; consumes the scroll; **bypasses known/prepared + material-component gates**). Build: (1) **cast-from-scroll** entry in `game/spells/cast.go` — the one real engine change (~40 lines: skip `knowsSpell`/`IsSpellPrepared`/`resolveComponents`, keep mana + shape resolution); (2) combat scroll-use path (attack/save scrolls need the monster target + damage/XP/kill — reuse `ProcessPlayerCast`'s consequence tail, ideally refactored into a shared helper) + out-of-combat scroll-use (heal/buff/utility); (3) a `game/crafting` (or `game/scroll`) service — recipe filtering + `CraftScroll` (validate → roll → consume → add) — plus `GET /api/scrolls/recipes` + `POST /api/scrolls/craft`; (4) content: ~20 recipe + ~20 scroll-item JSONs (+ sprites), verify referenced components exist, add `crafting_stations` to ~8 locations, `scroll_recipes` migration table; (5) frontend crafting panel (like the shop panels) + scroll-use surfacing. `spellbook.json` already declares `allowed_types:["spell-scroll"]`. **Decision:** magic-classes-only for use in alpha (non-casters lack mana; mana potions don't exist — defer). *(L; content authoring is the long pole.)*
- [ ] **Gathering POIs + lockpicking + minigames** (player-drafted; keep light — `draft_enviornment.txt` resource tables, `checks-design.md` hard-vs-soft checks, `environment-poi-system.md` "utility POIs", `skills.json` maps Mining/Woodcutting→Athletics, Fishing→Survival, Lockpicking→Thieving; tools `pick-miners.json` + `thieves-kit.json` exist, **fishing tool doesn't**):
  - [ ] **Gathering-node POIs (v1)** — a mining/fishing node is expressible **today** as a utility POI: tool `requirement` gate (pickaxe/fishing-tool, `consumed:false`) → `check` node (Athletics/Survival, DC from `gather_difficulty`) → `loot` node. Author them + create the resource items (`iron-ore`, …) + a fishing-tool item. *(S — content, no engine change.)*
  - [ ] **Tiered gathering yields** — `walker.go grantGuaranteedLoot` grants only `Guaranteed`; implement weighted-`tiers` rolling so the drafted common/uncommon/rare tables pay out. *(S–M)*
  - [ ] **Lockpicking** — a locked chest = a `check`(thieving) node gated by a `thieves-kit` requirement; tools grant advantage per `checks-design.md`. **Zero new mechanics for v1.** *(trivial)*
  - [ ] **Minigames (mining / fishing / lockpicking)** — the interactive layer the user wants. v1 = **skin-over-check**: the frontend animates the interaction while the server `check` node stays authoritative (preserves Go-first anti-cheat) — small, pure `poiExplore.js` + an overlay per game. A fuller version = a first-class `minigame` POI node type bridging out to a frontend overlay (mirroring the combat handoff) and reporting success/failure back. *(S skin-over-check → M–L first-class; decide per game.)*
  - **Explicitly not doing:** per-skill XP / professions — skills are purely stat-derived by design; a skilling-XP track conflicts with the hydration rule (§4). Gate gathering by tool + POI tier + the existing d20 check instead.

### M6 — Presentation overhaul (M) — full critique in §7

The targeted fixes, in priority order:

- [x] ✅ **Dialogue: speech onto the scene, options stay in the strip** (2026-07-12) — one reusable scene speech-box (`src/ui/sceneSpeech.js`) now serves **both** NPC dialogue and POI/encounter nodes: JRPG box on the scene's lower third (portrait placeholder + name plate + win95/Dogica text + optional in-box choices). NPC speech moved off the left message log; options still live in the bottom strip; the old centered `#poi-modal` text path is retired. *Follow-up:* portrait art (placeholder for now); the original note below is kept for detail. NPC speech moves out of the corner toast (`locationDisplay.js:862`) into a JRPG-style box on the scene's lower third — portrait placeholder + name plate + readable text. That's the "NPC text in player vision" fix. The **option buttons keep their current home and interaction** (the bottom strip that swaps in for the action buttons — the pattern is right); what changes is styling: the overlay is currently generic gray + yellow border + tan buttons, visually foreign to everything around it — restyle box and buttons into the win95 bevel language (dark palette, inset/outset borders, pixel font at the re-based sizes). The speech-box renderer is reused for POI/encounter nodes (M3) — one renderer, three systems
- [ ] **Fix scaling distortion**: replace the non-uniform stretch (`game.html scaleGameUI`) with uniform integer scaling + letterbox
- [ ] **Re-base the canvas**: 756×503 with 6–8px fonts is below comfortable legibility. Recommend 960×540 (16:9, integer-scales to 1080p/4K) with a real pixel font (Dogica is already in the intro) at 8/16px sizes; minimum effective text ≥ 10px
- [ ] **Log demotion + channels**: log becomes a scrollback *history* (system/travel/combat-summary); narrative never lives there; toasts only for save/error/unlock notices
- [ ] **Quest tracker chip** over scene (from M3) + level/XP visibility: XP bar exists, add "Lv 3 → 4" context on hover/click
- [ ] **Right rail content (tab layout stays exactly as it is — no merging)**: quest log goes real (M3); the Equipment tab gains the stats your gear actually produces, shown on the page (AC, main/off-hand attack + damage, ranged + ammo count); the Stats tab — skills and effects are already in — fills out with the missing character numbers: level + XP-to-next, proficiency bonus, AC, attack summary, carry weight vs capacity (exact list to workshop)
- [ ] **Rooms navigation polish** (from M2): Rooms column with occupant count badges ("Common Room (3)"), Exit always last, consistent ordering
- [ ] **Save ritual UI** (from M1): win95 save modal, "last saved" indicator, decline-able save prompts at inns/arrivals
- [ ] Replace remaining native `confirm()`s with the win95 modal; hide the Badges button until badges exist

### M7 — Alpha content pass (L, parallelizable with M6)

Work the `poi-quest-followups.md` triage top to bottom:

- [ ] **Fix all 116 broken refs** — remap where targets exist (`potion-of-healing`→`healing`, `chainmail`→`chainmail-set`, fatigue tiers, …), cut what isn't worth it, author the rest:
  - [ ] ~30 quest items as a `game-data/items/quest-items/` batch (1-line JSONs + pixellab sprites)
  - [ ] ~10 monsters (kobold variants, dire-wolf, cave-bear, rat-king-boss, shadow-stalker, …) — C4 monster editor (validated hand-editing as fallback) + the priority-monster pattern
  - [ ] 7 quest NPCs (oracle-seraphina, mayor-thomas, thistle-goldworthy ✅ drafted, tavern-owner-bob, high-priest-lawrence, …) with at least 2-slot schedules **and room placements** (M2)
  - [ ] 4 effects (blood-lust, divine-grace, mapped-terrain, wind-walker)
  - [ ] Resolve the district/building location-ref question (followups §1e) — recommend option 1 for alpha: city-level refs + flavor text; districts model is post-alpha
- [ ] **Quest pool for liveness**: grow dailies 2→6 and weeklies 0→4 (templates exist; these are cheap — bounty/fetch/delivery/survey patterns) so the board never repeats inside a real-world week
- [ ] **Bulletin board**: a board "building" or fixture per city center wired to the real-time daily/weekly roll (schema already anticipates `bulletin_board` start conditions)
- [ ] Design-quality pass from followups §2 (cooldowns on repeatables, walk-away paths, DC sanity vs skill formulas, perception over-use)
- [ ] **Room content floor** (from M2): taverns/inns 2–3 rooms each across the 9 cities; Goldenhaven temple rooms for the quest spine — authored with the C1 location editor (validated hand-editing as fallback if C1 slipped)
- [ ] **Art floor for alpha** (placeholder quality fine, consistency matters): 4 missing city scenes — Goldenhaven first — and 7 missing environment scenes; pixellab-in-Codex makes this tractable. Time-of-day tint variants are a cheap big win if easy
- [ ] Main quest stays *drafted* but ship its first 3–4 stages as the alpha "spine" to validate the multi-stage machinery (full act 1 is beta content)
- [ ] Move `-draft` dirs to canonical paths; retire `--fix-schema`

### M8 — Alpha release engineering (M)

- [ ] **Versioning**: tag `v0.1.0-alpha.1`; version string in the settings tab + bug-report template
- [ ] **Save policy**: schema v2 + migration shim from M1 means "we try not to wipe"; wipes only on schema breaks, announced via a login MOTD. Local save files are explicitly the stand-in for future relay events — schema discipline (§4) starts now so alpha saves can, in principle, be re-signed into relay events at beta
- [ ] **Systems bug-bash**: one dedicated pass across the §1 "implemented" table hunting undefined interactions and no-op buttons (the spellbook class of bug) — alpha testers should find *new* bugs, not the ones you already know how to find
- [ ] **Deploy**: the existing `docs/development/deployment` path; config hardening (debug mode OFF disables `/api/debug/*` + free `add_item` paths — verify nothing else cheaty leaks in prod build)
- [ ] **Don't ship the foot-guns**: gate `POST /api/session/update` (raw state overwrite) behind debug mode now; the official-server validation layer (§4) replaces it properly at beta
- [ ] **Feedback loop**: bug-report button already opens GitHub — add an in-game "what were you doing" pre-fill (location, version, last action); pin a known-issues list
- [ ] **Lightweight telemetry**: server-side counters (sessions started, combats, quest completions, deaths) — enough to see if testers actually play; no third-party analytics
- [ ] **Balance smoke pass**: one full playthrough per class archetype (martial/caster/hybrid) to level ~4; tune monster XP, shop prices, daily payouts
- [ ] **Perf sanity**: 417ms/client polling is fine at alpha scale (<50 testers); note SSE/batched ticks as the beta fix; check SQLite under concurrent sessions
- [ ] **First-session tutorial** (spotlight toasts — build *after* M6's canvas re-base so the UI anchors are stable):
  - Tiny overlay engine: darken the UI, cut a highlight around one element, explain it in a toast, advance on Next or on the player actually doing the thing. Steps are a data-driven script (anchor, text, advance condition); skippable at any time; nothing fancy — toasts all the way down, the *intro* is where cinematic lives
  - Script v1, in order: welcome to the world → how time works (play/pause/wait, the clock) → walk the city (districts, buildings, rooms) → talk to someone → how saving works (deliberate — sign and save before you log off) → **vault + shop tutorial** (bank your gold; death can never touch the vault) → finale at the city exit: "the world is out there — travel, discover new locations, be careful," with the death mechanic explained honestly so lean loadouts are an informed choice, not a surprise
  - Completion = one `omitempty` flag on the save (follows the character across clients; passes hydration — not derivable)
- [ ] Alpha onboarding: append one screen to the intro of "alpha notes: what's in, what's missing, how to report"

### The Codex track — tooling in parallel (slices land just-in-time)

Buildings live *inside* city JSON and rooms will live inside buildings, so world authoring and world validation are about to get heavier — and Codex doesn't cover locations at all yet. This track runs alongside the milestones; each slice lands right before the work that needs it.

- [ ] **C1 — Location editor** (needed before M7, ideally during M2): create/edit cities, districts, buildings, rooms, and environments in Codex instead of hand-editing JSON. The hard part is the *linking*, not the forms: district↔district connections, city↔environment↔city travel links, building→room containment, room access/locks (hours / key item / state), entry fees, open hours. Every link is a picker over existing IDs, never free text — that's what prevents the next 116-broken-refs pile
- [ ] **C2 — World-integrity validation** (minimum slice can precede C1): extend the validation panel / `--check-schema` to cross-check the world graph — **NPC schedule slots reference real buildings** (and rooms, post-M2), connection symmetry (A→B implies B→A or is flagged as intentional one-way), no orphan districts/buildings/rooms, shop inventories reference real items. Codex-added NPCs get validated at authoring time, not discovered broken at runtime
- [ ] **C3 — Derived world map** (derivation during M3–M7; player-facing later): build the location graph once — nodes = cities/environments (later: discovered POIs), edges = travel links with cardinal directions (the travel system already thinks in N/S/E/W) — and render it twice:
  1. **Codex view first**: a node-link map of the whole world for authoring sanity — dangling links, unreachable locations, geographic nonsense jump out visually. Cheap once the graph derivation exists; pays off immediately in M7
  2. **Player map screen later (beta)**: the same derivation filtered to `locations_discovered`, laid out by cardinal direction. A hand-styled "real" art map can replace the derived look at 1.0 — the derivation remains the data layer underneath either way
- [ ] **C4 — Creature/spell/NPC editors** (priority order matters): Codex has no editor for any of these today. **Monster editor first** — M5 wants monster-data expansion (`kill_bonus_xp`, saving throws) and M7 authors ~10 new monsters; the stat-block form + loot-table editor is the highest-leverage slice. **NPC editor second** — M7 authors 7 quest NPCs with schedules and room placements, and Codex-authored NPCs are exactly where C2's building/room validation pays off. **Spell editor last** — all 84 spells are data-complete and already validated, so authoring is rare; balancing edits can stay hand-edited JSON until it hurts. Validated hand-editing remains the fallback for all three — editors accelerate M7, they don't gate the engine milestones

### Sizing summary

| Milestone | Size | Theme |
|---|---|---|
| M0 stabilize | S | commit, clean, NPC schedule+room decision |
| M1 progression + save v2 | M | level-up, hydration-first schema, event recorder, session resilience |
| M2 rooms | M | buildings → rooms, NPCs placed per-room |
| M3 quest/POI/encounter runtime | **L** | the big engine build |
| M4 spells + items | **L** | casting engine, combat integration |
| M5 combat completion | M | interaction matrix, abilities, conditions, saves |
| **M5.5 core-gameplay completion** *(rescope)* | **L** | feats, spell scrolls, drop/pickup loop, effect-action removal, gathering/minigame POIs |
| M6 presentation | M | dialogue-on-scene, scaling, readability |
| M7 content pass | L (parallel) | refs, NPCs, rooms, dailies, art floor |
| M8 release eng | M | bug-bash, deploy, policy, feedback |
| Codex track C1–C4 | M–L (parallel) | location editor, world validation, derived map, monster/NPC/spell editors |

*Feats folded into M5.5 (was late-alpha/early-beta). Within M5.5: feats **L** (long-tailed — bounded by the shipped feat set), scrolls **L** (content is the long pole), drop/pickup + effect-action removal **M**, gathering v1 mostly **S** content + a **S–M** tiered-loot engine bit, minigames **S** (skin-over-check) to **M–L** (first-class node type).*

At solo+AI pace that's roughly **4–5 months of focused work** to alpha (M5.5 adds ~a month). If it must compress: M2 can ship taverns/inns-only, M5 conditions/abilities can ship half-done, M7 environment art can slip, and within M5.5 the interactive minigames can degrade to skin-over-check and feats to the clean-MVP set — but **M1, M3, M4, the M6 dialogue fix, and now the M5.5 core mechanics cannot be cut**, they *are* the (rescoped) alpha.

### Explicitly NOT in alpha
Relay-hosted saves (the schema discipline ships in alpha; the relay plumbing is beta, §4) · story set-piece engine & act-1 art (beta, §8) · **general item crafting** (turning gathered resources into gear — only *scroll* crafting is in alpha; gathered resources are sellable/quest loot for now) · **per-skill XP / professions** (skills stay stat-derived — §4 hydration) · magic items · mobile layout (landscape lock only) · districts model · per-room art & per-room encounter triggers · stealth/surprise · party/companions · monster lairs · player-to-player interaction (post-1.0, §4). Scene-image interactivity is not deferred — it's **not planned at all** (scenes are flavor art by design). *(Rescoped INTO alpha 2026-07-13, now in M5.5: feats, spell scrolls, drop/pickup loop, gathering-node POIs + mining/fishing/lockpicking minigames.)*

---

## 4. Nostr saves & the two-client trust model

The endgame: **a save is a Nostr event** — signed by the player, stored on relays, portable across any client. Players own their save data; that's the point of building on Nostr. This section is the architecture those words imply. It fully lands at beta, but it constrains M1's schema design *now*.

### The trust model: official vs. modded, both first-class

- **Official client + official server** (run by you): anti-cheat lives *here*, not in the save format. The server records save **event IDs** and the action stream (the M1 event recorder) per npub. A save is admitted if (a) it's the latest event the server has seen for that character (revert detection), and (b) its state is plausible against the recorded action history. The backend stays authoritative during play; the client's job is to sign checkpoints.
- **Modded clients/servers**: explicitly embraced. Anyone can run any save — hand-built, edited, generated with a save-builder helper — on community servers and custom clients. Those characters simply aren't admitted to the official server. This turns "cheating" from a security problem into a *federation boundary*: your server validates; everyone else's freedom is the feature.
- **Design consequence**: never build a mechanic whose integrity depends on the save being unforgeable. Integrity comes from the official server's memory, not the file.

### Save semantics: deliberate, Pokemon/Fallout-style

- Saving = the player signs an event. It is **always explicit** — no background autosave, because (1) silent signing can't be guaranteed (NIP-07/Amber prompts; users will disable auto-approve, and should) and (2) auto-signing away your revert freedom is anti-player.
- **Revert freedom is accepted**: a player can always reload their last save if it hasn't been superseded. Mitigations available *only* where they matter: addressable/replaceable events mean relays naturally keep the latest per character; each save event can carry a `["prev", <event-id>]` tag forming a hash chain, so the official server detects forks/reverts cheaply. Beyond that — let people save-scum their single-player adventure; design penalties (death, M5) so accepting the outcome usually beats re-grinding the reload.
- Suggested event shape: a **parameterized replaceable (addressable) kind** in the 30000–39999 range — the save JSON's existing `d` field (character name) is already the d-tag, which suggests this was the plan all along. NIP-78 (kind 30078, app-specific data) is the zero-friction fallback; a dedicated kind is cleaner once the official relay whitelists it. Since most public relays reject unknown kinds and large events, the **official relay accepts the game kind** and players' own relays are opt-in mirrors.

### Size budget: the optimization ladder

Relay reality: many relays cap events around 16–64KB and reject unknown kinds. Target: **save content ≤ ~8–16KB** with headroom. Optimize in this order — each step only after the previous is exhausted:

1. **Hydration-first schema (M1, do now)** — store nothing derivable. Level ⇒ derived from XP + `advancement.json`. Quest points ⇒ derived from completed quest IDs. Quest availability, skill values, weight, AC ⇒ all derived. The save holds *player decisions and irreversible outcomes only*: identity, XP, HP/mana current (max derives), gold, inventory/equipment placements, known spells + slot assignments, active effects (runtime remainder only — already compact), discovered locations, quest progress, POI state, day/time/position (city/district/building/room).
2. **ID references, never names or copies** (already mostly true) — items/spells/effects by ID; never embed display data. Audit `Stats`/`Inventory` `map[string]interface{}` blobs for accidental fat.
3. **Short keys + tuple encoding** — `"d"`-style one/two-char field names; arrays instead of objects for repetitive structures: `["healing",3]` not `{"item":"healing","quantity":3}`; quest progress as `["the-rising-shadow",3]` (quest, stage index). This is the `future-nostr-save-optimization.md` pass; do it as a versioned serializer (`SchemaVersion`), not a hand-edit.
4. **Integer indexing** — today every item/spell/effect/quest reference in the save is its full text ID string (`"rope-hempen-50-feet"`), and those strings are the bulk of inventory bytes. Replace them with integers from a versioned index table shipped with game data (item #117; quest tuple becomes `[12,3]`). Plan for this — it's expected, not a contingency — but it lands only once the index-table versioning story is solid (the event must pin which table version it was written against).
5. **Compression last** — gzip+base64 inside JSON costs ~33% overhead on the encoding and kills relay-side inspectability; only if all else fails.

**Compaction demands a viewer.** A save full of short keys, tuples, and integer indexes is unreadable — unacceptable for data the player is supposed to *own*. The **save inspector** ships alongside the step-3 serializer: a user-facing view that hydrates any compact save back into a readable character sheet — name, level (derived live, proving the hydration rules), gear, spells, quest log, discoveries. Client page first ("view my save" in settings); eventually a paste-any-save-event web tool, which doubles as the modded ecosystem's companion to the save-builder and as your own support/debugging tool. A dev slice (CLI/Codex decode of a compact save) can land as soon as the serializer exists; the polished user-facing element is a beta deliverable. Encode and explain are two halves of the same feature — never ship one without the other.

Decision to make at beta: **public vs. encrypted saves**. Public (plaintext content) enables the fun stuff — inspecting friends' characters, community tools, the modded ecosystem — and there's nothing secret in a save. NIP-44 encryption is available if scouting/grief concerns appear. Lean public.

### Nostr beyond saves

- **Badges/achievements**: issued by the official server as **server-signed events** (NIP-58 badges) fed by the event-recorder stats — which is why lifetime counters don't need to live in the save. Modded servers can issue their own; provenance is the signature.
- **Stats/profile flair**: optional server-published "character card" events (separate d-tag), kept out of the save event entirely.
- **In-game messaging, MOTD, community boards**: standard notes/DMs surfaced in-game — cheap liveness once Grain lands.
- **Future player interaction (post-1.0)**: mechanically single-player saves + explicit **sync-and-commit sessions** for anything shared — Pokemon link-cable model. A trade = both players' servers agree on a trade event signed by both parties; each side's next save references it. Reverting past a trade becomes a detectable fork (the other side's save still points at the trade event). Design rule until then: no mechanic may assume a single global mutable truth about a character outside an active session.

---

## 5. Alpha → Beta

1. **Persistence becomes Nostr (§4 executed)**: Grain upgrade + mill login landed; save serializer at ladder step 3 (short keys/tuples) hitting the size budget, **with the save inspector shipping in the same release** (encode and explain together); dedicated kind on the official relay; `prev`-chain + event-ID tracking live on the official server; the raw `session/update` endpoint deleted; plausibility validation (stats/gold/items vs. recorded action history) on save admission.
2. **Content depth**: main quest act 1 complete (the-rising-shadow → the-shadows-source chain w/ prerequisites), 2–3 side quests per town, class+race quests live, POI coverage so every environment has 2+ discoverables, dailies/weeklies pools doubled.
3. **First story set pieces** (§8): vignette engine + 2–3 authored beats in the main quest.
4. **Balance & economy pass** from alpha telemetry: XP curve, gold faucets (shows, dailies) vs sinks (inn, vault rites, components).
5. **Systems debt**: conditions complete, monster scaling real, short/long-rest combat rules, SSE or batched ticks, mobile landscape pass, room art/flavor where it earns it, **player map screen** (Codex track C3 derivation filtered to discovered locations, cardinal-direction layout).
6. **Spell scroll crafting** (full design: `docs/draft/spell-scroll-system.md`): location-gated crafting where casters inscribe scrolls (recipes filtered live by class/level/INT/station — no recipe tracking in the save), INT-scaled success chance with component+gold loss on failure, scrolls usable in combat for mana and cross-class, stored in a spellbook container. Builds on M4's casting engine; still needs the ~20 recipe + scroll-item JSONs, `crafting_stations` data on the wizard-tower/temple/grove locations, and its missing components + monster drops authored. **Reconcile before building**: the draft gates on raw INT, but alpha introduced a *Crafting skill* governed by the new *Crafting Kit* — decide whether scroll crafting keys off that skill/kit or stays INT-only, so the game has one crafting model, not two.
7. **No-wipe commitment** from beta day one — saves are now player-owned events; "wipe" stops being something you *can* do unilaterally, which is the point.

## 6. Beta → 1.0

Act 1 fully art-dressed (set pieces, city/environment finals, NPC portraits) · hand-styled world map replacing the derived one (same graph data underneath) · audio (music system already tracks unlocks; add SFX layer) · NIP-58 badges live off the event recorder · onboarding polish layered on the alpha tutorial · performance/load test · security review of auth + save admission · launch marketing through the Nostr ecosystem (the native audience — "your character is yours" is the headline feature). Magic items and feats are post-1.0 live content, as originally planned.

---

## 7. UI/UX critique (current build, file-specific)

**The core layout idea is right** — scene as centerpiece, persistent character rail, console-style window. Scenes are deliberately non-interactive flavor art; interaction belongs to the button UI and room navigation, so the critique focuses there. The problems are execution-level:

1. **Non-uniform stretch distorts everything.** `scaleGameUI()` (game.html) scales X and Y independently to fill the viewport, so on any non-1.503 aspect ratio the pixel art is stretched and text is smeared. Pixel-art games live and die on integer scaling. Fix: uniform scale = `min(vw/W, vh/H)`, floor to integer when ≥1, letterbox the rest. (One-hour fix, huge perceived-quality gain.)
2. **The base canvas is too small for its information density.** 756×503 carrying: 7px log, 6px stat labels, 7px dialogue buttons, 8px headers. Even *after* scaling, non-integer multiples of 6–7px fonts render mushy. Re-basing at 960×540 (16:9) buys ~60% more pixels, integer-scales cleanly to common displays, and lets the floor be 10px Dogica. This is the most invasive UX change (touches every template), which is why it's scheduled inside M6 rather than "someday."
3. **Narrative is presented in the worst surfaces.** NPC speech → transient top-right toast; flavor/system text → 209px-wide 7px log; quest text → nowhere. The player's eye is on the 347px scene; nothing narrative ever appears there. The M6 fix: speech moves into a dialogue box on the scene's lower third (portrait + name + readable text), while the **option buttons stay in the bottom strip where they already work** — that interaction pattern is keeper; the overlay just needs restyling to the win95 language (today it's generic Tailwind gray with a yellow border and tan buttons — functionally right, visually foreign). One speech renderer then serves NPC talk, POI nodes, and encounter vignettes. Tavern barkeeps are your most-written content — let people read them.
4. **The bottom strip overloads.** Travel D-pad + Buildings + People in 125px works until a district has 5 buildings and 4 NPCs, then it scrolls invisibly. The M2 room model is the structural fix (People = current room only; Buildings column becomes Rooms inside); add count badges ("Common Room (3)"), consistent ordering, hover names.
5. **Right rail: the layout is right, the content is thin.** The tab structure stays as-is (deliberate — buttons are where they belong). The gaps are *inside* the tabs: the quest log is a hardcoded placeholder (M3 fixes it), the Equipment tab shows your gear but none of the stats the gear produces (AC, attack, damage belong right there on the page), and the Stats tab — which already carries abilities, skills, and effects — is missing the headline character numbers (level, XP-to-next, proficiency, AC, carry weight).
6. **Moments don't land.** Level-up is a log line; death warp is silent; discovery is a toast; saving is a browser `confirm()`. Each deserves a scene-level interstitial (the combat overlay's dice/flair work proves the pattern). Cheap, and it's where "game feel" comes from — and in a deliberate-save game, the save moment itself should feel like a ritual, not a chore.
7. **No-op affordances erode trust.** The spellbook attack button that does nothing, "Badges coming soon," undefined equip-in-combat — every dead control teaches the player to stop clicking. The M5 interaction matrix plus the standing polish rule exist to drive these to zero before strangers play.
8. **Mixed UI idioms.** Native `confirm()` dialogs and emoji-in-text sit inside a win95 pixel aesthetic. Pick the aesthetic everywhere (the win95 modal style already exists — use it for save/logout/death).
9. **Good bones worth keeping**: time-of-day icon + day counter on the scene, radial hunger/fatigue icons, class-colored resource bar, GROUND button, the combat overlay's paced log. The style direction (dark win95 + pixel art) is distinctive — the issue is legibility, not identity.

---

## 8. Content & liveness strategy

Two tracks, exactly as you framed it: a slow-burn authored spine, and renewable systemic content that makes the world feel alive *now*.

**Track 1 — Story set pieces (slow, art-led, beta→1.0).**
The intro (full-bleed scenes, Caveat letter, paced text) is the quality bar. Make it reusable instead of one-off: a `vignette` stage/node type — `{scenes: [{image, text_beats[], choice?}]}` — rendered by a generalized intro engine. Quests trigger vignettes at act boundaries; everything between is normal dialogue/POI play. Pipeline per set piece: write beats → block with grayscale/pixellab placeholder → ship → replace with final art when ready. Budget 1–2 per beta milestone; never block a release on final art. The 15 intro scene images took real effort — that's the cadence reality, hence: alpha gets zero set pieces, beta gets the engine + 2–3, 1.0 finishes act 1.

**Track 2 — Renewable content (alpha, systems-led).**
- **Dailies** (reset once per *real-world* day): bounty/fetch/delivery/survey templates from the pool — 6 minimum so a week doesn't repeat. Payout: gold + QP + occasionally a consumable. Real-time cadence is deliberate: it's the "log in today" retention rhythm, decoupled from the 144× game clock.
- **Weeklies** (reset per *real* calendar week): multi-stage, tougher (recommended-danger High), QP-heavy, one guaranteed-interesting reward. 4 minimum.
- **Encounters**: the per-environment pools make *travel* the heartbeat — merchants, brawlers, thieves, vignettes. Cooldowns prevent spam (design doc §5 defaults).
- **POIs**: persistent discoverables with 3-in-game-day refresh — the "I found a thing" loop, plus Metroidvania item-gating (rope/crowbar/keys) that makes mundane gear interesting.
- **Quest points** as the meta-currency (always derived from completed quests): walls off higher-tier quests, later feeds badges and reputation.
- **Schedules + rooms + locks**: NPCs already keep hours and (after M2) keep *places* — the cook in the kitchen at noon, the bard in the common room at night, night-only NPCs, market-day stock changes, a back room that only unlocks after dark or behind a key from a quest. Cheap world-aliveness from systems you already built.
- **Nostr liveness (beta+)**: MOTD notes, badge drops announced in-game, community board events — §4's "beyond saves" list doubles as a liveness roadmap.

---

## 9. Cleanup list (code hygiene, fold into milestones)

- `CLAUDE.md` rewrite (M0) — current doc describes a repo three refactors ago
- Delete root `nul` artifact; `go mod tidy`; delete dead `legacy-registry.html`/old script comments if truly retired (M0)
- Gate `POST /api/session/update` raw overwrite behind debug (M8); delete it at beta when the §4 validation layer replaces it
- `docs/draft/` triage: `option-4-delta-architecture` shipped → move to docs/development; `future-nostr-save-optimization.md` superseded by §4 (fold its field-shortening tables into the step-3 serializer work); xlsx archives → note-and-archive
- Frontend `state/gameState.js` hidden-DOM cache: already write-through to Go; finish the migration path (read-only render cache) opportunistically during M6 — don't make it a project
- Standardize the win95 styling currently duplicated as inline `style=` strings everywhere into CSS classes during the M6 re-base (it will make the canvas re-base mechanical instead of painful)
- Tests: combat math (attack/damage/range), the M3 requirement evaluator, the M1 level-up table, and **save serialize→deserialize round-trip per SchemaVersion** are the four highest-value `tests/` targets — pure functions, and they're the things that silently break

## 10. Risks

1. **M3+M4 are both L-sized and both "the game."** Resist interleaving them; finish the quest engine before the casting engine. Half of each = nothing playable.
2. **Polish debt compounds.** Every milestone ships new surface area on top of systems whose edges are already undefined. The 20% polish rule and the M5 interaction matrix are the dam; if they slip, alpha feedback will be 80% "this button does nothing" instead of insight you can use.
3. **Save-schema drift is permanent debt.** Every field added to `SaveFile` is future relay bytes signed into players' events forever. Make the hydration test (§4 step 1) a review habit for *every* PR that touches `types/save.go` — it's much cheaper to refuse a field now than to migrate signed events later.
4. **Content refs will drift while systems are built** — keep `--check-schema` in the loop (pre-commit or Makefile gate) so M7 doesn't regrow to 116 errors.
5. **The canvas re-base touches everything** — do it as one focused pass (M6), not incrementally, or you'll live in a half-migrated layout for months.
6. **Solo-dev scope creep**: the schema supports far more than alpha needs (alignment gates, districts, deterministic shuffle, per-room everything). Build to the 11 drafted quests, not to the schema's ceiling.
7. **Grain/mill timing**: if the auth swap lands mid-alpha, session identity (npub-keyed saves/sessions) is the blast radius — freeze an interface (`auth.Identity`) before M8 so the swap is a module replacement, not surgery. Signing UX (NIP-07 prompts vs. Amber round-trips) directly shapes the §4 save ritual — prototype the "save = sign" flow early in the beta cycle, not at the end.
