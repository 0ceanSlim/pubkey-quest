# Party, Multi-Entity Combat & Multiplayer — Plan

Status: **DRAFT / proposed** (2026-10-07). Supersedes the "party/companions — not in alpha"
line in `docs/roadmap.md` §(Explicitly NOT in alpha) *if adopted*; roadmap must be updated
when the rescope is confirmed.

Each phase below is meant to be its own implementation plan + commit series. Order is
dependency order: nothing in a later phase is needed by an earlier one.

---

## 0. Where we are (findings, 2026-10-07)

**Combat** — the *shapes* are multi-ready (`CombatSession.Party []`, `Monsters []`,
`Initiative []` in `types/combat.go`), but everything that runs on them is 1v1:

- ~90–100 hard-coded sites: `Monsters[0]` / `Party[0]` / scalar `PlayerPos` / `MonsterPos` /
  `currentRange()` across `combat.go`, `ai.go`, `cast.go`, `abilities.go`, `conditions.go`,
  `api/game/combat.go`, `combatSystem.js`, `combat-overlay.html`.
- No turn loop: `CurrentTurnIndex` is never read. "End turn" = run `Monsters[0]` once.
  `Round++` happens in API handlers on every action (it's an action counter, not rounds).
- `InstanceID == template ID` → two goblins collide.
- First kill sets `Phase="loot"` → combat ends on the first death.
- No target param on any action; AI always targets `Party[0]`.
- Deepest coupling: every engine fn takes one `save *types.SaveFile` as "the" player.
- POI steps already carry `Count` and `Surprise` (`types/poi.go:114`), used by drafts
  (watchtower ×2, crypt ×4) but ignored at runtime.
- Difficulty guardrail rates one monster's CR; no group budget, no scaling.

**Session / presence**

- Sessions keyed `npub:saveID`, never evicted; `UpdatedAt` bumps every tick (usable heartbeat).
- No push channel — client polls `update_time` every 417 ms; deltas ride the response.
- **Game time is per-save and client-driven**; no world clock. Absolute time =
  `CurrentDay*1440 + TimeOfDay`.
- People list = `GetNPCIDsAtLocation` (stateless REST) + hourly `NPCsAtLocation` delta cache.
  `Room` is not in the snapshot/delta.
- `world.GroundKey()` = `Location|District|Building|Room` → ready-made "same spot" key.
- **Security gap:** `/api/game/action`, `/api/session/*`, `/api/combat/*` trust `npub` from
  the body; the Grain cookie is only checked on the page route. Must be fixed before any
  player can see/affect another.
- **Concurrency gap:** no per-session lock; reading another player's `SaveData` while their
  request mutates it is a data race.

**Nostr / Grain**

- `go.mod` pins `grain v0.8.0-rc4`; **v0.8.0 final is released**.
- Server initializes Grain's core client + relay pool but no game code uses it.
- No kind-3 (follow list) fetching anywhere. Profiles use a hand-rolled websocket fetch
  (`utils/fetchUserMetaData.go`) instead of Grain.
- Frontend: `nostr-mill` for login/signing; no relay pool or subscriptions client-side.

**UI**

- "Badges" button is a stub (`showMessage("coming soon")`); no NIP-58 code exists.
- Quest tab is the M3 journal (`questlog.html` + `src/ui/questDisplay.js`), category
  sections + guide modal; abandon has no UI.

**NPCs** — no recruitable/companion marker; dialogue actions are a switch in
`npc/dialogue.go:197` → a `recruit` action slots in naturally.

---

## Design decisions (proposed)

1. **Saves stay single-player.** Party membership, party combat state, and partner
   *assignment for this session* are memory-only. What persists per save is only what the
   hydration rule allows (e.g. recruited partner IDs if partners are permanent — derived
   where possible). Matches roadmap §4 "sync-and-commit sessions" rule.
2. **Party clock = leader's clock.** While partied, followers' `CurrentDay/TimeOfDay` are
   slaved to the leader (server sets them; client `smoothClock` follows the delta). The
   ±1 h presence window only governs *who you can see/invite*; once joined, you snap.
   Leaving the party keeps you at the party's time (no rewind).
3. **Leader directs movement/travel.** Followers' location tracks the leader server-side;
   follower movement/travel actions are rejected while partied (except leave).
4. **Combat is one shared `CombatSession`** owned by the party, referenced by every member
   session. Each `PartyCombatant` points to its member's session (or to an NPC partner
   sheet). Outcome (HP, XP, loot, death) applies per member into *their own* save.
5. **Turn model:** true initiative order over all combatants. A player's turn waits for
   that player (with a turn timer + auto-hold for AFK). Monsters/partners run server-side
   when the cursor reaches them. No simultaneous turns.
6. **Transport:** keep polling for now — party/combat state rides the existing 417 ms
   `update_time` delta. SSE is a later optimization, not a blocker.
7. **Friends = mutual follows** (kind 3 both ways), fetched server-side via Grain v0.8.0,
   cached like profiles.

---

## Phase 1 — Multi-entity combat engine (single human, N enemies)  ← start here

No networking. Makes the engine N-vs-M with the one player still `Party[0]`.

- Per-combatant position on every combatant; drop scalar `PlayerPos`/`MonsterPos`
  (keep them as derived fields in the response for one release if the UI needs it).
- Unique `InstanceID` (`goblin#1`, `goblin#2`); per-instance initiative.
- Real initiative cursor + `advanceTurn()` loop in the engine; `Round` increments when the
  cursor wraps (move `Round++` out of handlers).
- `target_id` on attack/cast/ability/use-item; range computed per (actor, target).
- AI: target selection (nearest / lowest HP / threat, data-driven priority), occupancy-aware
  stepping (no stacking), OA/readied attacks per pair.
- Victory = all hostiles dead/fled; XP + loot aggregate across kills.
- Difficulty: group XP budget (5e encounter math: summed XP × count multiplier vs party
  thresholds) replaces single-CR rating.
- Wire POI `Count` and `Surprise`; authored encounters can list multiple monsters.
- Frontend: multiple monster tokens on the grid, target selection (click token),
  initiative strip, per-monster HP list replacing the single monster panel.
- Tests: initiative ordering, victory-only-when-all-dead, targeting, group difficulty,
  no two combatants on one cell.

## Phase 2 — Combatant abstraction + NPC partners

Removes the "one `*SaveFile` = the player" coupling so a non-player ally can fight.

- Introduce a `Combatant` sheet interface (stats, AC, attacks, spells, resource pool,
  conditions) built from either a `SaveFile` or an NPC partner definition; engine fns take
  the actor/target sheets instead of `save`.
- Partner content schema: `partner` block on NPC JSON (combat stat block or class+level,
  recruit requirements, dialogue lines, leaves-at conditions). Validated by `--check-schema`.
- `recruit` / `dismiss` dialogue actions; partner follows the player out of combat
  (session-only, or persisted as an ID list if partners are permanent — see open questions).
- Partner AI turns (reuse monster AI with ally targeting; optional simple commands later:
  attack my target / defend / hold).
- Ally-aware rules: flanking/Pack Tactics, Sneak Attack "ally adjacent", Help action,
  heal/buff targeting allies, AoE friendly-fire.
- Scaling: encounter budget uses party size/levels.

## Phase 3 — Grain v0.8.0 + auth hardening + presence

Prerequisite for anything that lets players see each other.

- Bump `grain v0.8.0-rc4 → v0.8.0`; fix API changes; migrate profile fetch onto Grain's
  client (retire `fetchUserMetaData.go`).
- **Bind identity**: every game/session/combat handler derives npub from the Grain cookie
  session (`auth.GetCurrentUser`) and rejects mismatches.
- Per-session mutex (or immutable presence snapshot published on each tick) to make
  cross-session reads race-free.
- Idle eviction / "online" = heartbeat within N seconds; collapse multiple saves per npub.
- Presence index: spot key (normalized `GroundKey`) → online sessions; filter
  `|absTimeA − absTimeB| ≤ 60`; add `Room` to snapshot/delta; `players` added/removed delta.
- People list renders players with a distinct button style; clicking opens a player card
  (profile, level/class, actions: Invite to party / Trade (disabled) / Inspect).

## Phase 4 — Friends list + party formation

- Fetch kind-3 for self and candidates via Grain; friends = mutual follows. Cached with TTL.
- **UI:** Friends replaces the Badges button (online/offline, location if in-window);
  Badges moves to a tab inside the Quests area (stub until NIP-58 work).
- Party lifecycle (memory-only): invite → accept/decline → leader → leave/kick → promote →
  disband; invites only to mutual friends or players present at your spot.
- Leader-driven movement/travel; follower location + clock slaved to leader.
- Party HUD: member portraits, HP, leader crown; party chat optional (later via DMs).

## Phase 5 — Multiplayer party combat

- One `CombatSession` shared by all member sessions; each member's turn gated on its owner.
- Turn timer + AFK auto-hold; disconnect → member becomes AI-controlled or holds.
- Outcome applied per member into each save (HP, XP split rules, loot rolls per member or
  round-robin — decide).
- Scaling: same budget math as Phase 2 using human party size; boss stat blocks with
  per-player HP scaling + legendary actions / phases.
- Dungeons: POI chains that require multiple disciplines (e.g. a ward only a caster can
  break while a martial holds a door) — authored via POI requirement nodes per member.

## Phase 6 — Quest tab rework (parallel track, UI only)

Can run any time; independent of combat. Needs its own scoping pass (what "rework" means:
tracking pins, map hints, party-shared quests?, abandon button, Badges sub-tab).

## Later (out of scope here)

- **Trading** — dual-signed trade event; each side's next save references it (roadmap §4).
- **Shared multiplayer zones** — beyond per-spot presence.
- **PvP arena** — sanctioned duels; reuses Phase 5 shared combat with two parties.

---

## Open questions

- Is party/multiplayer now **alpha scope** (rescope roadmap), or post-alpha after M6–M8?
- Partners: **permanent** recruits stored in the save (ID list) vs **session hires**?
- XP/loot split in party combat: even split / per-kill / per-member rolls?
- Turn timer length and AFK behavior.
