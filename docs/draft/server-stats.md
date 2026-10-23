# Server stats, badges, highscores & the Dungeon Master — draft

Status: **DRAFT, 2026-10-22, nothing built.** Not on the alpha critical path. It doesn't
block degradation, food or potions. Fishing (§8 of [fishing-system.md](fishing-system.md))
is its first concrete producer.

Related: roadmap §4 (saves, the hydration rule, official vs modded),
[relay-saves-plan.md](relay-saves-plan.md) (the ledger), [game-relay-plan.md](game-relay-plan.md)
(the relay inside the game), [nostr-identity-plan.md](nostr-identity-plan.md),
`cmd/server/game/events/events.go` (the recorder that already names "the badge/stat tracker"
as a future consumer), [Gamestr for developers](https://gamestr.io/developers) (highscores).

### Decided (2026-10-22, your words)
- **A badge is an achievement.** One thing, one word. A badge is a **Nostr event**: a NIP-58
  award signed by the DM. There's no separate in-game achievement list; the game shows the
  DM's awards.
- **Badges are for completion only:** caught one of every fish, unlocked every song, and the
  like. No grind-for-points badges.
- **One badge per quest and side quest** is fine.
- **Server-side counters** are fine for things like "completed 10 dailies".
- **The store holds only what the save can't or shouldn't track.**
- **The server has a Dungeon Master: an npub of its own** (§6). Game updates come from him as
  posts. He authors and awards the badges. Players DM him for support.
- **Highscores go through Gamestr:** kind 30762 score events, signed by the DM so Gamestr
  shows them as verified (§7).
- **Highscores blend save data and store data, and publish when the player saves.** The
  headline score is **XP**. Another is **net worth**: the base value of everything you own,
  equipped, carried or in the vault. More fun metrics are welcome (§7).

Everything else below is a recommendation, collected in §11.

---

## 1. How it fits

Today the official server keeps exactly one player record: the **save ledger**
(`cmd/server/ledger/`). It's the last accepted save event id per npub and slot, kept apart
from `game.db`, and it exists because "integrity comes from the official server's memory,
not the file" (roadmap §4).

The stat store is the **second record, under the same rules**:

| Rule | Ledger | Stat store |
|---|---|---|
| Lives apart from `game.db` | yes, `data/ledger/` | yes, `data/stats.db` |
| Keyed by | npub, slot | npub |
| Official server only | yes (`saves.validation`) | yes (`stats.enabled`) |
| Holds | what the server accepted | what the server witnessed, and which events it signed |
| Copies of save data | none | none |

So it isn't a new kind of thing. It's the ledger's sibling: the server remembering what it
saw, because that memory is the only thing an edited save can't fake.

**The store is private memory; the events are what everyone sees.** Players, the game and
any other Nostr client see badges and highscores only as DM-signed events: NIP-58 awards and
Gamestr scores. The store decides *when* the DM signs, and remembers that he did.

## 2. What goes where

Three buckets. The test: **would the record survive the item being sold, the quest being
re-rolled, or the character being deleted?** If the save already holds it for as long as it
matters, derive it.

### Derivable from the save (no store data)
On the official server every admitted save comes from an unbroken chain of official play
(the ledger checks `prev`). So anything **derivable from an admitted save is already
trusted.** The server reads it at save time.

| Badge | Derived from |
|---|---|
| One per main, side, class and race quest | `QuestsCompleted` |
| "Every song" | `MusicTracksUnlocked` vs every track in `music.json` |
| "Every town discovered" (candidate) | `LocationsDiscovered` |

The store only remembers that the **award was signed** (§5), not the list it came from.

Most highscores come from the save the same way: XP, net worth, quest points, places
discovered (§7). They're computed from the accepted save at each save, not stored in it.

### In the store (not derivable)
| Record | Why the save can't hold it | Shape |
|---|---|---|
| Fish species ever caught (24) | fish get sold, eaten, cut into bait; no catch log in the save | set per npub |
| Biggest of each species | the `lb` slot field leaves with the fish | best per npub × species |
| Fish landed | a lifetime count | counter |
| Dailies completed | `RepeatableQuests` keeps only the *last* period per quest id | counter + last period |
| Weeklies completed | same | counter + last period |
| Awards signed | which badge events the DM has issued, so none is issued twice | row per npub × badge |
| Monsters slain | a lifetime count | counter |
| Toughest kill | the highest-CR monster ever defeated | best per npub |
| Each character's standing | XP, net worth and the like for *each* of the npub's characters, so the board can show the best one (§7) | row per npub × slot × board; a cache, rebuildable from the accepted saves |
| Scores published | the last Gamestr score sent per board, so an unchanged one isn't re-sent | row per npub × board |

### Neither
- **An action log.** The store keeps aggregates, never "what you did at 14:02".
- **Per-character copies of save data.** If it's in the save, it stays there.
- **Anything play reads.** Gameplay never asks the store a question. Badges and highscores
  are flair, and the game must play the same on a modded server with no store at all.

## 3. Where it lives and how it's keyed

**`data/stats.db`, a SQLite file** through the `modernc.org/sqlite` the server already uses.
Not `www/game.db`, which is a rebuildable cache and must never hold player records. Backed up
with `data/`.

Why SQLite, when the ledger is JSON files: rule checks and "is this a new best" are small
queries, and one transaction can apply a whole save's worth of stats atomically.

Generic tables, so a new producer needs data, not a schema change:

```
stat_counter (npub, key, value, last_period, updated_at)    PK (npub, key)
stat_set     (npub, key, member, first_at)                   PK (npub, key, member)
stat_best    (npub, key, value, label, at)                   PK (npub, key)
badge_award  (npub, badge_id, awarded_at, event_id)          PK (npub, badge_id)
slot_standing(npub, slot, board, value, label, at)          PK (npub, slot, board)
score_sent   (npub, board, score, event_id, sent_at)         PK (npub, board)
```

Examples:
- `stat_counter("npub1…", "fish_landed", 312, …)`
- `stat_set("npub1…", "fish_species", "river-pike", …)`
- `stat_best("npub1…", "fish_oz:river-pike", 310, "Mara", …)`
- `badge_award("npub1…", "quest-the-rising-shadow", …, "<kind 8 event id>")`

**Keyed by npub, not by character.** A player has up to 5 characters. Store records are
lifetime records for the *player*: they survive a deleted character, and every character
adds to the same set. That matches both event types, which name the player's pubkey, not a
character. The `label` keeps the character name at the time, for the score event's content
("Mara landed a 19 lb 6 oz river pike").

Save-derived badges are judged **per character** (one character has every song), because
that needs no store data. See §11, question 3.

## 4. Trust, reloads and rewinds

### Only what the official server saw
- Producers are server code: the fishing engine, the quest engine. The client never posts a
  stat.
- The store only switches on with `stats.enabled`, and only on a server with
  `saves.validation` on. Modded and test servers don't feed it.
- A modded server can run its own store and its own DM (§6). Its badges and scores are signed
  by its DM, so provenance is the signature: Gamestr shows them under that game's key, and the
  official game only shows its own DM's events.

### It counts when you save it
Stats are **staged, not written**, until the player signs a save the ledger accepts:

1. An event arrives (`fish_caught`). The stats consumer appends it to a **pending list** for
   that session, in memory.
2. The player saves. The relay answers OK and `ledger.Record` succeeds.
3. In one transaction, the pending list is applied to the store, and the badge rules run
   against the store *and* the just-accepted save.
4. The save's standing numbers (XP, net worth, …) are computed and written to
   `slot_standing`.
5. The DM signs and publishes any new awards (§5) and every score that changed (§7).
6. The pending list is cleared.

Why not count on sight: a daily turned in, then a reload, then turned in again would count
twice, because the reload rewinds `RepeatableQuests`. Gating on the save ties stats to the
same chain the ledger already protects. It's the Pokémon rule the saves already follow: **it
didn't happen unless you saved.**

| Scenario | What counts |
|---|---|
| Catch a fish, save | counted at the save |
| Catch a fish, reload before saving | nothing; pending dropped on load |
| Catch a fish, close the tab | nothing; pending dies with the session |
| Server restart mid-session | pending is journaled with the session's crash journal and comes back with it |
| Save, then try to load an older save | can't on official; the ledger only loads the last accepted one |
| Delete a character | store records and badges stay; they're the npub's. Its standing rows go, and the standing boards drop to the next-best character |
| Five characters do today's daily | counts once; dailies dedupe per npub per period (`last_period`) |
| Party | each member has their own pending list, committed on their own save |
| A record fish, then death before saving | not counted. Harsh, but it's the save rule |

## 5. Badges

### Definitions are content
`game-data/systems/badges.json` → migrated into `game.db` like everything else. Which badges
a player holds is never in `game.db`: it's the DM's award events, remembered in
`badge_award`.

```json
{ "id": "every-fish", "name": "Angler of the Realm",
  "description": "Caught one of every fish.",
  "image": "/res/img/badges/every-fish.png",
  "rule": { "set": "fish_species", "complete": "fish" } }

{ "id": "quest-the-one-that-got-away", "rule": { "quest": "the-one-that-got-away" } }
{ "id": "dailies-10",  "rule": { "counter": "dailies_completed", "at": 10 } }
{ "id": "every-song",  "rule": { "save": "music_tracks_unlocked", "complete": "music" } }
```

- Quest badges can be **generated** at migration (one per main, side, class and race quest),
  so a new quest gets its badge for free. A quest can opt out or override name and art.
- `--validate` checks that rules name real quests, sets, counters and catalogs.

### The events (NIP-58)
| Event | Kind | Signed by | When |
|---|---|---|---|
| Badge definition | 30009 (addressable, `d` = badge id; name, description, image, thumb) | the DM | at start-up, when a definition is new or changed |
| Badge award | 8 (`a` → `30009:<dm>:<badge id>`, `p` → the player) | the DM | at the save that earned it |
| Profile badges (which to show off) | 30008 | the **player** | optional, from the badges screen |

- Awards go to the game relay, the DM's write relays and the player's read relays, so any
  Nostr client that shows NIP-58 badges shows them, credited to the DM.
- **The game's badges screen shows the DM's kind 8 awards** that `p`-tag the player, joined to
  his kind 30009 definitions. A badge from any other issuer isn't an official badge, whatever
  it's called.
- **An award exists once it's signed.** If the signer or the relays are down, the
  `badge_award` row waits with an empty `event_id` and a sweeper retries. Until then the
  screen shows it as "being awarded", not as earned.
- **Retroactive for free:** save-derived badges are checked at every save, so quests done
  before badges ship are awarded at the next save. Store-backed badges start at zero. That's
  fine, since the official server starts empty anyway.

## 6. The Dungeon Master

The official server has an identity: **the DM**, an npub configured on the server. The client
shows things from that npub, and the server signs as it.

```yaml
dm:
  npub: npub1…                 # public; the client reads posts, profile, badges, scores from this
  name: The Dungeon Master
  bunker: bunker://…           # NIP-46 remote signer (recommended), or
  key_file: data/dm.key        # fallback, official host only, 0600
```

### What the DM does
| Use | How | Signed by |
|---|---|---|
| **Game updates and news** | kind 1 notes (short news, maintenance, MOTD); kind 30023 long-form for big patch notes; shown in-game in a news panel | DM (you, from any client, or the server for automatic ones) |
| **Badges** | NIP-58 definitions and awards, §5 | DM (server, automatic) |
| **Highscores** | Gamestr kind 30762 scores, §7 | DM (server, automatic) |
| **Support** | "Message the DM" in settings sends a NIP-17 DM (the same gift wraps whispers already use) to the DM's inbox relays. You answer from your own Nostr client | player → DM; DM replies |
| **Restart notices** | the existing restart warning also goes out as a DM note, so people outside the game see it | DM (server) |
| **Server identity** | NIP-05 `dm@pubkey.quest`; the game relay's NIP-11 `pubkey` field; the DM's kind 0 profile is the server's face; the developer pubkey Gamestr lists | — |

### More it could do (later, each its own decision)
- **Signed releases:** on promote, the DM publishes a NIP-94 file event (kind 1063) carrying
  the release binary's SHA-256. Anyone can check an official binary against the DM's key.
- **Official-save attestations:** a NIP-32 label (kind 1985) from the DM on each accepted save
  event. Then anyone, not only the official server, can tell an official character from a
  modded one.
- **Server firsts:** "first to finish *The Rising Shadow*" posted by the DM. Opt-in only.
- **Chat moderation:** the DM's public mute list (kind 10000) doubles as the server-wide chat
  mute list.
- **Admin role:** logging in as the DM npub *is* the owner login, for debug tools and, later,
  wiki editing.

### The key
The server must sign awards and scores with nobody at the keyboard, so it needs a way to sign
as the DM. The player-key rule ("the server never sees a private key") is about **players**.
The DM is the server's own identity, but the same caution applies.

| Option | Server holds the key? | Notes |
|---|---|---|
| **NIP-46 bunker** | no | The key stays in your signer (nsec.app, nsecbunker, your own). The server connects as a client, scoped to the kinds it signs automatically (8, 30009, 30762, 1, 1063). You post and answer support DMs as the DM from any client through the same bunker. If the bunker is down, awards and scores queue (the sweeper). |
| **Key file** | yes | Simplest, and what Gamestr's own examples do. `data/dm.key`, 0600, official host only, never in `config.yml` or the repo. A leak lets someone issue official-looking badges and verified scores until you rotate, and rotating orphans every old event's provenance. |

**Test gets its own DM**, a separate npub (e.g. "the Apprentice DM"). Test badges, scores and
posts can then never pass for official ones.

### The DM and `updates.json`
`www/res/updates.json` stays the one place release notes are written (CLAUDE.md). On promote
to official, the DM **publishes** the new entries as a post. So the file stays the source,
and the DM is its broadcast. The in-game news panel shows DM posts, which can also carry
things that aren't release notes: events, maintenance, a word from the DM.

## 7. Highscores via Gamestr

[Gamestr](https://gamestr.io/developers) is a Nostr leaderboard network. A game publishes
**kind 30762** score events; Gamestr aggregates them per game. A score signed by the game's
developer pubkey (the player goes in the `p` tag) shows as **verified**. That's exactly our
model: the DM signs, and only what the official server saw gets signed.

### When scores publish
**At every save the ledger accepts.** All boards are recomputed from the accepted save plus the
store, and every board whose value changed since `score_sent` is signed and published. Saves
are deliberate, so that's a handful of events per save, never a stream.

### Two kinds of board
| Kind | Means | Can go down? | Comes from |
|---|---|---|---|
| **Standing** | where you stand *now* | yes: XP loss, selling things, a deleted character | the accepted save, at each save |
| **Record** | the best you've *ever* done | never | the store |

**Standing boards use your best character.** Gamestr keeps one score per player per board, but
a player has up to 5 characters. So a standing score is the highest value across the npub's
characters (`slot_standing`), and the event's content names that character. When your best
character is deleted, the board falls to your next best at the next save.

### The boards
| Board (`level`) | Score | Kind | From |
|---|---|---|---|
| **`xp`** (the headline) | total XP | standing | `Experience` |
| **`net-worth`** | base value of everything owned | standing | the save, see below |
| `quest-points` | quest points | standing | derived from `QuestsCompleted` |
| `places-discovered` | count | standing | `LocationsDiscovered` |
| `badges` | DM awards held | record | `badge_award` |
| `monsters-slain` | count | record | store counter |
| `toughest-kill` | the monster's XP value | record | store best; content names the monster |
| `biggest-<species>` (24) | ounces | record | `stat_best` |
| `fish-landed` | count | record | `stat_counter` |

**Net worth** adds up the item definition's base `value` × quantity for everything the
character owns:
- equipped gear;
- the inventory, including what's inside containers (a tackle box, a quiver);
- the vault.

Gold is the `gold-piece` item, worth 1 each, so it counts with no special case, and selling an
item doesn't lower your net worth by more than the shop's cut. Generated items (fish, gems)
carry their own value. Base value, not shop prices, so it doesn't depend on where you are.

**Toughest kill** scores by the monster's XP value rather than its CR, because Gamestr scores
are integers and CRs come as 1/8, 1/4, 1/2.

### More fun boards (candidates)
| Board | Score | Kind | Note |
|---|---|---|---|
| `treasure` | the single most valuable item you own | standing | content names the item |
| `songs` | songs unlocked | standing | from `MusicTracksUnlocked` |
| `days-in-realm` | in-game days lived | standing | `CurrentDay`; the old-timer board |
| `spells-known` | spells known | standing | `spellbook.Known` |
| `wanderer` | in-game miles travelled | record | needs a distance on travel events |
| `trader` | gold spent at shops | record | needs a value on `shop_transaction` |
| `dailies-completed` | count | record | mostly measures who logs in most |

### The event
One addressable event per player per board. The `d` tag is `game:player:level`, and Gamestr's
`level` is our board, so each save simply replaces the old score.

```json
{
  "kind": 30762,
  "pubkey": "<the DM>",
  "content": "Mara, level 7 Druid: 23,400 XP.",
  "tags": [
    ["d", "pubkey-quest:<player hex>:xp"],
    ["game", "pubkey-quest"],
    ["i", "https://pubkey.quest"],
    ["k", "web"],
    ["score", "23400"],
    ["p", "<player hex>"],
    ["state", "active"],
    ["level", "xp"],
    ["mode", "single-player"],
    ["t", "rpg"]
  ]
}
```

- Required tags per Gamestr: `d`, `game`, `i` (the game's URL, NIP-73), `k` (`web`), `score`,
  `p`.
- **Scores are integers,** so fish weights go in **ounces** (19 lb 6 oz = 310). The content
  line carries the readable form: "Mara landed a 19 lb 6 oz river pike."
- Gamestr's optional `achievements` tag (comma-separated ids) can carry the player's badge ids
  on the `xp` event, so Gamestr shows badges next to the headline score.
- Published to `wss://relay.gamestr.io`, the game relay and the DM's write relays. **Test**
  publishes to `wss://test.gamestr.io` under the test DM.

### In the game
The highscores page shows the DM's 30762 events for `pubkey-quest`, read from the game relay
(or `relay.gamestr.io` until the game relay exists). It's the same board Gamestr shows, so the
game and the wider network can't disagree.

### Listing
Gamestr lists a game from its metadata (name, description, image, genres) on request, and
verifies scores by the registered developer pubkey. So: register **the DM's npub** as Pubkey
Quest's developer pubkey, and the test DM's for test. Ask them to show **`xp`** as the game's
main board.

## 8. Privacy and size

**Privacy:**
- Badges and scores are public Nostr events, like the saves already are. They're
  **permanent**: a NIP-09 deletion is only a request.
- No IPs, no timestamps beyond `first_at` / `updated_at`, no action log.
- "Delete my stats" removes the npub's rows and sends NIP-09 deletions for its awards and
  scores.
- Support DMs are NIP-17 gift wraps to the DM. The server can't read them.

**Size, per player:**

| Table | Rows |
|---|---|
| `stat_set` | ~24 fish species, plus future sets |
| `stat_best` | ~24 |
| `stat_counter` | ~10 |
| `badge_award` | ~40 (one per quest, plus completions and milestones) |
| `slot_standing` | ~4 standing boards × up to 5 characters = ~20 |
| `score_sent` | ~33 |

About 150 rows, a few KB. 10,000 players is around a million rows, tens of MB. The events
themselves live on relays; the store keeps only their ids.

## 9. The first event set

The recorder's `Event` is only `Kind`, `Target` and `Count` today. It gains two optional
fields; the quest consumer ignores both:

- `Value float64`: a measure for highscores (`lb`).
- `Tags []string`: details (`size-huge`, `spot:cragspire-falls`, `barehand`).

| Event | Target | Value / Tags | Store effect | Badges / boards |
|---|---|---|---|---|
| `fish_caught` (new, fishing slice 3) | species id | `lb`; size, spot, barehand | add to `fish_species`; `fish_landed` +1; best `fish_oz:<species>` | "every fish" badge; species and fish-landed boards |
| `quest_completed` (new) | quest id | — | daily or weekly: counter +1 if the period is new for this npub | dailies 10/50/100, weeklies 5/25 badges |
| `monster_killed` (exists) | monster id | — | `monsters_slain` +1; best `toughest_kill` by the monster's XP value | `monsters-slain`, `toughest-kill` boards |
| songs unlocked | none, read from the save | — | none | "every song" badge |
| quests | none, read from the save | — | none | one badge per quest; `quest-points` board |
| the save itself | none | — | `slot_standing` | `xp`, `net-worth`, `places-discovered` boards |

`quest_completed` is recorded where the quest engine finishes a quest (`quest.FinishNow` and
the repeatable completion path). It gives other consumers a clean hook too.

**The consumer.** `stats.Consumer()` subscribes at start-up, only when `stats.enabled`. It
keys pending lists by `save.InternalNpub` + `save.InternalID`. The save path calls
`stats.Commit(npub, id, save)` after `ledger.Record`, and the load and session-end paths call
`stats.Drop(npub, id)`.

## 10. Build slices

1. **The DM, read-only.** `dm.npub` in config; NIP-05; fetch the DM's profile and posts; an
   in-game news panel; "Message the DM" through the whisper path. No key needed yet.
2. **The store and the commit gate.** `data/stats.db`, the tables, `stats` package, pending
   lists, commit on ledger accept, drop on load, crash-journal round trip. Tests first: a
   reload drops, a save applies, five characters count one daily.
3. **Producers.** `Value`/`Tags` on `Event`; `quest_completed`; dailies and weeklies counters.
   `fish_caught` lands with fishing slice 3.
4. **The DM signs.** The bunker client (or key file), with a queue and retry sweeper shared by
   everything he signs automatically.
5. **Badges.** `badges.json`, generated quest badges, the migration, rule evaluation at commit,
   kind 30009 definitions and kind 8 awards, the badges screen reading the DM's awards, the
   player's optional kind 30008. Badge art: one crest template for quest badges, plus a
   handful of bespoke ones.
6. **Highscores.** The standing boards from the save (XP, net worth, quest points, places) and
   `slot_standing`; the record boards from the store; kind 30762 scores published at each save;
   the highscores page reading them; the Gamestr listing with the DM as developer pubkey.
   Tests: net worth counts containers and the vault; deleting the best character drops the
   board to the next one.
7. **Later:** `updates.json` → DM posts on promote, signed releases (1063), save attestations
   (1985), server firsts, DM mute list for chat, DM-as-admin.

**Depends on:** the official server running with `saves.validation` on (relay-saves cut-over).
The game relay (G2) makes publishing reliable but isn't required. Fishing for the fish badges
and boards.

## 11. Still open (each with a recommendation)

1. **Store format:** SQLite `data/stats.db`, or per-npub JSON like the ledger?
   **Recommend SQLite,** for atomic commits of a whole save's stats.
2. **When stats count:** at the save (commit gate), or the moment they happen?
   **Recommend at the save.** Counting on sight double-counts dailies through reloads.
3. **Whose records:** per npub for store records, per character for save-derived badges?
   **Recommend yes.** Both event types name the player, fish sets across all your characters
   feel right, and save-derived ones then need no store data.
4. **Daily dedupe:** count each daily period once per npub, however many characters do it?
   **Recommend yes.**
5. **Quest badge scope:** main, side, class and race quests, not dailies or weeklies?
   **Recommend yes,** generated at migration.
6. **The DM's key:** NIP-46 bunker or a key file? **Recommend the bunker,** with the key file
   as a fallback. The server then never holds a private key, and you use the same identity
   from your own client for posts and support.
7. **Test's DM:** a separate npub, publishing to `test.gamestr.io`? **Recommend yes.**
8. **Opting out:** can a player keep their badges or scores off Nostr?
   **Recommend no** for alpha. Saves are already public, and a badge *is* its event, so
   there's no in-game-only version to fall back to.
9. **The first boards:** the nine in §7 (XP, net worth, quest points, places, badges, monsters
   slain, toughest kill, biggest of each species, fish landed), with the candidates waiting
   for later? **Recommend yes.** `treasure` and `days-in-realm` are the cheapest to add, since
   both come straight from the save.
10. **Standing boards across characters:** the npub's best character, rather than one board
    entry per character? **Recommend yes.** Gamestr keys a score to a player, not a character.
11. **Net worth:** base value, counting equipped gear, the inventory with container contents,
    the vault and gold? **Recommend yes.** Base value, not shop prices, so it's the same in
    every town.
12. **Fish score unit:** ounces as the integer score? **Recommend yes.**
13. **`updates.json` vs DM posts:** the file stays the source, the DM broadcasts it on promote?
    **Recommend yes,** and the news panel shows all DM posts.
14. **Support:** you answer DMs from your own Nostr client, no in-game DM inbox?
    **Recommend yes** for alpha.
15. **Server firsts:** **Recommend later,** opt-in only.
16. **The DM's name:** "The Dungeon Master", or an in-world name? **Recommend "The Dungeon
    Master"** on the profile; it reads well in any Nostr client.
