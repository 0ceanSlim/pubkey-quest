/**
 * Combat System
 *
 * Drives the combat UI and communicates with the server-side combat engine.
 *
 * @module systems/combatSystem
 */

import { logger }     from '../lib/logger.js';
import { eventBus }   from '../lib/events.js';
import { gameAPI }    from '../lib/api.js';
import { smoothClock } from './smoothClock.js';
import { restoreActionButtonsLayout, displayCurrentLocation, closeNPCDialogue } from '../ui/locationDisplay.js';
import { crestSVG }   from '../ui/crest.js';
import { portraitSrc } from '../ui/portraitDialog.js';
import { showSceneSpeech, hideSceneSpeech } from '../ui/sceneSpeech.js';

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

// ─── Grid size ────────────────────────────────────────────────────────────────
// The server sizes the board by how many are fighting (cs.grid); these track
// the board currently built in the DOM.
let _gridCols = 0;
let _gridRows = 0;
// Cells never grow past this, so a small fight keeps the scene art around it.
const _MAX_CELL_PX = 28;
const _MIN_CELL_PX = 12;

// ─── Module state ─────────────────────────────────────────────────────────────
let _cachedNav       = null;
let _cachedBld       = null;
let _cachedNpc       = null;
let _cachedGameText  = null;
let _lastState       = null;
let _checkedOnLoad   = false;
let _baseExperience  = 0;
// Multi-enemy fights (M5.6): the enemy the player is aiming at, where each
// monster token is currently drawn (lags the server while a move animates),
// and the last HP the bars were drained to (so a re-render doesn't skip the
// drain the log stager has scheduled).
let _targetID        = null;
let _shownPos        = {};
let _hpByID          = {};
// Party fights (P5): how much of the shared log we've shown, the ally picked
// for heals / potions / Help, and the timers that keep us in step with others.
let _logIndex        = null;
let _allyTargetID    = null;
let _partyPollTimer  = null;
let _countdownTimer  = null;
let _busy            = 0;

// ─── Helpers ──────────────────────────────────────────────────────────────────
const getNpub   = () => gameAPI.npub   ?? null;
const getSaveID = () => gameAPI.saveID ?? null;
const $id       = id  => document.getElementById(id);
const _show     = id  => $id(id)?.classList.remove('hidden');
const _hide     = id  => $id(id)?.classList.add('hidden');
const _setText  = (id, t) => { const e = $id(id); if (e) e.textContent = t; };

function combatPost(endpoint, body) {
    _busy++;
    return fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    }).finally(() => { _busy--; });
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
    const count = parseInt(document.getElementById('debug-monster-count')?.value ?? '1', 10) || 1;
    const opening = document.getElementById('debug-combat-opening')?.value ?? '';
    try {
        const resp = await combatPost('/api/combat/debug/start', { npub, save_id: saveID, monster_id: monsterID, count, opening });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            window.showMessage?.(cs.error ?? `HTTP ${resp.status}`, 'error');
            return;
        }
        if (cs.phase === 'unaware') { showUnawareChoice(cs.unaware); return; }
        enterCombatMode(cs);
    } catch (err) {
        logger.error('debugStartCombat error:', err);
        window.showMessage?.('Failed to start combat.', 'error');
    }
}

export function enterCombatMode(cs) {
    logger.info('⚔️  Entering combat mode');
    // Freeze in-game time while fighting — combat is turn-based. A hold, not a
    // pause: when the fight ends the clock goes back to whatever the player had
    // chosen. (Combat actions use their own endpoints, not the tick.)
    smoothClock.hold('combat');
    _baseExperience = window.getGameStateSync?.()?.character?.experience ?? 0;
    // Clear any stale pacing timers from a prior combat that ended mid-flush.
    _logQueueTailAt = 0;
    _animationEndsAt = 0;
    _targetID = null;
    _shownPos = {};
    _hpByID = {};
    _replaceGameText();
    // A fight can start mid-conversation (an encounter firing in a tavern while you
    // talk to the innkeeper). NPC dialogue hides the whole action bar behind its
    // option strip, so close it first or the combat buttons render into a hidden
    // bar. Closing restores whichever layout is underneath (travel flex or the
    // city grid), so it's safe even while the NPC's line is still typing out.
    if (document.getElementById('npc-dialogue-overlay')) closeNPCDialogue();
    // If we entered from a travel encounter, the action bar is the travel flex
    // layout (no navigation/building/npc sub-elements). Rebuild the normal grid
    // first so the combat buttons have somewhere to render. No-op outside travel.
    restoreActionButtonsLayout();
    const bar = document.getElementById('action-buttons');
    if (bar && bar.style.display === 'none') bar.style.display = '';
    _replaceActionButtons(cs);
    _show('combat-overlay');
    renderCombatState(cs);
}

export function exitCombatMode() {
    logger.info('🏳️  Exiting combat mode');
    smoothClock.release('combat'); // the clock returns to the player's choice
    _hide('combat-overlay');
    _restoreGameText();
    _restoreActionButtons();
    _hide('death-saves-panel');
    _hide('loot-panel');
    _hide('defeat-panel');
    _lastState = null;
    _logQueueTailAt = 0;
    _animationEndsAt = 0;
    _targetID = null;
    _shownPos = {};
    _hpByID = {};
    _logIndex = null;
    _allyTargetID = null;
    clearInterval(_partyPollTimer); _partyPollTimer = null;
    clearInterval(_countdownTimer); _countdownTimer = null;
    const bar = $id('action-buttons');
    if (bar) { bar.style.opacity = ''; bar.style.pointerEvents = ''; bar.title = ''; }
    // Re-render the scene we returned to so the correct action bar comes back —
    // in particular the travel controls when a fight ended mid-journey (the
    // cached bar restore above only rebuilds the generic grid).
    Promise.resolve(displayCurrentLocation?.()).catch((e) => logger.error('post-combat location refresh failed:', e));
}

/** Single re-render function — reads cs and updates every DOM region. */
export function renderCombatState(cs) {
    if (!cs) return;
    // Party fights share one log: show whatever was added since we last looked
    // (our own action, or other players' and monsters' turns in between).
    if (cs.party_fight) {
        if (_logIndex === null) cs.new_log = undefined; // first look: dump the log
        else cs.new_log = (cs.log ?? []).slice(_logIndex);
        _logIndex = cs.log_index ?? (cs.log?.length ?? 0);
        _startPartyTimers(cs);
    }
    // Where each monster was last drawn. On combat start, monsters that won
    // initiative carry `pos_before` (their spawn cell, before the opening move)
    // so their sprites enter from where they appeared instead of teleporting.
    const prevPos = _monsterPositions(_lastState);
    _lastState = cs;
    _pickTarget(cs);

    // ── Target panel + turn-order strip ─────────────────────────────────────
    _renderTargetPanel(cs);
    _renderTurnStrip(cs);
    _renderDifficultyBadge(cs.difficulty);
    // Player conditions (things afflicting you) render over the scene, in red.
    _renderConditionsInto('combat-conditions', cs.player?.conditions || [], '#fca5a5');
    // …and the spells helping you, in green.
    _renderBuffsInto('combat-conditions', cs.player);
    _fitGridArea();

    // ── Round + range + movement budget ──────────────────────────────────────
    _setText('combat-round-display', `Round ${cs.round ?? 1}`);
    _renderRangeDisplay(cs);
    const budget = cs.movement_budget ?? 6;
    const spent  = cs.movement_spent  ?? 0;
    _setText('combat-move-budget', `Move ${budget - spent}/${budget}`);

    // ── Grid baseline + movement context ────────────────────────────────────
    // Every monster that changed cell this response becomes a "mover": it is
    // drawn at its old cell until the log line announcing its move plays, then
    // glides along its path — so several monsters' turns read in order.
    _ensureGrid(cs);
    const movers = {};
    for (const m of (cs.monsters ?? [])) {
        if (!m.is_alive || m.fled || !m.pos) continue;
        const from = prevPos[m.instance_id] ?? m.pos_before ?? null;
        if (from && (from.x !== m.pos.x || from.y !== m.pos.y)) {
            movers[m.instance_id] = { name: m.name, path: _chebyshevPath(from, m.pos), scheduled: false };
        }
    }
    _shownPos = {};
    for (const id of Object.keys(movers)) _shownPos[id] = prevPos[id] ?? _findMonster(cs, id)?.pos_before;
    _paintGrid(cs);
    const gridCtx = { cs, movers };

    // ── Combat log (staggered) ───────────────────────────────────────────────
    // Pacing model:
    //   • If the response carries `new_log`, those are the fresh beats from
    //     this round / combat start — pace them with the full classified-delay
    //     pipeline (dice animations, grid sync, etc.).
    //   • If `new_log` is missing, this is a state-resume (page reload) —
    //     instant-dump cs.log so the player isn't held up replaying history.
    //   • Otherwise (subsequent renders with no new content), emit nothing.
    const logEl = $id('combat-log');
    if (logEl) {
        const hasNew = Array.isArray(cs.new_log) && cs.new_log.length > 0;
        const isResume = !hasNew && logEl.childElementCount === 0;
        const entries = hasNew ? cs.new_log : (isResume ? (cs.log ?? []) : []);

        if (entries.length) {
            const fixed = isResume ? LOG_MS_INITIAL : null;
            _appendLogEntriesStaggered(logEl, entries, false, fixed, gridCtx);
        }
    }
    // Any mover whose move line never played (rare) still animates, right away.
    for (const [id, mv] of Object.entries(movers)) {
        if (!mv.scheduled) _scheduleGridSteps(cs, id, mv, performance.now());
    }

    // ── Action buttons ────────────────────────────────────────────────────────
    if (_cachedNav !== null) _renderCombatButtons(cs);

    // ── Phase panels ─────────────────────────────────────────────────────────
    _showPhasePanel(cs);

    // ── HP / XP sync ─────────────────────────────────────────────────────────
    // When the new log contains damage or XP events the stager schedules HP
    // drains and XP ticks at the matching line emit times — see below in
    // _appendLogEntriesStaggered. When there are none (resume, pure-narrative
    // round, action with no damage), update synchronously so initial bars are
    // populated.
    const newLines = Array.isArray(cs.new_log) ? cs.new_log : [];
    const hasTimedHP = newLines.some(l => _diceDamageNumber(l) !== null)
                    || newLines.some(l => /^\s*\+\d+\s*XP/i.test(l));
    if (!hasTimedHP) {
        _renderMonsterHP(cs);
        _syncSidePanelFromCombat(cs);
    }
}

// ─── Targeting (M5.6) ─────────────────────────────────────────────────────────

const _isActive = m => m && m.is_alive && !m.fled;

function _findMonster(cs, id) {
    return (cs?.monsters ?? []).find(m => m.instance_id === id) ?? null;
}

/** instance_id → last known cell, for animating moves between responses. */
function _monsterPositions(cs) {
    const out = {};
    for (const m of (cs?.monsters ?? [])) if (m.pos) out[m.instance_id] = m.pos;
    return out;
}

/** Keep the chosen target while it's still in the fight; otherwise aim at the nearest enemy. */
function _pickTarget(cs) {
    if (_isActive(_findMonster(cs, _targetID))) return;
    let best = null;
    for (const m of (cs.monsters ?? [])) {
        if (_isActive(m) && (best === null || (m.range ?? 99) < (best.range ?? 99))) best = m;
    }
    _targetID = best?.instance_id ?? null;
}

/** The targeted monster — or, once nobody is left, the first one (for the panel). */
function _targetMonster(cs) {
    return _findMonster(cs, _targetID) ?? cs?.monsters?.[0] ?? null;
}

/** Aim subsequent attacks/spells at a monster (grid cell or turn-strip chip click). */
export function selectCombatTarget(instanceID) {
    const cs = _lastState;
    if (!cs || !_isActive(_findMonster(cs, instanceID))) return;
    _targetID = instanceID;
    _renderTargetPanel(cs);
    _renderTurnStrip(cs);
    _paintHPViews(cs);
    _renderRangeDisplay(cs);
    _paintGrid(cs);
    if (_cachedNav !== null && cs.phase === 'active') _renderCombatButtons(cs);
}

function _renderTargetPanel(cs) {
    const monster = _targetMonster(cs);
    if (!monster) return;
    _setText('combat-monster-name', monster.name ?? 'Unknown');
    const nameEl = $id('combat-monster-name');
    if (nameEl && monster.template_id) {
        nameEl.dataset.wiki = `Monsters/${monster.template_id}`;
        nameEl.dataset.wikiName = monster.name ?? '';
    }
    _setText('combat-monster-ac-badge', `AC ${monster.armor_class ?? '?'}`);
    _renderConditionsInto('combat-monster-conditions', monster.conditions || [], '#c4b5fd');
    const img = $id('combat-monster-img');
    const art = monster.template_id || monster.instance_id;
    if (img && art) {
        const newSrc = `/res/img/monsters/${art}.png`;
        if (!img.src.endsWith(newSrc)) {
            img.src = newSrc;
            img.onerror = () => { img.src = '/res/img/monsters/unknown.png'; img.onerror = null; };
        }
    }
}

/**
 * Turn-order strip: one chip per combatant in initiative order, so the player
 * can see who acts when. Enemy chips carry a mini HP bar and pick the target on
 * click. Hidden for a lone enemy — the top panel already says it all.
 */
function _renderTurnStrip(cs) {
    const el = $id('combat-turn-strip');
    if (!el) return;
    const monsters = cs.monsters ?? [];
    if (monsters.length <= 1 && !cs.party_fight) { el.style.display = 'none'; el.innerHTML = ''; return; }
    el.style.display = 'flex';
    el.innerHTML = '';
    const current = (cs.initiative ?? [])[cs.current_turn_index ?? -1];
    for (const entry of (cs.initiative ?? [])) {
        const chip = document.createElement('div');
        chip.className = 'combat-chip';
        if (entry.type === 'player') {
            const ally = (cs.allies ?? []).find(a => a.id === entry.id);
            const me = !cs.party_fight || ally?.you;
            chip.classList.add(me ? 'you' : 'ally');
            if (ally && (ally.unconscious || ally.fled)) chip.classList.add('down');
            const label = me ? 'You' : (ally?.name ?? entry.name ?? 'Ally');
            chip.textContent = ally?.unconscious ? `💀 ${label}` : label;
            if (cs.party_fight && current?.id === entry.id) {
                chip.classList.add('turn');
                chip.dataset.countdown = '1';
                chip.textContent = `${label} ⏱`;
            }
            chip.title = `${label} — initiative ${entry.initiative}`;
            if (!me && ally && !ally.fled) {
                chip.style.cursor = 'pointer';
                chip.addEventListener('click', () => _selectAlly(ally.id));
            }
            el.appendChild(chip);
            continue;
        }
        const m = _findMonster(cs, entry.id);
        if (!m) continue;
        const label = document.createElement('span');
        label.textContent = m.name;
        chip.appendChild(label);
        if (m.template_id) {
            chip.dataset.wiki = `Monsters/${m.template_id}`;
            chip.dataset.wikiName = m.name;
        }
        if (_isActive(m)) {
            chip.classList.add('foe');
            if (m.instance_id === _targetID) chip.classList.add('target');
            const hp = _hpByID[m.instance_id] ?? { cur: m.current_hp, max: m.max_hp };
            const bar = document.createElement('div');
            bar.className = 'combat-chip-hp';
            bar.innerHTML = `<div data-hp-for="${m.instance_id}" style="width:${_hpPct(hp.cur, hp.max)}%"></div>`;
            chip.appendChild(bar);
            chip.title = `${m.name} — range ${m.range}, initiative ${entry.initiative}. Click to target.`;
            chip.addEventListener('click', () => selectCombatTarget(m.instance_id));
        } else {
            chip.classList.add('down');
            chip.title = m.fled ? `${m.name} fled` : `${m.name} is down`;
        }
        el.appendChild(chip);
    }
}

/** The grid sits under the top bar, whose height grows with the turn strip. */
function _fitGridArea() {
    const bar = $id('combat-topbar');
    if (!bar) return;
    const h = bar.offsetHeight;
    if (!h) return;
    const area = $id('combat-grid-area');
    if (area) area.style.top = `${h + 4}px`;
    const conds = $id('combat-conditions');
    if (conds) conds.style.top = `${h}px`;
}

function _renderRangeDisplay(cs) {
    const target = _targetMonster(cs);
    const range  = (_isActive(target) ? target.range : cs.range) ?? 0;
    const wID    = _equippedWeaponID('mainHand');
    const wRange = wID ? _isRangedWeapon(wID) : false;
    const reach  = wID ? _meleeReach(wID) : 1;
    const inReach = !wRange && range <= reach;
    const reachTag = !wRange ? (inReach ? ' ⚔' : '') : '';
    let sightTag = '';
    if (_isActive(target)) {
        if (target.in_sight === false) sightTag = ' · 🚫 no sight';
        else if (target.cover > 0) sightTag = ` · 🛡+${target.cover}`;
    }
    _setText('combat-range-display', `Range ${range} — ${RANGE_LABELS[range] ?? ''}${reachTag}${sightTag}`);
}

export async function doAttack(hand = 'main', thrown = false) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/action', {
            npub, save_id: saveID, target_id: _targetID ?? '',
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

/** Disengage — costs Action, prevents OAs triggered by player movement this turn. */
export async function doDisengage() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost('/api/combat/disengage', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doDisengage error:', err);
        _logError('Network error — could not process disengage.');
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

/** Placeholder for Ability buttons until that system ships. */
export function doStubAction(name) {
    _logInfo(`🚧 ${name} — coming soon.`);
}

// ─── Cast / Use-Item (M4 Phase E) ──────────────────────────────────────────────

/** Cast a prepared spell at the targeted monster. Mirrors doAttack. */
export async function doCastSpell(spellId) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _closeCombatChooser();
    try {
        // Offensive spells go at the enemy you're aiming at; heals and buffs at
        // the ally you picked (or yourself).
        const sp = window.getSpellById?.(spellId);
        const offensive = !!(sp?.spell_attack || sp?.save_type || sp?.damage ||
            sp?.applies_condition || sp?.combat_buff?.target === 'enemy');
        const target = offensive ? (_targetID ?? '') : (_allyTargetID ?? '');
        const resp = await combatPost('/api/combat/cast', { npub, save_id: saveID, spell_id: spellId, target_id: target });
        const cs = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doCastSpell error:', err);
        _logError('Network error — could not cast.');
    }
}

/** Use a consumable during combat. Healing lands on the combat HP pool. */
export async function doUseCombatItem(itemId) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _closeCombatChooser();
    try {
        // A potion goes to the ally you picked (they must be next to you), else
        // yourself; a scroll's spell picks like a cast.
        const it = window.getItemById?.(itemId);
        const sp = it?.spell_id ? window.getSpellById?.(it.spell_id) : null;
        const offensive = !!(sp && (sp.spell_attack || sp.save_type || sp.damage));
        const target = offensive ? (_targetID ?? '') : (_allyTargetID ?? '');
        const resp = await combatPost('/api/combat/use-item', { npub, save_id: saveID, item_id: itemId, target_id: target });
        const cs = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doUseCombatItem error:', err);
        _logError('Network error — could not use item.');
    }
}

/** Shape glyph for a spell — mirrors the engine's shape resolution. */
function _spellShapeIcon(spell) {
    if (spell.spell_attack) return '⚔';
    if (spell.save_type)    return '🎯';
    if (spell.heal)         return '❤';
    if (spell.effect)       return '✨';
    return '•';
}

/** Cells a spell reaches the one it's aimed at. Mirrors the Go spellReach. */
function _spellReach(spell) {
    const r = parseInt(spell.range, 10) || 0;
    if (r > 0) return r;
    return parseInt(spell.range_long, 10) || 1; // thrown (produce flame), else touch
}

/**
 * Why a spell can't reach the one it would be aimed at ('' when it can).
 * Mirrors the server's check (combat/spellrange.go): an enemy spell goes at the
 * targeted monster, a heal / cure / ally buff at the picked ally, and an area
 * spreading from the caster reaches as far as its own size.
 */
function _spellReachProblem(spell, cs) {
    if (!cs) return '';
    const shape = spell.aoe?.shape;
    if (shape && shape !== 'sphere') return '';
    const reach = _spellReach(spell);
    const atk = String(spell.spell_attack ?? '').toLowerCase();
    const hostile = ['ranged', 'melee', 'automatic'].includes(atk) || !!spell.save_type ||
        !!spell.applies_condition || spell.combat_buff?.target === 'enemy';
    let name, dist;
    if (hostile) {
        const m = _targetMonster(cs);
        if (!_isActive(m)) return '';
        name = m.name;
        dist = m.range ?? 0;
    } else {
        const aimsAtAlly = !!spell.heal || (spell.cures?.length ?? 0) > 0 || spell.combat_buff?.target === 'ally';
        const me = (cs.allies ?? []).find(a => a.you);
        const ally = aimsAtAlly ? (cs.allies ?? []).find(a => a.id === _allyTargetID && !a.you) : null;
        if (!me || !ally) return '';
        name = ally.name;
        dist = Math.max(Math.abs(ally.pos.x - me.pos.x), Math.abs(ally.pos.y - me.pos.y));
    }
    if (dist <= reach) return '';
    if (!(parseInt(spell.range, 10) > 0) && reach === 1) return `a touch spell, so move next to ${name} first`;
    return `${name} is ${dist * 5} ft away, and it reaches only ${reach * 5} ft`;
}

/** Read a possibly-nested item field (getItemById does NOT flatten .properties). */
function _itemField(item, name) {
    return item?.[name] ?? item?.properties?.[name];
}

/**
 * Count how many of a component item the player holds across loose general slots
 * and every worn container's contents. Mirrors the Go engine's countComponent.
 */
function _countComponent(inv, itemId) {
    if (!inv || !itemId) return 0;
    let n = 0;
    const tally = (slot) => {
        if (!slot) return;
        if (slot.item === itemId) n += slot.quantity ?? 0;
        for (const c of (slot.contents ?? [])) if (c?.item === itemId) n += c.quantity ?? 0;
    };
    for (const slot of (inv.general_slots ?? [])) tally(slot);
    for (const slot of Object.values(inv.gear_slots ?? {})) tally(slot);
    return n;
}

/**
 * True if an equipped focus grants unlimited of componentId. Mirrors the Go
 * engine's equippedFocusProvides (a focus is tagged 'focus' with provides === id).
 */
function _equippedFocusProvides(inv, componentId) {
    for (const slot of Object.values(inv?.gear_slots ?? {})) {
        const id = slot?.item;
        if (!id) continue;
        const item = window.getItemById?.(id);
        if (!item) continue;
        const tags = (_itemField(item, 'tags') ?? []).map((t) => String(t).toLowerCase());
        if (tags.includes('focus') && _itemField(item, 'provides') === componentId) return true;
    }
    return false;
}

/** Open the combat spell menu: prepared spells with mana + component have/need. */
export function openCombatSpellMenu() {
    const ch      = window.getGameStateSync?.()?.character ?? {};
    const mana    = ch.mana ?? 0;
    const maxMana = ch.max_mana ?? mana;
    const inv     = ch.inventory ?? {};
    const slots   = ch.spell_slots ?? {};
    const seen    = new Set();
    const entries = [];

    for (const level of Object.keys(slots)) {
        for (const s of (slots[level] ?? [])) {
            const id = s?.spell;
            if (!id || seen.has(id)) continue;
            seen.add(id);
            const spell = window.getSpellById?.(id);
            if (!spell) continue;

            const cost = spell.mana_cost ?? 0;
            const manaOK = mana >= cost;

            // Components: per requirement show have/need, or */need when a focus
            // provides it unlimited (mirrors the cast engine's gating).
            const required = spell.material_component?.required ?? [];
            const compParts = [];   // compact meta, e.g. "1/2" or "*/1"
            const compTips = [];    // named breakdown for the tooltip
            let compsOK = true;
            for (const req of required) {
                const compId = req.component;
                const need = req.quantity ?? 1;
                const name = window.getItemById?.(compId)?.name || String(compId).replace(/[-_]/g, ' ');
                if (_equippedFocusProvides(inv, compId)) {
                    compParts.push(`*/${need}`);
                    compTips.push(`${name}: */${need} (focus — unlimited)`);
                } else {
                    const have = _countComponent(inv, compId);
                    if (have < need) compsOK = false;
                    compParts.push(`${have}/${need}`);
                    compTips.push(`${name}: ${have}/${need}`);
                }
            }

            const reaction = spell.action_cost === 'reaction';
            const farOff = reaction ? '' : _spellReachProblem(spell, _lastState);
            const usable = manaOK && compsOK && !reaction && !farOff;
            let meta = `${_spellShapeIcon(spell)} ${cost}◆`;
            if (reaction) meta = `⚡ auto ${cost}◆`;
            if (compParts.length) meta += ` 🜂${compParts.join(' ')}`;

            const tip = [
                spell.description ?? '',
                `Mana: ${mana}/${cost}${manaOK ? '' : ' — short'}`,
                ...compTips,
                reaction ? '⚡ A reaction: it casts itself when its moment comes, if you have the mana.' :
                    !usable ? '⚠ Can’t cast: ' + (!manaOK ? 'not enough mana' : !compsOK ? 'missing components' : farOff) : '',
            ].filter(Boolean).join('\n');

            entries.push({
                label: spell.name ?? id,
                meta,
                disabled: !usable,
                tip,
                onClick: () => window.doCastSpell(id),
            });
        }
    }

    if (entries.length === 0) {
        _logInfo('No spells prepared — prepare one on the Spells tab first.');
        return;
    }
    _openCombatChooser(`✨ Cast a spell — ${mana}/${maxMana}◆ mana`, entries);
}

/**
 * Open the combat item menu. Items at hand — the general slots and pouches in
 * them — cost only your action. Items in your backpack are listed too, but
 * digging one out costs your bonus action as well (the server enforces it).
 */
export function openCombatItemMenu() {
    const inv = window.getGameStateSync?.()?.character?.inventory ?? {};
    const gen = inv.general_slots ?? [];
    const pack = inv.gear_slots?.bag?.contents ?? [];
    const atHand = new Map();
    const inPack = new Map();

    const tally = (into, slot) => {
        const id = slot?.item;
        if (!id) return;
        const item = window.getItemById?.(id);
        const tags = (item?.tags ?? []).map(t => String(t).toLowerCase());
        if (!tags.includes('consumable')) return;
        into.set(id, (into.get(id) ?? 0) + (slot.quantity ?? 0));
    };
    for (const slot of gen) {
        if (!slot) continue;
        tally(atHand, slot);
        for (const c of (slot.contents ?? [])) tally(atHand, c);
    }
    for (const slot of pack) tally(inPack, slot);

    const bonusUsed = _lastState?.bonus_action_used ?? false;
    const entries = [];
    for (const [id, qty] of atHand) {
        if (qty <= 0) continue;
        const item = window.getItemById?.(id);
        entries.push({
            label: item?.name ?? id,
            meta:  `×${qty}`,
            disabled: false,
            tip: item?.description ?? '',
            onClick: () => window.doUseCombatItem(id),
        });
    }
    for (const [id, qty] of inPack) {
        if (qty <= 0 || atHand.has(id)) continue;
        const item = window.getItemById?.(id);
        entries.push({
            label: `${item?.name ?? id} (in pack)`,
            meta:  `×${qty} · +bonus action`,
            disabled: bonusUsed,
            tip: bonusUsed
                ? 'In your backpack, and your bonus action is used. Keep what you need in your general slots or a pouch.'
                : 'In your backpack: digging it out also uses your bonus action.',
            onClick: () => window.doUseCombatItem(id),
        });
    }

    if (entries.length === 0) {
        _logInfo('No usable items on you.');
        return;
    }
    _openCombatChooser('🧪 Use an item', entries);
}

// ─── Class abilities (M5 Slice 3) ──────────────────────────────────────────────

// Abilities wired into the combat engine (must mirror abilityMechanics in the Go
// backend). Others in the class list aren't usable in combat yet, so we hide them
// rather than surface a button the server would reject.
const _COMBAT_ABILITIES = new Set([
    'enter-rage', 'intimidating-roar',      // barbarian
    'second-wind', 'action-surge',          // fighter
    'flurry-of-blows', 'patient-defense',   // monk
    'sneak-attack', 'shadow-step',          // rogue
]);

/** Activate a class ability, spending the resource pool. Mirrors doCastSpell. */
export async function doUseAbility(abilityId) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _closeCombatChooser();
    try {
        const resp = await combatPost('/api/combat/ability', { npub, save_id: saveID, ability_id: abilityId });
        const cs = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error('doUseAbility error:', err);
        _logError('Network error — could not use ability.');
    }
}

/** Open the combat ability menu: unlocked, in-combat abilities gated by resource + cooldown. */
export async function openCombatAbilityMenu() {
    const pool = _lastState?.player?.resource;
    if (!pool) {
        _logInfo('Your class has no combat abilities.');
        return;
    }
    const ch    = window.getGameStateSync?.()?.character ?? {};
    const cls   = String(ch.class ?? '').toLowerCase();
    const level = ch.level ?? 1;
    const used  = new Set(_lastState?.player?.abilities_used ?? []);

    let list = [];
    try {
        const resp = await fetch(`/api/abilities?class=${encodeURIComponent(cls)}&level=${level}`);
        const data = await resp.json();
        if (!resp.ok || !data.success) {
            _logError(data.error ?? `HTTP ${resp.status}`);
            return;
        }
        list = data.abilities ?? [];
    } catch (err) {
        logger.error('openCombatAbilityMenu fetch error:', err);
        _logError('Network error — could not load abilities.');
        return;
    }

    const entries = [];
    for (const a of list) {
        if (!_COMBAT_ABILITIES.has(a.id) || !a.is_unlocked) continue;
        const cost = a.current_tier?.override_cost ?? a.resource_cost ?? 0;
        const spent = used.has(a.id);
        const affordable = pool.current >= cost;
        const disabled = spent || !affordable;
        let tip = a.current_tier?.summary ?? a.description ?? '';
        if (spent) tip = 'Already used this fight';
        else if (!affordable) tip = `Not enough ${pool.label} — need ${cost}, have ${pool.current}`;
        entries.push({
            label: a.name ?? a.id,
            meta:  spent ? '✓ used' : `${cost} ${pool.label}`,
            disabled,
            tip,
            onClick: () => window.doUseAbility(a.id),
        });
    }

    if (entries.length === 0) {
        _logInfo('No abilities available right now.');
        return;
    }
    _openCombatChooser(`✦ ${pool.label} — ${pool.current}/${pool.max}`, entries);
}

// ─── Combat chooser popup ──────────────────────────────────────────────────────

/** Remove the combat chooser popup + backdrop if present. */
function _closeCombatChooser() {
    document.getElementById('combat-chooser-backdrop')?.remove();
    document.getElementById('combat-chooser')?.remove();
}

/**
 * Render a compact win95-dark chooser above the action bar.
 * entries: [{ label, meta, disabled, tip, onClick }]
 */
function _openCombatChooser(title, entries) {
    _closeCombatChooser();

    const backdrop = document.createElement('div');
    backdrop.id = 'combat-chooser-backdrop';
    backdrop.style.cssText = 'position:fixed;inset:0;z-index:199;background:transparent;';
    backdrop.addEventListener('click', _closeCombatChooser);
    document.body.appendChild(backdrop);

    const panel = document.createElement('div');
    panel.id = 'combat-chooser';
    panel.style.cssText = `position:fixed;left:50%;bottom:96px;transform:translateX(-50%);
        z-index:200;min-width:220px;max-width:340px;max-height:52vh;overflow-y:auto;
        background:#1a1a1a;padding:6px;image-rendering:pixelated;
        border-top:2px solid #4a4a4a;border-left:2px solid #4a4a4a;
        border-right:2px solid #0a0a0a;border-bottom:2px solid #0a0a0a;
        box-shadow:0 6px 24px rgba(0,0,0,0.6);`;

    const head = document.createElement('div');
    head.style.cssText = `display:flex;justify-content:space-between;align-items:center;
        color:#fbbf24;font-size:9px;font-weight:bold;text-transform:uppercase;
        margin-bottom:4px;padding:0 2px;`;
    head.innerHTML = `<span>${title}</span>`;
    const close = document.createElement('button');
    close.textContent = '✕';
    close.style.cssText = 'color:#9ca3af;background:none;border:none;cursor:pointer;font-size:11px;';
    close.addEventListener('click', _closeCombatChooser);
    head.appendChild(close);
    panel.appendChild(head);

    for (const e of entries) {
        const btn = document.createElement('button');
        btn.style.cssText = e.disabled ? _B_DISABLED : _B();
        btn.style.display = 'flex';
        btn.style.justifyContent = 'space-between';
        btn.style.alignItems = 'center';
        btn.style.marginBottom = '2px';
        btn.title = e.tip ?? '';
        btn.innerHTML = `<span>${e.label}</span><span style="opacity:0.8;margin-left:8px;">${e.meta ?? ''}</span>`;
        if (e.disabled) {
            btn.disabled = true;
        } else {
            btn.addEventListener('click', () => { e.onClick(); });
        }
        panel.appendChild(btn);
    }

    document.body.appendChild(panel);
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

/** End the player's turn — the server runs every monster's turn in initiative order. */
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

// One death save at a time: the button locks on click and stays locked until
// the roll's log and dice have played out and the bar is drawn again (or the
// fight moves on), so mashing it can't queue rolls ahead of the animation.
let _deathSaveBusy = false;

export async function rollDeathSave() {
    if (_deathSaveBusy) return;
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _deathSaveBusy = true;
    const btn = $id('death-save-roll-btn');
    if (btn) {
        btn.disabled = true;
        btn.style.opacity = '0.5';
        btn.style.cursor = 'default';
        btn.textContent = 'Rolling…';
    }
    try {
        const resp = await combatPost('/api/combat/death-save', { npub, save_id: saveID });
        const cs   = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            _deathSaveBusy = false;
            if (_lastState) _renderDeathSaveBar(_lastState);
            return;
        }
        renderCombatState(cs); // releases the lock once the result has played out
    } catch (err) {
        logger.error('rollDeathSave error:', err);
        _deathSaveBusy = false;
        if (_lastState) _renderDeathSaveBar(_lastState);
    }
}

/**
 * Defeated? Go back to the last save instead — fights aren't saved, so this is
 * how you retry a boss you saved in front of, with nothing lost but the time
 * since. The server rebuilds the session from the save (relay-aware); a full
 * page reload then redraws everything from it.
 */
export async function reloadLastSave() {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    const btn = $id('defeat-reload-btn');
    if (btn) { btn.disabled = true; btn.textContent = 'Reloading…'; }
    try {
        const resp = await fetch('/api/session/reload', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ npub, save_id: saveID }),
        });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        window.location.reload();
    } catch (err) {
        logger.error('reloadLastSave error:', err);
        _logError("Couldn't load your last save. Have you saved this character yet?");
        if (btn) { btn.disabled = false; btn.textContent = 'Reload last save'; }
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
        if (result.level_up?.leveled) window.showLevelUpModal?.(result.level_up);

        // If this fight happened inside a POI walk, reopen the exploration overlay
        // at the node past the monster (server resumed it on victory).
        if (result.poi_resumed) {
            try {
                const m = await import('../ui/poiExplore.js');
                await m.resumeFromCombat(result.poi_resumed);
            } catch (err) {
                logger.error('POI resume after combat failed:', err);
            }
        }
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
        if (cs.phase === 'unaware') {
            showUnawareChoice(cs.unaware);
        } else if (cs.phase) {
            logger.info('⚔️  Resuming active combat after page load');
            enterCombatMode(cs);
        }
    } catch (_) { /* no active combat */ }
}

// ─── Stealth openings (party-combat-plan §7) ──────────────────────────────────

/**
 * Monsters ahead haven't noticed you. The world holds still while you choose:
 * sneak away (a Stealth check — fail and the fight starts), sneak attack (you
 * act first, they lose their first turn, your attacks that turn have
 * advantage), or step out and face them.
 */
export function showUnawareChoice(view) {
    if (!view) return;
    smoothClock.hold('encounter');
    const names = [...new Set((view.monsters ?? []).map(m => m.name))].join(', ');
    showSceneSpeech({
        speaker: names ? `Unseen — ${names}` : 'Unseen',
        icon: '👁',
        text: view.text ?? "They haven't noticed you.",
        outcomes: view.log ?? [],
        buttons: [
            { label: '🤫 Sneak away',   onClick: () => _answerUnaware('sneak_away') },
            { label: '🗡 Sneak attack', onClick: () => _answerUnaware('sneak_attack') },
            { label: '⚔ Approach',      onClick: () => _answerUnaware('approach') },
        ],
    });
}

let _answeringUnaware = false;

async function _answerUnaware(choice) {
    if (_answeringUnaware) return;
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _answeringUnaware = true;
    try {
        const resp = await combatPost('/api/combat/approach', { npub, save_id: saveID, choice });
        const out  = await resp.json();
        if (!resp.ok || !out.success) {
            window.showMessage?.(out.error ?? `HTTP ${resp.status}`, 'error');
            hideSceneSpeech();
            smoothClock.release('encounter');
            return;
        }
        hideSceneSpeech();
        if (out.slipped_away) {
            for (const line of (out.log ?? [])) window.addGameLog?.(line.trim());
            window.showMessage?.(`🤫 ${out.message}`, 'success');
            smoothClock.release('encounter');
            return;
        }
        if (out.combat) enterCombatMode(out.combat);
        smoothClock.release('encounter'); // the fight holds the clock now
    } catch (err) {
        logger.error('approach error:', err);
        window.showMessage?.('Network error — try again.', 'error');
        smoothClock.release('encounter');
    } finally {
        _answeringUnaware = false;
    }
}

eventBus.on('combat:unaware', showUnawareChoice);

// Death outside combat (starvation, a road hazard, a POI trap). The server has
// already applied it — you wake in town, stripped — so, like a lost fight,
// offer to go back to the last save instead.
eventBus.on('player:died', (death) => {
    window.refreshGameState?.(true);
    showSceneSpeech({
        speaker: 'Fallen',
        icon: '💀',
        text: death?.message || 'You have fallen. You wake stripped of your belongings, but your spirit endures.',
        buttons: [
            { label: 'Carry on', onClick: () => hideSceneSpeech() },
            { label: 'Reload last save', onClick: () => reloadLastSave() },
        ],
    });
});

// A party member started a fight we're in (we hear it from the social poll):
// pull the fight's state and join in.
let _joining = false;
eventBus.on('social:in-combat', async () => {
    if (_lastState || _joining) return;
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _joining = true;
    try {
        const resp = await fetch(`/api/combat/state?npub=${encodeURIComponent(npub)}&save_id=${encodeURIComponent(saveID)}`);
        if (!resp.ok) return;
        const cs = await resp.json();
        if (cs?.phase && cs.phase !== 'unaware') {
            logger.info('⚔️  Joining the party\'s fight');
            enterCombatMode(cs);
        }
    } catch (_) { /* next poll */ } finally {
        _joining = false;
    }
});
eventBus.on('gameStateLoaded', checkActiveCombatOnLoad);

// A biome travel encounter fired on the world tick — drop straight into combat
// with the server-supplied state (same payload shape as a manual combat start).
eventBus.on('combat:started', enterCombatMode);

// ─── Left text-box replacement ────────────────────────────────────────────────
function _replaceGameText() {
    const gameText = $id('game-text');
    if (!gameText) return;
    _cachedGameText = gameText.innerHTML;
    gameText.innerHTML =
        `<div id="combat-log" style="display:flex;flex-direction:column;gap:0;"></div>`;
    eventBus.emit('log:replaced');
}

function _restoreGameText() {
    if (_cachedGameText === null) return;
    const gameText = $id('game-text');
    if (gameText) gameText.innerHTML = _cachedGameText;
    _cachedGameText = null;
    // Chat that arrived mid-fight is re-added by the social UI.
    eventBus.emit('log:restored');
}

// ─── Combat log helpers ───────────────────────────────────────────────────────

// Pause AFTER this kind of entry, before the next one appears. Classification
// drives combat's reading pace — dice rolls pause longest so the reader can see
// the roll before the outcome line arrives.
const LOG_MS_INITIAL     = 80;   // Initial log dump (state resume) — fast
const DELAY_ROLL         = 1400; // any "rolled N" line (attack, initiative, death save)
const DELAY_CRIT         = 1800; // CRITICAL HIT or critical miss — dwell longer
const DELAY_DAMAGE       = 1100; // "deals N damage" / "You deal N damage"
const DELAY_KILL         = 1600; // "is defeated", "Victory", "falls unconscious"
const DELAY_OA           = 1100; // opportunity attack announcements
const DELAY_INITIATIVE   = 1500; // "Initiative: ... goes first!"
const DELAY_XP           = 600;  // XP / stat deltas
const DELAY_NARRATIVE    = 800;  // movement, disengage, brace, standard flavor
const DELAY_DEFAULT      = 800;

// classifyDelay picks how long to pause AFTER `line` displays.
// Returning the post-line delay lets us chain cumulative timings correctly.
function _classifyDelay(line) {
    const L = line;
    // The standalone outcome line (just emitted right after a roll) is the
    // verdict reveal — give it more weight than a normal narrative beat so
    // the player can see what happened before damage rolls in.
    if (/^\s*💥/.test(L))                                               return DELAY_CRIT;
    if (/^\s*✘\s+(MISS|Critical miss)/i.test(L))                        return DELAY_OA;
    if (/^\s*⚔\s+HIT/i.test(L))                                         return DELAY_OA;
    if (/CRITICAL HIT|Natural 20|Natural 1\b/i.test(L))                 return DELAY_CRIT;
    if (/Initiative:/i.test(L))                                          return DELAY_INITIATIVE;
    if (/opportunity attack|readied stance pays off/i.test(L))           return DELAY_OA;
    // Roll lines must hold long enough for the d20 animation (1400ms total)
    // to finish tumbling, settling, and fading before the outcome line runs.
    if (/\brolled\s+\d/i.test(L))                                        return DELAY_ROLL;
    if (/is defeated|Victory!|fall unconscious|have died|stabilised|regains consciousness|escape/i.test(L))
                                                                         return DELAY_KILL;
    if (/deals\s+\d+\s+\w+\s+damage|You deal\s+\d+/i.test(L))            return DELAY_DAMAGE;
    if (/^\s*\+\d+\s*XP/i.test(L))                                       return DELAY_XP;
    if (/moves\s+(toward|away)|You move|disengage|braces? yourself|readying/i.test(L))
                                                                         return DELAY_NARRATIVE;
    return DELAY_DEFAULT;
}

function _appendSingleEntry(logEl, line, isError = false) {
    const p = document.createElement('p');
    p.style.cssText =
        `margin:0; padding:1px 2px; line-height:1.3; font-size:9px;
         border-bottom:1px solid rgba(255,255,255,0.04);
         color:${isError ? '#f87171' : '#e5e7eb'};`;
    p.textContent = `> ${line}`;
    logEl.appendChild(p);
    if (!isError) setTimeout(() => { p.style.color = '#9ca3af'; }, 6000);
}

// Queue of deferred log flushes shared across multiple renderCombatState calls.
// A new round's entries wait for the previous round's pacing to finish, so
// rapid-fire server responses never collapse on top of each other.
let _logQueueTailAt  = 0;
// Absolute time (performance.now scale) when the active grid animation will
// finish. The log stager holds back entries that would otherwise appear while
// the monster is still gliding across the board.
let _animationEndsAt = 0;

// _diceRollNumber extracts the face from a "rolled N" line: a d20 for attacks,
// initiative, saves and checks, a d100 for an escape chance ("Escape chance:
// 45% (rolled 37)"). Returns null when the line isn't a single-die roll.
// Initiative is emitted as two separate per-side rolls so each one fires its
// own animation.
function _diceRollNumber(line) {
    const m = line.match(/rolled\s+(\d+)/i);
    if (!m) return null;
    const n = parseInt(m[1], 10);
    const max = _rollSides(line);
    if (!Number.isFinite(n) || n < 1 || n > max) return null;
    return n;
}

// _rollSides is the die a "rolled N" line threw.
function _rollSides(line) {
    return /chance:\s*\d+%/i.test(line) ? 100 : 20;
}

// _damageDice reads the dice a damage line names: "You deal 9 slashing damage
// (1d8)." → { sides: 8, label: '1d8' }. Lines without them fall back to a d6.
function _damageDice(line) {
    const m = line.match(/damage\s*\((\d+)d(\d+)([+-]\d+)?\)/i);
    if (!m) return { sides: 6, label: 'dmg' };
    return { sides: parseInt(m[2], 10), label: `${m[1]}d${m[2]}${m[3] ?? ''}` };
}

// _diceRoller works out who threw a roll line and which side they're on:
// 'you', 'ally' (a party member) or 'foe'. The log names the actor at the
// start of the line; lines about your own checks start with "You", or with
// the check ("Death saving throw", "👁 Stealth", "🧭 Survival").
function _diceRoller(line, cs = _lastState) {
    const L = line.trim().replace(/^[⚡📯👁🧭]\s*/u, '');
    if (/^(You\b|Opportunity attack! You|Death saving throw|Stealth:|Survival:)/i.test(L) || /reflex save/i.test(L)) {
        return { name: 'You', side: 'you' };
    }
    const m = L.match(/^(.+?)\s+(?:attacks\b|rolled\b|deals\b|takes an opportunity attack|calls a retreat)/i);
    if (!m) return null;
    const name = m[1].trim();
    const me = (cs?.allies ?? []).find((a) => a.you);
    if (me && me.name === name) return { name: 'You', side: 'you' };
    if ((cs?.allies ?? []).some((a) => a.name === name)) return { name, side: 'ally' };
    if ((cs?.monsters ?? []).some((mo) => mo.name === name)) return { name, side: 'foe' };
    return { name, side: 'foe' };
}

// _playDiceAnimation spawns a die sprite that tumbles, settles on `result`,
// then fades. Scheduled at `atMs` (performance.now scale) so multiple rolls
// don't stack on screen even when triggered from the same response batch.
//
// kind:
//   'd20'    — yellow/blue d20 polygon for attack/initiative/death-save rolls
//   'damage' — red cube for weapon damage; faces during tumble can exceed 20
const DICE_TUMBLE_MS  = 600;
const DICE_SETTLE_MS  = 500;
const DICE_FADE_MS    = 300;

// The die shapes drawn (anything else is drawn as the nearest).
const DIE_SHAPES = [4, 6, 8, 10, 12, 20, 100];

function _playDiceAnimation(result, atMs, kind = 'd20', info = {}) {
    const now = performance.now();
    const startIn = Math.max(0, atMs - now);
    const sides = info.sides ?? (kind === 'damage' ? 6 : 20);
    const shape = DIE_SHAPES.reduce((best, s) => (Math.abs(s - sides) < Math.abs(best - sides) ? s : best), 20);

    setTimeout(() => {
        const zone = $id('combat-dice-zone');
        if (!zone) return;

        // Who rolled (a name tag in their side's colour), the die, and its type.
        const wrap = document.createElement('div');
        wrap.className = 'combat-die-wrap';
        if (info.roller) {
            const who = document.createElement('div');
            who.className = `combat-die-who who-${info.roller.side}`;
            who.textContent = info.roller.name;
            wrap.appendChild(who);
        }
        const die = document.createElement('div');
        die.className = `combat-die d${shape} tumbling`;
        if (kind === 'damage') {
            die.classList.add('damage');
        } else if (sides === 20) {
            if (result === 20) die.classList.add('crit');
            if (result === 1)  die.classList.add('fail');
        }
        wrap.appendChild(die);
        const label = document.createElement('div');
        label.className = 'combat-die-label';
        label.textContent = info.label ?? `d${sides}`;
        wrap.appendChild(label);

        // Tumble faces stay within what that die (or those dice) can show.
        const tumbleMax = kind === 'damage'
            ? Math.max(sides, result)
            : sides;
        die.textContent = String(1 + Math.floor(Math.random() * tumbleMax));
        zone.appendChild(wrap);

        const cycler = setInterval(() => {
            die.textContent = String(1 + Math.floor(Math.random() * tumbleMax));
        }, 70);

        setTimeout(() => {
            clearInterval(cycler);
            die.textContent = String(result);
            die.classList.remove('tumbling');
            die.classList.add('settling');
        }, DICE_TUMBLE_MS);

        setTimeout(() => {
            die.classList.remove('settling');
            wrap.classList.add('fading');
        }, DICE_TUMBLE_MS + DICE_SETTLE_MS);

        setTimeout(() => {
            wrap.remove();
        }, DICE_TUMBLE_MS + DICE_SETTLE_MS + DICE_FADE_MS);
    }, startIn);
}

// _diceDamageNumber extracts the rolled damage from a damage line. Returns
// null when the line isn't a damage announcement.
function _diceDamageNumber(line) {
    const m = line.match(/(?:You deal|deals)\s+(\d+)\s+\w+\s+damage/i);
    if (!m) return null;
    const n = parseInt(m[1], 10);
    if (!Number.isFinite(n) || n < 0) return null;
    return n;
}

// _isOutcomeLine identifies the outcome verdict line emitted right after a
// roll (HIT/MISS/CRIT). Flair fires on the outcome, not on the roll.
function _isOutcomeLine(line) {
    return /^\s*(💥|⚔|✘)/.test(line);
}

// _appendLogEntriesStaggered emits each line on a classified delay schedule.
// When `gridCtx` carries movers, each monster's grid animation is launched at
// the exact moment its own "<Name> moves toward/away" line emits — keeping
// sprite motion in lock-step with the narration, one monster at a time.
function _appendLogEntriesStaggered(logEl, lines, isError = false, fixedInterval = null, gridCtx = null) {
    const filtered = lines.map(r => r.trim()).filter(Boolean);
    if (!filtered.length) return;

    const now = performance.now();
    let t = Math.max(now, _logQueueTailAt);
    let sawMove = false;

    filtered.forEach((line, i) => {
        const atAbsolute = t;
        const moveMatch  = line.match(/^(.+?)\s+moves\s+(toward|away)/i);
        const isMoveLine = !!moveMatch;
        const rollN = _diceRollNumber(line);
        const dmgN  = rollN === null ? _diceDamageNumber(line) : null;

        // Schedule the named monster's grid animation at the move-line's emit
        // time. Doing it here (rather than on render) guarantees the sprite
        // never starts gliding before the player sees the announcement.
        if (moveMatch && gridCtx?.movers) {
            const name = moveMatch[1].trim();
            for (const [id, mv] of Object.entries(gridCtx.movers)) {
                if (!mv.scheduled && mv.name === name) {
                    _scheduleGridSteps(gridCtx.cs, id, mv, atAbsolute);
                    break;
                }
            }
        }

        if (!isError && fixedInterval === null) {
            if (rollN !== null) {
                _playDiceAnimation(rollN, atAbsolute, 'd20', { sides: _rollSides(line), roller: _diceRoller(line, gridCtx?.cs ?? _lastState) });
            } else if (dmgN !== null) {
                const dice = _damageDice(line);
                _playDiceAnimation(dmgN, atAbsolute, 'damage', { ...dice, roller: _diceRoller(line, gridCtx?.cs ?? _lastState) });

                // Drain the matching HP bar mid-die-animation (during the
                // settle phase) so the visual lands as the user reads the
                // damage number. Guard against stale renders.
                const drainAt = atAbsolute + DICE_TUMBLE_MS;
                // Damage the PLAYER dealt (drain the monster bar) vs. damage the
                // monster dealt (drain the player bar). Player damage lines:
                //   weapon      "You deal N damage"
                //   auto spell  "You cast X — it strikes automatically for N damage"
                //   attack spell "💥 X strikes for N damage"
                //   save spell  "✔ X resists — N damage (half)" / "✘ X fails — N damage"
                // A monster hit reads "<Name> deals N damage", which matches none.
                const isPlayerAttack = /^(You |💥|✔|✘)/.test(line);
                const ctxCs = gridCtx?.cs ?? null;
                setTimeout(() => {
                    if (ctxCs && _lastState !== ctxCs) return;
                    if (isPlayerAttack) _renderMonsterHP(ctxCs);
                    else                _renderPlayerHP(ctxCs);
                }, Math.max(0, drainAt - now));
            }

            // XP tick lands as the line emits.
            if (/^\s*\+\d+\s*XP/i.test(line)) {
                const ctxCs = gridCtx?.cs ?? null;
                setTimeout(() => {
                    if (ctxCs && _lastState !== ctxCs) return;
                    _renderXP(ctxCs);
                }, Math.max(0, atAbsolute - now));
            }
        }

        setTimeout(() => {
            _appendSingleEntry(logEl, line, isError);
            _scrollLogToBottom();
            // Flair on outcome lines (HIT!/MISS/CRIT) and non-roll narrative;
            // skip on roll/damage lines — the die sprite is the visual.
            if (!isError && rollN === null && dmgN === null) _spawnFlair([line]);
        }, Math.max(0, atAbsolute - now));

        if (isMoveLine) sawMove = true;

        if (i < filtered.length - 1) {
            t += fixedInterval ?? _classifyDelay(line);
            // Once a movement line has fired, the log is fully gated on the
            // grid animation: nothing emits until the sprite has come to rest.
            if (sawMove && t < _animationEndsAt) t = _animationEndsAt;
        }
    });

    _logQueueTailAt = t;
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

/**
 * Build the board's cells for this fight's size (rebuilt only when the size
 * changes) and fit the cell size to the room the scene gives it. Movement is
 * driven by the D-pad; the only cell interaction is clicking an enemy to target
 * it (delegated to the container).
 */
function _ensureGrid(cs) {
    const container = $id('combat-grid');
    if (!container) return;
    const cols = cs?.grid?.width  || 11;
    const rows = cs?.grid?.height || 8;

    if (cols !== _gridCols || rows !== _gridRows) {
        _gridCols = cols;
        _gridRows = rows;
        container.innerHTML = '';
        for (let r = 0; r < rows; r++) {
            for (let c = 0; c < cols; c++) {
                const cell = document.createElement('div');
                cell.id = `gc-${c}-${r}`;
                cell.className = 'combat-cell';
                cell.dataset.x = c;
                cell.dataset.y = r;
                container.appendChild(cell);
            }
        }
    }
    if (!container.dataset.built) {
        container.dataset.built = '1';
        container.addEventListener('click', (e) => {
            const foe = e.target.closest?.('.combat-cell.foe');
            if (foe?.dataset.foe) { selectCombatTarget(foe.dataset.foe); return; }
            const ally = e.target.closest?.('.combat-cell.ally');
            if (ally?.dataset.ally) { _selectAlly(ally.dataset.ally); return; }
            const reach = e.target.closest?.('.combat-cell.reach');
            if (reach) doMoveToCell(Number(reach.dataset.x), Number(reach.dataset.y));
        });
    }
    _fitGridCells(container);
}

/** Size cells to fill the grid area below the badge row, within the caps. */
function _fitGridCells(container) {
    const area = $id('combat-grid-area');
    if (!area || !_gridCols) return;
    const badges = container.previousElementSibling;
    const availW = area.clientWidth - 8;
    const availH = area.clientHeight - 6 - (badges ? badges.offsetHeight + 3 : 0);
    let cell = Math.floor(Math.min(availW / _gridCols, availH / _gridRows));
    if (!Number.isFinite(cell) || cell <= 0) cell = 22; // not laid out yet
    cell = Math.max(_MIN_CELL_PX, Math.min(_MAX_CELL_PX, cell));
    container.style.gridTemplateColumns = `repeat(${_gridCols}, ${cell}px)`;
    container.style.width = `${cell * _gridCols}px`;
    container.style.setProperty('--cell', `${cell}px`);
}

// ─── Grid tokens ──────────────────────────────────────────────────────────────

// Monster sprites live at /res/img/monsters/{template}.png, but not every
// monster has one yet. Each template is probed once; until (or unless) its
// sprite loads, the cell shows a glyph for the creature.
const _spriteState = {}; // template id → 'loading' | 'ok' | 'missing'

const _MONSTER_GLYPHS = {
    'giant-rat': '🐀', 'swarm-of-rats': '🐀', 'wolf': '🐺', 'werewolf': '🐺',
    'giant-spider': '🕷️', 'blood-hawk': '🦅', 'harpy': '🦅', 'stirge': '🦟',
    'owlbear': '🐻', 'manticore': '🦁', 'merrow': '🧜', 'skeleton': '🦴',
    'banshee': '👻', 'wraith': '👻', 'ghoul': '🧟', 'zombie': '🧟',
    'troll': '👹', 'ogre': '👹', 'lizardfolk': '🦎', 'kobold': '🦎',
};
const _TYPE_GLYPHS = {
    beast: '🐾', humanoid: '👤', undead: '🧟', monstrosity: '👹', fiend: '😈',
    dragon: '🐉', elemental: '🌪️', plant: '🌿', construct: '🗿', aberration: '👁️',
    fey: '🧚', giant: '🗿', ooze: '🫧', celestial: '✨',
};

function _monsterGlyph(m) {
    return _MONSTER_GLYPHS[m.template_id]
        ?? _TYPE_GLYPHS[(m.creature_type || '').toLowerCase()]
        ?? '👹';
}

/** Kick off a one-time sprite probe; repaint when it lands. */
function _probeSprite(templateID) {
    if (!templateID || _spriteState[templateID]) return;
    _spriteState[templateID] = 'loading';
    const img = new Image();
    img.onload  = () => { _spriteState[templateID] = 'ok'; if (_lastState) _paintGrid(_lastState); };
    img.onerror = () => { _spriteState[templateID] = 'missing'; };
    img.src = `/res/img/monsters/${templateID}.png`;
}

function _monsterFace(m) {
    _probeSprite(m.template_id);
    if (_spriteState[m.template_id] === 'ok') {
        return `<img class="combat-token-img" src="/res/img/monsters/${m.template_id}.png" alt="">`;
    }
    return `<span class="combat-token-glyph">${_monsterGlyph(m)}</span>`;
}

/** The player's face: their portrait if they've set one, else their npub crest (party-combat-plan §2c). */
function _playerFace() {
    const ch = window.getGameStateSync?.()?.character ?? {};
    const crest = `<span class="combat-token-crest">${crestSVG(getNpub() ?? '', ch.name ?? '', 18)}</span>`;
    if (!ch.portrait || _portraitFailed === ch.portrait) return crest;
    return `<img class="combat-token-img" src="${portraitSrc(ch.portrait)}" alt=""
                 onerror="window.__pqPortraitFailed && window.__pqPortraitFailed()">`;
}

// A portrait that won't load falls back to the crest (and isn't retried).
let _portraitFailed = '';
window.__pqPortraitFailed = () => {
    _portraitFailed = window.getGameStateSync?.()?.character?.portrait ?? '';
    if (_lastState) _paintGrid(_lastState);
};

// ─── Terrain ─────────────────────────────────────────────────────────────────

/** The terrain type id at (x, y), or '' for open ground. */
function _terrainAt(cs, x, y) {
    const t = cs?.grid?.terrain;
    const w = cs?.grid?.width ?? 0;
    return (t && t[y * w + x]) || '';
}

/** Legend entry (name, glyph, costs) for the terrain at (x, y), or null on open ground. */
function _terrainInfo(cs, x, y) {
    const id = _terrainAt(cs, x, y);
    return id ? (cs.grid?.legend?.[id] ?? null) : null;
}

/** Map "x,y" → movement cost for every cell the player can reach this turn. */
function _reachMap(cs) {
    return new Map((cs.reachable ?? []).map(c => [`${c.x},${c.y}`, c.cost]));
}

/** Tint + glyph + tooltip for one cell's terrain (party-combat-plan §2a, pass 1). */
function _paintTerrainCell(cs, cell, x, y) {
    const id = _terrainAt(cs, x, y);
    if (!id) return;
    const info = cs.grid?.legend?.[id];
    cell.classList.add(`t-${id}`);
    if (info?.glyph) cell.innerHTML = `<span class="combat-terrain-glyph">${info.glyph}</span>`;
    if (!info) return;
    const notes = [];
    if (info.blocks_movement) notes.push('blocks movement');
    else if (info.move_cost > 1) notes.push(`costs ${info.move_cost} to enter`);
    if (info.climb_cost) notes.push(`+${info.climb_cost} to climb`);
    if (info.blocks_sight) notes.push('blocks sight');
    else if (info.cover_ac) notes.push(`+${info.cover_ac} AC cover`);
    if (info.ranged_bonus) notes.push(`+${info.ranged_bonus} to hit with ranged attacks from here`);
    if (info.swim) notes.push('swimming — melee at disadvantage');
    if (info.hazard) notes.push(`${info.hazard} damage`);
    cell.title = notes.length ? `${info.name} — ${notes.join(', ')}` : info.name;
}

/**
 * Redraw the grid: the player, every enemy still in the fight (at its drawn
 * position, which lags the server while a move animates), and the fallen.
 * The current target's cell is outlined.
 */
function _paintGrid(cs) {
    const playerPos = cs.player_pos;
    if (!playerPos) return;

    for (let r = 0; r < _gridRows; r++) {
        for (let c = 0; c < _gridCols; c++) {
            const cell = $id(`gc-${c}-${r}`);
            if (!cell) continue;
            cell.innerHTML = '';
            cell.title = '';
            cell.className = 'combat-cell';
            delete cell.dataset.foe;
            delete cell.dataset.wiki;
            delete cell.dataset.wikiName;
            _paintTerrainCell(cs, cell, c, r);
        }
    }
    const cellAt = p => (p ? $id(`gc-${p.x}-${p.y}`) : null);

    // Where you can move this turn (terrain-costed by the server). Clicking moves you.
    for (const rc of (cs.reachable ?? [])) {
        const cell = cellAt(rc);
        if (!cell) continue;
        cell.classList.add('reach');
        const info = _terrainInfo(cs, rc.x, rc.y);
        cell.title = `Move here — costs ${rc.cost}${info ? ` (${info.name})` : ''}`;
    }

    // The fallen first, so a living combatant standing there paints over them.
    for (const m of (cs.monsters ?? [])) {
        if (m.is_alive || m.fled) continue;
        const cell = cellAt(m.pos);
        if (!cell) continue;
        cell.classList.add('fallen');
        cell.innerHTML = _spriteState[m.template_id] === 'ok'
            ? _monsterFace(m)
            : '<span class="combat-token-glyph">💀</span>';
        cell.title = `${m.name} is down`;
    }
    for (const m of (cs.monsters ?? [])) {
        if (!_isActive(m)) continue;
        const cell = cellAt(_shownPos[m.instance_id] ?? m.pos);
        if (!cell) continue;
        const hp = _hpByID[m.instance_id] ?? { cur: m.current_hp, max: m.max_hp };
        cell.classList.remove('fallen');
        cell.classList.add('foe');
        cell.classList.remove('reach');
        const sight = m.in_sight === false
            ? '<span class="combat-token-badge" title="No line of sight">🚫</span>'
            : m.cover > 0 ? `<span class="combat-token-badge" title="Cover +${m.cover} AC">🛡</span>` : '';
        cell.innerHTML = _monsterFace(m) + sight
            + `<div class="combat-token-hp"><div data-hp-for="${m.instance_id}" style="width:${_hpPct(hp.cur, hp.max)}%"></div></div>`;
        cell.dataset.foe = m.instance_id;
        if (m.template_id) {
            cell.dataset.wiki = `Monsters/${m.template_id}`;
            cell.dataset.wikiName = m.name;
        }
        const sightNote = m.in_sight === false ? ' — no line of sight' : m.cover > 0 ? ` — cover +${m.cover} AC` : '';
        cell.title = `${m.name} (${m.current_hp}/${m.max_hp} HP)${sightNote} — click to target`;
        if (m.instance_id === _targetID) cell.classList.add('target');
    }
    // Party members (P5): their portrait or crest, framed blue; the downed greyed.
    for (const a of (cs.allies ?? [])) {
        if (a.you || a.fled || !a.pos) continue;
        const cell = cellAt(a.pos);
        if (!cell) continue;
        cell.classList.remove('reach');
        cell.classList.add('ally');
        if (a.unconscious) cell.classList.add('fallen');
        if (a.id === _allyTargetID) cell.classList.add('ally-target');
        const face = a.portrait_src
            ? `<img class="combat-token-img" src="${a.portrait_src}" alt="">`
            : `<span class="combat-token-crest">${crestSVG(a.id, a.name ?? '', 18)}</span>`;
        const pct = a.max_hp > 0 ? Math.max(0, Math.min(100, a.current_hp / a.max_hp * 100)) : 0;
        cell.innerHTML = face + (a.unconscious ? '<span class="combat-token-badge">💀</span>' : '') +
            `<div class="combat-token-hp ally-hp"><div style="width:${pct}%"></div></div>`;
        cell.dataset.ally = a.id;
        cell.title = `${a.name} (${a.current_hp}/${a.max_hp} HP)${a.unconscious ? ' — down! Help or heal them' : ''} — click to pick for heals / Help`;
    }

    const pCell = cellAt(playerPos);
    if (pCell) {
        pCell.classList.remove('fallen');
        pCell.classList.add('you');
        pCell.innerHTML = _playerFace();
        pCell.title = 'You';
    }
}

// ─── Party fights (P5) ────────────────────────────────────────────────────────

/** Pick (or un-pick) an ally as the target for heals, potions and Help. */
function _selectAlly(id) {
    _allyTargetID = _allyTargetID === id ? null : id;
    if (_lastState) {
        _paintGrid(_lastState);
        _renderCombatButtons(_lastState);
        const a = (_lastState.allies ?? []).find(x => x.id === _allyTargetID);
        if (a) _logInfo(`Healing and Help will go to ${a.name}. Click them again to clear.`);
    }
}

/** Keep a party fight in step: poll for others' turns, tick the turn timer. */
function _startPartyTimers(cs) {
    const active = cs.phase === 'active';
    if (active && !_partyPollTimer) _partyPollTimer = setInterval(_pollPartyFight, 1200);
    if (!active && _partyPollTimer) { clearInterval(_partyPollTimer); _partyPollTimer = null; }
    if (!_countdownTimer) _countdownTimer = setInterval(_tickCountdown, 500);
    _tickCountdown();
}

async function _pollPartyFight() {
    if (_busy > 0 || !_lastState) return;
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const q = new URLSearchParams({ npub, save_id: saveID, since: String(_logIndex ?? 0) });
        const resp = await fetch(`/api/combat/state?${q}`);
        if (!resp.ok) return;
        const cs = await resp.json();
        if (!cs?.phase || _busy > 0) return;
        const last = _lastState;
        const changed = cs.log_index !== _logIndex || cs.phase !== last?.phase ||
            cs.turn_name !== last?.turn_name || cs.your_turn !== last?.your_turn;
        if (changed) renderCombatState(cs);
    } catch (_) { /* try again next tick */ }
}

function _tickCountdown() {
    const cs = _lastState;
    const chip = document.querySelector('#combat-turn-strip .combat-chip.turn');
    const banner = $id('combat-turn-banner');
    if (!cs || !cs.party_fight || cs.phase !== 'active' || !cs.turn_deadline) {
        if (banner) banner.style.display = 'none';
        return;
    }
    const left = Math.max(0, Math.ceil((cs.turn_deadline - Date.now()) / 1000));
    if (chip) chip.textContent = chip.textContent.replace(/⏱.*$/, `⏱ ${left}`);
    if (banner) {
        banner.style.display = 'block';
        banner.textContent = cs.your_turn ? `Your turn — ${left}s` : `Waiting for ${cs.turn_name ?? '…'} — ${left}s`;
        banner.style.color = cs.your_turn ? (left <= 10 ? '#fbbf24' : '#4ade80') : '#9ca3af';
    }
}

/** POST a party-fight action and render the result. */
async function _partyFightPost(endpoint, body = {}) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    try {
        const resp = await combatPost(endpoint, { npub, save_id: saveID, ...body });
        const cs = await resp.json();
        if (!resp.ok || !cs.success) {
            _logError(cs.error ?? `HTTP ${resp.status}`);
            if (_lastState) _renderCombatButtons(_lastState);
            return;
        }
        renderCombatState(cs);
    } catch (err) {
        logger.error(`${endpoint} error:`, err);
        _logError('Network error — try again.');
    }
}

export function doHelpAlly(id) { return _partyFightPost('/api/combat/help', { target_id: id }); }
export function doRetreat() { return _partyFightPost('/api/combat/retreat'); }

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

// _GRID_STEP_MS — wall time between monster grid steps.
const _GRID_STEP_MS = 450;

// _scheduleGridSteps animates one monster's sprite a Chebyshev cell at a time
// starting at `startAt` (performance.now scale). Steps fire on a fixed cadence.
// Extends `_animationEndsAt` so the log stager holds subsequent entries until
// the monster has come to rest.
function _scheduleGridSteps(cs, id, mover, startAt) {
    mover.scheduled = true;
    const path = mover.path;
    if (!path || !path.length) return;
    _animationEndsAt = Math.max(_animationEndsAt, startAt + path.length * _GRID_STEP_MS);
    path.forEach((p, i) => {
        const fireAt = startAt + (i + 1) * _GRID_STEP_MS;
        setTimeout(() => {
            if (_lastState !== cs) return;
            _shownPos[id] = p;
            _paintGrid(cs);
        }, Math.max(0, fireAt - performance.now()));
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
    { re: /\+(\d+) XP/i,     text: null, color: '#22d3ee', size: '11px', xp: true, glow: true },
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
            const text = cfg.text ?? (m[1] ? (cfg.xp ? `+${m[1]} XP` : `-${m[1]}`) : null);
            if (!text) continue;
            const xPct = 20 + Math.random() * 60;
            const yPct = 30 + Math.random() * 35;
            const el = document.createElement('span');
            el.className = 'combat-flair';
            let css = `left:${xPct}%;top:${yPct}%;font-size:${cfg.size};color:${cfg.color};`;
            if (cfg.glow) css += `text-shadow:1px 1px 2px #000,0 0 8px ${cfg.color},0 0 14px ${cfg.color};`;
            el.style.cssText = css;
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

    if (cs.phase === 'active') {
        _deathSaveBusy = false; // back on your feet
        return;
    }

    // Hold the terminal/death-save UI until the killing-blow dice + log have
    // finished playing out. _logQueueTailAt is the absolute time of the last
    // scheduled emit; show the panel right after it.
    const showAt = Math.max(performance.now(), _logQueueTailAt);
    const delay  = Math.max(0, showAt - performance.now());

    setTimeout(() => {
        // Bail if a newer render has superseded this one or combat exited.
        if (_lastState !== cs) return;
        _deathSaveBusy = false; // the roll has played out

        switch (cs.phase) {
        case 'death_saves':
            // Death-save UI hijacks the action bar, not the in-scene panel.
            _renderDeathSaveBar(cs);
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
    }, delay);
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

// _renderDeathSaveBar hijacks the three action-bar columns during death_saves
// phase: pip grid, flavor + counts, and a big roll button.
function _renderDeathSaveBar(cs) {
    _claimActionBar('combat');
    const navEl = $id('navigation-buttons');
    const bldEl = $id('building-buttons');
    const npcEl = $id('npc-buttons');
    if (!navEl || !bldEl || !npcEl) return;

    const player = cs.player ?? {};
    const s = player.death_save_successes ?? 0;
    const f = player.death_save_failures  ?? 0;

    const pipGrid = (filled, max, fillColor) => {
        let out = '';
        for (let i = 1; i <= max; i++) {
            const bg = i <= filled ? fillColor : '#111827';
            out += `<span style="width:14px;height:14px;border:1px solid #374151;background:${bg};display:inline-block;"></span>`;
        }
        return out;
    };

    navEl.innerHTML = `
        <h3 style="color:#86efac;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Successes</h3>
        <div style="display:flex;gap:3px;margin-bottom:6px;">${pipGrid(s, 3, '#166534')}</div>
        <h3 style="color:#fca5a5;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Failures</h3>
        <div style="display:flex;gap:3px;">${pipGrid(f, 3, '#7f1d1d')}</div>
    `;

    bldEl.innerHTML = `
        <h3 style="color:#fca5a5;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Dying</h3>
        <p style="font-size:9px;color:#d1d5db;line-height:1.3;margin:0 0 4px;">
            You lie bleeding. Three successes and you get back up with 1 HP; three failures and you're gone.
        </p>
        <p style="font-size:8px;color:#9ca3af;margin:0;">
            ${s}/3 successes, ${f}/3 failures
        </p>
    `;

    const rollBtnStyle = `
        width:100%;padding:8px 4px;font-weight:bold;color:#fff;font-size:11px;
        cursor:pointer;background:#7f1d1d;
        border-top:2px solid #dc2626;border-left:2px solid #dc2626;
        border-right:2px solid #450a0a;border-bottom:2px solid #450a0a;`;
    npcEl.innerHTML = `
        <h3 style="color:#9ca3af;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Action</h3>
        <button id="death-save-roll-btn" style="${rollBtnStyle}" onclick="window.rollDeathSave()" title="Roll a d20 — 10+ is a success, nat 20 revives, nat 1 = two failures">
            🎲 Roll Death Save
        </button>
    `;
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
    // Several kills can drop the same item — show one line per item.
    const merged = new Map();
    for (const d of (cs.loot_rolled ?? [])) merged.set(d.item, (merged.get(d.item) ?? 0) + (d.quantity ?? 0));
    const loot = [...merged].map(([item, quantity]) => ({ item, quantity }));
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

// ─── Action-bar ownership ─────────────────────────────────────────────────────
// The three button columns (navigation/building/npc) are touched by combat,
// location display, and NPC dialog. To avoid stale buttons after phase
// transitions, all combat-side mutation goes through claim/release.

let _actionBarOwner = null;

function _claimActionBar(ownerKey) {
    if (_actionBarOwner === ownerKey) return; // already ours
    if (_actionBarOwner !== null && _actionBarOwner !== ownerKey) {
        // Re-claim within combat — don't re-cache (would capture our own markup).
        _actionBarOwner = ownerKey;
        return;
    }
    const navEl = $id('navigation-buttons');
    const bldEl = $id('building-buttons');
    const npcEl = $id('npc-buttons');
    if (navEl) _cachedNav = navEl.innerHTML;
    if (bldEl) _cachedBld = bldEl.innerHTML;
    if (npcEl) _cachedNpc = npcEl.innerHTML;
    _actionBarOwner = ownerKey;
}

function _releaseActionBar() {
    if (_actionBarOwner === null) return;
    if (_cachedNav !== null) { const e = $id('navigation-buttons'); if (e) e.innerHTML = _cachedNav; _cachedNav = null; }
    if (_cachedBld !== null) { const e = $id('building-buttons');   if (e) e.innerHTML = _cachedBld; _cachedBld = null; }
    if (_cachedNpc !== null) { const e = $id('npc-buttons');        if (e) e.innerHTML = _cachedNpc; _cachedNpc = null; }
    _actionBarOwner = null;
}

function _replaceActionButtons(cs) {
    _claimActionBar('combat');
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

/**
 * The martial-class ability launcher button. Hidden for casters (no resource
 * pool — they use Cast Spell). Stays live even after the main action is spent so
 * bonus-action abilities (Rage, Flurry) remain reachable; the menu and the server
 * gate each ability by cost / cooldown / action economy.
 */
function _abilityButton(cs) {
    const pool = cs?.player?.resource;
    if (!pool) return '';
    return `<button style="${_B('color:#fde047;')}" onclick="window.openCombatAbilityMenu()"
                    title="Use a class ability">⚡ ${pool.label} ${pool.current}/${pool.max}</button>`;
}

function _renderCombatButtons(cs) {
    const navEl = $id('navigation-buttons');
    const bldEl = $id('building-buttons');
    const npcEl = $id('npc-buttons');

    const phase = cs.phase ?? 'active';
    // Death saves get their own bar renderer; non-active phases skip altogether.
    if (phase !== 'active') return;

    // Party fight, someone else's turn: show who we're waiting on.
    const waiting = cs.party_fight && !cs.your_turn;
    const actionBar = $id('action-buttons');
    if (actionBar) {
        actionBar.style.opacity = waiting ? '0.45' : '';
        actionBar.style.pointerEvents = waiting ? 'none' : '';
        actionBar.title = waiting ? `Waiting for ${cs.turn_name ?? 'the enemy'}…` : '';
    }

    const actionUsed   = cs.action_used ?? false;
    const bonusUsed    = cs.bonus_action_used ?? false;
    const range        = cs.range ?? 0;           // to the nearest enemy (flee gate)
    const target       = _targetMonster(cs);
    const targetRange  = _isActive(target) ? (target.range ?? range) : range;
    const targetName   = target?.name ?? 'the enemy';
    const foes         = (cs.monsters ?? []).filter(_isActive);
    const bonusAvail   = cs.bonus_attack_available ?? false;
    const ammo         = cs.ammo_remaining ?? 0;

    const budget    = cs.movement_budget ?? 0;
    const spent     = cs.movement_spent  ?? 0;
    const remaining = Math.max(0, budget - spent);

    const pPos = cs.player_pos ?? { x: -1, y: -1 };
    const cheb = (a, b) => Math.max(Math.abs(a.x - b.x), Math.abs(a.y - b.y));
    const reachCost = _reachMap(cs);
    const disengaged  = cs.disengaged ?? false;
    const reactionUsed = cs.reaction_used ?? false;

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
        const to = { x: tx, y: ty };
        const oob        = tx < 0 || tx >= _gridCols || ty < 0 || ty >= _gridRows;
        const intoMon    = foes.some(m => m.pos && m.pos.x === tx && m.pos.y === ty);
        const noBudget   = remaining <= 0;
        const cost       = reachCost.get(`${tx},${ty}`);
        const blockedBy  = !oob && !intoMon && !noBudget && cost === undefined ? _terrainInfo(cs, tx, ty) : null;
        const disabled   = oob || intoMon || noBudget || cost === undefined;

        // OA warning: moving this direction would leave some enemy's reach
        const provokesOA = !disabled && !disengaged && foes.some(m => {
            const reach = m.melee_reach ?? 0;
            return m.pos && reach > 0 && cheb(pPos, m.pos) <= reach && cheb(to, m.pos) > reach;
        });
        const label = provokesOA ? `⚠${d.label}` : d.label;
        const costNote = cost > 1 ? ` — ${_terrainInfo(cs, tx, ty)?.name ?? 'rough ground'}, costs ${cost}` : '';
        const title = disabled
            ? (noBudget ? 'No movement left' : intoMon ? 'Blocked by enemy' : oob ? 'Out of bounds'
                : blockedBy?.blocks_movement ? `Blocked — ${blockedBy.name}` : 'Not enough movement')
            : provokesOA
                ? `Move ${d.label}${costNote} — will provoke opportunity attack! Disengage first.`
                : `Move ${d.label}${costNote}`;
        return disabled
            ? `<button style="${_DPAD_BTN_DISABLED}" disabled title="${title}">${d.label}</button>`
            : `<button style="${_DPAD_BTN}${provokesOA ? 'color:#fbbf24;' : ''}" onclick="window.doStep(${d.dx},${d.dy})" title="${title}">${label}</button>`;
    }).join('');

    if (navEl) navEl.innerHTML = `
        <h3 style="color:#fbbf24;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Move</h3>
        <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:2px;">
            ${padCells}
        </div>`;

    // ── Building column: Attack main / off, Spell / Item / Ability stubs ─────
    // A non-weapon in hand (spellbook, torch, focus) isn't a legal attack source —
    // you strike Unarmed, matching the server. No more spellbook-attack no-op.
    const rawMainID = _equippedWeaponID('mainHand');
    const mainID    = _isWeaponItem(rawMainID) ? rawMainID : null;
    const isRanged  = mainID ? _isRangedWeapon(mainID) : false;
    const maxMelee  = mainID ? _meleeReach(mainID) : 1;
    const mainName  = mainID ? (_equippedWeaponName('mainHand') || 'Unarmed') : 'Unarmed';
    const meleeBlocked  = !isRanged && targetRange > maxMelee;
    const rangedBlocked = isRanged && ammo <= 0;
    const noSight       = _isActive(target) && target.in_sight === false;
    const coverNote     = _isActive(target) && target.cover > 0
        ? ` — ${targetName} has ${target.cover >= 5 ? 'three-quarters' : 'half'} cover (+${target.cover} AC)` : '';

    let mainLabel = `⚔ ${mainName}`;
    if (isRanged && ammo > 0) mainLabel += ` (${ammo})`;

    let attackBtn;
    if (actionUsed)         attackBtn = _B_GRAYED(mainLabel, 'Action used this turn');
    else if (meleeBlocked)  attackBtn = _B_GRAYED(mainLabel, `${targetName} is out of melee range — step closer or pick another target`);
    else if (rangedBlocked) attackBtn = _B_GRAYED(mainLabel, 'No ammo');
    else if (noSight)       attackBtn = _B_GRAYED(mainLabel, `No line of sight to ${targetName} — move to get a clear shot`);
    else attackBtn = `<button style="${_B()}" onclick="window.doAttack('main',false)" title="Attack ${targetName}${coverNote}">${coverNote ? '🛡' : ''}${mainLabel}</button>`;

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
            ${actionUsed
                ? _B_GRAYED('✨ Cast Spell', 'Action used this turn')
                : `<button style="${_B('color:#c4b5fd;')}" onclick="window.openCombatSpellMenu()"
                    title="Cast a prepared spell">✨ Cast Spell</button>`}
            ${actionUsed
                ? _B_GRAYED('🧪 Use Item', 'Action used this turn')
                : `<button style="${_B('color:#fca5a5;')}" onclick="window.openCombatItemMenu()"
                    title="Use a consumable (potion, food)">🧪 Use Item</button>`}
            ${_abilityButton(cs)}
        </div>`;

    // ── NPC column: Disengage / Hold / Flee / End Turn ───────────────────────
    const inMonsterReach = foes.some(m => (m.melee_reach ?? 0) > 0 && (m.range ?? 99) <= m.melee_reach);
    let disengageBtn;
    if (disengaged) {
        disengageBtn = _B_GRAYED('🕊 Disengaged', 'Already disengaged — movement safe');
    } else if (actionUsed) {
        disengageBtn = _B_GRAYED('🕊 Disengage', 'Action already used');
    } else if (!inMonsterReach) {
        disengageBtn = _B_GRAYED('🕊 Disengage', 'Not in enemy reach');
    } else {
        disengageBtn = `<button style="${_B('color:#a3e635;')}" onclick="window.doDisengage()"
                    title="Spend your action to move without provoking opportunity attacks">🕊 Disengage</button>`;
    }

    const holdBtn = actionUsed
        ? _B_GRAYED('🛡 Hold Position', 'Action already used')
        : remaining <= 0
            ? _B_GRAYED('🛡 Hold Position', 'Need movement left to brace')
            : `<button style="${_B('color:#60a5fa;')}" onclick="window.doHoldPosition()"
                    title="Spend remaining movement to ready a counter-strike">🛡 Hold Position</button>`;

    // Party fights: help an adjacent ally; the leader can call a retreat.
    const me = (cs.allies ?? []).find(a => a.you);
    const adjacent = (cs.allies ?? []).filter(a => !a.you && !a.fled && a.pos && me?.pos && cheb(a.pos, me.pos) <= 1);
    const helpTarget = adjacent.find(a => a.id === _allyTargetID) ?? adjacent.find(a => a.unconscious) ?? adjacent[0];
    let helpBtn = '';
    if (cs.party_fight && helpTarget) {
        const verb = helpTarget.unconscious ? `🤝 Revive ${helpTarget.name}` : `🤝 Help ${helpTarget.name}`;
        helpBtn = actionUsed ? _B_GRAYED(verb, 'Action already used')
            : `<button style="${_B('color:#86efac;')}" onclick="window.doHelpAlly('${helpTarget.id}')"
                title="${helpTarget.unconscious ? 'Get them back on their feet (1 HP)' : 'Their next attack has advantage'}">${verb}</button>`;
    }
    const leads = !!window.socialState?.()?.party?.you_lead;
    const retreatBtn = cs.party_fight && leads
        ? (actionUsed ? _B_GRAYED('📯 Retreat', 'Action already used')
            : `<button style="${_B('color:#fbbf24;')}" onclick="window.doRetreat()"
                title="Call a retreat: one roll for the whole party, at the slowest member's pace">📯 Retreat</button>`)
        : '';

    const fleeBtn = actionUsed
        ? _B_GRAYED('🏃 Flee', 'Action already used')
        : range < 3
            ? _B_GRAYED('🏃 Flee (need range ≥ 3)', 'Get every enemy to range 3 or more first')
            : `<button style="${_B('color:#fbbf24;')}" onclick="window.doFlee()"
                    title="Attempt to escape — success chance based on range and speed">🏃 Flee</button>`;

    if (npcEl) npcEl.innerHTML = `
        <h3 style="color:#9ca3af;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Turn</h3>
        <div style="display:flex;flex-direction:column;gap:2px;">
            ${helpBtn}
            ${disengageBtn}
            ${holdBtn}
            ${retreatBtn}
            ${fleeBtn}
            <button style="${_B('color:#f87171;')}" onclick="window.doEndTurn()"
                    title="End your turn — every enemy acts in initiative order">⏭ End Turn</button>
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

// _isWeaponItem reports whether an equipped item is a real weapon (attack source).
function _isWeaponItem(itemID) {
    if (!itemID) return false;
    try {
        const item = window.getItemById?.(itemID);
        if (!item) return false;
        if (String(item.type || '').toLowerCase().includes('weapon')) return true;
        return (item.tags || []).some(t => String(t).toLowerCase() === 'weapon');
    } catch (_) { return false; }
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

// Glyphs for the combat condition badges.
const _CONDITION_ICON = {
    restrained: '🕸', blinded: '🌫', poisoned: '☠', frightened: '😱', prone: '⬇',
    stunned: '💫', paralyzed: '❄', grappled: '✊', outlined: '✨', charmed: '💗',
    distracted: '🌀', exhausted: '😩', laughing: '😂', baned: '🜏', dueled: '⚔',
    burning: '🔥', moonlit: '🌙', beset: '👻',
};

// _renderConditionsInto paints active-condition badges into an element.
// Purple for the monster's conditions (things you inflicted), red for the
// player's (things afflicting you).
function _renderConditionsInto(elId, conditions, color) {
    const el = $id(elId);
    if (!el) return;
    el.innerHTML = '';
    (conditions || []).forEach(name => {
        const badge = document.createElement('span');
        const icon = _CONDITION_ICON[String(name).toLowerCase()] || '●';
        badge.textContent = `${icon} ${name}`;
        badge.title = name;
        badge.style.cssText =
            `font-size:7px;font-weight:bold;color:${color};text-transform:capitalize;` +
            'border:1px solid #4a4a4a;background:rgba(0,0,0,0.85);padding:0 3px;';
        el.appendChild(badge);
    });
}

// _renderBuffsInto appends badges for the player's spell effects after their
// conditions: temporary HP, each buff (hover for what it does and how long it
// lasts), and the spell they're concentrating on.
function _renderBuffsInto(elId, player) {
    const el = $id(elId);
    if (!el || !player) return;
    const badge = (text, tip, color) => {
        const b = document.createElement('span');
        b.textContent = text;
        b.title = tip;
        b.style.cssText =
            `font-size:7px;font-weight:bold;color:${color};` +
            'border:1px solid #4a4a4a;background:rgba(0,0,0,0.85);padding:0 3px;';
        el.appendChild(b);
    };
    if (player.temp_hp > 0) badge(`🛡 ${player.temp_hp} temp HP`, 'Temporary hit points: damage comes off these first.', '#67e8f9');
    for (const b of (player.buffs ?? [])) {
        const left = b.rounds > 0 ? ` (${b.rounds})` : '';
        badge(`✨ ${b.name}${left}`, b.summary || b.name, '#86efac');
    }
    if (player.concentration) badge(`◎ ${player.concentration}`, `Concentrating on ${player.concentration}: taking damage forces a Constitution save to keep it.`, '#fde68a');
}

// _renderDifficultyBadge shows a "tough"/"deadly" tag by the monster name when a
// fight outclasses the player's level (M5 §22). Hidden for trivial…fair fights.
const _DIFFICULTY_STYLE = {
    tough:  { label: '⚠ Tough',  color: '#fbbf24', border: '#78350f', bg: 'rgba(40,25,0,0.85)' },
    deadly: { label: '☠ Deadly', color: '#fca5a5', border: '#7f1d1d', bg: 'rgba(40,0,0,0.85)' },
};
function _renderDifficultyBadge(difficulty) {
    const el = $id('combat-monster-difficulty');
    if (!el) return;
    const s = _DIFFICULTY_STYLE[String(difficulty || '').toLowerCase()];
    if (!s) { el.style.display = 'none'; return; }
    el.textContent = s.label;
    el.title = `This fight is rated ${difficulty} for your level.`;
    el.style.display = '';
    el.style.color = s.color;
    el.style.borderColor = s.border;
    el.style.background = s.bg;
}

// _renderMonsterHP drains every monster's HP to the cs values: the targeted
// enemy's big bar and each turn-strip chip.
function _renderMonsterHP(cs) {
    if (!cs) return;
    for (const m of (cs.monsters ?? [])) _hpByID[m.instance_id] = { cur: m.current_hp, max: m.max_hp };
    _paintHPViews(cs);
}

function _hpPct(cur, max) {
    return max > 0 ? Math.max(0, Math.min(100, (cur / max) * 100)) : 0;
}

// _paintHPViews draws the bars from the last drained HP (_hpByID), so retargeting
// mid-animation doesn't leak damage the log hasn't announced yet.
function _paintHPViews(cs) {
    const target = _targetMonster(cs);
    if (target) {
        const hp = _hpByID[target.instance_id] ?? { cur: target.current_hp, max: target.max_hp };
        const bar = $id('combat-monster-hp-bar');
        if (bar) bar.style.width = `${_hpPct(hp.cur, hp.max)}%`;
        _setText('combat-monster-hp-text', `${hp.cur} / ${hp.max} HP`);
    }
    for (const el of document.querySelectorAll('#combat-turn-strip [data-hp-for], #combat-grid [data-hp-for]')) {
        const hp = _hpByID[el.dataset.hpFor];
        if (hp) el.style.width = `${_hpPct(hp.cur, hp.max)}%`;
    }
}

// _renderPlayerHP writes player HP (and mana) from combat state into the side
// panel. Mana matters because the clock is frozen in combat, so this is the only
// path that repaints the mana bar as spells spend it.
function _renderPlayerHP(cs) {
    const state = window.getGameStateSync?.();
    if (!state?.character) return;
    if (cs.player) {
        state.character.hp     = cs.player.current_hp;
        state.character.max_hp = cs.player.max_hp;
        if (cs.player.mana     !== undefined) state.character.mana     = cs.player.mana;
        if (cs.player.max_mana !== undefined) state.character.max_mana = cs.player.max_mana;
    }
    window.updateCharacterDisplay?.();
}

// _renderXP writes the cumulative XP so the side panel ticks up alongside
// the +N XP log line.
function _renderXP(cs) {
    const state = window.getGameStateSync?.();
    if (!state?.character) return;
    const xpEarned = cs.xp_earned ?? 0;
    state.character.experience = _baseExperience + xpEarned;
    window.updateCharacterDisplay?.();
}

// _syncSidePanelFromCombat is used when there are no damage/XP lines to drive
// timing — falls back to a synchronous full update.
function _syncSidePanelFromCombat(cs) {
    _renderPlayerHP(cs);
    _renderXP(cs);
}

function _restoreActionButtons() {
    _releaseActionBar();
    window.displayCurrentLocation?.().catch(() => {});
}
