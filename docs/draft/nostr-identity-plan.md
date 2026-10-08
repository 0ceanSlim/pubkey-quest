# Nostr identity in Pubkey Quest — plan

Written 2026-10-08. Scope agreed with the maintainer. Supersedes the identity half of
`grain-integration-handoff.md` (that doc's G2); the save-on-relays work (G3/G4) is
deliberately **not** in here.

Pins `github.com/0ceanslim/grain v0.8.1` (main, `04e187b`). API facts below were read
out of the v0.8.1 module, not assumed.

## Scope

**In:** a player's identity and where their events live.

- profile: display name + picture, **editable in-game** (kind 0)
- the user dropdown, filled out like Grain's reference client
- relay management: NIP-65 inbox/outbox editing, so a player can change relays without
  leaving the game
- a **pinned-relay mode**: "save this game to these relays and read from these", i.e.
  opt out of outbox routing
- the NIP-89 client tag on everything we publish

**Out, by decision:**

- **media servers / uploads.** A kind 0 stores a picture *URL*, not an image. The editor
  takes a URL; a player who needs hosting is pointed at a client that does it (Grain's
  reference client). Revisit only if it becomes a real complaint.
- **social surfaces.** Notes, long-form, a library of player-published books, writing on
  parchment with pen and paper — all later, and all intended to arrive *through the game's
  own fiction* rather than as a generic feed. Brainstormed when we get there.
- saves on relays — separate plan.

## N0 — Prove the key at login (do first; security)

`HandleLogin` (`cmd/server/auth/grain.go:83`) accepts whatever `public_key` the body
carries and mints a session for it. No signature, no challenge. That makes the ownership
binding shipped in `2fc9cf0` **bypassable in one step**: post a victim's npub, get their
cookie, and the middleware then authorises you correctly because you *are* that session.
The whitelist is the only friction, and a victim who is a tester is whitelisted by
definition (`isLocalConnection` skips it outright on a LAN).

The fix already exists, written and uncommitted on branch
`worktree-m56-p1-multi-entity-combat` as M5.6 G1:

- `cmd/server/auth/challenge.go` — `GET /api/auth/challenge` issues a single-use nonce;
  the browser signs a NIP-98 event (kind 27235, tags `u`, `method`, `challenge`) through
  mill; `HandleLogin` verifies with `core.VerifyEventSignature` before creating a session.
- `cmd/server/auth/auth_test.go`

**Open coordination item:** that slice needs to reach main ahead of the P1 combat work it
currently shares a branch with. Reimplementing it here would only guarantee a conflict.

### Resolve the duplicated identity middleware in the same pass

Two implementations of one control now exist:

| | `auth/owner.go` (main, shipped) | `auth/identity.go` (worktree, uncommitted) |
|---|---|---|
| shape | `RequireOwner` per route, 43 explicit registrations | `RequireIdentity` wraps the whole mux off a prefix list |
| forgetting a route | leaves it **open** | leaves it **protected** |
| 401 body | plain | `auth_required`, which drives silent re-login |

Keep `identity.go` — covering-by-default is the right failure mode for a security control,
and its `auth_required` contract pairs with the challenge flow. Port across from
`owner.go` before deleting it:

1. the `OwnerNpub(r)` context helper — `/api/debug/sessions` scoping depends on it and
   breaks silently without it
2. the public-route reasoning, in particular that **`/api/character` must stay public** or
   the `/discover` page (enter any npub, see its character) stops working
3. the eight tests in `tests/auth/owner_test.go`

## N1 — Profile, read and write

Grain v0.8.1 already does the fetching, so this is mostly deletion.

- `data.GetUserDataForSession(pubkey) (*nostr.Event, *core.Mailboxes, error)` returns
  metadata and mailboxes together.
- `cache.GetUserData` / `SetUserData` / `GetUserDataWithAge` / `IsExpiringSoon` is the
  profile cache.
- `CreateUserSession` already warms both in the background at login, non-blocking.

**Retire:** `cmd/server/utils/fetchUserMetaData.go` (151 lines, a hand-rolled websocket
fetch) and `cmd/server/cache/profile_cache.go` (118 lines, our own TTL cache). Callers are
`cmd/server/api/profile.go`, `app/app.go`, `auth/init.go`.

**Edit (publish kind 0):** the browser signs, the server verifies and routes — the pattern
in Grain's `client/api/profile.go` `PublishSignedHandler`:

```
mill signs kind 0  →  POST /api/profile/publish
                   →  core.VerifyEventSignature(ev)
                   →  uc.RoutePublish(ev)  →  uc.PublishEvent(ctx, ev, relays)
```

The server never holds a key. Picture is a URL field. Display name and picture are the
only fields that must work; the rest of kind 0 can be passed through untouched so we never
clobber fields set by another client.

**Client tag:** every event we publish gets NIP-89 `["client", …]`. One helper, applied at
the publish seam so it cannot be forgotten per call site.

## N2 — Relays

- Read lists: `Client.FetchUserRelayLists(pubkey)`, `Client.FetchRelayList(pubkey, kind)`.
- Per-player state: `cache.GetUserClientRelaysWithPermissions`,
  `AddClientRelayWithPermissions`, `RemoveClientRelay`, `ClearClientRelays`,
  `SetUserClientRelaysFromMailboxes`.
- Pinned mode is already an API, not new routing work: `uc.PinFixedRelays(read, write)`,
  `uc.FixedRelaysEnabled()`, `uc.ClearFixedRelays()` — all per-player on the UserContext.

**Retire** `src/systems/relayManager.js`, which is localStorage-only and so invisible to
the server and to every other client.

Editing a player's own relay list means publishing kind 10002, through the same
sign → verify → route path as the profile.

## UI

Port the flows from Grain's reference client — `profile-page.html`, `user-dropdown.html`,
`profile-page.js`, `relay-manager.js`, `nostr-publish.js` — and **restyle completely**:
win95 beveled `clip-path` panels with inset/outset borders, the Dogica pixel font, theme
`--color-*` tokens (MILL's `--mill-*` are mapped from them in `src/systems/millLogin.js`),
`image-rendering: pixelated`. The look comes from Pubkey Quest, never Grain's stylesheet.

Skip `media-servers.js` (out of scope).

## Known constraints

- **Rate limits.** Every player's REQs leave from the server's one IP. Relays cap per IP
  (nos.lol: 200 per 10s, with auto-bans), grain has no REQ pacing yet and discards the
  `rate-limited:` reason from `CLOSED` frames, so we get no signal. Prefer bounded
  `QueryEvents`/`FetchEvents` over long-lived per-player subscriptions, batch filters into
  one REQ, and lean on the profile cache rather than re-querying.
- **Pre-1.0 library.** v0.8.1 changed the AUTH API and the `CreateUserSession` /
  `CreateSession` signatures. Keep the pin exact.
- `collectLatestReplaceable` is still unexported upstream, and there is **no kind-3 helper** —
  both matter for later plans, neither blocks this one.
