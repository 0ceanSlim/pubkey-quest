# Saves on relays, the official server, and releases — plan

Status: **PLAN, open questions in §9** (2026-10-12). This is M5.6 P2. It builds on roadmap §4
(the trust model) and replaces `future-nostr-save-optimization.md` as the working plan.

**Why it matters:** once saves are on relays, they can't be taken back. Anyone can dig an old
save up later, and the official server has to keep honouring it. So **turning this on is the
last wipe**. The save format, versioning and migration path must be settled before then.

## 1. Decided (2026-10-12)

| Topic | Decision |
|---|---|
| **Where saves live** | The player's own write relays. |
| **Event** | **Kind 30078** (NIP-78, app-specific data). It's addressable, so one key can hold several saves, one per `d` tag. |
| **Client tag** | Every save carries the NIP-89 `client` tag for Pubkey Quest. |
| **Source of truth** | **The relay event.** The server's disk copy is only a cache. |
| **Visibility** | **Public**, plain JSON content, so friends and tools can inspect characters. |
| **Anti-cheat** | After a save is broadcast and **a relay answers OK**, the official server records that event's **id**, keyed by npub and slot. On load, the official server only accepts the save whose id matches. A save that was modified, written by another client, or rolled back to an older event won't match. |
| **Modded / test servers** | A config switch turns validation off. Such a server accepts any validly signed save, or one you build yourself, with unlimited slots. |
| **Save slots** | **5 per key** on the official server; unlimited on modded and test servers. |
| **Format first** | Before anything goes on a relay, the format is optimized as the drafts describe and **versioned**. Old saves are migrated forward, then re-signed and re-broadcast. |
| **Servers** | **Official** at `pubkey.quest`, holds no wipes once it's on. **Test** is the debug server and follows the modded rules. |
| **Releases** | The official server runs only a **single bundled binary** that is checksummed, has an **auto-incremented version**, and was built and released by a **passing CI**. |

## 2. The save event

```
kind: 30078
tags:
  ["d", "pubkey-quest:save:3"]     // the slot (1–5 on official)
  ["client", "pubkey-quest", ...]  // NIP-89
  ["v", "1"]                       // save-format version
  ["prev", "<event id>"]           // the save this one replaces (a hash chain; empty on a new character)
  ["alt", "Pubkey Quest save — Mara, level 4 Druid"]   // NIP-31 human-readable hint
content: <the compact save JSON, format v1>
```

- **NIP-78 asks** for the `d` tag to be namespaced by the app, hence `pubkey-quest:save:<slot>`.
  The character's name moves into the content.
- **Signed in the browser** by the player's own signer, then sent to the server, which publishes it
  through Grain to the player's write relays (`RoutePublish`), like profile edits today.
- **Size budget:** content of 16 KB or less. Today's pretty-printed saves are 10–12 KB, and the
  v1 compact format should bring them down to roughly 2–5 KB.

## 3. The official server's ledger

- **A small persistent store, separate from `game.db`.** `game.db` is a rebuildable cache and
  must never hold player records.
- One row per (npub, slot): the latest **accepted event id**, its `created_at`, its format
  version, and when it was accepted.
  - Exactly what goes in the row is §9 Q2.
- **Accepting a save:**
  1. the browser signs the event;
  2. the server checks the signature, kind, `d` slot, `client` tag, that `prev` equals the
     ledger's current id, and the size;
  3. the server publishes through Grain;
  4. on **at least one relay OK**, the ledger row is updated.
  - With no OK from any relay, the save didn't happen: the old id stays, and the player is told.
- **Loading on the official server:**
  - fetch the event **by the ledger's id** from the player's relays, and verify its id and
    signature;
  - if relays have the slot's newest event under a different id, it isn't playable here.
    The player sees why: edited, from another client, or an old copy.
  - **Revert freedom stays.** The player can always reload the last save they made here.
- **Loading on a modded or test server:** the newest valid event for the slot, no ledger check.

## 4. Save format v1 (do this first)

Taken from roadmap §4's optimization ladder, steps 1–3. Step 4, integer indexes, waits until
there is a versioned index table, and step 5, compression, is a last resort.

1. **Hydration audit:**
   - drop everything that can be derived (max HP and mana if derivable, weight, AC, level);
   - drop empty inventory slots and nulls.
2. **Compact keys and tuples** (sketch):
   - `{"v":1,"n":"Mara","r":"human","c":"druid","xp":4200,"hp":31,"mp":12,"g":250,"pos":["goldenhaven","east","sailors_tavern","cellar"],"t":[12,720],…}`
   - Inventory and equipment as `[slot, itemId, qty]` tuples.
   - Quests as `[id, stage, [counts]]`.
   - Effects as `[id, index, remaining, total, delay, tick]`.
   - The full key map is written down in the codec file and in this doc.
3. **A versioned codec:**
   - `types.SaveFile` stays the in-memory shape;
   - `savecodec.Encode(save) → v1 JSON` and `savecodec.Decode(bytes) → SaveFile` dispatch on
     `v`;
   - `Migrate(v_n → v_n+1)` steps are pure functions with round-trip tests, one per version.
4. **Migration on load:**
   - an old-version save decodes, migrates, and runs;
   - the player is asked to **re-sign at the next save**, so the event on relays moves to the
     current version. That's a normal save; its `prev` chains to the migrated one.
   - The official ledger records the version, so the server knows who is still on an old
     format.
5. **Save inspector (dev):** `codex --decode-save <file|nevent>` prints the hydrated sheet.
   Roadmap §4 says the encoder never ships without its viewer. A player-facing "view my save"
   page comes at beta.

## 5. Server modes

| Setting | Official | Test / modded |
|---|---|---|
| `saves.validation` (ledger check) | on | off |
| `saves.max_slots` | 5 | 0 (unlimited) |
| Debug tools (test combat, add item, …) | off | on |
| Accepts saves from other clients | no | yes |

These go in `config.yml`. The official build refuses to start with debug tools on.

## 6. Release engineering

- **One binary**, using Go `embed`. Inside it:
  - the built frontend (`www/dist`);
  - the templates and static art (`www/views`, `www/res`);
  - `game-data/`.
- **`game.db` is built at start-up** from the embedded game data (the migrate step, in
  process), so a binary always runs the exact data it was built with.
- **CI on `main`:** test → build → if it passes, **bump the version** (the next semver patch,
  as a git tag) → **create a GitHub release** with the binary and its `SHA256SUMS`.
- **The official server deploys only release binaries**, and verifies the checksum before
  swapping. **Test keeps deploying every push to `main`**, as now.
- **`/version` and the footer show the release version.** Saves record the version they were
  made with, which helps support.

## 7. Turning it on (the cut-over)

1. Ship format v1, the codec, migrations and inspector on test, and play with them for a while.
2. Ship the ledger, relay save and load, and the server modes on test, with validation off.
3. Ship the release pipeline and stand up the official server with validation on.
4. **The last wipe:** the official server starts empty. From the first save there on, no wipes
   ever.

## 8. Not in scope

- Encrypted saves (decided public).
- Integer indexing (comes once the index-table versioning is settled; a v2 format).
- Trading and PvP stakes (post-alpha), which will build on the ledger: a trade is a signed
  event that both sides' next saves reference.

## 9. Open questions

1. **What happens to test saves at launch?** Does the official server start empty, with test
   characters staying on test? Or should the test characters be carried over?
2. **The ledger: ids only, or the whole event too?** Keeping a copy of the latest signed event
   (2–5 KB) means a player can still load if every relay dropped their save. It's
   tamper-evident, so it adds no cheating risk.
3. **What happens to a save that doesn't match on the official server?** Refuse with an
   explanation? Or offer to load the last save the official server accepted (fetched by id),
   if relays still have it?
4. **How do official deploys happen?** Automatically on every release, or as a manual promote
   step (a button in GitHub Actions)? And does the official server run on the same machine as
   test?
