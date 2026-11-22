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
- [x] **Copy the `dm:` block to the official host's `config.yml`.** Done 2026-11-22.
- [ ] At launch: switch the wiki link in `dm.about` from test.pubkey.quest/wiki to
      pubkey.quest/wiki and republish his profile.
- [ ] Drop `relay.0xchat.com` (the domain is gone) from his relay lists, and from yours;
      republish both. `offchain.pub` refused connections on 2026-11-21: check it.

## Stats, badges, highscores
- [x] **Test host `config.yml`: `stats: enabled: true`.** Done 2026-11-22; the server logs
      "Stats store on" and no Dungeon Master line.
- [x] **Official host `config.yml`: `stats: enabled: true`,** (done 2026-11-22, plus `wiki:`) with `saves.relay: true` and
      `saves.validation: true` (only then does the DM sign badges and scores).
- [ ] After the official server first starts with stats on: check the DM's 43 badge
      definitions (kind 30009) reached his relays (`data/dm/definitions.json` lists them).
- [ ] Back up `data/stats/` and `data/dm/` on official with `data/` (the relays hold the
      records too, but the server's copy is the fast one).
- [ ] Gamestr: listed by a PR to teamgamestr/gamestr's `gameConfig.ts` or "Claim this game"; the
      entry is drafted in `docs/draft/gamestr-listing.md`. A sponsored spot is `featured` there.
- [ ] After the release with update posts: the DM's first run posts only the newest update as a
      kind 1 note (`data/dm/posts.json` records each); every later update entry posts once when
      its release goes live.

## Official launch
- [x] The official host set up per `docs/development/official-server.md` (service, `config.yml`,
      a self-hosted runner labelled `official`). Without the runner a tag's deploy waits in the
      queue.
- [x] Push the first beta tag (`git tag v0.2.0 && git push origin v0.2.0`): it builds the
      release and deploys it. The binary rebuilds its own `game.db`; no `codex --migrate` there.
- [x] v0.2.0 deployed 2026-11-22. The DM published no definitions: Grain sends by the URL it was given
      but keys connections by the normalized one, so his `wss://…/` relays all failed "not connected".
      Fixed in `utils.PublicRelays` (all publishing); ships in v0.2.1. After it: check
      `data/dm/definitions.json` lists 43.
- [x] Nightly backup cron on the nodebox: `data/` + `config.yml` → `/home/satoshi/pubkey-backups`
      (30 days). Same disk: copy it somewhere else too.
- [ ] `launch.closed: false` on official, then `sudo systemctl restart pubkey-quest`.
