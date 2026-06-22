/**
 * Ability-Point Allocation Modal
 *
 * Lets the player spend banked ability points (earned on the level cadence) into
 * their six ability scores. Opens from the level-up modal and from the
 * progression guide. All validation (unspent > 0, per-stat cap 20) is enforced
 * server-side; this renders the state and POSTs each allocation.
 *
 * @module systems/abilityAllocateModal
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { refreshGameState } from '../state/gameState.js';
import { updateAllDisplays } from '../ui/displayCoordinator.js';

const ABILITIES = [
    ['Strength', 'STR'],
    ['Dexterity', 'DEX'],
    ['Constitution', 'CON'],
    ['Intelligence', 'INT'],
    ['Wisdom', 'WIS'],
    ['Charisma', 'CHA'],
];

let busy = false;
let dirty = false; // a point was spent since opening → resync on close

/** Open the allocation modal and load current points + scores. */
export async function openAbilityAllocate() {
    const modal = document.getElementById('ability-allocate-modal');
    if (!modal) return;

    const header = document.getElementById('ability-allocate-points');
    const list = document.getElementById('ability-allocate-list');
    if (header) header.textContent = 'Loading…';
    if (list) list.innerHTML = '';
    dirty = false;
    modal.classList.remove('hidden');

    try {
        renderAllocate(await gameAPI.getAbilityPoints());
    } catch (error) {
        logger.error('Failed to load ability points:', error);
        if (header) header.textContent = 'Could not load ability points.';
    }
}

/** Close the modal and resync the rest of the UI if anything was allocated. */
export function closeAbilityAllocate() {
    document.getElementById('ability-allocate-modal')?.classList.add('hidden');
    if (dirty) {
        dirty = false;
        resyncDisplays();
    }
}

function renderAllocate(data) {
    const header = document.getElementById('ability-allocate-points');
    const list = document.getElementById('ability-allocate-list');
    const cap = data.cap ?? 20;

    if (header) {
        header.textContent = data.unspent > 0
            ? `${data.unspent} point${data.unspent === 1 ? '' : 's'} to spend`
            : 'No points to spend';
    }
    if (!list) return;

    list.innerHTML = '';
    for (const [name, abbr] of ABILITIES) {
        const score = data.scores?.[name] ?? 10;
        const atCap = score >= cap;
        const canSpend = data.unspent > 0 && !atCap && !busy;

        const row = document.createElement('div');
        row.style.cssText = 'display:flex; align-items:center; gap:6px; padding:3px 2px; border-bottom:1px solid #2f2f2f;';
        row.innerHTML =
            `<span style="font-size:8px; color:#fff; width:28px; font-weight:bold;">${abbr}</span>` +
            `<span style="flex:1; font-size:7px; color:#9ca3af;">${name}${atCap ? ' <span style="color:#fcd34d;">(max)</span>' : ''}</span>` +
            `<span style="font-size:11px; color:${atCap ? '#fcd34d' : '#fff'}; width:22px; text-align:right; font-weight:bold;">${score}</span>` +
            `<button data-ability="${name}" ${canSpend ? '' : 'disabled'} title="Spend a point on ${name}" style="font-size:11px; width:20px; height:20px; line-height:1; cursor:${canSpend ? 'pointer' : 'default'}; color:${canSpend ? '#fff' : '#555'}; background:${canSpend ? '#15803d' : '#222'}; border-top:1px solid ${canSpend ? '#4ade80' : '#333'}; border-left:1px solid ${canSpend ? '#4ade80' : '#333'}; border-right:1px solid #052e16; border-bottom:1px solid #052e16;">+</button>`;

        const btn = row.querySelector('button');
        if (canSpend && btn) btn.addEventListener('click', () => spend(name));
        list.appendChild(row);
    }
}

async function spend(ability) {
    if (busy) return;
    busy = true;
    try {
        const res = await gameAPI.spendAbilityPoint(ability);
        dirty = true;
        renderAllocate(res); // { unspent, cap, scores, max_hp, max_mana }
        // Reflect derived maxima immediately (CON / casting-stat bumps).
        setText('max-hp', res.max_hp);
        setText('max-mana', res.max_mana);
    } catch (error) {
        logger.error('Failed to spend ability point:', error);
        const header = document.getElementById('ability-allocate-points');
        if (header) header.textContent = error.message || 'Could not spend point.';
    } finally {
        busy = false;
    }
}

function setText(id, val) {
    if (val == null) return;
    const el = document.getElementById(id);
    if (el) el.textContent = val;
}

async function resyncDisplays() {
    // Pull fresh state so the stats tab + HP/MP bars reflect the new scores.
    try {
        await refreshGameState();
        updateAllDisplays();
    } catch (error) {
        logger.debug('resync after allocate skipped:', error);
    }
}

/**
 * Show/hide the "you have N points to spend" prompt on the level-up modal.
 * Called when the level-up modal opens.
 */
export async function refreshLevelUpPointsPrompt() {
    const prompt = document.getElementById('level-up-points-prompt');
    if (!prompt) return;
    try {
        const data = await gameAPI.getAbilityPoints();
        if (data.unspent > 0) {
            setText('level-up-points-count', data.unspent);
            prompt.classList.remove('hidden');
        } else {
            prompt.classList.add('hidden');
        }
    } catch {
        prompt.classList.add('hidden');
    }
}

if (typeof window !== 'undefined') {
    window.openAbilityAllocate = openAbilityAllocate;
    window.closeAbilityAllocate = closeAbilityAllocate;
    window.refreshLevelUpPointsPrompt = refreshLevelUpPointsPrompt;
}
