/**
 * Spells & Abilities Display UI Module
 *
 * Dual-view display:
 * - Casters: Spell slots (row layout) + known spells list with columns
 * - Martials: Class-specific ability layouts (Fighter/Barbarian/Monk/Rogue)
 *
 * @module ui/spellsDisplay
 */

import { logger } from '../lib/logger.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getSpellById } from '../state/staticData.js';
import { getClassResourceSync } from '../data/classResources.js';
import { getDamageEmoji } from '../lib/damageTypes.js';
import { gameAPI } from '../lib/api.js';
import { showActionText } from './messaging.js';

// Cache fetched abilities
let abilitiesCache = {};

/**
 * Update the spells/abilities display tab
 * Detects caster vs martial and shows appropriate view
 */
export function updateSpellsDisplay() {
    const state = getGameStateSync();
    const character = state.character;
    if (!character) return;

    const resourceConfig = getClassResourceSync(character.class);
    const isCaster = resourceConfig.type === 'mana';

    const casterView = document.getElementById('caster-view');
    const martialView = document.getElementById('martial-view');

    if (!casterView || !martialView) return;

    if (isCaster) {
        casterView.classList.remove('hidden');
        martialView.classList.add('hidden');
        renderCasterView(character);
    } else {
        casterView.classList.add('hidden');
        martialView.classList.remove('hidden');
        renderMartialView(character, resourceConfig);
    }
}

// ============================================================
// CASTER VIEW
// ============================================================

function renderCasterView(character) {
    renderSpellSlots(character);
    // Overlay live prep state (countdown rings) once the queue is fetched.
    refreshPrepQueue(character);
}

// Map of "slotLevel:slotIndex" → { spell, mins } from the prep queue.
let _prepMap = {};

async function refreshPrepQueue(character) {
    try {
        const q = await gameAPI.getPrepQueue();
        const next = {};
        (q.tasks || []).forEach(t => { next[`${t.slot_level}:${t.slot_index}`] = { spell: t.spell_id, mins: t.ready_in_minutes }; });
        _prepMap = next;
    } catch {
        _prepMap = {};
    }
    renderSpellSlots(character);
    ensurePrepTicker();
}

/**
 * Unprepare a slot (clear it / cancel its prep), then refresh.
 */
async function unprepareSlot(slotLevel, slotIndex) {
    try {
        await gameAPI.unslotSpell(slotLevel, slotIndex);
        showActionText('Spell unprepared', 'green', 2000);
        delete _prepMap[`${slotLevel}:${slotIndex}`];
        updateSpellsDisplay();
    } catch (err) {
        showActionText(`Cannot unprepare: ${err.message}`, 'red', 3000);
    }
}

/** Add a small "×" to a slot cell that unprepares it (stops the cell's click). */
function addUnprepareX(cell, slotLevel, slotIndex) {
    const x = document.createElement('span');
    x.textContent = '×';
    x.title = 'Unprepare';
    x.style.cssText = 'position:absolute; top:-2px; right:2px; color:#f87171; font-size:9px; line-height:1; cursor:pointer;';
    x.onclick = (e) => { e.stopPropagation(); unprepareSlot(slotLevel, slotIndex); };
    cell.appendChild(x);
}

const SVGNS = 'http://www.w3.org/2000/svg';
const SLOT_STYLE = 'position:relative; width:28px; height:28px; flex-shrink:0; background:#1a1a1a; ' +
    'border-top:2px solid #000; border-left:2px solid #000; border-right:2px solid #3a3a3a; border-bottom:2px solid #3a3a3a; ' +
    'clip-path: polygon(3px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0 calc(100% - 3px), 0 3px);';

function prettify(id) { return (id || '').replace(/-/g, ' '); }

/** Full-slot spell school icon (like an inventory item image). */
function spellIconFill(spell) {
    const img = document.createElement('img');
    img.src = `/res/img/spells/${spell?.school}.png`;
    img.style.cssText = 'width:100%; height:100%; object-fit:contain; image-rendering:pixelated; padding:2px;';
    img.onerror = () => { img.style.display = 'none'; };
    return img;
}

/** Small slot number in the top-left corner. */
function cornerNum(n) {
    const el = document.createElement('span');
    el.textContent = n;
    el.style.cssText = 'position:absolute; top:0; left:1px; font-size:6px; color:#9ca3af; line-height:1; pointer-events:none; text-shadow:1px 1px 1px #000;';
    return el;
}

function circleEl(stroke, dash, offset) {
    const c = document.createElementNS(SVGNS, 'circle');
    c.setAttribute('cx', '18'); c.setAttribute('cy', '18'); c.setAttribute('r', '15');
    c.setAttribute('fill', 'none'); c.setAttribute('stroke', stroke); c.setAttribute('stroke-width', '2.5');
    c.setAttribute('stroke-dasharray', String(dash)); c.setAttribute('stroke-dashoffset', String(offset));
    return c;
}

/** Live radial countdown ring over a preparing slot (reuses the hunger/fatigue pattern). */
function prepRing(prep, spell) {
    const total = Math.max((spell?.level || 1) * 60, 1);
    const progress = Math.max(0, Math.min(1, 1 - (prep.mins / total)));
    const circ = 94.25; // 2πr, r=15
    const svg = document.createElementNS(SVGNS, 'svg');
    svg.setAttribute('viewBox', '0 0 36 36');
    svg.style.cssText = 'position:absolute; inset:0; width:100%; height:100%; transform:rotate(-90deg); pointer-events:none;';
    svg.appendChild(circleEl('rgba(255,255,255,0.15)', circ, circ));
    svg.appendChild(circleEl('#f59e0b', circ, circ * (1 - progress)));
    return svg;
}

// Cursor-following name tooltip for spell slots (mouse only).
let _spellTip = null;
function attachNameHover(el, text) {
    el.addEventListener('mousemove', (e) => {
        if (!_spellTip) {
            _spellTip = document.createElement('div');
            _spellTip.style.cssText = 'position:fixed; z-index:2100; pointer-events:none; background:#1a1a1a; color:#fff; border:1px solid #6a6a6a; padding:2px 6px; font-size:9px; white-space:nowrap; box-shadow:1px 1px 0 #000;';
            document.body.appendChild(_spellTip);
        }
        _spellTip.textContent = text;
        _spellTip.style.display = 'block';
        _spellTip.style.left = `${e.clientX + 12}px`;
        _spellTip.style.top = `${e.clientY + 12}px`;
    });
    el.addEventListener('mouseleave', () => { if (_spellTip) _spellTip.style.display = 'none'; });
}

// Right-click context menu for a filled/preparing slot: Info / Replace / Remove.
let _slotMenu = null;
function closeSlotMenu() { if (_slotMenu) { _slotMenu.remove(); _slotMenu = null; } }

function openSlotMenu(e, slotLevel, slotIndex, spellId) {
    e.preventDefault();
    e.stopPropagation();
    closeSlotMenu();
    const menu = document.createElement('div');
    menu.className = 'context-menu fixed bg-gray-800 border-2 border-gray-600 shadow-lg z-50';
    menu.style.cssText = `left:${e.clientX}px; top:${e.clientY}px; min-width:100px;`;
    const add = (label, fn) => {
        const it = document.createElement('div');
        it.textContent = label;
        it.className = 'context-menu-item px-3 py-2 hover:bg-gray-700 cursor-pointer text-white';
        it.style.fontSize = '9px';
        it.onclick = (ev) => { ev.stopPropagation(); closeSlotMenu(); fn(); };
        menu.appendChild(it);
    };
    add('Info', () => window.openSpellModal && window.openSpellModal(spellId));
    add('Replace', () => window.openSpellPicker && window.openSpellPicker(slotLevel, slotIndex));
    add('Remove', () => removeSlot(slotLevel, slotIndex));
    document.body.appendChild(menu);
    _slotMenu = menu;
    setTimeout(() => document.addEventListener('click', closeSlotMenu, { once: true }), 0);
}

async function removeSlot(slotLevel, slotIndex) {
    try {
        await gameAPI.unslotSpell(slotLevel, slotIndex);
        showActionText('Spell removed', 'green', 2000);
        await refreshGameState(true);
        updateSpellsDisplay();
    } catch (err) {
        showActionText(`Cannot remove: ${err.message}`, 'red', 3000);
    }
}

/** Build one 28px spell-slot cell: empty (number) / preparing (icon + ring) / prepared (icon). */
function makeSlotCell(level, slotData, displayNum) {
    const prep = _prepMap[`${level}:${slotData.slot}`];
    const cell = document.createElement('div');
    cell.className = 'flex items-center justify-center cursor-pointer hover:brightness-125';
    cell.style.cssText = SLOT_STYLE;

    if (slotData.spell) {
        const spell = getSpellById(slotData.spell);
        cell.appendChild(spellIconFill(spell));
        cell.appendChild(cornerNum(displayNum));
        attachNameHover(cell, spell?.name || prettify(slotData.spell));
        cell.onclick = () => window.openSpellModal && window.openSpellModal(slotData.spell); // info only
        cell.oncontextmenu = (e) => openSlotMenu(e, level, slotData.slot, slotData.spell);
    } else if (prep) {
        const spell = getSpellById(prep.spell);
        const icon = spellIconFill(spell);
        icon.style.opacity = '0.4';
        cell.appendChild(icon);
        cell.appendChild(prepRing(prep, spell));
        cell.appendChild(cornerNum(displayNum));
        attachNameHover(cell, `Preparing ${spell?.name || prettify(prep.spell)} — ${prep.mins}m`);
        cell.onclick = () => window.openSpellModal && window.openSpellModal(prep.spell); // info only
        cell.oncontextmenu = (e) => openSlotMenu(e, level, slotData.slot, prep.spell);
    } else {
        const n = document.createElement('span');
        n.textContent = displayNum;
        n.style.cssText = 'font-size:11px; color:#4b5563;';
        cell.appendChild(n);
        cell.title = 'Prepare a spell';
        cell.onclick = () => window.openSpellPicker && window.openSpellPicker(level, slotData.slot);
    }
    return cell;
}

/** Render spell slots grouped by level: a label line + a horizontal row of numbered slots. */
function renderSpellSlots(character) {
    const container = document.getElementById('spell-slots-container');
    if (!container || !character?.spell_slots) return;
    container.innerHTML = '';

    const sortedLevels = Object.keys(character.spell_slots).sort((a, b) => {
        if (a === 'cantrips') return -1;
        if (b === 'cantrips') return 1;
        return (parseInt(a.split('_')[1]) || 0) - (parseInt(b.split('_')[1]) || 0);
    });

    sortedLevels.forEach(level => {
        const slots = character.spell_slots[level];
        if (!Array.isArray(slots) || slots.length === 0) return;

        const group = document.createElement('div');
        group.className = 'mb-2';

        const label = document.createElement('div');
        label.className = 'text-gray-300 mb-1';
        label.style.fontSize = '8px';
        label.textContent = level === 'cantrips' ? 'Cantrips' : `Level ${level.split('_')[1]}`;
        group.appendChild(label);

        const rowEl = document.createElement('div');
        rowEl.className = 'flex gap-1';
        rowEl.style.cssText = 'overflow-x:auto; padding-bottom:2px;';
        slots.forEach((slotData, i) => rowEl.appendChild(makeSlotCell(level, slotData, i + 1)));
        group.appendChild(rowEl);

        container.appendChild(group);
    });
}

// Re-fetch the prep queue on an interval while a prep is active + the tab is
// open, so the countdown rings tick down live.
let _prepTicker = null;
function ensurePrepTicker() {
    const active = Object.keys(_prepMap).length > 0;
    if (active && !_prepTicker) {
        _prepTicker = setInterval(() => {
            const cv = document.getElementById('caster-view');
            if (cv && !cv.classList.contains('hidden')) {
                const ch = getGameStateSync()?.character;
                if (ch) refreshPrepQueue(ch);
            }
        }, 3000);
    } else if (!active && _prepTicker) {
        clearInterval(_prepTicker);
        _prepTicker = null;
    }
}


// ============================================================
// MARTIAL VIEW
// ============================================================

async function renderMartialView(character, resourceConfig) {
    const className = character.class.toLowerCase();
    const level = character.level || 1;

    // Hide all layouts, show the right one
    ['fighter', 'barbarian', 'monk', 'rogue'].forEach(c => {
        const el = document.getElementById(`${c}-layout`);
        if (el) el.classList.toggle('hidden', c !== className);
    });

    // Fetch abilities from API
    const abilities = await fetchAbilities(className, level);
    if (!abilities) return;

    switch (className) {
        case 'fighter':
            renderFighterLayout(abilities, level, resourceConfig);
            break;
        case 'barbarian':
            renderBarbarianLayout(abilities, level, resourceConfig);
            break;
        case 'monk':
            renderMonkLayout(abilities, level, resourceConfig);
            break;
        case 'rogue':
            renderRogueLayout(abilities, level, resourceConfig);
            break;
    }
}

async function fetchAbilities(className, level) {
    const cacheKey = `${className}-${level}`;
    if (abilitiesCache[cacheKey]) return abilitiesCache[cacheKey];

    try {
        const response = await fetch(`/api/abilities?class=${className}&level=${level}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = await response.json();
        if (data.success) {
            abilitiesCache[cacheKey] = data.abilities;
            return data.abilities;
        }
    } catch (err) {
        logger.error('Failed to fetch abilities:', err);
    }
    return null;
}

/**
 * Create an ability card element (shared across layouts)
 */
function createAbilityCard(ability, level, resourceConfig, options = {}) {
    const { width = '100%', compact = false } = options;
    const card = document.createElement('div');
    const isUnlocked = ability.is_unlocked;
    const isPassive = ability.cooldown === 'passive';

    card.className = 'cursor-pointer';
    card.style.cssText = `
        width: ${width};
        padding: ${compact ? '4px 6px' : '6px 8px'};
        background: ${isUnlocked ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.3)'};
        border: 1px solid ${isUnlocked ? resourceConfig.color + '60' : '#333'};
        border-radius: 3px;
        opacity: ${isUnlocked ? '1' : '0.5'};
        font-size: 7px;
        box-sizing: border-box;
    `;

    if (isUnlocked) {
        card.addEventListener('mouseenter', () => { card.style.borderColor = resourceConfig.color; });
        card.addEventListener('mouseleave', () => { card.style.borderColor = resourceConfig.color + '60'; });
    }

    // Name row
    const nameRow = document.createElement('div');
    nameRow.className = 'flex items-center justify-between';

    const nameSpan = document.createElement('span');
    nameSpan.className = 'font-bold';
    nameSpan.style.color = isUnlocked ? '#fff' : '#666';
    nameSpan.textContent = isUnlocked ? ability.name : `🔒 ${ability.name}`;
    nameRow.appendChild(nameSpan);

    if (!isUnlocked) {
        const lvlSpan = document.createElement('span');
        lvlSpan.className = 'text-gray-600 flex-shrink-0';
        lvlSpan.style.fontSize = '6px';
        lvlSpan.textContent = `Lv ${ability.unlock_level}`;
        nameRow.appendChild(lvlSpan);
    }

    card.appendChild(nameRow);

    // Cost + summary
    if (isUnlocked && !compact) {
        const infoRow = document.createElement('div');
        infoRow.style.cssText = 'margin-top: 2px; font-size: 6px;';

        const costText = isPassive ? 'Passive' : `${ability.resource_cost} ${resourceConfig.short_label}`;
        const tierSummary = ability.current_tier?.summary || '';

        infoRow.innerHTML = `
            <span style="color: ${resourceConfig.color};">${costText}</span>
            <span class="text-gray-400"> · </span>
            <span class="text-gray-300">${tierSummary}</span>
        `;
        card.appendChild(infoRow);
    }

    // Click handler
    card.onclick = () => window.openAbilityModal && window.openAbilityModal(ability);

    return card;
}

// ============================================================
// FIGHTER: Battle Formation (2-col grid, 3 rows)
// ============================================================

function renderFighterLayout(abilities, level, resourceConfig) {
    const grid = document.getElementById('fighter-abilities-grid');
    if (!grid) return;
    grid.innerHTML = '';

    abilities.forEach(ability => {
        grid.appendChild(createAbilityCard(ability, level, resourceConfig));
    });
}

// ============================================================
// BARBARIAN: Rage Escalation (tiered sections, bottom to top)
// ============================================================

function renderBarbarianLayout(abilities, level, resourceConfig) {
    const container = document.getElementById('barbarian-abilities-tiers');
    if (!container) return;
    container.innerHTML = '';

    // Group abilities into themed tiers
    const tierNames = ['RAGE', 'WRATH', 'FURY', 'PRIMAL'];
    const tierGroups = [
        abilities.filter(a => a.unlock_level <= 3),
        abilities.filter(a => a.unlock_level > 3 && a.unlock_level <= 7),
        abilities.filter(a => a.unlock_level > 7 && a.unlock_level <= 10),
        abilities.filter(a => a.unlock_level > 10),
    ];

    tierGroups.forEach((group, i) => {
        if (group.length === 0) return;

        const section = document.createElement('div');
        section.style.cssText = `
            display: block;
            border: 1px solid ${resourceConfig.color}30;
            border-radius: 3px;
            padding: 4px;
            background: rgba(0,0,0,0.2);
            width: 100%;
            box-sizing: border-box;
        `;

        // Tier label
        const label = document.createElement('div');
        label.className = 'text-center font-bold mb-1';
        label.style.cssText = `font-size: 6px; color: ${resourceConfig.color}; letter-spacing: 2px;`;
        label.textContent = `── ${tierNames[i]} ──`;
        section.appendChild(label);

        // Abilities in this tier
        const abilitiesDiv = document.createElement('div');
        abilitiesDiv.className = 'space-y-1';
        group.forEach(ability => {
            abilitiesDiv.appendChild(createAbilityCard(ability, level, resourceConfig, { compact: true }));
        });
        section.appendChild(abilitiesDiv);

        container.appendChild(section);
    });
}

// ============================================================
// MONK: Chakra (diamond/centered pattern)
// ============================================================

function renderMonkLayout(abilities, level, resourceConfig) {
    const container = document.getElementById('monk-abilities-diamond');
    if (!container) return;
    container.innerHTML = '';

    // Diamond pattern: alternate between centered (1) and side-by-side (2)
    // Sort by unlock level, then arrange in diamond
    const sorted = [...abilities].sort((a, b) => a.unlock_level - b.unlock_level);

    // Pattern: [2 side], [1 center], [2 side], [1 center], etc.
    // Bottom = early abilities, top = late abilities (reversed for visual)
    const rows = [];
    let idx = 0;
    let isSide = true;
    while (idx < sorted.length) {
        if (isSide && idx + 1 < sorted.length) {
            rows.push([sorted[idx], sorted[idx + 1]]);
            idx += 2;
        } else {
            rows.push([sorted[idx]]);
            idx += 1;
        }
        isSide = !isSide;
    }

    // Reverse so strongest is at top
    rows.reverse();

    rows.forEach(rowAbilities => {
        const rowDiv = document.createElement('div');
        rowDiv.className = 'flex gap-2 justify-center';
        rowDiv.style.width = '100%';

        const cardWidth = rowAbilities.length === 1 ? '70%' : '48%';
        rowAbilities.forEach(ability => {
            rowDiv.appendChild(createAbilityCard(ability, level, resourceConfig, { width: cardWidth }));
        });

        container.appendChild(rowDiv);
    });
}

// ============================================================
// ROGUE: Shadow Web (web/network pattern with connecting lines)
// ============================================================

function renderRogueLayout(abilities, level, resourceConfig) {
    const container = document.getElementById('rogue-abilities-web');
    if (!container) return;
    container.innerHTML = '';

    // Web pattern: center, diverge, converge, diverge, center
    const sorted = [...abilities].sort((a, b) => a.unlock_level - b.unlock_level);

    // Pattern: [2 side], [1 center], [2 side], [1 center], etc.
    const rows = [];
    let idx = 0;
    let isSide = true;
    while (idx < sorted.length) {
        if (isSide && idx + 1 < sorted.length) {
            rows.push([sorted[idx], sorted[idx + 1]]);
            idx += 2;
        } else {
            rows.push([sorted[idx]]);
            idx += 1;
        }
        isSide = !isSide;
    }

    // Reverse so strongest at top
    rows.reverse();

    rows.forEach((rowAbilities, rowIdx) => {
        // Add connecting lines between rows
        if (rowIdx > 0) {
            const connector = document.createElement('div');
            connector.className = 'flex justify-center';
            connector.style.cssText = `color: ${resourceConfig.color}40; font-size: 8px; line-height: 1;`;
            const prevLen = rows[rowIdx - 1].length;
            const curLen = rowAbilities.length;
            if (prevLen === 1 && curLen === 2) {
                connector.textContent = '╱     ╲';
            } else if (prevLen === 2 && curLen === 1) {
                connector.textContent = '╲     ╱';
            } else {
                connector.textContent = '│';
            }
            container.appendChild(connector);
        }

        const rowDiv = document.createElement('div');
        rowDiv.className = 'flex gap-2 justify-center';
        rowDiv.style.width = '100%';

        const cardWidth = rowAbilities.length === 1 ? '65%' : '48%';
        rowAbilities.forEach(ability => {
            rowDiv.appendChild(createAbilityCard(ability, level, resourceConfig, { width: cardWidth }));
        });

        container.appendChild(rowDiv);
    });
}

logger.debug('Spells & abilities display module loaded');
