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
| Terrain | Wanted: cover, elevation, difficult ground, water. Brainstorm in §2. |

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
  costs 2 movement").
- **Movement preview:** cells within reach at your remaining movement light up, counting
  terrain cost. Unreachable ones stay dark.
- **Cover warning** on an attack button: "Target has half cover (+2 AC)".

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

## 7. Still to settle

1. **Terrain in the first pass:** all six cell types at once, or start with
   open / difficult / high cover / high ground and add water, low cover and hazards after?
2. **Grid sizing:** by total combatants (table above), or by party size only?
3. **Big fights:** should 7+ combatant grids fill the whole scene?
4. **Loot share:** the 1/N per-member rolls (recommended), or round-robin whole drops?
5. **Healer XP multiplier:** start at 1.0, and self-heals at half?
6. **Terrain for solo play too?** It makes ordinary fights richer, and it's independent of
   parties, so it could ship **before** P5.
