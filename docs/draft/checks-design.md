# Checks: hard requirements vs. soft skill checks

Two mechanics that look similar but do opposite jobs. Both are needed, in
different circumstances. This is the reference for the POI node-walker's check
nodes, the quest `check` objective, and discovery.

## The distinction

| | **Hard check** = *Requirement* | **Soft check** = *Skill check* |
|---|---|---|
| Answers | "Can you *attempt* this?" | "Do you *succeed*?" |
| Outcome | Deterministic — met or not | Random — d20 vs DC |
| D&D analogue | Having the tool / proficiency / class to even try | An ability/skill check |
| In code | `game/requirement` evaluator | `game/skillcheck` |
| Examples | have rope, be a Druid, STR ≥ 13, level ≥ 5, 10 QP | spot the trap, climb the cliff, pick the lock |

**Requirements gate the attempt; checks resolve the attempt.** They compose:
*"Forcing the shaft needs a rope (hard gate); with it, roll Athletics DC 15 to
climb without slipping (soft check)."*

## The soft-check formula (D&D, adapted)

D&D: `d20 + ability modifier + proficiency vs DC`, on the DC scale
5/10/15/20/25 (easy→very hard). Our skills are already blended **0–20 scores**
(the ratio weights sum to 1, so a skill sits in the ability-score range), and
everyone has all eight skills — so there's no separate proficiency layer; the
score *is* the competence. We therefore treat a skill score exactly like D&D
treats an ability score:

- **Modifier = `floor((skill − 10) / 2)`** — skill 14 → +2, 18 → +4, 20 → +5.
  Small on purpose, so the d20 stays meaningful and authored DCs (12–20) land on
  the standard scale unchanged.
- **Active check** (player chooses to try): `d20 + modifier vs DC` → success /
  failure branch.
- **Passive check** (ambient awareness, e.g. spotting a hidden POI while passing):
  `10 + modifier vs DC`, no roll.
- **Advantage/disadvantage** (2d20 keep higher/lower) is the clean way to grant
  situational edges later — e.g. a crowbar grants advantage rather than just
  gating.

Implemented in `cmd/server/game/skillcheck` (`Modifier`, `Resolve`, `Passive`).

## Effects feed checks (the seam)

Buffs/debuffs only modify the **six ability scores** (`effects.GetActiveStatModifiers`),
and skills derive from abilities — so an ability buff/fatigue penalty should
ripple into every skill. The skill value a check (or gate) reads **must** be the
*effective* skill:

```
effectiveStats = base stats + active effect modifiers   // effects.EffectiveStats
effective_skill = CalculateSkillValue(effectiveStats, ratio)
soft check       = d20 + (effective_skill − 10)/2  vs  DC
```

`save.Stats` already includes spent ability points (allocation raises the stored
score), so `EffectiveStats` does **not** add `AbilityIncreases` — that would
double-count. A skill-direct effect ("+2 Perception from a spyglass") isn't
supported yet; it'd be a small extension to that switch.

### Wired now
- `skillcheck` package + the effective-skill formula.
- `effects.EffectiveStats` (base + active modifiers).
- The requirement evaluator's skill/stat gate reads effective stats (`questContext`).
- The `/api/skills` display shows effective skills, so the number you see matches
  the number a gate/check uses.
- POI discovery's optional perception path uses a passive effective check.

### Decisions taken
- **Effective everywhere** (soft checks + all skill/stat gates use effective
  stats). Simpler than splitting; a transient buff helping you *qualify* for a
  quest is the rare case (one quest gates on a skill). Revisit if availability
  flipping on a 1-minute potion feels wrong — split would be "availability uses
  base, in-the-moment choice gates use effective."

### TODO (with the node-walker)
- POI `check` nodes (`d20 + (effective_skill−10)/2 vs DC` → success_next/failure_next).
- The quest `check` objective fires `SkillCheckPassed` when a POI check passes.
