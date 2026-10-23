# Playtest notes: the new start (intro + tutorial)

Running notes from the user's local playthroughs of the intro and tutorial
([intro-and-tutorial.md](intro-and-tutorial.md)). Newest first. Each item says what was seen, the
cause where known, and the fix planned. Nothing here is pushed until the user says so.

## Playtest 3 (2026-10-22): the reworked start

- **P1** The gear cards showed no damage and looked clunky. **Fixed:** the stats sit under
  `properties` in the client's item data, and the cards didn't look there. The card is now a
  compact row: the picture, the name, then damage (or AC) and properties. Single items have no
  second frame. Checked in a static preview with the built CSS.
- **P2** After the gear screens you were dropped back at the vault. **Fixed:** the conversation
  with the keeper picks up again (on claiming or on "Not now").
- **P3** The 💡 was hard to notice. **Fixed:** it's now an amber "💡 How?" pill that glows until
  that step's tip has been opened. Only the tutorial quests have tips. The screen tour still runs
  once per browser for a new character (no XP, day 1), right after the intro and before the first
  quest step. Settings → Show me around runs it again.
- **P4** The Wait window drew too big, and "Sleep till dawn" showed everywhere. **Fixed:** it's
  smaller (220 px), and the button is now "Make camp till dawn". It shows only out in the wild,
  after 9 PM, with a bedroll or a camp set (the preview's `can_camp`). Inn rooms keep their own
  Sleep button.
- **P5** Quest complete came after the giver's dialogue, then their menu, then Goodbye.
  **Fixed:** choosing Turn in closes the conversation and shows the rewards screen at once,
  with the giver's closing line on it (the quest's completed conversation's opening words, or a
  thank-you). The XP, the gold and the screen all arrive together.

## Playtest 2b (2026-10-22): the road and fights (separate list)

- **S1** The stats tab showed no total XP, only this level's share in the header bar. **Fixed:**
  the stats tab shows Total XP and the XP the next level needs.

- **R1** Finding a POI on the road paused the clock, but the Enter button only showed after pressing
  play, and the discovery XP popped up later too. Wanted: stop beside it, then go in, continue or
  turn back. **Fixed:** the discovery stops travel (server, `maybeDiscoverPOIs`), and the client
  redraws the road from fresh state: Enter, Continue Journey, Turn Back, and the XP. The cause was
  a race: the Enter buttons were painted before the new POI had loaded, with the old progress, and
  nothing repainted until the next tick.
- **R2** Sneaking up on the bandits in the watchtower (climb, then "drop down with a surprise
  attack") still rolled initiative, and they could act first. **Fixed:** a POI monster node's
  `surprise` means the player's ambush (schema: "player gets first round"), but the bridge passed
  it as the engine's `Surprise` (the *party* caught off guard). It's now `MonstersSurprised`. A sneak
  attack also skips the initiative dice in the log, since you have the first move
  (`tests/combat/sneak_attack_test.go`).
- **R3** A Barbarian saw Cast Spell in combat. **Fixed:** it shows only for characters with spell
  slots. Scrolls are cast through Use Item.
- **R4** Rage built up showed on the combat ability button but not in the header's resource bar.
  **Fixed:** the bar shows the fight's live pool (Rage, Ki, …) during combat, then goes back.

## Playtest 2 (2026-10-22): Fighter in the Kingdom, then a Barbarian

### Bugs (cause found)

| # | Seen | Cause | Fix |
|---|---|---|---|
| B1 | Robbed by a pickpocket right after arriving: "Failure (perception check)… your coin purse feels lighter. **You can't afford that.**" | `city-pickpocket` (5% per visit to Kingdom Center or Goldenhaven) charges its theft as a 10-gold *transaction*, so with an empty purse (the gold is in the vault) it reads as a failed purchase. Nothing keeps authored encounters away during the tutorial. | A theft takes what's there, or "your purse was empty: he got nothing". No authored encounters while the tutorial quest is active. **Fixed:** a `theft` cost (`cost.theft`) takes what you carry and never fails; the scheduler skips authored encounters while a creation quest is active. |
| B2 | After waiting until the inn opened, going in worked but the log still said "The Wayward Traveler Inn is closed. Opens at 5:00 PM." | A closed building's button gets its "closed" message attached with `addEventListener`; when it opens, the update adds an `onclick` to enter on top of it, so a click does both. | Replace the handler when a building opens or closes, so there's only ever one. **Fixed:** an opening or closing building gets a fresh button (`buildingButton`), which also fixes the stale "opens 5:00 PM" label. |
| B3 | A rented room stays rented after a night's sleep. | A rental lasts "through the end of the next day", and sleeping doesn't end it. | Sleeping ends the night's rental at check-out in the morning (breakfast still served). **Fixed:** sleeping ends the rental at 10 AM (`CheckoutTime`); the wake-up message says so. |
| B4 | Quest steps tick late: the tracker catches up a while after the action. | The tracker refreshes on a throttled state change (2 s), and actions outside the game-action route (shop buy, preparing a spell, saving) don't report quest progress back at all. | Refresh the tracker at once whenever a response carries quest news, and have those endpoints report it too. **Fixed:** shop buys, spell preparation and saving return `quests_completed` / `quests_advanced` (one client helper, `announceQuestNews`), and the tracker's throttle now catches up instead of dropping a change. |
| B5 | A daily was completed just by talking to the vault keeper: no turn-in, XP appeared only after leaving. The tutorial's last step did the same. | A final "talk to them" objective completes the moment the conversation opens (`NPCTalked`), and rewards are only a floating number. | See **T1**: explicit turn-ins and a rewards screen. |

### Gear screens

- **G1** The Barbarian's first choice (19 single weapons) is one long column. The option-list
  rule made its grid shrink to one card's width. Fix: single-item choices get the full width and
  wrap into rows.
- **G2** Mixed choices (a few bundles plus a list of single weapons, the Fighter's 2 of 2) are a
  mess: bundles along the bottom, singles in a narrow column. Fix: lay a choice out as sections,
  bundles in a row, then "or one of these" with the singles in a grid.
- **G3** Rarity dots aren't needed here: remove them from the gear cards.
- **G4** Put the important facts on the card itself instead of behind "?": damage dice and type
  for weapons (1d8 slashing), AC for armor, and the key tags (two-handed, finesse, light, thrown,
  versatile). "?" stays for the full panel.
- **G5** Moving between screens still isn't smooth. Fix: a short cross-fade of the panel, with no
  layout jump.

**G1–G5 built** (not yet seen in the browser): a choice is laid out in sections (`.gear-bundles`
side by side, then "or one of these" and a full-width `.gear-grid` of single items; weapon lists
and packs use the same). The cards have no rarity dots and show the facts: damage dice and type,
range for ranged weapons, AC (armor sets: total AC and the Str they need), and two-handed /
versatile (with its die) / finesse / light / thrown / reach / loading. "?" still opens the full
panel. Between screens the vault stays up and only the panel cross-fades (180 ms). It used to
hide the whole scene, so each screen dropped to black.

### Flow: too much, all at once

- **F1** It feels like information overload: a tip pops up while an NPC is talking, and so on. The
  rations explanation got missed entirely.
- **F2** It needs to be woven in more organically.

**F1/F2 built:**
- Tips are pulled: the tracker shows a 💡 while the step has a tip.
- Only `tip_push` stages (your-vault, supplies, a-bed-for-the-night) pop up by themselves, and never over a conversation, a fight, a POI, the tour or another window.
- Each city's shopkeeper ("I'm new in town. What do I need for the road?": rations, how to eat them, how to buy) and innkeeper ("How does a room work?": one night, sleep after 9, breakfast, check-out at 10) say it in their own voice (`dialogue.with` in the six tutorial quests).
- The rewards screen and tips wait for each other.

Plan for F1 and F2 (approved: "try what you want"):
- **Tips become pull, not push.** The quest tracker carries a 💡 when the current step has a "how
  it works" note; clicking it opens the note. Only three are pushed, and only when nothing else is
  on screen: the vault, the shop and saving.
- **Say it in the world first.** The keeper's lines (and the shopkeeper's, the innkeeper's) carry
  the explanation in voice. Grokmar mentions rations when you browse; the innkeeper tells you
  about breakfast and the morning check-out. The tip is the reference, not the first telling.
- **Never two things at once:** nothing pops up while a conversation, a window or the tour is
  open; it waits, one at a time.

### Quests: turn-ins (decided)

- **T1** Every quest needs a deliberate turn-in and a rewards screen, OSRS-style. The final "talk
  to them" step completes only when you choose **Turn in** with the giver (not by opening the
  conversation). A rewards screen then shows the quest's name, "Quest complete!", and the XP,
  gold, items and quest points, before you carry on. That goes for the tutorial, side quests and
  the dailies and weeklies.
  **Built.** A `talk` objective on a quest's last stage is the turn-in: greeting the NPC doesn't
  meet it. When the rest of the step is done, the NPC offers **📜 Turn in: <quest>**, and choosing
  it finishes the quest (`events.TurnedIn`, checked server-side with `quest.ReadyToTurnIn`). The
  giver then says their piece (the quest's `completed` conversation). A **Quest complete!** screen
  (`src/ui/questComplete.js`) shows the name, XP, gold, items and quest points once the conversation
  ends. Quests that end in a fight or a POI step get the screen too: combat end and POI responses
  now carry the quest news. The tracker reads "📜 … (Turn in)". Tests: `tests/api/turn_in_test.go`;
  the playthrough tests now turn in.
- **T2** Dailies and weeklies are offered and turned in at **vault keepers**. There's no board and
  no innkeeper involvement. (Docs, roadmap and memory corrected.)

### Audio

- **A1** Music changes cut hard: fade out and in between tracks (about a second each way).
  **Built:** a new track fades in over a second while the old one fades out (`musicSystem.js`).

### Waiting

- **A2** The Wait window may not be accurate, and could be better. To look at: what it shows
  against what actually passes (hunger, fatigue, shops opening), and making it clearer.
  **Fixed.** The window guessed in the browser, and most of it was wrong:
  - Fatigue showed +1 an hour, but waiting freezes fatigue (only sleep, or an item, lowers it).
  - Hunger used one drop every 3 hours. The real rate is every 4 hours (6 when Stuffed), and it
    carries on from the last tick. The names were also the old ones (Famished / Full).
  - It didn't show the healing: waiting restores HP and mana, 1/8 of the maximum an hour.
  - It also didn't show starving damage, effects wearing off or setting in, spells finishing
    preparation, buildings opening or closing, or that waiting out on the road can bring an
    encounter (and keeps you walking if you haven't stopped).
  The window now asks the server (`/api/game/wait-preview`). It runs the real wait on a copy of
  the save for each length from 15 minutes to 6 hours, so it can't drift from the rules
  (`tests/api/wait_preview_test.go` checks the preview against a real wait). Waiting and
  sleeping are also refused mid-fight now. The "finished preparing" message names the spell,
  not its id, and the tour's fatigue line says waiting holds fatigue and sleep clears it.

### Balance (open since playtest 1)

- **W1** Starting kits are over the carrying limit: an Orc Paladin in chain mail with the
  Explorer's Pack carries about 129 lb against 121 (Strength × 6 + 25 for the backpack), before
  the gold (20 lb per 1000). 5e uses Strength × 15. Decide: raise the multiplier, make coins
  weightless, or leave it. **Decided: leave it as is.**

## Playtest 1 (2026-10-22): Thorin and the first run

Fixed in the second pass (see intro-and-tutorial.md §8, "Second pass"): the backpack that
couldn't take anything ("invalid backpack"), the game loading twice from on-demand chunks (the
doubled music), the letter scene, the header, the settings buttons, held time showing ⏳, Cast (in
combat), the gear screens' size and item panel, gold and the stone moved to the vault, the letter
as step one, and the first set of tips.
