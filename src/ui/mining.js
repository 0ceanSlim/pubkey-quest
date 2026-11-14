/**
 * Mining minigame — Veins.
 *
 * A rock face as a grid of cells, in the lockpicking window's chrome. The
 * server holds the face (cmd/server/game/mining): this module renders the
 * public view it sends and posts the player's moves (mine_begin / mine_act).
 * A cell shows only what's been earned — a ring from a good sounding, what
 * broke out of it — and every move carries the session version so a double tap
 * can't count twice.
 *
 * Controls: pick a tool (Tap = sound, Swing = strike; T / S), then click or tap
 * a cell. Stop (Esc / ×) steps back with what you have. In the dark (no light
 * underground, or at night) there's no Tap: you swing blind.
 *
 * @module ui/mining
 */

import { gameAPI } from '../lib/api.js';
import { logger } from '../lib/logger.js';

// Ore chunk colours by item id (anything else gets the default).
const ORE_COLOURS = {
    'iron-ore': '#b5512e',
    'copper-ore': '#d8843a',
    'deepiron-ore': '#5d6e8c',
    'mithral-ore': '#cfe3ef',
    'adamantine-ore': '#3f2d5c',
};

// A pick's wear tier (the server's condition tiers).
const WEAR = ['Pristine', 'Worn', 'Damaged', 'Ruined'];

const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

let open = null; // { el, view, tool, busy, lines, resolve, lastData, prevFocus }

/**
 * Open the mining window on a mine node of the walk and play the face until it ends.
 * @param {{node:string}} target
 * @returns {Promise<{status:string, data:Object|null, message?:string}>}
 */
export async function openMining({ node }) {
    if (open) return { status: 'busy', data: null };
    let result;
    try {
        result = await gameAPI.sendAction('mine_begin', { node });
    } catch (err) {
        return { status: 'refused', data: null, message: err.message };
    }
    const data = result.data || {};
    return new Promise((resolve) => {
        open = {
            view: data.mine,
            tool: 'sound',
            busy: false,
            lines: (data.lines || [result.message || 'You size up the rock face.']).map((text) => ({ text, cls: '' })),
            resolve,
            lastData: data,
            prevFocus: document.activeElement,
        };
        mount();
        render();
    });
}

// ─── DOM ─────────────────────────────────────────────────────────────────────

function mount() {
    const back = document.createElement('div');
    back.className = 'lp-backdrop';
    back.innerHTML = `
        <div class="lp-window mn-window" role="dialog" aria-modal="true" aria-labelledby="mn-title" tabindex="-1">
            <div class="lp-title">
                <span class="mn-title-icon" aria-hidden="true">⛏</span>
                <div class="lp-title-text" id="mn-title"></div>
                <button class="lp-close" type="button" aria-label="Stop">X</button>
            </div>
            <div class="mn-viewport"><div class="mn-grid" role="grid" aria-label="Rock face"></div></div>
            <div class="mn-tools" role="group" aria-label="Tool">
                <button type="button" class="lp-btn mn-tool" data-tool="sound" title="Tap (T): sound a cell, Perception, 1 minute">🔔 Tap</button>
                <button type="button" class="lp-btn mn-tool" data-tool="strike" title="Swing (S): strike a cell, 5 minutes">⛏ Swing</button>
            </div>
            <div class="lp-message" aria-live="polite"></div>
            <div class="lp-status"></div>
            <div class="lp-actions"></div>
        </div>`;
    document.body.appendChild(back);
    open.el = back;
    back.querySelector('.lp-close').addEventListener('click', () => (isActive() ? act('stop') : close()));
    back.querySelectorAll('.mn-tool').forEach((b) => b.addEventListener('click', () => setTool(b.dataset.tool)));
    document.addEventListener('keydown', onKey, true);
    back.querySelector('.lp-window').focus();
}

function isActive() {
    return open?.view?.status === 'active';
}

// canSound: soundings left, and light to read the rock by.
function canSound(view) {
    return (view.soundings ?? 0) > 0 && !view.dark;
}

function setTool(tool) {
    if (!open || !isActive()) return;
    if (tool === 'sound' && !canSound(open.view)) return;
    open.tool = tool;
    render();
}

function cellLabel(c, i, w) {
    const pos = `row ${Math.floor(i / w) + 1}, column ${(i % w) + 1}`;
    if (c.state === 'broken') return `${pos}: broken${c.ore_name ? `, ${c.ore_name}` : c.kind === 'fault' ? ', a fault' : ''}`;
    const bits = [];
    if (c.state === 'sounded') bits.push(`rings ${c.ring}`);
    if (c.thud) bits.push('a dull thud');
    if (c.hollow) bits.push('hollow');
    if (c.too_hard) bits.push('too hard for your pick');
    if (c.hard) bits.push('hard rock');
    if (c.cracked) bits.push('cracked');
    return `${pos}: ${bits.length ? bits.join(', ') : 'unknown rock'}`;
}

function render(flash = {}) {
    if (!open) return;
    const { el, view } = open;
    const active = isActive();
    if (active && open.tool === 'sound' && !canSound(view)) open.tool = 'strike';
    const [w] = view.grid || [5, 4];

    el.querySelector('.lp-title-text').textContent = `Mining — ${view.name || 'Rock face'}`;

    // The face
    const grid = el.querySelector('.mn-grid');
    grid.style.gridTemplateColumns = `repeat(${w}, var(--mn-cell))`;
    grid.innerHTML = '';
    (view.cells || []).forEach((c, i) => {
        const b = document.createElement('button');
        b.type = 'button';
        const cls = ['mn-cell'];
        if (c.state === 'broken') cls.push('broken', `kind-${c.kind}`);
        if (c.state === 'sounded') cls.push('sounded');
        if (c.hard) cls.push('hard');
        if (c.cracked) cls.push('cracked');
        if (c.hollow) cls.push('hollow');
        if (c.thud) cls.push('thud');
        if (flash.cell === i) cls.push('hit');
        b.className = cls.join(' ');
        b.setAttribute('aria-label', cellLabel(c, i, w));
        let inner = '';
        if (c.state === 'sounded') inner += `<span class="mn-ring">${c.ring}</span>`;
        if (c.state === 'broken' && c.ore) inner += `<span class="mn-chunk" style="background:${ORE_COLOURS[c.ore] || '#9a8f80'}"></span>`;
        const tags = [];
        if (c.hollow) tags.push('hollow');
        if (c.too_hard) tags.push('too hard');
        if (tags.length) inner += `<span class="mn-tag">${tags.join(' ')}</span>`;
        b.innerHTML = inner;
        b.disabled = !active || open.busy || c.state === 'broken' || (open.tool === 'sound' && c.state === 'sounded');
        b.addEventListener('click', () => act(open.tool, i));
        grid.appendChild(b);
    });
    grid.classList.toggle('shake', !!flash.shake);
    const viewport = el.querySelector('.mn-viewport');
    viewport.classList.toggle('collapsed', view.status === 'collapsed');
    viewport.classList.toggle('dark', !!view.dark);

    // Tools
    el.querySelectorAll('.mn-tool').forEach((b) => {
        const on = b.dataset.tool === open.tool;
        b.classList.toggle('selected', on && active);
        b.setAttribute('aria-pressed', on ? 'true' : 'false');
        b.disabled = !active || open.busy || (b.dataset.tool === 'sound' && !canSound(view));
        if (b.dataset.tool === 'sound') {
            b.title = view.dark
                ? 'Too dark to read the rock: you need a light'
                : 'Tap (T): sound a cell, Perception, 1 minute';
        }
    });

    // Message panel
    el.querySelector('.lp-message').innerHTML = open.lines
        .slice(-3)
        .map((l) => `<div class="lp-line ${l.cls}">${esc(l.text)}</div>`)
        .join('');

    // Status strip
    const cracks = view.cracks || 0;
    const maxCracks = view.cracks_max || 3;
    let pips = '';
    for (let i = 0; i < maxCracks; i++) pips += `<span class="lp-cell${i < cracks ? ' full' : ''}"></span>`;
    const near = active && cracks === maxCracks - 1;
    el.querySelector('.lp-status').innerHTML = `
        <span title="Swings left">⛏ ${view.swings}/${view.swings_max}</span>
        <span title="Soundings left">🔔 ${view.soundings}/${view.soundings_max}</span>
        <span class="lp-strain${near ? ' near' : ''}" title="Cracks">Cracks
            <span class="lp-strain-cells" aria-hidden="true">${pips}</span>
            <span>${cracks}/${maxCracks}${near ? ' !' : ''}</span></span>
        <span>Tier ${esc(view.tier)} · DC ${esc(view.dc)}</span>
        ${view.pick_name ? `<span title="Your pick">${esc(view.pick_name)}${view.pick_wear > 0 ? ` (${WEAR[view.pick_wear] ?? ''})` : ''}</span>` : ''}
        ${view.dark ? '<span class="mn-dark" title="No light: you can\'t read the rock">Dark</span>' : ''}`;

    // Actions
    const actions = el.querySelector('.lp-actions');
    if (active) {
        actions.innerHTML = `<button type="button" class="lp-btn" data-act="stop">Stop</button>`;
        const stop = actions.querySelector('button');
        stop.disabled = open.busy;
        stop.addEventListener('click', () => act('stop'));
    } else {
        actions.innerHTML = `<button type="button" class="lp-btn primary">Close</button>`;
        const done = actions.querySelector('button');
        done.addEventListener('click', close);
        done.focus();
    }
}

// ─── Moves ───────────────────────────────────────────────────────────────────

async function act(kind, cell = -1) {
    if (!open || open.busy || !isActive()) return;
    open.busy = true;
    render();
    const before = open.view;
    let result;
    try {
        result = await gameAPI.sendAction('mine_act', {
            session_id: before.session_id,
            version: before.version,
            act: kind,
            cell,
        });
    } catch (err) {
        logger.error('mine_act failed:', err);
        open.busy = false;
        open.lines.push({ text: err.message || 'Something went wrong.', cls: 'bad' });
        render();
        return;
    }
    if (!open) return;
    const data = result.data || {};
    open.busy = false;
    open.lastData = data;
    if (data.mine) open.view = data.mine;
    if (data.stale) {
        render();
        return;
    }
    const after = open.view;
    const gotOre = after.cells?.[cell]?.ore && before.cells?.[cell]?.state !== 'broken';
    const cracked = (after.cracks ?? 0) > (before.cracks ?? 0);
    const cls = gotOre ? 'good' : cracked || after.status === 'collapsed' ? 'bad' : '';
    (data.lines || []).forEach((t) => open.lines.push({ text: t, cls }));
    if (kind === 'stop' || data.death) {
        close();
        return;
    }
    render({ cell: kind === 'strike' ? cell : -1, shake: cracked });
}

function onKey(e) {
    if (!open) return;
    const k = e.key;
    if (k === 'Escape') {
        e.preventDefault();
        isActive() ? act('stop') : close();
    } else if (k === 't' || k === 'T') {
        e.preventDefault();
        setTool('sound');
    } else if (k === 's' || k === 'S') {
        e.preventDefault();
        setTool('strike');
    }
}

function close() {
    if (!open) return;
    const { el, view, resolve, lastData, prevFocus } = open;
    document.removeEventListener('keydown', onKey, true);
    el.remove();
    open = null;
    try { prevFocus?.focus?.(); } catch { /* ignore */ }
    resolve({ status: view.status, data: lastData });
}
