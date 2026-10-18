# The game's own relay — plan

**Goal:** the game server ships with a Nostr relay inside it: one binary, one port. The relay
runs Grain, configured for Pubkey Quest. Saves and whispers always have a relay that accepts
them. Players' own relays stay their mirrors, not the game's lifeline.

**Decided (2026-10-17):**

- **B (embedded Grain) is what the full release needs.** The game binary mounts a Grain relay
  at `/relay`.
- **C (a pure-Go storage backend in Grain) is wanted as a Grain improvement.** It lets B drop
  cgo, so the game stays one pure-Go build like today.
- A (a sidecar process) is not the target. It's only a fallback if B slips.

## Where things stand

- The game only uses Grain's **client** packages (`client/core`, `session`, `connection`,
  `cache`, …) for login, saves, whispers and lists. It pins grain `v0.8.1`; the local Grain
  checkout is `v0.8.0-5` and newer.
- Saves go to the player's write relays. The ledger keeps event ids plus the relays that
  accepted them (`docs/draft/relay-saves-plan.md`, "a dedicated relay for this app comes
  later"). This plan is that "later".
- Neither embedding nor a pure-Go store is on Grain's roadmap yet, so both are new Grain work.

## Why Grain can't be embedded today (v0.8.1)

1. **`server.Run()` is a whole program.**
   - It reads `config.yml`, whitelist and blacklist files from a data directory and watches
     them, restarting itself when they change.
   - It catches SIGINT and SIGTERM itself.
   - It opens its own `http.Server` on its own port. Nothing returns an `http.Handler`.
2. **Global state everywhere.**
   - Config is a package singleton (`config.GetConfig`), and so is the store (`nostrdb.GetDB`,
     `SetGlobalDB`).
   - `handlers.OnEventStored` is a package variable.
   - Startup calls `client.InitializeClient`, the same client packages the game initializes
     for its own logins and saves. Two owners of one global would overwrite each other.
3. **Storage is nostrdb, a C library (LMDB underneath) behind cgo.** It needs a static build
   (`server/db/nostrdb/build.sh`: gcc, make, autotools) on every platform. The game is pure Go
   today: `modernc.org/sqlite`, and cross-compiled release binaries.

What helps:

- **Every storage call goes through one `*NDB` handle**, about 25 methods (`StoreEvent`,
  `Query`, `CountFiltered`, `ProcessDeletion`, `DeleteNoteByID`, expiration and purge,
  stats…).
- **That handle is used from five places:** `handlers/{event,req,count}.go`,
  `api/relayStats.go` and `startup.go`.

So a storage interface is a contained change.

## Part 1 — Grain: an embeddable relay (B)

A new package (working name `grain/relay` or `server/embed`) with a library entry point.
`server.Run()` becomes a thin wrapper around it:

```go
r, err := embed.New(embed.Options{
    DataDir: "data/relay",     // the store lives here; no config files are read
    Config:  cfg,              // a *ServerConfig built in code (or loaded by the host)
    Policy:  policy,           // whitelist/limits, see the game config below
    Logger:  logger,           // the host's logger
    PublicURL: "wss://test.pubkey.quest/relay", // for NIP-42 AUTH and NIP-11
})
mux.Handle("/relay", r.Handler()) // WebSocket upgrade + NIP-11 on the same path
defer r.Close()                   // flush and close the store
```

What it must and mustn't do:

- **No signal handling, no file watchers, no listener of its own** in embedded mode. The host
  owns the process, and a config change means calling `r.Reload(cfg)` or restarting the
  host.
- **Works under a path prefix.**
  - NIP-11 answers `Accept: application/nostr+json` at `/relay`.
  - NIP-42 checks the `relay` tag against `PublicURL`.
  - The dashboard and web client are off by default. Optionally they can be mounted elsewhere
    (for example `/relay/admin`, owner only).
- **Doesn't touch the client package's globals.**
  - Features that make the relay act as a client (mute-list refresh over NIP-65, NIP-66
    discovery, the upstream pool) are off in embedded mode, or take a client the host passes
    in.
  - This is the riskiest part: the game already initializes `client/core` for its own use.
- **One relay per process is fine to start.**
  - The globals can stay behind the new API for now, as long as only one embedded relay runs.
  - De-globalising (config, store and broadcast hook carried on the `Relay` value) can follow.
- **Tests:** start an embedded relay on `httptest`, publish, query and delete over a real
  WebSocket, check NIP-11 and NIP-42 under a prefix, and close cleanly with nothing leaking.

**Release impact while storage is still nostrdb:** the game's release build needs cgo and the
nostrdb static library per platform (Linux, Windows via MinGW, macOS). The CI and release
script gain that step. Part 2 removes it again.

## Part 2 — Grain: a pure-Go store (C)

- Extract a **`Store` interface** from `*NDB`'s methods and route the five call sites through
  it.
- **nostrdb** stays the default for the standalone relay. It moves behind a build tag
  (`nostrdb`, cgo).
- **A pure-Go implementation**, SQLite through `modernc.org/sqlite`:
  - Events table, tag index table, and indexes on (kind, pubkey, created_at) and tag values.
  - Covers NIP-01 filters, replaceable and addressable semantics, NIP-09 deletion, NIP-40
    expiration, and NIP-45 counts.
  - It becomes the default whenever cgo is off.
- **Gaps are allowed, but must be stated.**
  - NIP-50 full-text search needs SQLite FTS5 or is reported as unsupported in NIP-11.
  - The LMDB map-usage gauge becomes a disk-size gauge.
- **One conformance suite runs against both stores** (the same events in, the same results
  out), so the two can't drift.
- For the game, the embedded relay could use its own database file (`data/relay/relay.db`) or
  share the game's SQLite. A separate file is simpler and is the default.

Once this lands, the game builds with `CGO_ENABLED=0` again: one pure-Go binary with a relay
inside.

## Part 3 — the game: mounting and using its relay

**Config** (`config.yml`):

```yaml
relay:
  enabled: true
  path: /relay
  data_dir: data/relay
  public_url: wss://test.pubkey.quest/relay
```

**Policy: what the game's relay accepts.** It's a relay for the game, not a general-purpose
one.

| Kind | What | Rule |
|---|---|---|
| 30078 | Saves | Only `d` tags starting `pubkey-quest:`; size cap sized to the save budget |
| 5 | Deletion requests | Only ones naming the author's own events (NIP-09) |
| 1059 | Whisper gift wraps | Any author (wraps use throwaway keys); reads need NIP-42 AUTH as the `p`-tagged recipient |
| 10050 | DM inbox relay lists | Players' own; so whispers can route here |
| 0, 3, 10000, 10002 | Profile, contacts, mutes, relay list | Players' own; cached copies that make the friends list and whispers fast |

- Everything else is rejected.
- Rate limits are per pubkey and per IP.
- Who counts as a player is open, see the open questions.

**Wiring it in:**

- **Saves.**
  - Publish adds the game's relay to the player's write relays (`RoutePublish` extras).
  - The ledger records it like any relay that answered OK.
  - Loading and the saves list ask it first. It answers in milliseconds, which also fixes the
    slow saves screen at the source, and players' relays are asked in parallel as mirrors.
- **Whispers:** wraps also go to the game's relay; the inbox reads it first.
- **Lists:** friends and mutes read the game relay's cached copies first, then refresh from
  the player's relays.
- **Official mode:** the game's relay is where accepted saves are guaranteed to be found, so
  the ledger's "fetch it back" never depends on a third-party relay staying up.

**Deployment:**

- The reverse proxy must pass WebSocket upgrades on `/relay`. Check the test host's config.
- `data/relay/` is backed up with the saves.

## Order of work

| Step | Where | What | Done when |
|---|---|---|---|
| G1 | Grain | Embeddable relay API (Part 1), standalone `Run()` rebuilt on it | An `httptest` relay passes publish/req/delete/NIP-11/NIP-42 under a path |
| G2 | Game | Mount at `/relay`, game policy, saves/whispers/lists use it | Saves and whispers work with every external relay down |
| G3 | Game | Release builds with cgo + nostrdb per platform | Release binaries for all three platforms run the relay |
| G4 | Grain | `Store` interface + pure-Go SQLite store + shared conformance suite | Both stores pass the same suite |
| G5 | Game | Switch to the pure-Go store and `CGO_ENABLED=0` | One pure-Go binary with the relay inside |

G1–G3 are the release path (B). G4–G5 are the Grain improvement (C) that takes cgo back out.
G4 can start alongside G2 if wanted, since it doesn't depend on the embed API.

## Open questions

1. **Who may write.** Any valid signature, or only pubkeys that have logged in to this game
   server (the server knows them), or a NIP-42 AUTH gate on writes?
2. **Public reads.** Saves are signed and public by design (anyone can read a save on a public
   relay today). Keep that, or require AUTH for reads of saves too?
3. **Retention.** Keep every save event, or only the newest per address plus the ledger's
   accepted ids?
4. **Grain versioning.** Does the embed API ship as Grain v0.8.x or open v0.9, and does the
   game move off the `v0.8.1` pin to it?
5. **Admin.** Expose Grain's dashboard for the game relay (owner only), or keep it headless?
