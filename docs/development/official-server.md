# The official server (pubkey.quest)

How the official server is built, configured and deployed. The design is in
`docs/draft/relay-saves-plan.md`.

## What runs there

**One binary.** It's built by the **Release** workflow (`.github/workflows/release.yml`)
after CI passes on `main`:
- tagged `v0.1.N`, the version going up automatically;
- published as a GitHub release with `SHA256SUMS`.

**What the binary carries:** the frontend bundle, the templates and art, and the game data.

**What happens when it starts:**
1. It unpacks those into `.release/<version>/` next to where it runs.
2. It links the persistent `data/` folder in. That folder holds saves, the ledger,
   journals and reports, and it is never touched by an upgrade.
3. It rebuilds `game.db` from the game data it carries.
4. It serves from there.

There's no separate `codex --migrate` step, and no repo checkout on the server.

**Test is different:** `test.pubkey.quest` keeps deploying every push from a checkout
(`deploy-test.yml`), and remains the debug server.

## One-time setup

1. **Create a service directory**, e.g. `/opt/pubkey-quest`, and give it a `config.yml`:
   ```yaml
   server:
     port: 8585
     tls: false
     debug_mode: false   # required: the official server refuses to start with it on
   saves:
     relay: true         # signed saves on the players' relays
     validation: true    # only the save this server last accepted is playable
     max_slots: 5
   ```
2. **Create a systemd unit** `pubkey-quest.service`:
   ```ini
   [Unit]
   Description=Pubkey Quest (official)
   After=network-online.target

   [Service]
   WorkingDirectory=/opt/pubkey-quest
   ExecStart=/opt/pubkey-quest/pubkey-quest
   Restart=on-failure
   User=pubkeyquest

   [Install]
   WantedBy=multi-user.target
   ```
3. **Register a self-hosted GitHub runner on that machine** with the extra label `official`.
   - The promote workflow only runs there.
   - The runner user needs `sudo systemctl stop/start pubkey-quest` and write access to
     `/opt/pubkey-quest`.
4. **Point the web server or proxy for `pubkey.quest` at the port**, and drop the redirect to
   test.

## Deploying

Run **Actions → "Deploy: pubkey.quest" → Run workflow**, with a version or `latest`.

The workflow then:
1. downloads that release's binary;
2. checks it against `SHA256SUMS`;
3. keeps the old binary as `pubkey-quest.previous`;
4. swaps the new one in and restarts.

**Rolling back:** copy `pubkey-quest.previous` back and restart, or promote an older version.

## Rules that hold once it's live

- **No wipes.** Every save the official server accepted stays playable. A change to the save
  format is a new `savecodec` version plus a migration (`cmd/server/game/savecodec`), never an
  edit to an old one.
- **Keep `data/` backed up.** `data/ledger/` is the official server's only record of which save
  is real for each character.
