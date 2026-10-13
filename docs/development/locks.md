# Keys, locks and lockpicking

How locks work, how to put one in content, and how to add a new lockpicking minigame.
Code: `types/locks.go`, `cmd/server/game/locks/`, `cmd/server/api/game/locks.go`,
`src/ui/lockpicking.js`, `src/styles/lockpicking.css`.

## A lock

```json
{ "key": "silver-key", "dc": 16, "jam_on_miss": 5 }
```

| Field | Meaning |
|---|---|
| `key` | The item that opens it. Keys carry the `key` tag. A key on a keyring still counts. |
| `dc` | Thieving DC to pick it. `0` (or missing) means it can't be picked, only its key opens it. |
| `jam_on_miss` | When a pick breaks, a Thieving check is rolled against the DC. If it misses by this much or more, the lock jams and only the key opens it until the next in-game day. `0` means it never jams. |

**Where locks go:**
- **A room's door** (city JSON):
  `"access": { "lock": { ... } }`.
- **A POI or quest-site node:**
  `{ "type": "lock", "text": "...", "lock": { ... }, "success_next": "<opened>", "failure_next": "<walk away>" }`.
  An empty `failure_next` leaves the walk.
- **A spare room you sleep in with its key:** add `"sleep": true` to the room. It needs no rental.

The legacy `"access": { "key": "..." }` (key only) still works.

## The two ways to pick

Both use the same rules:
- a failure breaks one `lockpick`;
- a broken pick runs the jam check;
- each pick used costs `locks.PickMinutes` (10) of game time;
- picked-open and jammed state lasts the in-game day.

That state lives in session memory (`GameSession.Locks`) and is never saved.

| Method | How |
|---|---|
| **Quick attempt** | One Thieving roll against the DC: `pick_lock {room_id}`, or a lock node's `#quick` choice. |
| **Pick the lock** | The minigame. Below. |

**Thieving** is the effective skill: stats, effects, and carried tools. A tool's `skill_bonus` (the Thieves' Kit has `{"thieving": 2}`, which is +1 on the roll) counts while it's in your pack.

## The minigame protocol

Everything goes through `POST /api/game/action`, which holds the session lock.

**Starting:** `lock_begin {kind: "room"|"poi", room_id | node, style: "tumblers"}`.
- The server checks you're at the lock, it isn't open or jammed today, it's pickable, and you have a lockpick.
- It lays out the hidden puzzle and stores the session as `GameSession.LockSession`.
- It replies with `data.lock` (the public view), `data.picks` and `data.kit`.

**Moving:** `lock_act {session_id, version, act: "lift"|"inspect"|"give_up", pin}`.
- `version` must equal the session's current version. Every accepted move bumps it.
- A move with an old version gets `data.stale: true` and the current view, and changes nothing. That's what stops a double tap or a retried request from costing or opening twice.
- Invalid moves (a set pin, a pin that doesn't exist, a session that's over) are refused without changing anything.
- The server applies the consequences exactly once: uses up the pick, passes time, remembers the lock open or jammed.
- It replies with `data.lock`, `data.picks` and `data.lines`.
- For a lock node, once the session ends it also sends `data.step`: the walk through the door, or back to the lock's choices.

**Session status:** `active`, then `opened`, `jammed`, `no_picks` or `abandoned`. Any other game action abandons it for free.

**The public view** is `session_id, name, style, version, status, strain, strain_max, broken, inspected, tier`, plus the style's own fields. It never includes the solution.

## Tumblers

- **Pins:** 3–6 by DC: ≤10 → 3, ≤13 → 4, ≤16 → 5, otherwise 6.
- **Binding order:** hidden. Lift the binding pin and it sets.
- **Wrong lift:** drops every set pin and adds 1 strain.
- **Strain budget:** pins − 1 + Thieving modifier, minimum 2. Reach it and the pick snaps, and the next pick meets a reseated lock (a new order).
- **Inspect:** once per pick. It's a Thieving check vs the DC; on a success it rules out one pin that isn't binding right now.
- **Glint:** at Thieving modifier +2 and up, the binding pin sometimes glints (15% per point above +1, at most 60%).

**Style fields in the view:** `pins`, `set[]`, `ruled_out[]`, `glint?`.

## Adding a style (Sweet Spot, Wards)

1. Implement the `puzzle` interface in `cmd/server/game/locks` (`reseat`, `lift`, `inspect`, `view`) with the style's hidden state. Keep the solution out of `view`.
2. Add the style to the switch in `NewSession`, with its own strain budget.
3. Draw it in `src/ui/lockpicking.js`. The window, tray, message panel, status strip and actions are shared, so only the viewport and its controls change.
4. Add deterministic tests next to `locks_test.go` (seeded rng; the tests sit inside the package so they can read the hidden state).
