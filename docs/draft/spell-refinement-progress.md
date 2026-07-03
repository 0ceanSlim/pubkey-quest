# Spell Refinement Progress

Batch-by-batch checklist. Status: `[ ]` todo / `[x]` done / `[~]` needs-mechanic (see
`docs/draft/spell-mechanics-proposals.md`).

Total: 84 spells — 26 cantrips, 55 level-1, 2 level-2, 1 level-3.
(Several files use homebrew/renamed spells — noted inline.)

---

## Level 0 — Cantrips (26 spells) — Batch 1 complete

- [x] `acid-splash` — Acid Splash: range_long fixed to match range (4); added heal/effect null; empty material_component block; classes +artificer
- [x] `blade-ward` — Blade Ward: added `effect` prose; added `wizard` to classes; added heal null; empty material block
- [~] `chill-touch` — Chill Touch: added description of no-healing secondary; classes correct; added heal/effect null; **no-healing condition** in proposals
- [x] `dancing-lights` — **Blinding Flash** (homebrew): mana 1→2 (AoE); range fixed to 1; concentration false (1 round, no conc in 5e); fixed stale notes; added effect prose; `[~]` blinded condition also in proposals but shape is complete
- [x] `druidcraft` — Druidcraft: reverted to true utility description (removed invalid combat save_type); effect prose added; mana 1 ok
- [x] `eldritch-blast` — Eldritch Blast: mana 1→2 (strongest damage cantrip); removed redundant "warlock" tag; added heal/effect null; empty material block
- [x] `fire-bolt` — Fire Bolt: mana 1→2 (1d10 fire, equal to EB); classes +artificer; added heal/effect null; improved notes; range_long 6 kept
- [x] `guidance` — Guidance: description corrected (ability checks not attack rolls); added `effect` prose; improved notes; empty material block
- [x] `light` — **Revealing Light** (homebrew): fixed stale notes (removed "Touch range" misnote); added effect prose; added heal null; empty material block
- [~] `mage-hand` — **Spectral Strike** (homebrew): **FIXED** — had both `spell_attack` and `save_type` (schema violation); now spell_attack only; range fixed to 4; **push mechanic** in proposals
- [x] `mending` — **Repair Armor** (homebrew): fixed `action_cost` from "1 minute" to "action"; added effect prose; notes clarify out-of-combat nature; classes +artificer
- [~] `minor-illusion` — **Combat Illusion** (homebrew): fixed stale notes; added effect prose; range fixed to 4; **disadvantage-next-attack** in proposals
- [x] `poison-spray` — Poison Spray: range 2→1 (D&D 5e = 10 ft); classes +wizard+artificer; improved notes; empty material block
- [x] `prestidigitation` — **Arcane Weapon** (homebrew): fixed shape — removed `damage` field (spell is a buff not an attack), added `effect` prose; concentration true (needed for balance); mana 1→2; classes +artificer
- [~] `produce-flame` — Produce Flame: added effect prose for dual-use; range/range_long set (0 self, long 2 for throw); **dual-action** in proposals
- [~] `ray-of-frost` — Ray of Frost: range 2→4 (D&D 5e 60 ft); classes +artificer; **speed-reduction** in proposals
- [x] `resistance` — Resistance: added `effect` prose; empty material block; correct already
- [x] `sacred-flame` — Sacred Flame: range 2→4 (D&D 5e 60 ft); added cover note; added heal/effect null; empty material block
- [~] `shillelagh` — Shillelagh: `action_cost` "bonus action"→"bonus_action" (fixed enum); added effect prose; added material pollen (sprig-of-mistletoe focus); concentration true kept; **spellcast-ability-override** in proposals
- [~] `shocking-grasp` — Shocking Grasp: classes +artificer; added heal null; improved notes; **reaction-suppression** in proposals
- [x] `spare-the-dying` — Spare the Dying: added `effect` prose; removed "combat" tag (this is a stabilization spell); empty material block
- [x] `thaumaturgy` — Thaumaturgy: full description added; effect prose added; range fixed to 2 (30 ft); empty material block
- [~] `thorn-whip` — Thorn Whip: range 6→2 (D&D 5e = 30 ft); classes +artificer; added description of pull; **pull mechanic** in proposals
- [~] `true-strike` — True Strike: range fixed to 4 (D&D 5e = 30 ft); added effect prose; **advantage-next-attack** in proposals
- [~] `vicious-mockery` — Vicious Mockery: range 2→4 (D&D 5e 60 ft); mana 1→2 (double-effect); added effect prose; **disadvantage-next-attack** in proposals
- [x] `word-of-radiance` — Word of Radiance: mana 1→2 (AoE); range confirmed 1 (5 ft); removed explicit null material_component (replaced with block)

---

## Level 1 — 55 spells — Batch 2 complete (12 refined); Batch 3 complete (10 done); Batch 4 complete (10 done); Batch 5 complete (13 done)

### Batch 2 — refined + prep_time backfilled
- [ ] `alarm` — Alarm (ritual)
- [x] `animal-friendship` — Animal Friendship: range "6"→2 (30ft); added bard to classes; pollen×1 (sprig-of-mistletoe=free for druids/rangers); added heal/effect; prep_time 50
- [x] `armor-of-agathys` — Armor of Agathys: mana 2→3; free; range "self"→0; retaliatory-damage in proposals; **prep_time 90** (strong 1-hr no-conc self-buff)
- [~] `arms-of-hadar` — Arms of Hadar: mana 2→3; spirit-dust×1; range "self"→1; reaction-suppression-AoE in proposals; **prep_time 75** (AoE + rune cost → not triple-taxed)
- [x] `bane` — Bane: mana 2→3; spirit-dust×1; range 1→2; effect prose; **prep_time 105** (3-target conc hard debuff, complex coordination)
- [x] `bless` — Bless: mana 2→3; blessed-incense×1 (amulet=free); range_long fixed; **prep_time 90** (3-target conc buff; focus makes component free for clerics/paladins)
- [x] `burning-hands` — Burning Hands: mana 2→3; ash×1; +artificer; **prep_time 45** (fast AoE, instinctive fire; cheapest rune)
- [~] `charm-person` — Charm Person: mana 2→3; free; range 6→2; no conc; charm-condition in proposals; **prep_time 75** (hard control, 1-hour duration)
- [~] `color-spray` — Color Spray: mana 2→3 (AoE control); range "3"→0 (self-cone); no save/attack (HP-threshold mechanic — engine gap); removed null material→proper block; added heal/effect; prep_time 60; HP-threshold-blind proposal in proposals
- [~] `command` — Command: mana 2; free; range 12→4; command-variants in proposals; **prep_time 30** (1-round only, simplest control spell)
- [~] `compelled-duel` — Compelled Duel: range "6"→1 (30ft→1grid for paladin melee theme); null material→proper block; heal/effect; action_cost "bonus action"→"bonus_action"; free; **prep_time 50**; compelled condition + movement-restriction in proposals
- [x] `comprehend-languages` — Comprehend Languages: range "self"→0; null material→proper block; added sorcerer to classes; added heal/effect; added ritual tag; prep_time 30; mana 2 kept
- [ ] `create-water` — Create or Destroy Water
- [x] `cure-wounds` — → see Batch 3 section
- [x] `detect-evil` — → see Batch 3 section
- [x] `detect-magic` — → see Batch 3 section
- [x] `detect-poison` — → see Batch 3 section
- [x] `disguise-self` — Disguise Self: range "self"→0; null material→proper block; added artificer to classes; added heal/effect; prep_time 45; mana 2 kept
- [x] `divine-favor` — → see Batch 3 section
- [~] `ensnaring-strike` — Ensnaring Strike: mana 2→3 (hard control + DoT); spider-silk×1 (binding, always-consumed; stripped tree-sap=yew-wand free); range "self"→0; action_cost "bonus action"→"bonus_action"; save_type removed (on-hit buff shape); effect prose; **prep_time 70**; restrained condition + on-hit-rider in proposals
- [~] `entangle` — Entangle: mana 2→3 (AoE hard control); tree-sap×1 (yew-wand=free for druids; reduced from ×2); range "18"→2; heal/effect; **prep_time 80**; AoE persistent zone + restrained in proposals
- [~] `expeditious-retreat` — Expeditious Retreat: null material→proper block; +artificer class; action_cost "bonus action"→"bonus_action"; heal/effect; **prep_time 30**; bonus-action dash speed modifier in proposals
- [~] `faerie-fire` — Faerie Fire: mana 2→3 (AoE strong debuff); pollen×2 stripped (wrong domain + stub note "no focus provides" was false — sprig provides pollen); range "12"→2; heal/effect; **prep_time 65**; outlined/lit condition (AoE) in proposals
- [x] `false-life` — False Life: range "self"→0; heal "1d4+4"; effect prose; free; **prep_time 40** (minor necromantic utility, quick)
- [ ] `feather-fall` — Feather Fall
- [ ] `find-familiar` — Find Familiar (ritual)
- [ ] `fog-cloud` — Fog Cloud
- [~] `goodberry` — Goodberry: damage field→heal field (1d4+1); removed tree-sap (double-tax); kept pollen×1 (sprig-of-mistletoe=free); effect prose; prep_time 65; mana 2 kept; spell-created-item mechanic in proposals
- [ ] `grease` — Grease
- [~] `guiding-bolt` — Guiding Bolt: mana 2→3 (4d6 radiant = highest L1 damage); stripped blessed-incense×2+holy-water (over-costed double components); blessed-incense×1 (amulet=free for clerics); range "24"→4 (60ft); **prep_time 55**; on-hit advantage-next-attack (lit condition) in proposals
- [x] `healing-word` — → see Batch 3 section
- [~] `hellish-rebuke` — Hellish Rebuke: sulfur×2+arcane-powder stripped (both focus-provided, not a real warlock cost); ash×1 (fire domain, 15gp, no focus, always-consumed); range "12"→2; mana 2 (reaction fire is already limited); **prep_time 45**; reaction-trigger engine mechanic in proposals
- [x] `heroism` — → see Batch 3 section
- [~] `hex` — Hex: stripped ether-essence×2+mana-crystals (both focus-provided, not a real warlock cost); spirit-dust×1 (necrotic/curse domain, always-consumed); range "18"→2; action_cost "bonus action"→"bonus_action"; heal/effect; **prep_time 55**; ability-check debuff (chosen at cast) in proposals
- [~] `hunters-mark` — Hunter's Mark: stripped bone-dust×2+pollen (both focus-provided=free for rangers, not a real cost); free; range "18"→2; action_cost "bonus action"→"bonus_action"; heal/effect; +tracking/buff tags; **prep_time 50**; bonus-action target-transfer on kill in proposals
- [x] `identify` — **Analyze Weakness** (homebrew): removed quartz-dust×2+ether-essence×1 (over-costed utility); free; added artificer to classes; added divination tag; action_cost "1 minute"→"action" (out-of-combat noted); added heal/effect; prep_time 40; mana 2 kept
- [x] `inflict-wounds` — Inflict Wounds: mana 2→3; spirit-dust×1; necrotic 3d10; **prep_time 60** (standard melee attack; rune cost makes it not triple-taxed)
- [ ] `jump` — Jump
- [~] `longstrider` — Longstrider: null material→proper block; +artificer class; heal/effect; **prep_time 35**; speed_bonus modifier effect in proposals (shared with expeditious-retreat)
- [x] `mage-armor` — → see Batch 3 section
- [x] `magic-missile` — Magic Missile: mana 2; free; +artificer; automatic hit; **prep_time 35** (fast classic evocation, no component)
- [x] `protection-from-evil` — → see Batch 3 section
- [x] `purify-food` — Purify Food and Drink: null material→proper block; added druid to classes; added ritual tag; added heal/effect; prep_time 25; mana 2 kept
- [ ] `sanctuary` — Sanctuary
- [~] `searing-smite` — Searing Smite: STRIPPED phoenix-feather×1 (5000gp LEGENDARY on a L1 paladin smite!); free; range "self"→0; action_cost "bonus action"→"bonus_action"; save_type removed (on-hit buff shape); heal/effect; **prep_time 40**; on-hit-rider + burning DoT condition in proposals
- [x] `shield` — → see Batch 3 section
- [x] `shield-of-faith` — → see Batch 3 section
- [~] `silent-image` — Silent Image: range "12"→4 (60ft); removed save_type "investigation" (not a save); null material→proper block; added heal/effect; added concentration tag; prep_time 55; mana 2 kept; investigation-check mechanic in proposals
- [~] `sleep` — **Exhausting Hex** (homebrew): mana 2→3 (1hr no-conc hard control); range "12"→4 (60ft); removed ether-essence×2+mana-crystals×1 (focus-provided, not a real cost); added spirit-dust×1 (hex/curse domain, always-consumed); heal/effect null; prep_time 80; exhaustion-condition in proposals
- [~] `speak-with-animals` — Speak with Animals: range "self"→0; removed pollen×1 (wrong focus note + not substance-themed for ritual utility); null material→proper block; added ritual tag; added effect prose; prep_time 55; mana 2 kept; beast-command mechanic in proposals
- [~] `thunderous-smite` — Thunderous Smite: mana 2→3 (2d6 + push+prone hard control); elemental-sparks×2 stripped (150gp×2 over-tier; no thunder rune exists); free; range "self"→0; action_cost "bonus action"→"bonus_action"; save_type removed (on-hit buff shape); heal/effect; **prep_time 45**; on-hit-rider + prone + push in proposals
- [~] `thunderwave` — Thunderwave: mana 2→3; free; AoE-push in proposals; **prep_time 75** (AoE + push, powerful; no rune cost)
- [ ] `unseen-servant` — Unseen Servant (ritual)
- [~] `witch-bolt` — Witch Bolt: mana 2; iron-filings×1; range 6→2; sustained-conc-damage in proposals; **prep_time 55** (conc lightning; rune cost modest)
- [~] `wrathful-smite` — Wrathful Smite: spirit-dust stripped (totem→bone-dust, not spirit-dust — wrong focus note; fear/psychic not substance-themed at L1); free; range "self"→0; action_cost "bonus action"→"bonus_action"; save_type removed (on-hit buff shape); heal/effect; **prep_time 35**; on-hit-rider + frightened condition in proposals

### Batch 3 — abjuration + healing + divination group
- [x] `cure-wounds` — Cure Wounds: removed stub's 2-component cost (bark-shavings×2+pollen — not substance-themed); free; added heal null fix (1d8+3), effect null; prep_time 50; mana 2 kept
- [x] `detect-evil` — Detect Evil and Good: range "self"→0; fixed null material→proper block; added heal/effect; added cleric to classes; prep_time 40; mana 2; free
- [x] `detect-magic` — Detect Magic: range "self"→0; removed quartz-dust×2 (not substance-themed for basic divination); free; +artificer; prep_time 35; mana 2
- [x] `detect-poison` — Detect Poison and Disease: range "self"→0; concentration fixed false→true (5e correctness); free; added heal/effect; prep_time 35; mana 2
- [x] `divine-favor` — Divine Favor: removed starlight-essence×1 (LEGENDARY 10000gp on L1 paladin buff!); free; fixed action_cost/casting_time "bonus action"→"bonus_action"; range "self"→0; +concentration tag; added heal/effect; prep_time 45; mana 2
- [x] `healing-word` — Healing Word: removed sacred-oil×1+holy-water×1 (150+125gp over-costed on bonus-action heal); free; added effect null; prep_time 30; mana 2
- [x] `heroism` — Heroism: added heal/effect/damage nulls; effect prose; +bard class (5e); +concentration tag; free; prep_time 60; mana 2
- [x] `mage-armor` — Mage Armor: mana 2→3 (8-hour no-conc all-day buff); mana-crystals×3→×1 (staff=free; reduced from triple-stack); removed "resistance" homebrew effect (reverted to D&D standard); added heal/effect; prep_time 120 (longest L1 — all-day buff)
- [x] `protection-from-evil` — Protection from Evil and Good: added salt×1 (protection domain, 10gp, always-consumed); null material→proper block; added heal/effect; added wizard+warlock; prep_time 65; mana 2
- [x] `shield` — Shield: range "self"→0; duration "1 turn"→"1 round"; removed mana-crystals (keep — reaction arcane, staff free); added damage null; refined effect prose; prep_time 45; mana 2
- [x] `shield-of-faith` — Shield of Faith: range 12→4 (60ft = 4 grid); action_cost/casting_time "bonus action"→"bonus_action"; added heal/damage null; effect prose; free; prep_time 50; mana 2

---

## Level 2 — 2 spells — TODO

- [ ] `scorching-ray` — Scorching Ray
- [ ] `spiritual-weapon` — Spiritual Weapon

---

## Level 3 — 1 spell — TODO

- [ ] `fireball` — Fireball

---

## Summary

| Level | Total | Done ([x]) | Needs-mechanic ([~]) | Refined total | TODO |
|-------|-------|------------|----------------------|---------------|------|
| 0     | 26    | 15         | 11                   | 26            | 0    |
| 1     | 55    | ~20        | ~26                  | 46            | 9    |
| 2     | 2     | 0          | 0                    | 0             | 2    |
| 3     | 1     | 0          | 0                    | 0             | 1    |
| **Total** | **84** | **~35** | **~37**          | **72**        | **12** |

Note: "Done" = `[x]` (fully expressible in current schema), "Needs-mechanic" = `[~]`
(shape correct; secondary effects need engine mechanic — tracked in spell-mechanics-proposals.md).
Both count as refined. Approximate [x]/[~] split within L1 — exact counts in the checklist.

Level 1 remaining TODO (9): `alarm`, `create-water`, `feather-fall`, `find-familiar`,
`fog-cloud`, `grease`, `jump`, `sanctuary`, `unseen-servant`.

Batches: 1=cantrips, 2=12 L1 core, 3=10 L1 abjuration/healing, 4=10 L1 illusion/utilities,
5=13 L1 smites/warlock/ranger/nature group. DB rebuild needed after each batch (--migrate).
