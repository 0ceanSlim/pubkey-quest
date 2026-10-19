/**
 * Waystones and Homeward Stones (docs/draft/teleportation-design.md).
 *
 * The waystone panel opens from "🌀 Waystone" in a starting city's town
 * square: every waystone in the network, which are woken (their waystone quest
 * done), and the hop cooldown. Homeward Stones are used from the inventory
 * ("Use"). The server does every check; this only renders and asks.
 *
 * @module ui/waystone
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';
import { refreshGameState } from '../state/gameState.js';
import { updateAllDisplays } from './displayCoordinator.js';
import { showActionText } from './messaging.js';

let busy = false;

async function fetchStatus() {
    const s = window.sessionManager?.getSession?.();
    const saveID = new URLSearchParams(window.location.search).get('save');
    if (!s?.npub || !saveID) throw new Error('not signed in');
    const r = await fetch(`${API_BASE_URL}/teleport/status?npub=${encodeURIComponent(s.npub)}&save_id=${encodeURIComponent(saveID)}`);
    if (!r.ok) throw new Error(`status ${r.status}`);
    return (await r.json()).data;
}

/** After a teleport: refresh everything, the scene included. */
async function afterTeleport(res) {
    showActionText(res.message, res.color || 'purple', 5000);
    await refreshGameState();
    await updateAllDisplays();
}

/** Use a Homeward Stone from the inventory. */
export async function useHomewardStone(itemId) {
    if (busy) return;
    busy = true;
    try {
        const res = await gameAPI.sendAction('use_homeward_stone', { item_id: itemId });
        if (res.success) await afterTeleport(res);
        else showActionText(res.message || res.error || "The stone doesn't stir.", res.color || 'yellow', 4000);
    } catch (error) {
        logger.error('Homeward stone failed:', error);
        showActionText("The stone doesn't stir.", 'red', 3000);
    } finally {
        busy = false;
    }
}

export function closeWaystone() {
    document.getElementById('waystone-overlay')?.remove();
}

/** Open the waystone panel. */
export async function openWaystone() {
    closeWaystone();
    const scene = document.getElementById('scene-container') || document.body;
    const overlay = document.createElement('div');
    overlay.id = 'waystone-overlay';
    overlay.style.cssText = 'position:absolute; inset:0; z-index:120; background:rgba(0,0,0,0.85); display:flex; align-items:center; justify-content:center;';
    overlay.addEventListener('click', (e) => { if (e.target === overlay) closeWaystone(); });
    const panel = document.createElement('div');
    panel.style.cssText = 'width:92%; max-width:360px; max-height:92%; display:flex; flex-direction:column; background:#10131f; border-top:2px solid #4c5a8a; border-left:2px solid #4c5a8a; border-right:2px solid #05060c; border-bottom:2px solid #05060c; padding:8px;';
    panel.innerHTML =
        '<div class="flex justify-between items-center mb-1">' +
        '<span class="font-bold" style="font-size:11px; color:#a5b4fc;">🌀 The Waystone</span>' +
        '<button id="waystone-close" class="text-white px-2 py-1 font-bold" style="background:#dc2626;border-top:2px solid #ef4444;border-left:2px solid #ef4444;border-right:2px solid #991b1b;border-bottom:2px solid #991b1b;font-size:10px;">Close</button></div>' +
        '<div id="waystone-info" style="font-size:7px; color:#c7d2fe; line-height:1.4; margin-bottom:4px;">Loading…</div>' +
        '<div id="waystone-list" style="overflow-y:auto; flex:1; min-height:0;"></div>';
    overlay.appendChild(panel);
    scene.appendChild(overlay);
    panel.querySelector('#waystone-close').onclick = closeWaystone;
    try {
        render(await fetchStatus());
    } catch (error) {
        logger.error('Waystone status failed:', error);
        document.getElementById('waystone-info').textContent = 'The waystone is silent.';
    }
}

function render(st) {
    const info = document.getElementById('waystone-info');
    const list = document.getElementById('waystone-list');
    if (!info || !list) return;
    const notes = ['Ancient elven runes ring the stone. Woken waystones can carry you to one another, once a week. No time passes on the way.'];
    if (st.blocked) notes.push(`<span style="color:#fca5a5;">${st.blocked}</span>`);
    if (st.waystone_cooldown > 0) notes.push(`<span style="color:#fcd34d;">The network is recharging: ${st.waystone_cooldown_text} to go.</span>`);
    if (!st.waystone_here) notes.push('<span style="color:#fca5a5;">You need to stand at the waystone to use it.</span>');
    info.innerHTML = notes.join('<br>');

    list.innerHTML = '';
    for (const d of st.destinations || []) {
        const here = d.city === st.waystone_here;
        const canGo = !here && d.unlocked && st.waystone_here && !st.blocked && !(st.waystone_cooldown > 0);
        const row = document.createElement('div');
        row.style.cssText = 'display:flex; align-items:center; gap:6px; padding:4px 2px; border-bottom:1px solid #222842;';
        const status = here ? '<span style="color:#86efac;">you are here</span>'
            : d.unlocked ? '<span style="color:#a5b4fc;">awake</span>'
            : '<span style="color:#6b7280;">dormant</span>';
        row.innerHTML =
            `<span style="flex:1; font-size:8px; color:${d.unlocked || here ? '#e0e7ff' : '#6b7280'}; font-weight:bold;">${d.name}</span>` +
            `<span style="font-size:7px;">${status}</span>` +
            `<button class="ws-go" style="font-size:8px; padding:1px 6px; ${canGo
                ? 'color:#fff; background:#3730a3; border-top:1px solid #a5b4fc; border-left:1px solid #a5b4fc; border-right:1px solid #1e1b4b; border-bottom:1px solid #1e1b4b; cursor:pointer;'
                : 'color:#666; background:#222; border:1px solid #333; cursor:not-allowed;'}" ${canGo ? '' : 'disabled'}>Travel</button>`;
        if (canGo) row.querySelector('.ws-go').addEventListener('click', () => hop(d.city));
        list.appendChild(row);
    }
    if (!(st.destinations || []).some((d) => d.unlocked)) {
        const hint = document.createElement('div');
        hint.style.cssText = 'font-size:7px; color:#9ca3af; padding:6px 2px; line-height:1.4;';
        hint.textContent = 'Every waystone is dormant. The court arcanist in the Kingdom\'s mage tower studies the network: ask him about waking them.';
        list.appendChild(hint);
    }
}

async function hop(city) {
    if (busy) return;
    busy = true;
    try {
        const res = await gameAPI.sendAction('waystone_hop', { destination: city });
        if (res.success) {
            closeWaystone();
            await afterTeleport(res);
        } else {
            showActionText(res.message || res.error || 'The waystone stays dark.', res.color || 'yellow', 4000);
        }
    } catch (error) {
        logger.error('Waystone hop failed:', error);
        showActionText('The waystone stays dark.', 'red', 3000);
    } finally {
        busy = false;
    }
}

if (typeof window !== 'undefined') {
    window.openWaystone = openWaystone;
    window.useHomewardStone = useHomewardStone;
}
