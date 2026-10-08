# Character slots — divergent rosters and an earned free build

Written 2026-10-08. Design agreed with the maintainer. Separate from the Nostr identity
work; they only touch at the saves screen.

## The idea

A player's roster should feel like a set of genuinely different people, not five rolls of
the same dice. So each derived character is generated from the pool with the previous
characters' defining traits **removed**, and the last slot stops deriving altogether:

| Slot | Unlocks at | How the character is made |
|---|---|---|
| 1 | — | derived from the npub, as today |
| 2 | level 5 | derived, excluding slot 1's race / class / background |
| 3 | level 10 | derived, excluding slots 1–2's |
| 4 | level 15 | derived, excluding slots 1–3's |
| 5 | level 20 | **player-authored.** The reward for taking someone to 20 |

Unlock level is the highest level reached on any of that player's characters.

**Excluded traits: race, class, background.** Alignment stays free — it is flavour, and
reusing it does not make two characters feel alike.

Because every derived slot is a pure function of the npub and the slots before it, the
whole roster is **pre-computable**: the saves screen can show a player exactly who waits
in slot 3 long before they unlock it.

## Two problems this fixes

**Every slot currently generates the same character.** `create.go:137` calls
`gamecharacter.GenerateCharacter(pubKey, &weightDataStruct)` — the slot index never enters
the seed. Slot 2 today is a carbon copy of slot 1.

**Slot unlocking is client-side only.** `www/views/saves.html:124-130` holds the
`unlockLevel` table (0/5/10/15/20) and nothing on the server enforces it. Now that
requests are authenticated as the player, this is exactly the sort of rule that has to
move server-side: a player can otherwise mint slot 5 on day one.

## How generation changes

The existing design takes this well. Generation is already context-seeded:

```go
func CreateDeterministicSeed(hexKey, context string) int64   // sha256(hexKey + context)

GenerateRace(hexKey, w)              // context "race"
GenerateClass(hexKey, w, race)       // context "class_" + race
GenerateBackground(hexKey, w, class) // context "background_" + class
GenerateAlignment(hexKey, w)         // context "alignment"
GenerateStats(hexKey, class)         // context "stat_tier", …
```

So two changes, both local:

1. **Slot enters the context.** `"race"` → `"race#2"`, and so on. Slot 1 must keep its
   present context strings verbatim, or every existing character re-rolls.
2. **Exclusions filter the pools** before `DeterministicWeightedChoice`. Drop the excluded
   options and their weights; the function already handles arbitrary weights, so what is
   left re-normalises naturally.

A `GenerateRoster(hexKey, weightData) [4]types.Character` walks slots 1→4, accumulating
exclusions. Slot 5 is not derived.

### Pool headroom

Verified against `game-data/systems/new-character/generation-weights.json`:

| Pool | Size | After 3 exclusions |
|---|---|---|
| Races | 10 | 7 |
| Classes (per race) | 12, for every race | 9 |
| Alignments | 9 | not excluded |
| Backgrounds (per class) | 5–9 | **as few as 2** |

Backgrounds are the tight one, because the pool is scoped to the character's class
(Barbarian and Druid have 5; Bard has 9) and a later character's class differs anyway, so
overlap is partial. It can never empty, but the rule needs a stated fallback:

> **If applying exclusions would empty a pool, ignore exclusions for that trait.**
> A repeated background is far better than a failed generation.

## Server-side gating

`create-save` must validate, not trust:

- the requested slot is 1–5
- the slot is unlocked: highest level across the player's own saves ≥ the slot's threshold
- the slot is not already occupied
- for slots 1–4, the submitted character **matches what the server derives** for that
  `(npub, slot)` — the client proposes nothing
- for slot 5, the submitted build passes the custom-build rules below

Level comes from XP (already derived — see the hydration rule, roadmap §4), so "highest
level reached" is read off the save files rather than stored.

## Slot 5 — the free build

The reward, so it should feel like one: pick race, class, background, alignment, and
distribute stats. It needs a budget that cannot out-roll a lucky derived character, and
the budget already exists in `GenerateStats`:

- total stat points 79–89, weighted so 82–86 is the common band
- individual stats clamped 10–16
- the class's primary stat at minimum 15

Proposal: give slot 5 a **fixed 84 points** inside the same 10–16 clamp — mid-band, so it
is reliably good without beating a high roll. Every rule validated server-side; the
endpoint is the authority and the UI is a convenience.

*Open:* whether slot 5 may reuse traits its roster already has. Recommendation: yes. It is
the player's own build, and constraining it would undercut the reward.

## Surfacing it

`GET /api/character/slots` returns all five entries: the derived preview for 1–4, the
unlock threshold, whether it is unlocked, and whether a save already occupies it. Slot 5
reports itself as custom rather than carrying a preview.

The saves screen then shows a locked slot as the character actually waiting there —
"Level 10 · Thorin, Dwarf Barbarian" — which turns a grey slot into something to play
toward. `src/logic/characterGenerator.js` already wraps the Go API rather than
reimplementing generation, so there is one implementation to change, not two.

## Order of work

1. Slot in the seed + exclusion filtering + `GenerateRoster`, with tests: slot 1 unchanged,
   slots diverge, same npub always yields the same roster, empty-pool fallback holds.
2. `GET /api/character/slots`.
3. Server-side gating in `create-save`, including the derive-and-compare for slots 1–4.
4. Saves-screen previews.
5. Slot 5 custom build: validation first, then the UI.
