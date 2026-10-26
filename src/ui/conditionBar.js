/**
 * The countdown bar (degradation §2): one thin bar along the bottom of an item
 * slot that counts down whatever the item has left. Food shows the time until
 * it spoils: green while fresh, amber once stale. (Lit lights, wear and charges
 * will use the same bar.)
 *
 * Food stores when it was made (the slot's "t", an absolute game minute); the
 * bar works the rest out from the item's shelf life and the clock, so it moves
 * without any server traffic.
 *
 * @module ui/conditionBar
 */

import { getItemById } from '../state/staticData.js';
import { smoothClock } from '../systems/smoothClock.js';

/** The item's shelf life in minutes (0 when it never spoils). */
function shelfLife(itemId) {
    const item = getItemById(itemId);
    const spoils = item?.spoils ?? item?.properties?.spoils;
    return Number(spoils?.shelf_life) || 0;
}

/** The clock's absolute game minute. */
function nowMinute() {
    const t = smoothClock.getCurrentTime();
    return (t.currentDay ?? 1) * 1440 + (t.timeOfDay ?? 0);
}

/**
 * How a food is doing: tier, minutes left and the share of its life left.
 * Null when the item doesn't spoil or hasn't been stamped yet.
 */
export function foodCondition(itemId, made) {
    const life = shelfLife(itemId);
    if (!life || made === undefined || made === null || made === '') return null;
    const age = nowMinute() - Number(made);
    const left = Math.max(0, life - age);
    const tier = age >= life ? 'spoiled' : age * 2 >= life ? 'stale' : 'fresh';
    return { tier, left, share: left / life };
}

/** "2 days", "9 hours", "40 minutes": the time left, roughly. */
function timeLeft(minutes) {
    if (minutes >= 2880) return `${Math.floor(minutes / 1440)} days`;
    if (minutes >= 1440) return '1 day';
    if (minutes >= 120) return `${Math.floor(minutes / 60)} hours`;
    if (minutes >= 60) return '1 hour';
    return `${Math.max(1, Math.round(minutes))} minutes`;
}

/** The tooltip line for a food ("Fresh · 2 days left"), or null. */
export function conditionText(itemId, made) {
    const c = foodCondition(itemId, made);
    if (!c) return null;
    const tier = c.tier === 'stale' ? 'Stale' : c.tier === 'spoiled' ? 'Spoiling' : 'Fresh';
    return { text: `${tier} · ${timeLeft(c.left)} left`, color: c.tier === 'fresh' ? '#4ade80' : '#fbbf24' };
}

function paint(slotEl) {
    const bar = slotEl.querySelector(':scope > .cond-bar');
    const c = foodCondition(slotEl.dataset.condItem, slotEl.dataset.made);
    if (!c) {
        bar?.remove();
        return;
    }
    let el = bar;
    if (!el) {
        el = document.createElement('div');
        el.className = 'cond-bar';
        el.style.cssText = 'position:absolute;left:2px;right:2px;bottom:1px;height:2px;background:rgba(0,0,0,0.6);pointer-events:none;';
        el.innerHTML = '<div style="height:100%;"></div>';
        slotEl.appendChild(el);
    }
    const fill = el.firstChild;
    fill.style.width = `${Math.round(c.share * 100)}%`;
    fill.style.background = c.tier === 'fresh' ? '#22c55e' : '#f59e0b';
}

/**
 * Put (or take off) the countdown bar on an inventory slot element.
 * @param {HTMLElement} slotEl - the slot (position: relative)
 * @param {string} itemId
 * @param {number|undefined} made - the slot's "t"
 */
export function attachConditionBar(slotEl, itemId, made) {
    if (!slotEl) return;
    if (!itemId || made === undefined || made === null || !shelfLife(itemId)) {
        delete slotEl.dataset.made;
        delete slotEl.dataset.condItem;
        slotEl.querySelector(':scope > .cond-bar')?.remove();
        return;
    }
    slotEl.dataset.condItem = itemId;
    slotEl.dataset.made = String(made);
    paint(slotEl);
}

// The bars creep down with the clock: repaint every few seconds.
if (typeof window !== 'undefined') {
    setInterval(() => document.querySelectorAll('[data-made]').forEach(paint), 3000);
}

// ─── Lights ──────────────────────────────────────────────────────────────────
// A lit light's burn time left (orange), or an unlit lamp's oil (blue), from the
// slot's "b" (minutes) against the item's light.burn. The server burns it down;
// the tick keeps the held slot's "b" fresh in the cached state.

function lightInfo(itemId) {
    const item = getItemById(itemId);
    return item?.light ?? item?.properties?.light ?? null;
}

/** A light's state for its bar and tooltip, or null. */
export function lightCondition(itemId, slot) {
    const light = lightInfo(itemId);
    if (!light || !slot) return null;
    const lit = slot.l !== undefined && slot.l !== null;
    const left = slot.b !== undefined && slot.b !== null ? Number(slot.b) : Number(light.burn);
    if (!lit && (!light.fuel || left >= light.burn)) return null; // an unlit torch, a full lamp
    return { lit, left, share: Math.max(0, Math.min(1, left / light.burn)) };
}

/** The tooltip line for a light ("Lit · 42 minutes left", "Oil · 3 hours left"). */
export function lightText(itemId, slot) {
    const c = lightCondition(itemId, slot);
    if (!c) return null;
    if (c.lit && slot?.s !== undefined && slot?.s !== null) return { text: `Lit, shutter closed · ${timeLeft(c.left)} left`, color: '#fbbf24' };
    if (c.lit) return { text: `Lit · ${timeLeft(c.left)} left`, color: '#fb923c' };
    return c.left > 0 ? { text: `Oil · ${timeLeft(c.left)} left`, color: '#93c5fd' } : { text: 'Dry: refill it with oil', color: '#f87171' };
}

/**
 * Put (or take off) a light's bar on a slot element.
 * @param {HTMLElement} slotEl
 * @param {string} itemId
 * @param {object} slot - the slot data (its "l" and "b")
 */
export function attachLightBar(slotEl, itemId, slot) {
    if (!slotEl) return;
    // A burning light shows its lit sprite (<id>-lit.png); otherwise the plain,
    // unlit one.
    const img = slotEl.querySelector('img');
    if (img && itemId && lightInfo(itemId)) {
        const want = itemSrc(itemId, slot);
        if (img.getAttribute('src') !== want) img.src = want;
    }
    const c = lightCondition(itemId, slot);
    let bar = slotEl.querySelector(':scope > .light-bar');
    if (!c) {
        bar?.remove();
        delete slotEl.dataset.lightTip;
        return;
    }
    if (!bar) {
        bar = document.createElement('div');
        bar.className = 'light-bar';
        bar.style.cssText = 'position:absolute;left:2px;right:2px;bottom:1px;height:2px;background:rgba(0,0,0,0.6);pointer-events:none;';
        bar.innerHTML = '<div style="height:100%;"></div>';
        slotEl.appendChild(bar);
    }
    bar.firstChild.style.width = `${Math.round(c.share * 100)}%`;
    bar.firstChild.style.background = c.lit ? '#f97316' : '#3b82f6';
    slotEl.dataset.lightTip = JSON.stringify(lightText(itemId, slot));
}

/**
 * An item's sprite in a slot: a light that's burning uses its lit art
 * (<id>-lit.png); everything else its plain sprite.
 */
export function itemSrc(itemId, slot) {
    // Lit and shining; a lantern with its shutter closed looks dark.
    const lit = slot && slot.l !== undefined && slot.l !== null && (slot.s === undefined || slot.s === null);
    return lit && lightInfo(itemId) ? `/res/img/items/${itemId}-lit.png` : `/res/img/items/${itemId}.png`;
}

// ─── Wear ────────────────────────────────────────────────────────────────────
// A weapon's or armour piece's condition as a fill level, from the slot's "w"
// (wear taken) against the item's durability_max (degradation §4). The bar
// shows once it has any wear: green while pristine, then amber, orange and
// red. The tooltip and Examine say its condition even while it's unworn.

const WEAR_TIERS = [
    { name: 'Pristine', at: 0.75, color: '#22c55e' },
    { name: 'Worn', at: 0.5, color: '#eab308' },
    { name: 'Damaged', at: 0.25, color: '#f97316' },
    { name: 'Ruined', at: 0, color: '#ef4444' },
];

/** A worn item's tier and the share of its durability left, or null. */
export function wearCondition(itemId, slot) {
    const item = getItemById(itemId);
    const max = Number(item?.durability_max ?? item?.properties?.durability_max) || 0;
    const wear = Number(slot?.w) || 0;
    if (!max) return null;
    const left = Math.max(0, 1 - Math.min(wear, max) / max);
    const tier = WEAR_TIERS.find((t) => left >= t.at) ?? WEAR_TIERS[3];
    return { tier, left, worn: wear > 0 };
}

/** The tooltip line for a worn item ("Worn · 61%"). */
export function wearText(itemId, slot) {
    const c = wearCondition(itemId, slot);
    return c ? { text: `${c.tier.name} · ${Math.round(c.left * 100)}%`, color: c.tier.color } : null;
}

/** Put (or take off) the wear bar on a slot element. */
export function attachWearBar(slotEl, itemId, slot) {
    if (!slotEl) return;
    const c = itemId ? wearCondition(itemId, slot) : null;
    let bar = slotEl.querySelector(':scope > .wear-bar');
    if (!c) {
        bar?.remove();
        delete slotEl.dataset.wearTip;
        return;
    }
    slotEl.dataset.wearTip = JSON.stringify(wearText(itemId, slot));
    if (!c.worn) {
        bar?.remove();
        return;
    }
    if (!bar) {
        bar = document.createElement('div');
        bar.className = 'wear-bar';
        bar.style.cssText = 'position:absolute;left:2px;right:2px;bottom:1px;height:2px;background:rgba(0,0,0,0.6);pointer-events:none;';
        bar.innerHTML = '<div style="height:100%;"></div>';
        slotEl.appendChild(bar);
    }
    bar.firstChild.style.width = `${Math.round(c.left * 100)}%`;
    bar.firstChild.style.background = c.tier.color;
}

// ─── Charges ─────────────────────────────────────────────────────────────────
// Uses left (degradation §7): the slot's "u" counts uses spent against the
// item's charges.max. A waterskin's drinks, a bottle of ink's scrolls, a
// whetstone's sharpenings. The bar shows once one is spent.

/** An item's charges and how many are left, or null. */
export function chargeCondition(itemId, slot) {
    const item = getItemById(itemId);
    const charges = item?.charges ?? item?.properties?.charges;
    const max = Number(charges?.max) || 0;
    if (!max) return null;
    const left = Math.max(0, max - (Number(slot?.u) || 0));
    return { left, max, noun: charges.noun || 'uses', share: left / max };
}

/** The tooltip line ("3 of 5 drinks", "Empty: refill it"). */
export function chargeText(itemId, slot) {
    const c = chargeCondition(itemId, slot);
    if (!c) return null;
    if (c.left === 0) return { text: 'Empty: refill it at water', color: '#f87171' };
    return { text: `${c.left} of ${c.max} ${c.noun} left`, color: c.share > 0.25 ? '#93c5fd' : '#fb923c' };
}

/** Put (or take off) the charges bar on a slot element. */
export function attachChargeBar(slotEl, itemId, slot) {
    if (!slotEl) return;
    const c = itemId ? chargeCondition(itemId, slot) : null;
    let bar = slotEl.querySelector(':scope > .charge-bar');
    if (!c) {
        bar?.remove();
        delete slotEl.dataset.chargeTip;
        return;
    }
    slotEl.dataset.chargeTip = JSON.stringify(chargeText(itemId, slot));
    if (c.left === c.max) {
        bar?.remove();
        return;
    }
    if (!bar) {
        bar = document.createElement('div');
        bar.className = 'charge-bar';
        bar.style.cssText = 'position:absolute;left:2px;right:2px;bottom:1px;height:2px;background:rgba(0,0,0,0.6);pointer-events:none;';
        bar.innerHTML = '<div style="height:100%;"></div>';
        slotEl.appendChild(bar);
    }
    bar.firstChild.style.width = `${Math.round(c.share * 100)}%`;
    bar.firstChild.style.background = c.share > 0.25 ? '#3b82f6' : '#f97316';
}

// ─── Examine ─────────────────────────────────────────────────────────────────

/**
 * Everything a slot's bars say about its item, as labelled lines for the
 * Examine panel: freshness, light, condition and charges.
 */
export function conditionLines(itemId, slotEl) {
    if (!slotEl) return [];
    const lines = [];
    const fresh = conditionText(itemId, slotEl.dataset.made);
    if (fresh) lines.push({ label: 'Freshness', ...fresh });
    for (const [key, label] of [['lightTip', 'Light'], ['wearTip', 'Condition'], ['chargeTip', 'Charges']]) {
        if (!slotEl.dataset[key]) continue;
        try {
            const tip = JSON.parse(slotEl.dataset[key]);
            if (tip) lines.push({ label, ...tip });
        } catch (_) { /* a stale tip: skip it */ }
    }
    return lines;
}
