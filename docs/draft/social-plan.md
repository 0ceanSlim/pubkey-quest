# Social: presence, friends, chat, whispers & parties — build plan

Status: **BUILT** (2026-10-12, awaiting a two-player playtest) — S0–S6 and P4; S5b (NIP-42 AUTH for DM relays) still to do. Planned 2026-10-11. This expands `party-multiplayer-plan.md` Phase 3 (presence,
the Friends panel, chat) and Phase 4 (parties) into buildable slices. Decisions come from the
maintainer; the code facts come from three code surveys run the same day. UI draft:
`docs/draft/social-ui-draft.html` (open it in a browser).

## 0. Decisions this plan builds on

1. **Strangers and friends are different lists.**
   - **Players online** is how you meet new people. Presence shows everywhere.
   - **Friends** are your mutual Nostr follows. From the friends list you message them and
     coordinate where to meet.
2. **People column:** players at your exact spot appear next to the NPCs, but only when the
   in-game clocks are within **60 minutes**. A player can *wait* to line up with someone.
3. **Light headcount everywhere:** a small `👥 N` on the main screen counts players online in
   your location. Clicking it opens the Friends panel on its **Area** tab.
4. **No teleporting and no hubs.** To party, players must stand in the same place. A
   Grand-Exchange-style market later is where people naturally gather.
5. **Joining a party snaps the follower's clock to the leader's.** The leader then moves
   the party.
6. **Chat lives in the left log.**
   - Channels: **Say**, **Area**, **Whisper** (a real Nostr DM), **Party**.
   - Actions live in a right-click / long-press menu on a name.
7. **Friends' status:** in-game status when they're playing; otherwise their latest Nostr
   event, giving *"active on Primal · 12 min ago"* from its `client` tag, or *"last seen
   3 h ago"*.
8. **(2026-10-11) Headcount** goes in the empty spot on the right of the scene's bottom
   strip.
9. **(2026-10-11) Say** only reaches players who can see you (same spot, clocks within
   60 min).
10. **(2026-10-11) Travel presence:** players on the same road appear "nearby" when they're
    within a few percent of each other along it, like POI discovery. You can stop on the
    road and watch people pass.
11. **(2026-10-11) Party sleep:** everyone needs a bed. In town that's a rented room each.
    In the wild it's a bedroll each, or one **camp set** that beds down the whole party.
12. **(2026-10-11) Group-tailored encounters** belong with party combat (P5), still to be
    talked through.

## 1. Facts the plan has to respect

| Fact | Consequence |
|---|---|
| The client sends **no ticks while its clock is paused** (combat, encounter overlays, menus): `tickManager.js:76`. | Presence needs its own heartbeat, so §2 adds a social poll that runs paused or not. |
| `GameSession` has **no lock**; handlers mutate `SaveData` freely (`session/types.go:10-85`). | Fine for presence, because readers only touch the registry. A party, where one player's action moves another, needs per-session locking (§9). |
| **The client drives the clock.** The server accepts any forward client time (`gametime.go:98-119`). | A follower's own ticks would run their clock past the leader's. Parties must override follower time on the server (§9). |
| The log is `#game-text` (209px wide, 347px tall) and has **no input**. **Combat replaces its contents** and restores a cached copy (`combatSystem.js:903-916`). | Chat lines arriving during a fight would be lost on restore, so chat is kept in its own buffer (§5). |
| `/api/publish` only whitelists kinds **0 and 10002**, and requires the signer to be the session user. | Follow (kind 3), mute (10000) and whispers (1059, signed by a throwaway key) each need their own server path (§4, §7). |
| MILL signers: **private key, bunker, Google/pomegranate** expose `nip44`; **extensions only if they support it**; **Amber's nip44 is broken** (it encrypts to the wrong key); read-only has none. | Whisper is feature-detected per signer; Amber users see "whispers need another sign-in method" (§7). |
| Grain v0.8.1 **encodes tag filters correctly** and **verifies inbound signatures**. It doesn't cache kind 3 / 10000 / 1059. | Friends and DMs are buildable now; the game keeps its own short TTL caches. |
| Grain's `FetchRelayList` can return a **stale** list (it returns on the first EOSE). | Follow/unfollow reads the newest kind 3 with `QueryEvents` across the user's relays and keeps the highest `created_at` (§4). |
| Many DM-inbox relays require **NIP-42 AUTH** before serving kind 1059; the game has no auth flow. | Whisper v1 reads relays that don't require AUTH; NIP-42 is a follow-up slice (§7). |

## 2. Slice S0 — Foundations: presence wiring and the social poll

### Server
- **Presence snapshot after every action:** `GameActionHandler` (after `UpdateSession`) calls
  `publishPresence(sess)`. The registry `cmd/server/presence` is already written and tested.
- **Normalized spot:** computed in `noteLocation` when the place changes, stored as
  `sess.PresenceSpot`. That's `location|district|building|room` with the building's default
  room filled in (`building.GetBuildingRooms`), or `""` while travelling (`!PlaceInCity`).
- **Snapshot contents:** npub, saveID, character name, race, class, level (from XP via the
  advancement table, cached once), spot, location id, district, building, absolute minute,
  in-combat flag, `LastSeen = now`.
- **On the road** (decided 2026-10-11):
  - The snapshot also carries the route (the environment id) and a **route position** in
    0–1, measured from a fixed end of the road. Your progress counts from the end you set
    out from, so a traveller from the far end is at `1 − progress`.
  - Two travellers are "nearby" when they're on the same route, within **`ROAD_NEARBY`
    (start at 0.05 = 5% of the road)** of each other, and their clocks are within 60 min.
  - Nearby travellers fill the People list on the travel screen, the same way spot-mates
    do in town.
  - A stopped traveller (`TravelStopped`) stays put while others move past, which is the
    "watch people come and go" case.
  - The headcount and Area tab treat the **whole road** as one area.
- **One envelope endpoint, `GET /api/social/poll?npub&save_id`,** behind the identity gate
  (add `/api/social/` to the protected prefixes). It:
  1. refreshes the caller's snapshot (`publishPresence`), which is the heartbeat;
  2. returns everything social since the last poll:
     ```json
     { "success": true, "data": {
         "here":  [PlayerView],          // People-column players (spot + 60 min)
         "area_count": 4,                // headcount: online in this location
         "chat":  [ChatLine],            // S4: Say / Area / Party lines for you
         "invites": [InviteView],        // P4
         "party": PartyView | null       // P4
     } }
     ```
  3. **Area list:** fetched only when the Area tab is open, via `GET /api/social/area`
     (everyone online in your location, with where they are and the clock offset).
- **`PlayerView`:** `{npub, name, race, class, level, in_combat, where, clock_offset}`.
  `where` is "Salty Anchor · Taproom". `clock_offset` is in minutes, + meaning they're ahead
  of you.
- **Housekeeping:**
  - `presence.Prune(10 min)` runs every minute (a goroutine next to the journal loop).
  - `presence.Remove` runs on `CleanupSessionHandler` and on logout, which resolves the npub
    before `ClearSession`.

### Client
- **`src/systems/socialPoll.js`:** every **3 s** while the game page is open, paused or
  not. It skips while a previous poll is in flight, and fans each result out as
  `eventBus` events:
  - `social:here` → People column
  - `social:count` → headcount
  - `social:chat` → log
  - `social:invites`, `social:party` → P4
- It runs separately from `tickManager`, so a paused or fighting player stays online.

### Edge cases
- **Several saves on one npub:** the most recently active save wins (the registry keys by
  npub).
- **Reload:** `ReloadSession` makes a new session object; presence is keyed by npub, so
  nothing dangles.
- **Closed tab:** the player goes offline after 20 s without a poll, and is pruned after
  10 min.

### Tests
- **Done:** the registry tests (spot, clock window, online window, self, latest save,
  prune).
- **New:** `publishPresence` normalizes the default room and blanks the spot while
  travelling; the poll handler returns `here`/`area_count` for two seeded sessions.

## 3. Slice S1 — Players in the People column, player card, context menu

### People column (`locationDisplay.js`)
- After NPCs and room actions, `renderPlayersHere(list)` appends one button per player:
  - new type **`player`**, colour **#4a7ab0** (muted blue, distinct from the NPC purple);
  - label `👤 Name` plus a small `⚔` when they're in combat;
  - `data-player-npub` set.
- Existing player buttons are removed before re-rendering, so NPC re-renders and the
  delta applier don't fight it. The "No one here" message is suppressed when players are
  present.
- `social:here` re-renders only the player buttons.

### Context menu (shared)
- **Generalize the inventory menu shell into `src/ui/contextMenu.js`:** `showMenu(x, y,
  items[{label, onClick, disabled, title}])`. It keeps the same styling, viewport clamping
  and click-away close.
- **Triggers:** right-click on desktop, long-press (500 ms) on touch, and plain click on a
  People-column player button.
- **Player menu items** (each enabled by its slice; earlier slices show it disabled with a
  tooltip):
  - 💬 Whisper (S5)
  - 🎉 Invite to party (P4; only at your spot)
  - ➕ Follow / ➖ Unfollow (S3)
  - 👁 View profile (S1)
  - 🔇 Mute (S6)

### Profile card (modal over the scene, the quest-modal pattern)
- New template `www/views/game/player-card.html`, included in `#scene-container`.
- 300px wide, title bar `👤 PLAYER` with ✕.
- **Top:** Nostr picture (32px, pixelated) and display name, from `/api/profile?npub=` (cached
  client-side for the session), plus the NIP-05 if present.
- **Character:** "Mara Thistlewood · Level 3 Human Druid", where they are, "⚔ in combat" if
  so.
- **Buttons along the bottom:** the same actions as the menu.

### Tests
- A JS syntax check, plus a manual playtest with two browsers (two keys) at the same inn.

## 4. Slice S2 — Headcount and the Friends panel (Area tab)

### Headcount chip
- **Placement (decided):** the empty 48px spacer on the right of the scene's bottom
  strip (`scene.html:153`), across from GROUND. While an NPC's speech box is open it
  covers the strip, and the headcount with it; that's accepted.
- **Look:** tiny, 6px, `👥 4` on a translucent dark pill. Hidden when the count is 0.
  Tooltip "4 adventurers in Goldenhaven".
- **Click:** opens the Friends panel on Area.

### The Friends panel (a right-rail tab)
- **`www/views/game/tabs/friends.html`** (`#friends-panel`).
- **game.html:**
  - add `friends` to `switchTab`'s tabs and its panel/button maps, keyed to `#btn-friends`;
  - `openFriends()` → `switchTab('friends')`;
  - `openFriends('area')` selects the sub-tab.
- **Header** `😊 FRIENDS`. **Sub-tabs `Friends | Area`**, styled like the journal's
  Quests | Badges.
- **Sized for the 200px rail:** 8px names, 7px detail, one line each, truncating.

**Area tab:**
- Fetches `/api/social/area` when opened, then every 10 s while open.
- Each row:
  - `● Name` (green dot), then `Lv3 Druid` on the right;
  - a second line: `Salty Anchor · 40 min ahead`, or `here` when they're at your spot.
- Sorted: here first, then by district.
- Right-click / long-press gives the player menu.
- Empty state: *"No one else is adventuring in Goldenhaven right now."*

## 5. Slice S3 — Friends tab (mutual follows) and Follow/Unfollow

### Server: `GET /api/friends` (identity-gated, cached 10 min per user)
1. **Your follows:** `uc.QueryEvents(ctx, core.ContactListFilter(me), cc.OwnListRelays(me))`;
   take the highest `created_at`, and collect its `p` tags.
2. **Who follows you back:** query the followees' own kind 3 in chunks of 150 authors,
   `Filter{Authors: chunk, Kinds: [3]}` on the index relays, newest per author. A followee
   is a **friend** if their list `p`-tags you. (This is bounded by your follow count. Never
   use a `#p` query on yourself: popular keys get flooded.)
3. **Status per friend:**
   - **In game:** `presence.OnlineSet(friends)` gives where they are and the clock offset.
   - **Otherwise:** their latest event of any kind, one per-author filter each, batched
     through `FetchEvents` grouped by outbox, keeping the highest `created_at` (not
     limit-1-first). Read its `client` tag. Cached 5 min.
4. **Names and pictures** via the existing profile cache (Grain user data, 1 h).
5. **Returns** `{friends: [FriendView]}`, where `FriendView = {npub, name, picture,
   in_game: PlayerView|null, last_event_at, client}`.

### Client (Friends tab)
- **Online in game first:**
  - green `●`, name, `Lv3 Druid`;
  - second line `Salty Anchor · 2h ahead` or `here`.
- **Then the rest, most recently active first:**
  - grey `○`;
  - second line `active on Primal · 12 min ago`, or `last seen 3 h ago`.
- Right-click / long-press gives the player menu (Invite only when they're `here`).
- **Footer:** `Friends are people you follow who follow you back.` plus a refresh button.

### Follow / Unfollow (from the player menu)
- **`GET /api/friends/contacts`:** the newest kind 3 event (raw), read as above.
- **Client:** add or remove one `p` tag on that event's tags (content and every other tag
  kept as they are), set `created_at = now`, sign with **`getActiveSigner()`**, and post it.
- **`POST /api/friends/contacts`** gets its own handler instead of widening
  `/api/publish`:
  - signer == session user, signature verified;
  - **it must differ from the newest list by exactly one `p` entry**, so it can never
    clobber follows made in other clients;
  - then `RoutePublish` + `PublishEvent`, and the friends cache for that user is dropped.

## 6. Slice S4 — Chat: Say and Area, in the log

### Server
- **`cmd/server/social/chat.go`:** an in-memory **mailbox per npub** (a ring of the last 50
  lines). Nothing is stored or published.
- **`POST /api/social/say`** `{channel: "say"|"area", text}`. Recipients are decided **at
  send time**:
  - **say:** `presence.Here(sender spot, clock)` plus the sender;
  - **area:** everyone online in the sender's location.

  Each recipient's mailbox gets `ChatLine{id, channel, from_npub, from_name, text, at}`.
- **Limits:**
  - ≤ 200 characters, trimmed, control characters stripped;
  - **5 lines per 10 s per sender**, then `429 "Slow down"`;
  - an empty or whitespace-only line is refused.
- **Delivery:** the social poll drains the caller's mailbox (`chat` in the envelope).

### Client — the log gains an input row
- **`game-text.html`:** the scroll area keeps its 7px log. Below it goes a **16px input
  row**:
  - a channel button (`Say ▾`) cycling Say / Area / Party / Whisper→name;
  - a text input (7px font, Win95 inset);
  - Enter sends.
- **Shortcuts:** `/s`, `/a`, `/p`, `/w name`, and `/r` (reply to the last whisper).
- **Chat lines in the log:** `[Say] Mara: hello`, with the channel tag coloured:

  | Channel | Colour |
  |---|---|
  | Say | white |
  | Area | sky |
  | Party | green |
  | Whisper | pink |

  Your own lines are dimmed.
- **During combat:** chat lines also go into a `_chatHistory` buffer. When the log is
  restored after a fight, lines that arrived during it are appended again, so nothing is
  lost. They also show live in the combat log.
- **Mute:** lines from muted npubs are dropped client-side (S6).
- **Typing never triggers game hotkeys,** and the input stops propagation.

## 7. Slice S5 — Whisper (NIP-17 DMs)

### Capability
- Available when `typeof getActiveSigner()?.nip44?.encrypt === 'function'` **and** the
  method isn't `nip55`.
- Otherwise the Whisper item is disabled: *"Whispers need a sign-in that supports encrypted
  messages (key, bunker, Google, or a NIP-44 extension)."*

### Sending (browser; MILL signers don't expose a secret key, so the NIP-59 layers are built by hand)
1. **Rumor:** unsigned kind 14 `{pubkey: me, created_at: now, tags: [["p", to]],
   content: text}` with `id = getEventHash`.
2. **Seal:** kind 13, `content = signer.nip44.encrypt(to, JSON.stringify(rumor))`,
   `created_at` randomised up to 2 days back, signed by `signer.signEvent`.
3. **Wrap:** `nip59.createWrap(seal, to)` (random key; kind 1059, `p` = to).
4. **A copy to yourself:** a second seal and wrap addressed to `me`, so your other clients
   see sent messages.
5. **No client tag** on the seal or wrap (metadata leak).

### Server: `POST /api/dm/send` `{wraps: [event, event]}`
- Each wrap: kind 1059, a valid signature, exactly one `p` tag, content ≤ 64 KB.
- Rate limit: 10 per minute per sender.
- **Relays are picked by hand:** `ResolveRelays(recipient).DMInbox`, else `.Inbox`, then
  `uc.PublishEvent(ctx, wrap, relays)`. Don't use `RoutePublish`: it resolves the throwaway
  author key, adds broadcast relays, and in pinned mode skips the recipient altogether.

### Receiving: `GET /api/dm/inbox?since=`
- Queries `Filter{Kinds: [1059], Tags: {"p": [me]}, Since: since − 2 days}` on your
  `DMInbox` (else `Inbox`), and returns the raw wraps.
- The client polls every **15 s**, separate from the 3 s social poll because of relay load.
- **The client unwraps:**
  1. `nip44.decrypt(wrap.pubkey, wrap.content)` gives the seal;
  2. `verifyEvent(seal)`;
  3. `nip44.decrypt(seal.pubkey, seal.content)` gives the rumor;
  4. check `rumor.pubkey === seal.pubkey`.
- Dedupe by rumor id, kept in the session's memory.
- Shown in the log as pink `[Whisper] Mara: …` (`[To Mara]` for your own).
- **Consent prompts:** private-key signers ask consent for the `dms` category the first
  time; that's expected, and they offer "this session".

### Follow-up slice S5b — NIP-42
- For DM relays that require AUTH: `uc.AuthChallenge(ctx, url)`, the browser signs kind
  22242, then `uc.Authenticate(ctx, url, signed)`.
- Two endpoints, mirroring Grain's own `/api/v1/client/auth/*`.
- Until then, AUTH-only relays are skipped and noted in the log once.

## 8. Slice S6 — Mute

- **`GET /api/mutes`:** your newest kind 10000 (the same newest-wins read as kind 3).
- **Toggling a mute:** the client adds or removes one **public** `p` tag; encrypted private
  entries in `content` are kept byte-for-byte. It signs and posts to
  **`POST /api/mutes`**, which applies the same "exactly one change" guard as contacts.
- **Effects:**
  - muted players' Say / Area / Party lines are dropped client-side;
  - their whispers are hidden;
  - they're still listed in Area, marked `🔇`.

## 9. Phase P4 — Parties

### Foundations (P4a)
- **A per-session mutex:** `GameSession.mu sync.Mutex`, held for the whole of
  `GameActionHandler` and the combat, POI, session and debug handlers. It also closes the
  journal race that exists today.
- **No cross-locking:** a leader's request never touches a follower's session directly. It
  appends a **`PartySync`** record (the leader's new place, clock and travel fields, plus an
  optional "you arrived" or "rest" marker) to the follower's **pending queue**. That queue
  has its own tiny mutex.
- **The follower's next request** (its tick or its social poll) applies pending syncs at the
  start, under its own lock.

### Party registry (`cmd/server/social/party.go`, memory only)
- `Party{ID, Leader npub, Members [npub:saveID], CreatedAt}`.
- Maps: `partyByNpub`, `invites (to → {from, party, expires 2 min})`.

### Invites
- **`POST /api/party/invite {to}`:**
  - both online;
  - the **same spot** (presence), any clock;
  - the target isn't in a party;
  - the inviter's party has fewer than 5 members.
- The invite shows up in the target's poll (`invites`). The target's log shows
  `🎉 Mara invites you to her party  [Accept] [Decline]`.
- **`POST /api/party/accept|decline {from}`:**
  - accepting re-checks the same spot;
  - the follower's clock **snaps to the leader's** (a PartySync with the leader's
    absolute time);
  - both get a `party` view.

### While in a party
- **The leader moves the party.** After a leader action that changed place, time or travel
  (compare the place key, absolute time and travel fields before and after), a PartySync is
  queued to each follower.
- **Followers can't move on their own.** A deny map, like `combatBlockedActions`, covers
  `move`, `enter_building`, `exit_building`, `move_to_room`, `start_travel`, `stop_travel`,
  `resume_travel`, `turn_back`, `wait`, `sleep`, `rest`, `advance_time` and `play_show`. It
  answers *"Your party leader is leading the way."* Followers can still use items,
  inventory, dialogue, shops and their vault.
- **Follower clocks:**
  - the follower's own `update_time` ignores the client's time and uses the leader's
    absolute time;
  - effects, hunger and fatigue advance on the follower's own character through
    `AdvanceTime(elapsed)`, never by assigning the time;
  - followers are exempt from auto-pause.
- **Applying a sync to a follower:**
  - set place and travel fields;
  - `AdvanceTime` by the elapsed time;
  - refresh the building and NPC caches;
  - on travel arrival, run a follower variant of `processArrival`, so discovery, XP and
    music land on *their* save.
- **Travel progress:** computed on the leader only, and copied to followers (so one
  member's Athletics doesn't pull the party apart).
- **Sleeping, waiting and resting:**
  - wait and rest apply to all members;
  - **sleep (decided):** everyone needs a bed.
    - **In town:** a room rented at that inn, each.
    - **In the wild:** a bedroll each, or **one camp set** in any member's pack, which
      beds down the whole party at bedroll comfort.
    - Otherwise the leader's sleep is refused: *"Not everyone in your party has a bed."*
  - **New item `camp-set`:** a tent and cookware bundle. Heavier and pricier than a
    bedroll, with tags `camping` and `party-camp`. Sold where bedrolls are.
- **The client hears** `data.party_sync` on the follower's next tick or poll. It does a
  full refresh and `smoothClock.syncFromBackend(time, day, true)`. A move that only
  changes rooms produces no delta, so the flag is what carries it.
- **Encounters:** rolls run on the leader only.
  - **Random and authored encounters are suppressed while in a party** (decided
    2026-10-11) until encounters built for parties exist (P5). The roads are safe for
    groups.
  - Quest-stage encounters for a quest only the leader holds are also suppressed until P5.

### Leaving
- **`POST /api/party/leave`:**
  - a follower who leaves keeps the party's time and place;
  - if the leader leaves, the next member is promoted;
  - a party of one disbands.
- **Kick and promote:** `POST /api/party/kick|promote` (leader only).
- **Disconnects:** logging out, or a member going stale (presence offline for 60 s), leaves
  automatically.

### UI
- **Party HUD:** a compact strip on the scene's left edge, under the quest tracker. One row
  per member:
  - 👑 for the leader;
  - the name (6px);
  - a 2px HP bar.

  Clicking a row gives the player menu. Hidden when solo.
- **Party chat** channel (green).
- **Log lines** for joins, leaves, promotions and syncs: `🚶 Your party moves to the Salty
  Anchor.`

## 10. Build order and sizes

| Slice | What | Size |
|---|---|---|
| S0 | presence wiring, spot normalization, social poll, prune/remove | S |
| S1 | People-column players, shared context menu, profile card | M |
| S2 | headcount chip, Friends panel + Area tab | S–M |
| S3 | friends (mutual follows), status, follow/unfollow | M |
| S4 | chat Say/Area, log input row, combat-safe buffer | M |
| S5 | whisper NIP-17 send/receive (+ S5b NIP-42) | M (+S) |
| S6 | mute list | S |
| P4a | session mutex, PartySync queue | M |
| P4b | invites, party registry, clock snap, follow-the-leader, HUD, party chat | L |
| P5 | party combat | XL |

## 11. Open questions

1. ~~Headcount spot~~ **Decided:** the bottom strip's right-hand spacer.
2. ~~Say range~~ **Decided:** only players who can see you.
3. ~~Party sleep~~ **Decided:** everyone needs a bed; a camp set covers a party in the wild.
4. ~~Encounters while partied~~ **Decided:** parties get no random or authored encounters
   until party-ready encounters are authored (P5). P4 ships on its own.
5. ~~Area scope on the road~~ **Decided:** a road is one area; "nearby" is within 5% of the
   road (tunable).
