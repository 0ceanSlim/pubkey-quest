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

## Level 1 spells

### witch-bolt.json — sustained concentration damage (auto-damage on subsequent turns)
**Missing mechanic:** After the initial ranged spell attack hits, the caster can use their
action on subsequent turns to deal 1d12 lightning damage automatically (no new attack roll
needed) while concentration holds and the target is within range.
**Proposal:** Add a `"concentration_followup"` field to the spell schema:
`{ "action_cost": "action", "damage": "1d12", "damage_type": "lightning", "auto_hit": true }`.
When the caster has this active concentration effect, the combat UI offers a "sustain" 
action that triggers the auto-damage without a new attack roll. Alternatively model via an
ActiveEffect on the target with a `"per_turn_damage"` modifier that the engine applies
while the caster chooses to sustain (and expends an action).

---

### thunderwave.json — push on AoE save (extends mage-hand push proposal)
**Missing mechanic:** AoE push — all creatures in a 15 ft radius that fail a Constitution
save are pushed 10 ft away from the caster.
**Proposal:** The `push_5ft` / `push_10ft` forced-movement mechanic proposed for
mage-hand applies here but must support AoE (every creature that fails the save gets
pushed, not just one). The combat engine should apply the push to all save-fail targets
when the spell has `"on_save_fail": "push_10ft"` in a `secondary_effect` block. Extend
the on_hit_save proposal to also support `on_save_fail` for save-primary spells.

---

### arms-of-hadar.json — reaction suppression on AoE save fail
**Missing mechanic:** All creatures that fail the Strength save also can't take reactions
until their next turn. This is the same `no_reactions` condition as shocking-grasp but
applied to every save-fail target in the AoE.
**Proposal:** Re-use the `no_reactions` condition proposed for shocking-grasp. For
AoE spells, apply the condition to all save-fail targets. Add `"on_save_fail_condition":
"no_reactions"` to the secondary_effect block, parallel to the push proposal above.

---

### armor-of-agathys.json — retaliatory damage on melee hit
**Missing mechanic:** When a melee attacker hits you while Armor of Agathys temp HP are
active, the attacker automatically takes 5 cold damage (no save, no attack roll).
**Proposal:** Add a `"retaliate_on_melee_hit"` ActiveEffect trigger: while the effect is
active, any melee attack that hits the caster causes the attacker to take `damage` cold
damage. The combat engine checks for this after resolving the incoming attack. Field:
`"retaliate": { "trigger": "melee_hit", "damage": "5", "damage_type": "cold" }` inside
the effect block. Conditional on temp HP remaining.

---

### charm-person.json — charm condition
**Missing mechanic:** The charmed condition: target treats the caster as a friendly
acquaintance, cannot attack the caster, and the caster has advantage on social checks
vs. the target.
**Proposal:** Add `"charmed"` as a named condition in the effects system with:
`cannot_attack_charmer: true`, `charmer_social_advantage: true`. Duration from the spell's
`duration` field. The condition breaks immediately if the caster or allies harm the target.
`on_harm_break: true` flag in the condition definition. Primarily a social/exploration
mechanic — in combat, the target simply cannot attack the caster.

---

### command.json — specific command action variants
**Missing mechanic:** The specific word commanded (Approach, Drop, Flee, Grovel, Halt)
produces different behaviors on the target's next turn — these are distinct forced actions,
not a generic debuff.
**Proposal:** Model Command as a `"forced_action"` condition with a `"command_type"` field
(flee / grovel / halt / approach / drop). Each type maps to a specific behavior the combat
engine executes on the target's next turn (e.g. flee = move away at max speed; halt = no
movement; grovel = fall prone). The caster selects command_type at cast time. For now,
the JSON models the save shape; the specific command_type is in `effect` prose.

---

## Level 1 spells (batch 4 additions)

### sleep.json — "Exhausting Hex" — exhaustion condition
**Missing mechanic:** Exhaustion levels (D&D 5e exhaustion is a stacking debuff with 6
tiers: disadvantage on checks, halved speed, disadvantage attacks/saves, halved max HP,
speed 0, death). This spell applies 1 exhaustion level and -10 ft movement for 1 hour.
**Proposal:** Add `"exhaustion"` as a named condition in the effects system, supporting
`level` (1–6) and a duration. Each level carries penalties: level 1 = disadvantage on
ability checks + -10 ft speed. Add an `ActiveEffect` modifier type `"exhaustion_level"`
with integer value. Duration from the spell's `duration` field. At level 6 the creature
is incapacitated. For this spell: `{ "condition": "exhaustion", "level": 1,
"speed_penalty": -10, "duration_minutes": 60 }`. Initial implementation can handle
level 1 only (the most common case).

---

### color-spray.json — HP-threshold blind (AoE)
**Missing mechanic:** Color Spray affects creatures based on HP totals rather than a save
or attack roll — roll 6d10, blind creatures in order of proximity (lowest HP first)
until the pool is exhausted. This is a unique "HP-threshold" targeting mechanic.
**Proposal:** Add a `"hp_threshold_aoe"` targeting mode to the combat engine for spells
with no `spell_attack` and no `save_type`: roll the specified dice pool (`"threshold_dice":
"6d10"`), then auto-apply the `"blinded"` condition to eligible creatures (nearest first)
while their HP <= remaining pool, decrementing the pool each time. Uses the `blinded`
condition already proposed for dancing-lights. The cone shape (15 ft from self) maps to
range 0 with an implicit 3-cell cone arc.

---

### silent-image.json — investigation-check interaction
**Missing mechanic:** Creatures that succeed on an INT Investigation check (active
perception while interacting with the illusion) see through Silent Image. This is a
non-combat mechanic — creatures get a check, not a save during casting.
**Proposal:** Add an `"investigation_dc"` field to illusion-school spells (DC = caster's
spell save DC). When a creature interacts with the illusion in the engine's exploration
context, it rolls an INT Investigation check vs. this DC. On success the creature is
flagged as "seen through illusion" and the effect is suppressed for them. In combat,
spending an action to investigate triggers the same check. For JSON: `"investigation_dc":
"spell_dc"` (calculated from caster's INT).

---

### speak-with-animals.json — beast command action
**Missing mechanic:** The homebrew version lets the caster spend an action to command a
nearby beast NPC to make one attack. This requires the engine to recognize beast-type
NPCs as commandable allies while this spell effect is active.
**Proposal:** Add an `ActiveEffect` type `"beast_ally"` — while active, all beast-type
NPCs within 2 grid cells of the caster are flagged as temporarily allied and the caster
has a `"command_beast"` action in combat. The commanded beast uses its standard attack
action (existing NPC attack resolution). The effect's `duration` from the spell. Beast
NPCs with Intelligence 4+ ignore the command.

---

### goodberry.json — spell-created consumable items
**Missing mechanic:** Goodberry creates 5 temporary consumable item instances (the
berries) that exist as real inventory items for up to 1 hour, each usable as a bonus
action for healing.
**Proposal:** Add a `"creates_items"` block to the spell schema: `{ "item_id":
"goodberry", "quantity": 5, "duration_minutes": 60 }`. The engine creates temporary
inventory entries for these items on cast, flagged with an expiry timestamp (1 hour).
The item `goodberry` would be a new `game-data/items/goodberry.json` consumable with
`action_cost: "bonus_action"` and `heal: "1d4+1"`. Items past expiry are auto-removed.
This reuses the existing inventory stack system with a time-to-live flag.

---

## Level 1 spells (batch 5 additions)

### searing-smite.json — on-hit rider + burning DoT condition
**Missing mechanic:** (1) On-hit rider: extra damage triggers on the caster's NEXT weapon
hit, not on the spell cast. (2) Secondary CON save on hit: failure starts a burning DoT
(1d6 fire at start of each of the target's turns until extinguished).
**Proposal:** Smites and similar bonus-action "next-hit" buffs need an `"on_next_hit"`
buff type in the ActiveEffect system: `{ "trigger": "next_melee_hit", "damage": "1d6",
"damage_type": "fire", "secondary_save": { "type": "constitution", "on_fail":
"burning_1d6_per_turn" } }`. The "burning" condition is a new DoT condition consumed
each turn at the start of the target's turn. An action can extinguish it. This mechanic
is shared across all smite spells — implement once, reuse.

---

### thunderous-smite.json — on-hit rider + push + prone (secondary STR save)
**Missing mechanic:** Same on-hit-rider mechanic as searing-smite. Secondary STR save
forces push 10 ft + knocked prone on fail.
**Proposal:** Extend `"on_next_hit"` buff mechanic (see searing-smite) with `"secondary_save":
{ "type": "strength", "on_fail": ["push_10ft", "prone"] }`. The push/prone conditions
reuse proposals from mage-hand/thunderwave. Knocked prone = speed 0 until creature uses
half movement to stand (requires a `prone` condition in the combat engine).

---

### wrathful-smite.json — on-hit rider + frightened condition (secondary WIS save)
**Missing mechanic:** On-hit-rider damage (same as above). Secondary WIS save: failure
applies `frightened` condition — disadvantage on attacks/checks while the source of fear
is visible, cannot willingly move closer. Repeatable save at end of each turn.
**Proposal:** Add `"frightened"` as a named condition: `cannot_approach_source: true`,
`attacks_at_disadvantage: true` while source is visible. `repeated_save: { "end_of_turn":
"wisdom" }` — condition breaks on successful save. Reuses `on_next_hit` mechanic.

---

### hex.json — ability-check penalty (chosen at cast)
**Missing mechanic:** Hex lets the caster choose one ability (STR/DEX/CON/INT/WIS/CHA)
at cast time; the target has disadvantage on all checks with that ability for the duration.
**Proposal:** Add a `"chosen_ability_debuff"` field to the ActiveEffect for Hex: `{
"ability": "<player_choice>", "effect": "disadvantage_on_checks" }`. The engine prompts
the caster for an ability choice when the spell is cast. The ActiveEffect stores the chosen
ability and applies disadvantage-on-ability-checks against that ability throughout combat
and exploration for the duration.

---

### hunters-mark.json — bonus-action target transfer on kill
**Missing mechanic:** When the marked target drops to 0 HP, the caster can use a bonus
action to move the mark to a new target (same spell duration, no new cast).
**Proposal:** Add a `"transfer_on_kill"` flag to the ActiveEffect:
`{ "action_cost": "bonus_action", "trigger": "marked_target_drops_to_zero" }`. The
combat engine checks this at kill resolution and offers the caster a bonus-action
"transfer mark" option if they have remaining concentration.

---

### guiding-bolt.json — advantage on next attack vs. target
**Missing mechanic:** On a hit, the next attack roll made by any creature against the
target before the end of the caster's next turn has advantage.
**Proposal:** Apply a `"lit"` condition (or `"outlined"`) to the target for 1 round:
`attackers_have_advantage: true`. This is the same condition proposed for faerie-fire
below — share implementation. Cleared at end of caster's next turn, or after the first
attack roll is made against the target (whichever comes first).

---

### hellish-rebuke.json — reaction trigger (cast when taking damage)
**Missing mechanic:** Hellish Rebuke is cast AS a reaction to taking damage — the trigger
is "when you take damage from a creature within range". This requires the combat engine
to offer reactive spell casting as a response to incoming damage.
**Proposal:** Add a `"reaction_trigger"` field to spell JSON: `{ "event": "take_damage",
"source_range": 2 }`. During the attack resolution loop, if the caster has this spell
prepped and takes damage from a creature within range, the engine offers a reaction prompt
(spend mana + reaction to cast). On confirmation, the DEX save resolves against the source.
This mechanic also covers Shield (reaction to being hit) and similar reactions.

---

### ensnaring-strike.json — on-hit rider + restrained condition
**Missing mechanic:** On-hit-rider (same as smites); STR save on fail = `restrained`
condition with DoT piercing damage each turn while restrained. Restrained: speed 0,
disadvantage on DEX saves, attackers have advantage. Repeatable STR action to break free.
**Proposal:** `"restrained"` condition: `speed: 0`, `dex_saves_at_disadvantage: true`,
`attackers_have_advantage: true`. Add `"escape_action": { "check": "strength", "dc":
"spell_dc" }` to the condition. The DoT while restrained uses an `"on_condition_tick":
{ "condition": "restrained", "damage": "1d6", "damage_type": "piercing" }` per-turn
trigger. Reuses `on_next_hit` mechanic from smites.

---

### faerie-fire.json — lit/outlined condition (AoE)
**Missing mechanic:** Creatures that fail the DEX save are outlined in colored light:
attack rolls against them have advantage, they can't benefit from invisibility. This is
an AoE save (cube) applying a persistent per-creature condition.
**Proposal:** Add `"outlined"` (or `"lit"`) as a named condition: `attackers_have_advantage:
true`, `invisible_suppressed: true`. Applied per-creature that fails the DEX save in the
AoE. Duration from spell's duration field (1 min conc). This condition also serves
guiding-bolt's "next attacker advantage" — share the same condition with duration variant
(round-limited vs. full duration). AoE cube targeting extends the AoE zone proposal from
thunderwave/entangle.

---

### compelled-duel.json — attack-roll debuff vs. non-caster + movement restriction
**Missing mechanic:** (1) Target has disadvantage on attacks against any creature other
than the caster. (2) Each time target tries to move more than 30 ft from caster, must make
a WIS save; on fail, movement toward-away is blocked for that turn.
**Proposal:** Add a `"compelled"` condition: `attacks_against_non_caster_at_disadvantage:
true` + a per-movement-check `{ "trigger": "move_away_from_caster", "distance_threshold":
6, "save": { "type": "wisdom", "on_fail": "cancel_movement" } }`. Movement check requires
the engine to evaluate per-step movement costs and conditional interrupts.

---

### expeditious-retreat.json + longstrider.json — speed modifier effect
**Missing mechanic:** Both spells modify movement speed. Expeditious Retreat lets the
caster Dash (double speed) as a bonus action each turn. Longstrider adds a flat +10 ft.
**Proposal:** Add `"speed_bonus"` as an ActiveEffect modifier type with a flat value
(`+10 ft`) for Longstrider, already partially proposed under ray-of-frost's speed_reduction.
For Expeditious Retreat, add `"bonus_action_dash": true` as an ActiveEffect flag — the
combat engine grants a bonus-action Dash option to the caster each turn while active.
Both use the effects system with duration from the spell's duration field (timer in minutes).

---

## Level 1 spells (batch 6 — final L1 additions)

### fog-cloud.json — persistent AoE obscured zone
**Missing mechanic:** A persistent spherical terrain zone that heavily obscures all creatures
inside it (attacks from and into the zone have disadvantage; creatures inside can't see out
and vice versa).
**Proposal:** Extend the AoE zone mechanic proposed for thunderwave/entangle/grease (see
Level 1 below) with an `"obscured"` terrain tag. Any cell marked `"obscured"` causes the
combat engine to grant disadvantage on attack rolls that pass through or originate from it.
Add a `"zone_shape"` field: `{ "type": "sphere", "radius_cells": 4, "terrain": "obscured" }`.
Concentration dropping removes all zone cells. Reuses the persistent-zone engine component.

---

### grease.json — persistent terrain zone with per-entry save (prone)
**Missing mechanic:** A persistent ground zone (10 ft square) that applies a DEX save to
any creature entering or ending its turn in it; failure = prone condition. Distinct from
fog-cloud's obscured zone — this is a save-and-condition zone, not a line-of-sight blocker.
**Proposal:** Extend the AoE zone proposal with a `"zone_effect"` block: `{ "trigger":
["on_enter", "end_of_turn"], "save": { "type": "dexterity", "dc": "spell_dc" },
"on_fail": "prone" }`. The zone persists for the spell's full duration (no concentration).
Prone condition: speed 0 until creature uses half movement to stand; melee attacks against
prone have advantage; attacks by prone creature have disadvantage.

---

### jump.json — jump-distance tripling
**Missing mechanic:** Triples a creature's jump distance (horizontal and vertical). In D&D
5e, jump distance is derived from Strength score. No equivalent movement modifier exists
in this engine.
**Proposal:** Add a `"jump_multiplier"` ActiveEffect modifier with value `3`. Interacts
with the movement/travel system's terrain-gap traversal: if a grid cell is flagged as a
gap or chasm, the engine checks whether the player's effective jump distance clears it.
For now, modeled as a utility effect prose only; engine support deferred until the
traversal system is built.

---

### sanctuary.json — attacker-must-save ward
**Missing mechanic:** Any creature that would target the warded creature with an attack or
harmful spell must first make a Wisdom save; on failure, it must choose a new target.
This requires intercepting the attacker's targeting decision before the attack resolves.
**Proposal:** Add a `"ward"` effect type to ActiveEffects: `{ "type": "ward",
"intercept": "before_attack", "save": { "type": "wisdom", "dc": "caster_spell_dc" },
"on_fail": "retarget" }`. The combat engine checks for active ward effects on the target
before resolving incoming attacks. `retarget` forces the attacker AI (or blocks player
attack) and requires a new target selection. Ward breaks if the warded creature deals
damage to another creature (tracked as a `"break_on": "warded_creature_attacks"` flag).

---

### unseen-servant.json — persistent summoned task entity
**Missing mechanic:** A persistent intangible force entity that obeys simple commands
(fetch, carry, pour, open, mend). Not a combat entity — AC 10, 1 HP, no attacks.
**Proposal:** Add a lightweight `"task_entity"` summon type to the engine (distinct from
the full NPC/familiar summon). Task entities have a list of allowed actions (fetch, carry,
open, etc.) resolved by the engine's object-interaction system. They do not participate
in combat rounds — they act on their own initiative slot between the caster's turns but
cannot attack. If they reach 0 HP the spell ends. Persistent for duration; re-summon resets.

---

### find-familiar.json — persistent familiar summon
**Missing mechanic:** A spirit companion in animal form that persists indefinitely (not a
time-limited summon). The familiar acts on its own initiative, can relay sensory info
telepathically, and can take the Help action in combat to grant advantage on the caster's
attacks. It cannot attack on its own, but some forms have special utility abilities.
**Proposal:** Extend the NPC/entity system with a `"familiar"` entity type: persists in
the caster's save data (not just session-local), has its own stats derived from the chosen
animal form, and participates in combat as a friendly non-attacking entity. The Help-action
mechanic grants `advantage_next_attack` to the caster when the familiar is adjacent. On
dismissal, the familiar's spirit is stored (not deleted) and re-summoned without re-casting.
Full implementation deferred to M4 or M5 (requires save-file entity persistence).

---

## Level 2 spells (batch 6)

### scorching-ray.json — multi-ray attack (3 independent rolls per cast)
**Missing mechanic:** Three separate ranged spell attack rolls per cast, each independently
hitting or missing, each dealing 2d6 fire on hit. Rays can be split between targets.
**Proposal:** Add a `"multi_attack"` block to spell JSON: `{ "count": 3, "attack_type":
"ranged", "damage": "2d6", "damage_type": "fire", "split_targets": true }`. The combat
engine runs `count` independent attack rolls when this block is present, resolving each
against the target's AC. With `split_targets: true`, the player UI allows assigning each
ray to a different target before rolls. Damage field shows per-attack dice; total is
reported per-roll. Shares the multi-attack resolution path with any future multi-projectile spells.

---

### spiritual-weapon.json — persistent summoned weapon (bonus-action attacker each turn)
**Missing mechanic:** A persistent floating weapon entity that the caster moves (up to
20 ft) and attacks with via a bonus action on each of the caster's turns for the spell's
duration. No concentration. It is not a creature — just a persistent magic effect that
generates melee spell attacks.
**Proposal:** Add a `"bonus_action_repeating_attack"` ActiveEffect: on each turn where the
caster uses their bonus action with this effect active, the engine resolves a melee spell
attack from the weapon's current position. The weapon has a position in the grid (starts
adjacent to target at cast, moves up to 4 cells per bonus action). The effect expires
after 1 minute (10 combat rounds) regardless of concentration. For positioning, the
weapon occupies a virtual grid cell tracked in the active effect's state. No AC/HP for
the weapon itself — it cannot be targeted or destroyed by enemies.

---

## Level 3 spells (batch 6)

### fireball.json — AoE-all-targets-in-radius (DEX save, half on success)
**Missing mechanic:** Fireball hits ALL creatures and objects in a 20-foot-radius sphere
(approximately 4 grid cells). Every creature in the radius makes a DEX save independently:
fail = full 8d6 fire, success = half (4d6 fire). The caster can also harm allies.
**Proposal:** The combat engine needs a `"radius_save_aoe"` resolution mode: (1) identify
all creatures within N cells of the target point, (2) for each creature roll a DEX save
vs. the caster's spell DC, (3) apply full damage on fail, half on success. This differs
from single-target saves and from the zone mechanic — it's an instantaneous multi-target
resolution not a persistent zone. Add `"aoe": { "type": "sphere", "radius_cells": 4,
"save_type": "dexterity", "half_on_success": true }` to the spell schema. The caster
must select a center point (not a creature) within range. Friendly fire is intentional
and documented. This is the primary engine gap for classic blast spells; implement once,
reuse for comparable spells (Ice Storm, Lightning Bolt, etc.).
