/**
 * POI Exploration UI
 *
 * Drives a discovered point-of-interest as a node walk over the scene. The
 * server holds the walk (cmd/server/api/game/poi.go); this module renders each
 * node into the #poi-modal overlay and posts the player's choices back.
 *
 * The overlay pauses the world clock the same way combat does, so travel
 * progress holds while you explore and you return to the same spot on exit. A
 * monster node hands off to the combat UI (combat:started); on victory the
 * combat system calls resumeFromCombat() to reopen the walk where it left off.
 *
 * @module ui/poiExplore
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';
import { eventBus } from '../lib/events.js';
import { smoothClock } from '../systems/smoothClock.js';
import { refreshGameState } from '../state/gameState.js';
import { showMessage } from './messaging.js';

const $ = (id) => document.getElementById(id);
const esc = (s) =>
    String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

// Discovered POIs in the current environment (id, name, position, …), cached for
// the travel-screen markers. Refreshed on entering a travel view and after a new
// discovery.
let _envPOIs = [];

/** Fetch the discovered POIs in the player's current environment. */
export async function loadEnvPOIs() {
    const { npub, saveID } = gameAPI;
    if (!npub || !saveID) {
        _envPOIs = [];
        return _envPOIs;
    }
    try {
        const resp = await fetch(`${API_BASE_URL}/poi/list?npub=${npub}&save_id=${saveID}`);
        const json = await resp.json();
        _envPOIs = resp.ok && json.success && Array.isArray(json.data) ? json.data : [];
    } catch (err) {
        logger.error('loadEnvPOIs error:', err);
        _envPOIs = [];
    }
    return _envPOIs;
}

/** The cached environment POI list (for the travel markers/buttons). */
export function getEnvPOIs() {
    return _envPOIs;
}

async function poiPost(path, body) {
    const resp = await fetch(`${API_BASE_URL}${path}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ npub: gameAPI.npub, save_id: gameAPI.saveID, ...body }),
    });
    return resp.json();
}

/** Enter a discovered POI and render its start node. */
export async function enterPOI(poiId) {
    const json = await poiPost('/poi/enter', { poi_id: poiId });
    if (!json.success) {
        showMessage(json.error ?? 'You cannot enter there.', 'error');
        return;
    }
    openOverlay();
    handleStepData(json.data);
}

/**
 * Open the overlay on a step the server already produced — used when an authored
 * encounter fires on the world tick (the walk is already active server-side, so
 * advances flow through /poi/advance exactly like a POI).
 * @param {Object} step - the encounter's start StepResult
 */
export function openFromStep(step) {
    if (!step) return;
    openOverlay();
    renderStep(step);
}

/** Advance the active walk to a node the player chose. */
async function advancePOI(next) {
    const json = await poiPost('/poi/advance', { next });
    if (!json.success) {
        showMessage(json.error ?? 'You cannot go that way.', 'error');
        return;
    }
    // A node may have changed HP / effects / inventory — pull fresh state so the
    // stats bar reflects it while the overlay stays open.
    await refreshGameState(true);
    handleStepData(json.data);
}

// handleStepData processes a {step, combat_started?, combat?} payload: a monster
// node hands off to combat (overlay hides, clock stays paused), everything else
// renders the step.
function handleStepData(data) {
    if (!data) {
        closeOverlay();
        return;
    }
    if (data.combat_started && data.combat) {
        hideOverlay();
        eventBus.emit('combat:started', data.combat);
        return;
    }
    renderStep(data.step);
}

function renderStep(step) {
    if (!step) {
        closeOverlay();
        return;
    }
    const textEl = $('poi-modal-text');
    const outEl = $('poi-modal-outcome');
    const btnEl = $('poi-modal-buttons');

    if (textEl) textEl.innerHTML = step.text ? esc(step.text) : '';
    if (outEl) {
        outEl.innerHTML = (step.outcome || [])
            .map((o) => `<div style="color:#fcd34d; font-size:10px; margin-bottom:2px;">${esc(o)}</div>`)
            .join('');
    }
    if (!btnEl) return;

    btnEl.innerHTML = '';
    if (Array.isArray(step.choices) && step.choices.length) {
        step.choices.forEach((ch) => btnEl.appendChild(makeButton(ch.label, () => advancePOI(ch.next))));
    } else if (step.next) {
        btnEl.appendChild(makeButton('Continue', () => advancePOI(step.next)));
    } else {
        // Terminal (or a dead-end) — the walk is over server-side; just leave.
        btnEl.appendChild(makeButton('Leave', () => closeOverlay()));
    }
}

// makeButton builds a win95-beveled action button matching the dialogue overlay.
function makeButton(label, onClick) {
    const b = document.createElement('button');
    b.textContent = label;
    b.style.cssText = [
        'width:100%', 'text-align:left', 'color:#fff', 'font-size:9px', 'font-weight:bold',
        'padding:3px 6px', 'cursor:pointer', 'background:#2f7d4f',
        'border-top:1px solid #5fd98a', 'border-left:1px solid #5fd98a',
        'border-right:1px solid #0b3d22', 'border-bottom:1px solid #0b3d22',
    ].join(';');
    b.addEventListener('click', onClick);
    return b;
}

// ─── overlay open/close (clock pause mirrors combat) ────────────────────────────

function openOverlay() {
    smoothClock.pause(); // freeze the world while exploring → travel progress holds
    $('poi-modal')?.classList.remove('hidden');
}

// hideOverlay tucks the overlay away WITHOUT resuming the clock — used when
// handing off to combat (the fight keeps the world frozen).
function hideOverlay() {
    $('poi-modal')?.classList.add('hidden');
}

async function closeOverlay() {
    $('poi-modal')?.classList.add('hidden');
    smoothClock.unpause(); // resume the world tick now the walk is done
    await refreshGameState(true);
    // Re-render the scene we returned to so the travel markers/progress refresh.
    try {
        const m = await import('./locationDisplay.js');
        await m.displayCurrentLocation?.();
    } catch (err) {
        logger.error('post-POI location refresh failed:', err);
    }
}

/**
 * Reopen the walk after a POI fight ends in victory. Called by the combat system
 * with the resumed step. A chained monster node (step.combat set) re-enters
 * combat by fetching the live combat state.
 * @param {Object} step - the resumed POI StepResult
 */
export async function resumeFromCombat(step) {
    if (!step) return;
    if (step.combat) {
        // The resume node started another fight server-side; pull its state and
        // drop back into combat rather than the overlay.
        try {
            const resp = await fetch(
                `${API_BASE_URL}/combat/state?npub=${encodeURIComponent(gameAPI.npub)}&save_id=${encodeURIComponent(gameAPI.saveID)}`
            );
            const cs = await resp.json();
            if (resp.ok && cs.phase) {
                eventBus.emit('combat:started', cs);
                return;
            }
        } catch (err) {
            logger.error('resumeFromCombat: combat re-entry failed:', err);
        }
    }
    openOverlay();
    renderStep(step);
}
