# Gamestr listing

Gamestr's game directory is hardcoded in its site: `INITIAL_GAME_CONFIG` in
`src/lib/gameConfig.ts` of [teamgamestr/gamestr](https://github.com/teamgamestr/gamestr), keyed
`"<developer-pubkey>:<game-id>"`. A listed developer pubkey is what shows our scores as
**verified**. The DM already signs every score in their format (`cmd/server/dm/events.go`,
kind 30762 to `wss://relay.gamestr.io`); listing only adds the name, art and board tabs.

Sent as [teamgamestr/gamestr#68](https://github.com/teamgamestr/gamestr/pull/68) on 2026-11-22.

Two ways in: a PR adding the entry below, or "Claim this game" on
gamestr.io/games/pubkey-quest (a NIP-17 DM to Gamestr's pubkey `5748fbe6…2453`). The page only
exists once a first score is published.

The DM posts **one score per player**: XP in `score`, which Gamestr ranks, and each other
board in a tag of its own (`net-worth`, `quest-points`, …), so `leaderboards` gives each board a
tab, read by `scoreTag` from that one event. (It first posted a score per board, which an
unlisted game shows as one mixed list; those are retired.) The biggest-fish boards
(`biggest-<species>`, ounces) ride along as tags but stay on our own Scores tab: 24 more tabs
is too many there.

```ts
  // Pubkey Quest
  "838ec336a8e84d72408b6832f91684b40f7718fd3894f1a0bd443bfbe68ba186:pubkey-quest": {
    name: "Pubkey Quest",
    description:
      "A D&D-style RPG in your browser where your Nostr key becomes your hero. Explore, fight, quest, fish and mine; saves are events you sign.",
    image:
      "https://blossom.oslim.dev/4c0493e0ecef44eb9b5157ad212b9abffd00addf0eac66c2ea16f0c3a3778483.jpg",
    genres: ["rpg", "adventure", "multiplayer", "retro"],
    url: "https://pubkey.quest",
    developer:
      "npub1sw8vxd4gapxhysytdqe0j95yks8hwx8a8z20rg9agsalhe5t5xrqrvmq3a",
    featured: false,
    addedAt: "2026-11-22",
    leaderboards: [
      { label: "Experience", scoreTag: "score", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Net worth", scoreTag: "net-worth", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Quest points", scoreTag: "quest-points", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Places found", scoreTag: "places-discovered", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Badges", scoreTag: "badges", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Monsters slain", scoreTag: "monsters-slain", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Toughest kill", scoreTag: "toughest-kill", direction: "desc", deduplicateByPlayer: "latest" },
      { label: "Fish landed", scoreTag: "fish-landed", direction: "desc", deduplicateByPlayer: "latest" },
    ],
  },
```
