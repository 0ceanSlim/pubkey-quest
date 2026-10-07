/**
 * SmoothClock - 60fps interpolated game clock display
 *
 * Provides smooth visual time display independent of tick rate.
 * The display runs at 60fps using requestAnimationFrame for smooth visuals,
 * while syncing to backend authoritative time on each action/tick response.
 *
 * Key features:
 * - 60fps smooth clock display (no jerky updates)
 * - Syncs to backend authoritative time after each action
 * - Pause/unpause support
 * - Day rollover handling
 */

import { logger } from '../lib/logger.js';
import { eventBus } from '../lib/events.js';

// Constants
const TIME_MULTIPLIER = 144; // 144x real-time speed
const MINUTES_PER_DAY = 1440; // 24 hours * 60 minutes

class SmoothClock {
    constructor() {
        // Current interpolated game time
        this.gameTime = 720; // In-game minutes (0-1439), default to noon
        this.gameDay = 1;

        // Backend sync state
        this.lastSyncTime = 720;       // Last backend-confirmed time
        this.lastSyncRealTime = 0;     // Real timestamp (ms) of last sync
        this.lastSyncDay = 1;

        // Animation state
        this.isPaused = true;
        this.animationId = null;
        this.hasInitialSync = false;  // Don't display until first sync from backend

        // DOM element cache
        this.clockEl = null;
        this.dayEl = null;
        this.timeImageEl = null;
        this.lastDisplayedTime = '';
        this.lastDisplayedDay = '';
        this.lastDisplayedHour = -1; // Track hour for image updates (changes every 2 hours)

        // Time-of-day images (each covers 2 hours)
        this.timeImages = [
            '00-midnight.png',     // 0-1 (12 AM - 1 AM)
            '01-twilight.png',     // 2-3
            '02-witching.png',     // 4-5
            '03-dawn.png',         // 6-7
            '04-morning.png',      // 8-9
            '05-latemorning.png',  // 10-11
            '06-highnoon.png',     // 12-13 (12 PM - 1 PM)
            '07-midday.png',       // 14-15
            '08-afternoon.png',    // 16-17
            '09-golden.png',       // 18-19
            '10-dusk.png',         // 20-21
            '11-evening.png'       // 22-23
        ];

        logger.debug('SmoothClock initialized');
    }

    /**
     * Sync clock to backend authoritative time.
     * Called after every action response and tick.
     * Only actually resyncs if there's significant drift (>1 minute) to prevent micro-jumps.
     * @param {number} timeOfDay - In-game minutes (0-1439)
     * @param {number} currentDay - Current game day
     * @param {boolean} forceSync - Force sync even if drift is small (for actions like wait/sleep)
     */
    syncFromBackend(timeOfDay, currentDay, forceSync = false) {
        // First sync - always sync immediately to prevent showing default 12:00 PM
        if (!this.hasInitialSync) {
            this.lastSyncTime = timeOfDay;
            this.lastSyncRealTime = performance.now();
            this.lastSyncDay = currentDay;
            this.gameTime = timeOfDay;
            this.gameDay = currentDay;
            this.hasInitialSync = true;
            logger.info(`Clock initial sync: Day ${currentDay}, ${this.formatTime(timeOfDay)}`);
            this.updateDisplay(); // Force immediate display update
            return;
        }

        // Calculate current interpolated time for comparison
        const currentInterpolated = this.gameTime;
        const currentDay_interpolated = this.gameDay;

        // Calculate drift
        let drift = Math.abs(timeOfDay - currentInterpolated);
        // Handle day boundary (e.g., interpolated at 1439, backend at 1)
        if (drift > MINUTES_PER_DAY / 2) {
            drift = MINUTES_PER_DAY - drift;
        }

        // Check if day changed (always sync on day change)
        const dayChanged = currentDay !== currentDay_interpolated;

        // Only sync if:
        // 1. Force sync requested (from wait/sleep/major actions)
        // 2. Day changed
        // 3. Drift is significant (>1 minute)
        const DRIFT_THRESHOLD = 1; // 1 in-game minute
        if (forceSync || dayChanged || drift > DRIFT_THRESHOLD) {
            this.lastSyncTime = timeOfDay;
            this.lastSyncRealTime = performance.now();
            this.lastSyncDay = currentDay;
            this.gameTime = timeOfDay;
            this.gameDay = currentDay;

            if (forceSync || dayChanged || drift > 2) {
                // Only log significant syncs
                logger.debug(`Clock synced to backend: Day ${currentDay}, ${this.formatTime(timeOfDay)} (drift: ${drift.toFixed(1)} min)`);
            }
        }
        // Otherwise, let interpolation continue smoothly without interruption
    }

    /**
     * Animation frame loop - runs at 60fps for smooth display.
     */
    tick() {
        try {
            if (this.isPaused) {
                this.animationId = requestAnimationFrame(() => this.tick());
                return;
            }

            const now = performance.now();
            const realElapsedMs = now - this.lastSyncRealTime;
            const realElapsedSeconds = realElapsedMs / 1000;

            // Calculate interpolated game time
            const gameSecondsElapsed = realElapsedSeconds * TIME_MULTIPLIER;
            const gameMinutesElapsed = gameSecondsElapsed / 60;

            let newTime = this.lastSyncTime + gameMinutesElapsed;
            let newDay = this.lastSyncDay;

            // Handle day rollover
            while (newTime >= MINUTES_PER_DAY) {
                newTime -= MINUTES_PER_DAY;
                newDay++;
            }

            this.gameTime = newTime;
            this.gameDay = newDay;
            this.updateDisplay();
        } catch (error) {
            logger.error('SmoothClock tick error:', error);
        }

        // Always schedule next frame, even if there was an error
        this.animationId = requestAnimationFrame(() => this.tick());
    }

    /**
     * Update DOM clock display (surgical update).
     * Only updates if the displayed value would change.
     */
    updateDisplay() {
        // Don't display until we have the initial sync from backend
        // This prevents showing the default 12:00 PM before real data loads
        if (!this.hasInitialSync) {
            return;
        }

        const timeStr = this.formatTimeAMPM(this.gameTime);
        const dayStr = `Day ${this.gameDay}`;
        const currentHour = Math.floor(this.gameTime / 60) % 24;

        // Re-fetch DOM elements if not cached or if stale (detached from DOM)
        if (!this.clockEl || !this.clockEl.isConnected) {
            this.clockEl = document.getElementById('time-of-day-text-stats');
            if (this.clockEl) {
                this.lastDisplayedTime = ''; // Force update on new element
            }
        }
        if (!this.dayEl || !this.dayEl.isConnected) {
            this.dayEl = document.getElementById('day-counter');
            if (this.dayEl) {
                this.lastDisplayedDay = ''; // Force update on new element
            }
        }
        if (!this.timeImageEl || !this.timeImageEl.isConnected) {
            this.timeImageEl = document.getElementById('time-of-day-image');
            if (this.timeImageEl) {
                this.lastDisplayedHour = -1; // Force update on new element
            }
        }

        // Only update if changed (avoid unnecessary DOM writes)
        if (this.clockEl && timeStr !== this.lastDisplayedTime) {
            this.clockEl.textContent = timeStr;
            this.lastDisplayedTime = timeStr;
        }

        if (this.dayEl && dayStr !== this.lastDisplayedDay) {
            this.dayEl.textContent = dayStr;
            this.lastDisplayedDay = dayStr;
        }

        // Update time-of-day image when 2-hour period changes
        const imageIndex = Math.floor(currentHour / 2);
        if (this.timeImageEl && imageIndex !== this.lastDisplayedHour) {
            const imageName = this.timeImages[imageIndex] || this.timeImages[6]; // Default to noon
            const newSrc = `/res/img/time/${imageName}`;
            // Only update if src actually changed (check pathname to avoid full URL comparison)
            if (!this.timeImageEl.src.endsWith(imageName)) {
                this.timeImageEl.src = newSrc;
                this.timeImageEl.alt = `Time: ${timeStr}`;
            }
            this.lastDisplayedHour = imageIndex;
        }
    }

    /**
     * Format time as HH:MM string (24-hour format)
     * @param {number} timeInMinutes - Time in minutes (0-1439)
     * @returns {string} Formatted time string
     */
    formatTime(timeInMinutes) {
        const hours = Math.floor(timeInMinutes / 60) % 24;
        const minutes = Math.floor(timeInMinutes % 60);
        return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}`;
    }

    /**
     * Format time in AM/PM format (matches timeDisplay.js)
     * @param {number} timeInMinutes - Time in minutes (0-1439)
     * @returns {string} Formatted time string
     */
    formatTimeAMPM(timeInMinutes) {
        const hour = Math.floor(timeInMinutes / 60) % 24;
        const minute = Math.floor(timeInMinutes % 60);
        const minuteStr = minute.toString().padStart(2, '0');

        if (hour === 0) return `12:${minuteStr} AM`;
        if (hour < 12) return `${hour}:${minuteStr} AM`;
        if (hour === 12) return `12:${minuteStr} PM`;
        return `${hour - 12}:${minuteStr} PM`;
    }

    /**
     * Pause the clock (stops interpolation)
     */
    pause() {
        this.isPaused = true;
        // Many systems pause the clock directly (combat, POI/encounter overlays,
        // auto-pause) — announce it so the play/pause button can follow.
        eventBus.emit('clock:pauseChanged', true);
        logger.debug('Clock paused');
    }

    /**
     * Unpause the clock and re-anchor to current time
     */
    unpause() {
        // Re-anchor to current interpolated time so we don't jump
        this.lastSyncTime = this.gameTime;
        this.lastSyncRealTime = performance.now();
        this.lastSyncDay = this.gameDay;
        this.isPaused = false;
        eventBus.emit('clock:pauseChanged', false);
        logger.debug('Clock unpaused');
    }

    /**
     * Start the animation loop
     */
    start() {
        if (this.animationId) {
            return; // Already running
        }
        this.lastSyncRealTime = performance.now();
        this.tick();
        logger.info('SmoothClock animation started');
    }

    /**
     * Stop the animation loop
     */
    stop() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        logger.info('SmoothClock animation stopped');
    }

    /**
     * Get current interpolated time (for UI queries)
     * @returns {{timeOfDay: number, currentDay: number, hours: number, minutes: number, formatted: string, synced: boolean}}
     */
    getCurrentTime() {
        return {
            timeOfDay: Math.floor(this.gameTime),
            currentDay: this.gameDay,
            hours: Math.floor(this.gameTime / 60) % 24,
            minutes: Math.floor(this.gameTime % 60),
            formatted: this.formatTime(this.gameTime),
            synced: this.hasInitialSync
        };
    }

    /**
     * Get current hour (0-23) for schedule checks
     * @returns {number}
     */
    getCurrentHour() {
        return Math.floor(this.gameTime / 60) % 24;
    }

    /**
     * Check if currently paused
     * @returns {boolean}
     */
    isPausedState() {
        return this.isPaused;
    }

    /**
     * Toggle play/pause state
     */
    togglePause() {
        if (this.isPaused) {
            this.unpause();
        } else {
            this.pause();
        }
        return this.isPaused;
    }

    /**
     * Clear DOM element cache (call when DOM is rebuilt)
     */
    clearCache() {
        this.clockEl = null;
        this.dayEl = null;
        this.timeImageEl = null;
        this.lastDisplayedTime = '';
        this.lastDisplayedDay = '';
        this.lastDisplayedHour = -1;
        // Don't reset hasInitialSync - keep the time data even if DOM changes
    }

    /**
     * Reset clock state (call when loading a new game/save)
     */
    resetForNewGame() {
        this.hasInitialSync = false;
        this.gameTime = 720;
        this.gameDay = 1;
        this.lastSyncTime = 720;
        this.lastSyncDay = 1;
        this.clearCache();
        logger.debug('SmoothClock reset for new game');
    }
}

// Create singleton instance
export const smoothClock = new SmoothClock();

// Export for global access (onclick in HTML)
window.smoothClock = {
    togglePause: () => smoothClock.togglePause(),
    pause: () => smoothClock.pause(),
    unpause: () => smoothClock.unpause(),
    getCurrentTime: () => smoothClock.getCurrentTime(),
    isPaused: () => smoothClock.isPausedState(),
    resetForNewGame: () => smoothClock.resetForNewGame()
};

logger.debug('SmoothClock module loaded');
