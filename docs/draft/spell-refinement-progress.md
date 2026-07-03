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

## Level 1 — 55 spells — Batch 2 complete (12 refined); Batch 3 in progress (10 done)

### Batch 2 — refined + prep_time backfilled
- [ ] `alarm` — Alarm (ritual)
- [ ] `animal-friendship` — Animal Friendship
- [x] `armor-of-agathys` — Armor of Agathys: mana 2→3; free; range "self"→0; retaliatory-damage in proposals; **prep_time 90** (strong 1-hr no-conc self-buff)
- [~] `arms-of-hadar` — Arms of Hadar: mana 2→3; spirit-dust×1; range "self"→1; reaction-suppression-AoE in proposals; **prep_time 75** (AoE + rune cost → not triple-taxed)
- [x] `bane` — Bane: mana 2→3; spirit-dust×1; range 1→2; effect prose; **prep_time 105** (3-target conc hard debuff, complex coordination)
- [x] `bless` — Bless: mana 2→3; blessed-incense×1 (amulet=free); range_long fixed; **prep_time 90** (3-target conc buff; focus makes component free for clerics/paladins)
- [x] `burning-hands` — Burning Hands: mana 2→3; ash×1; +artificer; **prep_time 45** (fast AoE, instinctive fire; cheapest rune)
- [~] `charm-person` — Charm Person: mana 2→3; free; range 6→2; no conc; charm-condition in proposals; **prep_time 75** (hard control, 1-hour duration)
- [ ] `color-spray` — Color Spray
- [~] `command` — Command: mana 2; free; range 12→4; command-variants in proposals; **prep_time 30** (1-round only, simplest control spell)
- [ ] `compelled-duel` — Compelled Duel
- [ ] `comprehend-languages` — Comprehend Languages (ritual)
- [ ] `create-water` — Create or Destroy Water
- [ ] `cure-wounds` — → Batch 3
- [ ] `detect-evil` — → Batch 3
- [ ] `detect-magic` — → Batch 3
- [ ] `detect-poison` — → Batch 3
- [ ] `disguise-self` — Disguise Self
- [ ] `divine-favor` — → Batch 3
- [ ] `ensnaring-strike` — Ensnaring Strike
- [ ] `entangle` — Entangle
- [ ] `expeditious-retreat` — Expeditious Retreat
- [ ] `faerie-fire` — Faerie Fire
- [x] `false-life` — False Life: range "self"→0; heal "1d4+4"; effect prose; free; **prep_time 40** (minor necromantic utility, quick)
- [ ] `feather-fall` — Feather Fall
- [ ] `find-familiar` — Find Familiar (ritual)
- [ ] `fog-cloud` — Fog Cloud
- [ ] `goodberry` — Goodberry
- [ ] `grease` — Grease
- [ ] `guiding-bolt` — Guiding Bolt
- [ ] `healing-word` — → Batch 3
- [ ] `hellish-rebuke` — Hellish Rebuke
- [ ] `heroism` — → Batch 3
- [ ] `hex` — Hex
- [ ] `hunters-mark` — Hunter's Mark
- [ ] `identify` — **Analyze Weakness** (homebrew)
- [x] `inflict-wounds` — Inflict Wounds: mana 2→3; spirit-dust×1; necrotic 3d10; **prep_time 60** (standard melee attack; rune cost makes it not triple-taxed)
- [ ] `jump` — Jump
- [ ] `longstrider` — Longstrider
- [ ] `mage-armor` — → Batch 3
- [x] `magic-missile` — Magic Missile: mana 2; free; +artificer; automatic hit; **prep_time 35** (fast classic evocation, no component)
- [ ] `protection-from-evil` — → Batch 3
- [ ] `purify-food` — Purify Food and Drink (ritual)
- [ ] `sanctuary` — Sanctuary
- [ ] `searing-smite` — Searing Smite
- [ ] `shield` — → Batch 3
- [ ] `shield-of-faith` — → Batch 3
- [ ] `silent-image` — Silent Image
- [ ] `sleep` — **Exhausting Hex** (homebrew)
- [ ] `speak-with-animals` — Speak with Animals (ritual)
- [ ] `thunderous-smite` — Thunderous Smite
- [~] `thunderwave` — Thunderwave: mana 2→3; free; AoE-push in proposals; **prep_time 75** (AoE + push, powerful; no rune cost)
- [ ] `unseen-servant` — Unseen Servant (ritual)
- [~] `witch-bolt` — Witch Bolt: mana 2; iron-filings×1; range 6→2; sustained-conc-damage in proposals; **prep_time 55** (conc lightning; rune cost modest)
- [ ] `wrathful-smite` — Wrathful Smite

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

| Level | Total | Done | Needs-mechanic | TODO |
|-------|-------|------|----------------|------|
| 0     | 26    | 15   | 11             | 0    |
| 1     | 55    | 18   | 5              | 32   |
| 2     | 2     | 0    | 0              | 2    |
| 3     | 1     | 0    | 0              | 1    |
| **Total** | **84** | **33** | **16** | **35** |

Note: "Done" = `[x]`, "Needs-mechanic" = `[~]` (shape is correct; engine mechanic
pending). Both count as refined. Batch 2 refined 12 L1 spells (+prep_time backfilled).
Batch 3 refined 10 more L1 spells (abjuration/healing/divination group).
