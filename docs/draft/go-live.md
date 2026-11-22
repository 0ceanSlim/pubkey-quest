# Going live — the operations log

Everything that has to be done **by hand, on a host or an outside service**, to take the
competition gate (and anything else built for beta) live. Code that ships with a push isn't
listed. Newest work adds to the bottom of its section; tick items off as they're done.

## The Dungeon Master
- [x] Key made; in the local `config.yml` (`dm:`). npub
      `npub1sw8vxd4gapxhysytdqe0j95yks8hwx8a8z20rg9agsalhe5t5xrqrvmq3a`.
- [ ] **Back up his nsec** (password manager). `config.yml` is the only copy.
- [x] `dm@oslim.dev` in oslim.dev's `nostr.json` (also whitelists him on wheat).
- [x] Profile (kind 0), relay lists (10002, 10050) published; picture and banner on Blossom.
- [x] Test host (nodebox, `/home/satoshi/pubkey-test`, service runs as `satoshi`) gets **no** `dm:` block: without the key the DM
      can't sign there whatever `saves.validation` says, and the nsec stays on fewer machines.
- [ ] **Copy the `dm:` block to the official host's `config.yml`.**
- [ ] At launch: switch the wiki link in `dm.about` from test.pubkey.quest/wiki to
      pubkey.quest/wiki and republish his profile.
- [ ] Drop `relay.0xchat.com` (the domain is gone) from his relay lists, and from yours;
      republish both. `offchain.pub` refused connections on 2026-11-21: check it.

## Stats, badges, highscores
- [x] **Test host `config.yml`: `stats: enabled: true`.** Done 2026-11-22; the server logs
      "Stats store on" and no Dungeon Master line.
- [ ] **Official host `config.yml`: `stats: enabled: true`,** with `saves.relay: true` and
      `saves.validation: true` (only then does the DM sign badges and scores).
- [ ] After the official server first starts with stats on: check the DM's 43 badge
      definitions (kind 30009) reached his relays (`data/dm/definitions.json` lists them).
- [ ] Back up `data/stats/` and `data/dm/` on official with `data/` (the relays hold the
      records too, but the server's copy is the fast one).
- [ ] Gamestr: ask for Pubkey Quest to be listed with the DM's npub as its developer pubkey and
      `xp` as the main board. A sponsored spot, when you want one, is the same listing.

## Official launch
- [ ] The official host set up per `docs/development/official-server.md` (service, `config.yml`,
      a self-hosted runner labelled `official`). Without the runner a tag's deploy waits in the
      queue.
- [ ] Push the first beta tag (`git tag v0.2.0 && git push origin v0.2.0`): it builds the
      release and deploys it. The binary rebuilds its own `game.db`; no `codex --migrate` there.
- [ ] `launch.closed: false` on official.
