# Party combat (P5) — design

Status: **DESIGN, decisions + open brainstorm** (2026-10-11). Builds on P1 (multi-monster
combat, shipped) and the social plan's P4 parties (`docs/draft/social-plan.md`).

## 1. Decided (2026-10-11)

| Topic | Decision |
|---|---|
| Who fights | The whole party (always at one spot) drops into the leader's fight. Players spawn on the left, monsters on the right. |
| Grid | **Slightly bigger by default, and bigger with more combatants** (§3). |
| Turns | True initiative: **each player keeps their own place** in the order. |
| Turn timer | **45 s** per player turn; when it runs out you Dodge automatically. A disconnected member Dodges every turn until they're back. |
| Turn-order display | After initiative, a strip lists **every combatant in order**, marks **whose turn it is**, and shows that player's **countdown** (§6). |
| Group encounters | The budget scales with the party (5e per-member share added together). The roll becomes **more monsters**, an **elite** (+HP and +1 attack per extra player), or a **boss** (HP per player, extra boss actions = party size − 1). Authored encounters can say solo / group. |
| Monster targeting | Add **threat**: monsters favour whoever hurt them most, so tanks can hold aggro. |
| Loot | **Personal loot, without doubling the total** (§4). |
| XP | Damage XP goes to the damager mid-fight (as now); the kill bonus is shared by everyone still in the fight; **healers earn XP for HP healed, mid-fight** (§5). |
| Going down | Allies can revive (healing, a potion on an ally, Help). If the party wins, downed members wake at 1 HP with no death penalty. **Only a full wipe** sends everyone home stripped. |
| Fleeing | Fleeing alone also leaves the party. The leader can call a **retreat**: one shared roll at the slowest member's speed. |
| Ally rules | Flanking, Help, ally-targeted heals and buffs, and **AoE friendly fire with a warning** before you confirm. |
| Terrain | **All six cell types** (§2). Shown first with tints + glyphs, then a generated tileset (§2a). |
| Grid size | **By total combatants** (§3); a big fight may **fill the whole scene**. |
| Loot share | **1/N per-member rolls** (§4). |
| Healer XP | As §5. |
| Tokens | **Monster sprites on the grid** instead of emoji; players shown by Nostr picture (§2b). |
| Stealth openings | Come upon a monster that hasn't noticed you: **sneak away or sneak attack** (§7). |
| Scope | **All of it gets built.** Terrain, bigger grids, tokens and stealth openings improve solo fights too, so they ship first, before party combat. |

## 2. Terrain (brainstorm)

Each grid cell gets a terrain type. Everything in this section is data, a table in
`game-data/systems/combat-terrain.json`, so it's tunable like the gem curves.

### Cell types
| Terrain | Moving | Fighting | Looks like |
|---|---|---|---|
| **open** | 1 move | — | plain |
| **difficult** (brush, rubble, mud, shallow water) | **2 moves** per cell | — | stippled |
| **deep water** | 2 moves; needs swimming. A creature with no swim speed may only enter if it can swim (Athletics check) | melee attacks at disadvantage, unless it's an aquatic monster | blue |
| **low cover** (crate, low wall, fallen log) | can't stand on it; it sits *between* attacker and target | **half cover: +2 AC** vs attacks passing through or beside it | ▭ |
| **high cover** (wall, pillar, boulder, tree trunk) | blocks movement | **blocks line of sight**; adjacent targets behind it get **three-quarters cover: +5 AC** | ▮ |
| **high ground** (ledge, hill, stairs) | climbing up costs +1 move; stepping down is free | **+1 to hit** on ranged attacks from high to low; melee unchanged | ▲ |
| **hazard** (fire, thorns, caustic pool) — later | allowed | damage when entered and at the start of a turn there | 🔥 |

### Line of sight and cover
- A ranged attack draws a line from cell centre to cell centre (Bresenham).
  - Crossing a **high cover** cell blocks the shot.
  - Crossing or brushing a **low cover** cell gives the target +2 AC.
- Melee against an adjacent target ignores cover, except **high cover between diagonal
  neighbours**: no attacking around a pillar's corner.

### Where terrain comes from
- **Generated per fight** from the environment, with density bands:
  - **forest:** trees (high cover) and brush (difficult);
  - **swamp:** shallow and deep water, mud;
  - **cave / dungeon:** pillars, rubble, ledges;
  - **road / plains:** mostly open, a cart (low cover), the odd boulder;
  - **city streets / buildings:** crates, tables (low cover), counters, stairs.
- **Authored arenas** for bosses and POI set pieces: a fixed layout in the encounter or POI
  node, e.g. the Rat King's drainage chamber with deep water down the middle.
- **Spawn columns are kept clear** (the left two and right two columns stay open) so nobody
  starts stuck. Every open cell must be reachable from both spawn sides (a flood-fill check
  before the fight starts; re-roll if it fails).

### AI
- **Pathfinding:** the greedy stepper becomes **A\*** with terrain costs, routing around
  high cover.
- **Ranged monsters** prefer cells with cover from their target, and high ground. Melee
  monsters take the cheapest path.
- **Cautious monsters** retreat behind cover when wounded instead of running straight
  for the edge.

### Display
- Cells get a terrain tint plus a tiny glyph (🌲 🪨 💧 ▲ ▭) and a hover tooltip ("Brush —
  costs 2 movement"). This is the **first pass** (§2a).
- **Movement preview:** cells within reach at your remaining movement light up, counting
  terrain cost. Unreachable ones stay dark.
- **Cover warning** on an attack button: "Target has half cover (+2 AC)".

## 2a. How terrain looks — no tileset needed to start

Terrain is **rules first, art second**. The renderer draws each cell from its terrain
type, in whichever way is available:

1. **Pass 1, no new art:** a **tint plus a glyph**.

   | Terrain | Look |
   |---|---|
   | open | the scene shows through, dimmed (as now) |
   | difficult | a brown stipple, `·` dots |
   | shallow water | translucent blue |
   | deep water | darker blue with a `≈` glyph |
   | low cover | a ▭ crate glyph |
   | high cover | a solid dark cell with a 🪨 or 🌲 glyph, depending on the environment |
   | high ground | a lighter cell with a **shadow on its lower edge** (a 2px dark bottom border), so it reads as raised |
   | hazard | a red tint with 🔥 |

   Everything is readable on day one and costs no art.
2. **Pass 2, a small tileset per environment family:**
   - **Size:** 16×16 or 24×24, pixelated, about **10 tiles per family** (open ground, two
     difficult variants, shallow and deep water, low cover, high cover, a high-ground top
     and its edge, hazard).
   - **Families:** forest, swamp, cave/dungeon, road/plains, town interior. That's ~50 tiles.
   - **Made with Codex's built-in PixelLab** (already wired into `cmd/codex`), with the same
     review-and-pick flow as item sprites.
   - The renderer uses a tile when one exists for (environment, terrain) and falls back to
     the pass-1 look otherwise, so art can land a family at a time.

**Hard to show, so kept out of the always-on view:**
- **Line of sight:** shown only while you're aiming. When an attack or spell is picked,
  cells your target can't be seen from dim, and the target gets a 🛡 badge with its cover
  bonus.
- **Exact movement costs:** not printed on every cell. While you're moving, reachable cells
  light up (already counting terrain), and hovering one shows its cost.
- **Elevation heights:** only one level of "high ground", no multi-level heights. That's
  enough for tactics and readable top-down.

## 2b. Tokens on the grid

- **Monsters:** their **sprite** (`/res/img/monsters/{template}.png`; 114 exist for 32
  monsters) drawn pixelated and filling the cell.
  - A thin HP bar along the cell's bottom.
  - A red outline when it's the current target.
  - Conditions as tiny badges in a corner.
  - **Fallback when a sprite is missing:** `unknown.png`.
- **Players:**
  - **In a party:** each player's **Nostr profile picture** in a square frame, so you can
    tell real people apart at a glance. The frame is green for you and blue for allies, and
    the leader gets a 👑 pip.
  - **Fallback when there's no picture** (or in solo play, if preferred): a small
    **class sigil**. That's 12 icons, one per class, and the same art family can later feed
    the paperdoll work.
- **Turn order strip** uses the same faces: picture or sprite chips.

## 3. Grid size (brainstorm)

**The fit:** the scene's grid area is about **339 × 285 px**. Today's 9 × 7 grid draws
26 px cells (234 px wide). Cells can stay at 26 px up to 13 columns.

**Size by total combatants, players plus monsters:**

| Combatants | Grid | Cell | Use |
|---|---|---|---|
| ≤ 3 | **11 × 8** | 26 px | solo vs 1–2 (the new default, slightly bigger) |
| 4 – 6 | **13 × 9** | 26 px | duo/trio, or a solo player vs a pack |
| 7 – 10 | **15 × 10** | 22 px | a full party vs a group |
| 11+ / bosses | **17 × 11** or authored | 19–20 px | big fights, dungeons |

- **Starting distance** scales with the width, so the opening range by environment (close
  in dungeons, far on plains) keeps its feel.
- **Client changes:** the grid stops being hard-coded at 9 × 7. It reads `cs.grid`, and the
  cell size comes from the scene width ÷ columns.
- **Option:** for 7+ combatants, the grid could fill the whole scene, hiding the scene art
  behind a dimmed backdrop. More room, less atmosphere.

## 4. Personal loot without doubling (brainstorm)

**The goal:** a party of N earns roughly what N solo players would earn over the same kills.
Every member gets their own drops, so there's no splitting argument. But one kill must
**not** pay out N full loot tables.

**Proposal: rolls stay per kill, and each member gets a 1/N share.**
- **Gold:** the kill's gold is rolled once, then **split evenly**. Remainders go to whoever
  dealt the most damage to that monster.
- **Guaranteed drops** ("the bandit always has a dagger"): rolled once, and **awarded to one
  member**. Who gets it: the top contributor to that kill (damage + healing during the
  fight). Ties rotate.
- **Random tier rolls:** every member gets their **own roll at 1/N of the table's rolls**.
  With N = 3 and a table of 2 rolls, each member rolls with ⅔ chance of one roll.
  - Totals land at about one solo table per kill, spread across the party.
  - Each player's drop is private: they see their own loot.
- **Gems:** each member rolls at **`gem_chance ÷ N`**, using the same per-member trick.
- **Group encounters already have more monsters,** since the budget scales with the party.
  So the total loot per fight grows naturally with party size, while each kill stays honest.

**Alternatives considered:**
- **One roll, round-robin:** each kill's whole drop goes to the next member in turn. Fair
  over time but swingy, and it feels like someone "got" the good kill.
- **Need/greed rolls** (MMO-style): an extra decision per item; too fiddly for this UI.
- **Fully personal at full rate:** N× the loot. That's the doubling we're avoiding.

## 5. Healer XP (brainstorm)

- **Healing is worth what the same HP of damage would be.**
  - A heal of H HP on an ally grants `H × (the fight's average monster XP per HP) × heal
    multiplier`, starting at **1.0**, tunable.
  - Damage XP is already `monster XP ÷ monster max HP × damage`, so healing pays at the same
    rate as fighting this fight's monsters.
- **No overheal XP:** only HP actually restored counts, capped at the ally's missing HP.
- **No farming:**
  - only damage taken **in this fight** can be healed for XP;
  - self-heals count at half rate, so a fighter can't farm it with Second Wind;
  - there's no XP for healing outside combat.
- **Awarded mid-fight,** the moment the heal lands, like damage XP (`+12 XP` in the log).
- **Later:** the same idea for support, e.g. XP when a Help action or a buff turns a miss
  into a hit. Not in the first pass.

## 6. Turn order strip and timer (UI)

- **Above the grid:** one chip per combatant, in initiative order (P1's turn-order strip,
  extended).
  - The **active chip** is outlined and shows a **countdown**: `Mara ⏱ 32`.
  - Players' chips are blue and monsters' red; downed combatants are greyed out with 💀.
- **On your turn:** a banner reads *"Your turn — 45 s"*, the action buttons light up, and a
  soft tick plays in the last 10 s.
- **On someone else's turn:**
  - your action buttons are greyed with *"Waiting for Mara…"*;
  - their moves animate from the log as they happen;
  - you can still chat.
- **Timer expiry:** *"⏱ Mara hesitates and takes the Dodge action."*

## 7. Stealth openings — "it hasn't seen you yet"

Some encounters start with the monster **unaware**. That's a moment before the fight where
you choose.

**When an encounter starts, roll who noticed whom:**
- the party's stealth against the monsters' **passive perception** (already in the monster
  data);
- the party's stealth is a group check (5e: the party succeeds if at least half its members
  succeed), each member rolling DEX + Stealth;
- terrain, light and the environment can adjust it later (fog, night, thick forest).

| Result | What happens |
|---|---|
| **You notice them, they don't notice you** | A pre-fight choice appears: *"A goblin hasn't noticed you."* |
| **Both notice** | A normal fight. |
| **They notice you first** | An ambush: the party is surprised. This already exists as `Surprise` on POI nodes and encounters. |

**The choices when you're unseen:**
- **Sneak away:** another stealth check. On success there's no fight. On failure the
  monster spots you and the fight starts normally.
- **Sneak attack:** combat starts with **the monsters surprised**. They lose their first
  turn, and the opening attacks get advantage. Rogues' Sneak Attack is a natural fit here.
- **Approach openly:** a normal fight. This is for when you want to talk to an intelligent
  creature; leaves room for parley later.

**Engine:**
- `EncounterSpec.Surprise` becomes a side, `none | party | monsters`.
- An "unaware" state comes before `StartEncounter` (the choice overlay, the same pattern as
  the encounter speech box).
- Random travel encounters get this roll automatically. Authored ones can force a result
  ("the brawler always notices you").

**Note:** the roadmap had *stealth/surprise* in "not in alpha". This pulls it in.

## 8. Still to settle

1. **Player tokens in solo play:** your Nostr picture, or the class sigil?
2. **Who makes the tileset:** PixelLab via Codex, or hand-drawn/commissioned? Pass 1 doesn't
   need either.
3. **Sneak away on failure:** the monster gets the first move, or just a normal fight?
4. **Build order** (proposed):
   1. bigger grid + monster sprites + player tokens;
   2. terrain rules with the pass-1 look;
   3. A* and cover-aware AI;
   4. stealth openings;
   5. — all of that improves solo fights —
   6. then party combat (shared fight, turn timer, loot share, healer XP, ally rules,
      group encounters);
   7. the tileset whenever the art lands.
