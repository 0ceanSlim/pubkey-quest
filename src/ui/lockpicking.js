/**
 * Lockpicking minigame — Tumblers.
 *
 * A small Win95-style window over a dimmed screen: an iron lock plate on a
 * wooden door with 3–6 pin tumblers. The server holds the puzzle
 * (cmd/server/game/locks): this module only renders the public view it sends
 * and posts the player's moves (lock_begin / lock_act). Nothing is shown as
 * set, broken or open until the server says so, and every move carries the
 * session version so a double tap can't count twice.
 *
 * Controls: tap/click a pin (or 1–6, ←/→) to select it; Lift (Enter/Space);
 * Inspect (I); Give up (Esc / ×).
 *
 * @module ui/lockpicking
 */

import { gameAPI } from '../lib/api.js';
import { logger } from '../lib/logger.js';

// ─── Pixel icons (drawn from string art so they stay crisp and swappable) ────

const PAL = {
    k: '#0a0a0a', i: '#8a8f98', d: '#4a4d54', b: '#c8963e', y: '#f0c66b',
    w: '#7a5233', p: '#e8dcbc', r: '#e0573c', s: '#d8dce4',
};

const ICONS = {
    lock: [
        '..kkkk..',
        '.kiddik.',
        '.ki..ik.',
        'kbbbbbbk',
        'kbyyyybk',
        'kbbkkbbk',
        'kbbbbbbk',
        'kkkkkkkk',
    ],
    pick: [
        '.......k',
        '......ks',
        '.....ks.',
        '....ks..',
        '...ks...',
        '..ks....',
        'kbk.....',
        'bbk.....',
    ],
    wrench: [
        'kk......',
        'kik.....',
        'kik.....',
        'kik.....',
        'kik.....',
        'kiikkkkk',
        'kiiiiiik',
        'kkkkkkkk',
    ],
    kit: [
        '........',
        '.kkkkkk.',
        'kwwwwwwk',
        'kwswswwk',
        'kwwwwwwk',
        'kwwswswk',
        '.kkkkkk.',
        '........',
    ],
    glint: [
        '..y..',
        '..y..',
        'yyyyy',
        '..y..',
        '..y..',
    ],
};

function pix(name, scale = 2) {
    const rows = ICONS[name];
    const h = rows.length;
    const w = rows[0].length;
    let rects = '';
    rows.forEach((row, y) => {
        [...row].forEach((c, x) => {
            if (PAL[c]) rects += `<rect x="${x}" y="${y}" width="1" height="1" fill="${PAL[c]}"/>`;
        });
    });
    return `<svg width="${w * scale}" height="${h * scale}" viewBox="0 0 ${w} ${h}" aria-hidden="true">${rects}</svg>`;
}

const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

// ─── State of the open window ────────────────────────────────────────────────

let open = null; // { el, view, picks, kit, selected, busy, lines, resolve, lastData, prevFocus }

/**
 * Open the lockpicking window on a lock and play it until it ends.
 * @param {{kind:'room'|'poi', roomId?:string, node?:string}} target
 * @returns {Promise<{status:string, opened:boolean, data:Object|null}>}
 */
export async function openLockpicking({ kind, roomId = '', node = '' }) {
    if (open) return { status: 'busy', opened: false, data: null };
    let result;
    try {
        result = await gameAPI.sendAction('lock_begin', { kind, room_id: roomId, node, style: 'tumblers' });
    } catch (err) {
        return { status: 'refused', opened: false, data: null, message: err.message };
    }
    const data = result.data || {};
    return new Promise((resolve) => {
        open = {
            view: data.lock,
            picks: data.picks ?? 0,
            kit: !!data.kit,
            selected: firstUnset(data.lock),
            busy: false,
            lines: [{ text: result.message || 'You feel for the pins.', cls: '' }],
            resolve,
            lastData: data,
            prevFocus: document.activeElement,
        };
        mount();
        render();
    });
}

function firstUnset(view) {
    const i = (view?.set || []).findIndex((s) => !s);
    return i < 0 ? 0 : i;
}

// ─── DOM ─────────────────────────────────────────────────────────────────────

function mount() {
    const back = document.createElement('div');
    back.className = 'lp-backdrop';
    back.innerHTML = `
        <div class="lp-window" role="dialog" aria-modal="true" aria-labelledby="lp-title" tabindex="-1">
            <div class="lp-title">
                ${pix('lock')}
                <div class="lp-title-text" id="lp-title"></div>
                <button class="lp-close" type="button" aria-label="Give up">X</button>
            </div>
            <div class="lp-viewport">
                <div class="lp-plate">
                    <span class="lp-rivet tl"></span><span class="lp-rivet tr"></span>
                    <span class="lp-rivet bl"></span><span class="lp-rivet br"></span>
                    <div class="lp-pins" role="group" aria-label="Pins"></div>
                    <div class="lp-keyway" aria-hidden="true"></div>
                </div>
            </div>
            <div class="lp-numbers" role="group" aria-label="Select a pin"></div>
            <div class="lp-tray"></div>
            <div class="lp-message" aria-live="polite"></div>
            <div class="lp-status"></div>
            <div class="lp-actions"></div>
        </div>`;
    document.body.appendChild(back);
    open.el = back;
    back.querySelector('.lp-close').addEventListener('click', () => (isActive() ? act('give_up') : close()));
    document.addEventListener('keydown', onKey, true);
    back.querySelector('.lp-window').focus();
}

function isActive() {
    return open?.view?.status === 'active';
}

function render(flash = {}) {
    if (!open) return;
    const { el, view } = open;
    const n = view.pins || 0;
    const set = view.set || [];
    const ruled = view.ruled_out || [];
    const active = isActive();

    el.querySelector('.lp-title-text').textContent = `Lockpicking — ${view.name || 'Lock'}`;

    // Pins
    const pins = el.querySelector('.lp-pins');
    pins.innerHTML = '';
    for (let p = 0; p < n; p++) {
        const b = document.createElement('button');
        b.type = 'button';
        const state = set[p] ? 'set' : ruled[p] ? 'not binding' : 'unset';
        b.className = 'lp-pin' + (set[p] ? ' is-set' : '') + (ruled[p] ? ' ruled-out' : '') +
            (p === open.selected && active ? ' selected' : '') + (view.glint === p && active ? ' glint' : '') +
            (flash.clicked === p ? ' clicked' : '');
        b.setAttribute('aria-label', `Pin ${p + 1}, ${state}${view.glint === p ? ', glinting' : ''}`);
        b.innerHTML = `
            <span class="lp-glint">${pix('glint', 2)}</span>
            <span class="lp-stack"><span class="lp-driver" style="display:block"></span><span class="lp-keypin" style="display:block"></span></span>
            <span class="lp-mark">${set[p] ? 'SET' : ruled[p] ? 'X' : p + 1}</span>`;
        b.disabled = !active || set[p];
        b.addEventListener('click', () => select(p));
        pins.appendChild(b);
    }
    pins.classList.toggle('slip', !!flash.slip);
    el.querySelector('.lp-plate').classList.toggle('opened', view.status === 'opened');

    // Number buttons
    const nums = el.querySelector('.lp-numbers');
    nums.innerHTML = '';
    for (let p = 0; p < n; p++) {
        const b = document.createElement('button');
        b.type = 'button';
        b.className = 'lp-btn lp-num' + (p === open.selected && active ? ' selected' : '');
        b.textContent = String(p + 1);
        b.disabled = !active || set[p];
        b.setAttribute('aria-pressed', p === open.selected ? 'true' : 'false');
        b.addEventListener('click', () => select(p));
        nums.appendChild(b);
    }

    // Tool tray
    el.querySelector('.lp-tray').innerHTML = `
        <div class="lp-tool">${pix('pick')}<span>Lockpicks: ${open.picks}</span></div>
        <div class="lp-tool">${pix('wrench')}<span>Tension wrench</span></div>
        <div class="lp-tool${open.kit ? '' : ' off'}">${pix('kit')}<span>${open.kit ? "Thieves' Kit +1" : 'No kit'}</span></div>`;

    // Message panel
    el.querySelector('.lp-message').innerHTML = open.lines
        .slice(-3)
        .map((l) => `<div class="lp-line ${l.cls}">${esc(l.text)}</div>`)
        .join('');

    // Status strip
    const max = view.strain_max || 1;
    const strain = view.strain || 0;
    const near = active && strain === max - 1;
    let cells = '';
    for (let i = 0; i < max; i++) cells += `<span class="lp-cell${i < strain ? ' full' : ''}"></span>`;
    el.querySelector('.lp-status').innerHTML = `
        <span class="lp-strain${near ? ' near' : ''}" title="Strain">Strain
            <span class="lp-strain-cells" aria-hidden="true">${cells}</span>
            <span>${strain}/${max}${near ? ' !' : ''}</span></span>
        <span>${esc(view.tier || '')} lock · ${n} pins</span>
        ${view.broken ? `<span>Broken: ${view.broken}</span>` : ''}`;

    // Actions
    const actions = el.querySelector('.lp-actions');
    if (active) {
        actions.innerHTML = `
            <button type="button" class="lp-btn primary" data-act="lift">Lift pin ${open.selected + 1}</button>
            <button type="button" class="lp-btn" data-act="inspect" ${view.inspected ? 'disabled' : ''}>Inspect</button>
            <button type="button" class="lp-btn" data-act="give_up">Give up</button>`;
        actions.querySelectorAll('button').forEach((b) => {
            b.disabled = b.disabled || open.busy;
            b.addEventListener('click', () => act(b.dataset.act));
        });
    } else {
        actions.innerHTML = `<button type="button" class="lp-btn primary" data-act="done">${view.status === 'opened' ? 'Open the door' : 'Close'}</button>`;
        const done = actions.querySelector('button');
        done.addEventListener('click', close);
        done.focus();
    }
}

function select(p) {
    if (!open || !isActive() || open.busy || open.view.set?.[p]) return;
    open.selected = p;
    render();
}

// ─── Moves ───────────────────────────────────────────────────────────────────

async function act(kind) {
    if (!open || open.busy || !isActive()) return;
    open.busy = true;
    render();
    const before = open.view;
    let result;
    try {
        result = await gameAPI.sendAction('lock_act', {
            session_id: before.session_id,
            version: before.version,
            act: kind,
            pin: open.selected,
        });
    } catch (err) {
        logger.error('lock_act failed:', err);
        open.busy = false;
        open.lines.push({ text: err.message || 'Something went wrong.', cls: 'bad' });
        render();
        return;
    }
    if (!open) return;
    const data = result.data || {};
    open.busy = false;
    open.lastData = data;
    if (data.lock) open.view = data.lock;
    if (typeof data.picks === 'number') open.picks = data.picks;
    if (data.stale) {
        render();
        return;
    }

    const after = open.view;
    const lines = data.lines || [];
    const brokeOrSlipped = (after.strain ?? 0) > (before.strain ?? 0) || (after.broken ?? 0) > (before.broken ?? 0);
    const cls = after.status === 'opened' ? 'good' : brokeOrSlipped ? 'bad' : '';
    lines.forEach((t) => open.lines.push({ text: t, cls }));

    const flash = {};
    if (kind === 'lift') {
        if (after.set?.[open.selected] && !before.set?.[open.selected]) flash.clicked = open.selected;
        else if (brokeOrSlipped) flash.slip = true;
    }
    if (isActive() && after.set?.[open.selected]) open.selected = firstUnset(after);
    if (kind === 'give_up') {
        close(); // walking away needs no "Close" click
        return;
    }
    render(flash);
}

function onKey(e) {
    if (!open) return;
    const k = e.key;
    if (k === 'Escape') {
        e.preventDefault();
        isActive() ? act('give_up') : close();
        return;
    }
    if (!isActive()) return;
    const n = open.view.pins || 0;
    if (/^[1-9]$/.test(k) && Number(k) <= n) {
        e.preventDefault();
        select(Number(k) - 1);
    } else if (k === 'ArrowLeft' || k === 'ArrowRight') {
        e.preventDefault();
        const dir = k === 'ArrowLeft' ? -1 : 1;
        let p = open.selected;
        for (let i = 0; i < n; i++) {
            p = (p + dir + n) % n;
            if (!open.view.set?.[p]) break;
        }
        select(p);
    } else if (k === 'Enter' || k === ' ') {
        e.preventDefault();
        act('lift');
    } else if (k === 'i' || k === 'I') {
        e.preventDefault();
        if (!open.view.inspected) act('inspect');
    }
}

function close() {
    if (!open) return;
    const { el, view, resolve, lastData, prevFocus } = open;
    document.removeEventListener('keydown', onKey, true);
    el.remove();
    open = null;
    try { prevFocus?.focus?.(); } catch { /* ignore */ }
    resolve({ status: view.status, opened: view.status === 'opened', data: lastData });
}
