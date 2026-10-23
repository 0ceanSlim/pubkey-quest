/**
 * 144x Real-Time Clock System (Delta Architecture)
 *
 * Integrates the delta-based systems:
 * - smoothClock.js: 60fps interpolated clock display
 * - tickManager.js: 417ms tick backend sync
 * - deltaApplier.js: Surgical DOM updates
 *
 * Time display runs at 60fps for smooth visuals.
 */

import { logger } from '../lib/logger.js';
import { getGameStateSync } from '../state/gameState.js';
import { gameAPI } from '../lib/api.js';
import { eventBus } from '../lib/events.js';

// Import delta architecture systems
import { smoothClock } from './smoothClock.js';
import { tickManager } from './tickManager.js';
import { deltaApplier } from './deltaApplier.js';
import { getSocialState } from './socialPoll.js';

/** A party member who isn't the leader: the clock is the leader's to run. */
export function isPartyFollower() {
    const party = getSocialState()?.party;
    return Boolean(party && !party.you_lead);
}

/**
 * Initialize the clock system
 */
export function initTimeClock() {
    logger.info('Initializing 144x time clock');

    initDeltaSystem();

    // The button follows the clock; the clock follows the server's
    // clock_running (on every action response and every social poll).
    eventBus.on('clock:pauseChanged', () => updatePlayPauseButton());
    eventBus.on('clock:running', (running) => smoothClock.setRunning(running));
    eventBus.on('social:party', () => updatePlayPauseButton());

    // Update button state
    updatePlayPauseButton();

    logger.debug('Time clock initialized (paused)');
}

/**
 * Initialize the delta-based clock system
 */
function initDeltaSystem() {
    logger.info('Using delta architecture for time updates');

    // Get initial time from game state (may not be loaded yet)
    syncClockToGameState();

    // Start smooth clock animation (60fps) - starts paused
    smoothClock.start();

    // Start tick manager (417ms ticks) - respects pause state
    tickManager.start();

    // Listen for character stat updates - update cache only, NO full re-renders
    eventBus.on('character:statsUpdated', (data) => {
        // Update cached game state silently (delta applier handles DOM)
        const state = getGameStateSync();
        if (state.character) {
            if (data.fatigue !== undefined) state.character.fatigue = data.fatigue;
            if (data.hunger !== undefined) state.character.hunger = data.hunger;
            if (data.hp !== undefined) state.character.hp = data.hp;
            if (data.active_effects) state.character.active_effects = data.active_effects;
        }
        // DO NOT emit gameStateChange - delta applier already updated DOM surgically
    });

    // Listen for game state loaded event to re-sync clock with correct time
    eventBus.on('gameStateLoaded', (loadedState) => {
        logger.info('📥 Game state loaded, re-syncing clock');
        // Pass the loaded state directly to avoid any timing issues
        syncClockToGameState(loadedState);
        // Clear smoothClock's DOM cache in case elements were recreated
        smoothClock.clearCache();
    });

    logger.info('Delta architecture initialized');
}

/**
 * Sync the clock to the current game state
 * @param {Object} stateOverride - Optional state to use instead of calling getGameStateSync
 */
function syncClockToGameState(stateOverride = null) {
    const state = stateOverride || getGameStateSync();
    logger.debug('syncClockToGameState called with state:', state ? 'exists' : 'null');

    // Check if we have actual time data - don't sync with defaults
    const hasCharacterTime = state?.character?.time_of_day !== undefined && state?.character?.time_of_day !== null;
    const hasTopLevelTime = state?.time_of_day !== undefined && state?.time_of_day !== null;

    if (hasCharacterTime) {
        let timeOfDay = state.character.time_of_day;
        logger.debug('Raw time_of_day from state:', timeOfDay);

        // Convert old hour format if needed
        if (timeOfDay < 24) {
            timeOfDay = timeOfDay * 60;
        }
        const currentDay = state.character.current_day || 1;

        // Sync smooth clock to state (force sync on initial load)
        smoothClock.syncFromBackend(timeOfDay, currentDay, true);
        logger.info(`Clock synced to game state: Day ${currentDay}, time ${timeOfDay} mins`);
    } else if (hasTopLevelTime) {
        // Alternative: state might have time_of_day at top level
        let timeOfDay = state.time_of_day;
        if (timeOfDay < 24) {
            timeOfDay = timeOfDay * 60;
        }
        const currentDay = state.current_day || 1;
        smoothClock.syncFromBackend(timeOfDay, currentDay, true);
        logger.info(`Clock synced from top-level state: Day ${currentDay}, time ${timeOfDay} mins`);
    } else {
        // No state available yet - DON'T sync with default values
        // This keeps hasInitialSync = false so clock displays "--:-- --" until real data
        logger.debug('No game state available yet, waiting for real data before clock sync');
    }
}

/**
 * Toggle play/pause
 */
export function togglePause() {
    return smoothClock.isRunning() ? pause() : play();
}

/**
 * Get current pause state
 */
export function isPausedState() {
    return smoothClock.isPausedState();
}

/**
 * Get current time including minutes (uses interpolated display time for smooth UI)
 * @returns {{hour: number, minute: number, synced: boolean}} Current game time
 */
export function getCurrentTime() {
    const time = smoothClock.getCurrentTime();
    return { hour: time.hours, minute: time.minutes, synced: time.synced };
}

/**
 * Pause: the player's choice, recorded on the server. Stops locally at once.
 */
export async function pause() {
    if (isPartyFollower()) return;
    smoothClock.setRunning(false);
    try {
        if (gameAPI.initialized) await gameAPI.sendAction('set_clock', { running: false });
    } catch (error) {
        logger.warn('Failed to pause the clock:', error);
    }
}

/**
 * Play: the player's choice (the ▶ button, and actions that set time moving).
 * The clock only starts once the server has it — otherwise a tick could reach
 * the server first, find the clock stopped, and stop it again.
 * Followers can't: the leader runs the party's clock.
 */
export async function play() {
    if (isPartyFollower()) return;
    if (!gameAPI.initialized) {
        smoothClock.setRunning(true);
        return;
    }
    try {
        await gameAPI.sendAction('set_clock', { running: true });
    } catch (error) {
        logger.warn('Failed to start the clock:', error);
    }
}

/**
 * Update the play/pause button appearance
 */
function updatePlayPauseButton() {
    const playButton = document.getElementById('time-play-button');
    const pauseButton = document.getElementById('time-pause-button');
    const waitButton = document.getElementById('time-wait-button');

    if (!playButton || !pauseButton) {
        return;
    }

    // Held (a conversation, a fight, a vault, a find): time is stopped whatever
    // the player chose, so show an hourglass rather than a pause button that
    // looks like the clock is still running.
    const held = smoothClock.isHeld();
    const running = smoothClock.isRunning() && !held;
    playButton.style.display = running || held ? 'none' : '';
    pauseButton.style.display = running || held ? '' : 'none';
    pauseButton.textContent = held ? '⏳' : '⏸';
    const clockText = document.getElementById('time-of-day-text-stats');
    if (clockText) clockText.style.opacity = held ? '0.6' : '';

    // Followers ride the leader's clock; a held clock (a fight, a POI) can't be
    // started or stopped until it's released.
    const follower = isPartyFollower();
    const locked = follower || smoothClock.isHeld();
    for (const btn of [playButton, pauseButton, waitButton]) {
        if (!btn) continue;
        btn.disabled = locked;
        btn.style.opacity = locked ? '0.45' : '';
        btn.style.cursor = locked ? 'not-allowed' : 'pointer';
        btn.title = follower ? "Your party leader keeps the time"
            : held ? 'Time stands still while this is open (talking, fighting, a vault, a find)'
            : btn === waitButton ? 'Wait' : '';
    }
}

/**
 * Cleanup on page unload
 */
export function cleanupTimeClock() {
    smoothClock.stop();
    tickManager.stop();
    deltaApplier.clearCache();
    logger.debug('Delta time systems stopped');
}

// Export for global access (onclick in HTML)
window.timeClock = {
    togglePause,
    pause,
    play,
    getCurrentTime
};

logger.debug('Time clock module loaded');
