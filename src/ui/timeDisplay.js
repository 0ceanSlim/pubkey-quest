/**
 * Time Display UI Module
 *
 * Handles the display of in-game time and day counter.
 * Shows time-of-day images and formatted time text.
 *
 * @module ui/timeDisplay
 */

import { logger } from '../lib/logger.js';
import { getGameStateSync } from '../state/gameState.js';
import { weekdayOf } from '../config/constants.js';

/**
 * Update time of day display
 */
export function updateTimeDisplay() {
    const state = getGameStateSync();

    // Check if timeClock is available and has synced to real data
    let clockData = null;
    if (window.timeClock && window.timeClock.getCurrentTime) {
        clockData = window.timeClock.getCurrentTime();
    }

    // If clock hasn't synced yet and we don't have state data, keep showing placeholder
    const hasTimeData = state.character?.time_of_day !== undefined && state.character?.time_of_day !== null;
    const clockSynced = clockData?.synced === true;
    if (!hasTimeData && !clockSynced) {
        return; // Keep showing "--:-- --" until real data is available
    }

    const currentDay = state.character?.current_day || 1;

    // Get current time including minutes from time clock (only if synced)
    let hour, minute;
    if (clockSynced) {
        hour = clockData.hour;
        minute = clockData.minute;
    } else if (hasTimeData) {
        // Fallback to state data if clock not synced
        const timeOfDay = state.character.time_of_day;
        // Handle both minute format (0-1439) and hour format (0-23)
        if (timeOfDay >= 24) {
            hour = Math.floor(timeOfDay / 60);
            minute = timeOfDay % 60;
        } else {
            hour = timeOfDay;
            minute = 0;
        }
    } else {
        return; // No valid time source
    }

    // Map time (0-23) to 12 PNG filenames (each image covers 2 hours)
    const timeImages = [
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

    // Calculate which image to use (divide by 2 since each image covers 2 hours)
    const imageIndex = Math.floor(hour / 2);

    // Update time image (only if changed to avoid reloading)
    const timeImage = document.getElementById('time-of-day-image');
    if (timeImage) {
        const imageName = timeImages[imageIndex] || timeImages[6]; // Default to noon if invalid
        const newSrc = `/res/img/time/${imageName}`;
        // Compare just the pathname to avoid full URL issues
        const currentSrc = new URL(timeImage.src, window.location.origin).pathname;
        if (currentSrc !== newSrc) {
            timeImage.src = newSrc;
        }
        timeImage.alt = `Time: ${formatTime(hour, minute)}`;
    }

    // Update time text in stats bar
    const timeTextStats = document.getElementById('time-of-day-text-stats');
    if (timeTextStats) {
        timeTextStats.textContent = formatTime(hour, minute);
    }

    // Update day counter in scene (top-right)
    const dayCounter = document.getElementById('day-counter');
    if (dayCounter) {
        dayCounter.textContent = `Day ${currentDay} · ${weekdayOf(currentDay)}`;
    }
}

/**
 * Helper function to format time in AM/PM format
 * @param {number} hour - Hour in 24-hour format (0-23)
 * @param {number} minute - Minute (0-59)
 * @returns {string} Formatted time string
 */
export function formatTime(hour, minute = 0) {
    const minuteStr = minute.toString().padStart(2, '0');

    if (hour === 0) return `12:${minuteStr} AM`;
    if (hour < 12) return `${hour}:${minuteStr} AM`;
    if (hour === 12) return `12:${minuteStr} PM`;
    return `${hour - 12}:${minuteStr} PM`;
}

logger.debug('Time display module loaded');
