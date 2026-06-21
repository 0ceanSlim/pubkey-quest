# M1 Scope — Progression that works + Save schema v2

Grounded against the current code (2026-06-21). Roadmap §3 M1 is the brief; this is the
buildable breakdown. **Done when:** killing things levels you up with visible stat growth;
the save passes the §4 hydration audit; a server crash restores the session; quitting
without saving reverts to the last deliberate save.

## Already in place — build on, don't rebuild

- **Level derives from XP.** `character.GetLevelFromXP` + `WillLevelUp` + `LoadAdvancement`
  (`cmd/server/game/character/progression.go`). `advancement.json` = XP thresholds L1–20 +
  an XP multiplier. **No proficiency field** → proficiency is derived from level in code.
- **Combat already detects level-up.** Fight XP accumulates in `cs.XPEarnedThisFight`;
  `handleMonsterKill` sets `cs.LevelUpPending` when it crosses a boundary
  (`combat.go:911`). `CombatEndResponse` already returns `XPEarned` + `LevelUpPending`;
  the victory/end path commits XP to `save.Experience`.
- **Data is complete:** `spell-slots.json` (per class/level cantrip + slot counts),
  `base-hp.json` (hit die by class), `class-resources.json`
  (Fighter stamina 10 flat · Barbarian rage 100 flat · Monk ki `wisdom_mod + level` ·
  Rogue cunning 10 flat · all casters = mana).

## The finding that reshapes M1: most "gains" are *derived*, not stored

Walking the data, nearly everything the roadmap lists under "apply the gains" is already
derivable and needs **no** stored-state mutation:

- **Class-resource max** is computed at combat start from `class-resources.json` (flat
  value, or Ki = `wisdom_mod + level`). It already tracks level — nothing to persist or
  grow. ✅ (just confirm combat-start reads the *current* level for Ki.)
- **Proficiency bonus** derives from level in `resolveAttackBonus`. ✅ (audit the formula.)
- **Level** derives from XP. ✅
- The only stored, non-derived "max" values today are **`MaxHP` / `MaxMana`**
  (`types.SaveFile`). And §4 explicitly says these should derive too: *"HP/mana current
  stored, max derives."*

So the central M1 design question isn't "how do we grow MaxHP" — it's **whether MaxHP/
MaxMana are stored at all.**

## ⛳ Gating decision D1 — Max HP/Mana: derive, or store-and-grow?

**✅ DECIDED (2026-06-21): Derive.** `MaxHP`/`MaxMana` leave the persisted save and are
computed on load from class+level+CON; implies **D2 = fixed (average) HP gain**. Working
defaults for the rest unless changed: D3 auto-grant new spells, D4 cut `kill_bonus_xp`,
D5 heal to full on level-up.

- **Derive (recommended — §4 program-of-record).** Drop `MaxHP`/`MaxMana` from the
  persisted save. Add `DeriveMaxHP(class, level, conMod)` / `DeriveMaxMana(class, level,
  castingMod)`; a `Hydrate(save)` step on load populates the in-memory values and clamps
  current HP/Mana ≤ max. Leveling raises them automatically because level rises with XP —
  "level-up works" becomes a *consequence* of "level derives from XP," with no apply step
  and nothing derivable written into signed events. Cost: a load-time hydrate + making the
  persisted form omit the two fields (current HP/Mana still stored).
- **Store-and-grow (M1 literal text).** Keep `MaxHP`/`MaxMana` stored; `ApplyLevelUp`
  writes the increases. Less refactor now, but stores derivable data into player-signed
  events forever and needs a real apply-path + idempotency guard.

**Recommendation: Derive.** Implies **D2 = fixed (average) HP gain**, since a deterministic
gain is what makes MaxHP derivable.

## Tasks

### A. Progression (D1 = derive)
- `character/derive.go`:
  - `DeriveMaxHP` = `hitDieMax + conMod + (level-1) * (avgGain + conMod)`, where
    `avgGain = hitDie/2 + 1` (d6→4, d8→5, d10→6, d12→7) from `base-hp.json`.
  - `DeriveMaxMana` — **confirm the creation-time formula in `generation.go`** and mirror
    it (roadmap: casting-mod + level scaling). *(grep didn't surface the assignment — pin
    the exact formula during impl.)*
  - Cantrip + spell-slot counts from `spell-slots.json[class][level]`.
- `Hydrate(save)` in `session/save.go` (`LoadSaveFile`): populate MaxHP/MaxMana/slot counts;
  clamp current HP/Mana.
- `CheckAndApplyLevelUps(save)` — only the **non-derived** parts: open the new empty spell
  slots, grant new cantrips/spells-known (see D3), build the old→new payload. (No MaxHP
  write if derived.) Call it on the combat victory/end path and anywhere XP is granted.
- **Audit proficiency** in `resolveAttackBonus` for L1–5 (expect +2 at 1–4, +3 at 5).
- **Level-up moment** (win95 modal over the scene): "Level 3 — +6 HP, new L2 slot." Driven
  by the payload; make the reward beat feel good even simple.

### B. Save schema v2 (one migration, all at once)
- Add to `types.SaveFile`: `Quests` (completed `[]string` + in-progress tuples),
  `POIState`, `Room string`, `Rentals`, `SchemaVersion int`. Zero-valued now; M2/M3 fill them.
- If D1 = derive: **remove** `MaxHP`/`MaxMana` from the persisted form (current HP/Mana stay).
- Migration shim in `LoadSaveFile`: v1 saves → set `SchemaVersion`, zero new fields, hydrate
  maxes. **Reconcile the two load paths** — `session/save.go` (uses `types.SaveFile`) and
  `api/saves.go` (uses a local `SaveFile` type); the shim must cover the session path.
- **Central event recorder** `game/events/record.go`: `Record(save, kind, target, n)`.
  Call sites: combat kill, item pickup, location discovery, NPC talk, shop transaction,
  sleep. Feeds M3 quest objectives, dailies, and badge/stat tracking — build it once.

### C. Deliberate saves + session resilience (two different things)
- **Saves = explicit player action** (already 409s in combat); no autosave. Win95 save
  modal replacing the browser `confirm()`; "last saved: N min ago" indicator; optional
  decline-able prompts at inn rest / arrival.
- **Session journaling (server concern):** snapshot dirty in-memory sessions to
  `data/sessions/` every few minutes; restore on restart; cleared on clean save/quit.
  Never signed, never leaves the server. This is crash-resilience, *not* a save.

## Smaller decisions
- **D2 — HP gain:** fixed average (required if D1 = derive). Recommend fixed.
- **D3 — new cantrips/spells-known on level-up:** auto-grant vs. player choice. Recommend
  **auto-grant** for alpha (defer the choose-a-spell UI); known-casters (sorcerer/bard) can
  get a real choice later. New *slots* are mechanical; new *known spells* are the only
  "decision" and it's cheap to auto-fill for now.
- **D4 — `kill_bonus_xp`:** the `KillBonusXP` stub returns 0. Recommend **cut for alpha**
  (`XPForDamage` already rewards kills) — or add a `kill_bonus_xp` field to `MonsterData`
  for boss bonuses. Low stakes either way.
- **D5 — heal on level-up?** Recommend **yes** — restore HP/Mana to the new max as the
  reward beat.

## Suggested order
D1 decision → `derive.go` + `Hydrate` (unblocks everything) → schema-v2 fields + migration
shim → `CheckAndApplyLevelUps` + payload + modal → event recorder → save modal / last-saved
→ session journaling.

## Tests (highest value)
- Derive-max table: MaxHP/MaxMana per class for L1–5.
- Save serialize→deserialize round-trip per `SchemaVersion` (v1→v2 shim).
- Level-up crossing: XP that spans a boundary applies exactly once (idempotency).
