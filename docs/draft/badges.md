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
`every-town`, `every-wild`, `every-place`, `level-5` … `level-20`, `dailies-10`, `dailies-50`,
`dailies-100`.

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

A set grows with content (a new fish species, a new town), so a badge says what it covered
when it was earned, and a new member doesn't take an earned badge away.

## 5. Milestones — lifetime counts

| Badge | Earned by |
|---|---|
| **A Reliable Hand** | 10 dailies done |
| **Trusted by the Vaults** | 50 dailies done |
| **Pillar of the Realm** | 100 dailies done |

Dailies count once per npub per day, however many of your characters do them.

## 6. Levels — a rank that grows

One badge at **levels 5, 10, 15, 16, 17, 18, 19 and 20**, each grander than the last (your call,
2026-11-21). They're read from the save: a character who reaches the level earns it, and one
who's already past it gets every step they've passed at their next save.

**The number is on the badge, big:** a bold numeral on a ribbon across the lower third,
readable even as a 64-pixel thumb. It's the only text on any badge.

**A player shows their highest.** Every level badge earned stays awarded, but the game's badge
screen and the player's profile badges (kind 30008, which the game fills in) show only the
highest, like a rank.

| Badge | Level | Look |
|---|---|---|
| **Adventurer's Mark** | 5 | plain iron rim; a short sword and walking staff |
| **Sworn Blade** | 10 | polished steel; longsword and round shield |
| **Knight of the Realm** | 15 | gold with four rubies; sword, kite shield, laurel |
| **Bannerlord** | 16 | gold ringed with rubies and a star; sword and war banner |
| **Champion** | 17 | platinum with sapphires and small wings; enchanted sword |
| **Paragon** | 18 | glowing mithral, diamonds, larger wings and a crown; runed sword, griffin shield |
| **Dragonsworn** | 19 | glowing adamantine, dragon-scale wings, spiked crown; a dragon around a blade |
| **Legend of the Realm** | 20 | super epic: obsidian with molten-gold runes, a crown of flames, golden dragon wings and a jewelled crown; a blazing hero's sword |

---

**The list: 43 badges** (4 main, 17 side, 4 class and race, 7 collections, 3 milestones,
8 levels).

### Candidates (not in the 43; your call)
- **Every hearth:** all six stone quests. They're independent, so unlike the waystones it's a
  real extra.
- **Bestiary:** slain every kind of monster. 75 kinds, and not all of them appear anywhere yet.
- **Weeklies 5 / 25:** waits for weekly quests (none are written yet).
- **Founder:** played during the open beta. Not completion, but a nice thank-you.

## 7. The art

- **One image per badge, 1024×1024**, NIP-58's recommended size, plus two thumbnails: 256 for
  badge lists and profiles, 64 for inline icons. The DM's kind 30009 definition carries
  `["image", <image>, "1024x1024"]`, `["thumb", <thumb_256>, "256x256"]` and
  `["thumb", <thumb_64>, "64x64"]`, the URLs from `badge-media.json`.
- **Made with gpt-image-2.6 Sunburst at high quality,** OpenAI's newest quality model (your
  call: no cheaper model; it beat gpt-image-2 in a side-by-side trial). It draws the
  transparent background itself (spec `"background": "transparent"`). For a model that can't,
  codex can key out a flat colour instead (spec `"key": "#ff00ff"`, `imagegen/keyout.go`).
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
    Today that's **Pillar of the Realm** (100 dailies). The level badges have their own rising
    rims, topped by Legend of the Realm.
    Any badge can be promoted to it at approval, and later extra-hard achievements get it too.
- **You approve every badge one by one.** Candidates wait in `www/res/img/badges/_candidates/<id>/`
  (gitignored). Publish a pick with
  `go run ./cmd/codex --badge-art every-fish=v1,level-20=v2`.
- **Web images:** `--badge-art` writes `www/res/img/badges/<id>.png` (1024) and
  `<id>-256.png`, `<id>-64.png`. PNG, because the background is transparent and JPEG can't hold
  that, cut to 256 colours (`imagegen/quantize.go`, a median cut that splits where the colour
  error is largest). That takes the 1024 from about 1 MB to about 320 KB; the thumbs are about
  35 KB and 4 KB. Side by side with the original there's no visible difference.
- **All 43 picked and published (2026-11-21):** v1 of each, except The Rising Shadow, The
  Shadow's Source and Barbarian Fishing (v2). The level badges are your picks of the art (5 and
  10 v2, 15 and 20 v1) with the number edited in, and 16–19 drawn to match, all v1. The 129
  files come to about 16 MB.
- **Hosted on Blossom, as the DM's media (your call, 2026-11-21):** all 129 files are uploaded
  to `blossom.oslim.dev`, signed by the DM, and `game-data/systems/badge-media.json` maps each
  badge to its image and thumbs. His kind 30009 definitions link those URLs. The same files stay
  in `www/res/img/badges/` for the game to draw. A Blossom URL is the file's sha256, so redrawn
  art gets a new URL: upload it and update the map.
