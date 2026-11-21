# Enchanting — effects and enchanters (proposal)

Status: **BUILT 2026-11-20, unplaytested.** Balance-reviewed 2026-11-16; the maintainer
approved the hooks. Fees and success rates get tuned in playtest. This fills the two gaps
[gem-system.md](gem-system.md) §4 left open, the unique huge effects and the enchanter roster,
and writes out the tier table that §4 says a generator can produce.

**What changed in the build** (see §11 for where everything lives):
- **Three pieces, not two.** There are two ring slots and a necklace, so up to three pieces of
  one gem can stack their abilities (still capped at 20).
- **Unbroken keeps the large tier's 4 temp HP**, so the huge jasper is a superset of the large.
- **Moonwell is 1 mana every 30 minutes at night.** The game's night is 23:00–05:00, six hours;
  at 1 an hour it would have given less than the large azurite.
- **Heart of Change for a caster** (no Rage, Stamina, Ki or Cunning pool) is +2 mana at a fight's
  start instead. For the pool classes it's +10% of the pool's max, at least 1 (Rage's 100 gets 10).
- **An enchanted piece is worth half its base fee more, not the whole fee.** A shop at Charisma 20
  pays 0.95× value and a flawless stone at Brunhild costs 0.8× the base, so the full fee would
  have made enchant-and-sell pay. The validator holds this line (`value_share`).
- **Two pre-existing bugs fixed on the way.** A ring or necklace swapped out for another kept its
  worn effect, so swapping back and forth stacked a Ring of Protection without limit (an exploit).
  And the Ring of Protection gave no AC unless you wore armour.

What §4 already decided and this doesn't reopen: only rings and necklaces; size sets power,
quality sets ease; failure costs the fee and the piece survives; same-gem pieces stack; every
effect is a worn effect (`effects_when_worn`).

## 1. Balance review: what the numbers say

Checked against the game as it is (75 monsters, CR 0–5), the gem rolls (simulated, 2M rolls
each), and what the game already sells.

**How rare each size really is.** Large and huge mostly come from deep mining, not from kills:

| Source | Medium | Large | Huge |
|---|---|---|---|
| Kill, CR 1 | 1 in 460 kills | 1 in 12,000 | 1 in 290,000 |
| Kill, CR 5 | 1 in 57 | 1 in 530 | 1 in 8,800 |
| Rock face tier 3, per gem cell | 14% | 2.4% | 0.24% |
| Rock face tier 4, per gem cell | 28% | 9% | 1.7% |

At about three gem cells a day per tier-4 seam, a dedicated miner sees a large gem every few
days and a huge one every few weeks. So **large is the endgame tier most players will wear, and
huge is a long goal, not a myth.** Huge has to be strong but not game-breaking.

**What a worn piece is up against.** These are already in the game:
- Ring of Protection: AC +1 for 10,000 gold (uncommon).
- Guild Signet Ring: Thieving +2 for 5,000.
- Amulet of Light: Resolve +2 for 20,000 (rare).
- Waystone fragment: travel +15% for 15,000 (rare).

There's no natural HP or mana regeneration. A Regeneration potion gives 10 HP over 10 hours. A
caster has about 3 + level mana, and a 1st-level spell costs about 2.

**What monsters actually do.**
- Necrotic damage: 9 monsters (undead).
- Poison: 6, as a save on a bite.
- Fire: 2. Cold, lightning, acid and radiant: 1 each.
- Conditions they inflict: restrained 7, frightened 4, prone 4, poisoned 2; paralyzed, charmed
  and blinded 1 each.
- Monster defences: 7 resist fire and 7 are vulnerable to it; 11 resist or are immune to
  necrotic; 16 are immune to poison; 5 undead are vulnerable to radiant.

**What that changed from the first draft:**

1. **Regeneration was far too strong.** A small bloodstone at 1 HP every 30 minutes was 48 HP a
   day, forever. A small azurite at 1 mana every 30 minutes refilled a caster six times a day.
   New rates: small 1 per 2 hours (HP) or 4 hours (mana), large 1 per hour or 2 hours, huge 1 per
   30 minutes (HP) or 1 hour (mana).
2. **No darkvision on small gems.** A 500-gold small moonstone would have made torches, lanterns
   and the deep-mine light gates pointless. Darkvision starts at large.
3. **Resistance halves, and only from large up.** That settles question #2. The engine's
   resistance is a halving, and a 10% cut rounds to nothing on a 1d6 hit, so there's no point in
   it. 5e's Ring of Resistance is a rare item, so a small gem shouldn't give it either. Fire,
   cold and lightning resistance are nearly blank today (1–2 monsters each), so large gems pair
   them with something else.
4. **No temporary HP or combat movement on small gems.** 3 temp HP every fight is about 30% more
   effective HP for a level-1 character, on a gem that can cost 500 gold. Temp HP starts at large,
   and so does +1 move.
5. **Combat riders from two pieces of the same gem count once.** On-hit damage, resistances,
   immunities, retaliation and wards don't double. Abilities and skills still add: a ruby ring and
   a ruby necklace give STR +2, but one fire rider, not two. Two large rubies would otherwise
   double a dagger's damage. Different gems' riders do combine: ruby and topaz give fire and
   lightning.
6. **Two huge effects toned down.** Quartz's "+1d4 to every attack" was a permanent bless, better
   than a +2 weapon, so it's now +1 to attack rolls. Onyx's AC +2 on top of a Ring of Protection
   was AC +3, so it's now AC +1.
7. **Pearls can't be huge.** Shellfish give at most a large pearl, and even that is near
   impossible: about 1 large white pearl per 9,700 oysters and 1 large black pearl per 86,000.
   Medium (white 1 in 620, black 1 in 5,600) is the realistic best. So pearls get no huge effect,
   their large tier is their peak, and the black pearl's signature lives there. **That leaves 19
   huge effects, not 21.** If pearls should matter more, raise the oyster's `size_tilt` in
   `gems.json`; the effects don't need to change.
8. **Ability bonuses fade at the top, by design.** With the 20 cap, a +2 STR ring does nothing for
   a fighter whose STR is already 20. Ability gems help secondary stats, and the riders are what
   stay valuable at the cap.

## 2. The bridge, and what's already there

Marked ✅ means a worn effect can do it today: the six abilities, `skill_<id>`, `armor_class`,
`darkvision`, `travel_speed`, `ambush_ward`, `temp_hp` (granted at each fight's start),
`combat_move`, `weight_capacity`, and ticking `hp` / `mana`.

Marked 🔧 means it needs the **worn combat buff**. An effect definition gets a `combat_buff` block
in the `types.SpellBuff` schema, and every worn effect's block goes on the wearer for the whole
fight. That covers resistances, immunities, on-hit dice, retaliation, wards, death ward,
sanctuary and save dice, with no per-gem Go. This is also where the rule in point 5 lives: the
same rider from two pieces is added once.

Marked 🆕 means a one-off hook. These are **for the maintainer to review**, and each has a
fallback in §5.

## 3. Each gem, tiny to large

| Gem | Ability | Tiny | Small | Medium | Large |
|---|---|---|---|---|---|
| quartz | INT | Perception +1 | Perception +2 | INT +1 | INT +2 |
| malachite | CON | Medicine +1 | Medicine +2 | CON +1 | CON +1, resist poison 🔧 |
| azurite | INT | Resolve +1 | 1 mana / 4 h | INT +1 | INT +1, 1 mana / 2 h |
| obsidian | DEX | Thieving +1 | Thieving +2 | DEX +1 | DEX +1, +1d4 necrotic on a hit 🔧 |
| turquoise | DEX | Survival +1 | travel +5% | DEX +1 | DEX +1, travel +10%, never ambushed on the road |
| onyx | CON | Resolve +1 | Resolve +2 | CON +1 | CON +1, AC +1 |
| moonstone | WIS | Perception +1 | Perception +2 | WIS +1 | WIS +1, darkvision |
| citrine | CHA | Influence +1 | Medicine +2 | CHA +1 | CHA +1, +1d4 radiant on a hit 🔧 |
| jasper | STR | Athletics +1 | carry +10 lb | STR +1 | STR +1, 4 temp HP each fight |
| bloodstone | CON | Medicine +1 | 1 HP / 2 h | CON +1 | CON +1, 1 HP / hour |
| pearl | CHA | Influence +1 | Influence +2 | CHA +1 | CHA +2, Influence +2 |
| black pearl | CHA | Influence +1 | Thieving +2 | CHA +1 | **Abyssal Pearl:** CHA +2; the first foe to attack you each fight must pass a WIS save or lose the attack 🔧 |
| amber | WIS | Resolve +1 | Resolve +2 | WIS +1 | WIS +1, resist lightning and thunder 🔧 |
| amethyst | WIS | Resolve +1 | Resolve +2 | WIS +1 | WIS +1, can't be frightened 🔧 |
| topaz | DEX | Thieving +1 | Thieving +2 | DEX +1 | DEX +1, +1 move in a fight |
| garnet | STR | Athletics +1 | Athletics +2 | STR +1 | STR +2 |
| alexandrite | class | class skill +1 🆕 | class skill +2 🆕 | class ability +1 🆕 | class ability +2 🆕 |
| ruby | STR | Athletics +1 | Athletics +2 | STR +1 | STR +1, +1d4 fire on a hit 🔧 |
| sapphire | INT | Crafting +1 | Crafting +2 | INT +1 | INT +1, +1d4 cold on a hit, spells too 🔧 |
| emerald | WIS | Survival +1 | Survival +2 | WIS +1 | WIS +2 |
| diamond | CON | Resolve +1 | Resolve +2 | CON +1 | CON +1, resistances by quality (§4) 🔧 |

Large is either +2 to the ability or +1 with a rider; the rider gems are the expensive ones or the
situational ones. A small ruby could later count as warm clothing once cold weather lands with the
survival gear; that's noted for that plan, not built here.

## 4. The 19 huge effects

Each is its gem's large tier plus one more real thing, and different in kind.

| Gem | Name | Effect | Needs |
|---|---|---|---|
| quartz | **Lens of Clear Sight** | INT +2, Perception +3, +1 to attack rolls (weapons and spells) | 🔧 |
| malachite | **Serpent's Bane** | CON +2, Medicine +3, resist poison, can't be poisoned | 🔧 |
| azurite | **Wellspring** | INT +2, 1 mana / hour, and +2 mana at the start of every fight | 🆕 |
| obsidian | **Nightglass** | DEX +2, Thieving +3, +1d6 necrotic on a hit | 🔧 |
| turquoise | **Wayfarer's Star** | DEX +2, travel +20%, never ambushed on the road, carry +20 lb | ✅ |
| onyx | **Black Bulwark** | CON +2, AC +1, resist necrotic, undead attack you at disadvantage | 🔧 |
| moonstone | **Moonwell** | WIS +2, darkvision, 1 mana / 30 min at night (23:00–05:00) | 🆕 |
| citrine | **Sunshard** | CHA +2, sheds light like a torch that never burns down, +1d6 radiant on a hit | 🔧 🆕 |
| jasper | **Unbroken** | STR +2, Athletics +2, Resolve +2, 4 temp HP each fight, fatigue builds half as fast | 🆕 |
| bloodstone | **Heartstone** | CON +2, 1 HP / 30 min, and once a fight a blow that would drop you leaves you at 1 HP | 🔧 |
| amber | **Stormstill** | WIS +2, resist lightning and thunder, can't be stunned or paralyzed | 🔧 |
| amethyst | **Clear Mind** | WIS +2, can't be charmed or frightened, +1d4 to saving throws | 🔧 |
| topaz | **Quicksilver** | DEX +2, +2 move in a fight, +1d6 lightning on a hit | 🔧 |
| garnet | **Lifeblood** | STR +2, Athletics +2, 8 temp HP at the start of every fight | ✅ |
| alexandrite | **Heart of Change** | class ability +2, class skill +3, class resource (Rage, Stamina, Ki, Cunning) +1 | 🆕 |
| ruby | **Emberheart** | STR +2, +1d6 fire on a hit, and a melee attacker that hits you takes 1d6 fire | 🔧 |
| sapphire | **Rimeheart** | INT +2, +1d6 cold on a hit (spells too), resist cold | 🔧 |
| emerald | **Verdant Heart** | WIS +2, Medicine +3, healing spells heal 1d6 more | 🆕 |
| diamond | **Heart of Purity** | CON +2, resistances by quality (below) | 🔧 |

**Diamonds: quality sets the resistances.** This is the exception where quality is power as well
as ease. Diamonds are the rarest type, and this gives a flawless one its own pull. The effect id
carries the quality (`jewel-diamond-<size>-<quality>`); every other gem's doesn't.

| Diamond quality | Large resists | Huge resists |
|---|---|---|
| flawed | fire | fire, cold |
| standard | fire, cold | fire, cold, lightning |
| fine | fire, cold, lightning | + acid, poison |
| flawless | + acid, poison | all seven: + necrotic, radiant |

## 5. Hooks (🆕, approved and built)

| Hook | Used by | Where it'd go | Fallback if skipped |
|---|---|---|---|
| Extra mana at a fight's start | azurite huge | beside `temp_hp` at fight start | 1 mana / 30 min instead |
| An effect only between dusk and dawn | moonstone huge | a `"when": "night"` on a modifier, read where effects tick | 1 mana / 2 h, all day |
| A worn light that never burns down | citrine huge | the light rules (`combat.InTheDark`, `lightAtNight`) | Perception +3 instead |
| Fatigue builds half as fast | jasper huge | `status`, where fatigue is added | 6 temp HP each fight |
| Effects that follow the class | alexandrite, every size | a class → ability/skill table, read at equip | make alexandrite a plain WIS gem (Clarity's twin) |
| Bigger class resource | alexandrite huge | the resource pool max at fight start | class skill +4 instead |
| Healing spells heal more | emerald huge | where a heal is rolled | 1 HP / hour instead |

## 6. Quality

| Quality | Success | Fee | Extra |
|---|---|---|---|
| flawed | base −20% | ×1 | none |
| standard | base | ×1 | none |
| fine | base +15% | ×0.8 | none |
| flawless | always | ×0.8 | adds the gem's **tiny** effect too |

Success tops out at 95% short of flawless. The server rolls it. A failure costs the fee and the
piece survives, so retrying costs nothing but fees, and there's nothing to exploit.

## 7. Fees

A flat base by size; the gem's value is the real gate. Fee = base × enchanter × quality factor.

| Size | Base fee |
|---|---|
| tiny | 100 gold |
| small | 500 gold |
| medium | 2,500 gold |
| large | 10,000 gold |
| huge | 50,000 gold |

**Check against the shop items.** The whole cost of a piece is the gem, the ring (1,000 gold),
the jeweller's labour (10% of the gem's value) and the fees divided by the success chance:
- **Medium quartz ring, INT +1:** about 6,500 gold with Corvin. Below the Ring of Protection's 10,000, and an
  ability +1 is worth less than AC +1.
- **Large onyx ring, CON +1 and AC +1:** about 68,000 gold. Well above the Ring of Protection,
  which it beats.
- **Large ruby ring:** about 1,100,000 gold, almost all of it the gem. Expensive gems aren't
  stronger than cheap ones at the same size; they carry the better riders and are worth more.
  Value is a wealth axis, size is power.
- **Huge quartz ring:** about 245,000 gold.

None of these undercut what the shops already sell, and the gem's rarity does the gating. **Fit;
tune in playtest.**

## 8. Enchanters: four NPCs who already exist

No new NPCs. Enchanting becomes a service of four characters who are already placed, scheduled
and fit the job. That's "don't clutter the NPCs" and the "real job" rule at once. Each gets an
`enchant_config` and a labelled dialogue option.

| Enchanter | Where | Today | Pieces | Gems | Up to | Success | Fee × | Cost per success (medium / large / huge) |
|---|---|---|---|---|---|---|---|---|
| **Corvin Ashgrove**, Court Arcanist | Kingdom | sells reagents | both | any | large | 85% | ×1.5 | 4,400 / 17,600 / – |
| **Silvanthir Moonbough**, Warden of Roots | Verdant | vault keeper; tends the enchanted trees | both | malachite, emerald, amethyst, moonstone, azurite, pearl, black pearl | large | 90% | ×1.2 | 3,300 / 13,300 / – |
| **Brunhild Hammerfall**, Forgemistress | Ironpeak | smith, commissions | rings only (runes cut into the band) | onyx, garnet, jasper, ruby, amber, bloodstone, diamond | large | 80% | ×1.0 | 3,100 / 12,500 / – |
| **Seraphina**, Oracle of the Dawnflame | Goldenhaven | dialogue only, no shop | both | any | **huge** (the only one) | 75% | ×2.0 | 6,700 / 26,700 / 133,000 |

Why these four:
- **Corvin** is the generalist in the capital.
- **Silvanthir** already speaks to enchanted trees.
- **Brunhild** already takes commissions, so runes fit her work.
- **Seraphina** is the one magic NPC in Goldenhaven and has no job beyond talk; this gives her
  one. A huge commission is then one trip: the jeweller (Nissa) and the only huge enchanter are in
  the same city.

Coverage: every gem has Corvin up to large and Seraphina up to huge; 14 of the 21 also have a
specialist who is cheaper per success.

## 9. Items

Generated like the rest (gem-system.md §7, Option A):
- **Pieces:** `ring-<type>-<size>-<quality>-enchanted` and `necklace-…`.
- **Their worn effect:** `effects_when_worn` → `jewel-<type>-<size>`, plus `jewel-<type>-tiny`
  when flawless. Diamonds use `jewel-diamond-<size>-<quality>`.
- **Value:** the piece plus half the base fee (`value_share`; see the status note).
- **Count:** 2 pieces × 21 types × 5 sizes × 4 qualities, less the pearls' huge sizes = 824 more
  items. Effects: 164 — 17 gems × 5 sizes, the pearls × 4, the diamond's 3 plain sizes and 4
  qualities each at large and huge, and alexandrite's 5 sizes × 12 classes.

## 10. Still open

- **Fees and success rates:** tune in playtest. Effects are data, so a ring someone already wears
  can be retuned after launch without touching their save.
- **Sprites:** enchanted pieces use their gem's ring and necklace sprites, with a glow from
  `src/styles/gems.css`. Their own art can come later.

## 11. Where it lives

- **Data:** `game-data/systems/gems.json` → `enchanting` (fees, qualities, the class table, every
  gem's effect by size). Enchanters: `enchant_config` on the NPC JSON, plus `"enchant"` in the
  working shift's `available_actions`.
- **Generation:** `game/gems/enchant.go` (`Effects`, `EffectID`, `Describe`, fees, chance,
  `Refusal`); `--migrate` inserts the effects after the effects table (`migration/gems.go`).
- **The service:** `api/game/enchanter.go` (the "Enchant jewelry" option and the `enchant`
  action) and `src/ui/enchanter.js`.
- **Worn effects follow the gear:** `effects.SyncWorn` runs on every gear change and on load:
  one instance per piece, none for a piece that's gone.
- **Fights:** `combat.wornBuffs` puts every worn `combat_buff` on at a fight's start, one per
  `spell_id` (the gem), the higher `rank` winning. `attack_spells` and `sanctuary_once` are new
  buff fields.
- **Hooks:** `"when": "night"` on a periodic modifier and `fatigue_slow` in `effects.TickEffects`
  (and `status.Exert`); `light` in `combat.holdsLight` and the night stealth rule; `fight_mana`
  and `resource_pct` at a fight's start (`combat.go`); `heal_d6` on heal spells
  (`spells/cast.go`); `effects_when_worn_by_class` beside `_by_race` (`effects.WornEffectIDs`).
- **Checks:** `--validate` checks the table, the enchanters and `value_share`; the wiki gives each
  gem an "Enchanted" table and each enchanter a service line.
