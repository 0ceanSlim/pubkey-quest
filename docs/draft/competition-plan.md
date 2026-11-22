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

## One change to the design: test plays badges, unsigned

Test is the production nightly, and it never signs (your rule). Under the draft as written, it
would never *show* badges either, so you couldn't playtest them before launch. So:

- **`stats.enabled` runs the store** on any server that sets it: test and official both earn
  badges and post scores **in the game**.
- **Signing and publishing happen only with `saves.validation` on** (official). Test's badges
  and scores never leave the server: no kind 8 awards, no 30762 scores, no Gamestr.
- Test's store is its own file, so nothing earned on test can reach official.

## Slices

Each slice is its own commit, tests first, and the game plays the same with `stats.enabled` off.

### 1. The store and the commit gate
- `cmd/server/stats`: `data/stats.db` (SQLite, the server's existing driver), the six tables of
  server-stats.md §3.
- **Pending per session**, keyed by npub + save id; journaled with the session (like its
  ground). Dropped on load.
- **Commit at a kept save**: after `ledger.Record` on official, after the disk save on test. One
  transaction applies the pending stats, then the badge rules and boards run (slices 3 and 4).
- Tests: a reload drops; a save applies; five characters do one daily and it counts once; a crash
  restores the pending stats from the journal.

### 2. Producers
- New events: `QuestCompleted` (target = quest id, from `FinishNow` and the repeatable path),
  `GemFound` (kills, rock faces, Veins cells, shucking), `OreMined`.
- `FishCaught` gains the catch's weight (the `lb` slot key). `MonsterKilled` already names the
  monster, whose `xp` gives the toughest-kill board.
- Store effects (server-stats.md §9): fish species set, fish landed, biggest per species (in
  ounces), gem types, ores, dailies per day per npub, monsters slain, toughest kill.

### 3. Badges
- `game-data/systems/badges.json`: the 43, each with id, name, description, rim and rule.
  `--migrate` loads it with `badge-media.json`. `--validate` checks every rule names a real
  quest, set or counter, and warns when a main, side, class or race quest has no badge.
- Rules run at commit, against the store and the save just kept. A new badge is a
  `badge_award` row.
- **Levels:** every step is awarded; only the highest is shown (badges.md §6).

### 4. Boards
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

## Open (the draft's §11, still unanswered; my recommendation first)
1. SQLite store, **yes**.
2. Stats count at a kept save, **yes**.
3. Store records per player, save-derived badges per character, **yes**.
4. A daily counts once per player per day, **yes**.
5. No opting out of badges or scores, **yes** for beta.
6. The nine first boards (XP, net worth, quest points, places, badges, monsters slain, toughest
   kill, biggest of each fish, fish landed), **yes**.
7. Standing boards show the player's best character, **yes**.
8. Net worth = base value of everything owned, gold and vault included, **yes**.
9. Fish scores in ounces, **yes**.
10. New: test earns and shows badges and scores in the game, unsigned. **Recommend yes.**
