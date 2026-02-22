# Combat & Encounters System — Master Planning Document

**Status**: Phase 1 ✅ Complete — Phase 2 ✅ Complete — Phase 9 (Combat UI) next
**Created**: 2026-02-20
**Updated**: 2026-02-22
**Priority**: Major System
**Related**: environment-poi-system.md, draft_enviornment.txt

---

## Table of Contents

| # | Section | Status |
|---|---------|--------|
| 1 | [Overview & Design Philosophy](#1-overview--design-philosophy) | ✅ Designed |
| 2 | [Range System (0–6 Scale)](#2-range-system-06-scale) | ✅ Implemented |
| 3 | [Encounter Triggering](#3-encounter-triggering) | ❌ Not started |
| 4 | [Monster Data Schema Expansion](#4-monster-data-schema-expansion) | 🚧 Partial (priority monsters done, rest stubbed) |
| 5 | [Combat State (Session Memory)](#5-combat-state-session-memory-only--not-saved-to-file) | ✅ Implemented |
| 6 | [Initiative](#6-initiative) | ✅ Implemented |
| 7 | [Turn Structure](#7-turn-structure) | 🚧 Partial (attack + bonus action; Dash/Dodge/Hide/Flee not wired) |
| 8 | [Attack Resolution](#8-attack-resolution) | ✅ Implemented |
| 9 | [Damage Resolution](#9-damage-resolution) | ✅ Implemented |
| 10 | [Weapon Properties](#10-weapon-properties--full-implementation-spec) | ✅ Implemented |
| 11 | [Armor & AC Calculation](#11-armor--ac-calculation) | 🚧 Partial (base AC done; per-piece additive / set bonus not fully wired) |
| 12 | [Class Combat Features & Abilities](#12-class-combat-features--abilities) | ❌ Not started |
| 13 | [Magic in Combat](#13-magic-in-combat) | ❌ Not started — **next priority** |
| 14 | [Consumables in Combat](#14-consumables-in-combat) | ❌ Not started |
| 15 | [Conditions](#15-conditions--full-implementation) | ❌ Not started |
| 16 | [Death System](#16-death-system) | ✅ Implemented |
| 17 | [Monster AI](#17-monster-ai) | ✅ Implemented (basic — flee, preferred range, action selection) |
| 18 | [Flee Mechanic](#18-flee-mechanic) | ❌ Not started |
| 19 | [Combat UI Design](#19-combat-ui-design) | ❌ Not started — **next after magic** |
| 20 | [XP & Loot](#20-xp--loot) | ✅ Implemented |
| 21 | [Environment → Encounter Schema](#21-environment--encounter-schema) | ❌ Not started |
| 22 | [Monster Difficulty Scaling](#22-monster-difficulty-scaling) | ❌ Not started |
| 23 | [Saving Throws in Combat](#23-saving-throws-in-combat) | ❌ Not started |
| 24 | [Short Rest & Long Rest](#24-short-rest--long-rest-combat-relevance) | ❌ Not started |
| 25 | [Implementation Phases](#25-implementation-phases) | 🚧 Phase 1 ✅, Phase 2 ✅, rest pending |
| 26 | [Open Questions](#26-open-questions) | 🚧 Some resolved |
| 27 | [Priority Monster List](#27-priority-monster-list-for-phase-1-data-entry) | 🚧 Partial (data entry ongoing) |
| 28 | [Technical Architecture Notes](#28-technical-architecture-notes) | ✅ Reference only |
| 29 | [Stealth & Surprise](#29-stealth--surprise) | ❌ Not started |
| 30 | [Party & Companion Architecture](#30-party--companion-architecture-note) | ❌ Not started (low priority) |

---

## 1. Overview & Design Philosophy

Combat is the core gameplay loop of Pubkey Quest. It is **D&D 5e-inspired** but adapted for a single-player, web-based, turn-based RPG. Key adaptations:

- **Turn-based**: Player takes their turn, then all monsters take theirs (no simultaneous resolution)
- **Text-driven**: Narrated actions with a clean combat log, not a graphical battle screen
- **Range abstracted**: 0–6 scale instead of feet (see Range System below)
- **Single character**: No party — player is alone (can have hireling/summon companions later)
- **Time pauses**: Combat completely stops in-game time advancement
- **Backend authoritative**: All combat math done in Go — frontend just renders results

### What We Keep from D&D 5e

- d20 attack rolls vs AC
- STR/DEX/INT/WIS modifiers on attacks and saves
- Proficiency bonus scaling with level
- Action economy (Action, Bonus Action, Reaction, Movement)
- All weapon property mechanics
- Spell slot consumption and saving throws
- All 15 conditions (Blinded, Poisoned, etc.)
- Death saving throws at 0 HP
- Critical hits (double damage dice)

### What We Simplify or Adapt

- No Flanking (optional rule, skip for now)
- No grid/hex — just range 0–6
- Monster AI is simple (see Monster AI section)
- Opportunity attacks simplified (see Reactions)
- No grapple contests initially (can add later)

---

## 2. Range System (0–6 Scale)

**All distances in combat are measured on a 0–6 integer scale.**

| Range Value | D&D Equivalent | Description                                  |
| ----------- | -------------- | -------------------------------------------- |
| 0           | 0–5 ft         | In contact — melee, grapple                  |
| 1           | 5–10 ft        | Adjacent — melee reach, close throw          |
| 2           | 10–30 ft       | Short range — handaxe, javelin, dagger throw |
| 3           | 30–60 ft       | Medium range — shortbow, hand crossbow       |
| 4           | 60–120 ft      | Long range — longbow normal, spells          |
| 5           | 120–150 ft     | Very long — longbow long range               |
| 6           | 150+ ft        | Extreme — max range, always disadvantage     |

### Range in Combat

- **Melee weapons**: Require range 0. Weapons with **reach** tag can attack at range 0 or 1.
- **Thrown weapons**: Use item's `range` (normal) and `range-long` (long range, disadvantage)
- **Ranged weapons**: Use item's `range` and `range-long`
- **Spells**: Use spell's `range` and `range_long`

### Starting Range

When an encounter begins, starting range is determined by:

- Environment type (forest → 1-2, open plains → 3-4, dungeon corridor → 0-1)
- Monster size (Large+ start closer, Tiny start farther)
- Player vs Monster initiative (winner may choose to adjust +/-1 before first round)
- **Default**: Range 2 for most encounters

### Movement in Combat

Each turn, the player and monsters can **move** (change range):

- **Move closer**: Decrease range by 1
- **Move away**: Increase range by 1
- Movement does not cost an action (it's free as part of the turn)
- **Dash action**: Change range by an additional 1 step (total 2); if player Athletics ≥ 14, Dash adds 2 extra steps (total 3)
- Movement triggers **Opportunity Attacks** if moving away from melee range (see Reactions)

### Athletics & Movement

Player movement uses the **Athletics** score (already implemented):

```
Athletics = (STR × 0.5) + (CON × 0.35) + (DEX × 0.15)
```

**Simplified movement model**: Free movement is always 1 step. Athletics affects Dash range and flee mechanics (Section 18), not base movement speed.

| Athletics Score | Dash Adds | Total Range Change |
| --------------- | --------- | ------------------ |
| ≤ 13            | +1 step   | 2 steps            |
| 14+             | +2 steps  | 3 steps            |

### Monster Speed → Athletics Score

Monster speed is derived from its stats using the same formula as the player, so the comparison in flee calculations is apples-to-apples:

```
monster_athletics = (STR × 0.5) + (CON × 0.35) + (DEX × 0.15)
```

`speed.walk` in the monster JSON is kept for reference/display but is not used in flee math. The athletics score is computed at runtime from the monster's stat block.

---

## 3. Encounter Triggering

### Environment Encounters (Random)

Encounters happen during environment traversal based on in-game minutes elapsed. The save file already stores `TimeOfDay` in minutes (0–1439); all schedule windows use this directly.

**Schedule format** (minute-based, replaces hour strings):

```json
"encounter_settings": {
  "encounter_check_interval_minutes": 30,
  "base_encounter_chance_per_interval": 0.25,
  "stealth_modifier": 2,
  "encounter_schedule": [
    { "name": "dawn",  "start": 300,  "end": 419,  "rate_mult": 0.8,  "diff_mult": 0.9  },
    { "name": "day",   "start": 420,  "end": 1019, "rate_mult": 1.0,  "diff_mult": 1.0  },
    { "name": "dusk",  "start": 1020, "end": 1199, "rate_mult": 1.3,  "diff_mult": 1.2  },
    { "name": "night", "start": 1200, "end": 299,  "rate_mult": 1.8,  "diff_mult": 1.5  }
  ]
}
```

**Night wraps midnight**: `is_night = (time >= 1200 || time < 300)`

### Encounter Check Trigger

Rather than a static interval with modified odds, the trigger is **dynamic** — the `rate_mult` and environment base rate together determine how often a check fires, not just the probability when it does.

```
effective_interval = base_interval_minutes / (base_encounter_chance × schedule.rate_mult)
// Example: base 30 min, day rate 1.0, chance 0.25 → check every 120 min
// Example: base 30 min, night rate 1.8, chance 0.25 → check every ~67 min
```

Checks fire more frequently at night and in dangerous environments — the world feels more alive rather than just rolling harder dice at the same clock ticks.

**Each check has a fixed roll-to-trigger**:

```
roll d100 — if roll ≤ (base_encounter_chance × 100): encounter triggers
```

The rate multiplier is absorbed into how often the check runs, not into the per-check probability. Weather and difficulty can still modify the per-check probability on top of that:

```
per_check_chance = base_encounter_chance × weather.encounter_modifier × difficulty_modifier
```

### Pre-Encounter Stealth Phase (Automatic)

When an encounter check would trigger, the game silently runs a **stealth check before locking in the encounter**. No UI prompt — the player has no knowledge of the roll either way.

**Stealth Roll**:

```
roll = d20 + DEX_modifier + stealth_proficiency (if proficient)
roll -= fatigue_penalty (see Section 29)
roll -= armor_penalty (heavy armor = -5, medium armor = -2)
roll += environment.stealth_modifier (from encounter_settings JSON)
```

- **Roll > Monster Passive Perception**: Player sneaks past. Encounter does NOT trigger. No XP.
- **Roll ≤ Monster Passive Perception**: Encounter triggers. See Section 29 for surprise determination.

**Monster Passive Perception**: `10 + WIS_modifier` (stored in monster JSON `senses.passive_perception`)

### Dungeon Encounters (Static)

Defined in dungeon JSON — specific monster(s) at specific steps. See environment-poi-system.md.

### Dungeon Encounter Types (future expansion)

These apply to static dungeon encounters only, not random environment encounters:

- **Ambush**: Monsters surprise player (advantage on first round attacks)
- **Patrol**: Player can choose to avoid (Stealth check)
- **Boss**: Pre-defined, guaranteed at certain dungeon steps

---

## 4. Monster Data Schema Expansion

**Current monster JSON** is missing nearly all combat data. Every monster JSON needs these fields added:

```json
{
  "id": 17,
  "name": "Goblin",
  "challenge_rating": 0.25,
  "xp": 50,
  "type": "Humanoid",
  "size": "Small",
  "armor_class": 15,
  "hit_points": 7,
  "hp_dice": "2d6",
  "alignment": "neutral evil",
  "tags": ["Goblinoid"],
  "img": "/static/img/monster/goblin.svg",
  "environment": ["forest", "grassland", "mountain", "urban"],

  "speed": {
    "walk": 30,
    "fly": 0,
    "swim": 0,
    "climb": 0
  },

  "stats": {
    "strength": 8,
    "dexterity": 14,
    "constitution": 10,
    "intelligence": 10,
    "wisdom": 8,
    "charisma": 8
  },

  "saving_throws": {
    "dexterity": 4
  },

  "skills": {
    "stealth": 6
  },

  "damage_resistances": [],
  "damage_immunities": [],
  "damage_vulnerabilities": [],
  "condition_immunities": [],

  "senses": {
    "darkvision": 60,
    "passive_perception": 9
  },

  "preferred_range": 1,

  "actions": [
    {
      "name": "Scimitar",
      "type": "melee_attack",
      "attack_bonus": 4,
      "reach": 0,
      "range": null,
      "hit": {
        "dice": "1d6",
        "mod": 2,
        "type": "slashing"
      }
    },
    {
      "name": "Shortbow",
      "type": "ranged_attack",
      "attack_bonus": 4,
      "reach": null,
      "range": 3,
      "range_long": 5,
      "hit": {
        "dice": "1d6",
        "mod": 2,
        "type": "piercing"
      }
    }
  ],

  "special_abilities": [
    {
      "name": "Nimble Escape",
      "description": "Goblin can take Disengage or Hide as bonus action each turn.",
      "type": "passive"
    }
  ],

  "bonus_actions": [],
  "reactions": [],
  "legendary_actions": [],

  "loot_table": [
    { "item": "shortsword", "chance": 0.3, "quantity": [1, 1] },
    { "item": "gold-piece", "chance": 0.8, "quantity": [1, 8] }
  ],

  "behavior": {
    "aggression": "aggressive",
    "flee_threshold": 0.25,
    "preferred_range": 1,
    "target_priority": "lowest_hp",
    "relentless": false
  }
}
```

### Fields to Add to ALL Monster JSONs

This is a large data task. Priority order:

1. `xp` — needed for rewards immediately
2. `stats` (all 6) — needed for attack modifiers
3. `actions` — what the monster can do each turn
4. `loot_table` — what drops on death
5. `behavior` — AI decision making
6. `damage_resistances/immunities` — combat math
7. `special_abilities` — unique monster traits
8. `condition_immunities` — which conditions affect them
9. `senses` — darkvision, perception for ambush/detection
10. `saving_throws` — for spell targeting

**Strategy**: Start with the ~30 most commonly encountered monsters, add full data. Others can be stubs.

---

## 5. Combat State (Session Memory Only — NOT Saved to File)

Combat state lives **entirely in Go server-side session memory**. It is never written to the save file.

### Why

- Save files are kept minimal (Nostr-friendly)
- Encounters are random — if a player disconnects mid-fight and reconnects, losing that combat instance is acceptable. They won't be re-rolling the same encounter.
- This intentionally accepts the soft exploit where a player facing death can disconnect and reconnect to avoid it. The cost (random encounter lost) is low enough that persisting combat to prevent it isn't worth the save file bloat.

### Saving is Disabled During Combat

The `POST /api/saves/{npub}` endpoint **rejects save requests** while the session has an active combat instance. Auto-save is also suppressed. The last save reflects the player's state immediately before the encounter began.

### Session Memory Structure

Held in Go server memory keyed by npub, discarded on disconnect or combat end:

```go
type CombatSession struct {
    Party   []PartyCombatant
    Monsters []MonsterInstance
    Initiative []InitiativeEntry
    CurrentTurnIndex int
    Round  int
    Range  int
    Log    []string
    EnvironmentContext string
}

type PartyCombatant struct {
    Type              string  // "player", future: "companion"
    ID                string  // npub
    IsPlayerControlled bool
    CharacterSnapshot  CharacterSnapshot  // stats snapshotted at combat start
    CombatState        PlayerCombatState
}
```

**The `party` array is future-proof** — Phase 1 always has exactly 1 entry. Companions and multiplayer slot into the same structure without redesign (see Section 30).

### Session Memory is the Live Game State

The save file on disk is only written when the player **manually saves**. In between manual saves, all game state — XP, HP, inventory (including gold-piece), location, active effects — lives in Go server-side session memory. The save file is a snapshot of that memory at the moment the player chose to save.

This means:

- XP earned per hit during combat → updates **session memory**, not disk
- HP lost during combat → updates **session memory**, not disk
- Loot received after combat → updates **session memory**, not disk
- The player must manually save after combat if they want to persist those gains

On **combat victory**: session memory is updated with final HP, mana, XP, and loot. The player is returned to the world and can save whenever they choose.
On **disconnect during combat**: session memory is lost. Player reconnects at their last manual save, before the encounter.

**No forced disk writes exist** — in the full game, saves are signed Nostr events published to relays. The player must sign with their private key; the server cannot force a save on their behalf. The "disk save" in the test server and alpha is a local convenience only. Death strips session memory to the post-death state, but the player still chooses when to publish/save that state.

### Save File Has No Combat Fields

The save file schema is unchanged by combat. It reflects the game state as of the last manual save.

---

## 6. Initiative

**Formula**: `d20 + DEX_modifier`

- Player rolls on combat start
- Each monster has a fixed initiative bonus = their DEX modifier (can add proficiency for legendary monsters)
- Ties broken by: higher DEX score → higher initiative bonus → player wins tie
- Initiative order is fixed for the entire combat (not re-rolled each round)
- **Advantage on initiative**: Ranger Hunter's Mark, Rogue (Alacrity Rogue feature later), some environments

---

## 7. Turn Structure

Each combatant's turn consists of:

```
[ Movement ] + [ Action ] + [ Bonus Action? ] + [ Reaction* ]
```

\*Reactions happen on others' turns, not your own.

### Actions (Player Can Choose)

| Action         | Description                                                  |
| -------------- | ------------------------------------------------------------ |
| **Attack**     | Make one or more weapon attacks                              |
| **Cast Spell** | Cast a spell (unless bonus action spell)                     |
| **Dash**       | Move an additional range step                                |
| **Disengage**  | Move away without triggering opportunity attacks             |
| **Dodge**      | Attackers have disadvantage, you have advantage on DEX saves |
| **Help**       | (Future: party) Give advantage to an ally                    |
| **Hide**       | Attempt to become hidden (Stealth vs passive Perception)     |
| **Use Item**   | Use a consumable (potion, alchemist's fire, etc.)            |
| **Ready**      | Prepare a reaction for a trigger (advanced, implement later) |
| **Grapple**    | Athletics contest to grab monster (STR vs STR/DEX)           |
| **Shove**      | Athletics contest to push monster (changes range)            |

### Bonus Actions (When Available)

| Bonus Action            | Who Gets It                                         |
| ----------------------- | --------------------------------------------------- |
| Off-hand attack         | Two-weapon fighting (light weapon in off-hand)      |
| Cunning Action          | Rogues: Dash, Disengage, or Hide                    |
| Second Wind             | Fighters: heal 1d10 + Fighter level (once per rest) |
| Nimble Escape           | Goblins specifically                                |
| Cast bonus-action spell | Some spells (Healing Word, Hunter's Mark, etc.)     |
| Rage                    | Barbarian activates/maintains rage                  |
| Ki: Patient Defense     | Monk: Dodge as bonus action                         |
| Ki: Step of the Wind    | Monk: Disengage/Dash as bonus action                |

### Reactions (Triggered, Not Chosen on Your Turn)

| Reaction               | Trigger                             |
| ---------------------- | ----------------------------------- |
| **Opportunity Attack** | Enemy moves away from melee range   |
| **Shield spell**       | When hit by an attack               |
| **Absorb Elements**    | When hit by elemental damage        |
| **Uncanny Dodge**      | Rogue: halve damage from one attack |

---

## 8. Attack Resolution

### Step 1: Choose Attack

Player selects:

- Which weapon (equipped mainHand, offHand, or thrown)
- Or which spell
- Target (if multiple monsters)

### Step 2: Check Range

- Melee weapon: Must be at range 0 (reach = range 0 or 1)
- Ranged weapon: Must be within `range-long`
- Attacking at long range (> `range`, ≤ `range-long`): **Disadvantage**
- If monster is at range 0 and player fires a ranged weapon: **Disadvantage**

### Step 3: Roll Attack

```
Attack Roll = d20 + ability_modifier + proficiency_bonus
```

**Ability modifier for attacks**:

- Melee weapon → STR modifier (or DEX if Finesse and DEX is higher)
- Ranged weapon → DEX modifier
- Thrown weapon → STR modifier (or DEX if Finesse)
- Spell attack → spellcasting modifier (INT for Wizard/Eldritch Knight, WIS for Cleric/Druid, CHA for Paladin/Sorcerer/Warlock/Bard)

**Proficiency bonus by level**:
| Levels | Proficiency |
|--------|-------------|
| 1–4 | +2 |
| 5–8 | +3 |
| 9–12 | +4 |
| 13–16 | +5 |
| 17–20 | +6 |

Players are proficient with all weapons of their class:

- **Barbarian**: Simple, Martial
- **Fighter**: Simple, Martial
- **Paladin**: Simple, Martial
- **Ranger**: Simple, Martial
- **Rogue**: Simple, hand crossbow, longsword, rapier, shortsword
- **Bard**: Simple, hand crossbow, longsword, rapier, shortsword
- **Cleric**: Simple + deity-granted (typically no martial unless war domain)
- **Druid**: Simple + non-metal weapons
- **Monk**: Simple, shortsword
- **Sorcerer/Wizard/Warlock**: Daggers, darts, slings, quarterstaffs, light crossbows

If **not proficient**: No proficiency bonus added to attack roll.

### Step 4: Compare to AC

- Attack roll ≥ Monster AC → **Hit**
- Natural 20 (d20 shows 20) → **Critical Hit** (double damage dice, always hits)
- Natural 1 (d20 shows 1) → **Critical Miss** (always misses, can fumble)
- Critical Miss: No additional effect for now (possible fumble table later)

### Step 5: Advantage / Disadvantage

Roll two d20s, take **higher** (advantage) or **lower** (disadvantage).
Cannot have double-advantage; they cancel out (one advantage + one disadvantage = normal).

**Advantage on attacks** when:

- Target is Prone and attacker is adjacent (range 0)
- Target is Blinded, Paralyzed, Petrified, Stunned, or Unconscious
- Attacker is Invisible
- Barbarian uses Reckless Attack
- Some spells grant it

**Disadvantage on attacks** when:

- Attacker is Blinded, Frightened, Poisoned, Restrained, or Exhausted 3+
- Ranged attack while at range 0 (in melee)
- Long range (beyond normal range)
- Heavy weapon used by Small creature

---

## 9. Damage Resolution

### Step 1: Roll Damage

```
Damage = weapon_dice + ability_modifier
```

Same ability modifier as the attack roll (STR for melee, DEX for ranged, etc.).

**Critical Hit**: Roll ALL damage dice **twice** (not just double the total). Modifiers added once.
Example: Longsword crit → 2d8+STR (one-handed) or 2d10+STR (two-handed versatile).

### Step 2: Apply Resistances / Immunities / Vulnerabilities

- **Resistance**: Halve damage (round down) — e.g., Skeleton resists piercing from arrows
- **Immunity**: Take 0 damage — e.g., Fire elemental immune to fire
- **Vulnerability**: Double damage — e.g., Skeleton vulnerable to bludgeoning

### Step 3: Reduce HP

```
new_hp = current_hp - final_damage
```

If HP reaches 0: monster is dead (most) or player goes to Death Saving Throws.

### Damage Types (All 13)

| Type        | Common Sources                           |
| ----------- | ---------------------------------------- |
| Acid        | Acid splash, black dragon                |
| Bludgeoning | Clubs, maces, quarterstaffs              |
| Cold        | Ray of Frost, ice storms                 |
| Fire        | Fire Bolt, Fireball, Alchemist's Fire    |
| Force       | Magic Missile (ignores most resistances) |
| Lightning   | Lightning Bolt, shock                    |
| Necrotic    | Inflict Wounds, undead attacks           |
| Piercing    | Arrows, daggers, spears                  |
| Poison      | Poison effects, snakes                   |
| Psychic     | Mind Blast, Dissonant Whispers           |
| Radiant     | Sacred Flame, Holy effects               |
| Slashing    | Swords, axes, claws                      |
| Thunder     | Thunderwave, Shatter                     |

---

## 10. Weapon Properties — Full Implementation Spec

### `finesse`

**Rule**: May use STR or DEX modifier for attack AND damage rolls. Player uses whichever is higher.
**Affected Items**: Dagger, Rapier, Shortsword, Scimitar
**Implementation**: When building attack, compare player STR and DEX modifiers, use higher.

### `light`

**Rule**: Can be used as off-hand weapon in two-weapon fighting.
**Affected Items**: Dagger, Handaxe, Shortsword
**Implementation**: If mainHand is light AND offHand is light, enable off-hand bonus action attack. Off-hand attack: NO ability modifier added to damage (unless Dual Wielder feat, future).

### `thrown`

**Rule**: Can be thrown as a ranged attack. Uses STR modifier (or DEX if also Finesse). Uses `range` and `range-long` values.
**Affected Items**: Dagger, Handaxe, Javelin, Spear, Trident, Net
**Implementation**: Ranged attack option using existing range values. Throws consume one item from stack if quantity > 1.

### `versatile`

**Rule**: Can be wielded one-handed (smaller die) OR two-handed (larger die). The two damage values in `damage` field are `"1d8,1d10"` = 1d8 one-hand, 1d10 two-hand.
**Affected Items**: Longsword, Spear, Quarterstaff, Warhammer, Trident
**Implementation**: Player chooses grip style. Two-handed requires offHand to be empty.
**Damage field format**: Already `"1d8,1d10"` — first = 1H, second = 2H.

### `two-handed`

**Rule**: Requires both hands. Cannot use off-hand. Cannot have shield equipped.
**Affected Items**: Greatsword, Greataxe, Maul, Longbow, Heavy Crossbow, Glaive
**Implementation**: Block equipping offHand if two-handed weapon in mainHand. Block equipping two-handed in mainHand if offHand has item.

### `heavy`

**Rule**: Small and Tiny creatures have **disadvantage** on attack rolls with heavy weapons.
**Affected Items**: Greatsword, Greataxe, Maul, Longbow, Heavy Crossbow, Glaive, Halberd
**Races affected**: Halfling, Gnome (Small size) → add to character race data
**Implementation**: Check if player race is Small + weapon is heavy → disadvantage flag.

### `ammunition`

**Rule**: Uses ammunition from inventory. Expend one piece per attack. If no ammo → cannot attack with weapon.
**Affected Items**: Longbow (arrows), Shortbow (arrows), Hand Crossbow (bolts), Light Crossbow (bolts), Heavy Crossbow (bolts)
**Implementation**: Check `ammunition` field on weapon → find matching ammo item in general_slots → deduct 1 per attack.
**Ammunition items**: `arrows`, `crossbow-bolts` (already in item list)

### `loading`

**Rule**: Can only make ONE attack with this weapon per turn, even with Extra Attack.
**Affected Items**: Hand Crossbow, Light Crossbow, Heavy Crossbow
**Implementation**: If weapon has loading tag AND player has Extra Attack → only allow 1 attack. (Crossbow Expert feat removes this — future).

### `reach`

**Rule**: Extends melee range by 5ft (+1 on 0-6 scale). Can attack at range 0 OR 1 without disadvantage.
**Affected Items**: Glaive, Halberd, Lance, Pike, Whip, Quarterstaff (some builds)
**Implementation**: Melee weapons with reach can attack at range 0 or 1.

### `special`

**Rule**: Has special rules defined in its own entry.
**Affected Items**: Lance (disadvantage on adjacent targets unless mounted), Net (can entangle, no damage)
**Implementation**: Per-item special logic.

---

## 11. Armor & AC Calculation

### How AC is Calculated

AC is built up additively from every equipped piece of gear that has an `ac` or `ac_formula` field. Each slot is evaluated independently and summed.

```
total_ac = base_ac + sum(piece.ac for each equipped piece) + set_bonuses + class_unarmored_bonus
```

**Base AC** (when nothing is equipped that sets a base):

- Default: `10 + DEX mod`
- Barbarian unarmored: `10 + DEX mod + CON mod`
- Monk unarmored: `10 + DEX mod + WIS mod`

If a piece sets `"ac_base": true`, it replaces the default base rather than adding to it. Only one base-setting piece applies (the highest, or the `armor` slot takes precedence).

### Per-Piece AC on Item JSONs

Every armor piece in the `gear_slots` (helmet, armor, boots, gloves, cloak, ring1, ring2, necklace, offHand for shield) can carry its own AC contribution:

```json
{
  "id": "chainmail-cuirass",
  "armor_type": "heavy",
  "ac_base": true,
  "ac": 16,
  "dex_cap": null,
  "str_requirement": 13
}
```

```json
{
  "id": "iron-helmet",
  "armor_type": "heavy",
  "ac": 1
}
```

```json
{
  "id": "shield",
  "armor_type": "shield",
  "ac": 2
}
```

**`ac_base: true`** — this piece sets the base AC formula (replaces the 10+DEX default). Used for chest pieces.
**`ac`** — flat AC added on top of the current base. Used for helmets, boots, gloves, shields, rings, cloaks.
**`dex_cap`** — if set, DEX mod contribution to base is capped at this value (`null` = uncapped for light; `2` for medium; `0` or omitted for heavy).

### AC Formula by Armor Category

| Category  | Base AC Formula             | DEX Cap |
| --------- | --------------------------- | ------- |
| Unarmored | 10 + DEX mod                | none    |
| Light     | piece.ac + DEX mod          | none    |
| Medium    | piece.ac + min(DEX mod, 2)  | 2       |
| Heavy     | piece.ac (DEX ignored)      | 0       |
| Shield    | +piece.ac (always additive) | —       |

The `armor_type` on the chest piece (slot: `armor`) determines which category applies for the DEX cap. Non-chest pieces (helmet, boots, etc.) add their flat `ac` value regardless of category — they never interact with the DEX cap.

### Set Bonuses

Armor sets are packages that bundle related pieces together for purchase/display. Wearing the complete set grants an additional `set_bonus.ac` on top of each piece's individual contribution.

```json
"set_bonus": {
  "required_pieces": ["chainmail-cuirass", "chainmail-helmet", "chainmail-boots"],
  "ac": 1
}
```

Wearing a partial set: each piece still contributes its individual `ac`. The set bonus only applies when **all** `required_pieces` are equipped simultaneously.

### AC Calculation — Full Example

Player wearing: Iron Helmet (+1), Chainmail Cuirass (base 16, heavy), Leather Boots (+0), Shield (+2). DEX mod = +2 but armor is heavy so DEX ignored.

```
base_ac       = 16  (chainmail cuirass, ac_base = true, heavy → DEX ignored)
+ helmet      = +1
+ shield      = +2
+ set bonus   = 0   (partial set — no bonus)
─────────────────
total_ac      = 19
```

### STR Requirement

If the equipped chest piece has `str_requirement` and the player's STR is below it, apply a movement penalty (disadvantage on Athletics checks, slower travel). **Does not block equipping** — just a mechanical penalty.

### Adding These Fields to Item JSONs

Each armor item needs:

- `armor_type`: `"light"` | `"medium"` | `"heavy"` | `"shield"` | `"clothing"`
- `ac`: flat AC value this piece contributes
- `ac_base`: `true` only on chest pieces that set the base formula
- `dex_cap`: `null` (light), `2` (medium), `0` (heavy) — omit for non-base pieces
- `str_requirement`: integer, omit if none

---

## 12. Class Combat Features & Abilities

### Architecture — Already Implemented

The ability system is **already data-driven and already partially built**. Class abilities live in `game-data/systems/abilities/{class}/*.json` and the spells/abilities tab already exists in the UI as a placeholder for all classes. Everything below describes the system as it exists and how to extend it for full combat.

### Per-Class Resources

Each class has its own resource type defined in `game-data/systems/class-resources.json`. These are **not** D&D 5e slots — they are adapted to fit the game's real-time feel.

| Class     | Resource | Label | Regeneration         | Starts At |
| --------- | -------- | ----- | -------------------- | --------- |
| Fighter   | Stamina  | ST    | +2/turn              | Max       |
| Barbarian | Rage     | RG    | +10 per hit taken    | 0         |
| Monk      | Ki       | KI    | +1/turn              | Max       |
| Rogue     | Cunning  | CN    | +2/turn, +1 per crit | Max       |
| Wizard    | Mana     | —     | —                    | Per save  |
| Sorcerer  | Mana     | —     | —                    | Per save  |
| Warlock   | Mana     | —     | —                    | Per save  |
| Bard      | Mana     | —     | —                    | Per save  |
| Cleric    | Mana     | —     | —                    | Per save  |
| Druid     | Mana     | —     | —                    | Per save  |
| Paladin   | Mana     | —     | —                    | Per save  |
| Ranger    | Mana     | —     | —                    | Per save  |

Note: Barbarian Rage builds up as the barbarian takes hits rather than starting full — they get angrier as the fight goes on. Fighter Stamina and Monk Ki start at max and drain down.

### Ability Data Schema

Each ability JSON has:

```json
{
  "id": "second-wind",
  "name": "Second Wind",
  "class": "fighter",
  "unlock_level": 1,
  "resource_cost": 3,
  "resource_type": "stamina",
  "cooldown": "once_per_combat",
  "description": "...",
  "scaling_tiers": [
    {
      "min_level": 1,
      "max_level": 4,
      "effects_applied": ["second-wind-t1"],
      "summary": "Heal 25% max HP"
    }
  ]
}
```

Abilities scale via **tiers** keyed to level ranges, not a linear formula. Each tier references effect IDs from the effects system. Cooldowns can be `"none"`, `"once_per_combat"`, or a number (uses per combat). Higher tiers can `override_cooldown` to allow more uses.

### UI — Spells/Abilities Tab

All abilities and spells surface through the **existing spells/abilities tab**. Nothing gets its own separate screen. Per-class sub-tabs are possible within that tab if a class has enough abilities to warrant it. The tab already exists with placeholder content for all classes — combat implementation fills it in properly.

### Automatic vs Activated — Determine Per Ability

Some features never need a button press and should be evaluated silently by the backend:

- **Sneak Attack** — checked automatically on every eligible attack (advantage or ally adjacent)
- **Extra Attack** — backend grants additional attacks within the Attack action
- **Unarmored Defense** — passive AC calculation, applied in AC formula
- **Proficiency bonus** — passive attack modifier, always applied

Others are always explicit player choices shown as buttons in the abilities tab:

- **Rage, Second Wind, Action Surge** — player taps to activate
- **Ki spends** (Flurry, Patient Defense, Step of the Wind) — player chooses when
- **Cunning Action** — player chooses when to Dash/Disengage/Hide as bonus action

**Where it's ambiguous**: Clarify the specific behavior — automatic vs activated — as each ability is implemented. Note it in this document when decided. Do not assume D&D 5e's framing maps cleanly; some abilities that are explicit in D&D may work better as automatic here and vice versa.

### Current Ability Files

Fighter: `second-wind`, `power-strike`, `action-surge`, `disarming-blow`, `rally`, `indomitable`
Barbarian: `enter-rage`, `reckless-attack`, `intimidating-roar`, `savage-leap`, `blood-frenzy`, `berserker-mode`
Monk: `flurry-of-blows`, `patient-defense`, `stunning-strike`, `step-of-the-wind`, `deflect-missiles`, `quivering-palm`
Rogue: `sneak-attack`, `hide-in-shadows`, `poison-blade`, `evasion`, `assassinate`, `shadow-step`

Spellcaster classes use the spell system (Section 13) as their primary combat mechanism. Paladin and Ranger also have ability JSONs to be added as their melee/hybrid features are fleshed out.

---

## 13. Magic in Combat

> **Note**: The spell activation model, resource costs, and UI must be designed jointly with class combat features (Section 12) before implementation. The mechanics below describe the spell data that already exists and the rules that will apply — but the _how_ of tying them into the unified ability system is unresolved.

### Spell Attack Types

From existing spell data, we already have:

| `spell_attack` Value | What Happens                                              |
| -------------------- | --------------------------------------------------------- |
| `"automatic"`        | Always hits (Magic Missile)                               |
| `"ranged"`           | Make ranged spell attack (d20 + INT/WIS/CHA + prof vs AC) |
| `"melee"`            | Make melee spell attack (d20 + INT/WIS/CHA + prof vs AC)  |
| `null`               | Saving throw spell (see below)                            |

### Saving Throw Spells

If `spell_attack` is null and `save_type` is set:

```
Spell Save DC = 8 + proficiency_bonus + spellcasting_modifier
Monster rolls: d20 + monster_stat_modifier vs DC
Failed save → full effect
Passed save → half damage or no effect
```

### Concentration

Spells with `duration != "instantaneous"` often require concentration (need to add `concentration: true/false` to spell JSON).

- Only one concentration spell at a time
- Taking damage: CON save DC = max(10, half damage taken)
- Failed save → concentration breaks, spell ends
- Tracked in combat state: `concentration_spell` field

### Mana Cost

Already implemented via `mana_cost` on spells. Mana must be available to cast.

### Area Effect Spells

Spells tagged `area_effect` hit all monsters at the targeted range. For now, single encounters = single monster, so this mostly just deals full damage. When multi-monster encounters are added, area spells hit all enemies in range.

### How the Spell System Actually Works

Spell slots in this game **do NOT get consumed on casting**. They define how many spells of each level a caster can have **prepared** simultaneously. The number of slots allowed per level comes from `spell-slots.json[class][level][level_N]` — that cap is the preparation limit, not a casting resource.

**Save file `spell_slots` — simplified format** (no `used` boolean):

```json
"spell_slots": {
  "cantrips": ["fire-bolt", "prestidigitation", "mage-hand"],
  "level_1": ["magic-missile", "shield"],
  "level_2": ["misty-step", "scorching-ray"]
}
```

**To cast a prepared spell**: Mana (sufficient) **AND** all components present in inventory. Both must be satisfied. No slot is consumed — the spell can be cast again next turn.

### Cantrips

Cantrips work the same as leveled spells — spellcasters have a fixed number of cantrip slots and choose which cantrips to fill them with, just as they choose leveled spells. The number of slots comes from `spell-slots.json` like everything else. Cantrips have no component cost and typically cost 1 mana. They are not free or infinite — they are just the cheapest tier.

### Class Feature Adjustments for Mana System

| Feature                     | D&D 5e Version                                 | Adapted Version                                                 |
| --------------------------- | ---------------------------------------------- | --------------------------------------------------------------- |
| Paladin Divine Smite        | Expend spell slot → `(slot_level+1)d8` radiant | Spend N mana → N×d8 radiant (min 2 mana)                        |
| Warlock Short Rest Recovery | Regain all slots on short rest                 | Warlocks start with a higher mana pool instead                  |
| Arcane Recovery (Wizard)    | Regain slots (½ level) on short rest           | Restore mana = ½ Wizard level (rounded up) on short rest        |
| Cleric higher-level slots   | More powerful spells via higher slots          | Higher-tier spells stay prepared as level grows (same mechanic) |

---

## 14. Consumables in Combat

Items usable in combat as the **Use Item** action:

| Item              | Range                | Effect                                                           |
| ----------------- | -------------------- | ---------------------------------------------------------------- |
| Potion of Healing | 0 (self or adjacent) | `2d4+2` HP restored                                              |
| Antitoxin         | 0 (self)             | Advantage on poison saves for 1 hour                             |
| Alchemist's Fire  | Thrown (range 1-2)   | 1d6 fire per turn until DC10 DEX check to extinguish             |
| Oil               | 0-1                  | Slick surface — difficult terrain OR coat weapon for fire damage |
| Sacred Oil        | 0                    | Apply to weapon for radiant damage                               |

### Thrown Items as Attack Action

Alchemist's Fire and other thrown gear use the **Attack action**, not Use Item:

- Make a ranged attack (DEX + prof vs AC if catching, or just auto-hit on ground splash)
- Actually, Alchemist's Fire in D&D is: Attack roll, on hit = `1d4 fire` at START of each turn until target spends action to extinguish (DC 10 DEX)

### Healer's Kit

Can stabilize a dying creature (0 HP, removing death save requirement). Action. No healing, just stops the death spiral.

---

## 15. Conditions — Full Implementation

All conditions tracked as an array on the combat state. Each condition has defined mechanical effects.

| Condition         | Mechanical Effect                                                                                                                                                    |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Blinded**       | Auto-fail sight checks; attacks against you have Advantage; your attacks have Disadvantage                                                                           |
| **Charmed**       | Can't attack charmer; charmer has Advantage on social checks vs you                                                                                                  |
| **Deafened**      | Auto-fail hearing checks; no direct mechanical combat effect                                                                                                         |
| **Exhaustion**    | Level 1: Disadvantage on ability checks; Level 2: Speed halved; Level 3: Disadvantage on attacks AND saves; Level 4: Max HP halved; Level 5: Speed 0; Level 6: Death |
| **Frightened**    | Disadvantage on attacks and ability checks while source in sight; can't willingly move closer to source                                                              |
| **Grappled**      | Speed = 0; ends if grappler incapacitated or target moved out of reach                                                                                               |
| **Incapacitated** | Can't take Actions or Reactions                                                                                                                                      |
| **Invisible**     | Attacks against you have Disadvantage; your attacks have Advantage; auto-fail sight detection                                                                        |
| **Paralyzed**     | Incapacitated; auto-fail STR/DEX saves; attacks against have Advantage; melee attacks within range 0 auto-crit                                                       |
| **Petrified**     | Incapacitated + weight ×10; auto-fail STR/DEX saves; resistance to all damage; immune to poison/disease                                                              |
| **Poisoned**      | Disadvantage on attack rolls AND ability checks                                                                                                                      |
| **Prone**         | Must crawl (move costs double); melee attacks against: Advantage; ranged attacks against: Disadvantage; your attacks: Disadvantage                                   |
| **Restrained**    | Speed 0; your attacks: Disadvantage; attacks against you: Advantage; Disadvantage on DEX saves                                                                       |
| **Stunned**       | Incapacitated; auto-fail STR/DEX saves; attacks against have Advantage                                                                                               |
| **Unconscious**   | Incapacitated + Prone; auto-fail STR/DEX saves; attacks have Advantage; melee crits within range 0                                                                   |

### Condition Sources in Combat

- Paralyzed: Hold Person spell (WIS save), some monster abilities
- Poisoned: Poison damage, snake bites, some monsters
- Prone: Knocked down (some monster abilities, Shove action)
- Stunned: Monk Stunning Strike (CON save), some monster abilities
- Frightened: Menacing Attack, Dragon Frightful Presence, etc.
- Blinded: Darkness spell, some monster abilities

### Condition Removal

- Most end at start/end of next turn, or on a save
- Some are permanent until specific cure (Petrified, Paralyzed from Hold Person → WIS save each turn)
- Tracked in combat state with duration: `{ "condition": "poisoned", "duration_rounds": 3, "save_dc": 12, "save_stat": "constitution" }`

---

## 16. Death System

### Player at 0 HP

1. Player drops to 0 HP → **Unconscious** condition applied
2. Each round at 0 HP: Make a **Death Saving Throw** (d20, no modifiers)
   - Roll ≥ 10: **Success** (mark 1 success)
   - Roll < 10: **Failure** (mark 1 failure)
   - Natural 20: Regain 1 HP immediately + regain consciousness
   - Natural 1: Count as 2 failures
3. **3 Successes**: Player becomes Stable (stops rolling, stays unconscious)
4. **3 Failures**: Player dies (game over — handle with last save or resurrection mechanic)
5. Taking ANY damage at 0 HP: 1 auto-failure
6. Taking a crit at 0 HP: 2 auto-failures
7. Allies (future) can stabilize with Healer's Kit (action) or Cure Wounds

### Player Death Handling

On actual death (3 failures):

1. **Show death screen** with the final combat log
2. **Determine the 3 most valuable items** the player had on them:
   - Flatten all inventory into individual items: expand every stack into its constituent units (a stack of 20 arrows becomes 20 individual arrows), then add all equipped gear as single items
   - Sort every individual item by its `cost` value from the items data
   - Keep the top 3 individual items. If the same item type appears multiple times in the top 3, those units merge into a stack of that count in the kept slot
   - Example: 3 arrows (0.05gp each), 3 gold-piece (1gp each), 2 torches (say 0.1gp each) → sorted: gold-piece, gold-piece, gold-piece, torch, torch, arrow... but only 3 kept → player keeps 3 gold-piece
   - A stack of 500 gold-piece still only contributes individual units at 1gp each — it cannot dominate all 3 slots beyond what 3 units of gold-piece would be worth
3. **Strip the save file**:
   - All gear slots → empty
   - All general slots → empty
   - All backpack contents → empty
   - `gold-piece` stack → removed like any other item
4. **Place the 3 items** into general slots 1–3 (the quick-access slots). If an item was a stack, carry over the full stack as one slot entry.
5. **Return player to starting city** — set `location` to the player's racial starting city (from `starting-locations.json`, same value used at character creation)
6. **Restore HP and mana to full** — player wakes up recovered, just stripped of gear
7. **Persist XP and level** — death does not roll back experience or level gains
8. **Session memory reflects the stripped state** — the player is prompted to save. No forced write: in the full game saves are signed Nostr events the player must publish themselves. The test server local save behaves the same way for consistency.

**Death screen message**: Something flavourful — _"You wake in [Starting City], stripped of your belongings by whoever dragged you back from the brink."_

**No resurrection mechanic planned** — death is a hard reset of inventory with a location warp, not a game-over.

### Monster at 0 HP

Most monsters simply die. Some mechanics for the future:

- Undead: Some reform unless holy damage
- Lycanthropes: Revert to human form
- Legendary monsters: May have legendary resistances

---

## 17. Monster AI

Monsters make decisions each turn. Simple AI hierarchy:

### Decision Tree (Per Monster Turn)

```
1. Am I alive? (hp > 0)
2. Should I flee? (hp < flee_threshold% of max) → Disengage + move away
3. Can I use a special ability? (has SP ability ready, roll chance)
4. What is my preferred_range vs current range?
   - If prefer melee (0) and at range > 0 → Move toward player
   - If prefer ranged (2+) and at range 0 → Disengage or Dash away
5. Attack if in range, else move toward preferred range
6. Bonus action if available (Nimble Escape, etc.)
```

### Monster Behavior Types

- `"aggressive"`: Always moves toward player, attacks immediately
- `"defensive"`: Prefers ranged, retreats if outnumbered (future)
- `"cautious"`: Stays at optimal range, uses abilities strategically
- `"berserker"`: Never flees, attacks recklessly (advantage on attacks, but lower defense)

### Target Priority (Future Multi-Monster)

- `"lowest_hp"`: Focus fire on most wounded
- `"highest_threat"`: Target whoever dealt most damage
- `"random"`: Random target each turn

---

## 18. Flee Mechanic

### Flee Availability

The **Flee** action only appears as a combat option when **current range ≥ 3**. At range 0–2 the option is greyed out with tooltip: _"Too close to escape."_

### Flee Action Cost

- Uses the player's **full Action** (not bonus action)
- Player must be at range ≥ 3 at **start of their turn** — move first, then declare Flee

### Flee Chance Formula

```
// Speed comparison — both sides use the same athletics formula
monster_athletics    = (monster.STR × 0.5) + (monster.CON × 0.35) + (monster.DEX × 0.15)
speed_advantage      = player_athletics - monster_athletics
speed_modifier       = speed_advantage × 0.03     // e.g. athletics 14 vs score 10 → +0.12

// Range provides the base chance
base_chance = (range - 2) × 0.25                  // range 3=25%, 4=50%, 5=75%, 6=90%

// Penalties
fatigue_penalty           = max(0, (fatigue - 7) × 0.05)   // fatigue 9 → -0.10
monster_persistence_penalty = monster.behavior.relentless ? -0.20 : 0

final_chance = clamp(base_chance + speed_modifier - fatigue_penalty - monster_persistence_penalty, 0.05, 0.95)
```

Range 6 is still capped at 90% — relentless monsters (wolves, lycanthropes) always have some chance to run the player down.

### On Flee Success

- Combat ends immediately
- Player keeps all XP earned so far (no kill bonus awarded)
- Player returns to travel at current position (slightly behind encounter trigger point)
- Log: _"You manage to put enough distance between you and the goblin to escape!"_

### On Flee Failure

- Monster gets an **Opportunity Attack** if player was at range 0–1 before the flee attempt
- Aggressive monsters advance range by 1 (behavior-dependent)
- Combat continues; player must wait until next turn to try again
- Log: _"The goblin cuts off your escape! You're still in combat."_

### Relentless Monsters (Monster JSON Flag)

```json
"behavior": { "relentless": true }
```

Relentless monsters impose −20% to flee chance and always pursue on failed flee (advance range by 1 regardless of behavior type). Applies to: wolves, lycanthropes, and similar aggressive pursuers.

---

## 19. Combat UI Design

### Layout

Combat reuses the existing game UI — no separate full-screen combat view. Each region of the UI takes on a combat-specific role:

**Scene image area (top/center)**

- Environment background stays as-is
- Monster image overlaid on top of the scene
- Overlay elements on the scene image (not replacing it):
  - Monster name
  - Monster HP bar
  - Round counter
  - Range indicator
  - Flair popups: damage numbers, status effects applied, crits — appear and fade over the scene
  - Active conditions on the monster shown as badges

**Left message box (combat log)**

- Replaces the normal location/event text during combat
- Scrolling text log of everything that happened this combat
- Each line: `> You attack with Longsword: 19 vs AC 15 — HIT! 9 slashing damage.`

**Bottom button area (actions)**

- Replaces travel/NPC buttons while in combat
- Primary action buttons: `[ Attack ]  [ Abilities ]  [ Use Item ]  [ Move ]  [ Flee ]`
- Clicking a button opens its submenu inline (weapon list, ability list, item list, etc.)

**Right panel (player stats)**

- HP, mana/class resource already displayed here in normal UI — unchanged during combat
- No duplication needed

### Action Submenus (Bottom Area)

**Attack**: Equipped weapons as options; off-hand if applicable; thrown weapons if in inventory. Grayed out if out of range.

**Abilities**: The existing spells/abilities tab content surfaced inline — prepared spells and class abilities, filtered to what's currently usable (enough resource, correct range, action type available).

**Use Item**: Combat-usable items from inventory — potions, thrown items, etc.

**Move**: `[ Closer ]  [ Away ]  [ Dash ]` — shows current range and what each step would do.

**Flee**: Only active when range ≥ 3. Shows estimated flee chance. Grayed with tooltip at range 0–2.

---

## 20. XP & Loot

### XP Awards — Per Damage Dealt (Real-Time)

XP is applied to **session memory** immediately when damage is dealt — not held until end of combat.

```
xp_per_hp       = monster.xp / monster.max_hp
xp_this_hit     = damage_dealt × xp_per_hp × night_multiplier × player.xp_multiplier
```

- `night_multiplier` = 1.25 if `is_night`, else 1.0
- `player.xp_multiplier` = the `XPMultiplier` for the player's current level from `advancement.json`

Combat log shows XP as it accumulates: _"+4 XP"_. **Fleeing preserves all XP** — session memory already has it. HP regeneration (e.g. Troll regen) does **not** refund XP already earned for those hits. XP only reaches disk on a manual save (or on death, which forces a write).

### Level Thresholds & XP Multipliers

Defined in `game-data/systems/advancement.json`. Exponential scaling (OSRS-influenced). Each level grants an `XPMultiplier` applied to **all XP gained** at that level — higher levels earn faster to offset the steeper curve.

| Level | Total XP Required | XP Multiplier |
| ----- | ----------------- | ------------- |
| 1     | 0                 | 1.00×         |
| 2     | 250               | 1.05×         |
| 3     | 650               | 1.10×         |
| 4     | 1,350             | 1.15×         |
| 5     | 2,750             | 1.20×         |
| 6     | 4,750             | 1.25×         |
| 7     | 8,050             | 1.30×         |
| 8     | 13,425            | 1.35×         |
| 9     | 22,300            | 1.40×         |
| 10    | 36,700            | 1.45×         |
| 11    | 66,800            | 1.50×         |
| 12    | 110,000           | 1.55×         |
| 13    | 180,000           | 1.60×         |
| 14    | 296,000           | 1.65×         |
| 15    | 486,000           | 1.70×         |
| 16    | 797,000           | 1.75×         |
| 17    | 1,445,000         | 1.80×         |
| 18    | 2,370,000         | 1.85×         |
| 19    | 3,885,000         | 1.90×         |
| 20    | 6,380,000         | 1.95×         |

**Do not hardcode these values in Go.** Load `advancement.json` at startup like other game data.

### Kill Bonus (Post-Combat Only)

Only monsters with an explicit `kill_bonus_xp` field grant a kill bonus. Also multiplied by `player.xp_multiplier`. Standard monsters omit this field. Bosses and notable enemies have it set to whatever value fits.

```json
{ "kill_bonus_xp": 75 }
```

- **Standard monsters (CR < 3)**: No kill bonus — XP is purely damage-proportional
- **Big monsters (CR 3–5)**: `kill_bonus_xp` ≈ 25% of base `monster.xp`
- **Boss monsters** (`"boss": true`): `kill_bonus_xp` ≈ 50% of base `monster.xp`

### Level Up System

On level up:

- HP increases: Roll `hit_die`, add CON modifier (minimum 1)
- Proficiency bonus updates
- Class features granted (if level milestone)
- New spell slots (if caster)
- Mana pool update
- `xp_multiplier` updates to the new level's value from `advancement.json`

### Loot

After combat a **loot UI** appears showing what the monster dropped. The player clicks items to pick up; anything not taken is left on the ground (discarded — no persistent ground state needed yet). Nothing is auto-added to inventory.

#### Drop Table Format (OSRS-inspired)

Tables support tiers and guaranteed drops, not just a flat chance-per-item list:

```json
"loot_table": {
  "guaranteed": [
    { "item": "gold-piece", "quantity": [2, 6] }
  ],
  "rolls": 2,
  "tiers": [
    {
      "name": "common",
      "weight": 70,
      "entries": [
        { "item": "shortsword",  "weight": 30, "quantity": [1, 1] },
        { "item": "leather-cap", "weight": 20, "quantity": [1, 1] },
        { "item": "nothing",     "weight": 50 }
      ]
    },
    {
      "name": "rare",
      "weight": 25,
      "entries": [
        { "item": "potion-of-healing", "weight": 60, "quantity": [1, 1] },
        { "item": "gold-piece",        "weight": 40, "quantity": [10, 25] }
      ]
    },
    {
      "name": "very_rare",
      "weight": 5,
      "entries": [
        { "item": "goblin-ear", "weight": 100, "quantity": [1, 1] }
      ]
    }
  ]
}
```

**`rolls`**: How many times to roll the tier table. Each roll picks a tier by weight, then picks an entry within that tier by weight. `"nothing"` entries are valid empty rolls. Guaranteed drops always appear.

#### Loot Pickup UI

- Appears after combat ends
- Shows each dropped item with icon and quantity
- Player clicks items to take them (go into general slots / backpack)
- "Take All" button for convenience when inventory has space
- Untaken items are discarded

---

## 21. Environment → Encounter Schema

Add to each environment JSON:

```json
{
  "encounter_settings": {
    "encounter_check_interval_minutes": 30,
    "base_encounter_chance_per_interval": 0.25,
    "stealth_modifier": 2,
    "terrain_type": "forest",
    "encounter_schedule": [
      {
        "name": "dawn",
        "start": 300,
        "end": 419,
        "rate_mult": 0.8,
        "diff_mult": 0.9
      },
      {
        "name": "day",
        "start": 420,
        "end": 1019,
        "rate_mult": 1.0,
        "diff_mult": 1.0
      },
      {
        "name": "dusk",
        "start": 1020,
        "end": 1199,
        "rate_mult": 1.3,
        "diff_mult": 1.2
      },
      {
        "name": "night",
        "start": 1200,
        "end": 299,
        "rate_mult": 1.8,
        "diff_mult": 1.5
      }
    ]
  }
}
```

**`stealth_modifier`**: Environment stealth modifier added to the player's pre-encounter stealth roll. Dense forest might be +2; open road −3; mountain pass 0.

**Schedule time windows** use `TimeOfDay` minutes (0–1439). Night wraps midnight: `start: 1200, end: 299` means `time >= 1200 || time < 300`.

Monster selection uses existing `environment[]` arrays on monster JSONs — filter monsters whose environment list includes this terrain type, then apply CR ceiling (Section 22).

Random encounters are **monster-only** for now. Discoveries and random events will be defined in their own Point of Interest JSONs when that system is implemented — each POI carries its own discovery mechanics (class, time of day, level, ability score conditions, etc.). No `encounter_types` split in this schema.

---

## 22. Monster Difficulty Scaling

CR scaling sets a **ceiling** to protect low-level players from overwhelming encounters. It is **not** a narrow band — high-level players can and will still encounter low-CR monsters; those just become uncommon.

### Monster Selection Formula

```
max_cr    = player_level × 1.5 × schedule.diff_mult   // hard cap
ideal_cr  = player_level × 0.5                        // most common target
```

**Step 1 — Filter**: `cr <= max_cr` AND environment matches current terrain
**Step 2 — Weight**: Selection probability tilted toward appropriately challenging monsters

| CR vs ideal_cr          | Weight                    |
| ----------------------- | ------------------------- |
| Within ±50% of ideal_cr | **3×** (most common)      |
| Below that range        | **1×** (still selectable) |
| Above max_cr            | **Excluded** (hard cap)   |

**Examples**:

| Player Level | max_cr | ideal_cr | Can Encounter     | Excluded |
| ------------ | ------ | -------- | ----------------- | -------- |
| 1            | 1.5    | 0.5      | CR 0.125–1.5      | CR 2+    |
| 5            | 7.5    | 2.5      | CR 0.125–7.5      | CR 8+    |
| 10           | 15     | 5        | Any CR ≤ 15       | CR 16+   |
| 20           | 30     | 10       | Any CR (uncapped) | Nothing  |

A level 20 player CAN still encounter a CR 0.25 goblin — it simply has very low selection weight compared to CR 8–10+ threats. World-realism preserved, low-level player safety protected.

### HP Scaling (Environmental Encounters Only, Not Dungeon Static)

After monster selection, apply a small HP adjustment to fine-tune difficulty:

```
scaled_hp = clamp(base_hp × (ideal_cr / monster_base_cr), base_hp × 0.7, base_hp × 1.3)
```

XP from per-damage-dealt formula automatically reflects actual HP dealt — no separate XP scaling needed.

---

## 23. Saving Throws in Combat

When a spell or ability requires a save:

```
Monster Save: d20 + monster.saving_throws[stat] (or stat_modifier if not listed)
vs. Player Spell Save DC = 8 + proficiency + spellcasting_modifier
```

Some monster special abilities also require player saves:

```
Player Save: d20 + player.stats[stat] modifier
vs. Monster Ability DC (defined in monster action)
```

---

## 24. Short Rest & Long Rest (Combat Relevance)

Some class features recharge on short rest (1 hour of downtime):

- Fighter: Second Wind, Action Surge
- Monk: Ki Points
- Warlock: Spell Slots

Long rest (8 hours sleep, needs a safe rest spot) recharges:

- All HP
- All Spell Slots / Mana
- Most class features
- Barbarian: Rage uses
- Paladin: Lay on Hands pool

These need to be tracked in the save file and tied to the rest system.

---

## 25. Implementation Phases

### Phase 1: Core Combat Engine (Foundation) ✅ COMPLETE (2026-02-21)

**Goal**: Working combat for basic melee encounters

- [x] Add full stat blocks to priority monsters (see Section 27)
- [x] Add XP values and loot tables to those monsters
- [x] Combat session memory struct in Go (keyed by npub, never written to save file)
      → `types/combat.go`: CombatSession, PartyCombatant, MonsterInstance, InitiativeEntry, PlayerCombatState
      → `session/types.go`: `ActiveCombat *types.CombatSession` with `json:"-"`
- [x] Initiative calculation (player d20+DEX, monster DEX-mod tiebreaker)
      → `cmd/server/game/combat/combat.go`: `rollInitiatives`, `buildInitiativeOrder`
- [x] Basic attack resolution: d20 + STR/DEX + prof vs AC; finesse; crits; critical misses
      → `cmd/server/game/combat/attack.go`
- [x] Basic damage: weapon dice + modifier; resistances/immunities/vulnerabilities; crits double dice
      → `cmd/server/game/combat/damage.go`
- [x] HP tracking for player and monster in session memory
- [x] Win condition: monster → 0 HP → loot roll → `phase="loot"` → `POST /api/combat/end` applies XP + loot
      → `handleMonsterKill`, `applyVictoryOutcome`, `addLootToInventory`
- [x] Lose condition: player → 0 HP → `phase="death_saves"` → 3 failures → `phase="defeat"` → strips inventory
      → `ProcessDeathSave`, `applyDefeatOutcome`, `stripInventoryForDeath`
- [x] Combat log returned per round (`new_log`) and cumulative (`log`)
- [x] XP applied per hit to session memory; `level_up_pending` set when threshold crossed
- [x] Save file stores raw `experience`; level derived at runtime from `advancement.json`
- [x] Basic loot roll using tiered drop table (Section 20) → `cmd/server/game/combat/loot.go`
- [x] Save blocking during active combat → `cmd/server/api/saves.go` returns 409 Conflict

**Endpoints delivered (5 total — one more than originally planned):**

- `POST /api/combat/start` — init encounter, roll initiative, optional monster-first-turn
- `GET  /api/combat/state` — re-sync state after page refresh
- `POST /api/combat/action` — player attack round (move + attack + monster response)
- `POST /api/combat/death-save` — one death save + monster response
- `POST /api/combat/end` — apply victory/defeat outcome, clear combat

**Files created/modified:**

- `types/combat.go`, `session/types.go`
- `cmd/server/game/combat/`: dice.go, loader.go, attack.go, damage.go, ai.go, loot.go, xp.go, combat.go
- `cmd/server/api/game/combat.go` (handlers + Swagger docs)
- `cmd/server/api/routes.go`, `cmd/server/api/saves.go`

**Known Phase 1 simplifications (intentional):**

- 3 death save successes → `phase="victory"` (stabilise = combat ends; monster still technically present but walks away)
- Unconscious player AC = `10 + DEX mod` (simplified; `resolveDeathSaveAttack` has no DB access)
- Monster AI selects first usable action, not highest-threat action
- Ranged disadvantage: only ranged-weapon-at-range-0; melee-vs-ranged-monster not yet handled
- `KillBonusXP` always returns 0 until `kill_bonus_xp` field is added to MonsterData JSON schema

### Phase 2: Weapon Properties ✅ COMPLETE (2026-02-21)

**Goal**: All weapon tags work correctly

- [x] Finesse: Choose best of STR/DEX _(was already done in Phase 1)_
- [x] Versatile: 1H vs 2H damage choice _(was already done in Phase 1)_
- [x] Two-handed: Validation and blocking of offhand _(was already done in Phase 1)_
- [x] Heavy: Disadvantage for Small races (halfling, gnome)
      → `resolveAttackAdvantage()` checks `save.Race` against "halfling"/"gnome"
- [x] Light + two-weapon fighting: bonus action off-hand attack; no ability mod on damage
      → `hand="off"` in request; `validateTwoWeaponFighting()`; `resolvePlayerDamageNoMod()`
- [x] Thrown: Ranged attack with DEX, consumes item from gear slot
      → `thrown=true` in request; `consumeFromGearSlot()`; checks `thrown` tag
- [x] Ammunition: Consume from `gear_slots["ammunition"]` per ranged attack; blocked if empty
      → `consumeAmmo()`; `AmmoUsedThisCombat` counter on `CombatSession`
- [x] Ammunition recovery: 50% of ammo used recovered on victory
      → `addAmmoToSlot()` called in `applyVictoryOutcome()`
- [x] Loading: Bonus action blocked when main hand has `loading` tag
      → checked in `validateTwoWeaponFighting()`
- [x] Reach: Weapons with `reach` tag can attack at range 0 or 1
      → `getMeleeReach()` reads `range` field on reach weapons
- [x] Melee range gate: Attack blocked if enemy is beyond weapon reach
      → `validateAttackRange()` returns error with clear message
- [x] Long-range disadvantage: ranged attacks beyond normal range get disadvantage
      → `resolveAttackAdvantage()` uses `getRangedReach()` to compare `cs.Range`
- [x] Out-of-max-range block: ranged attack beyond long range returns error
      → `validateAttackRange()` computes `maxRange` from `range-long` (fallback: `range`)
- [x] `bonus_attack_available` added to `CombatStateResponse`
      → `checkBonusAttackAvailable()` runs after every action
- [x] `ammo_remaining` added to `CombatStateResponse`
      → `getAmmoRemaining()` reads from ammo gear slot

**Files created/modified:**

- `types/combat.go` — added `AmmoUsedThisCombat int` to `CombatSession`
- `cmd/server/game/combat/attack.go` — added `parseRangeInt`, `getMeleeReach`, `getRangedReach`; updated `resolveAttackBonus` and `resolveAttackAdvantage`
- `cmd/server/game/combat/combat.go` — updated `ProcessPlayerAttack` (new `hand`/`thrown` params); added `validateAttackRange`, `validateTwoWeaponFighting`, `consumeAmmo`, `consumeFromGearSlot`, `slotQty`, `resolvePlayerDamageNoMod`; updated `resolvePlayerDamage`
- `cmd/server/api/game/combat.go` — added `Hand`/`Thrown` to request; `BonusAttackAvailable`/`AmmoRemaining` to response; added helper functions; ammo recovery in `applyVictoryOutcome`

**Intentional simplifications / known gaps:**

- Thrown attacks always use DEX (plan spec); D&D 5e uses STR unless finesse — this is a deliberate simplification
- Loading only blocks bonus actions (two-weapon fighting); it does not prevent a second main-hand attack if the player calls the action endpoint twice in one "round"
- `BonusActionUsed` resets when the player makes a new main-hand attack — no strict per-round turn enforcement yet

### Phase 3: Unified Ability System (Class Features + Magic)

**Goal**: One pipeline for all active combat abilities — class features and spells both flow through it

> This phase requires design work first (see Section 12). Do not begin implementation until the resource model, data schema, and UI approach are settled.

- [ ] Design unified ability schema (data-driven vs hardcoded handlers decision)
- [ ] Design resource model (one pool vs per-class pools vs hybrid)
- [ ] Design combined ability/spell UI panel
- [ ] Implement automatic passive features (Sneak Attack check, Extra Attack, Unarmored Defense)
- [ ] Implement activated martial abilities (Rage, Second Wind, Action Surge, Ki, Superiority Dice)
- [ ] Implement hybrid features (Divine Smite, Hunter's Mark, Channel Divinity)
- [ ] Integrate with magic system (Section 13) through the same activation pipeline

### Phase 4: Magic in Combat

**Goal**: Full spell integration

- [ ] Spell selection UI in combat
- [ ] Ranged/melee spell attacks (d20 + spellcasting mod + prof)
- [ ] Auto-hit spells (Magic Missile)
- [ ] Saving throw spells (DC calculation, monster roll)
- [ ] Area effect spells (hit all monsters)
- [ ] Concentration tracking and breaking on damage
- [ ] Mana deduction
- [ ] Healing spells in combat
- [ ] Metamagic (Sorcerer — Quickened, Twinned, Empowered)

### Phase 5: Conditions

**Goal**: All 15 conditions function correctly

- [ ] Condition application and tracking
- [ ] Condition expiration (duration-based, save-based)
- [ ] Advantage/Disadvantage from conditions
- [ ] Speed effects (Grappled, Restrained = speed 0)
- [ ] Auto-crit conditions (Paralyzed, Unconscious in melee)
- [ ] Concentration breaking on damage

### Phase 6: Monster Special Abilities

**Goal**: Monsters have unique mechanics

Priority abilities (implement these on targeted monsters first):

- [ ] Nimble Escape (Goblin): Bonus action Disengage/Hide
- [ ] Undead Fortitude (Zombie): CON save on 0 HP to survive with 1 HP
- [ ] Pack Tactics (Wolf/Kobold): Advantage when ally adjacent to target
- [ ] Frightful Presence (Dragons): WIS save or Frightened
- [ ] Poison (Spider/Snake): CON save or Poisoned condition
- [ ] Multi-attack: Some monsters attack 2+ times per turn
- [ ] Regeneration (Troll): Regain HP each turn unless acid/fire damage

### Phase 7: Flee & Tactical Depth

**Goal**: Combat has strategic options beyond just attacking

- [ ] Flee mechanic (Disengage + move away checks)
- [ ] Dodge action (disadvantage on incoming attacks)
- [ ] Hide attempt (Stealth vs Perception)
- [ ] Shove action (range change)
- [ ] Opportunity attacks (when monster moves away)

### Phase 8: Encounter System Integration

**Goal**: Encounters trigger properly during travel

- [ ] Encounter check formula (time × rate × weather)
- [ ] Day/night schedule with rates/difficulty
- [ ] Monster selection by environment tag + CR matching
- [ ] HP scaling for difficulty
- [ ] Environment encounter data added to all environment JSONs
- [ ] Random event encounters (non-combat) — connect to event system

### Phase 9: Death & Rewards Polish

**Goal**: Complete game-loop closure

- [ ] Death saving throws UI
- [ ] Stabilization (healer's kit)
- [ ] Death game-over screen with options
- [ ] Full loot UI (items added to inventory with overflow handling)
- [ ] XP display and level up notification
- [ ] Level up stat allocation UI (HP roll shown to player)

### Phase 10: Multi-Monster Encounters (Future)

**Goal**: Fight groups of monsters

- [ ] Multiple monster instances in combat state
- [ ] Initiative for each monster
- [ ] Target selection UI
- [ ] Area effect hitting multiple
- [ ] Flanking optional rule

---

## 26. Open Questions

1. **Multi-monster encounters**: Start with 1 monster for simplicity, add groups in Phase 10?
   - Proposed: Yes, single monster initially. Group encounters when system is stable.

2. **Flee distance**: How far must player go to escape? Propose: range > 5 OR 2 consecutive turns at max range.

3. **Death persistence**: ✅ Resolved — Player wakes at racial starting city keeping the 3 most valuable **individual items**: all stacks are flattened into units, sorted by unit `cost`, top 3 kept (if 2 of those are the same item they merge back into a stack of 2). Everything else lost. XP and level kept. Session memory updated; player prompted to save. No forced write. No load-last-save, no resurrection.

4. **Monster loot quantity**: Some monsters (goblins) should drop small `gold-piece` stacks, rare monsters more. Gold is an item like any other — quantity in `loot_table` determines the stack size dropped. Balance TBD.

5. **Short rest in environment**: Can player rest mid-travel? After combat? Propose: Player can camp (see environment-poi-system.md rest spots) to recover short rest features, but takes time.

6. **Sneak Attack conditions**: In single combat (no allies), Sneak Attack only triggers with Advantage. This means Rogue needs Stealth or some other advantage source. Is this fair?
   - Option: Cunning Action Hide lets Rogue hide and gain advantage frequently.

7. **Paladin/Cleric armor restriction**: Both need Strength check to wear heavy armor. Non-proficient classes have disadvantage on all physical activities. How to enforce?

8. **Critical Fumbles**: Natural 1 causes what? Currently: just miss. Option: -1 to initiative for next round, or dropped weapon chance.

9. **Combat speed in UI**: Show full animation/narration per action (slower, more dramatic) or instant resolve with log? Propose: Show narrated log with slight delay per line.

10. **Saving multiple targets**: If Fireball hits the one monster, it's just damage. Future: Area effects when multi-monster arrives.

---

## 27. Priority Monster List for Phase 1 Data Entry

These 30 monsters should have full stat blocks added first (most likely to be encountered in early game environments):

| Monster       | CR    | Environment             | Priority |
| ------------- | ----- | ----------------------- | -------- |
| Goblin        | 0.25  | forest, mountain, urban | HIGH     |
| Goblin Boss   | 1     | forest, mountain, urban | HIGH     |
| Wolf          | 0.25  | forest, grassland       | HIGH     |
| Bandit        | 0.125 | highway, urban          | HIGH     |
| Skeleton      | 0.25  | dungeon, graveyard      | HIGH     |
| Zombie        | 0.25  | dungeon, graveyard      | HIGH     |
| Kobold        | 0.125 | mountain, dungeon       | HIGH     |
| Rat (Swarm)   | 0.25  | dungeon, urban          | HIGH     |
| Stirge        | 0.125 | swamp, cave             | HIGH     |
| Giant Spider  | 1     | forest, dungeon         | HIGH     |
| Orc           | 0.5   | mountain, forest        | HIGH     |
| Bugbear       | 1     | forest, dungeon         | HIGH     |
| Hobgoblin     | 0.5   | forest, mountain        | HIGH     |
| Harpy         | 1     | mountain, coast         | MED      |
| Lizardfolk    | 0.5   | swamp, coast            | MED      |
| Gnoll         | 0.5   | grassland, desert       | MED      |
| Giant Rat     | 0.125 | dungeon, urban          | MED      |
| Blood Hawk    | 0.125 | mountain, coast         | MED      |
| Merrow        | 2     | coast, swamp            | MED      |
| Ghoul         | 1     | dungeon, graveyard      | MED      |
| Wight         | 3     | dungeon, graveyard      | MED      |
| Troll         | 5     | mountain, forest        | MED      |
| Owlbear       | 3     | forest                  | MED      |
| Manticore     | 3     | mountain, desert        | LOW      |
| Werewolf      | 3     | forest, urban           | LOW      |
| Ogre          | 2     | mountain, grassland     | LOW      |
| Wraith        | 5     | dungeon, graveyard      | LOW      |
| Banshee       | 4     | dungeon, haunted        | LOW      |
| Vampire Spawn | 5     | dungeon, urban          | LOW      |
| Young Dragon  | 7-10  | varies                  | LOW      |

---

## 28. Technical Architecture Notes

### Backend (Go)

- New package: `game/combat/`
  - `combat.go` — combat state management, round resolution
  - `attack.go` — attack roll calculation
  - `damage.go` — damage resolution, resistances
  - `conditions.go` — condition application and effects
  - `ai.go` — monster decision making
  - `encounter.go` — encounter triggering and monster selection
  - `loot.go` — loot table rolling

### API Endpoints Needed

| Endpoint             | Method | Description                                             |
| -------------------- | ------ | ------------------------------------------------------- |
| `/api/combat/start`  | POST   | Trigger encounter, roll initiative, return combat state |
| `/api/combat/action` | POST   | Player takes action (attack/spell/item/flee)            |
| `/api/combat/end`    | POST   | Resolve end of combat (loot, XP)                        |
| `/api/combat/state`  | GET    | Get current combat state                                |

### Frontend

- Combat UI as a separate "view" that overlays when in combat
- Combat actions call backend, receive updated combat state + log entries
- No game logic in JavaScript — just render the state returned by backend

### Save File Changes

- No `active_combat` field — combat lives in session memory only (see Section 5)
- Save file is written to disk on manual save only; session memory holds live state between saves
- Add `level_up_pending: bool` to session memory for when XP threshold crossed mid-combat
- Add `short_rest_features_used` tracking per rest cycle

---

## 29. Stealth & Surprise

### Pre-Encounter Stealth Phase (Automatic Passive Roll)

When an encounter check triggers during travel, the game silently runs a stealth check **before** locking in the encounter. The player receives no prompt and has no knowledge of the roll either way.

**Stealth Roll Formula**:

```
roll = d20 + DEX_modifier + stealth_proficiency (if proficient)
roll -= fatigue_penalty (see table below)
roll -= armor_penalty (heavy armor = -5, medium armor = -2)
roll += environment.stealth_modifier (from encounter_settings JSON)
```

**Fatigue Penalty to Stealth**:

| Fatigue | Stealth Penalty |
| ------- | --------------- |
| 0–5     | 0               |
| 6       | −1              |
| 7       | −2              |
| 8       | −3              |
| 9       | −5              |
| 10      | −7              |

**Outcome**:

- **Roll > Monster Passive Perception** (`10 + WIS_mod`): Player sneaks past. Encounter does NOT trigger. No XP (no combat occurred).
- **Roll ≤ Monster Passive Perception**: Encounter triggers. Proceed to surprise determination.

### Surprise Determination (When Encounter Triggers)

After the stealth phase fails, the **margin of failure** determines if anyone is surprised:

```
margin = monster.passive_perception - stealth_roll
```

| Margin | Result                                                                                                |
| ------ | ----------------------------------------------------------------------------------------------------- |
| 1–4    | Normal combat. Both sides roll initiative normally.                                                   |
| 5+     | Monster gets a full **surprise round**. Player cannot act in round 1; monster attacks with advantage. |

High fatigue makes extreme-failure surprises far more common — exhausted travel is genuinely dangerous at night.

**Combat log entry on surprise**: _"You stumble into the goblin's path — it attacks before you can react!"_

**Note**: Monster `passive_perception` should be stored in the monster JSON under `senses.passive_perception` (already included in the schema defined in Section 4).

---

## 30. Party & Companion Architecture Note

This section flags architectural decisions to keep in mind during Phase 1. **Nothing here is implemented yet.** The goal is to avoid data structure choices now that require a redesign later.

### Key Point: Use `party[]` From Day One

The `active_combat` state (Section 5) uses a `party` array rather than a single player object. Phase 1 always has exactly 1 entry. Future companions or multiplayer slots into the same array additively — no redesign needed.

**Companion types (future scope)**:

- `"type": "companion"` — Hired NPC, AI-controlled, persistent across saves
- `"type": "player"` with a different `id` — Multiplayer party member

### Sneak Attack with Companions

When companions are added, the "ally adjacent to target" condition (range 0) becomes easier to satisfy. The Sneak Attack logic should check all `party` members when evaluating adjacency — not a hardcoded ally flag.

```go
// Pseudocode — check party members, not a boolean flag
hasAllyAdjacent := false
for _, member := range combat.Party {
    if member.ID != attackerID && combat.Range == 0 {
        hasAllyAdjacent = true
    }
}
```

### No Behavioral Changes for Phase 1

During Phase 1 implementation, treat `party[0]` exactly as the current single-player model. The array wrapper costs nothing and future-proofs the schema.

---

**Document Status**: Living document — will expand as implementation progresses.
**Phase 1**: ✅ Complete (2026-02-21) — backend engine + monster data + HTTP handlers
**Phase 2**: ✅ Complete (2026-02-21) — all weapon properties implemented
**Next Step**: Phase 9 (Combat UI frontend) — backend is ready to wire up
