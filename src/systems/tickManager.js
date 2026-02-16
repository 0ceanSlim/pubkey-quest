/**
 * TickManager - Game tick orchestration system
 *
 * Manages the 417ms tick timer that syncs game state with the backend.
 * Each tick:
 * 1. Sends current time to backend
 * 2. Backend processes effects and returns delta
 * 3. Delta is applied to DOM via deltaApplier
 * 4. SmoothClock syncs to authoritative backend time
 *
 * Tick rate: 417ms (~2.4 ticks/second)
 * This corresponds to 1 in-game minute at 144x speed:
 *   60 seconds / 144 = 0.417 seconds = 417ms
 */

import { logger } from '../lib/logger.js';
import { smoothClock } from './smoothClock.js';
import { deltaApplier } from './deltaApplier.js';
import { gameAPI } from '../lib/api.js';
import { eventBus } from '../lib/events.js';
import { effectsDisplay } from '../ui/effectsDisplay.js';
import { refreshGameState } from '../state/gameState.js';

// Constants
const TICK_INTERVAL_MS = 417; // 1 in-game minute at 144x speed

class TickManager {
    constructor() {
        this.tickIntervalId = null;
        this.isRunning = false;
        this.tickCount = 0;
        this.lastTickTime = 0;
        this.isPendingRequest = false; // Prevent overlapping requests

        logger.debug('TickManager initialized');
    }

    /**
     * Start the tick loop
     */
    start() {
        if (this.tickIntervalId) {
            logger.warn('TickManager already running');
            return;
        }

        this.isRunning = true;
        this.tickCount = 0;
        this.lastTickTime = performance.now();

        this.tickIntervalId = setInterval(() => {
            this.tick();
        }, TICK_INTERVAL_MS);

        logger.info(`TickManager started (${TICK_INTERVAL_MS}ms interval)`);
    }

    /**
     * Stop the tick loop
     */
    stop() {
        if (this.tickIntervalId) {
            clearInterval(this.tickIntervalId);
            this.tickIntervalId = null;
        }
        this.isRunning = false;
        logger.info('TickManager stopped');
    }

    /**
     * Main tick function - called every 417ms
     * Always sends update_time. Backend handles travel progress automatically.
     */
    async tick() {
        // Skip if paused
        if (smoothClock.isPausedState()) {
            return;
        }

        // Skip if a request is already pending (prevent overlapping)
        if (this.isPendingRequest) {
            logger.debug('Skipping tick - request pending');
            return;
        }

        // Skip if API not initialized
        if (!gameAPI.initialized) {
            logger.debug('Skipping tick - API not initialized');
            return;
        }

        this.tickCount++;
        const now = performance.now();
        this.lastTickTime = now;

        // Get current interpolated time from smooth clock
        const currentTime = smoothClock.getCurrentTime();

        try {
            this.isPendingRequest = true;

            // Always send update_time - backend advances travel progress automatically
            const response = await gameAPI.sendAction('update_time', {
                time_of_day: currentTime.timeOfDay,
                current_day: currentTime.currentDay
            });

            this.isPendingRequest = false;

            if (!response) {
                logger.warn('Tick: No response from backend');
                return;
            }

            if (!response.success) {
                logger.warn('Tick: Backend returned error:', response.message || response.error);
                return;
            }

            // Update local state cache from response data BEFORE applying delta
            // This ensures effectsDisplay can read updated effects when delta is applied
            // Note: Clock sync is handled by deltaApplier, not here (avoid double sync)
            const data = response.data || response;
            if (data) {
                this.updateLocalState(data);
            }

            // Apply delta if present (optimized path)
            // Delta applier handles clock sync via charDelta.time_of_day
            if (response.delta) {
                logger.debug('Tick delta keys:', Object.keys(response.delta));
                if (response.delta.npcs) {
                    logger.debug('NPC delta received:', response.delta.npcs);
                }
                if (response.delta.show_ready) {
                    logger.info('Show ready delta received:', response.delta.show_ready);
                }
                deltaApplier.applyDelta(response.delta);
            }

            // Handle travel data from response (backend includes travel_progress when in environment)
            if (data && data.travel_progress !== undefined) {
                try {
                    const { updateTravelProgress } = await import('../ui/locationDisplay.js');
                    updateTravelProgress(data.travel_progress);
                } catch (e) {
                    // locationDisplay not loaded yet
                }
            }

            // Handle arrival at destination
            if (data && data.arrived) {
                logger.info('Arrived at destination:', data.dest_city_name);

                // Sync clock after arrival
                if (data.time_of_day !== undefined) {
                    smoothClock.syncFromBackend(data.time_of_day, data.current_day || 1, true);
                }

                // Refresh full state and UI
                await refreshGameState();
                const { updateAllDisplays } = await import('../ui/displayCoordinator.js');
                await updateAllDisplays();

                // Show arrival message
                if (response.message) {
                    eventBus.emit('notification:show', {
                        message: response.message,
                        color: data.newly_discovered ? 'green' : 'yellow',
                        duration: 5000
                    });
                }

                // Handle music unlocks
                if (data.music_unlocked && data.music_unlocked.length > 0) {
                    for (const track of data.music_unlocked) {
                        eventBus.emit('notification:show', {
                            message: `Music unlocked: ${track}`,
                            color: 'blue',
                            duration: 3000
                        });
                    }
                }
            }

            if (data) {

                // Check for auto-pause (6+ in-game hours of idle)
                if (data.auto_pause) {
                    logger.info('Auto-pause triggered by backend');
                    smoothClock.pause();

                    // Show notification to user
                    eventBus.emit('notification:show', {
                        message: 'Game auto-paused after 6 hours of idle time.',
                        color: 'yellow',
                        duration: 5000
                    });
                }
            }

            // Emit tick completed event
            eventBus.emit('tick:completed', {
                tickCount: this.tickCount,
                delta: response.delta,
                data: response.data
            });

        } catch (error) {
            this.isPendingRequest = false;
            logger.error('Tick failed:', error);
            console.error('Tick error details:', error);
        }
    }

    /**
     * Update local state from backend response
     * This ensures the cached game state stays in sync
     * @param {object} data - Backend response data
     */
    updateLocalState(data) {
        // Use enriched_effects if available (has name, category, stat_modifiers)
        // Falls back to active_effects for backward compatibility
        const effects = data.enriched_effects || data.active_effects;

        // Emit state update event for any system that caches state
        if (data.fatigue !== undefined || data.hunger !== undefined || data.hp !== undefined) {
            eventBus.emit('character:statsUpdated', {
                fatigue: data.fatigue,
                hunger: data.hunger,
                hp: data.hp,
                active_effects: effects
            });
        }

        // Always update radial progress indicators on every tick
        // This ensures the accumulator progress is shown even when levels don't change
        if (effects) {
            // Render active effects display (shows/hides effects like performance-high, tired, etc.)
            effectsDisplay.renderEffects(effects);

            // Update fatigue/hunger radial progress
            const accumulators = effectsDisplay.getAccumulatorValues(effects);
            effectsDisplay.updateFatigueIcon(data.fatigue, accumulators.fatigueAccumulator, accumulators.fatigueInterval);
            effectsDisplay.updateHungerIcon(data.hunger, accumulators.hungerAccumulator, accumulators.hungerInterval);
        }
    }

    /**
     * Force an immediate tick (used after actions that affect time)
     */
    async forceTick() {
        if (this.isPendingRequest) {
            logger.debug('Force tick skipped - request pending');
            return;
        }
        await this.tick();
    }

    /**
     * Get tick statistics
     * @returns {{tickCount: number, isRunning: boolean, intervalMs: number}}
     */
    getStats() {
        return {
            tickCount: this.tickCount,
            isRunning: this.isRunning,
            intervalMs: TICK_INTERVAL_MS
        };
    }

    /**
     * Pause ticking (delegates to smoothClock)
     */
    pause() {
        smoothClock.pause();
        logger.debug('Ticking paused via smoothClock');
    }

    /**
     * Resume ticking (delegates to smoothClock)
     */
    resume() {
        smoothClock.unpause();
        logger.debug('Ticking resumed via smoothClock');
    }

    /**
     * Toggle pause state
     * @returns {boolean} New pause state
     */
    togglePause() {
        return smoothClock.togglePause();
    }
}

// Create singleton instance
export const tickManager = new TickManager();

// Export for global access
window.tickManager = {
    start: () => tickManager.start(),
    stop: () => tickManager.stop(),
    pause: () => tickManager.pause(),
    resume: () => tickManager.resume(),
    togglePause: () => tickManager.togglePause(),
    forceTick: () => tickManager.forceTick(),
    getStats: () => tickManager.getStats()
};

logger.debug('TickManager module loaded');
