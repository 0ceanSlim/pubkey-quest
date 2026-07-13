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
import { restoreActionButtonsLayout, displayCurrentLocation } from '../ui/locationDisplay.js';

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
    // Freeze in-game time while fighting — combat is turn-based; the world tick
    // resumes on exit. (Combat actions use their own endpoints, not the tick.)
    smoothClock.pause();
    _baseExperience = window.getGameStateSync?.()?.character?.experience ?? 0;
    // Clear any stale pacing timers from a prior combat that ended mid-flush.
    _logQueueTailAt = 0;
    _animationEndsAt = 0;
    _replaceGameText();
    // If we entered from a travel encounter, the action bar is the travel flex
    // layout (no navigation/building/npc sub-elements). Rebuild the normal grid
    // first so the combat buttons have somewhere to render. No-op outside travel.
    restoreActionButtonsLayout();
    _replaceActionButtons(cs);
    _show('combat-overlay');
    renderCombatState(cs);
}

export function exitCombatMode() {
    logger.info('🏳️  Exiting combat mode');
    smoothClock.unpause(); // resume the world tick now the fight is over
    _hide('combat-overlay');
    _restoreGameText();
    _restoreActionButtons();
    _hide('death-saves-panel');
    _hide('loot-panel');
    _hide('defeat-panel');
    _lastState = null;
    _logQueueTailAt = 0;
    _animationEndsAt = 0;
    // Re-render the scene we returned to so the correct action bar comes back —
    // in particular the travel controls when a fight ended mid-journey (the
    // cached bar restore above only rebuilds the generic grid).
    Promise.resolve(displayCurrentLocation?.()).catch((e) => logger.error('post-combat location refresh failed:', e));
}

/** Single re-render function — reads cs and updates every DOM region. */
export function renderCombatState(cs) {
    if (!cs) return;
    // On combat start with monster-first initiative the backend sends
    // `monster_pos_before` (the spawn cell, before the opening turn move).
    // Use it as the animation seed so the sprite enters from where it
    // appeared instead of teleporting to its post-move position.
    const prevMonsterPos = _lastState?.monster_pos
        ?? cs.monster_pos_before
        ?? null;
    _lastState = cs;

    const monster = cs.monsters?.[0];

    // ── Monster panel (HP drain deferred — see below) ───────────────────────
    if (monster) {
        _setText('combat-monster-name', monster.name ?? 'Unknown');
        _setText('combat-monster-ac-badge', `AC ${monster.armor_class ?? '?'}`);
        _renderConditionsInto('combat-monster-conditions', monster.conditions || [], '#c4b5fd');
        _renderDifficultyBadge(cs.difficulty);
    }

    // Player conditions (things afflicting you) render over the scene, in red.
    _renderConditionsInto('combat-conditions', cs.player?.conditions || [], '#fca5a5');
    if (monster) {

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

    // ── Grid baseline + movement context ────────────────────────────────────
    _ensureGrid();
    const newMonsterPos = cs.monster_pos;
    const movedThisFrame = prevMonsterPos && newMonsterPos &&
        (prevMonsterPos.x !== newMonsterPos.x || prevMonsterPos.y !== newMonsterPos.y);
    const movePath = movedThisFrame ? _chebyshevPath(prevMonsterPos, newMonsterPos) : [];
    if (!movedThisFrame) {
        // No monster movement — paint statically (player may have stepped).
        _updateGridHighlights(cs);
    }

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
            _appendLogEntriesStaggered(logEl, entries, false, fixed, {
                cs, prevMonsterPos, movePath,
            });
        } else if (movedThisFrame) {
            // No new log entries but monster moved (rare) — still animate.
            _scheduleGridSteps(cs, prevMonsterPos, movePath, performance.now());
        }
    } else if (movedThisFrame) {
        _scheduleGridSteps(cs, prevMonsterPos, movePath, performance.now());
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

/** Cast a prepared spell at the monster. Mirrors doAttack. */
export async function doCastSpell(spellId) {
    const npub = getNpub(), saveID = getSaveID();
    if (!npub || !saveID) return;
    _closeCombatChooser();
    try {
        const resp = await combatPost('/api/combat/cast', { npub, save_id: saveID, spell_id: spellId });
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
        const resp = await combatPost('/api/combat/use-item', { npub, save_id: saveID, item_id: itemId });
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

            const usable = manaOK && compsOK;
            let meta = `${_spellShapeIcon(spell)} ${cost}◆`;
            if (compParts.length) meta += ` 🜂${compParts.join(' ')}`;

            const tip = [
                spell.description ?? '',
                `Mana: ${mana}/${cost}${manaOK ? '' : ' — short'}`,
                ...compTips,
                !usable ? '⚠ Can’t cast: ' + (!manaOK ? 'not enough mana' : 'missing components') : '',
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

/** Open the combat item menu: reachable consumables (loose + in general-slot pouches). */
export function openCombatItemMenu() {
    const inv = window.getGameStateSync?.()?.character?.inventory ?? {};
    const gen = inv.general_slots ?? [];
    const counts = new Map();

    const tally = (slot) => {
        const id = slot?.item;
        if (!id) return;
        const item = window.getItemById?.(id);
        const tags = (item?.tags ?? []).map(t => String(t).toLowerCase());
        if (!tags.includes('consumable')) return;
        counts.set(id, (counts.get(id) ?? 0) + (slot.quantity ?? 0));
    };
    for (const slot of gen) {
        if (!slot) continue;
        tally(slot);
        for (const c of (slot.contents ?? [])) tally(c);
    }

    const entries = [];
    for (const [id, qty] of counts) {
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

    if (entries.length === 0) {
        _logInfo('No usable items within reach.');
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
        if (cs.phase) {
            logger.info('⚔️  Resuming active combat after page load');
            enterCombatMode(cs);
        }
    } catch (_) { /* no active combat */ }
}
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
}

function _restoreGameText() {
    if (_cachedGameText === null) return;
    const gameText = $id('game-text');
    if (gameText) gameText.innerHTML = _cachedGameText;
    _cachedGameText = null;
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

// _diceRollNumber extracts the d20 face from a roll line. Returns null when
// the line isn't an actionable single-die roll. Initiative is now emitted as
// two separate per-side rolls so each one fires its own animation.
function _diceRollNumber(line) {
    const m = line.match(/rolled\s+(\d+)/i);
    if (!m) return null;
    const n = parseInt(m[1], 10);
    if (!Number.isFinite(n) || n < 1 || n > 20) return null;
    return n;
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

function _playDiceAnimation(result, atMs, kind = 'd20') {
    const now = performance.now();
    const startIn = Math.max(0, atMs - now);

    setTimeout(() => {
        const zone = $id('combat-dice-zone');
        if (!zone) return;

        const die = document.createElement('div');
        die.className = 'combat-die tumbling';
        if (kind === 'damage') {
            die.classList.add('damage');
        } else {
            if (result === 20) die.classList.add('crit');
            if (result === 1)  die.classList.add('fail');
        }
        // Initial tumble face — random within a believable range.
        const tumbleMax = kind === 'damage'
            ? Math.max(20, result * 2)  // ensure tumble values look plausible up to result
            : 20;
        die.textContent = String(1 + Math.floor(Math.random() * tumbleMax));
        zone.appendChild(die);

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
            die.classList.add('fading');
        }, DICE_TUMBLE_MS + DICE_SETTLE_MS);

        setTimeout(() => {
            die.remove();
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
// When `gridCtx` is provided and contains a non-empty movePath, the monster's
// grid animation is launched at the exact moment the matching "moves toward/
// away" log line emits — keeping sprite motion in lock-step with the narration.
function _appendLogEntriesStaggered(logEl, lines, isError = false, fixedInterval = null, gridCtx = null) {
    const filtered = lines.map(r => r.trim()).filter(Boolean);
    if (!filtered.length) return;

    const now = performance.now();
    let t = Math.max(now, _logQueueTailAt);
    let sawMove = false;
    let scheduledGrid = false;

    filtered.forEach((line, i) => {
        const atAbsolute = t;
        const isMoveLine = /moves\s+(toward|away)/i.test(line);
        const rollN = _diceRollNumber(line);
        const dmgN  = rollN === null ? _diceDamageNumber(line) : null;

        // Schedule the monster grid animation at the move-line's emit time.
        // Doing it here (rather than on render) guarantees the sprite never
        // starts gliding before the player sees the announcement.
        if (isMoveLine && !scheduledGrid && gridCtx && gridCtx.movePath && gridCtx.movePath.length) {
            _scheduleGridSteps(gridCtx.cs, gridCtx.prevMonsterPos, gridCtx.movePath, atAbsolute);
            scheduledGrid = true;
        }

        if (!isError && fixedInterval === null) {
            if (rollN !== null) {
                _playDiceAnimation(rollN, atAbsolute, 'd20');
            } else if (dmgN !== null) {
                _playDiceAnimation(dmgN, atAbsolute, 'damage');

                // Drain the matching HP bar mid-die-animation (during the
                // settle phase) so the visual lands as the user reads the
                // damage number. Guard against stale renders.
                const drainAt = atAbsolute + DICE_TUMBLE_MS;
                const isPlayerAttack = /^\s*You deal/i.test(line);
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

// _GRID_STEP_MS — wall time between monster grid steps.
const _GRID_STEP_MS = 450;

// _scheduleGridSteps animates the monster sprite one Chebyshev cell at a time
// starting at `startAt` (performance.now scale). The previous position is
// painted immediately; subsequent steps fire on a fixed cadence. Updates
// `_animationEndsAt` so the log stager can hold subsequent entries until the
// monster has come to rest.
function _scheduleGridSteps(cs, prevMonsterPos, path, startAt) {
    if (!path || !path.length) return;
    _updateGridHighlightsAt(cs, prevMonsterPos);
    _animationEndsAt = startAt + path.length * _GRID_STEP_MS;
    path.forEach((p, i) => {
        const fireAt = startAt + (i + 1) * _GRID_STEP_MS;
        setTimeout(() => {
            if (_lastState !== cs) return;
            _updateGridHighlightsAt(cs, p);
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

    if (cs.phase === 'active') return;

    // Hold the terminal/death-save UI until the killing-blow dice + log have
    // finished playing out. _logQueueTailAt is the absolute time of the last
    // scheduled emit; show the panel right after it.
    const showAt = Math.max(performance.now(), _logQueueTailAt);
    const delay  = Math.max(0, showAt - performance.now());

    setTimeout(() => {
        // Bail if a newer render has superseded this one or combat exited.
        if (_lastState !== cs) return;

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
            You lie bleeding. Three successes stabilise you; three failures and you're gone.
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
        <button style="${rollBtnStyle}" onclick="window.rollDeathSave()" title="Roll a d20 — 10+ is a success, nat 20 revives, nat 1 = two failures">
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
    const mReach      = cs.monster_melee_reach ?? 0;
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
        const oob        = tx < 0 || tx >= GRID_COLS || ty < 0 || ty >= GRID_ROWS;
        const intoMon    = tx === mPos.x && ty === mPos.y;
        const noBudget   = remaining <= 0;
        const disabled   = oob || intoMon || noBudget;

        // OA warning: moving this direction would leave monster's reach
        const prevR = Math.max(Math.abs(pPos.x - mPos.x), Math.abs(pPos.y - mPos.y));
        const newR  = Math.max(Math.abs(tx     - mPos.x), Math.abs(ty     - mPos.y));
        const provokesOA = !disabled && !disengaged && mReach > 0 &&
                           prevR <= mReach && newR > mReach;
        const label = provokesOA ? `⚠${d.label}` : d.label;
        const title = disabled
            ? (noBudget ? 'No movement left' : intoMon ? 'Blocked by enemy' : 'Out of bounds')
            : provokesOA
                ? `Move ${d.label} — will provoke opportunity attack! Disengage first.`
                : `Move ${d.label}`;
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
    const inMonsterReach = range <= mReach && mReach > 0;
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

    const fleeBtn = actionUsed
        ? _B_GRAYED('🏃 Flee', 'Action already used')
        : range < 3
            ? _B_GRAYED('🏃 Flee (need range ≥ 3)', 'Move away on the grid first')
            : `<button style="${_B('color:#fbbf24;')}" onclick="window.doFlee()"
                    title="Attempt to escape — success chance based on range and speed">🏃 Flee</button>`;

    if (npcEl) npcEl.innerHTML = `
        <h3 style="color:#9ca3af;font-size:8px;font-weight:bold;text-transform:uppercase;margin-bottom:2px;">Turn</h3>
        <div style="display:flex;flex-direction:column;gap:2px;">
            ${disengageBtn}
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

// _renderMonsterHP drains the monster HP bar to the cs values.
function _renderMonsterHP(cs) {
    const monster = cs.monsters?.[0];
    if (!monster) return;
    const pct = monster.max_hp > 0
        ? Math.max(0, Math.min(100, (monster.current_hp / monster.max_hp) * 100))
        : 0;
    const bar = $id('combat-monster-hp-bar');
    if (bar) bar.style.width = `${pct}%`;
    _setText('combat-monster-hp-text', `${monster.current_hp} / ${monster.max_hp} HP`);
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
