/**
 * Combat System
 *
 * Drives the combat UI and communicates with the server-side combat engine.
 *
 * @module systems/combatSystem
 */

import { logger }   from '../lib/logger.js';
import { eventBus } from '../lib/events.js';
import { gameAPI }  from '../lib/api.js';

// ─── Range descriptions ────────────────────────────────────────────────────────
const RANGE_LABELS = {
    0: 'Contact',
    1: 'Adjacent',
    2: 'Short',
    3: 'Medium',
    4: 'Long',
    5: 'Very long',
    6: 'Extreme',
};

// ─── Grid constants ────────────────────────────────────────────────────────────
const GRID_COLS = 9;
const GRID_ROWS = 7;

// ─── Module state ─────────────────────────────────────────────────────────────
let _cachedNav       = null;
let _cachedBld       = null;
let _cachedNpc       = null;
let _cachedGameText  = null;
let _lastState       = null;
let _checkedOnLoad   = false;
let _baseExperience  = 0;

// ─── Helpers ──────────────────────────────────────────────────────────────────
const getNpub   = () => gameAPI.npub   ?? null;
const getSaveID = () => gameAPI.saveID ?? null;
const $id       = id  => document.getElementById(id);
const _show     = id  => $id(id)?.classList.remove('hidden');
const _hide     = id  => $id(id)?.classList.add('hidden');
const _setText  = (id, t) => { const e = $id(id); if (e) e.textContent = t; };

function combatPost(endpoint, body) {
    return fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
}

// ─── Public API ───────────────────────────────────────────────────────────────

export async function debugStartCombat() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) {
        logger.error('Combat: session not initialised');
        window.showMessage?.('Session not ready. Try again in a moment.', 'error');
        return;
    }
    const monsterID = document.getElementById('debug-monster-select')?.value ?? '';
    try {
        const resp = await combatPost('/api/combat/debug/start', { npub, save_id: saveID, monster_id: monsterID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            window.showMessage?.(cs.error ?? `HTTP ${resp.status}`, 'error');
            return;
        }
        enterCombatMode(cs);
    } catch (err) {
        logger.error('debugStartCombat error:', err);
        window.showMessage?.('Failed to start combat.', 'error');
    }
}

export function enterCombatMode(cs) {
    logger.info('⚔️  Entering combat mode');
    _baseExperience = window.getGameStateSync?.()?.character?.experience ?? 0;
    _replaceGameText();
    _replaceActionButtons(cs);
    _show('combat-overlay');
    renderCombatState(cs);
}

export function exitCombatMode() {
    logger.info('🏳️  Exiting combat mode');
    _hide('combat-overlay');
    _restoreGameText();
    _restoreActionButtons();
    _hide('death-saves-panel');
    _hide('loot-panel');
    _hide('defeat-panel');
    _lastState = null;
}

/** Single re-render function — reads cs and updates every DOM region. */
export function renderCombatState(cs) {
    if (!cs) return;
    const prevMonsterPos = _lastState?.monster_pos ?? null;
    _lastState = cs;

    const monster = cs.monsters?.[0];

    // ── Monster panel ────────────────────────────────────────────────────────
    if (monster) {
        _setText('combat-monster-name', monster.name ?? 'Unknown');
        _setText('combat-monster-ac-badge', `AC ${monster.armor_class ?? '?'}`);

        const pct = monster.max_hp > 0
            ? Math.max(0, Math.min(100, (monster.current_hp / monster.max_hp) * 100))
            : 0;
        const bar = $id('combat-monster-hp-bar');
        if (bar) bar.style.width = `${pct}%`;
        _setText('combat-monster-hp-text', `${monster.current_hp} / ${monster.max_hp} HP`);

        const img = $id('combat-monster-img');
        if (img && monster.instance_id) {
            const newSrc = `/res/img/monsters/${monster.instance_id}.png`;
            if (img.src !== newSrc) {
                img.src = newSrc;
                img.onerror = () => { img.src = '/res/img/monsters/unknown.png'; img.onerror = null; };
            }
        }
    }

    // ── Round + range + movement budget ──────────────────────────────────────
    _setText('combat-round-display', `Round ${cs.round ?? 1}`);
    const range  = cs.range ?? 0;
    const budget = cs.movement_budget ?? 6;
    const spent  = cs.movement_spent  ?? 0;
    const remaining = budget - spent;

    const wID    = _equippedWeaponID('mainHand');
    const wRange = wID ? _isRangedWeapon(wID) : false;
    const reach  = wID ? _meleeReach(wID) : 1;
    const inReach = !wRange && range <= reach;
    const reachTag = !wRange ? (inReach ? ' ⚔' : '') : '';
    _setText('combat-range-display', `Range ${range} — ${RANGE_LABELS[range] ?? ''}${reachTag}`);
    _setText('combat-move-budget', `Move ${remaining}/${budget}`);

    // ── Combat log (staggered) ───────────────────────────────────────────────
    const logEl = $id('combat-log');
    if (logEl) {
        const isFirstRender = logEl.childElementCount === 0;
        const entries = isFirstRender
            ? (cs.log ?? [])
            : (cs.new_log?.length ? cs.new_log : []);
        if (entries.length) {
            const interval = isFirstRender ? LOG_MS_INITIAL : LOG_MS_ROUND;
            _appendLogEntriesStaggered(logEl, entries, false, interval);
        }
    }

    // ── Grid (animate monster path if it moved) ──────────────────────────────
    _renderGridAnimated(cs, prevMonsterPos);

    // ── Action buttons ────────────────────────────────────────────────────────
    if (_cachedNav !== null) _renderCombatButtons(cs);

    // ── Phase panels ─────────────────────────────────────────────────────────
    _showPhasePanel(cs);

    // ── Side-panel sync ───────────────────────────────────────────────────────
    _syncSidePanelFromCombat(cs);
}

export async function doAttack(hand = 'main', thrown = false) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/action', {
            npub, save_id: saveID,
            weapon_slot: hand === 'off' ? 'offHand' : 'mainHand',
            hand, thrown,
        });
        const cs = await resp.json();
        if (!resp.ok || !cs.success) {
            const msg = cs.error ?? `HTTP ${resp.status}`;
            _logError(msg);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doAttack error:', err);
        _logError('Network error — could not process attack.');
    }
}

/** Move player token to grid cell (x, y). Called by grid cell click handler. */
export async function doMoveToCell(x, y) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/move', { npub, save_id: saveID, x, y });
        const cs = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doMoveToCell error:', err);
        _logError('Network error — could not process movement.');
    }
}

/** Brace into a readied stance — costs Action, consumes remaining movement. */
export async function doHoldPosition() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/hold', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doHoldPosition error:', err);
        _logError('Network error — could not process hold.');
    }
}

/** Move the player one cell in the given direction (D-pad press). */
export async function doStep(dx, dy) {
    if (!_lastState?.player_pos) return;
    const x = _lastState.player_pos.x + dx;
    const y = _lastState.player_pos.y + dy;
    await doMoveToCell(x, y);
}

/** Placeholder for Spell / Use Item / Ability buttons until those systems ship. */
export function doStubAction(name) {
    _logInfo(`🚧 ${name} — coming soon.`);
}

export async function doFlee() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/flee', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doFlee error:', err);
        _logError('Network error — could not process flee attempt.');
    }
}

/** End the player's turn — triggers the monster's response turn on the server. */
export async function doEndTurn() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/end-turn', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doEndTurn error:', err);
        _logError('Network error — could not end turn.');
    }
}

export async function rollDeathSave() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/death-save', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('rollDeathSave error:', err);
    }
}

export async function endCombat() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp   = await combatPost('/api/combat/end', { npub, save_id: saveID });
        const result = await resp.json();
        if (!resp.ok || !result.success) {
            _logError(result.error ?? 'Failed to end combat.');
            return;
        }
        window.showMessage?.(result.message,
            result.outcome === 'victory' ? 'success' : 'error');
        exitCombatMode();
        if (window.refreshGameState) await window.refreshGameState();
    } catch (err) {
        logger.error('endCombat error:', err);
    }
}

// ─── Page-load combat resume ──────────────────────────────────────────────────
async function checkActiveCombatOnLoad() {
    if (_checkedOnLoad) return;
    _checkedOnLoad = true;
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await fetch(
            `/api/combat/state?npub=${encodeURIComponent(npub)}&save_id=${encodeURIComponent(saveID)}`
        );
        if (!resp.ok) return;
        const cs = await resp.json();
        if (cs.phase) {
            logger.info('⚔️  Resuming active combat after page load');
            enterCombatMode(cs);
        }
    } catch (_) { /* no active combat */ }
}
eventBus.on('gameStateLoaded', checkActiveCombatOnLoad);

// ─── Left text-box replacement ────────────────────────────────────────────────
function _replaceGameText() {
    const gameText = $id('game-text');
    if (!gameText) return;
    _cachedGameText = gameText.innerHTML;
    gameText.innerHTML =
        `<div id="combat-log" style="display:flex;flex-direction:column;gap:0;"></div>`;
}

function _restoreGameText() {
    if (_cachedGameText === null) return;
    const gameText = $id('game-text');
    if (gameText) gameText.innerHTML = _cachedGameText;
    _cachedGameText = null;
}

// ─── Combat log helpers ───────────────────────────────────────────────────────

const LOG_MS_INITIAL = 80;
const LOG_MS_ROUND   = 420;

function _appendSingleEntry(logEl, line, isError = false) {
    const p = document.createElement('p');
    p.style.cssText =
        `margin:0; padding:1px 2px; line-height:1.3; font-size:9px;
         border-bottom:1px solid rgba(255,255,255,0.04);
         color:${isError ? '#f87171' : '#e5e7eb'};`;
    p.textContent = `> ${line}`;
    logEl.appendChild(p);
    if (!isError) setTimeout(() => { p.style.color = '#9ca3af'; }, 4000);
}

function _appendLogEntriesStaggered(logEl, lines, isError = false, intervalMs = LOG_MS_ROUND) {
    const filtered = lines.map(r => r.trim()).filter(Boolean);
    filtered.forEach((line, i) => {
        setTimeout(() => {
            _appendSingleEntry(logEl, line, isError);
            _scrollLogToBottom();
            if (!isError) _spawnFlair([line]);
        }, i * intervalMs);
    });
}

function _logError(msg) {
    const logEl = $id('combat-log');
    if (logEl) {
        _appendSingleEntry(logEl, `⚠ ${msg}`, true);
        _scrollLogToBottom();
    }
}

function _logInfo(msg) {
    const logEl = $id('combat-log');
    if (logEl) {
        _appendSingleEntry(logEl, msg, false);
        _scrollLogToBottom();
    }
}

function _scrollLogToBottom() {
    const gameText = $id('game-text');
    if (!gameText) return;
    const scrollBox = gameText.parentElement;
    if (scrollBox) scrollBox.scrollTop = scrollBox.scrollHeight;
}

// ─── Combat grid ──────────────────────────────────────────────────────────────

/** Build the 63 cell divs once. Cells are passive — movement is driven by the D-pad. */
function _ensureGrid() {
    const container = $id('combat-grid');
    if (!container || container.dataset.built) return;
    container.dataset.built = '1';

    for (let r = 0; r < GRID_ROWS; r++) {
        for (let c = 0; c < GRID_COLS; c++) {
            const cell = document.createElement('div');
            cell.id = `gc-${c}-${r}`;
            cell.className = 'combat-cell';
            cell.dataset.x = c;
            cell.dataset.y = r;
            container.appendChild(cell);
        }
    }
}

/** Redraw the grid: only the player and monster cells carry colour + emoji. */
function _updateGridHighlights(cs) {
    const playerPos  = cs.player_pos;
    const monsterPos = cs.monster_pos;
    if (!playerPos || !monsterPos) return;

    for (let r = 0; r < GRID_ROWS; r++) {
        for (let c = 0; c < GRID_COLS; c++) {
            const cell = $id(`gc-${c}-${r}`);
            if (!cell) continue;

            cell.style.background = '';
            cell.textContent = '';

            if (c === playerPos.x && r === playerPos.y) {
                cell.style.background = 'rgba(74,222,128,0.28)';
                cell.textContent = '⚔';
            } else if (c === monsterPos.x && r === monsterPos.y) {
                cell.style.background = 'rgba(239,68,68,0.28)';
                cell.textContent = '👹';
            }
        }
    }
}

/** Full grid update: ensure built, redraw cells. */
function _renderGrid(cs) {
    _ensureGrid();
    _updateGridHighlights(cs);
}

/** Redraw grid but with the monster at a custom position (for step animation). */
function _updateGridHighlightsAt(cs, mPos) {
    const playerPos = cs.player_pos;
    if (!playerPos || !mPos) return;
    for (let r = 0; r < GRID_ROWS; r++) {
        for (let c = 0; c < GRID_COLS; c++) {
            const cell = $id(`gc-${c}-${r}`);
            if (!cell) continue;
            cell.style.background = '';
            cell.textContent = '';
            if (c === playerPos.x && r === playerPos.y) {
                cell.style.background = 'rgba(74,222,128,0.28)';
                cell.textContent = '⚔';
            } else if (c === mPos.x && r === mPos.y) {
                cell.style.background = 'rgba(239,68,68,0.28)';
                cell.textContent = '👹';
            }
        }
    }
}

/** Greedy Chebyshev path from → to, excluding the start cell. */
function _chebyshevPath(from, to) {
    const path = [];
    let x = from.x, y = from.y;
    let guard = 0;
    while ((x !== to.x || y !== to.y) && guard++ < 32) {
        x += Math.sign(to.x - x);
        y += Math.sign(to.y - y);
        path.push({ x, y });
    }
    return path;
}

/** Render the grid, animating the monster step-by-step from prevMonsterPos. */
function _renderGridAnimated(cs, prevMonsterPos) {
    _ensureGrid();
    const newPos = cs.monster_pos;
    if (!prevMonsterPos || !newPos ||
        (prevMonsterPos.x === newPos.x && prevMonsterPos.y === newPos.y)) {
        _updateGridHighlights(cs);
        return;
    }
    _updateGridHighlightsAt(cs, prevMonsterPos);
    const path = _chebyshevPath(prevMonsterPos, newPos);
    path.forEach((p, i) => {
        setTimeout(() => {
            if (_lastState !== cs) return; // newer render superseded us
            _updateGridHighlightsAt(cs, p);
        }, (i + 1) * LOG_MS_ROUND);
    });
}

// ─── Flair popups ─────────────────────────────────────────────────────────────
const FLAIR_CFG = [
    { re: /you go first!/i,  text: '⚡ YOU GO FIRST!',  color: '#4ade80', size: '12px' },
    { re: /goes first!/i,    text: '⚡ ENEMY FIRST!',   color: '#f87171', size: '12px' },
    { re: /CRITICAL HIT/i,  text: '💥 CRIT!',  color: '#fbbf24', size: '14px' },
    { re: /critical miss/i, text: 'CRIT MISS', color: '#6b7280', size: '11px' },
    { re: /\bMISS\b/,       text: 'MISS',      color: '#6b7280', size: '11px' },
    { re: /you deal (\d+)/i, text: null, color: '#f87171', size: '13px' },
    { re: /deals (\d+)/i,    text: null, color: '#ef4444', size: '12px' },
    { re: /\+(\d+) XP/i,     text: null, color: '#4ade80', size: '11px' },
    { re: /readied stance pays off/i,        text: '⚡ COUNTER!',  color: '#fbbf24', size: '13px' },
    { re: /twist away just in time/i,        text: '⚡ EVADED!',   color: '#4ade80', size: '13px' },
    { re: /take the dodge action/i,   text: '🛡 DODGE!',   color: '#60a5fa', size: '12px' },
    { re: /manage to put enough distance/i, text: '🏃 ESCAPED!', color: '#4ade80', size: '13px' },
    { re: /cuts off your escape/i, text: 'CAUGHT!', color: '#f87171', size: '12px' },
    { re: /stabilised|revived/i, text: '💚 Stable', color: '#4ade80', size: '11px' },
    { re: /level up!/i,      text: '⬆ LEVEL UP!', color: '#fde047', size: '13px' },
];

function _spawnFlair(lines) {
    const zone = $id('combat-flair-zone');
    if (!zone) return;
    for (const raw of lines) {
        const line = raw.trim();
        for (const cfg of FLAIR_CFG) {
            const m = line.match(cfg.re);
            if (!m) continue;
            const text = cfg.text ?? (m[1] ? (cfg.color === '#4ade80' && cfg.re.source.includes('XP') ? `+${m[1]} XP` : `-${m[1]}`) : null);
            if (!text) continue;
            const xPct = 20 + Math.random() * 60;
            const yPct = 30 + Math.random() * 35;
            const el = document.createElement('span');
            el.className = 'combat-flair';
            el.style.cssText = `left:${xPct}%;top:${yPct}%;font-size:${cfg.size};color:${cfg.color};`;
            el.textContent = text;
            zone.appendChild(el);
            setTimeout(() => el.remove(), 1700);
            break;
        }
    }
}

// ─── Phase panels ─────────────────────────────────────────────────────────────
function _showPhasePanel(cs) {
    _hide('death-saves-panel');
    _hide('loot-panel');
    _hide('defeat-panel');

    switch (cs.phase) {
        case 'death_saves':
            _show('death-saves-panel');
            _renderDeathSavePips(cs.player);
            break;
        case 'loot':
        case 'victory':
            _show('loot-panel');
            _renderLootPanel(cs);
            break;
        case 'defeat':
            _show('defeat-panel');
            _renderDefeatPanel();
            break;
    }
}

function _renderDeathSavePips(player) {
    if (!player) return;
    const s = player.death_save_successes ?? 0;
    const f = player.death_save_failures  ?? 0;
    for (let i = 1; i <= 3; i++) {
        const se = $id(`ds-success-${i}`);
        if (se) se.style.background = i <= s ? '#166534' : '#111827';
        const fe = $id(`ds-fail-${i}`);
        if (fe) fe.style.background = i <= f ? '#7f1d1d' : '#111827';
    }
}

function _renderLootPanel(cs) {
    const xpEl = $id('combat-xp-earned');
    if (xpEl) {
        const xp = cs.xp_earned ?? 0;
        xpEl.textContent = xp > 0
            ? `+${xp} XP${cs.level_up_pending ? '  ⬆ LEVEL UP!' : ''}`
            : '';
    }
    const listEl = $id('loot-list');
    if (!listEl) return;
    listEl.innerHTML = '';
    const loot = cs.loot_rolled ?? [];
    if (!loot.length) { listEl.textContent = 'No loot.'; return; }
    for (const drop of loot) {
        const row = document.createElement('div');
        row.style.cssText = 'display:flex;align-items:center;gap:3px;margin-bottom:1px;';
        const img = document.createElement('img');
        img.style.cssText = 'width:12px;height:12px;image-rendering:pixelated;flex-shrink:0;';
        img.src = `/res/img/items/${drop.item}.png`;
        img.onerror = () => { img.style.display = 'none'; img.onerror = null; };
        const lbl = document.createElement('span');
        lbl.textContent = `${drop.item} ×${drop.quantity}`;
        row.appendChild(img);
        row.appendChild(lbl);
        listEl.appendChild(row);
    }
}

function _renderDefeatPanel() {
    const msgEl = $id('defeat-message');
    if (msgEl) msgEl.textContent =
        'You have fallen. You wake stripped of your belongings, but your spirit endures.';
}

// ─── Action-button replacement ────────────────────────────────────────────────

const _B = (extra = '') =>
    `width:100%;text-align:left;padding:2px 4px;font-size:9px;font-weight:bold;
     color:#fff;cursor:pointer;background:#2a2a2a;
     border-top:2px solid #4a4a4a;border-left:2px solid #4a4a4a;
     border-right:2px solid #1a1a1a;border-bottom:2px solid #1a1a1a;${extra}`;

const _B_DISABLED =
    `width:100%;text-align:left;padding:2px 4px;font-size:9px;font-weight:bold;
     color:#4b5563;cursor:not-allowed;background:#1a1a1a;
     border-top:2px solid #2a2a2a;border-left:2px solid #2a2a2a;
     border-right:2px solid #111;border-bottom:2px solid #111;`;

const _B_GRAYED = (label, title = '') =>
    `<button style="${_B_DISABLED}" disabled title="${title}">${label}</button>`;

function _replaceActionButtons(cs) {
    const navEl = $id('navigation-buttons');
    const bldEl = $id('building-buttons');
    const npcEl = $id('npc-buttons');
    if (navEl) _cachedNav = navEl.innerHTML;
    if (bldEl) _cachedBld = bldEl.innerHTML;
    if (npcEl) _cachedNpc = npcEl.innerHTML;
    _renderCombatButtons(cs);
}

// D-pad button styles — fixed height so all 3 rows fit the column
const _DPAD_SIZE = 22;
const _DPAD_BTN = `
    width:100%;height:${_DPAD_SIZE}px;display:flex;align-items:center;justify-content:center;
    font-size:12px;font-weight:bold;color:#fff;cursor:pointer;background:#2a2a2a;
    border-top:2px solid #4a4a4a;border-left:2px solid #4a4a4a;
    border-right:2px solid #1a1a1a;border-bottom:2px solid #1a1a1a;padding:0;line-height:1;`;
const _DPAD_BTN_DISABLED = `
    width:100%;height:${_DPAD_SIZE}px;display:flex;align-items:center;justify-content:center;
    font-size:12px;font-weight:bold;color:#4b5563;cursor:not-allowed;background:#1a1a1a;
    border-top:2px solid #2a2a2a;border-left:2px solid #2a2a2a;
    border-right:2px solid #111;border-bottom:2px solid #111;padding:0;line-height:1;`;

function _renderCombatButtons(cs) {
    const navEl = $id('navigation-buttons');
    const bldEl = $id('building-buttons');
    const npcEl = $id('npc-buttons');

    const phase = cs.phase ?? 'active';
    if (phase !== 'active' && phase !== 'death_saves') return;

    const actionUsed   = cs.action_used ?? false;
    const bonusUsed    = cs.bonus_action_used ?? false;
    const range        = cs.range ?? 0;
    const bonusAvail   = cs.bonus_attack_available ?? false;
    const ammo         = cs.ammo_remaining ?? 0;

    const budget    = cs.movement_budget ?? 0;
    const spent     = cs.movement_spent  ?? 0;
    const remaining = Math.max(0, budget - spent);

    const pPos = cs.player_pos ?? { x: -1, y: -1 };
    const mPos = cs.monster_pos ?? { x: -1, y: -1 };

    // ── Nav column: 3×3 D-pad ────────────────────────────────────────────────
    const dirs = [
        { dx: -1, dy: -1, label: '↖' }, { dx:  0, dy: -1, label: '↑' }, { dx:  1, dy: -1, label: '↗' },
        { dx: -1, dy:  0, label: '←' }, { dx:  0, dy:  0, label: null }, { dx:  1, dy:  0, label: '→' },
        { dx: -1, dy:  1, label: '↙' }, { dx:  0, dy:  1, label: '↓' }, { dx:  1, dy:  1, label: '↘' },
    ];
    const padCells = dirs.map(d => {
        if (d.label === null) {
            const color = remaining > 0 ? '#4ade80' : '#6b7280';
            return `<div style="display:flex;align-items:center;justify-content:center;
                        font-size:9px;font-weight:bold;color:${color};
                        background:rgba(0,0,0,0.4);height:${_DPAD_SIZE}px;line-height:1;text-align:center;">
                        ${remaining}/${budget}
                    </div>`;
        }
        const tx = pPos.x + d.dx, ty = pPos.y + d.dy;
        const oob        = tx < 0 || tx >= GRID_COLS || ty < 0 || ty >= GRID_ROWS;
        const intoMon    = tx === mPos.x && ty === mPos.y;
        const noBudget   = remaining <= 0;
        const disabled   = oob || intoMon || noBudget;
        const title = disabled
            ? (noBudget ? 'No movement left' : intoMon ? 'Blocked by enemy' : 'Out of bounds')
            : `Move ${d.label}`;
        return disabled
            ? `<button style="${_DPAD_BTN_DISABLED}" disabled title="${title}">${d.label}</button>`
            : `<button style="${_DPAD_BTN}" onclick="window.doStep(${d.dx},${d.dy})" title="${title}">${d.label}</button>`;
    }).join('');

    if (navEl) navEl.innerHTML = `
        <h3 style="color:#fbbf24;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Move</h3>
        <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:2px;">
            ${padCells}
        </div>`;

    // ── Building column: Attack main / off, Spell / Item / Ability stubs ─────
    const mainID    = _equippedWeaponID('mainHand');
    const isRanged  = mainID ? _isRangedWeapon(mainID) : false;
    const maxMelee  = mainID ? _meleeReach(mainID) : 1;
    const mainName  = _equippedWeaponName('mainHand') || 'Unarmed';
    const meleeBlocked  = !isRanged && range > maxMelee;
    const rangedBlocked = isRanged && ammo <= 0;

    let mainLabel = `⚔ ${mainName}`;
    if (isRanged && ammo > 0) mainLabel += ` (${ammo})`;

    let attackBtn;
    if (actionUsed)         attackBtn = _B_GRAYED(mainLabel, 'Action used this turn');
    else if (meleeBlocked)  attackBtn = _B_GRAYED(mainLabel, 'Out of melee range — step closer');
    else if (rangedBlocked) attackBtn = _B_GRAYED(mainLabel, 'No ammo');
    else attackBtn = `<button style="${_B()}" onclick="window.doAttack('main',false)">${mainLabel}</button>`;

    let bonusBtn = '';
    if (bonusAvail) {
        const offName = _equippedWeaponName('offHand') || 'Off Hand';
        bonusBtn = bonusUsed
            ? _B_GRAYED(`⚔ ${offName} (bonus)`, 'Bonus action used')
            : `<button style="${_B('background:#162716;')}" onclick="window.doAttack('off',false)">⚔ ${offName} (bonus)</button>`;
    }

    if (bldEl) bldEl.innerHTML = `
        <h3 style="color:#fbbf24;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Action</h3>
        <div style="display:flex;flex-direction:column;gap:2px;overflow:hidden;">
            ${attackBtn}
            ${bonusBtn}
            <button style="${_B('color:#c4b5fd;')}" onclick="window.doStubAction('Cast Spell')"
                    title="Coming soon">✨ Cast Spell</button>
            <button style="${_B('color:#fca5a5;')}" onclick="window.doStubAction('Use Item')"
                    title="Coming soon">🧪 Use Item</button>
            <button style="${_B('color:#fde047;')}" onclick="window.doStubAction('Ability')"
                    title="Coming soon">⚡ Ability</button>
        </div>`;

    // ── NPC column: Hold / Flee / End Turn ───────────────────────────────────
    const holdBtn = actionUsed
        ? _B_GRAYED('🛡 Hold Position', 'Action already used')
        : remaining <= 0
            ? _B_GRAYED('🛡 Hold Position', 'Need movement left to brace')
            : `<button style="${_B('color:#60a5fa;')}" onclick="window.doHoldPosition()"
                    title="Spend remaining movement to ready a counter-strike">🛡 Hold Position</button>`;

    const fleeBtn = actionUsed
        ? _B_GRAYED('🏃 Flee', 'Action already used')
        : range < 3
            ? _B_GRAYED('🏃 Flee (need range ≥ 3)', 'Move away on the grid first')
            : `<button style="${_B('color:#fbbf24;')}" onclick="window.doFlee()"
                    title="Attempt to escape — success chance based on range and speed">🏃 Flee</button>`;

    if (npcEl) npcEl.innerHTML = `
        <h3 style="color:#9ca3af;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Turn</h3>
        <div style="display:flex;flex-direction:column;gap:2px;">
            ${holdBtn}
            ${fleeBtn}
            <button style="${_B('color:#f87171;')}" onclick="window.doEndTurn()"
                    title="End your turn and let the monster act">⏭ End Turn</button>
        </div>`;

    // ── A/B resource badges ──────────────────────────────────────────────────
    _updateResourceBadges(actionUsed, bonusAvail, bonusUsed);
}

function _updateResourceBadges(actionUsed, bonusAvail, bonusUsed) {
    const aEl = $id('combat-action-badge');
    if (aEl) {
        if (actionUsed) {
            aEl.style.color = '#4b5563';
            aEl.style.borderColor = '#2a2a2a';
        } else {
            aEl.style.color = '#4ade80';
            aEl.style.borderColor = '#14532d';
        }
    }
    const bEl = $id('combat-bonus-badge');
    if (bEl) {
        if (!bonusAvail || bonusUsed) {
            bEl.style.color = '#4b5563';
            bEl.style.borderColor = '#2a2a2a';
        } else {
            bEl.style.color = '#60a5fa';
            bEl.style.borderColor = '#1e3a5a';
        }
    }
}

// ─── Weapon helpers ───────────────────────────────────────────────────────────

function _equippedWeaponID(slot) {
    try {
        const eq = window.getGameStateSync?.()?.equipment ?? {};
        const data = eq[slot] ?? eq[slot.toLowerCase()];
        return typeof data === 'string' ? data : (data?.item ?? null);
    } catch (_) { return null; }
}

function _equippedWeaponName(slot) {
    const id = _equippedWeaponID(slot);
    if (!id) return null;
    try {
        const item = window.getItemById?.(id);
        return item?.name ?? id;
    } catch (_) { return id; }
}

function _isRangedWeapon(itemID) {
    try {
        const item = window.getItemById?.(itemID);
        if (!item) return false;
        const tags = item.tags ?? [];
        return tags.some(t => ['ammunition', 'thrown', 'range'].includes(String(t).toLowerCase()));
    } catch (_) { return false; }
}

function _meleeReach(itemID) {
    try {
        const item = window.getItemById?.(itemID);
        if (!item) return 1;
        const tags = item.tags ?? [];
        return tags.some(t => String(t).toLowerCase() === 'reach') ? 2 : 1;
    } catch (_) { return 1; }
}

// ─── Side-panel sync ──────────────────────────────────────────────────────────

function _syncSidePanelFromCombat(cs) {
    const state = window.getGameStateSync?.();
    if (!state?.character) return;

    if (cs.player) {
        state.character.hp     = cs.player.current_hp;
        state.character.max_hp = cs.player.max_hp;
    }

    const xpEarned = cs.xp_earned ?? 0;
    state.character.experience = _baseExperience + xpEarned;

    window.updateCharacterDisplay?.();
}

function _restoreActionButtons() {
    if (_cachedNav !== null) { const e = $id('navigation-buttons'); if (e) e.innerHTML = _cachedNav; _cachedNav = null; }
    if (_cachedBld !== null) { const e = $id('building-buttons');   if (e) e.innerHTML = _cachedBld; _cachedBld = null; }
    if (_cachedNpc !== null) { const e = $id('npc-buttons');        if (e) e.innerHTML = _cachedNpc; _cachedNpc = null; }
    window.displayCurrentLocation?.().catch(() => {});
}
