# Competition — the build plan

Status: **PLAN, 2026-11-21. Nothing built.** The last beta gate: the level and XP leaderboard,
the 43 badges and the highscores. The design is [server-stats.md](server-stats.md) (the store,
the DM, Gamestr) and [badges.md](badges.md) (the list and the art, all 43 drawn). This file is
the order to build it in, and what it touches.

## What the code already has

| Piece | Where | Use |
|---|---|---|
| The official save record | `cmd/server/ledger` (`ledger.Record` in `api/relay_saves.go`) | the moment a save counts on official |
| The disk save | `api/game/session.go` `SaveSessionHandler` | the moment a save counts on test |
| The event recorder | `game/events` (`MonsterKilled`, `FishCaught`, `TurnedIn`, …) | the store's input |
| Quest completion | `game/quest` `FinishNow` + the repeatable path | badges, dailies count |
| The crash journal | `session/journal.go` | pending stats survive a restart |
| Signing and publishing | grain `core.EventSigner`, `UserContext.PublishEvent` | the DM's events |
| The DM's key, relays, media | `utils.DMConfig`, `systems/badge-media.json` | who signs, where it goes, which images |

## Two changes to the draft (your calls, 2026-11-21)

### The relays are the store
Everything the store keeps is published, signed by the DM, so the relays hold the record and
the server only keeps a copy:

| Record | Event (signed by the DM) |
|---|---|
| A player's lifetime stats: counters, sets, bests | one addressable **kind 30078** per player, `d` = `pubkey-quest:stats:<player hex>`, replaced at each kept save |
| A badge earned | **kind 8** award (NIP-58) |
| A score | **kind 30762** (Gamestr) |

- **The server's copy** is one small JSON file per player under `data/stats/`, like the save
  ledger: fast to read at a save, and every board is built from these files in memory at
  start. **It's a cache.** If it's lost, the server rebuilds it from the DM's events on his
  relays (the newest `created_at` wins).
- **A commit** writes the copy first, then publishes. A relay that's down just means a retry,
  and a replaceable event that arrives twice changes nothing.
- **The game relay** (the embedded Grain at `/relay`, [game-relay-plan.md](game-relay-plan.md))
  becomes the first relay all of this goes to once it's built.
- **Public by design:** the stats event holds only what the scores already show (fish caught,
  dailies done, monsters slain), so nothing new is exposed.
- So the draft's SQLite `stats.db` is dropped.

### Test plays badges, unsigned

Test is the production nightly, and it never signs (your rule). Under the draft as written, it
would never *show* badges either, so you couldn't playtest them before launch. So:

- **`stats.enabled` runs the store** on any server that sets it: test and official both earn
  badges and post scores **in the game**.
- **Signing and publishing happen only with `saves.validation` on** (official). Test's badges
  and scores never leave the server: no kind 8 awards, no 30762 scores, no Gamestr.
- Test keeps only the server's copy (`data/stats/` on the test host), with nothing on relays,
  so nothing earned on test can reach official.

## Slices

Each slice is its own commit, tests first, and the game plays the same with `stats.enabled` off.

### 1. The store and the commit gate — BUILT 2026-11-21

`cmd/server/stats` (pending per session, `Commit`, the record file), committed from the disk save and
the relay save, journaled, `stats.enabled` in config. Publishing the record is slice 5.

- `cmd/server/stats`: a player's record (counters, sets, bests, badges, scores), kept as
  `data/stats/<npub>.json` and, on official, published as the DM's kind 30078. Rebuilding it
  from the DM's events when the file is missing.
- **Pending per session**, keyed by npub + save id; journaled with the session (like its
  ground). Dropped on load.
- **Commit at a kept save**: after `ledger.Record` on official, after the disk save on test. One
  transaction applies the pending stats, then the badge rules and boards run (slices 3 and 4).
- Tests: a reload drops; a save applies; five characters do one daily and it counts once; a crash
  restores the pending stats from the journal.

### 2. Producers — BUILT 2026-11-21

Built simpler than planned: the places that matter call the store directly (`stats.Count`,
`CountOnce`, `AddMember`, `Best`, keys in `stats/keys.go`) instead of new event kinds and a
translating consumer. Fish land in `addCatch`; kills and the kill's rough gem in
`combat.killRewards`; dailies and weeklies in `quest.markRepeatableDone`; pearls and geodes in
`inventory`; mined gems and ores through `noteFound` (the quick face, Veins, and POI loot, so a
cache's gem counts as found too). A best remembers the character who set it.

- New events: `QuestCompleted` (target = quest id, from `FinishNow` and the repeatable path),
  `GemFound` (kills, rock faces, Veins cells, shucking), `OreMined`.
- `FishCaught` gains the catch's weight (the `lb` slot key). `MonsterKilled` already names the
  monster, whose `xp` gives the toughest-kill board.
- Store effects (server-stats.md §9): fish species set, fish landed, biggest per species (in
  ounces), gem types, ores, dailies per day per npub, monsters slain, toughest kill.

### 3. Badges — BUILT 2026-11-21

`game-data/systems/badges.json` (the 43, migrated with `badge-media.json`), `game/badges` (`Judge`,
the catalogs from game.db), judged inside `stats.Commit` so the record and its new badges are
written together, against the save as it was kept (a relay save: as it was signed). Every kept
save is judged, even with nothing pending. Save responses carry `badges_earned`.
`--validate` checks rules, rims, art on Blossom and in the game, and warns on a quest with no
badge.

- `game-data/systems/badges.json`: the 43, each with id, name, description, rim and rule.
  `--migrate` loads it with `badge-media.json`. `--validate` checks every rule names a real
  quest, set or counter, and warns when a main, side, class or race quest has no badge.
- Rules run at commit, against the store and the save just kept. A new badge is a
  `badge_award` row.
- **Levels:** every step is awarded; only the highest is shown (badges.md §6).

### 4. Boards — BUILT 2026-11-21

`game/boards`: each kept save records its character's standing (XP, net worth, quest points,
places found) under its slot; `Rescore` sets the player's score on every board, the best
character standing for them, plus the record boards (monsters slain, toughest kill, fish
landed, biggest of each species, badges held). A score that changes gets a new time and an
empty `Event`, so slice 5 knows to sign it. `gameutil.NetWorth`; `stats.Top` ranks a board from
every record, kept in memory. Deleting a character drops it from the boards; the lifetime
record and badges stay.

- Standing boards from the kept save (XP, net worth, quest points, places discovered) into
  `slot_standing`; the player's best character stands for them. Record boards from the store
  (monsters slain, toughest kill, fish landed, biggest of each species, badges held).
- A board that changed gets a new score row.
- Tests: net worth counts the vault and container contents; deleting your best character drops
  you to the next one.

### 5. The DM signs (official only)
- Badge definitions (kind 30009) signed at start when one is new or changed: image and thumbs
  from `badge-media.json`.
- Awards (kind 8) and scores (kind 30762) signed at commit.
- Published to the DM's relays, the player's read relays (awards), and `relay.gamestr.io`
  (scores). A queue and a retry sweeper: an award waits as "being awarded" until a relay
  takes it.
- Never with `saves.validation` off (test).

### 6. In the game
- **Badges:** a badges screen on the character (earned, with the level rank showing only its
  highest; locked ones greyed with how to earn them), and a "The Dungeon Master awards you…"
  popup at the save that earns one.
- **Highscores:** a page with the boards, XP first. It reads the server's own record of each
  score (what it published, or on test what it would have), so the game and Gamestr can't
  disagree and the page needs no relay query.
- An `updates.json` entry.

### 7. Outside the build (you)
- Ask Gamestr to list Pubkey Quest with the DM as its developer pubkey, `xp` as the main board.
  A sponsored spot, when you're ready, is the same listing.

## Decided (2026-11-21)
- **The relays are the store** (above), not SQLite.
- **Test earns and shows badges and scores in the game, unsigned.**
- The draft's other calls stand: stats count at a kept save; lifetime records per player,
  save-derived badges per character; a daily counts once per player per day; no opting out for
  beta; the nine first boards; your best character stands for you on a board; net worth is the
  base value of everything you own, gold and vault included; fish scores in ounces.

## Later
- **The DM logs in as admin.** Signing in to the game as the DM's npub opens admin controls
  (server-stats.md §6, "Admin role"). It's not part of this build.
