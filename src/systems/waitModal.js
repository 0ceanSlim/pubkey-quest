/**
 * Wait Modal System
 * Handles the wait functionality with granular time slider (minutes)
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { updateAllDisplays } from '../ui/displayCoordinator.js';
import { showMessage } from '../ui/messaging.js';
import { deltaApplier } from './deltaApplier.js';
import { smoothClock } from './smoothClock.js';

let currentWaitMinutes = 15;
let updateIntervalId = null;

// The server's preview (/api/game/wait-preview): what each wait length does,
// worked out by running the real wait on a copy of the save. Re-fetched when
// the live clock has moved on a few minutes since.
let preview = null;
let previewAt = null; // game minute the preview was taken at
let previewLoading = false;

async function loadPreview() {
    if (previewLoading) return;
    previewLoading = true;
    try {
        const result = await gameAPI.getWaitPreview();
        if (result?.success) {
            preview = result.data;
            previewAt = liveMinutes();
            document.getElementById('wait-camp-btn')?.classList.toggle('hidden', !preview.can_camp);
            renderEffects();
        }
    } catch (error) {
        logger.warn('Wait preview unavailable:', error);
    } finally {
        previewLoading = false;
    }
}

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** The live clock's time of day, in minutes. */
function liveMinutes() {
    if (window.timeClock && window.timeClock.getCurrentTime) {
        const t = window.timeClock.getCurrentTime();
        return (t.hour * 60) + t.minute;
    }
    const state = getGameStateSync();
    return state.time_of_day || state.character?.time_of_day || 720;
}

/** Draw the preview for the chosen wait length. */
function renderEffects() {
    const box = document.getElementById('wait-effects');
    if (!box) return;
    const step = preview?.steps?.find((s) => s.minutes === currentWaitMinutes);
    // Redrawn on the modal's 100ms tick: only when something changed.
    const key = step ? `${currentWaitMinutes}:${previewAt}` : 'loading';
    if (box.dataset.key === key) return;
    box.dataset.key = key;
    if (!step) {
        box.innerHTML = '<span class="text-gray-400">Working it out…</span>';
        return;
    }
    const row = (label, value, cls = 'text-gray-300') =>
        `<div class="flex justify-between gap-2"><span>${label}</span><span class="${cls} text-right">${value}</span></div>`;
    const change = (from, to, max) => {
        const d = to - from;
        const text = `${from} → ${to}${max ? ` / ${max}` : ''}`;
        if (d > 0) return [`${text} (+${d})`, 'text-green-400'];
        if (d < 0) return [`${text} (${d})`, 'text-red-400'];
        return [text, 'text-gray-400'];
    };
    const lines = [];
    const [hp, hpCls] = change(preview.hp, step.hp, preview.max_hp);
    lines.push(row('❤️ HP', hp, hpCls));
    if (step.hp_lost) lines.push(row('', `${step.hp_lost} lost along the way`, 'text-red-400'));
    if (preview.max_mana > 0) {
        const [mana, manaCls] = change(preview.mana, step.mana, preview.max_mana);
        lines.push(row('✨ Mana', mana, manaCls));
    }
    const hungerCls = step.hunger < preview.hunger ? (step.hunger <= 1 ? 'text-red-400' : 'text-orange-400') : 'text-gray-400';
    lines.push(row('🍽️ Hunger', step.hunger === preview.hunger
        ? esc(step.hunger_name) : `${esc(preview.hunger_name)} → ${esc(step.hunger_name)}`, hungerCls));
    lines.push(row('💤 Fatigue', `${preview.fatigue} (holds; sleep clears it)`, 'text-gray-400'));

    const list = (icon, label, names, cls) => {
        if (names?.length) lines.push(`<div class="${cls}">${icon} ${label}: ${names.map(esc).join(', ')}</div>`);
    };
    list('🔮', 'Ready to cast', step.spells_ready, 'text-purple-300');
    list('⏳', 'Wears off', step.effects_end, 'text-yellow-300');
    list('⚠️', 'Sets in', step.effects_gain, 'text-red-400');
    list('🏪', 'Opens', step.opens, 'text-green-400');
    list('🚪', 'Closes', step.closes, 'text-orange-400');
    if (preview.walking) {
        lines.push(`<div class="text-yellow-300">🚶 You'll keep walking (you haven't stopped).</div>`);
    }
    if (preview.in_wild) {
        lines.push('<div class="text-red-400">⚔️ Out here, something may find you.</div>');
    }
    box.innerHTML = lines.join('');
}

/**
 * Open the wait modal
 */
export function openWaitModal() {
    logger.debug('Opening wait modal');

    const modal = document.getElementById('wait-modal');
    if (!modal) {
        logger.error('Wait modal element not found');
        return;
    }

    // Reset to default 15 minutes
    currentWaitMinutes = 15;
    const slider = document.getElementById('wait-slider');
    if (slider) {
        slider.value = 15;
    }

    // Update display with initial values
    preview = null;
    document.getElementById('wait-camp-btn')?.classList.add('hidden');
    const box = document.getElementById('wait-effects');
    if (box) delete box.dataset.key;
    updateWaitDisplay(15);
    loadPreview();

    // Start updating the display every 100ms to sync with live time
    updateIntervalId = setInterval(() => {
        updateWaitDisplay(currentWaitMinutes);
    }, 100);

    // Show modal
    modal.classList.remove('hidden');
}

/**
 * Close the wait modal
 */
export function closeWaitModal() {
    logger.debug('Closing wait modal');

    const modal = document.getElementById('wait-modal');
    if (modal) {
        modal.classList.add('hidden');
    }

    // Stop the update interval
    if (updateIntervalId) {
        clearInterval(updateIntervalId);
        updateIntervalId = null;
    }
}

/**
 * Format minutes as human-readable duration
 * @param {number} minutes - Duration in minutes
 * @returns {string} Formatted string like "1 hour 30 minutes"
 */
function formatDuration(minutes) {
    const hours = Math.floor(minutes / 60);
    const mins = minutes % 60;

    if (hours === 0) {
        return `${mins} minute${mins !== 1 ? 's' : ''}`;
    } else if (mins === 0) {
        return `${hours} hour${hours !== 1 ? 's' : ''}`;
    } else {
        return `${hours}h ${mins}m`;
    }
}

/**
 * Update the wait display based on slider value (in minutes)
 */
export function updateWaitDisplay(minutes) {
    currentWaitMinutes = parseInt(minutes);

    // Update duration display
    const durationDisplay = document.getElementById('wait-duration-display');
    if (durationDisplay) {
        durationDisplay.textContent = formatDuration(currentWaitMinutes);
    }

    // Calculate target time - use same source as time display for consistency
    let currentTimeMinutes;
    if (window.timeClock && window.timeClock.getCurrentTime) {
        const currentTime = window.timeClock.getCurrentTime();
        currentTimeMinutes = (currentTime.hour * 60) + currentTime.minute;
    } else {
        // Fallback to game state if time clock not available
        const state = getGameStateSync();
        currentTimeMinutes = state.time_of_day || state.character?.time_of_day || 720;
    }

    const targetTimeMinutes = (currentTimeMinutes + currentWaitMinutes) % 1440;

    const targetHours = Math.floor(targetTimeMinutes / 60);
    const targetMins = targetTimeMinutes % 60;
    const targetPeriod = targetHours >= 12 ? 'PM' : 'AM';
    const targetHours12 = targetHours % 12 || 12;

    const targetTimeDisplay = document.getElementById('wait-target-time');
    if (targetTimeDisplay) {
        targetTimeDisplay.textContent = `${targetHours12}:${targetMins.toString().padStart(2, '0')} ${targetPeriod}`;
    }

    // The live clock moved on since the preview was taken: take it again.
    if (preview && previewAt !== null && Math.abs(currentTimeMinutes - previewAt) >= 5) {
        loadPreview();
    }
    renderEffects();
}

/**
 * Confirm and execute the wait action
 */
export async function confirmWait() {
    logger.debug(`Confirming wait for ${currentWaitMinutes} minutes`);

    try {
        // Send minutes directly - backend now supports granular waiting
        const result = await gameAPI.sendAction('wait', {
            minutes: currentWaitMinutes
        });

        if (result.success) {
            // Apply delta if present (surgical updates)
            if (result.delta) {
                logger.debug('Applying wait delta:', result.delta);
                deltaApplier.applyDelta(result.delta);
            }

            // Sync clock to new time from data (force sync for major time jump)
            if (result.data) {
                const timeOfDay = result.data.time_of_day;
                const currentDay = result.data.current_day || 1;
                if (timeOfDay !== undefined) {
                    smoothClock.syncFromBackend(timeOfDay, currentDay, true); // Force sync
                    logger.debug(`Clock synced after wait: Day ${currentDay}, time ${timeOfDay}`);
                }
            }

            showMessage(result.message || `You waited ${formatDuration(currentWaitMinutes)}.`, 'warning');
            closeWaitModal();

            // NOTE: Don't call displayCurrentLocation() here!
            // The delta system already updated buildings/NPCs surgically.
            // A full re-render would use stale cached state and overwrite the delta updates.
        } else {
            showMessage(result.error || 'Failed to wait', 'error');
        }
    } catch (error) {
        logger.error('Failed to execute wait:', error);
        showMessage('Failed to wait', 'error');
    }
}

/**
 * Sleep the night through. The backend routes this: an inn bed when indoors (needs
 * a rented room), or making camp when out in a travel environment (a bedroll makes
 * it comfier; without one it's a rough sleep). Restores HP/mana by how long you
 * sleep and fatigue by comfort; only an inn's fee includes breakfast (hunger).
 */
export async function confirmSleep() {
    logger.debug('Confirming sleep');
    try {
        const result = await gameAPI.sendAction('sleep', {});

        if (result.delta) {
            deltaApplier.applyDelta(result.delta);
        }
        if (result.data && result.data.time_of_day !== undefined) {
            smoothClock.syncFromBackend(result.data.time_of_day, result.data.current_day || 1, true);
        }
        showMessage(result.message || 'You sleep until dawn.', 'success');
        closeWaitModal();
        await refreshGameState(true);
    } catch (error) {
        // A rejected sleep (wrong place / too early) surfaces its reason here; keep
        // the modal open so the player can wait or move instead.
        logger.debug('Sleep not available:', error);
        showMessage(error?.message || "You can't sleep here right now.", 'error');
    }
}

// Export functions to window for onclick handlers
if (typeof window !== 'undefined') {
    window.openWaitModal = openWaitModal;
    window.closeWaitModal = closeWaitModal;
    window.updateWaitDisplay = updateWaitDisplay;
    window.confirmWait = confirmWait;
    window.confirmSleep = confirmSleep;
}
