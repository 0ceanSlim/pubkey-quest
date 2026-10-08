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

## Design decisions

1. **Saves stay single-player.** Party membership and party combat state are memory-only.
   Matches the roadmap §4 "sync-and-commit sessions" rule.
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

## Phase 1 — Multi-entity combat engine (single human, N enemies)  ← implemented, awaiting playtest

No networking. Makes the engine N-vs-M with the one player still `Party[0]`.

**Status (2026-10-08):** implemented. Engine:
- `combat/grid.go`: positions, occupancy, targeting, placement, AI target choice.
- `combat/turns.go`: initiative cursor, round = one full pass, monster turn loop,
  death-save loop.
- `encounter/group.go`: 5e XP-budget group difficulty.

`StartEncounter(EncounterSpec{MonsterIDs, EnvironmentID, Surprise})` is the entry
point; `StartCombat` is kept as the single-monster wrapper. POI `count`/`surprise` are
wired, and `/api/combat/start` plus the debug start (with a count picker) accept a count.

UI: target panel, clickable turn-order strip, multi-token grid with per-monster move
animation, click-to-target cells.

Tests: `combat/multi_entity_test.go`, `tests/combat/multi_entity_test.go`,
`tests/encounter/group_test.go`.

Behaviour changes worth knowing:
- No combatant ever shares a cell; dungeon "range 0 contact" spawns now start adjacent.
- Unarmed reach is 1. It was 0, which made unarmed effectively unusable.
- Ranged disadvantage now applies at range ≤ 1.
- Loot from earlier kills survives a later monster fleeing. A player flee still
  forfeits it.
- Round counts full turn-order passes, not actions.
- Hold and Dodge last until your next turn instead of one monster's turn.
- Intimidating Roar hits every enemy.

Not done in P1: AoE spells (single target only), flanking / Pack Tactics, and monsters
approaching a downed player (only those already in range attack during death saves).

Group composition: the encounter rollers (`encounter.Roll` for biomes, POI nodes,
authored encounters) decide **what** a fight contains. `encounter/group.go` only
**rates** a given group. Biome encounters still roll one monster; rolling packs there is
a later tuning call.

## Phase 2 — Nostr identity + saves on relays  ← next (the big lift)

Reordered 2026-10-07: Grain comes before any multiplayer, because presence, friends,
parties and trading all stand on it.

- **G1 — proof-of-key login + cookie-bound identity: DONE on main** (`c8cb355`;
  grain v0.8.1 in `04e187b`).
- **Identity** (profiles, user dropdown, NIP-65 relay editing, pinned-relay mode,
  NIP-89 client tag) is owned by `docs/draft/nostr-identity-plan.md`.
- **Saves on relays is alpha scope** (decided 2026-10-07): a browser-signed addressable
  event with a `["prev", id]` chain, server admission by event id, and the
  ~8–16 KB size budget. See `docs/draft/future-nostr-save-optimization.md`. It needs
  Grain's outbound tag/since filter encoding fixed to fetch by `d`.

## Phase 3 — Presence

- Per-session mutex (or an immutable presence snapshot published on each tick) to make
  cross-session reads race-free.
- Idle eviction; "online" = heartbeat within N seconds; collapse multiple saves per npub.
- Presence index: spot key (normalized `GroundKey`) → online sessions. Filter
  `|absTimeA − absTimeB| ≤ 60`. Add `Room` to the snapshot/delta, plus a `players`
  added/removed delta.
- The people list renders players with a distinct button style. Clicking opens a player
  card: profile, level/class, and actions Invite to party / Trade (disabled) / Inspect.

## Phase 4 — Friends list + party formation

- Friends = mutual follows (kind 3 both ways). Show online/offline, and location if
  within the time window.
- **UI:** Friends replaces the Badges button; Badges moves to a tab inside the Quests
  area (a stub until the NIP-58 work).
- Party lifecycle (memory-only): invite → accept/decline → leader → leave/kick → promote →
  disband. Invites only go to mutual friends or players present at your spot.
- **Party size cap: 5** (D&D's usual 4–5). Partners, when they arrive, count toward it.
- **Party clock snaps to the leader.** Leaving keeps the party's time (no rewind).
- The leader directs movement/travel; followers' location and clock are slaved to
  the leader.
- Party HUD: member portraits, HP, leader crown. Party chat is optional, later via DMs.

## Phase 5 — Multiplayer party combat

- A `Combatant` sheet built from any member's `SaveFile`: engine functions take
  actor/target sheets instead of "the" save. This is the coupling P1 left in place.
- One `CombatSession` shared by all member sessions; each member's turn is gated on its
  owner.
- Turn timer + AFK auto-hold; a disconnected member holds.
- Outcome applied per member into each save: HP, XP split rules, loot (decide).
- Ally-aware rules: flanking / Pack Tactics, Sneak Attack "ally adjacent", Help,
  heal/buff allies, AoE with friendly fire.
- Scaling: `encounter.GroupDifficulty` already takes party size. Boss stat blocks get
  per-player HP scaling + legendary actions / phases.
- Dungeons that need multiple disciplines, e.g. a ward only a caster can break while a
  martial holds a door. Authored via POI requirement nodes per member.

## Phase 6 — Quest tab rework (parallel track, UI only)

Can run any time. Needs its own scoping pass: tracking pins, map hints, party-shared
quests?, abandon button, Badges sub-tab.

## Post-beta — NPC partners (earmarked)

For when nobody else is online. Deferred 2026-10-07: multiplayer comes first, and
partners need their own companion AI.

- A **select few** partners, each favouring different things, each with its own **join
  requirements**.
- **Favor:** acting against what a partner values while they're with you breaks it — they
  leave, and you must regain their favor before you can invite them back.
- Permanent recruits: IDs in the save, everything else derived.
- Can travel alongside a party of real players; they count toward the 5 cap.
- Builds on Phase 5's `Combatant` sheet. Content goes in a `partner` block on NPC JSON
  (validated by `--check-schema`), plus `recruit` / `dismiss` dialogue actions.

## Later (out of scope here)

- **Trading:** a dual-signed trade event; each side's next save references it
  (roadmap §4).
- **Shared multiplayer zones:** beyond per-spot presence.
- **PvP arena: in the Sunscorch Desert** (decided 2026-10-07).
  - **Entrance:** one of **Dusthaven's south exits** leads to the arena. Dusthaven is the
    oasis "last outpost before the great desert".
  - **Model:** RuneScape 2's Duel Arena — a PvP zone with challenges, duel rules,
    **staking** and gambling on the outcome.
  - **Builds on:** Phase 5 shared combat (two sides) plus Trading.
  - **Stakes:** escrowed as a dual-signed stake event before the fight, then settled by
    the server into each side's next save, so neither player can revert out of a loss.

---

## Open questions

- XP/loot split in party combat: even split, per-kill, or per-member rolls?
- Turn timer length and AFK behavior.
- Whether biome encounters should roll packs (P1 made it possible; it's a tuning call).
