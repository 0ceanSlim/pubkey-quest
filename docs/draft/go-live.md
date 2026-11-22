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
- [ ] **Copy the whole `dm:` block to the test host's `config.yml`** (same key: test and
      official share the DM).
- [ ] **Copy the `dm:` block to the official host's `config.yml`.**
- [ ] At launch: switch the wiki link in `dm.about` from test.pubkey.quest/wiki to
      pubkey.quest/wiki and republish his profile.
- [ ] Drop `relay.0xchat.com` (the domain is gone) from his relay lists, and from yours;
      republish both. `offchain.pub` refused connections on 2026-11-21: check it.

## Stats, badges, highscores
- [ ] **Test host `config.yml`: `stats: enabled: true`.** Without it test counts nothing.
- [ ] **Official host `config.yml`: `stats: enabled: true`,** with `saves.relay: true` and
      `saves.validation: true` (only then does the DM sign badges and scores).
- [ ] After deploying the badge build: `codex --migrate` on each host (loads `badges.json` and
      `badge-media.json`).
- [ ] Gamestr: ask for Pubkey Quest to be listed with the DM's npub as its developer pubkey and
      `xp` as the main board. A sponsored spot, when you want one, is the same listing.

## Official launch
- [ ] `launch.closed: false` on official.
