# Scene art backlog

The large background art: towns (`www/res/img/locations/<id>.png`, set as `image` on the city
JSON), roads (`www/res/img/environments/<id>.png`, found by id, no data change) and intro or story
scenes (`www/res/img/scene/`). All of it is **1408×736**.

PixelLab can't make these (its canvas tops out at 400×400), so scenes come from **OpenAI's image
API** through the same codex batch tool as the sprites.

## Workflow

1. **Spec.** A batch JSON with `"provider": "openai"` (template below). Prompts describe the
   place from its location JSON.
2. **Generate.**
   ```
   go run ./cmd/codex --config codex-config.yml --generate-images <spec.json>
   ```
   Candidates land in `<img_dir>/_candidates/<id>/gpt-image-2-v<n>.png` (gitignored). A rerun
   skips variants that exist, so raising `count` only adds new ones.
3. **Review.** A sheet of the candidates next to existing art, to pick from.
4. **Promote.** Copy the pick to its live path; for a town, point the city's `image` at it and
   re-migrate. The image and any data change go in one commit.

```json
{
  "provider": "openai",
  "model": "gpt-image-2",
  "quality": "medium",
  "width": 1408,
  "height": 736,
  "count": 2,
  "img_dir": "www/res/img/locations",
  "jobs": [
    {
      "id": "goldenhaven",
      "prompt": "Match the art style, palette, lighting and level of detail of the reference images exactly … A NEW scene … No text, no UI, no border.",
      "refs": ["www/res/img/locations/ironpeak.png", "www/res/img/locations/millhaven.png"]
    }
  ]
}
```

- **Sizes** must be multiples of 16; 1408×736 is, so images come out at the game's size with no
  cropping.
- **`refs`** (optional) sends existing art as a style reference so new scenes match the old.
  It costs a little more per image.
- **`quality`**: `low` for rough composition passes, `medium` for picks (about 4 cents an image
  at 1408×736, a little more with refs), `high` only for a final pass on a chosen composition.
- **Cost** shows as tokens in the tool's output; the OpenAI usage page has the money. The account
  is prepaid with auto-recharge off and a monthly cap.

## House style (chosen 2026-10-20): 16-bit, early-2000s

The first "modern retro" pass looked too painterly and detailed for the game. The style is **an
authentic 16-bit SNES-era look**, from OpenAI with this prompt and used **raw** (not snapped to a
grid; the raw image read better than the snapped one):

> Authentic 16-bit SNES-era pixel art background, like a scene from Chrono Trigger, Secret of Mana
> or Final Fantasy VI. Very low resolution with big chunky square pixels (the whole image is about
> 350 by 185 pixels, scaled up), a limited palette of about 32 colours, bold dark outlines, simple
> readable shapes, flat shading with a little dithering. No fine detail, no painterly gradients.
> … No text, no UI, no border.

- **Towns: a close three-quarter view of the town square**, not an aerial of the whole place:
  big readable shapes need a close camera.
- **Roads: the land with a road through it, and nobody on it** (no travelling party).
- PixelLab at 352×184 was tried too: true pixel art, but flatter and more cartoony.

- **Towns and roads: from text**, written from the location JSON so the art shows what the game
  says is there (Millhaven's well and the road east).
- **Intro scenes: redrawn from the current image** (decided). The old image goes in as `refs`
  with "keep its composition, render it fresh in this style", so the story beats keep their
  framing.
- **The ten background scenes** are new, built on scene3's room and framing (the caretaker teaching
  the young hero), each dressed and acted for its upbringing.
- **The caretaker ages on purpose.** The background scenes are memories: the hero is a child and
  the caretaker a dark-haired man in his prime (as in scene3). In scene2 the hero is grown and the
  caretaker is the white-bearded old man dying in bed. Keep that split in every new scene.

## Backlog

Status: ⬜ to do · 🎨 candidates ready · ✅ live (16-bit style, promoted 2026-10-20)

### 1. Towns (all 9 ✅)
millhaven v1 (the bare lower-left windmill spar was replaced by a copy of the opposite sail),
kingdom v1, goldenhaven v2 (its own art at last; it used kingdom.png), ironpeak v2, marshlight v1,
verdant v2, dusthaven v2, frosthold v2, saltwind v1. The last three had no art before.

### 2. Roads
| Id | Status | Notes |
|---|---|---|
| merchants-highway | ✅ kept | the current art stays |
| darkwood-forest | ✅ kept | the current art stays (2880×1472) |
| greywind-plains, windswept-hills, shadowmere-wetlands, suncrest-coastlands, sunscorch-desert, frozen-wastes | ✅ | v2 each |
| cragspire-mountains | ✅ for now | v2, but neither candidate is liked: **reroll** |

### 3. The intro (all v2 ✅)
scene1, scene2, scene4, scene7, and scene4letter (with the room blurred behind the page, as before,
so the overlaid letter reads). The ten background memories (`bg-*.png`) replace the shared scene3
in `introductions.json`. **Small fixes to come in each.**

The intro's gear and spells screens (scene4b, scene5, scene5a–d, scene6) keep the old style until
the intro rework cuts them. scene3.png is now unused.

### 4. The caretaker's friends (6, new)
One interior per starting city, behind the gear-selection panel (intro draft §5). Waits on the
friends' names and places.

`friend-kingdom`, `friend-verdant`, `friend-ironpeak`, `friend-millhaven`, `friend-marshlight`,
`friend-goldenhaven`: all ⬜.

### 5. Points of interest (20, new)
`www/res/img/poi/<id>.png`, one per POI draft, from its description. Generated now so the art is
ready; POI screens don't show art yet (wiring it is a small frontend job).

### 6. Later
- **Rooms and building interiors** (inns, shops), if buildings get their own backdrops.
- **Fishing and mining** scenes, from those drafts.

### Budget
About 39 scenes with 2 candidates each is ~80 images: **about $3–4 at medium quality**. A `high`
pass on the picks only, if wanted, is roughly another $5–7.
