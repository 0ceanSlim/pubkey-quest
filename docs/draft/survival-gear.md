# Survival gear: cold, heat and the whistle

**Status:** BUILT 2026-11-21 (approved 2026-11-20 as written). Part of the beta gate (roadmap M7,
"Survival-gear mechanics", approved 2026-10-14 one at a time). Unplaytested.

What changed in the build:
- **Three small counters in the save** (`types.Limits`), not none as the draft said:
  - `drank`, the minute of your last drink;
  - `dry`, hours in the heat since then;
  - `exposure`, failed saves toward Unbroken's next point of fatigue.
  They can't be derived, and without them a reload would forget a drink.
- **The saves fall on the clock's hour marks.** A spell of time that crosses three marks is three
  saves, so there's no timer, and fishing, mining, waiting and walking all count alike.
- **One fire warms a party.** At a party camp, a tinderbox in anyone's pack keeps the cold off
  everyone.
- **What counts as a drink:** a waterskin, any potion or tea, and Create Water. Fresh water's
  effect got an instant `water` modifier. A drink shows **Watered** for its two hours.
- **The whistle's cooldown is the road's:** six hours of game time after any fight.

Already done:
- light and darkness;
- weapon coatings (whetstone, basic poison);
- ball bearings and caltrops;
- the waterskin's charges, with Drink and Fill (`api/game/water.go`).

This draft covers the three that are left:
1. cold in the Frozen Wastes;
2. heat in the Sunscorch Desert, which finally gives the waterskin a job;
3. the signal whistle.

Shared ground items are a separate plan.

## 1. Cold: the Frozen Wastes

The 5e extreme-cold rule, using our fatigue track (0–10) in place of exhaustion levels.

- **Every hour in the Frozen Wastes:** a Constitution save, DC 10. On a fail you gain +1 fatigue
  and the log says "🥶 The cold gets into you." The hour counts whether you're walking, stopped or
  waiting, and it pauses during a fight because the clock is frozen.
- **Warm means the save passes automatically.** You're warm if any of these hold:
  - you carry **winter furs**, a new item (see below);
  - a worn effect gives cold resistance: Rimeheart (huge sapphire), or an enchanted large
    or huge diamond;
  - you're asleep at a camp with a fire.
- **A blanket in your pack** gives advantage on the save. This keeps the roadmap's "blanket clears
  it" without making the furs pointless.
- **Sleeping out there:** in the Wastes a camp needs a **tinderbox** for its fire, as well as the
  usual bedding. With a fire, no saves while you sleep. Without one you can still sleep rough, but
  the hourly saves keep coming all night.
- **What a crossing costs:** the Wastes take 24 hours to cross. A CON 12 traveller with no
  protection fails about 45% of the time, which is roughly +10 fatigue: exhausted by the far side.
  A blanket alone brings that to about +5. Furs bring it to nothing. The cold doesn't kill you
  outright; exhaustion makes the Wastes' monsters dangerous.
- **New item, winter furs:**
  - a hooded fur coat and mittens, carried in the pack (no slot, so it doesn't fight the necklace);
  - 8 lb, 450 gold, common, tags `camping`;
  - sold at the Frosthold and Ironpeak general stores;
  - needs a sprite.

## 2. Heat and water: the Sunscorch Desert

This uses the 5e extreme-heat rule.

- **When it applies:** by day in the desert, 08:00–18:00, set in a new `survival.json`.
- **Each hour:** a Constitution save at DC 5, plus 1 for each hour since you last drank. On a fail
  you gain +1 fatigue and the log says "🥵 The heat saps you."
- **Medium or heavy armour** gives disadvantage on the save, as in 5e.
- **Watered means the save passes automatically:** for 2 hours after a drink you're **Watered**, a
  visible 120-minute effect. These count as a drink:
  - a waterskin charge;
  - the Create Water spell (its `fresh-water` effect also grants Watered);
  - any potion or tea.
- **Auto-drink:** when a heat save comes due and you aren't Watered, you drink from a waterskin
  if you carry one with water in it. The log says "You take a swig from your waterskin (3 left)."
  This saves the player from tracking it every two hours. You can still drink by hand to get ahead.
- **What a crossing costs:**
  - The desert takes 16 hours. About 10 of those fall in a single day's heat, which is 5 drinks,
    exactly one full waterskin. That's the waterskin's job.
  - With an empty skin, the DC climbs each hour: by the tenth hour it's DC 15, and most travellers
    reach the far side very tired.
  - Fire resistance (an enchanted large or huge diamond) makes you heat-proof.
- **Water at both ends:** Goldenhaven and Dusthaven are towns, so you can fill your skin before you
  set out.

## 3. The signal whistle

- **Blow the whistle:** an action from the pack, in the wild only, and not during a fight, a POI or
  another encounter. It rolls a road encounter now, using the same biome, level band, day or night
  monsters and stealth opening as the roads.
- **For people who want a fight:** fishing or mining XP runs, testing a build, or a bounty.
- **It keeps the road's encounter cooldown,** so it can't be used to farm. Blowing it too soon
  says "Nothing answers. Whatever's out here is still keeping its distance."
- **In a party,** only the leader can blow it, just as only the leader rolls the road.

## 4. Build notes

- **`game-data/systems/survival.json`** holds the environments, DCs, heat hours, Watered minutes,
  the item ids for furs, blanket and tinderbox, and the armour rule. It's validated by codex.
- **`game/status/exposure.go`** has `Exposure(state, minutes, tally)`, called from the time-advance
  path beside hunger and fatigue.
  - The hourly tally is session memory, like `Exert`'s, so nothing new goes in the save.
  - Watered is an ordinary timed effect, so it lives in `ActiveEffects` like the others.
  - The fatigue goes through `HandleFatigueChange`, so Unbroken's `fatigue_slow` applies here too.
- **Camp check:** `npc.HasBedding` and its siblings get a "fire" requirement in the Wastes.
- **The whistle:** a `use_on`-style action calls `maybeRollTravelEncounter` with a forced hit,
  behind the cooldown.
- **Client:**
  - a cold or heat icon by the hunger and fatigue icons while you're exposed;
  - the item actions (Blow, and Drink already exists).
- **Item notes** for the waterskin, blanket, tinderbox and whistle lose their "no use yet" lines,
  and the wiki picks them up.
- **Tests:**
  - save odds with and without each kind of protection;
  - auto-drink spends a charge;
  - Watered skips the save;
  - the camp in the Wastes needs a fire;
  - the whistle respects the cooldown and the party leader rule.
- **`updates.json`** gets an entry.

## 5. Where it lives

- **Config:** `game-data/systems/survival.json`, typed by `types.SurvivalConfig` / `types.Exposure`,
  and validated by `codex/validation/survival.go`.
- **`game/survival`:**
  - `Pass` is called from `gametime.AdvanceTime`.
  - `Sleep` is called from `npc.applySleep`, whose fire flag is `partyFire` and `PartySync.Fire`.
  - It also shows and clears the weather effects `bitter-cold` and `desert-heat`.
- **Drinks:** `effects.Drink` (`effects/water.go`) is called by:
  - the instant `water` modifier;
  - the waterskin's Drink (`api/game/water.go`);
  - potions in `handleUseItemAction`;
  - the auto-drink.
- **Whistle:**
  - `api/game/whistle.go` (`blow_whistle`) calls `rollTravelEncounter(force)`, which uses
    `encounter.Pick`.
  - On the client it's the whistle's **Blow** action (`inventoryInteractions.js`).
- **Items:**
  - `winter-furs` is new and needs a sprite. Bjorn Frostmantle in Frosthold and Ironpeak's
    mining supplier sell it.
  - The notes on the waterskin, blanket, tinderbox and whistle say what each does.
- **Tests:** `tests/status/survival_test.go` and `api/game/whistle_test.go`.
