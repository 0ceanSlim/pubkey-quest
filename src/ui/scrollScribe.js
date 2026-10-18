/**
 * Scroll scribing panel — opened from "📜 Scribe" inside a building with a
 * crafting station. Lists the spells the caster could scribe (server-filtered:
 * known, within their slot level) with gold, time, chance and anything
 * missing. The server does every check and the roll.
 *
 * @module ui/scrollScribe
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { refreshGameState } from '../state/gameState.js';
import { updateAllDisplays } from './displayCoordinator.js';
import { showActionText } from './messaging.js';

let busy = false;

const BTN = 'font-size:8px; color:#fff; padding:1px 6px; cursor:pointer;';
const BTN_ON = 'background:#7c2d12; border-top:1px solid #fdba74; border-left:1px solid #fdba74; border-right:1px solid #431407; border-bottom:1px solid #431407;';
const BTN_OFF = 'background:#333; border:1px solid #555; color:#888; cursor:not-allowed;';

/** Open the panel over the scene. */
export async function openScrollScribe() {
    closeScrollScribe();
    const scene = document.getElementById('scene-container') || document.body;
    const overlay = document.createElement('div');
    overlay.id = 'scroll-scribe-overlay';
    overlay.style.cssText = 'position:absolute; inset:0; z-index:120; background:rgba(0,0,0,0.9); display:flex; align-items:center; justify-content:center;';
    overlay.addEventListener('click', (e) => { if (e.target === overlay) closeScrollScribe(); });

    const panel = document.createElement('div');
    panel.style.cssText = 'width:96%; max-width:480px; max-height:92%; display:flex; flex-direction:column; background:#1a140f; border-top:2px solid #5a4636; border-left:2px solid #5a4636; border-right:2px solid #0a0705; border-bottom:2px solid #0a0705; padding:8px;';
    panel.innerHTML =
        '<div class="flex justify-between items-center mb-1">' +
        '<span class="text-yellow-400 font-bold" style="font-size:11px;">📜 Scribe Scrolls</span>' +
        '<button id="scroll-scribe-close" class="text-white px-2 py-1 font-bold" style="background:#dc2626;border-top:2px solid #ef4444;border-left:2px solid #ef4444;border-right:2px solid #991b1b;border-bottom:2px solid #991b1b;font-size:10px;">Close</button></div>' +
        '<div id="scroll-scribe-info" style="font-size:7px; color:#d6c7b0; line-height:1.4; margin-bottom:4px;">Loading…</div>' +
        '<div id="scroll-scribe-list" style="overflow-y:auto; flex:1; min-height:0;"></div>';
    overlay.appendChild(panel);
    scene.appendChild(overlay);
    panel.querySelector('#scroll-scribe-close').onclick = closeScrollScribe;
    await load();
}

export function closeScrollScribe() {
    document.getElementById('scroll-scribe-overlay')?.remove();
}

async function load() {
    const info = document.getElementById('scroll-scribe-info');
    try {
        const res = await gameAPI.getScribable();
        if (!res.success) {
            if (info) info.textContent = res.error || 'You cannot scribe scrolls.';
            return;
        }
        render(res.data);
    } catch (error) {
        logger.error('Failed to load scribable scrolls:', error);
        if (info) info.textContent = 'Could not load your scrolls.';
    }
}

function hours(min) {
    return min % 60 === 0 ? `${min / 60}h` : `${min} min`;
}

function render(st) {
    const info = document.getElementById('scroll-scribe-info');
    const list = document.getElementById('scroll-scribe-list');
    if (!info || !list) return;
    const ability = st.ability ? st.ability.charAt(0).toUpperCase() + st.ability.slice(1) : '';
    info.innerHTML = st.here
        ? `Scribe a spell you know onto parchment. Your ${ability} sets the chance; a failed attempt still spends the materials, gold and time. Ink and a pen must be carried.`
        : `<span style="color:#fca5a5;">You scribe ${st.tradition} scrolls at ${st.where}, not here.</span>`;

    const options = st.options || [];
    if (!options.length) {
        list.innerHTML = '<div style="font-size:8px; color:#9ca3af; text-align:center; padding:8px;">No spells you know can be scribed yet.</div>';
        return;
    }
    list.innerHTML = '';
    for (const o of options) {
        const row = document.createElement('div');
        row.style.cssText = 'padding:4px 2px; border-bottom:1px solid #3a2e24;';
        const lvl = o.level === 0 ? 'Cantrip' : `L${o.level}`;
        const needs = (o.needs || []).join(', ');
        const missing = (o.missing || []).length
            ? `<div style="font-size:6px; color:#fca5a5;">Missing: ${o.missing.join(', ')}</div>` : '';
        row.innerHTML =
            '<div style="display:flex; align-items:center; gap:4px;">' +
            `<span style="flex:1; font-size:8px; color:#f3e8d8; font-weight:bold;">${o.name}</span>` +
            `<span style="font-size:7px; color:#fdba74;">${lvl} · ${o.gold.toLocaleString()} gold · ${hours(o.minutes)} · ${o.chance}%</span>` +
            `<button class="scribe-go" style="${BTN}${o.can_scribe ? BTN_ON : BTN_OFF}" ${o.can_scribe ? '' : 'disabled'}>Scribe</button></div>` +
            `<div style="font-size:6px; color:#a8977f;">Uses: ${needs}</div>` + missing;
        if (o.can_scribe) row.querySelector('.scribe-go').addEventListener('click', () => scribe(o.spell_id));
        list.appendChild(row);
    }
}

async function scribe(spellId) {
    if (busy) return;
    busy = true;
    try {
        const res = await gameAPI.scribeScroll(spellId);
        if (!res.success) {
            showActionText(res.error || 'You cannot scribe that.', 'red', 3500);
        } else {
            showActionText(res.message, res.color || 'green', 4500);
        }
        await refreshGameState();
        updateAllDisplays();
        await load();
    } catch (error) {
        logger.error('Scribe failed:', error);
        showActionText('Scribing failed.', 'red', 3000);
    } finally {
        busy = false;
    }
}

if (typeof window !== 'undefined') {
    window.openScrollScribe = openScrollScribe;
    window.closeScrollScribe = closeScrollScribe;
}
