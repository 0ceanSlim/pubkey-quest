# Gems — design draft

Status: **DRAFT, for discussion** (2026-10-08). Source data: `game-data/systems/gems.json`
(not read by any code yet). Only one gem item exists: `rough-gem` (placeholder, 500 copper).

## Why this matters now

The Ember Vault keeper (`npcs/goldenhaven/scalekeeper.json`) wants **40 gold + 2 uncut gems**
(`rough-gem`). The only sources are the **Ironvein Seam** mining POI (Cragspire, 8% per dig)
and the **Buried Strongbox** POI, so a player in Goldenhaven effectively can't register there.

## What the draft already says

- **20 gem types** in 5 tiers: basic (quartz, malachite… 1,000c) → common (onyx 5,000c,
  amethyst 10,000c) → uncommon (topaz 50,000c) → rare (ruby/sapphire/emerald 100,000c) →
  legendary (diamond 500,000c). Values are copper, i.e. D&D gp ×100, and match the 5e
  gemstone tables.
- **Size** multiplies value: tiny ×0.01 (42%), small ×0.1 (32%), medium ×1 (18%),
  large ×10 (6%), huge ×100 (2%).
- **Drops:** 50% nothing, else roll type (better with CR: ×1.2 at CR 1–2 … ×5 at CR 16+),
  then size.
- **Uses:** sell for value, special purchases (vault rites), future socketing, collections.

## The structural problem

Every item today is one static JSON definition with one weight and one value, and inventory
slots are `{item, quantity}`. A gem's worth and weight depend on **type × size** (and, by
the vault rite, **uncut vs cut**). That's 20 × 5 × 2 = 200 combinations.

### Option A — generated variant items (recommended)

Migration expands `gems.json` into real items, e.g. `gem-ruby-small-uncut`, each with its
own computed value and weight, the type's sprite, and tags `gem`, `gem-ruby`, `uncut`,
`size-small`, `tier-rare`.

- **Everything just works:** stacking (only identical gems stack), weight, shop prices,
  death keep-3, loot tables, the save format (still an id + quantity — hydration-friendly).
- **No hand-written files:** the 200 items are generated, not authored; `gems.json` stays
  the single source of truth. One sprite per type (×2 for cut/uncut) via the `image` field.
- **Requirements by tag:** the vault rite asks for "2 items tagged `uncut` + `gem`" rather
  than a specific id. Needs a small addition: `consume_items` / `items` requirements that
  match by tag.
- **Cost:** a bigger items table, and the inventory/shop UI should group them sensibly.

### Option B — item instances with properties

One `ruby` item; the slot carries `{item: "ruby", size: "small", cut: false}`. More
flexible, but every system that reads slots (stack, move, weight, pricing, shops, vault,
death, loot placement, save schema, the save-size budget) has to learn instance properties.
Far more surface area for the same result.

## Proposed slices (Option A)

1. **Unblock now:** a source of `rough-gem` near Goldenhaven. Options: a small chance in
   early monster loot (goblin, kobold, bandit), and/or stock at the **Mountain Ore Exchange**
   (Goldenhaven north — on theme, river ore trade).
2. **Generator:** migration expands `gems.json` → variant items; `rough-gem` becomes an alias
   or is replaced by the basic-tier uncut variants.
3. **Tag requirements:** dialogue `requirements.items` / `consume_items` accept
   `{tag: [...], quantity}`; the vault rite switches to "any 2 uncut gems".
4. **Monster drops:** a shared gem roll appended to every combat loot roll, using the CR
   scaling (replaces per-monster hand-placed gems).
5. **Cutting** (later): a jeweller turns uncut → cut for a fee, raising value; cut gems feed
   socketing/crafting when those exist.

## Open questions

- Should "uncut" also lower value vs cut (e.g. uncut = 50%)? The rite implies cut gems exist.
- Weight per size? (Proposal: tiny/small 0.01 lb, medium 0.05, large 0.2, huge 1.0.)
- Drop rate feel: 50% chance on every kill seems high for the economy at value ×size; maybe
  25%, with big sizes rarer at low CR.
- Is "quality" (flawed / fine / flawless) a third axis, or is size enough?
