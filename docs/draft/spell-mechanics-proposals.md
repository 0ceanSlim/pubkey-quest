# Spell Mechanics Proposals

Spells whose full effect cannot be expressed in the current schema are flagged `[~]` in
the progress log. This file records what engine mechanic each spell needs and a minimal
proposal for implementing it.

---

## Cantrips (level 0)

### mage-hand.json — "Spectral Strike" — forced movement (push)
**Missing mechanic:** On-hit push: if the target fails a Strength save, they are pushed
5 feet away from the caster.
**Proposal:** Add a `secondary_save` block to spell JSON (separate from primary
`save_type`) that triggers only on a hit and applies a named condition/effect. Minimal:
`"on_hit_save": { "type": "strength", "dc_source": "spell", "on_fail": "push_5ft" }`.
The combat engine checks `on_hit_save` after a successful `spell_attack` resolves.

---

### minor-illusion.json — "Combat Illusion" — disadvantage on next attack
**Missing mechanic:** Applying disadvantage to the target's next attack roll as a
one-time triggered condition.
**Proposal:** Add `"disadvantage_next_attack": true` as an `ActiveEffect` modifier type
(duration: 1 attack consumed, not time-based). The effect is cleared when the affected
creature makes its next attack roll. Tag: `debuff_next_attack`.

---

### dancing-lights.json — "Blinding Flash" — blinded condition
**Missing mechanic:** Blinded condition: affected creatures have disadvantage on attack
rolls and attackers have advantage against them.
**Proposal:** Add `"blinded"` as a named combat condition to the effects system with:
`attackers_have_advantage: true`, `self_attacks_at_disadvantage: true`. Duration 1 round
(cleared at end of the affected creature's next turn). The save sets this condition on
fail.

---

### shillelagh.json — spellcasting-ability weapon attacks
**Missing mechanic:** Swap weapon attack stat: use spellcasting modifier (Wisdom for
druids) instead of Strength for melee attack and damage rolls with a specific held weapon.
**Proposal:** Add an `ActiveEffect` modifier type `"attack_stat_override"` with
`{ "weapon_types": ["club", "quarterstaff"], "use_stat": "spellcasting" }`. The combat
engine reads this before computing weapon attack rolls.

---

### shocking-grasp.json — reaction suppression
**Missing mechanic:** Target cannot take reactions until the start of their next turn.
**Proposal:** Add `"no_reactions"` as a named combat condition in the effects system.
Duration: until start of affected creature's next turn. Applied on a successful melee
spell attack hit.

---

### chill-touch.json — cannot regain hit points
**Missing mechanic:** Target cannot regain hit points until start of caster's next turn.
**Proposal:** Add `"no_healing"` as a named combat condition / ActiveEffect modifier.
Duration: 1 round (until start of caster's next turn). Applied on a successful ranged
spell attack hit.

---

### ray-of-frost.json — speed reduction
**Missing mechanic:** Target's movement speed is reduced by 10 feet until the start of
the caster's next turn.
**Proposal:** Add `"speed_reduction"` as an `ActiveEffect` modifier type with a value
field (e.g. `{ "modifier": -10 }`). Applied on hit; duration: until start of caster's
next turn (1 round in combat).

---

### true-strike.json — advantage on next attack
**Missing mechanic:** Caster gains advantage on their first attack roll against the
target on their next turn.
**Proposal:** Add `"advantage_next_attack_vs_target"` as a caster-side combat condition.
Cleared when the caster makes their next attack roll against any target. Could be modeled
as a personal `ActiveEffect` with `"scope": "self"` and the target id stored.

---

### vicious-mockery.json — disadvantage on next attack (same as Combat Illusion)
**Missing mechanic:** Same as Combat Illusion — target has disadvantage on its next
attack roll.
**Proposal:** Same mechanic as Combat Illusion's `debuff_next_attack` condition (see
above). Share implementation.

---

### thorn-whip.json — forced movement (pull)
**Missing mechanic:** On hit, pull Large-or-smaller creature up to 10 feet closer to
the caster.
**Proposal:** Add `"pull_10ft"` as a forced-movement effect, symmetric to `push_5ft`.
Triggered on a successful melee spell attack hit. Size check (Large or smaller) is a
prerequisite the combat engine evaluates.

---

### produce-flame.json — dual-use (light + attack)
**Missing mechanic:** Spell creates a persistent light source that the caster holds;
throwing it is a separate action using a ranged spell attack. The "cast to create flame"
and "action to throw" are two different action costs in 5e.
**Proposal:** Model as: initial cast creates an `ActiveEffect` buff (light aura, 10 min)
on the caster. A second action `"throw_flame"` triggers the ranged spell attack. For now,
the JSON models the attack (primary shape); the light effect lives in `effect` prose.
True dual-action modeling may require a `follow_up_action` field.

---
