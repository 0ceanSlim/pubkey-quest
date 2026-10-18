# Content gaps — review sheet (2026-10-17)

**Why:** every gap the schema check reports (`go run ./cmd/codex --check-schema`), plus Thok's
dialogue. Each one gets a concrete fix here.

**The rule for quest items:**

- **Handed over means taken.** Anything you hand over is consumed at hand-in. Today only The
  Rising Shadow does this (`"consume": true`). Everywhere else the item stays in your pack
  forever, and `quest_item` means you can never drop, sell or store it.
- **Rewards are useful.** Every reward, and every item a quest leaves you holding, does
  something in play. Selling counts only for plain treasure.

Prices use the game's scale (1 gold = 1 copper in 5e).

---

## A. Small engine fixes the content needs

| # | Fix | Why |
|---|---|---|
| E1 | **Dialogue requirements gain `race`, `class`, `stat`, `skill`**, and the dialogue reader takes labelled options (`{label, next, requirements}`) next to plain keys | Thok's and Barnaby's dialogue is written this way and the engine reads neither; their race, class and skill branches come back. Today the engine checks only gold, items, native, registered and quest_completed |
| E2 | **District moves count as visits.** Moving to `goldenhaven-east` records that district for "explore" objectives (today only the city is recorded) | The Rogue's Shadow Trial's "go to the merchant district" can never complete |
| E3 | **A worn light counts as light.** A `light-source` item in the neck slot also lifts the night darkness penalty | So the Amulet of Light (B1) frees your off hand |
| E5 | **Rings wearable.** The character has `ring1`/`ring2` slots, but the equipment tab doesn't show them and the equip code files rings under a `ring` slot that doesn't exist | B2 and B4 are rings; nothing can wear a ring today |
| E4 | **Hand-ins consume** (data only): `"consume": true` on every fetch that's handed over, listed per quest below | No quest junk stuck in packs |

## B. Quest rewards (each must do something)

| # | Item (quest) | Proposal | Use in play |
|---|---|---|---|
| B1 | `amulet-of-light` (The Black Flame) | Rare, neck. Sheds light: tag `light-source` (E3) plus effect `skill_resolve +2` | No darkness penalty at night without holding a torch; steadier against fear and corruption checks |
| B2 | `cloak-of-protection` → **`ring-of-protection`** (The Missing Heirloom) | There's no cloak or back slot. 5e's ring of protection is the same item, so: uncommon ring (E5), effect `armored_ac +1` and `unarmored_ac +1`. The mayor's reward line changes from cloak to ring | +1 AC whatever you wear |
| B3 | `ancestral-blade-restored` (Sword of the Ancestors) | Rare dwarven warhammer: 1d8 (versatile 1d10), effect when worn `strength +1`; value high but **dwarves can't sell it** (or just a high price) | A real step-up weapon for a dwarf |
| B4 | `guild-signet-ring` (The Rogue's Shadow Trial) | Uncommon, ring slot (first ring item); effect `skill_thieving +2` | Better lockpicking, pickpocketing and traps; proves guild membership in later dialogue |
| B5 | `waystone-fragment` (Elven Heritage) | Rare, neck; effect `travel_speed` +15% | Faster on every road, for good. Neck slot on purpose: it competes with B1 and D5, so which charm you wear is a real choice |
| B6 | `purified-gem` (The Shadow's Source) | A cleansed sapphire-tier gem (value per `gems.json`, 100,000) | Plain treasure: sell it, or keep it for whatever the next main-quest step needs |
| B7 | `poison-basic` ×3 (Rogue's reward) | Swap for `lockpick` ×5 | Poison has no use in play yet; lockpicks pair with the thieves' kit given in the same reward |
| — | `anchor-spare-key`, `holy-water`, `thieves-kit`, `blessed-incense`, `healing`, `rations`, `spices` | Keep as they are | Already useful |

## C. Items a quest asks you to find

| # | Item (quest) | Proposal | After the quest |
|---|---|---|---|
| C1 | `guild-dead-drop-letter` (Rogue) | Quest item; **consumed** when the stage ends (handed to Jack) | Gone |
| C2 | `golden-viper-statue` (Rogue) | Quest item; **consumed** at hand-in to Viper | Gone |
| C3 | `cult-manifesto`, `shadow-fragment` (Black Flame) | Quest items; **consumed** at hand-in to Seraphina | Gone |
| C4 | `eye-of-the-abyss` (Shadow's Source) | Quest item; **consumed** at the shrine (it becomes B6) | Gone |
| C5 | `silver-signet-ring` (Missing Heirloom) | Quest item; **consumed** at hand-in to the mayor | Gone |
| C6 | `ancestral-blade` (Sword of the Ancestors) | Quest item; **consumed** at the cache (it becomes B3) | Gone |
| C7 | `moon-essence-shard` (Elven Heritage) | Quest item, kept from stage 4; **consumed** at the waystone in stage 7 | Gone |
| C8 | `marsh-willow-bark`, `silver-moss` (Elven Heritage) | **Ordinary materials, not quest items**: gatherable in Shadowmere, sold by the herbalist; consumed for the ritual | Extras sell, and both are rune-style components (willow bark → a druid spell, silver moss → a cleric spell) |

## D. Other missing items (POI and encounter loot)

| # | Item | Proposal | Use |
|---|---|---|---|
| D1 | `leather-cap` (watchtower chest) | The head slot exists but no item uses it, and adding head AC would be a balance change for everyone. **Replace it with `padded-leggings`** in the loot table (alternative: make the first head item, a cosmetic cap) | Armour piece |
| D2 | `potion-of-water-breathing` (Shadowmere hermit) | Uncommon potion: effect `skill_athletics +5` and `constitution +2` for 1 hour, "breathe where the water is thick" | The Sunken Temple's and Flooded Mine's swim and breath checks |
| D3 | `soul-gem-fragment` (ethereal caravan) | Rare material, 5,000 gold | Treasure; sells to a temple or arcanist |
| D4 | `owlbear-fur` (wounded owlbear) | Uncommon material, 600 gold | Sells to tanners and general stores |
| D5 | `owlbear-feather-enchanted` (wounded owlbear, kindest ending) | Uncommon, neck charm; effect `skill_perception +1`, `skill_survival +1` | The reward for sparing it |

## E. Missing effects

| # | Effect | Proposal (all timed, minutes as authored) |
|---|---|---|
| E-a | `divine-grace` (crypt, raven shrine, hermit, caravan, bridge) | `skill_resolve +2`, `temp_hp +5` |
| E-b | `blood-lust` (raven shrine's dark offering) | `strength +2`, `wisdom -2` |
| E-c | `mapped-terrain` (Sentinel's Perch) | `travel_speed` +20% and `ambush_ward`: the land you've memorised is quicker and safer |
| E-d | `wind-walker` (Four Winds shrine, elven waystone, Elven Heritage) | `travel_speed` +25%, `combat_move +1` |

Encounter rewards with no duration get 12 hours by default (`divine-grace`, `wind-walker`).

## F. Missing monsters (all new; sprites to follow, for your review)

| # | Monster | Quest | Concept (5e base) | CR |
|---|---|---|---|---|
| F1 | `restless-spirit` ×3 | Paladin's Vow | A weakened specter: incorporeal, necrotic touch, night | ¼ |
| F2 | `shadow-stalker` (boss) | Black Flame | A shadow-mastiff/shadow hybrid: shadow stealth, necrotic bite, strength drain, sunlight sensitivity | 2 |
| F3 | `shadow-tentacle` ×5 | Elven Heritage | A minion: reach-2 grab (restrained), low HP, night | ⅛ |
| F4 | `corrupted-dryad` | Elven Heritage | A twisted dryad: entangle rider, charm (frightened instead), fey/undead | 1 |
| F5 | `kobold-warrior` ×10 | Sword of the Ancestors | The 5e kobold with spear and shield, pack tactics, sunlight sensitivity | ⅛ |
| F6 | `kobold-shaman` | Sword of the Ancestors | A kobold caster: fire bolt-style ranged spell, healing word on allies, sunlight sensitivity | ½ |
| F7 | `dire-wolf` ×3 | The Missing Mule | The 5e dire wolf: bite with knock prone, pack tactics | 1 |

None of these join the random road pools (the environment tags stay off), so they appear only
where their quests put them.

## G. Missing places

| # | Ref (quest) | Proposal |
|---|---|---|
| G1 | `goldenhaven-east` (Rogue) | Valid once E2 lands (the validator accepts district ids for explore objectives) |
| G2 | `flooded-mine-depths` (Sword) | A new branch in the existing **Flooded Mine** POI: past the iron grate (crowbar, from stage 1) down to the kobold warren; waypoint `flooded-mine-depths`; the kobold fight and the geode check live here |
| G3 | `dwarven-geode-cache` (Sword) | A waypoint node at the warren's end of that branch: the clan ritual check, then the stage reward |
| G4 | `shadowmere-mirror-pool` (Elven Heritage) | A waypoint node in the **Sunken Temple** past the guardian: the reflection check and the shard; the tentacle and dryad fight follows |
| G5 | `ancient-elven-waystone` (Elven Heritage) | Make the waystone a small **POI** in the Darkwood (keep the road vignette too); its spirit is G-NPC2 |
| G6 | `mountain-pass-overlook` (Missing Mule) | A small new POI in Cragspire: a clearing, the mule cornered, the dire-wolf fight, then Buttercup |

## H. Missing NPCs and dialogue

| # | NPC | Proposal |
|---|---|---|
| H1 | `town-square-board` (both dailies) | A notice board "NPC" in the Kingdom's centre district: talk opens the daily bounties; one board, both dailies point at it |
| H2 | `ancient-elven-waystone` (Elven Heritage) | Rename to `waystone-spirit`: the stone's voice, homed at the G5 POI; dialogue that opens and closes the quest |
| H3 | `barnaby-merchant` (Missing Mule) | **Make it Thistle's mule.** Thistle already complains that the mule hates her driving. The quest starts and ends with `thistle-goldworthy` at the Wayward Wagon. Barnaby stays the wandering trader |
| H4 | Thok (`exiled-orc`) | With E1, his dialogue loads as authored (race- and stat-gated branches, the rations offer). Rename his `main` node to `main_menu` |
| H5 | Barnaby | With E1, restore his class- and skill-gated branches. The filtered shops and the discount need actions that don't exist, so they stay out |

## Order of work, once approved

1. E1–E5 (engine, small), plus the validator accepting district ids.
2. Effects (E-a…E-d), items (B, C, D) and quest data edits (consume flags, B7 swap, H3
   retarget).
3. Places: Flooded Mine branch, Sunken Temple pool, waystone POI, mountain overlook.
4. NPCs: the board, the waystone spirit, Thok's file.
5. Monsters F1–F7: stat sheets for your review, then sprites.
6. `--check-schema` clean, `--validate`, `--migrate`, the wiki regenerated.
