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

// Hunger level names (higher = better)
const HUNGER_NAMES = ['Famished', 'Hungry', 'Satisfied', 'Full'];

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
    updateWaitDisplay(15);

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

    // Get current fatigue and hunger from game state
    const state = getGameStateSync();
    const currentFatigue = state.character?.fatigue ?? 0;
    const currentHunger = state.character?.hunger ?? 3; // 3 = Full

    // Calculate expected effects based on actual game ticker rates
    // Fatigue: increases by 1 per hour (60 minutes)
    // Hunger: decreases by 1 every 3 hours (180 minutes)
    // These match the backend effects system

    const fatiguePerMinute = 1 / 60; // 1 fatigue per 60 minutes
    const hungerPerMinute = 1 / 180; // 1 hunger loss per 180 minutes

    // Calculate expected fatigue (capped at 10)
    const expectedFatigueIncrease = Math.floor(currentWaitMinutes * fatiguePerMinute);
    const expectedFatigue = Math.min(10, currentFatigue + expectedFatigueIncrease);

    // Calculate expected hunger decrease (capped at 0=Famished)
    const expectedHungerDecrease = Math.floor(currentWaitMinutes * hungerPerMinute);
    const expectedHunger = Math.max(0, currentHunger - expectedHungerDecrease);

    // Update fatigue display
    const fatigueChange = document.getElementById('wait-fatigue-change');
    if (fatigueChange) {
        if (expectedFatigueIncrease > 0) {
            fatigueChange.textContent = `${currentFatigue} → ${expectedFatigue} (+${expectedFatigueIncrease})`;
            fatigueChange.className = 'text-red-400';
        } else {
            fatigueChange.textContent = `${currentFatigue} → ${expectedFatigue}`;
            fatigueChange.className = 'text-gray-400';
        }
    }

    // Update hunger display - show level names
    const hungerChange = document.getElementById('wait-hunger-change');
    if (hungerChange) {
        const currentHungerName = HUNGER_NAMES[currentHunger] || 'Unknown';
        const expectedHungerName = HUNGER_NAMES[expectedHunger] || 'Unknown';

        if (expectedHungerDecrease > 0) {
            hungerChange.textContent = `${currentHungerName} → ${expectedHungerName}`;
            // Color based on how bad it's getting
            if (expectedHunger <= 1) {
                hungerChange.className = 'text-red-400'; // Getting hungry/famished
            } else {
                hungerChange.className = 'text-orange-400';
            }
        } else {
            hungerChange.textContent = `${currentHungerName} → ${expectedHungerName}`;
            hungerChange.className = 'text-gray-400';
        }
    }
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
