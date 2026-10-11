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

## Phase 2 — Nostr identity + saves on relays  ← identity shipped; saves on relays still to do

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

## Phase 3 — Presence, the friends panel and chat  ← next

Designed with the maintainer 2026-10-11. Three ways players show up, one panel to manage
them, and chat in the log.

### Decisions

- **Two different lists.**
  - **Players online** is how you meet strangers. Presence shows everywhere, as a light
    headcount, plus an Area list.
  - **Friends** are your mutual follows on Nostr (you follow them and they follow you).
    From the friends list you message them and coordinate where to meet.
- **No teleporting, no travel-to-friend.** To party, players must physically be at the same
  place. Friends arrange it by whispering ("meet me at the Salty Anchor").
- **Party clocks sync on joining:** the follower's clock snaps to the leader's (decided
  2026-10-07). Before a party, everyone's clock is their own.
- **No hubs.** A Grand-Exchange-style market later becomes the natural gathering spot.
- **Chat lives in the left log,** with an input line under it. Whisper is a real Nostr DM.

### Where players appear

| Surface | Who | Rule |
|---|---|---|
| **People column** (with the NPCs) | players in your exact spot | same location / district / building / room, **and** in-game clocks within **60 minutes**. A player can wait to line up with someone they see is close. |
| **Headcount** (small, unobtrusive, on the main screen, e.g. `👥 4`) | players online in your current location (city or environment) | any district, any clock. Clicking it opens the Friends panel on its **Area** tab. |
| **Friends panel → Area tab** | the same players as the headcount | each with district/building and clock offset ("Salty Anchor · 40 min ahead"), so you can find them |
| **Friends panel → Friends tab** | your mutual follows | in-game: online, where, clock offset. Not in game: Nostr activity (below). |

**Friends' status when they aren't in the game:** their most recent Nostr event tells you
something.
- Its NIP-89 `client` tag says where they are: *"active on Primal · 12 min ago"*.
- With no client tag, fall back to *"last seen 3 h ago"*.
- Grain fetches each friend's latest event (one per author), cached for a few minutes.

### The 😊 Friends panel

The bottom-row 😊 opens it, as do the headcount (straight to Area) and the player card's
"friends" link. It has two tabs:
- **Friends:** online friends first, then the rest by most recently seen.
- **Area:** everyone online in this location.

**Right-click / long-press on a name** opens a context menu. Each item applies where it
makes sense:
- **Whisper:** opens a whisper to them in the chat line.
- **Invite to party:** enabled only when they're at your spot.
- **Follow / Unfollow:** publishes your updated Nostr contact list (kind 3). Following
  someone who follows you back makes them a friend.
- **View profile:** their Nostr profile card plus their character (name, level, race,
  class).
- **Mute:** adds them to your Nostr mute list (kind 10000), which also hides their chat.

Clicking a player in the People column opens the same menu.

### Chat in the log

Messages appear in the left log, colour-coded by channel. A one-line input under the log
has a channel picker.

| Channel | Reaches | Carried by |
|---|---|---|
| **Say** | everyone who can see you in the People column (same spot, clocks within 60 min) | the server: in memory, never stored |
| **Area** | everyone online in your location | the server |
| **Whisper** | one person (a friend, or anyone from Area/People) | **a real Nostr DM** (NIP-17, sealed and gift-wrapped in the browser by your signer, published to their inbox relays). It shows up in their other Nostr clients too. |
| **Party** | your party wherever they are | the server (Phase 4) |

- **Delivery:** new Say/Area/Party messages ride the presence poll (every few seconds,
  paused or not). Incoming whispers are fetched from your inbox relays through Grain and
  decrypted in the browser.
- **Safety:** a per-player rate limit, a message length cap, and your Nostr mute list (it
  carries over from other clients).

### Build slices

1. **Presence core.** The registry is written and tested (`cmd/server/presence`).
   - Every action publishes the player's snapshot (character, spot with the default room
     filled in, game minute, in combat).
   - `GET /api/players/here` and `GET /api/players/area` sit behind the identity gate, and
     the client polls one of them as its heartbeat.
   - Players appear in the People column with their own colour.
   - Clicking one opens the player card / context menu: View profile works; the rest are
     stubbed.
2. **Headcount + Friends panel shell + Area tab.** The 😊 button stops being a stub.
3. **Friends tab.**
   - Mutual follows via Grain: your kind 3, plus the kind-3 lists that tag you.
   - Their in-game status, and Nostr "active on client · ago" from their latest event.
   - **Follow / Unfollow** with a safe kind-3 update: fetch the newest list first and
     append, so it never clobbers follows made in other clients.
4. **Chat: Say + Area.** A server-side message bus, the log input and channel picker, rate
   limits, mute.
5. **Whisper.** NIP-17 send and receive with the browser signer (MILL's `nip44`), published
   through `/api/publish`.

## Phase 4 — Parties

- **Invite** from the context menu, only when the invitee is **at the same spot**. There's
  no remote join: friends meet up first. The invite reaches them through the poll and
  appears in their log with Accept / Decline.
- **Joining snaps the follower's clock to the leader's.** From then on the leader's
  movement and travel carry the party; members can't move on their own except to **Leave**.
- **Lifecycle (memory-only):** invite → accept / decline → leave / kick → promote →
  disband. Logging out leaves the party.
- **Cap: 5** (partners count when they arrive post-beta).
- **Party chat** channel. **Party HUD:** member names, HP, a leader crown.
- **Combat together is Phase 5.**

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
