/**
 * Active-quest tracker chip.
 *
 * Renders a single line over the scene showing the player's current quest
 * objective (e.g. "◆ Speak with the innkeeper (0/1)") — the highest-bang quest
 * UX element (roadmap M3/M6). Reads /api/quests/log, shows the first active
 * quest's first incomplete objective, and hides itself when there's no active
 * quest. Refreshes on state changes (throttled) and on load.
 *
 * @module ui/questTracker
 */

import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';
import { eventBus } from '../lib/events.js';
import { logger } from '../lib/logger.js';

/** Fetch the quest log and render the tracker chip (or hide it). */
export async function updateQuestTracker() {
    const el = document.getElementById('quest-tracker');
    const txt = document.getElementById('quest-tracker-text');
    const { npub, saveID } = gameAPI;
    if (!el || !txt || !npub || !saveID) return;

    try {
        const resp = await fetch(`${API_BASE_URL}/quests/log?npub=${npub}&save_id=${saveID}`);
        const json = await resp.json();
        const active = (resp.ok && json.success && json.data?.active) || [];
        if (!active.length) {
            el.classList.add('hidden');
            return;
        }
        // First active quest, its first not-yet-done objective (fallback: first).
        const q = active[0];
        const objs = q.objectives || [];
        const obj = objs.find((o) => !o.done) || objs[0];
        if (!obj) {
            el.classList.add('hidden');
            return;
        }
        const prog = obj.target > 1 ? ` (${obj.count}/${obj.target})` : '';
        txt.textContent = `◆ ${obj.description}${prog}`;
        el.title = q.name;
        el.classList.remove('hidden');
    } catch (err) {
        logger.error('quest tracker update failed:', err);
    }
}

// Refresh on state changes — throttled so action bursts don't spam the log API.
let _lastUpdate = 0;
function throttledUpdate() {
    const now = Date.now();
    if (now - _lastUpdate < 2000) return;
    _lastUpdate = now;
    updateQuestTracker();
}

eventBus.on('gameStateChange', throttledUpdate);
eventBus.on('gameStateLoaded', updateQuestTracker);

if (typeof window !== 'undefined') {
    window.updateQuestTracker = updateQuestTracker;
}
