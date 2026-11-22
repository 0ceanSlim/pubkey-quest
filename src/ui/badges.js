/**
 * Badges and highscores — the journal's 🏆 Badges and 🏅 Highscores tabs, and
 * the "The Dungeon Master awards you…" popup a save that earns a badge brings.
 *
 * Badges come from /api/badges (every badge, earned or not, with progress);
 * a series (the level rank) shows only its highest earned, and the next one to
 * earn. Highscores come from /api/highscores, one board at a time. The server
 * judges everything at each kept save (docs/draft/competition-plan.md); this
 * only draws it.
 *
 * @module ui/badges
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';

const $ = (id) => document.getElementById(id);
const esc = (s) =>
    String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

const GROUPS = [
    ['Main story', (b) => b.rim === 'gold'],
    ['Side quests', (b) => b.rim === 'silver'],
    ['Class and race', (b) => b.rim === 'bronze'],
    ['Collections', (b) => b.rim === 'gems'],
    ['Milestones', (b) => b.id.startsWith('dailies-')],
    ['Level', (b) => b.rank === 'level'],
];

let badgeCache = null; // the last /api/badges answer
let board = 'xp';

// ─── Badges ─────────────────────────────────────────────────────────────────

export async function loadBadges() {
    const view = $('qj-badges-view');
    if (!view) return null;
    try {
        const res = await fetch(`${API_BASE_URL}/badges?npub=${encodeURIComponent(gameAPI.npub)}&save_id=${encodeURIComponent(gameAPI.saveID)}`);
        const json = await res.json();
        if (!json.success) throw new Error('no badges');
        badgeCache = json.data;
        renderBadges(view, json.data);
        return json.data;
    } catch (e) {
        logger.warn('Badges failed to load:', e);
        view.innerHTML = '<div class="qj-empty">Badges couldn\'t be loaded.</div>';
        return null;
    }
}

// shown drops a series (the level rank) down to its highest earned and the
// next one to earn.
function shown(list) {
    const ranks = {};
    for (const b of list) if (b.rank) (ranks[b.rank] ||= []).push(b);
    const keep = new Set(list.filter((b) => !b.rank).map((b) => b.id));
    for (const series of Object.values(ranks)) {
        const earned = series.filter((b) => b.earned_at);
        if (earned.length) keep.add(earned[earned.length - 1].id);
        const next = series.find((b) => !b.earned_at);
        if (next) keep.add(next.id);
    }
    return list.filter((b) => keep.has(b.id));
}

function tile(b) {
    const got = !!b.earned_at;
    const prog = !got && b.of ? `<div class="qj-bprog"><div style="width:${Math.round((100 * b.have) / b.of)}%"></div></div>` : '';
    const tip = `${b.name}: ${b.description}${!got && b.of ? ` (${b.have}/${b.of})` : ''}`;
    return `<button class="qj-badge ${got ? 'got' : ''}" data-badge="${esc(b.id)}" data-tip="${esc(tip)}">
        <img src="${esc(b.image)}" alt="${esc(b.name)}" loading="lazy">${prog}</button>`;
}

function renderBadges(view, data) {
    if (!data.enabled) {
        view.innerHTML = '<div class="qj-empty">Badges aren\'t kept on this server.</div>';
        return;
    }
    const list = shown(data.badges);
    const earned = data.badges.filter((b) => b.earned_at).length;
    const groups = GROUPS.map(([title, test]) => {
        const items = list.filter(test);
        if (!items.length) return '';
        return `<div class="qj-bgroup">${esc(title)}</div><div class="qj-badges">${items.map(tile).join('')}</div>`;
    }).join('');
    const note = data.signed
        ? 'Each badge is awarded on Nostr by the Dungeon Master.'
        : 'Badges here stay on this server. On the official server the Dungeon Master awards them on Nostr.';
    view.innerHTML = `
        <div class="qj-bsum">${earned} of ${data.badges.length} earned · counted when you save</div>
        ${groups}
        <div id="qj-bdetail" class="qj-bdetail">Tap a badge to see how it's earned.</div>
        <div class="qj-empty">${esc(note)}</div>`;
}

function showDetail(id) {
    const b = badgeCache?.badges.find((x) => x.id === id);
    const el = $('qj-bdetail');
    if (!b || !el) return;
    let status = '';
    if (b.earned_at) {
        status = `Earned ${new Date(b.earned_at * 1000).toLocaleDateString()}`;
        if (badgeCache.signed) status += b.awarded ? ' · awarded on Nostr' : ' · being awarded';
    } else if (b.of) {
        status = `${b.have} of ${b.of}`;
    } else {
        status = 'Not yet earned';
    }
    el.innerHTML = `<b>${esc(b.name)}</b> — ${esc(b.description)}<br><span>${esc(status)}</span>`;
}

// ─── Highscores ─────────────────────────────────────────────────────────────

export async function loadHighscores(which = board) {
    const view = $('qj-scores-view');
    if (!view) return;
    board = which;
    try {
        const res = await fetch(`${API_BASE_URL}/highscores?board=${encodeURIComponent(board)}&npub=${encodeURIComponent(gameAPI.npub || '')}`);
        const json = await res.json();
        if (!json.success) throw new Error('no scores');
        renderScores(view, json.data);
    } catch (e) {
        logger.warn('Highscores failed to load:', e);
        view.innerHTML = '<div class="qj-empty">Highscores couldn\'t be loaded.</div>';
    }
}

function renderScores(view, data) {
    const options = data.boards
        .map((b) => `<option value="${esc(b.id)}" ${b.id === data.board ? 'selected' : ''}>${esc(b.name)}</option>`)
        .join('');
    const rows = (data.entries || [])
        .map((e) => `<div class="qj-score ${e.you ? 'you' : ''}" data-tip="${esc(e.line)}">
            <span class="qj-srank">${e.rank}</span>
            <span class="qj-sname">${esc(e.name || e.npub.slice(0, 12) + '…')}</span>
            <span class="qj-sval">${Number(e.value).toLocaleString()}</span></div>`)
        .join('');
    view.innerHTML = `
        <div class="qj-tools"><div>Board <select id="qj-board">${options}</select></div></div>
        ${rows || '<div class="qj-empty">No scores yet. Scores count when you save.</div>'}`;
}

// ─── The journal tabs ───────────────────────────────────────────────────────

/** Wires clicks inside the badges and highscores views (once). */
export function wireBadgeViews() {
    const panel = $('questlog-panel');
    if (!panel || panel.dataset.badgesWired) return;
    panel.dataset.badgesWired = '1';
    panel.addEventListener('click', (e) => {
        const t = e.target.closest('.qj-badge[data-badge]');
        if (t) showDetail(t.dataset.badge);
    });
    panel.addEventListener('change', (e) => {
        if (e.target.id === 'qj-board') loadHighscores(e.target.value);
    });
}

// ─── The award popup ────────────────────────────────────────────────────────

const queue = [];
let showing = false;

/** Shows "The Dungeon Master awards you…" for each badge a save earned. */
export async function showBadgesEarned(ids) {
    if (!ids?.length) return;
    const data = badgeCache?.badges ? badgeCache : await loadBadges();
    for (const id of ids) {
        const b = data?.badges?.find((x) => x.id === id);
        if (b) queue.push(b);
    }
    pump();
}

function pump() {
    if (showing || !queue.length) return;
    // After any quest-complete screens the same save brought.
    if ($('quest-complete') || $('level-up-modal')?.classList.contains('hidden') === false) {
        setTimeout(pump, 600);
        return;
    }
    showing = true;
    render(queue.shift());
}

function render(b) {
    const host = $('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'badge-award';
    el.innerHTML = `
      <style>
        #badge-award { position: ${host === document.body ? 'fixed' : 'absolute'}; inset: 0; z-index: 8650;
          display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.65);
          font-family: 'Dogica', monospace; animation: ba-in 0.3s ease-out; }
        @keyframes ba-in { from { opacity: 0; transform: scale(0.96); } to { opacity: 1; transform: none; } }
        #badge-award .ba-box { width: 260px; max-width: calc(100% - 16px); box-sizing: border-box; text-align: center;
          background: #2b2416; color: #f5e6c8; font-size: 7px; line-height: 1.7; padding: 10px 12px 9px;
          border-top: 2px solid #a16207; border-left: 2px solid #a16207; border-right: 2px solid #000; border-bottom: 2px solid #000;
          box-shadow: 0 2px 14px rgba(0,0,0,0.8); }
        #badge-award .ba-from { color: #d6c7a1; }
        #badge-award img { width: 128px; height: 128px; image-rendering: pixelated; margin: 6px auto; display: block;
          filter: drop-shadow(0 0 6px rgba(251,191,36,0.55)); }
        #badge-award .ba-name { font-size: 10px; color: #fbbf24; text-shadow: 1px 1px 0 #000; margin-bottom: 3px; }
        #badge-award .ba-desc { margin-bottom: 9px; }
        #badge-award button { font: inherit; font-size: 8px; color: #fff; cursor: pointer; padding: 3px 14px; background: #0e7490;
          border: 2px solid; border-color: #22d3ee #155e75 #155e75 #22d3ee; }
      </style>
      <div class="ba-box" role="dialog" aria-label="Badge earned">
        <div class="ba-from">The Dungeon Master awards you</div>
        <img src="${esc(b.image)}" alt="">
        <div class="ba-name">${esc(b.name)}</div>
        <div class="ba-desc">${esc(b.description)}</div>
        <div><button type="button">Continue</button></div>
      </div>`;
    el.querySelector('button').addEventListener('click', () => {
        el.remove();
        showing = false;
        pump();
    });
    host.appendChild(el);
    el.querySelector('button').focus();
}

if (typeof window !== 'undefined') {
    window.showBadgesEarned = showBadgesEarned;
}
