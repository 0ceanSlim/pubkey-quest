/**
 * The day as the player feels it: the clock's pace in town vs on the road, and
 * a splash as night falls and as dawn breaks.
 *
 * The rules come from the server (game-data "clock"): town_time_scale, and the
 * night window that pays more combat XP (types.NightStart/NightEnd).
 *
 * The first nightfall a player sees explains what night means; after that it's
 * a short splash that fades on its own. Splashes only fire when the clock runs
 * across the edge — not on load, and not when sleep jumps past it.
 *
 * @module systems/dayCycle
 */

import { smoothClock } from './smoothClock.js';
import { eventBus } from '../lib/events.js';

let clock = { town_time_scale: 1, night_start: 1380, night_end: 300 };
let lastAbs = null;
let watching = false;

// The longest stretch of clock that still counts as time running past an edge
// (a tick or two); anything bigger is a sleep or a wait that jumped over it.
const MAX_STEP = 90;

/** Take the rules from game data. */
export function configureDayCycle(cfg) {
    if (cfg && typeof cfg === 'object') clock = { ...clock, ...cfg };
}

/** In town the clock runs at town_time_scale; on the road at full pace. */
export function setTownPace(inTown) {
    smoothClock.setSpeedScale(inTown ? clock.town_time_scale : 1);
}

function introKey() {
    const npub = window.sessionManager?.getSession?.()?.npub || '';
    return `pq_night_intro_seen:${npub}`;
}

function seenIntro() {
    try { return localStorage.getItem(introKey()) === '1'; } catch (_) { return true; }
}

function markIntroSeen() {
    try { localStorage.setItem(introKey(), '1'); } catch (_) { /* storage blocked */ }
}

function fmt(minutes) {
    const h = Math.floor(minutes / 60) % 24;
    const suffix = h < 12 ? 'AM' : 'PM';
    return `${h % 12 || 12} ${suffix}`;
}

// ─── Splash: a line over the scene that fades on its own ─────────────────────

function splash(title, subtitle, color) {
    const host = document.getElementById('scene-container') || document.body;
    const el = document.createElement('div');
    el.style.cssText =
        'position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center; ' +
        'pointer-events:none; z-index:60; opacity:0; transition:opacity 0.6s ease;';
    el.innerHTML =
        `<div style="font-size:16px; font-weight:bold; color:${color}; text-shadow:0 0 8px rgba(0,0,0,0.9), 2px 2px 0 #000;">${title}</div>` +
        `<div style="margin-top:6px; font-size:8px; color:#e5e7eb; text-shadow:1px 1px 0 #000;">${subtitle}</div>`;
    host.appendChild(el);
    requestAnimationFrame(() => { el.style.opacity = '1'; });
    setTimeout(() => { el.style.opacity = '0'; }, 2400);
    setTimeout(() => el.remove(), 3100);
}

// ─── First night: what night means here ──────────────────────────────────────

function nightIntro() {
    smoothClock.hold('notice');
    const box = document.createElement('div');
    box.style.cssText = 'position:fixed; inset:0; z-index:400; display:flex; align-items:center; justify-content:center; background:rgba(0,0,0,0.55);';
    box.innerHTML = `
        <div style="width:300px; max-width:90vw; background:#1f2433; border-top:2px solid #6a7a9a; border-left:2px solid #6a7a9a; border-right:2px solid #0b0e16; border-bottom:2px solid #0b0e16; box-shadow:0 0 24px rgba(0,0,0,0.8);">
            <div style="padding:3px 6px; font-size:9px; font-weight:bold; color:#fff; background:#2e3a6e;">🌙 NIGHT FALLS</div>
            <div style="padding:10px; font-size:8px; line-height:1.8; color:#d1d5db;">
                <p style="margin:0 0 6px; color:#c7d2fe;">From ${fmt(clock.night_start)} until ${fmt(clock.night_end)} the world is dark.</p>
                <ul style="margin:0 0 8px; padding-left:12px; list-style:disc;">
                    <li><b style="color:#facc15;">Fights pay 25% more XP</b> until dawn, for those bold enough to take them.</li>
                    <li>Most shops have shut and townsfolk have gone home to bed. Come back in the morning.</li>
                    <li>An inn will rent you a room: sleep to rest up and wake to a new day.</li>
                </ul>
                <button data-ok style="width:100%; padding:4px; font-size:9px; font-weight:bold; color:#fff; cursor:pointer; background:#2e3a6e; border-top:2px solid #6a7a9a; border-left:2px solid #6a7a9a; border-right:2px solid #0b0e16; border-bottom:2px solid #0b0e16;">Into the night</button>
            </div>
        </div>`;
    document.body.appendChild(box);
    const close = () => {
        box.remove();
        smoothClock.release('notice');
    };
    box.querySelector('[data-ok]').addEventListener('click', close);
    markIntroSeen();
}

// ─── Watching the clock cross the edges ──────────────────────────────────────

function crossed(fromAbs, toAbs, edge) {
    // Does (fromAbs, toAbs] contain a moment that is `edge` minutes into a day?
    for (let day = Math.floor(fromAbs / 1440); day <= Math.floor(toAbs / 1440); day++) {
        const at = day * 1440 + edge;
        if (at > fromAbs && at <= toAbs) return true;
    }
    return false;
}

function check() {
    const t = smoothClock.getCurrentTime();
    if (!t.synced) return;
    const abs = t.currentDay * 1440 + t.timeOfDay;
    if (lastAbs === null || abs < lastAbs || abs - lastAbs > MAX_STEP) {
        lastAbs = abs; // first look, a reload, or a jump (sleep/wait): no splash
        return;
    }
    if (abs === lastAbs) return;
    if (crossed(lastAbs, abs, clock.night_start)) {
        if (seenIntro()) splash('🌙 Night falls', 'Fights pay more XP until dawn.', '#a5b4fc');
        else nightIntro();
    } else if (crossed(lastAbs, abs, clock.night_end)) {
        splash('☀️ Dawn breaks', 'The night\'s XP bonus is over.', '#fde68a');
    }
    lastAbs = abs;
}

/** Start watching once the game is loaded. */
export function startDayCycle() {
    if (watching) return;
    watching = true;
    lastAbs = null;
    setInterval(check, 1000);
}

eventBus.on('gameStateLoaded', () => startDayCycle());
