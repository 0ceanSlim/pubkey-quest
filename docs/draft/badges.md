# Badges — the list

Status: **DRAFT 2026-11-21, for your review.** Nothing built. The engine, events and store are
[server-stats.md](server-stats.md) §5; this file is *which* badges exist and what they look like.

### The rules (decided)
- **A badge is an achievement**, awarded by the Dungeon Master as a NIP-58 event. There's no
  other achievement list.
- **Completion only:** one per quest, every one of a set, a lifetime count. Nothing for
  grinding points.
- **Every badge has its own art.**
- **Only official play earns them.** Test (the production nightly) runs under the same DM key,
  so it must never sign a badge or a score. Stats only run on a server with
  `saves.validation` on, which test never has.

---

**Ids:** a quest badge is `quest-<quest id>` (the caretaker's letters share
`quest-caretakers-letter`); the rest are `every-fish`, `every-gem`, `every-ore`, `every-song`,
`every-town`, `every-wild`, `every-place`, `level-20`, `dailies-10`, `dailies-50`, `dailies-100`.

## 1. Main story — one per main quest

Main is **one chain that runs through every level** and carries the story and lore. Today it's
the caretaker's letter and the shadow arc. More main quests are coming, and each gets its badge
from the same rule.

| Badge | Earned by | Note |
|---|---|---|
| **Heir to the Vault** | any one of the six *Caretaker's Letter* quests | one badge, whichever city you started in |
| **The Rising Shadow** | *The Rising Shadow* | |
| **The Black Flame** | *The Black Flame* | |
| **The Shadow's Source** | *The Shadow's Source* | |

## 2. Side quests — one per quest

| Badge | Earned by |
|---|---|
| **Barbarian Fishing** | *Barbarian Fishing* |
| **The Rats of Goldenhaven** | *The Rats of Goldenhaven* |
| **The Missing Heirloom** | *The Missing Heirloom* |
| **The Missing Mule** | *The Missing Mule* |
| **The One That Got Away** | *The One That Got Away* |
| **The Golden Toll** | Goldenhaven's stone quest |
| **A Stone Worth Carving** | Ironpeak's stone quest |
| **The Crown's Ledger** | the Kingdom's stone quest |
| **Prove Your Mettle** | Marshlight's stone quest |
| **Keys Go Missing** | Millhaven's stone quest |
| **Root and Song** | Verdant's stone quest |
| **The Dormant Spire** | the Kingdom's waystone |
| **The Windmill Stone** | Millhaven's waystone |
| **The Elders' Song** | Verdant's waystone |
| **The Harbor Beacon** | Goldenhaven's waystone |
| **The Source of the Fade** | Marshlight's waystone |
| **Beneath the Flooded Mine** | Ironpeak's waystone |

The waystone chain moved from main to side today (your call). It runs in order, so its last
quest already means all six: no separate "every waystone" badge.

## 3. Class and race quests — one per quest

| Badge | Earned by |
|---|---|
| **The Paladin's Vow** | *The Paladin's Vow* |
| **The Rogue's Shadow Trial** | *The Rogue's Shadow Trial* |
| **Elven Heritage** | *Elven Heritage* |
| **The Axe of the Ancestors** | *The Axe of the Ancestors* |

Most classes and races have no quest yet. Each one written later gets its badge.

## 4. Collections — every one of a set

| Badge | Earned by | Set | From |
|---|---|---|---|
| **Angler of the Realm** | caught one of every fish | 24 species | store (`fish_species`) |
| **The Jeweller's Eye** | found one of every gem, pearls included | 21 types | store (`gem_types`, new) |
| **Deep Delver** | mined every ore, iron to adamantine | 5 ores | store (`ores_mined`, new) |
| **Every Song Heard** | unlocked every song | 17 tracks | the save |
| **Freedom of the Realm** | visited every town | 9 towns | the save |
| **Wayfarer** | crossed every wild region | 9 regions | the save |
| **Cartographer** | found every place: towns, wilds and every point of interest | 51 today | the save |
| **Legend of the Realm** | reached level 20 | — | the save |

A set grows with content (a new fish species, a new town), so a badge says what it covered
when it was earned, and a new member doesn't take an earned badge away.

## 5. Milestones — lifetime counts

| Badge | Earned by |
|---|---|
| **A Reliable Hand** | 10 dailies done |
| **Trusted by the Vaults** | 50 dailies done |
| **Pillar of the Realm** | 100 dailies done |

Dailies count once per npub per day, however many of your characters do them.

---

**The list: 36 badges** (4 main, 17 side, 4 class and race, 8 collections, 3 milestones).

### Candidates (not in the 36; your call)
- **Every hearth:** all six stone quests. They're independent, so unlike the waystones it's a
  real extra.
- **Bestiary:** slain every kind of monster. 75 kinds, and not all of them appear anywhere yet.
- **Level steps:** 5, 10 and 15 on the way to Legend.
- **Weeklies 5 / 25:** waits for weekly quests (none are written yet).
- **Founder:** played during the open beta. Not completion, but a nice thank-you.

## 6. The art

- **One image per badge, 1024×1024**, NIP-58's recommended size, plus two thumbnails: 256 for
  badge lists and profiles, 64 for inline icons. The DM's kind 30009 definition carries
  `["image", "<url>/<id>.png", "1024x1024"]`, `["thumb", "<url>/<id>-256.png", "256x256"]` and
  `["thumb", "<url>/<id>-64.png", "64x64"]`.
- **Made with gpt-image-2 at high quality** (your call: no cheaper model). gpt-image-2 can't draw
  on transparency, so each badge is drawn on flat magenta (spec `"key": "#ff00ff"`) and codex
  keys it out (`imagegen/keyout.go`): a flood fill from the edge, stopped by the rim's outline,
  then the magenta tint is taken off the edge pixels. The drawn original is kept beside each
  candidate as `-raw.png`.
- **Style: the DM's own.** His portrait is the reference: chunky pixel art with black outlines
  on aged parchment.
- **Each badge is a round medallion on a transparent background,** with its own emblem. The rim
  is strongly defined: a black outer outline, a bevelled highlight and shadow, an engraved
  band, and a dark inner outline. The rim tells the kind:
  - **gold:** main story;
  - **silver:** side quests;
  - **bronze:** class and race;
  - **gem-set:** collections;
  - **plain iron, notched by step:** milestones;
  - **legendary** (black obsidian, glowing gold runes, a crown of flames): the hardest few.
    Today that's **Legend of the Realm** (level 20) and **Pillar of the Realm** (100 dailies).
    Any badge can be promoted to it at approval, and later extra-hard achievements get it too.
- **You approve every badge one by one.** Candidates wait in `www/res/img/badges/_candidates/<id>/`
  (gitignored). Publish a pick with
  `go run ./cmd/codex --badge-art every-fish=v1,level-20=v2`.
- **Web images:** `--badge-art` writes `www/res/img/badges/<id>.png` (1024) and
  `<id>-256.png`, `<id>-64.png`. PNG, because the background is transparent and JPEG can't hold
  that.
- **Hosted on the main server:** the files are committed and served at
  `https://pubkey.quest/res/img/badges/`, so a badge's image lives as long as the game does.
