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

## Level 1 — 55 spells — Batch 2 in progress (12 done)

- [ ] `alarm` — Alarm (ritual)
- [ ] `animal-friendship` — Animal Friendship
- [x] `armor-of-agathys` — Armor of Agathys: mana 2→3 (no-conc 1hr strong warlock self-buff); removed mana-crystals+quartz-dust (wrong tier/theme); now free (no thematic rune fit for cold/retaliation); fixed range "self"→"0"; added heal/effect null; **retaliatory-damage** in proposals
- [~] `arms-of-hadar` — Arms of Hadar: mana 2→3 (AoE necrotic); removed void-crystal+demon-ichor (capstone components on L1!); added spirit-dust×1 (dark/necrotic, 75gp — correct L1 tier); fixed range "self"→"1" (10ft AoE); added area_effect tag; added heal/effect null; **reaction-suppression-AoE** in proposals
- [x] `bane` — Bane: mana 2→3 (hard 3-target conc debuff); removed ether-essence+arcane-powder (wrong theme — arcane not dark); added spirit-dust×1 (dark curse, 75gp); fixed range 1→2 (30ft); added heal null; updated effect prose
- [x] `bless` — Bless: mana 2→3 (strong 3-target conc buff); removed blessed-incense×2+sacred-oil (over-costed, two components); kept blessed-incense×1 only (divine theme, focus: amulet free); fixed range_long 2=range; added heal null; updated notes
- [x] `burning-hands` — Burning Hands: mana 2→3 (AoE fire cone); removed sulfur×2+arcane-powder (sulfur 500gp×2 = Fireball-tier cost on L1!); added ash×1 (fire theme, 15gp — correct L1 tier); +artificer class; added heal/effect null
- [~] `charm-person` — Charm Person: mana 2→3 (hour-long hard control); removed ether-essence+quartz-dust; now free (basic enchantment, not substance-themed); fixed range 6→2 (30ft); removed concentration (not conc in 5e); added heal/effect null; added charm/social tags; **charm-condition** in proposals
- [ ] `color-spray` — Color Spray
- [~] `command` — Command: mana 2 (1-round, expires fast); free (not substance-themed); fixed range 12→4 (60ft); added heal/effect null; **command-action-variants** in proposals
- [ ] `compelled-duel` — Compelled Duel
- [ ] `comprehend-languages` — Comprehend Languages (ritual)
- [ ] `create-water` — Create or Destroy Water
- [ ] `cure-wounds` — Cure Wounds
- [ ] `detect-evil` — Detect Evil and Good
- [ ] `detect-magic` — Detect Magic (ritual)
- [ ] `detect-poison` — Detect Poison and Disease (ritual)
- [ ] `disguise-self` — Disguise Self
- [ ] `divine-favor` — Divine Favor
- [ ] `ensnaring-strike` — Ensnaring Strike
- [ ] `entangle` — Entangle
- [ ] `expeditious-retreat` — Expeditious Retreat
- [ ] `faerie-fire` — Faerie Fire
- [x] `false-life` — False Life: fixed range "self"→"0"; fixed material_component null→proper block; added heal "1d4+4" (models temp HP grant); added effect prose; removed combat tag (buff); mana 2 kept (minor no-conc buff); free (utility, not substance-themed)
- [ ] `feather-fall` — Feather Fall
- [ ] `find-familiar` — Find Familiar (ritual)
- [ ] `fog-cloud` — Fog Cloud
- [ ] `goodberry` — Goodberry
- [ ] `grease` — Grease
- [ ] `guiding-bolt` — Guiding Bolt
- [ ] `healing-word` — Healing Word
- [ ] `hellish-rebuke` — Hellish Rebuke
- [ ] `heroism` — Heroism
- [ ] `hex` — Hex
- [ ] `hunters-mark` — Hunter's Mark
- [ ] `identify` — **Analyze Weakness** (homebrew)
- [x] `inflict-wounds` — Inflict Wounds: mana 2→3 (3d10 highest single-target damage at L1); removed holy-water+blessed-incense (divine/healing domain — WRONG for necrotic); added spirit-dust×1 (necrotic dark, 75gp); added heal/effect null
- [ ] `jump` — Jump
- [ ] `longstrider` — Longstrider
- [ ] `mage-armor` — Mage Armor
- [x] `magic-missile` — Magic Missile: mana 2 kept (reliable auto-hit, not signature/substance); now free (removed arcane-powder×3 — not substance-themed); +artificer class; removed stale D&D note; added heal/effect null
- [ ] `protection-from-evil` — Protection from Evil and Good
- [ ] `purify-food` — Purify Food and Drink (ritual)
- [ ] `sanctuary` — Sanctuary
- [ ] `searing-smite` — Searing Smite
- [ ] `shield` — Shield
- [ ] `shield-of-faith` — Shield of Faith
- [ ] `silent-image` — Silent Image
- [ ] `sleep` — **Exhausting Hex** (homebrew)
- [ ] `speak-with-animals` — Speak with Animals (ritual)
- [ ] `thunderous-smite` — Thunderous Smite
- [~] `thunderwave` — Thunderwave: mana 2→3 (AoE + push); removed bone-dust+tree-sap (wrong theme — necrotic/nature, not thunder); now free (thunder has no substance rune); fixed range_long 1=range; added heal/effect null; **AoE-push** in proposals
- [ ] `unseen-servant` — Unseen Servant (ritual)
- [~] `witch-bolt` — Witch Bolt: mana 2 kept (single-target conc); removed sulfur+arcane-powder (fire/arcane — wrong for lightning); added iron-filings×1 (lightning theme, 20gp); fixed range 6→2 (D&D 30ft); added heal/effect null
- [ ] `wrathful-smite` — Wrathful Smite

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
| 1     | 55    | 7    | 5              | 43   |
| 2     | 2     | 0    | 0              | 2    |
| 3     | 1     | 0    | 0              | 1    |
| **Total** | **84** | **22** | **16** | **46** |

Note: "Done" = `[x]`, "Needs-mechanic" = `[~]` (shape is correct; engine mechanic
pending). Both count as refined for the batch. Batch 2 refined 12 L1 spells.
