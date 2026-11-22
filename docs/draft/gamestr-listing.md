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

Our `level` tag names the board, so `leaderboardSplit` gives each board its own tab. The
biggest-fish boards (`biggest-<species>`, ounces) stay on our own Scores tab: 24 more tabs is
too many there.

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
    leaderboardSplit: {
      splitTag: "level",
      direction: "desc",
      deduplicateByPlayer: "latest",
      layout: "tabs",
      values: [
        { value: "xp", label: "Experience" },
        { value: "net-worth", label: "Net worth" },
        { value: "quest-points", label: "Quest points" },
        { value: "places-discovered", label: "Places found" },
        { value: "badges", label: "Badges" },
        { value: "monsters-slain", label: "Monsters slain" },
        { value: "toughest-kill", label: "Toughest kill" },
        { value: "fish-landed", label: "Fish landed" },
      ],
    },
  },
```
