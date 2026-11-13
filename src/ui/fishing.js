/**
 * Fishing window — Cast & Reel (docs/draft/fishing-system.md).
 *
 * A Win95-style window over a dimmed screen, like the lockpicking one. The
 * server rolls everything (cmd/server/game/fishing): this module only renders
 * the public view it sends and posts the player's moves. What's on the line
 * stays a mystery until it's landed; every move carries the session version
 * so a double tap can't count twice.
 *
 * At a spot you pick a bait and Cast (the minigame), or Quick fish (one roll),
 * Fish barehanded (after Barbarian Fishing), Haul the net (shallows) or Dig for
 * worms. Then: Wait and Strike while you watch the float; Reel, Hold or Give
 * line in a fight; Recast after a catch.
 *
 * Keys: W wait · Space strike · R reel · H hold · G give · C recast · Esc stop.
 *
 * @module ui/fishing
 */

import { gameAPI } from '../lib/api.js';
import { eventBus } from '../lib/events.js';
import { logger } from '../lib/logger.js';
import { refreshGameState } from '../state/gameState.js';

// ─── Pixel icons ─────────────────────────────────────────────────────────────

const PAL = {
    k: '#0a0a0a', r: '#d83a2a', w: '#f4efe2', y: '#f0c66b', b: '#7a5233',
    s: '#c9ced6', d: '#3b2a1a', g: '#5a8a3a', f: '#9fb4c4',
};

const ICONS = {
    float: [
        '...kk...',
        '..kwwk..',
        '.kwwwwk.',
        'krrrrrrk',
        'krrrrrrk',
        '.krrrrk.',
        '..kkkk..',
        '...kk...',
    ],
    rod: [
        '.......k',
        '......kb',
        '.....kb.',
        '....kb..',
        '...kb...',
        '.kkb....',
        'kysk....',
        '.kk.....',
    ],
    fish: [
        '........',
        '..kkkk.k',
        '.kffffkk',
        'kfkffffk',
        'kffffffk',
        '.kffffkk',
        '..kkkk.k',
        '........',
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

// ─── The town Fish button ────────────────────────────────────────────────────

let townSpot = '';
eventBus.on('fishing:spot', (spot) => {
    townSpot = spot || '';
    const btn = document.getElementById('fish-btn');
    if (btn) btn.classList.toggle('hidden', !townSpot);
});

/** Open the fishing window at the town spot you're standing at. */
export async function openTownFishing() {
    if (!townSpot) return;
    await openFishing({ kind: 'town' });
    await refreshGameState(true);
}

// ─── State of the open window ────────────────────────────────────────────────

// { el, kind, node, win, view, bait, busy, lines, resolve, lastData, prevFocus }
let open = null;

/**
 * Open the fishing window at a spot and play until the player walks away.
 * @param {{kind:'town'|'poi', node?:string}} where
 * @returns {Promise<{status:string, data:Object|null, message?:string}>}
 */
export async function openFishing({ kind, node = '' }) {
    if (open) return { status: 'busy', data: null };
    let result;
    try {
        result = await gameAPI.sendAction('fish_open', { kind, node });
    } catch (err) {
        return { status: 'refused', data: null, message: err.message };
    }
    const win = result.data?.window || {};
    return new Promise((resolve) => {
        open = {
            kind, node, win, view: null,
            bait: defaultBait(win),
            busy: false,
            lines: [{ text: introLine(win), cls: '' }],
            resolve,
            lastData: result.data || {},
            prevFocus: document.activeElement,
        };
        mount();
        render();
    });
}

function introLine(win) {
    if (win.fished_out) return "This water's fished out for today.";
    if (!win.rod && !win.barehand) return "You've no rod. You could still try other ways.";
    return `You look out over ${win.spot || 'the water'}.`;
}

// The first bait the player carries that isn't a lure, else the first lure.
function defaultBait(win) {
    const baits = win.baits || [];
    return (baits.find((b) => !b.lure) || baits[0])?.item_id || '';
}

// ─── DOM ─────────────────────────────────────────────────────────────────────

function mount() {
    const back = document.createElement('div');
    back.className = 'fs-backdrop';
    back.innerHTML = `
        <div class="fs-window" role="dialog" aria-modal="true" aria-labelledby="fs-title" tabindex="-1">
            <div class="fs-title">
                ${pix('rod')}
                <div class="fs-title-text" id="fs-title"></div>
                <button class="fs-close" type="button" aria-label="Stop fishing">X</button>
            </div>
            <div class="fs-viewport"></div>
            <div class="fs-gauges"></div>
            <div class="fs-message" aria-live="polite"></div>
            <div class="fs-tray"></div>
            <div class="fs-actions"></div>
        </div>`;
    document.body.appendChild(back);
    open.el = back;
    back.querySelector('.fs-close').addEventListener('click', stop);
    document.addEventListener('keydown', onKey, true);
    back.querySelector('.fs-window').focus();
}

function phase() {
    return open?.view?.phase || 'menu';
}

function render(flash = '') {
    if (!open) return;
    const { el, win, view } = open;
    el.querySelector('.fs-title-text').textContent = `Fishing — ${win.spot || 'the water'}`;
    renderScene(el.querySelector('.fs-viewport'), flash);
    renderGauges(el.querySelector('.fs-gauges'));
    el.querySelector('.fs-message').innerHTML = open.lines
        .slice(-4)
        .map((l) => `<div class="fs-line ${l.cls}">${esc(l.text)}</div>`)
        .join('');
    renderTray(el.querySelector('.fs-tray'));
    renderActions(el.querySelector('.fs-actions'));
    if (view?.hint) el.querySelector('.fs-message').insertAdjacentHTML('beforeend', `<div class="fs-line good">${esc(view.hint)}</div>`);
}

// The water: a float bobbing (or a shadow, barehanded), a fish being fought,
// or the catch on the bank.
function renderScene(box, flash) {
    const { win, view } = open;
    const water = win.water || 'river';
    box.className = `fs-viewport water-${water}${win.dark ? ' dark' : ''}`;
    const p = phase();
    if (p === 'done' && view?.catch) {
        const c = view.catch;
        const id = c.item_id.replace(/^fish-/, '').replace(/-(tiny|small|medium|large|huge)$/, '');
        box.innerHTML = `
            <div class="fs-card">
                <img src="/res/img/items/fish-${esc(id)}.png" alt="" onerror="this.style.display='none'">
                <div class="fs-card-text">
                    <div class="fs-card-name">${esc(c.size)} ${esc(c.species)}</div>
                    <div>${esc(c.weight)}</div>
                    <div class="fs-card-value">${esc(c.value)} gold</div>
                </div>
            </div>`;
        return;
    }
    if (p === 'fight') {
        box.innerHTML = `<div class="fs-surface"></div>
            <div class="fs-splash ${view.spent ? 'spent' : ''} ${flash}">${pix('fish', 3)}</div>`;
        return;
    }
    if (view?.barehand) {
        const f = view.float || 'still';
        box.innerHTML = `<div class="fs-surface"></div><div class="fs-shadow shadow-${f} ${flash}"></div>`;
        return;
    }
    const f = p === 'watch' ? view.float || 'still' : 'idle';
    box.innerHTML = `<div class="fs-surface"></div>
        <div class="fs-float float-${f} ${flash}">${pix('float', 3)}</div>
        ${f === 'dive' ? '<div class="fs-ring"></div>' : ''}`;
}

// Tension and distance bars in a fight; the float's state otherwise.
function renderGauges(box) {
    const v = open.view;
    if (phase() !== 'fight') {
        const read = {
            still: v?.barehand ? 'The water is still.' : 'The float sits still.',
            twitch: v?.barehand ? 'A shadow circles.' : 'The float twitches!',
            dive: v?.barehand ? 'It\'s close, by your legs!' : 'The float dives under!',
        }[v?.float] || '';
        box.innerHTML = phase() === 'watch' ? `<div class="fs-read">${esc(read)}</div>` : '';
        return;
    }
    const cells = (n, filled, cls) => {
        let s = '';
        for (let i = 0; i < n; i++) s += `<span class="fs-cell${i < filled ? ' ' + cls : ''}"></span>`;
        return s;
    };
    const line = v.line || 10;
    const near = v.tension >= line - 3;
    const max = v.max_distance || 12;
    box.innerHTML = `
        <div class="fs-gauge${near ? ' near' : ''}" title="Line tension: it snaps at ${line}">
            <span class="fs-label">Tension</span><span class="fs-cells">${cells(line, v.tension, 'hot')}</span>
            <span>${v.tension}/${line}</span></div>
        <div class="fs-gauge" title="How far out it is: land it at 0; the line runs out at ${max}">
            <span class="fs-label">Distance</span><span class="fs-cells">${cells(max, v.distance, 'far')}</span>
            <span>${v.distance}/${max}</span></div>
        <div class="fs-tell">${v.tell ? esc(v.tell) : '<span class="muted">You can\'t read its next move.</span>'}</div>
        <div class="fs-feel">It feels <b>${esc(v.feel)}</b>${v.spent ? ' · <span class="good">tiring</span>' : ''}</div>`;
}

// Rod and bait: the bait picker in the menu and between catches.
function renderTray(box) {
    const { win, view } = open;
    const choosing = phase() === 'menu' || phase() === 'done';
    const rod = win.rod ? `${esc(win.rod.name)} (line ${win.rod.line})` : 'No rod';
    let bait;
    if (view?.barehand) {
        bait = 'Bare hands';
    } else if (choosing) {
        const opts = (win.baits || [])
            .map((b) => `<option value="${esc(b.item_id)}" ${b.item_id === open.bait ? 'selected' : ''}>${esc(b.name)} ×${b.count}</option>`)
            .join('');
        bait = `<label class="fs-bait">Bait <select ${open.busy ? 'disabled' : ''}>${opts}<option value="" ${open.bait ? '' : 'selected'}>Bare hook</option></select></label>`;
    } else {
        bait = esc(view?.bait_name || 'Bare hook');
    }
    box.innerHTML = `<div class="fs-tool">${pix('rod')}<span>${rod}</span></div><div class="fs-tool">${bait}</div>`;
    box.querySelector('select')?.addEventListener('change', (e) => { open.bait = e.target.value; });
}

function renderActions(box) {
    const { win, view } = open;
    const b = (act, label, primary = false, disabled = false) =>
        `<button type="button" class="fs-btn${primary ? ' primary' : ''}" data-act="${act}" ${disabled || open.busy ? 'disabled' : ''}>${label}</button>`;
    let html = '';
    switch (phase()) {
        case 'menu':
            html = b('cast', 'Cast', true, !win.rod || win.fished_out) +
                b('quick', 'Quick fish', false, !win.rod || win.fished_out) +
                (win.barehand ? b('barehand', 'Bare hands', false, win.fished_out) : '') +
                (win.net ? b('net', 'Haul the net') : '') +
                (win.worms ? b('worms', 'Dig worms') : '') +
                b('leave', 'Leave');
            break;
        case 'watch':
            html = b('wait', 'Wait <kbd>W</kbd>', view.float !== 'dive') +
                b('strike', view.barehand ? 'Grab <kbd>␣</kbd>' : 'Strike <kbd>␣</kbd>', view.float === 'dive') +
                b('quit', 'Stop');
            break;
        case 'fight':
            html = b('reel', 'Reel <kbd>R</kbd>', true) + b('hold', 'Hold <kbd>H</kbd>') + b('give', 'Give line <kbd>G</kbd>');
            break;
        case 'done':
            html = b('recast', view.barehand ? 'Wade in again <kbd>C</kbd>' : 'Recast <kbd>C</kbd>', true, win.fished_out) + b('quit', 'Stop');
            break;
        default:
            html = b('leave', 'Close', true);
    }
    box.innerHTML = html;
    box.querySelectorAll('button').forEach((btn) => btn.addEventListener('click', () => press(btn.dataset.act)));
}

// ─── Moves ───────────────────────────────────────────────────────────────────

function press(act) {
    if (!open || open.busy) return;
    switch (act) {
        case 'cast': return begin(false);
        case 'barehand': return begin(true);
        case 'quick': return oneShot('fish_quick', { bait: open.bait });
        case 'net': return oneShot('haul_net', {});
        case 'worms': return oneShot('dig_worms', {});
        case 'leave': return close();
        default: return move(act);
    }
}

async function send(type, params) {
    open.busy = true;
    render();
    try {
        return await gameAPI.sendAction(type, { kind: open.kind, node: open.node, ...params });
    } catch (err) {
        logger.error(`${type} failed:`, err);
        if (open) open.lines.push({ text: err.message || 'Something went wrong.', cls: 'bad' });
        return null;
    } finally {
        if (open) {
            open.busy = false;
        }
    }
}

function take(result, cls = '') {
    if (!open || !result) return {};
    const data = result.data || {};
    open.lastData = data;
    if (data.window) {
        open.win = data.window;
        if (!(open.win.baits || []).some((b) => b.item_id === open.bait)) open.bait = defaultBait(open.win);
    }
    if (data.fish) open.view = data.fish;
    (data.lines || (result.message ? [result.message] : [])).forEach((t) => open.lines.push({ text: t, cls: lineClass(t, cls) }));
    return data;
}

function lineClass(text, fallback) {
    if (/^Landed|haul it out|You land|dig up|purse/.test(text)) return 'good';
    if (/SNAP|gone|throws the hook|spooks|twists free|last /.test(text)) return 'bad';
    return fallback;
}

async function begin(barehand) {
    const r = await send('fish_begin', { bait: barehand ? '' : open.bait, barehand });
    take(r);
    render();
}

async function oneShot(type, params) {
    const r = await send(type, params);
    take(r);
    render();
}

async function move(act) {
    const v = open.view;
    if (!v) return;
    const params = { session_id: v.session_id, version: v.version, act };
    if (act === 'recast') params.bait = open.bait;
    const r = await send('fish_act', params);
    if (!open) return;
    const before = v.phase;
    const data = take(r);
    if (data.stale) return render();
    if (data.combat_started) {
        // Something nasty took the bait: the fight takes over.
        close();
        eventBus.emit('combat:started', data.combat);
        return;
    }
    if (phase() === 'over') {
        close();
        return;
    }
    const flash = before === 'watch' && phase() === 'fight' ? 'hooked' : act === 'strike' && phase() === 'watch' ? 'miss' : '';
    render(flash);
}

function stop() {
    if (!open || open.busy) return;
    if (open.view && phase() !== 'over') {
        move('quit');
        return;
    }
    close();
}

function onKey(e) {
    if (!open) return;
    const k = e.key.toLowerCase();
    const p = phase();
    const keys = {
        watch: { w: 'wait', ' ': 'strike', enter: 'strike' },
        fight: { r: 'reel', h: 'hold', g: 'give' },
        done: { c: 'recast' },
    }[p] || {};
    if (k === 'escape') {
        e.preventDefault();
        stop();
    } else if (keys[k]) {
        e.preventDefault();
        if (k === 'w' && open.view?.float === 'dive') return; // don't wait it out by accident
        press(keys[k]);
    }
}

function close() {
    if (!open) return;
    const { el, view, resolve, lastData, prevFocus } = open;
    document.removeEventListener('keydown', onKey, true);
    el.remove();
    open = null;
    try { prevFocus?.focus?.(); } catch { /* ignore */ }
    resolve({ status: view?.phase || 'closed', data: lastData });
}
