# Pubkey Quest - Deployment & Versioning Strategy

This document outlines the multi-server deployment architecture, versioning strategy, and CI/CD pipeline for Pubkey Quest.

## Server Environments

Pubkey Quest runs three separate server environments with different purposes, access controls, and save systems.

### 1. Test Server (`test.pubkey.quest`)

**Purpose**: Development and testing environment

**Configuration**:

- Runs compiled binaries (built by CI/CD)
- Debug logging **always enabled**
- Auto-deployed on every push to main

**Save System**:

- Saves stored **on disk only** (no Nostr relay integration)
- No event ID tracking
- Full local save file management

**Access Control**:

- **Whitelist by pubkey** (curated list)
- Restricted to developers and select testers

**Deployment**:

- **Auto-deploy on every push** to main via self-hosted GitHub Actions runner
- Build failure stops deployment — old binaries keep running
- Test failures are reported but don't block deployment
- See `setup-runner.md` for self-hosted runner setup

**Timeline**: Active now

---

### 2. Official Server (Primary Domain - TBD)

**Purpose**: Production environment for official releases

**Configuration**:

- Runs **versioned releases** (compiled binaries)
- Production optimizations enabled
- Stable, tested code only

**Save System**:

- Saves tracked via **Nostr event ID**
- Saves published to **official Nostr relays**
- Server fetches saves from official relays only
- Versioned save compatibility tracking

**Access Control**:

- **Alpha**: HappyTavern members only
- **Beta+**: Free public access

**Deployment**:

- Deploy on **manual trigger only** via `deploy-production.yml` workflow
- Requires version input (e.g. `0.1.0-alpha`)
- All tests must pass before deploy
- Tags the commit with the version

**Timeline**: Launches with 1st alpha release

---

### 3. Modded Server (deferred)

**Purpose**: Community server with relaxed save restrictions

**Timeline**: Launches with beta release

---

## CI/CD Pipeline

All CI/CD is handled by **GitHub Actions**. Three workflows:

### `ci.yml` — Every push and PR

Runs on `ubuntu-latest` (GitHub-hosted runner):

1. Checkout, setup Go 1.24, setup Node 18
2. `npm install` + `npm run build` (frontend)
3. `go build -o codex ./cmd/codex` + `./codex -migrate` (database)
4. `./codex -validate` (game data validation)
5. `swag init ...` (swagger generation)
6. `go build -o pubkey-quest ./cmd/server` (server compiles)
7. `go test ./tests/...` (API tests)

**All steps must pass** — blocks PR merge on failure.

### `deploy-test.yml` — Push to main

Runs on `self-hosted` runner (the Ubuntu server):

1. **Build steps** (failure = workflow fails, old binaries stay running):
   - `npm install` + `npm run build`
   - Build codex + migrate database
   - `swag init` (swagger docs)
   - Build server binary with version ldflags
2. **Tests** (continue-on-error, don't block deploy):
   - API tests
   - Game data validation
3. **Deploy** (only reached if build succeeded):
   - `sudo systemctl restart pubkey-quest-test`
   - `sudo systemctl restart codex`
4. **Report**: Write test results to job summary

### `deploy-production.yml` — Manual trigger only

Stub workflow — will be fleshed out at alpha release. Requires version input.

---

### Restart warnings

Before a deploy stops the server, it warns everyone online and gives them time to save
(`cmd/server/maintenance`):

1. The deploy writes `data/restart-notice.json` (`{"in_seconds": N, "version": "…"}`) in the
   service's working directory.
2. Within about 2 seconds, every player sees a banner with a countdown and a **Save now**
   button.
3. The server answers in `data/restart-ack.json` with how many players are online. With nobody
   online the deploy restarts at once; it also stops waiting early if everyone leaves.

| Deploy | Wait | Repo variable |
|---|---|---|
| test.pubkey.quest (every push to main) | 60 s | `TEST_RESTART_GRACE` |
| pubkey.quest ("Deploy: pubkey.quest") | 300 s | `PROD_RESTART_GRACE` |

Release builds (the official server) check GitHub's latest release every hour. When a newer
one is out, the server logs it, and the settings tab's version line shows that an update is
ready to promote.

## Versioning

Version is derived entirely from **git state** — no version files to maintain.

### How It Works

`git describe --tags --always --dirty` produces the version string at build time:

| Git state | Example output |
|-----------|---------------|
| No tags yet | `abc1234` |
| No tags, dirty tree | `abc1234-dirty` |
| On a tag | `v0.1.0-alpha` |
| 3 commits after tag | `v0.1.0-alpha-3-gabc1234` |
| After tag, dirty | `v0.1.0-alpha-3-gabc1234-dirty` |

### Build-Time Injection

Version injected at compile time via Go ldflags:

```bash
VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X main.Version=${VERSION}" -o pubkey-quest ./cmd/server
```

Both binaries support `-version` flag:

```bash
./pubkey-quest -version   # pubkey-quest abc1234
./codex -version          # codex abc1234
```

### Build Contexts

| Context | Version string | How |
|---------|---------------|-----|
| `air` (dev) | `dev` | No ldflags, uses default |
| CI (`ci.yml`) | `ci-abc1234` | ldflags with fallback prefix |
| Deploy (`deploy-test.yml`) | `abc1234` or tag | ldflags from `git describe` |
| Production (future) | `v0.1.0-alpha` | ldflags from git tag |

### Tagging Releases

When ready for alpha, tag the commit:

```bash
git tag v0.1.0-alpha
git push origin v0.1.0-alpha
```

All subsequent builds automatically pick up the tag via `git describe`.

---

## Deployment Infrastructure

All servers run on **Cloudflare Argo Tunnel** as Linux systemd services.

### Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Linux Server                                                │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Test Service │  │ Codex Svc    │  │ Official Svc │      │
│  │ (Binary)     │  │ (Binary)     │  │ (Binary)     │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
│  ┌──────┴─────────────────┴─────────────────┴───────┐      │
│  │ Cloudflare Argo Tunnel (cloudflared)             │      │
│  └──────┬───────────────────┬─────────────────┬─────┘      │
└─────────┼───────────────────┼─────────────────┼────────────┘
          │                   │                 │
          ▼                   ▼                 ▼
  test.pubkey.quest   codex.pubkey.quest   pubkey.quest
```

### Systemd Service Templates

Located in this directory:

| Template | Service | Directory |
|----------|---------|-----------|
| `pubkey-quest-test.service.template` | Test server | `/home/pubkey-test/` |
| `codex.service.template` | Codex (dev tools) | `/home/pubkey-test/` |
| `pubkey-quest.service.template` | Production server | `/home/pubkey-quest/` |

### Self-Hosted Runner

The `deploy-test.yml` workflow runs on a self-hosted GitHub Actions runner installed on the server. See `setup-runner.md` for installation instructions.

---

## Server Directory Layout

### `/home/pubkey-test/` (test + codex)

Full git checkout. The self-hosted runner checks out code here:

```
/home/pubkey-test/
├── config.yml              # Test server config (port, debug_mode: true)
├── codex-config.yml        # Codex config
├── pubkey-quest            # Built server binary
├── codex                   # Built codex binary
├── www/
│   ├── game.db             # Migrated database
│   └── dist/               # Built frontend
├── game-data/              # JSON source data
├── data/saves/             # Test save files
└── ... (full repo)
```

### `/home/pubkey-quest/` (production, deferred)

Same layout, different config (no debug_mode, different port).

---

## Access Control Implementation

### Alpha Phase (Official Server Launch)

**HappyTavern Member Verification**:

- Option 1: Maintain whitelist of HappyTavern member npubs
- Option 2: Check for HappyTavern badge/credential on Nostr profile
- Option 3: Token-based access (distributed to members)

**Test Server**:

- Hardcoded pubkey whitelist in `config.yml` or database
- Check npub on login
- Reject non-whitelisted users

---

## Implementation Checklist

### Phase 1: Test Server CI/CD (Current)

- [x] Basic server running
- [x] CI workflow (ci.yml) — build + test on every push/PR
- [x] Deploy-test workflow — auto-deploy on push to main
- [x] Self-hosted runner setup guide
- [x] Systemd service templates
- [x] Versioning system (git-based + ldflags)
- [x] Codex `-validate` flag for CI
- [ ] Self-hosted runner installed on server
- [ ] Services created and enabled
- [ ] Pubkey whitelist enforcement
- [ ] Disk-based save system

### Phase 2: Alpha Prep (Official Server)

- [ ] Deploy-production workflow fleshed out
- [ ] Nostr relay integration for saves
- [ ] HappyTavern member verification
- [ ] Production service setup
- [ ] Version tagging automation

### Phase 3: Beta & Beyond

- [ ] Modded server
- [ ] Release notes generation
- [ ] Nostr announcement posting (Dungeon Master npub)

---

**Last Updated**: 2026-02-06
**Status**: CI/CD pipeline implemented, awaiting runner setup on server
