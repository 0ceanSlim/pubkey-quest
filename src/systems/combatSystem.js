/**
 * Combat System
 *
 * Drives the combat UI and communicates with the server-side combat engine.
 *
 * Layout per spec (section 19):
 *   - Scene area:   monster image + HP bar / name / round / range / conditions
 *   - Left text box (#game-text): replaced with scrolling combat log during combat
 *   - Bottom buttons: replaced with Attack / Move / Flee submenus
 *   - Right panel:  player stats — unchanged
 *
 * @module systems/combatSystem
 */

import { logger }   from '../lib/logger.js';
import { eventBus } from '../lib/events.js';
import { gameAPI }  from '../lib/api.js';

// ─── Range descriptions (combat plan §2) ─────────────────────────────────────
const RANGE_LABELS = {
    0: 'In contact',
    1: 'Adjacent',
    2: 'Short range',
    3: 'Medium range',
    4: 'Long range',
    5: 'Very long range',
    6: 'Extreme range',
};

// ─── Module state ─────────────────────────────────────────────────────────────
let _cachedNav       = null;   // innerHTML caches for action columns
let _cachedBld       = null;
let _cachedNpc       = null;
let _cachedGameText  = null;   // innerHTML cache for left text box
let _lastState       = null;   // last successful CombatStateResponse
let _checkedOnLoad   = false;  // page-load resume guard

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
    try {
        const resp = await combatPost('/api/combat/debug/start', { npub, save_id: saveID });
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
    _lastState = cs;

    const monster = cs.monsters?.[0];

    // ── Scene overlay: monster panel ─────────────────────────────────────────
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

    // ── Scene overlay: round + range ─────────────────────────────────────────
    _setText('combat-round-display', `Round ${cs.round ?? 1}`);
    const range = cs.range ?? 0;
    _setText('combat-range-display', `Range ${range} — ${RANGE_LABELS[range] ?? 'Unknown'}`);

    // ── Left text box: combat log (staggered) ────────────────────────────────
    const logEl = $id('combat-log');
    if (logEl) {
        const isFirstRender = logEl.childElementCount === 0;
        const entries = isFirstRender
            ? (cs.log ?? [])
            : (cs.new_log?.length ? cs.new_log : []);
        if (entries.length) {
            // Initial dump uses a faster interval; per-round entries are slower so
            // each roll result can be read and flair can be seen individually.
            const interval = isFirstRender ? LOG_MS_INITIAL : LOG_MS_ROUND;
            _appendLogEntriesStaggered(logEl, entries, false, interval);
        }
    }
    // Note: flair is now fired per-entry inside _appendLogEntriesStaggered —
    // no separate _spawnFlair call needed here.

    // ── Action buttons ────────────────────────────────────────────────────────
    if (_cachedNav !== null) _renderCombatButtons(cs);

    // ── Phase panels ─────────────────────────────────────────────────────────
    _showPhasePanel(cs);
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

export async function doMove(dir) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/move', {
            npub, save_id: saveID,
            move_dir: dir,
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
        logger.error('doMove error:', err);
        _logError('Network error — could not process movement.');
    }
}

export async function passTurn() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/pass', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('passTurn error:', err);
        _logError('Network error — could not pass turn.');
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

const LOG_MS_INITIAL = 80;   // Fast stagger for the first dump at combat start
const LOG_MS_ROUND   = 420;  // Readable stagger for each round's entries

/** Append one log entry immediately. */
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

/**
 * Stagger-append log entries, firing flair for each line as it appears.
 * @param {HTMLElement} logEl
 * @param {string[]}    lines
 * @param {boolean}     isError
 * @param {number}      intervalMs  — delay between entries (ms)
 */
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

function _scrollLogToBottom() {
    // The scrollable container is game-text's parent (the div with overflow-y-auto)
    const gameText = $id('game-text');
    if (!gameText) return;
    const scrollBox = gameText.parentElement;
    if (scrollBox) scrollBox.scrollTop = scrollBox.scrollHeight;
}

// ─── Flair popups ─────────────────────────────────────────────────────────────
const FLAIR_CFG = [
    // Initiative result (checked first so the pattern fires on combat start)
    { re: /you go first!/i,  text: '⚡ YOU GO FIRST!',  color: '#4ade80', size: '12px' },
    { re: /goes first!/i,    text: '⚡ ENEMY FIRST!',   color: '#f87171', size: '12px' },
    // Attack outcomes
    { re: /CRITICAL HIT/i,  text: '💥 CRIT!',  color: '#fbbf24', size: '14px' },
    { re: /critical miss/i, text: 'CRIT MISS', color: '#6b7280', size: '11px' },
    { re: /\bMISS\b/,       text: 'MISS',      color: '#6b7280', size: '11px' },
    { re: /you deal (\d+)/i, text: null, color: '#f87171', size: '13px' },  // player hits monster
    { re: /deals (\d+)/i,    text: null, color: '#ef4444', size: '12px' },  // monster hits player
    { re: /\+(\d+) XP/i,     text: null, color: '#4ade80', size: '11px' },
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

// Win95-style button CSS
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

function _renderCombatButtons(cs) {
    const navEl = $id('navigation-buttons');
    const bldEl = $id('building-buttons');
    const npcEl = $id('npc-buttons');

    // Don't render combat buttons if the panels have taken over
    const phase = cs.phase ?? 'active';
    if (phase !== 'active' && phase !== 'death_saves') return;

    const turnPhase = cs.turn_phase ?? 'move';
    const range     = cs.range ?? 0;
    const bonus     = cs.bonus_attack_available ?? false;
    const ammo      = cs.ammo_remaining ?? 0;

    const mainID   = _equippedWeaponID('mainHand');
    const isRanged = mainID ? _isRangedWeapon(mainID) : false;
    const maxMelee = mainID ? _meleeReach(mainID) : 1;

    const phaseColor = turnPhase === 'move' ? '#93c5fd' : '#fbbf24';

    if (turnPhase === 'move') {
        // ── MOVE PHASE ───────────────────────────────────────────────────────
        const canAdvance = range > 0;
        const canRetreat = range < 6;

        if (navEl) navEl.innerHTML = `
            <h3 style="color:${phaseColor};font-size:8px;font-weight:bold;text-transform:uppercase;
                       margin-bottom:2px;">🏃 Move</h3>
            <div style="display:flex;flex-direction:column;gap:2px;">
                ${canAdvance
                    ? `<button style="${_B('color:#4ade80;')}" onclick="window.doMove(-1)">▼ Advance</button>`
                    : _B_GRAYED('▼ Advance (at contact)', 'Already at contact range')}
                <button style="${_B()}" onclick="window.doMove(0)"
                    title="Brace — if the enemy advances into reach, you counter-attack first with advantage">— Hold &amp; Ready</button>
                ${canRetreat
                    ? `<button style="${_B('color:#fb923c;')}" onclick="window.doMove(1)">▲ Retreat</button>`
                    : _B_GRAYED('▲ Retreat (max range)', 'Already at max range')}
            </div>`;

        if (bldEl) bldEl.innerHTML = `
            <h3 style="color:#4b5563;font-size:8px;font-weight:bold;text-transform:uppercase;
                       margin-bottom:2px;">Action</h3>
            <p style="font-size:8px;color:#4b5563;margin:0;">Move first, then choose your action.</p>`;

        if (npcEl) npcEl.innerHTML = `
            <h3 style="color:#4b5563;font-size:8px;font-weight:bold;text-transform:uppercase;
                       margin-bottom:2px;">Other</h3>`;

    } else {
        // ── ACTION PHASE ─────────────────────────────────────────────────────
        const mainName = _equippedWeaponName('mainHand') || 'Unarmed';
        const offName  = _equippedWeaponName('offHand')  || 'Off Hand';

        const meleeBlocked  = !isRanged && range > maxMelee;
        const rangedBlocked = isRanged && ammo <= 0;
        const attackBlocked = meleeBlocked || rangedBlocked;

        let mainLabel = `⚔ ${mainName}`;
        if (isRanged && ammo > 0) mainLabel += ` (${ammo})`;

        let attackBtn;
        if (attackBlocked) {
            const reason = meleeBlocked
                ? `out of melee range (${range} > ${maxMelee}) — should have advanced`
                : 'no ammo';
            attackBtn = _B_GRAYED(mainLabel, reason);
        } else {
            attackBtn = `<button style="${_B()}" onclick="window.doAttack('main',false)">${mainLabel}</button>`;
        }

        let bonusBtn = '';
        if (bonus) {
            bonusBtn = `<button style="${_B('background:#162716;')}"
                onclick="window.doAttack('off',false)">⚔ Bonus: ${offName}</button>`;
        }

        if (navEl) navEl.innerHTML = `
            <h3 style="color:${phaseColor};font-size:8px;font-weight:bold;text-transform:uppercase;
                       margin-bottom:2px;">⚔ Action</h3>
            <div style="display:flex;flex-direction:column;gap:2px;overflow:hidden;">
                ${attackBtn}
                ${bonusBtn}
                ${_B_GRAYED('Abilities (Phase 4)', 'Not yet implemented')}
                ${_B_GRAYED('Use Item (Phase 6)', 'Not yet implemented')}
            </div>`;

        if (bldEl) bldEl.innerHTML = `
            <h3 style="color:#4b5563;font-size:8px;font-weight:bold;text-transform:uppercase;
                       margin-bottom:2px;">Move</h3>
            <p style="font-size:8px;color:#4b5563;margin:0;">Movement used.</p>`;

        const canFlee = range >= 2;
        if (npcEl) npcEl.innerHTML = `
            <h3 style="color:#4b5563;font-size:8px;font-weight:bold;text-transform:uppercase;
                       margin-bottom:2px;">Other</h3>
            <div style="display:flex;flex-direction:column;gap:2px;">
                ${canFlee
                    ? `<button style="${_B('color:#fbbf24;')}" onclick="window.endCombat()">🏃 Flee</button>`
                    : _B_GRAYED('🏃 Flee (retreat first)', 'Move back to range ≥ 2 then flee')}
                <button style="${_B('color:#6b7280;')}" onclick="window.passTurn()"
                    title="Forfeit your action — monster still acts">⏭ End Turn</button>
            </div>`;
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

/** Returns the maximum melee range for a weapon (1 for normal, 2 for reach weapons). */
function _meleeReach(itemID) {
    try {
        const item = window.getItemById?.(itemID);
        if (!item) return 1;
        const tags = item.tags ?? [];
        return tags.some(t => String(t).toLowerCase() === 'reach') ? 2 : 1;
    } catch (_) { return 1; }
}

function _restoreActionButtons() {
    if (_cachedNav !== null) { const e = $id('navigation-buttons'); if (e) e.innerHTML = _cachedNav; _cachedNav = null; }
    if (_cachedBld !== null) { const e = $id('building-buttons');   if (e) e.innerHTML = _cachedBld; _cachedBld = null; }
    if (_cachedNpc !== null) { const e = $id('npc-buttons');        if (e) e.innerHTML = _cachedNpc; _cachedNpc = null; }
    window.displayCurrentLocation?.().catch(() => {});
}
