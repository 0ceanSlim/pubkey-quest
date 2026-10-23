# The intro and a first-morning tutorial

Status: **built locally** (2026-10-22), not pushed: to be **played through locally first**. §8
says what was built and where.

## 1. What's there now

The intro (`src/pages/gameIntro.js`, text in `game-data/systems/new-character/introductions.json`)
runs about 18–20 screens before you can play:

| # | Screen | Kind |
|---|---|---|
| 1 | Name | input |
| 2–4 | Rainy night, the caretaker, their final words | story |
| 5–7 | Background intro, letter intro, the background's letter | story |
| 8–9 | Spell knowledge + your starting spells (casters) | display only, no choice |
| 10–12 | "The equipment before you", the class's equipment intro, its quote | story |
| 13 | Items provided | display only |
| 14+ | **One screen per equipment choice**, then a confirmation | choice |
| … | Preparation at dawn, the pack note, **pack choice** | story + choice |
| last | Departure, final text, Begin Journey | story |

Then `/api/character/create-save` builds the save, and you appear in your race's starting city
(center district, outdoors).

**What works:** the story. The rain, the caretaker, the letter written for your background, and
the push out the door all set the tone, and they're short.

**What doesn't:**
- **The choice comes before you've seen the game.** You pick gear before you know what
  encumbrance, AC or a finesse weapon do here.
- **The spells screen is display only.** It lists spells you can't do anything with yet, and
  it's out of sight once you're in the game.
- **Story and menus are interleaved**, which slows both down. Six of the story screens exist
  only to set up the menus ("the equipment before you", "as dawn approaches", "the pack note").
- **The client posts the choices** as a blob of strings and JSON (`equipment_choices`), and the
  server trusts that shape. It works, but it's the one place character creation isn't plainly
  server-driven.

## 2. Two ways to fix it

**A. Keep it all in the intro**, and just trim the story screens around the menus.

**B. Move gear into the game: a first-morning tutorial (chosen).** The intro is only the story.
You arrive in your city with the caretaker's letter, a little gold and your Homeward Stone. The
letter sends you to the caretaker's old friend, who opens **today's gear selection**, unchanged
in what it offers, in their place. Then a few short errands teach the game by doing it.

Why B:
- Gear is chosen **from someone in the world, in a place you can see**, after you've arrived.
- The tutorial teaches what new players currently have to discover alone: moving between
  districts, buildings, talking to NPCs, the inventory, shops, the inn, the spells tab, and
  **saving** (there's no autosave by design, so it must be taught).
- The server checks the choices against `starting-gear.json` when the friend hands them over,
  instead of trusting what the client posts at creation.

The rest of this draft is B.

## 3. The new intro (story only, about 9 screens)

| Keep | Cut |
|---|---|
| Name | Spell knowledge + starting spells (taught in game, §5) |
| Rainy night, the caretaker, final words | "The equipment before you", equipment intros + quotes |
| Background intro | Items provided, every equipment choice screen, confirmation |
| Letter intro, the background's letter | Preparation at dawn, the pack note, pack choice |
| Departure, final text, Begin Journey | |

- **The letter gains a last line, by starting city:** "…When you reach Ironpeak, find
  Brannoc at the Anvil and tell him whose child you are. He owes me more than he'll admit."
  The letter text is per background, so the friend line is a short per-city addition to
  `introductions.json`.
- **A new arrival screen** after departure, using the city's location art: "Ironpeak rises out
  of the mountain…" Five of the six starting cities have art. **Goldenhaven doesn't yet**, which
  puts it at the top of the scene-art backlog.
- The departure text now ends at the road instead of at your pack.
- Some of the cut text is good (the equipment intros' quotes, "Take what suits your
  strength…"). It moves into the friend's dialogue, where it's said to you.

## 4. Arriving: what a new character has

| Item | Now | New |
|---|---|---|
| Starting gold | yes | yes, unchanged |
| Homeward Stone | yes | yes, unchanged |
| **The Caretaker's Letter** | no | **new item**, readable any time (the full letter); shown to the friend |
| Class essentials | given items | only what the class can't work without: the **wizard's spellbook** |
| Weapons, armor, pack, given items | chosen in the intro | **from the friend** (§5) |

You start in the center district as now. Towns have no random encounters, so being unarmed
until you meet the friend is safe.

## 5. The tutorial quest: "The Caretaker's Letter"

Auto-started at creation (quest state lives in the save, as any quest). One quest per starting
city, following the `stone-<city>` pattern: `caretakers-letter-<city>`, six files sharing text.
The quest tracker always shows the next step.

| Step | Where | It teaches | Done when |
|---|---|---|---|
| 1. **Find your vault's keeper** | the vault (center district) | entering a building, talking | you talk to them |
| 2. **Choose your gear** | the friend | reading items before you take them | you confirm your gear |
| 3. **Gear up** | inventory | equipping, the equipment slots, AC and weight | weapon and armor equipped |
| 4. **Supplies** | the general store | buying and selling | you hold rations |
| 5. **Your caretaker's lessons** (casters only) | the spells tab | known spells, preparing, mana and runes | a spell is prepared |
| 6. **A bed for the night** | the inn | renting a room, resting, **saving** | you rest and save |
| 7. **Report back** | the friend | | XP and a nudge toward the first road and a starting quest |

- About 10 minutes. Every step is something the player needs anyway.
- **Skip:** the friend's first line offers "I've done this before. Just the gear, please." That
  opens the gear selection straight away and completes the quest once it's confirmed. Second characters
  and players who already know the game aren't held up.
- **Existing characters aren't touched.** They already have gear, and the quest is only started
  at creation.

### 5b. "Show me around": the UI tour (built 2026-10-22)

Before the quest's first step, a guided tour spotlights every part of the screen with one line on
what it's for: the portrait, name and level, the clock (play, pause, wait), XP, HP, the class
resource, fatigue, hunger, weight, effects; the Quest Log, Stats, Inventory, Equipment and Spells
tabs; Friends, Music, Settings, Report Bug and Quit; where you are, the day, who's here, the quest
tracker; Travel, Buildings and People; the story and chat; and saving (no autosave, signed with
your key). 32 stops, about a minute; Back, Next, Skip, arrow keys and Esc.

- `src/ui/uiTour.js`. It runs once for a brand-new character (no XP, day 1) once the first-save
  prompt and the what's-new popup are out of the way, and any time from **Settings → 🧭 Show me
  around**. Time stops while it runs.
- Seen-or-not is per browser (localStorage), not save data.
- The tutorial quest then teaches by doing what the tour pointed at: the Inventory and Equipment
  tabs at step 3, the store at step 4, the Spells tab at step 5, Settings → Save at step 6.

### The friend and the gear selection
- **The "friend" is your people's vault keeper** (workshop proposal, §5a): one per starting
  city, already in the game, which suits that the start differs by race.
- **The gear selection stays as it is**: the same choices from `starting-gear.json`, the same
  given items, the same pack choice, the same item cards and info buttons. What changes is where
  it's shown and how it's framed.
- **The screens are redesigned to sit inside a scene image**, the keeper's vault, instead of the
  plain dark background:
  - The scene fills the screen; the choices sit in a framed panel over it (win95 bevel, the
    game's panel style), sized so the scene still shows around it.
  - The friend's line for each choice is the panel's heading, taken from today's equipment
    intros and quotes ("Take what suits your strength…").
  - Back and Continue stay; the confirmation becomes the friend's "Is that everything?".
  - On a phone the panel takes the width and scrolls; the scene stays behind it.
- **Scene art needed:** one vault interior per keeper (6), from the scene-art backlog. They're
  also the vault's own backdrop from then on.
- Opened by a dialogue action on the friend (`open_starting_gear`), like `open_shop` opens a
  shop. It can be used once: after it's confirmed the option is gone.

### 5a. Workshop: who the friend is, and the lore behind it

**The idea: the caretaker left you an inheritance in trust at your people's vault.** Under the
Sixfold Compact (`docs/draft/world-lore.md` §4) each people keeps its wealth in its own vault under
its own law. For eighteen years the caretaker put by what they could, in your name, at the vault of
your people. The letter's last line sends you to its keeper, who **knows you by your true sigil**,
the mark every soul is born with that can't be forged (the lore's reason your character is fixed by
your key). The caretaker registered it when you were a baby.

Why the keepers:
- **They already exist, one per starting city**, with names, homes and a full-day schedule
  (always at the vault, so a new character arriving at noon can't miss them).
- **They're already the people who know you.** Creation gives you your home keeper's acceptance
  and your Homeward Stone; this makes them the start of your story.
- **Every vault is in the center district**, where you arrive, so step 1 is short and certain.
- **It teaches the vault** (a system new players don't find on their own) and ties the Compact,
  the sigil and the waystones into the first ten minutes.
- No new NPCs to write, place and schedule; the keepers just get a "first meeting" branch.

**The six, and the letter line that sends you to each** (the caretaker's voice):

| Start (races) | Keeper | The letter says | In person |
|---|---|---|---|
| **The Royal Kingdom** (Human, Half-Elf, Half-Orc, Tiefling) | Marcus Goldvault, Royal Custodian, Vault of Crowns | "Marcus Goldvault keeps it under my name in the Vault of Crowns. His ledgers will show every coin, and he'll show you them whether you ask or not." | Precise, proud of his ledgers. Treats every race the same by the Crown's law, and says so pointedly to a Half-Orc or Tiefling who expects otherwise. |
| **Verdant** (Elf) | Silvanthir Moonbough, Warden of Roots, Glade of Safekeeping | "Silvanthir sang it into the roots of the Glade. Ask the old tree for it. It will answer to your name." | Ancient and slow, speaks to the trees. Mentions that the waystones' song is fading, the first hint of the main quest. |
| **Ironpeak** (Dwarf) | Thorin Ironledger, Vaultwright, the Stonevault | "Thorin Ironledger carved the stone your share sits behind. Tell him the old debt is settled. He'll know." | Gruff, counts twice. The "old debt" is never fully explained (a hook). Grumbles about Goldenhaven taxing the river ore. |
| **Millhaven** (Halfling, Gnome) | Rosie Lockwood, Keywarden, the Burrowlock | "Rosie Lockwood will have kept it dry and counted, and probably added a jar of preserves." | Warm, chatty; she does add the preserves (a ration or two). |
| **Marshlight** (Orc) | Grommash Ironhide, Hoardkeeper, the Warhoard | "Grommash Ironhide holds it in the Warhoard. The clans keep their word on a hoard, even to an outsider like me." | Scarred, blunt, unexpectedly kind. Speaks for the outcasts the other cities squeezed out. |
| **Goldenhaven** (Dragonborn) | Zarithax the Golden, Scalekeeper, the Ember Vault | "Zarithax will charge you a fee for keeping my coin. Pay it. It means they kept it honestly." | Imperious; waives the fee for a dragonborn, with a remark about who won the peace. |

What the caretaker's line *doesn't* depend on: the background. The letter body stays per background;
only this last line is per city, so 10 letters × 6 lines, not 60 letters.

**The tutorial in each city** (the same seven steps, the places are the city's own):

| City | 1–2. The keeper | 4. Supplies | 6. The inn | Lore note on the way |
|---|---|---|---|---|
| Kingdom | Vault of Crowns (center) | Grand Market (center) | The Wayward Traveler (south) | the Royal Palace, the Mage Tower for casters |
| Verdant | Glade of Safekeeping (center) | Greenleaf Provisions (west) | The Moonlit Glade (north) | the fading waystone in the plaza |
| Ironpeak | the Stonevault (center) | Ironhand's Supplies (south) | The Pickaxe & Pint (center) | ore carts bound for Goldenhaven |
| Millhaven | the Burrowlock (center) | Haversham's General Store (center) | The Grain & Grape (east) | the windmills, the hills |
| Marshlight | the Warhoard (center) | Swamp Treasures (center) | The Glowing Lantern (east) | the stilt walkways, Shadowmere's dark |
| Goldenhaven | the Ember Vault (center) | Goldscale Trading House (west) | Caravan Rest Inn (west) | the Merchant Princes, the river port |

**Step 7, report back:** the keeper hands you the last of the inheritance (a little gold or XP) and
points you at work: the vault keepers' bounties (the daily and weekly roll) and your city's waystone,
"fading, like they all are."

**Open questions for the workshop:**
1. Keepers as the friend (above), or new NPCs with a personal history with the caretaker?
2. The letter lines and personalities: keep, change, or more of a backstory for the caretaker's
   tie to each keeper (Thorin's "old debt", for one)?
3. Should part of the inheritance sit **in the vault** (say the pack, or a keepsake) so the
   tutorial teaches withdrawing, or is all of it handed over?
4. Kingdom serves four races. Should Marcus have a line for each (the Half-Orc and Tiefling
   ones matter most, given how they're treated elsewhere)?
5. Step 7's pointer: the vault keepers' bounties, the waystone, or both?

### Engine work
Most of this is data on the dialogue and quest systems that already exist. The new pieces:
1. **Claiming the gear.** A new endpoint takes the same `equipment_choices` and `pack_choice` the
   creation call takes today, checks them against the class's `starting-gear.json`, and grants
   them through the existing `buildInventoryFromChoices`. Once only: it's recorded in
   `DialogueGifts` (`<friend>:starting_gear`), so no new save field.
2. **Start a quest at creation**, from `starting-locations.json` per city.
3. **Two quest step checks:** "has this equipped" (step 3) and "has a spell prepared" (step 5).
   Both read existing save state; nothing new is stored.
4. **Character creation** stops reading `equipment_choices` and `pack_choice` (they move to the
   claim, step 1) and gives the letter instead. The old path stays readable for one release in case an old client is open.
5. **The intro page** loses the selection screens and the spells screen. `equipmentSelection.js`
   moves into the game page, restyled to the in-scene panel; `showStartingSpells` goes.
6. **The Caretaker's Letter item**: a "Read" action that shows the letter (the text comes from
   `introductions.json` by background), plus a sprite.

## 6. Testing locally before pushing

Nothing in this goes to `main` until it's been played through locally.
- Build in order: the friend NPCs + the quest (data), the engine pieces (§5), then the
  intro cut.
- Each new character needs an npub without a save. Test with a throwaway key, or delete the
  local save under `data/saves/<npub>/`.
- Play through all six starting cities: the arrival art, the friend, every step, Skip.
- Check one caster and one non-caster (step 5 shows only for casters).
- Check that an existing save loads unchanged, with no quest added.
- `go test ./...`, `codex --check-schema` for the quests, and `--validate`.
- Push only when you've played it and say so.

## 7. Decisions

Taken (2026-10-20):
1. **B:** the intro becomes story only; gear is chosen in game from the caretaker's friend,
   inside a tutorial quest.
2. **Gear selection stays as it is** (the 5e choices, given items, pack choice); only the place
   moves, and the screens are redesigned to sit inside the friend's scene image.
3. Starting spells: taught in the spells tab during the tutorial; the intro screen goes.
4. One new friend NPC per starting city.
5. A Skip option at the friend.
6. An arrival screen with the city's art (Goldenhaven's needed).

## 8. As built (2026-10-22)

The workshop's open questions (§5a) went with the recommendations: the keepers are the friend,
the inheritance is all handed over, step 7 points at both the vault keepers' bounties and the
fading waystone.

- **Intro** (`src/pages/gameIntro.js`): name, rainy night, the caretaker, final words, the
  upbringing's memory (`bg-*.png`), the letter intro, the letter (its last line from
  `introductions.json` `letter_closings` by starting city), departure, arrival (`arrivals`, the
  town's art), final text. Creation posts no gear.
- **Creation** (`api/character/create.go`): gold, the Homeward Stone, `caretakers-letter`, and
  `quest.StartForNewCharacter` on `caretakers-letter-<city>`. A client that still sends gear picks
  gets the old path, without the tutorial.
- **Quests** (`game-data/quests-drafts/main/caretakers-letter-<city>.json`): six, one per starting
  city, `start_condition.type: "creation"` (never offered, never listed as locked). Stages: talk to
  the keeper, claim the inheritance, buy something, prepare a spell (`classes`: the six level-1
  casters; skipped for anyone else), sleep at the inn and save, report back (150 XP, 100 gold;
  Rosie adds 2 rations).
- **Engine**: a `do` objective type with `types.DoActions` (`claim-starting-gear`, `shop-buy`,
  `prepare-spell`, `sleep-inn`, `save-game`), recorded as `events.Did` where each happens;
  `QuestStage.Classes`; `quest.FinishNow` for the skip.
- **The keeper's lines** are the quest's `active` dialogue, gated by stage. `open_starting_gear`
  and `open_starting_gear_skip` are quest dialogue actions that open the gear selection.
- **The claim** (`claim_starting_gear`, `api/game/starting_gear.go` → `character.ClaimStartingGear`):
  once (`DialogueGifts` `starting_gear`), only during the tutorial, and every pick is checked
  against the class's `starting-gear.json` options (the old creation path trusted any JSON item
  list). Gear is laid out as creation did it; what the character carried is put back.
- **The gear screens** (`src/ui/startingGear.js` + `systems/equipmentSelection.js`): the intro's
  screens, unchanged in what they offer, in a full-screen overlay over the keeper's vault
  (`/res/img/scene/vault-<city>.png`, or the town's art), with "Not now" to come back later.
- **The letter**: right-click → Read (`ui/letterReader.js`), the intro's parchment scene.
- **Tests**: `tests/quest/tutorial_test.go`, `tests/character/starting_gear_test.go`,
  `tests/api/tutorial_test.go` (creation → keeper → claim, once; a forged claim; the skip).

### Second pass (after the first playthrough, 2026-10-22)

- **Steps now:** read the letter (`read-letter`, the Read action), talk to the keeper, take the
  inheritance, **take your gold and Homeward Stone from the vault** (fetch objectives; the keeper's
  "Could you open my vault?" is a quest-dialogue `open_vault` action), buy supplies, prepare a
  spell (casters), sleep and save, report back.
- **Gold and the stone start in the vault**, the caretaker's savings and the stone they had
  attuned; every keeper explains the stone (`ask-stone`).
- **Tips:** `QuestStage.tip`, sent as `stage_tip`, shown once per stage by `ui/tutorialTip.js`
  (after the tour, never over a vault or shop): items and the tracker, buildings and people, the
  gear screens, the vault, shops and rations, the bag and containers, preparing and casting,
  inns and saving.
- **Fixed:** the claimed pack's contents were in a shape the inventory rejected ("invalid
  backpack": nothing could go in it, so buying found no room); worn effects (the backpack's
  capacity) now apply at the claim; on-demand chunks imported `../game.js` while the page loads
  `game.js?v=…`, so opening the inheritance, the letter or a lock **ran the whole game a second
  time** (the doubled music, an empty second game state); the letter reader's text now comes from
  the right state and sits on the page at any window shape (`lib/letterLayout.js`); held time
  shows ⏳; Cast (in combat) is always listed; item values say gold and ranges say squares; the
  letter is a `keepsake` (can't be destroyed).
- **Open:** carrying capacity. An Orc Paladin in chain mail with the Explorer's Pack carries about
  129 lb against 121 (Strength × 6 + the backpack's 25) before the gold, which adds 20 lb per
  1000. 5e carrying capacity is Strength × 15.

