# Handoff → the multi-entity combat chat

Written 2026-10-08, from the Nostr/identity session. Read this before touching
`worktree-m56-p1-multi-entity-combat` again.

**The short version: the G1 auth slice is now on main. Your worktree should drop its copy
and keep only combat.**

## What moved to main

The auth half of what that worktree was carrying is committed and pushed to `main` as
`c8cb355` ("auth: prove the key at login, and gate identity in one place"). Main also
moved on underneath you in five other commits — see *Rebase* below.

Taken from the worktree, essentially as written:

| File | State on main |
|---|---|
| `cmd/server/auth/challenge.go` | identical |
| `cmd/server/auth/identity.go` | **+2 changes**, see below |
| `cmd/server/auth/auth_test.go` | **+2 changes**, see below |
| `cmd/server/auth/grain.go` (proof in `HandleLogin`, `Proof` field, read-only refusal) | as written |
| `cmd/server/auth/init.go` (`startSessionCleanup`) | identical |
| `cmd/server/app/app.go` (`auth.RequireIdentity(mux)`) | identical |
| `cmd/server/api/routes.go` (`/api/auth/challenge`) | as written |
| `src/lib/session.js` (proof signing, silent re-login, fetch replay) | identical |

Credit where due: the design is yours, and it is good. Two changes were made on the way in.

### 1. A protected route now requires a session even when it names no player

The worktree version passed `claimed == ""` straight through to the handler. On main that
returns 401 instead.

Why: `/api/debug/sessions` carries no npub, and under the old rule an anonymous caller
reached the handler. That endpoint used to dump every active session's full save data —
which is what main's `2fc9cf0` had already scoped to the caller. Requiring a session is
defence in depth for exactly that shape of route.

The test table records it:

```go
{"names no player, signed in",  "POST", "/api/game/action", `{"action":"x"}`, true,  200},
{"names no player, anonymous",  "POST", "/api/game/action", `{"action":"x"}`, false, 401},
{"npub-less protected route still needs a session", "GET", "/api/debug/sessions", "", false, 401},
```

### 2. `RequireIdentity` publishes the proven npub on the request context

`auth.OwnerNpub(r)` returns the npub the gate proved, or `""` for a request that never
passed it. `/api/debug/sessions` uses it to scope its answer to the caller, and would fail
silently without it. `TestRequireIdentityPublishesTheProvenNpub` covers it.

Handlers that need to know *who* is calling should read this rather than the request body —
the body is what the gate just finished distrusting.

## What you must do in the worktree

1. **Delete the auth files.** `cmd/server/auth/challenge.go`, `identity.go`, `auth_test.go`
   are on main now; keeping them guarantees a conflict.
2. **Revert the auth-only edits**: `cmd/server/auth/grain.go`, `cmd/server/auth/init.go`,
   `cmd/server/app/app.go`, `src/lib/session.js`. Main has all of them.
3. **Keep `cmd/server/api/routes.go` and `cmd/server/api/game/actions.go`** — both carry
   combat work *and* auth work. Keep your combat hunks, drop the auth ones
   (`/api/auth/challenge` registration; any `RequireIdentity` wiring).
4. **Drop the `go.mod` / `go.sum` change.** You pinned `grain v0.8.0`; main is on
   **v0.8.1**, which fixes the session data race and marks the cookie `Secure` over HTTPS.
   Take main's.
5. `docs/draft/party-multiplayer-plan.md` and `docs/draft/grain-integration-handoff.md`
   are already on main — drop your copies.

Everything else in those 31 dirty files is yours and untouched: `game/combat/{grid,turns}.go`,
`game/encounter/group.go`, `types/combat.go`, the combat engine edits, `combatSystem.js`,
`game.html`, and the combat tests.

## Rebase

The worktree is pinned at `37f4545`; main is six commits ahead. Conflicts to expect, all in
files you also touched:

| Main commit | Touches |
|---|---|
| `2fc9cf0` auth binding | `api/routes.go` — **superseded**, the per-route wrappers are gone again |
| `1657339` encounters | `api/game/actions.go`, `game/encounter/`, `session/types.go` |
| `87320f4` travel bar | `src/ui/locationDisplay.js` |
| `04e187b` grain v0.8.1 | `go.mod`, `go.sum`, `auth/grain.go` |
| `807ba1c` plans | docs only |
| `c8cb355` auth slice | the auth files above |

Two live interactions with your work:

- **`1657339` touched `cmd/server/api/game/actions.go`**, which you also modify.
  `maybeFireEncounter` gained a `minutesElapsed` parameter, the place-based triggers moved
  off `move`/`enter_building` onto a dwell timer, and `session.GameSession` gained
  `PlaceKey` / `PlaceSince` / `PlaceRolled` / `PlaceInCity` / `PlaceBuildingType`.
- **`maybeRollTravelEncounter` starts combat**, so your P1 multi-combatant rework meets the
  encounter path there. New `game/encounter/group.go` on your side and the biome roller on
  main both decide what a fight contains — worth deciding which owns group composition.

`grain v0.8.1` also changed two signatures you may be calling: `session.CreateUserSession`
and `SessionManager.CreateSession` both take the `*http.Request` now (that is how the
cookie's `Secure` flag is decided), and `CreateSession` additionally takes the user's
connected relays. Your `auth_test.go` called the old three-argument form; main's copy is
already fixed.

## Pre-existing gap, not yours to fix

`src/lib/nostrConnect.js` posts `/api/auth/login` directly instead of going through the
session manager, so it sends no proof and is now refused. Its exports are referenced from
no view or page — MILL owns login and NIP-55 covers Amber — so it is dead UI. It wants
deleting, not wiring up, and that is a separate cleanup.

## Verifying after the rebase

```
go build ./... && go vet ./...
go test ./tests/... ./cmd/server/auth/
npm run build
go run ./cmd/codex --migrate && go run ./cmd/codex --validate
```

CI runs all of that, so a failure blocks the test deploy. Main is green on all of it as of
`c8cb355`.

---

# Cleanup backlog

Found while doing the vault, encounter and auth work. **The deletions below were done on
2026-10-08** (see the "cleanup" commit); what remains is marked.

## Done — dead code removed

- **The legacy HTML5-drag inventory layer.** 407 lines from
  `src/systems/inventoryInteractions.js`: `bindSlotEvents`,
  `bindEquipmentSlotEvents`, `handleDragStart/End/Over`, `handleDrop`,
  `handleDropOnEquipment`, `handleLeftClick`, `handleRightClick`,
  `showItemTooltip`, `hideItemTooltip`, plus the `bindInventoryEvents` no-op and its
  caller in `characterDisplay.js`. Reachability was computed from the live roots rather
  than eyeballed; `slotInteractions.js` is the live implementation.
- **The `newGame` page bundle** — `src/pages/newGame.js`, `src/entries/newGame.js`,
  `www/views/new-game.html`, and the Vite entry. `/new-game` serves `game-intro.html`.
- **`src/lib/nostrConnect.js`** and its side-effect import.
- **`www/views/components/profile-dropdown.html`** — a 218-line template no page included;
  the live dropdown is built in JS by `nav-play.html`.
- **`/api/debug/sessions`** and `DebugSessionsHandler` — no caller anywhere.
  `/api/debug/state` is used and stays.
- Superseded by grain v0.8.1 (done with the identity work): `fetchUserMetaData.go`,
  `cache/profile_cache.go`, `src/systems/relayManager.js`, and the `gorilla/websocket`
  dependency they were the last users of.

### A bug the cleanup turned up

`initializeInventoryInteractions` was never called — and it was what installed "click
outside closes the context menu". So right-clicking a slot opened a menu that then
lingered until you opened another one. The behaviour now lives in
`slotInteractions.initSlotInteractions`, which owns the context menu; `closeContextMenu`
is exported for it. Worth a look in play: open an item menu and click away.

## Still open

**`SaveFile.LegacyVaults` and `migrateVaultsToShared`** (conditional). The schema-v4 shim
(`types/save.go`, `cmd/server/session/save.go`) folds pre-v4 per-building vault grids into
the shared vault. Once every save under `data/saves/` has been loaded and re-written at
`schema_version: 4`, the field, the migration and `tests/save/vault_migration_test.go` can
go. **Do not remove it on a hunch** — a save nobody has opened since the change still
carries the old shape, and deleting the migration early eats that player's vault.

**`isLocalConnection` skips the whitelist for any private-network IP**
(`cmd/server/auth/grain.go`, `ip.IsPrivate()` — 10./172.16–31./192.168). Since login
requires proof of key this is no longer an identity hole, but anyone on the LAN can still
play with any key regardless of the whitelist. Fine for development; worth a conscious
decision before anything resembling a public deploy.

**`auth.OwnerNpub`** currently has no production caller — `/api/debug/sessions` was the
only one. Kept because it is the documented contract of the identity gate (the gate stores
the proven npub either way) and the save-on-relays work will want handlers reading it
instead of the request body. If that does not materialise, delete it.

**Stale roadmap status.** `docs/roadmap.md` still describes M6 as active in places, which
M5.6 superseded on 2026-10-07.

*Noted and handled:* `tests/api` attaches handlers to its own mux, bypassing
`auth.RequireIdentity`, so it cannot catch an unprotected route — that limitation is now
documented in the package comment, and `cmd/server/auth` holds the real checks.
