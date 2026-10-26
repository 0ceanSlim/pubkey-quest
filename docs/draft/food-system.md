# Food: hours of fullness, cooking, and what there is to eat

The food workshop (2026-10-25), part of the degradation system
([degradation-system.md](degradation-system.md) §3, which covers spoilage, the stale tier and
spoiled food). This draft settles what food is worth, where it comes from, how it's cooked
and preserved, and the full list of foods. Fishing ([fishing-system.md](fishing-system.md))
takes its fish food values from here. **Status: decided 2026-10-25; built 2026-10-25/26** as step 8 of the degradation build (degradation
§9 lists the files). Not yet: gathered berries and mushrooms (they come with gathering POIs), and
boars and deer (no such monsters yet; other beasts drop the meat).

## 1. Decisions so far

- **Food is measured in hours of fullness.** Each food keeps you fed for a time: berries 1h,
  bread 4h, a stew 8h. Eating feeds the hunger countdown you already have (the hunger level and
  its tick accumulator). Time past the current level's bar spills into the next level up. The
  four levels (Starving, Hungry, Well Fed, Stuffed), their pace (one level every 4 hours, every
  6 when Stuffed) and their effects stay as they are. Tooltips read "Keeps you fed 4 hours".
- **Cooking is simple transforms at a fire.** One item in, one item out: raw becomes cooked,
  and raw plus salt becomes salted fish or jerky. There's no recipe system (general crafting
  stays out of the alpha). The hearty stew comes ready-made from inns.
- **The fire:** in the wild, a **tinderbox** lights a cooking fire (a Cook action that takes 30
  minutes). In town you hand your raw food to an **innkeeper**, who cooks it for a few gold.
  The tinderbox and the inns both gain a job.
- **No thirst.** Drinks are food or effects (beer). The waterskin matters only in the desert,
  through the dehydration idea queued with camping.
- **A few meal buffs, from proper meals only.** Inn dishes give a short buff (a hearty stew
  +1 Constitution for 4 hours). One meal buff at a time: a new meal replaces the last. Most food
  just fills you up.
- **A big fish is one meal, with a cap.** Fish don't stack, so a fish isn't cut into
  portions. Fullness grows with size up to a cap (large and huge are worth the same as food);
  a huge fish's value is selling or mounting it.

## 2. The foods

Prices follow the PHB, as for other mundane goods (1 gold here = a D&D copper). Stale food (past
half its shelf life, degradation §3) gives half the hours and no buff.

| Food | Fullness | Keeps | Weight | Price | From | Notes |
|---|---|---|---|---|---|---|
| Rations | 6h | forever | 2 lb | 50 | shops | −3 fatigue; still stacks 10. Three a day: the starting 6 last two days |
| Bread | 4h | 3 days | 0.5 lb | 2 | general stores, inns | the cheap staple |
| Cheese | 3h | 10 days | 0.5 lb | 10 | general stores | |
| Apple | 1h | 7 days | 0.3 lb | 1 | shops, forest gathering | |
| Wild berries | 1h | 1 day | 0.1 lb | — | forest and hill gathering | |
| Mushrooms | 2h | 2 days | 0.1 lb | 5 | swamp and forest gathering | some are poisonous: a Survival check when gathered |
| Raw meat | 2h | 12 hours | 1 lb | 5 | beast loot (boar, deer, wolf, bear) | eaten raw: a CON save or food poisoning |
| Cooked meat | 5h | 2 days | 1 lb | 15 | cooking raw meat | |
| Jerky | 4h | 60 days | 0.5 lb | 20 | meat + salt at a fire; shops | the long-trip food |
| Raw fish | by size (§3) | 12 hours | by size | by species | fishing | eaten raw: like raw meat |
| Cooked fish | by size (§3) | 2 days | as raw | | cooking raw fish | |
| Salted fish | by size (§3) | 30 days | as raw | | raw fish + salt | |
| Hearty stew | 8h, +1 CON for 4h | eaten there | — | 30 | inns | |
| Spoiled food | 1h | — | | 0 | anything that spoils | a CON save (DC 10) or food poisoning (degradation §3) |
| Beer | 0 | forever | | 4 | inns | unchanged: a 50% chance of getting drunk |
| Goodberry | 4h | 1 day | | — | the spell | generous, as in 5e (one berry is a day's food) |

## 3. Fish as food

One fish is one meal. Fullness grows with size and stops at large:

| Size | Raw | Cooked | Salted |
|---|---|---|---|
| tiny | 1h | 2h | 1h |
| small | 2h | 4h | 2h |
| medium | 3h | 6h | 3h |
| large | 4h | 8h | 4h |
| huge | 4h | 8h | 4h |

## 4. Eating, inns and selling (decided)

- **Eating when full:** at Stuffed with a full bar you can't eat ("You couldn't manage another
  bite"). Below that you can eat anything, and whatever spills past full is lost. The tooltip
  warns when a food would be partly wasted.
- **Inn meals are ordered from the innkeeper:** a dialogue option, "Order a hot meal". You pay
  and eat it on the spot, so the buff starts at once. Bread and beer stay on the innkeeper's
  shelf like shop goods, to carry away.
- **Breakfast:** sleeping at an inn wakes you at Well Fed with its bar full (about 4 hours
  before you're Hungry). No buff: a buff takes a proper meal.
- **Selling food goes by freshness:** fresh at its price, stale at half, spoiled for nothing. It
  covers fish too, so a fresh catch is worth hurrying back for.

## 5. Defaults (approved)

- **Cook** (in the wild, with a tinderbox, outside a building) cooks all the raw meat and fish
  you carry in one go. It takes 30 minutes, like the wait. **Salt** is per item: one salt turns
  one raw fish into salted fish, or one raw meat into jerky, at the same fire.
- **Innkeeper cooking:** "Cook my food" in the innkeeper's dialogue, 2 gold per item, no wait.
  Innkeepers don't salt.
- **Eating raw meat or fish:** a Constitution save (DC 10), or food poisoning (degradation §3:
  −1 CON and no hunger recovery for 4 hours). Spoiled food uses the same save.
- **Meat comes from beasts:** boar, deer, wolf and bear drop raw meat (1–2), added to their
  loot tables. Other monsters don't.
- **Gathered foods** (berries, apples, mushrooms) arrive with gathering POIs (M5.5). Until then
  apples are only in shops, and berries and mushrooms wait.
- **Meal buffs** are ordinary timed effects (`meal-hearty` for now) in one group, so a
  new meal replaces the last.

## 6. Build notes

- The `hours` field on food: `"food": {"hours": 4}` (rations also keep `fatigue: -3`). Fish
  take their hours from the size band (§3) at migration, alongside the generated fish ids
  (fishing §2).
- Eating adds the hours to the hunger accumulator in the right direction: it pushes the
  next drop back, and overflow raises the level, capped at Stuffed-full. That's one function in
  `game/status/hunger.go` that every food (and goodberry) goes through.
- Stale halves the hours (degradation §3). The Wait preview already runs the real time
  advance, so it shows food spoiling for free.
- Build it with degradation, in the §9 order: spoilage needs condition storage first.
