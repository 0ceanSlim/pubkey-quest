# UI rework — brainstorm and plan

Status: **BRAINSTORM** (started 2026-10-12). This is the M6 presentation milestone, widened
into a full rework that has to work well on desktop and on phones. Brainstorm first, then a
clickable draft, then a build plan with a changeover.

## Where we start

**Today's layout:**
- a fixed 756×503 window, stretched unevenly to fill the screen;
- text at 6–8 px, in the Dogica pixel font;
- four zones:
  - the log on the left (209 px);
  - the scene (347 px);
  - eight tabs on the right (200 px);
  - an action strip along the bottom (125 px), plus a stats bar on top.
- On phones it only works in landscape.

**Already fixed from the roadmap's UI critique (§7):**
- NPC speech shows on the scene;
- buildings have rooms;
- the quest tracker sits over the scene;
- the save ritual has its own window.

**Still open from that critique:**
- the uneven stretch;
- text too small to read;
- a crowded bottom strip;
- mixed UI styles (browser dialogs inside the game);
- big moments (level-up, death, discovery) that don't land.

## What the current screen holds (inventory, 2026-10-12)

**Problems:**

- **Targets are tiny.**
  - Play/pause and Wait are 6 px buttons in the stats block.
  - The quest tracker and headcount are 6 px.
  - Most buttons are 7–9 px.
  - The tab buttons are emoji at 10 px.
- **Some things need a mouse.**
  - These need a right-click, with no touch path:
    - buying in a shop;
    - casting, replacing or removing a spell from its slot;
    - container menus.
  - These are hover-only:
    - effect details;
    - spell names in slots;
    - POI pin names on the travel bar;
    - terrain, HP and why a button is disabled (all `title` tooltips).
- **The canvas distorts.** The whole 756×503 canvas is stretched unevenly, with no phone handling at all. In landscape on a phone, text renders at about 4–7 px.
- **Things are duplicated or dead.**
  - There are two death-save surfaces.
  - `poi-modal.html`, `#message-area` and `#action-text` aren't used.
  - "Log Game State" in Settings is shown to every player.
- **Styles are mixed.** There's a native `confirm()` (Reload from Save), and the shop uses its own grid idiom.

**Keep:**

- pointer-event drag in the inventory, equipment and vault;
- 500 ms long-press menus for items and players;
- the speech box on the scene;
- the combat log's pacing, dice and flair;
- the radial timers on effects, fatigue and hunger;
- the time-of-day art.

**Everything the screen holds today, by area:**

| Area | Contents |
|---|---|
| Time | play/pause, wait, clock, day, time-of-day art |
| Vitals | HP, the class resource, XP, fatigue, hunger, weight, effects |
| Identity | name, level, portrait |
| Where you are | travel D-pad, buildings or rooms, people, the GROUND button, district/room name, the travel screen |
| Talk | speech box and dialogue options |
| Tabs | inventory, equipment, spells/abilities, journal, stats, music, settings, friends; plus logout and bug report |
| Log and chat | the game log, the chat line |
| Social over the scene | quest tracker, party HUD, headcount |
| Modals | shop, vault, container, ground, save, level-up, ability points, level guide, wait, quest, spell/ability detail, spell picker, examine, player card, portrait, show booking, logout, bug report, debug console |
| Combat | target bar, turn strip, badges, grid, D-pad, action/turn buttons, chooser, death saves, victory/defeat |

## Principles (from the inventory)

1. **Every target is at least 44 px on touch and 32 px with a mouse.** That includes time controls, tabs and dialogue options.
2. **Nothing is hover-only or right-click-only.**
   - Every menu opens from a tap or long-press, and also from a visible "⋯".
   - Every tooltip is reachable by tapping to inspect.
3. **One component set:** button, panel/window, tab, bar, chip, menu, sheet, modal, inspect popover. Each is themed once and used everywhere.
4. **Readable sizes.**
   - Body text in a clean font at 15–16 px.
   - Dogica for headings, numbers and buttons, at whole multiples of 8 px.
   - A text-size setting.
5. **Keyboard shortcuts on desktop:**

   | Key | Does |
   |---|---|
   | Space | play/pause |
   | W | wait |
   | I | inventory |
   | J | journal |
   | C | character |
   | M | magic |
   | Enter | chat |
   | 1–9 | dialogue options and combat actions |
   | Esc | close |

## Decided

| Topic | Decision (2026-10-12) |
|---|---|
| Layout | **B: the scene becomes a pixel "stage"** scaled in whole steps (crisp art). Everything around it is a **responsive shell**: real font sizes, panels that rearrange per screen, and a real **portrait phone layout** (scene on top, actions below, a tab bar). |
| Look | **Keep the dark Windows-95 + pixel-art identity, refined:** one set of bevels, colours and spacing used everywhere, and no stray browser dialogs. |
| Text | **Pixel headings, readable body:** Dogica for titles, numbers and buttons; a clean, larger font for dialogue, descriptions and the log. |
| Tabs | **Group them together in the draft.** The roadmap's earlier "tabs stay exactly as they are" decision is reopened for this. |
| Customizing (desktop) | **A Windows 95 desktop.** Every area is a window you can move, resize, minimize and snap. A taskbar has a Start menu, and a few preset layouts give a starting point. |
| Phones | **Two first-class layouts, portrait and landscape.** Some content is **landscape-only story mode**: the intro now, and later in-depth quests and lore-heavy scenes. These play full-screen like the intro and ask the player to rotate. |
| Saving layouts | **They follow you via Nostr:** your layout and UI settings are a small app-data event (kind 30078, `d = pubkey-quest:ui`) on your relays, with a per-device fallback when offline. |
| Dialogue options | **Inside the speech box,** like POI choices already are: one place for all talk, and the area actions stay visible. On phones it's a sheet. |
| Time controls | Still open: they have to work on desktop, phone portrait, phone landscape and story mode. One idea so far: a dial on the scene's corner, with Wait beside it, a copy in the taskbar tray and the space bar on desktop. To be workshopped as piece 1. |

## How we get there (decided 2026-10-12): one piece at a time

No big-bang rewrite. The decisions above are the north star, not a spec to build in one go.

**For each piece:**
1. Workshop a **real-size draft** in the game's own font, colours and bevels, to open on a
   phone and a desktop.
2. Tune it.
3. Ship it into the current UI.
4. Move to the next.

The Windows 95 desktop and the phone layouts come together gradually.

**Candidate pieces** (order to agree; each is small and useful on its own):

1. **Time controls** (play/pause, wait, clock): big, easy to hit, on every screen.
2. **Stop the stretch:** scale evenly and letterbox (no redesign, an immediate gain in sharpness).
3. **Touch paths:**
   - every right-click and hover-only thing gets a tap/long-press and a visible "⋯";
   - tap to inspect tooltips.
4. **Text:** a readable body font and sizes, plus a text-size setting.
5. **The speech box takes the dialogue options.**
6. **The "Here" area:** exits, places and people.
7. **Character and vitals.**
8. **Tabs:** the grouping.
9. **Log and chat.**
10. **Combat on phones.**
11. **Phone portrait and landscape layouts.**
12. **Windows (move, resize, minimize, snap) and the taskbar.**
13. **Story mode** for the intro and later lore scenes.

## Open (being brainstormed)

- **Time controls in every layout.**

- How the eight tabs group.
- Where the log and chat live.
- The world actions: travel, buildings, rooms, people.
- Combat on a phone.
- Big moments.
- The top bar.
