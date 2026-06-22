# Feats — acquisition & progression (draft)

**Status:** design draft, not implemented. The feat *content* list is [`feats.json`](feats.json)
(this dir) — a narrowed set chosen to work for the game. Acquisition is gated behind M4/M5
(see *Slotting*). This note defines *when and how* a character gains feats and how that
coexists with ability-point allocation without breaking the §4 hydration rule.

## The choice: feat OR ability point

Progression grants land on a fixed **cadence of levels**. At each cadence level a character
gains **+1 ability point**; at *feat-eligible* levels they may instead **choose a feat** for
that level. Mutually exclusive — a feat consumes that level's point.

- **Points bank.** Added to an unspent pool, spent on any ability later (+1 at a time,
  per-stat cap **20**, total cap **100**).
- **Feats commit.** Pick a specific feat at the level; its effects apply immediately.
  Half-feats (Actor, Durable, Resilient, Keen Mind, Observant) also grant their +1 to a
  stat as part of the feat.
- **The trade preserves the ceiling.** A no-feat character banks all 11 points → 100 max
  total (the [about-page](../../www/views/tabs/about.html) promise). Every feat trades one
  stat point for an ability.

## Cadence

Ability-point levels (all classes): **2, 4, 6, 8, 10, 12, 14, 16, 18, 19, 20** → 11 points.

Feat-eligible levels are a *subset* — choose a feat instead of that level's point:

| Class | Feat-eligible levels | Max feats | Min guaranteed points |
|---|---|---|---|
| Most | 4, 8, 12, 16, 19 | 5 | 6 |
| Fighter | 4, 6, 8, 12, 14, 16, 19 | 7 | 4 |
| Rogue | 4, 8, 10, 12, 16, 19 | 6 | 5 |

Mirrors D&D 5e's ASI cadence (base 4/8/12/16/19; Fighter +6/14; Rogue +10) — but the slot is
spent from the shared 11-point schedule instead of a separate ASI track. Every feat level is
already a point level, so a feat always cleanly consumes a point.

## Save storage & derivation (hydration-first, §4)

Two stored player-choice fields — nothing else:

- `AbilityIncreases map[string]int` — points spent per ability (the only record of
  allocation; base stats stay a creation snapshot, never re-derived from the editable
  generation tables).
- `FeatsChosen []string` — selected feat IDs.

Everything else derives. The accounting identity holds regardless of *when* a choice was made:

```
cadenceSlots(level) = len(FeatsChosen) + Σ(AbilityIncreases) + unspentPoints
→ unspentPoints = cadenceSlots(level) − len(FeatsChosen) − Σ(AbilityIncreases)
```

- `cadenceSlots(level)` = count of cadence levels ≤ current level (level itself derives from
  XP) — **class-independent**.
- Feat stat bonuses ride on `FeatsChosen` (looked up from feat data), **not** counted as
  point-spends — the two tracks stay independent and each is auditable.
- **Selection validation** (class-aware, server-side, at the level-up moment): a feat is
  choosable only at a class-eligible level the character has reached;
  `len(FeatsChosen) ≤ eligible-levels-reached`; prerequisites satisfied.

## Prerequisites

`feats.json` carries prereqs (e.g. CHA 13+, "ability to cast a spell", armor-proficiency
chains). Evaluate them at selection with the **M3 requirement evaluator** — the same one
shared with POIs/encounters/dialogue.

## Slotting (why feats follow M5)

Most feat *effects* depend on systems built later, so the feat *system* can't precede them —
but the *seam* is reserved in M1:

- **M1:** reserve `FeatsChosen` in save schema v2 (empty); ship ability-point allocation
  only. Every cadence level grants a point; no feat UI yet.
- **Feats milestone (after M5):** activate feat selection + effects. Effects need **M4**
  (casting feats: War Caster, Magic Initiate, Elemental Adept) and **M5** (reaction /
  condition / save feats: Alert, Mage Slayer, Lucky, Mobile). Migrate `feats.json` →
  `game-data/systems/feats/*.json` (one per feat, like `systems/abilities/`); add codex
  validation + migration.
- Update the [about.html](../../www/views/tabs/about.html) stat-points copy to mention
  trading points for feats once live.
