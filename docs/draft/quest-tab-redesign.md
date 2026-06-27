# Quest tab redesign (per playtest feedback, 2026-06-26)

The current functional quest tab (commit 7fa1657) is a flat available/active/
completed list with inline Accept buttons. Replace it with a **quest journal**.

## Vision
- **It's a journal/guide, not an accept screen.** You do NOT accept quests from
  the quest tab. A quest is *started in the world* by meeting its start
  condition (talk to the quest-giver NPC, reach a place, a bulletin board, etc.).
  The tab is where you read about quests and track them.
- **Each quest is a clickable row → opens a detail modal** (the "quest guide"):
  - **Available** (not yet started): *how and where to start it* — the start
    hint (e.g. "Speak with Mara at the Wayward Traveler Inn, Kingdom"),
    recommended stats / danger, and the rewards.
  - **In progress**: *what to do next* — current stage description + objective
    checklist with progress, and the rewards.
  - **Completed**: summary.
- **Collapsible category sections**, in order. We have these categories
  (`types.QuestCategory`): **main, side, class, race, daily, weekly** — plus a
  **Completed** section. Active (in-progress) quests appear within their category
  marked "in progress" (the modal then shows the next objective). Order roughly:
  Main → Side → Class → Race → Daily → Weekly → Completed.
- **Fix the scaling** — the current cards are sized wrong for the narrow
  right-rail tab; needs a proper compact layout.

## Acceptance / start (the open dependency)
Because acceptance moves *out* of the tab and into the world, this needs the
**start-condition system** — primarily the **talk-to-NPC quest offer**: when you
talk to a quest-giver NPC, they offer the quest in dialogue and you accept
there. (Start conditions in the schema: `talk`, `explore`, `item`,
`bulletin_board`.) Until that lands there is no way to start a quest, so either
build the talk-offer alongside the redesign, or keep a temporary "Begin" action
in the modal as a bridge.

## Backend support already in place
`GET /api/quests/log` returns active (with objective progress) / completed /
available / QP. The redesign mostly reshapes the frontend; the modal "how to
start" text comes from each quest's `start_condition.start_hint` (already in the
data and returned by the log endpoint as `start_hint`). Recommended stats/danger
come from `QuestData.Recommended` (would need adding to the log payload).
