/**
 * "What's new": once per update, after the game loads, a popup with what
 * changed since this browser last saw the notes.
 *
 * The notes are www/res/updates.json (newest first), the same file the
 * homepage Updates tab renders. The newest update the player missed is shown
 * in full; other missed major updates get their summary and a few highlights;
 * minor ones a single line; past four, the rest are a link to the Updates tab.
 *
 * What was seen is remembered per browser (localStorage). A browser that has
 * never seen the notes is treated as a newcomer: marked caught up, no popup.
 *
 * @module ui/whatsNew
 */

import { eventBus } from '../lib/events.js';
import { smoothClock } from '../systems/smoothClock.js';

const SEEN_KEY = 'pq_updates_seen';
const SHOWN_MAX = 4;     // updates listed before "and N older"
const OLDER_HIGHLIGHTS = 3;
const AFTER_LOAD_MS = 1600; // let the loading screen fade first

// First rollout (Oct 2026): every browser is unmarked, so existing testers
// would all count as newcomers and miss the two updates the popup launched
// with. While the last update before the popup existed is at most this many
// entries back, an unmarked browser is treated as having seen it. After the
// next update, newcomers start clean again.
const ROLLOUT_BASELINE = '2026-10-14-shops';
const ROLLOUT_WINDOW = 2;

function getSeen() {
    try { return localStorage.getItem(SEEN_KEY); } catch (_) { return undefined; }
}

function setSeen(id) {
    try { localStorage.setItem(SEEN_KEY, id); } catch (_) { /* storage blocked */ }
}

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** The updates this browser hasn't seen, newest first ([] when caught up). */
export function missedUpdates(updates, seen) {
    if (!updates.length || seen === updates[0].id) return [];
    const i = updates.findIndex((u) => u.id === seen);
    // An id we no longer know (renamed or archived): just the newest.
    return i === -1 ? updates.slice(0, 1) : updates.slice(0, i);
}

function entryHTML(u, full) {
    const shown = full ? u.highlights ?? [] : u.size === 'major' ? (u.highlights ?? []).slice(0, OLDER_HIGHLIGHTS) : [];
    const more = !full || (u.highlights ?? []).length > shown.length;
    return `
        <div style="margin-bottom:10px; padding:8px; background:#1a1a1a; border-top:2px solid #111; border-left:2px solid #111; border-right:2px solid #3a3a3a; border-bottom:2px solid #3a3a3a;">
            <div style="display:flex; align-items:baseline; gap:6px; flex-wrap:wrap;">
                <span style="font-size:${full ? 14 : 11}px;">${esc(u.icon)}</span>
                <span style="font-size:${full ? 11 : 9}px; font-weight:bold; color:#facc15;">${esc(u.title)}</span>
                <span style="font-size:7px; color:#9ca3af;">${esc(u.when)}</span>
            </div>
            ${u.summary ? `<div style="font-size:8px; line-height:1.6; color:#e5e7eb; margin-top:3px;">${esc(u.summary)}</div>` : ''}
            ${shown.length ? `<ul style="margin:4px 0 0; padding-left:12px; list-style:disc; font-size:8px; line-height:1.7; color:#d1d5db;">${shown.map((h) => `<li>${esc(h)}</li>`).join('')}</ul>` : ''}
            ${more ? `<a href="/#updates/${encodeURIComponent(u.id)}" target="_blank" rel="noopener" style="display:inline-block; margin-top:4px; font-size:7px; color:#93c5fd;">Read more →</a>` : ''}
        </div>`;
}

function show(missed, onClose) {
    smoothClock.hold('notice');
    const older = missed.length - SHOWN_MAX;
    const box = document.createElement('div');
    box.id = 'whats-new';
    box.style.cssText = 'position:fixed; inset:0; z-index:2600; display:flex; align-items:center; justify-content:center; background:rgba(0,0,0,0.6);';
    box.innerHTML = `
        <div style="width:420px; max-width:92vw; max-height:84vh; display:flex; flex-direction:column; background:#2a2a2a; border-top:2px solid #6a6a6a; border-left:2px solid #6a6a6a; border-right:2px solid #111; border-bottom:2px solid #111; box-shadow:0 0 24px rgba(0,0,0,0.8);">
            <div style="padding:4px 8px; font-size:10px; font-weight:bold; color:#fff; background:#7c5c12;">📰 WHAT'S NEW${missed.length > 1 ? ` · ${missed.length} updates since you last played` : ''}</div>
            <div style="padding:10px; overflow-y:auto;">
                ${missed.slice(0, SHOWN_MAX).map((u, i) => entryHTML(u, i === 0)).join('')}
                ${older > 0 ? `<a href="/#updates" target="_blank" rel="noopener" style="display:block; font-size:8px; color:#93c5fd; margin-bottom:8px;">…and ${older} older update${older === 1 ? '' : 's'} →</a>` : ''}
            </div>
            <div style="padding:8px 10px; border-top:1px solid #111;">
                <button data-close style="width:100%; padding:5px; font-size:10px; font-weight:bold; color:#fff; cursor:pointer; background:#166534; border-top:2px solid #22c55e; border-left:2px solid #22c55e; border-right:2px solid #14532d; border-bottom:2px solid #14532d;">Back to the adventure</button>
            </div>
        </div>`;
    document.body.appendChild(box);
    const close = () => {
        box.remove();
        smoothClock.release('notice');
        onClose();
    };
    box.querySelector('[data-close]').addEventListener('click', close);
    box.addEventListener('click', (e) => { if (e.target === box) close(); });
}

async function check() {
    const seen = getSeen();
    if (seen === undefined) return; // storage blocked: never nag
    const version = document.getElementById('settings-panel')?.dataset.version || '';
    let updates = [];
    try {
        const r = await fetch(`/res/updates.json?v=${encodeURIComponent(version)}`);
        if (r.ok) updates = (await r.json())?.updates ?? [];
    } catch (_) { return; }
    if (!updates.length) return;
    const newest = updates[0].id;
    let since = seen;
    if (since === null) {
        const base = updates.findIndex((u) => u.id === ROLLOUT_BASELINE);
        if (base < 1 || base > ROLLOUT_WINDOW) {
            setSeen(newest); // a newcomer isn't missing anything
            return;
        }
        since = ROLLOUT_BASELINE;
    }
    const missed = missedUpdates(updates, since);
    if (missed.length) show(missed, () => setSeen(newest));
}

let checked = false;
eventBus.on('gameStateLoaded', () => {
    if (checked) return;
    checked = true;
    setTimeout(check, AFTER_LOAD_MS);
});
