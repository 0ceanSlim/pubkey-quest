# Shared ground

**Status:** BUILT 2026-11-21 from the roadmap's M7 spec. Part of the beta gate. Unplaytested.

What you drop lies where it fell, and anyone standing there can take it.

## The rule

- **Your drop is yours until you save.** Only you see it, and you can pick it back up. Your save
  still holds the item, so if you reload or quit without saving it's back in your pack and the
  drop never happened.
- **Saving lays it on the shared ground.** Your save no longer holds it, so it's anyone's: every
  player at that spot sees it on the next tick and can take it. On the official server only your
  latest save is playable, so you can't reload past the drop and keep both.
- **It despawns two real hours after it was left** (after the save that laid it down). A spot
  keeps at most 60 piles; the oldest goes first.
- **Quest items stay yours.** One only reaches the ground when a full pack overflows. Saved, it
  still shows to no one else, so nobody gets a quest's item without doing the quest.

Everything that lands at your feet follows the same rule. That covers drops, and the overflow from
fishing, mining, the jeweller, enchanters and smiths when your pack is full.

## Where "here" is

- **In town:** the spot presence uses (`location|district|building|room`, with a building's
  default room filled in). If you'd see another player there, you see their drops.
- **On a road:** the road and how far along it, measured from its first end. A traveller sees what
  lies within 0.05 of the road either side, the distance travellers see each other, whichever way
  they're heading.
- **The game clock doesn't matter.** Clocks are per save; what's on the ground is real time.

## How it's built

- `world/ground.go`: a `GroundStore` of piles. Each session has its own store for unsaved drops,
  and `world.World` is the shared one.
- **Pickup** takes from your own drops first, then the world's, by pile id. Whatever doesn't fit
  goes back as the same pile.
- **On a kept save:**
  - the disk save, or the relay save once a relay accepts it, moves your drops to `World`
    (`publishGround`);
  - a relay save publishes only the drops made before it was prepared (`Ground.Mark()`, stored on
    `PendingSave`). A pile covered by a save in hand takes no more drops: a later one makes its
    own.
- **Crash safety:**
  - the crash journal keeps your unsaved drops beside your unsaved play, from the same moment. A
    session restored from the journal, after a crash, an idle unload or a restart, gets them back.
    A clean load from the save doesn't.
  - `World` is written to `data/ground.json` after the journals, every two minutes and at shutdown,
    and read at start. Writing in that order means a crash can lose a pile but never duplicate one.
- **The client:**
  - `data.ground` on every action and tick lists your own piles (`yours: true`, marked ● in the
    GROUND window) and everyone's;
  - the GROUND button shows the count and lights up when anything's there.
