/**
 * Active-quest tracker chip.
 *
 * Renders a single line over the scene showing the player's current quest
 * objective (e.g. "◆ Speak with the innkeeper (0/1)"). Reads /api/quests/log and
 * shows the journal's tracked (📌) quest — or the first active one — at its
 * first incomplete objective; hides itself when there's no active quest.
 * Clicking it opens that quest in the journal. Refreshes on state changes
 * (throttled) and on load.
 *
 * @module ui/questTracker
 */

import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';
import { eventBus } from '../lib/events.js';
import { logger } from '../lib/logger.js';
import { getTrackedQuestId } from './questDisplay.js';
import { maybeShowTip, showTip, tipSeen } from './tutorialTip.js';

// The quest the tracker shows (its 💡 opens this one's current tip).
let _tracked = null;

/** Open the tracked step's "how it works" note (the tracker's 💡). */
function showTrackedTip() {
    if (_tracked) showTip(_tracked);
}

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
        // The quest pinned in the journal (📌 Track) if it's still active,
        // otherwise the first active one; then its first not-yet-done objective.
        const q = active.find((a) => a.id === getTrackedQuestId()) || active[0];
        const objs = q.objectives || [];
        const obj = objs.find((o) => !o.done) || objs[0];
        if (!obj) {
            el.classList.add('hidden');
            return;
        }
        const prog = obj.target > 1 ? ` (${obj.count}/${obj.target})` : '';
        // A turn-in says how: talk to them and choose "Turn in".
        txt.textContent = obj.turn_in ? `📜 ${obj.description} (Turn in)` : `◆ ${obj.description}${prog}`;
        el.title = `${q.name} — click to open in the journal`;
        el.dataset.quest = q.id;
        el.classList.remove('hidden');
        _tracked = q;
        const tipBtn = document.getElementById('quest-tracker-tip');
        tipBtn?.classList.toggle('hidden', !q.stage_tip);
        // A tip not yet opened glows until it is.
        tipBtn?.classList.toggle('fresh', !!q.stage_tip && !tipSeen(q));
        // A tutorial step's "how it works" note, once.
        for (const a of active) maybeShowTip(a);
    } catch (err) {
        logger.error('quest tracker update failed:', err);
    }
}

// Refresh on state changes — throttled so action bursts don't spam the log API.
// A change inside the window isn't dropped: one trailing refresh catches it up.
// (Quest news on a response refreshes at once, through window.updateQuestTracker.)
let _lastUpdate = 0;
let _trailing = null;
function throttledUpdate() {
    const wait = 2000 - (Date.now() - _lastUpdate);
    if (wait <= 0) {
        _lastUpdate = Date.now();
        updateQuestTracker();
        return;
    }
    if (!_trailing) {
        _trailing = setTimeout(() => {
            _trailing = null;
            _lastUpdate = Date.now();
            updateQuestTracker();
        }, wait);
    }
}

eventBus.on('gameStateChange', throttledUpdate);
eventBus.on('gameStateLoaded', updateQuestTracker);

if (typeof window !== 'undefined') {
    window.updateQuestTracker = updateQuestTracker;
    window.showTrackedTip = showTrackedTip;
}
