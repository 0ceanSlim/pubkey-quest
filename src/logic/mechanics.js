/**
 * Game Mechanics Logic Module
 *
 * Pure game logic functions for movement, item usage, and gameplay calculations.
 * These functions contain no state management - they receive state as parameters.
 *
 * @module logic/mechanics
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getLocationById } from '../state/staticData.js';
import { showMessage } from '../ui/messaging.js';
import { updateAllDisplays } from '../ui/displayCoordinator.js';

/**
 * Move to a new location
 * @param {string} locationId - Full location ID (e.g., "village-west-center")
 */
export async function moveToLocation(locationId) {
    logger.info('Moving to location:', locationId);

    const state = getGameStateSync();
    const locationData = getLocationById(locationId);

    if (!locationData) {
        showMessage('Unknown location: ' + locationId, 'error');
        return;
    }

    // Check if this is an environment (outside city) - block it
    if (locationData.location_type === 'environment') {
        showTravelDisabledPopup(locationId, showMessage);
        return;
    }

    // Parse the locationId to extract city and district
    // Format: "city-id-districtKey" (e.g., "village-west-center")
    const { cityId, districtKey } = parseCityAndDistrict(locationId);

    logger.debug('Parsed location:', { locationId, cityId, districtKey });

    // Check if Game API is initialized
    if (!gameAPI.initialized) {
        logger.error('Game API not initialized');
        showMessage('Game not initialized', 'error');
        return;
    }

    try {
        // Send move action to Go backend (handles time, hunger, fatigue automatically)
        await gameAPI.move(cityId, districtKey, '');

        // Refresh UI from Go memory
        await refreshGameState();

        // Update all displays to show changes
        await updateAllDisplays();

        showMessage('Moved to ' + locationData.name, 'info');

    } catch (error) {
        logger.error('Failed to move:', error);
        showMessage('Failed to move: ' + error.message, 'error');
    }
}

/**
 * Parse location ID to extract city ID and district key
 * @param {string} locationId - Full location ID (e.g., "village-west-center")
 * @returns {Object} Object with cityId and districtKey
 * @example
 * parseCityAndDistrict("village-west-center")
 * // Returns: {cityId: "village-west", districtKey: "center"}
 */
export function parseCityAndDistrict(locationId) {
    // Handle simple city IDs (no district)
    if (!locationId.includes('-')) {
        return { cityId: locationId, districtKey: 'center' };
    }

    // Find the last hyphen to separate district key
    const lastHyphenIndex = locationId.lastIndexOf('-');
    const cityId = locationId.substring(0, lastHyphenIndex);
    const districtKey = locationId.substring(lastHyphenIndex + 1);

    return { cityId, districtKey };
}

/**
 * Show popup when trying to travel to disabled location
 * NOTE: This function creates DOM elements and should probably be in the UI layer
 * @param {string} locationId - Location ID
 * @param {Function} showMessage - UI message callback (optional)
 */
export function showTravelDisabledPopup(locationId, showMessage) {
    const locationData = getLocationById(locationId);
    const locationName = locationData ? locationData.name : locationId;

    // Create modal backdrop
    const backdrop = document.createElement('div');
    backdrop.id = 'travel-disabled-backdrop';
    backdrop.className = 'fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-[9999]';
    backdrop.style.fontFamily = '"Dogica", monospace';

    // Create modal content
    const modal = document.createElement('div');
    modal.className = 'bg-gray-800 rounded-lg p-6 max-w-md mx-4 relative';
    modal.style.border = '3px solid #ef4444';
    modal.style.boxShadow = '0 0 20px rgba(239, 68, 68, 0.5)';

    modal.innerHTML = `
        <h2 class="text-xl font-bold text-red-400 mb-4 text-center">🚧 Travel Not Available</h2>

        <div class="text-gray-300 space-y-3 text-sm leading-relaxed">
            <p>
                You can't travel to <span class="text-yellow-400 font-bold">${locationName}</span> yet!
            </p>

            <div class="bg-gray-900 border border-red-600 rounded p-3 text-xs">
                <p class="text-red-300 mb-2">⚠️ This feature is coming soon</p>
                <p class="text-gray-400">The game UI is still in development. Travel and exploration mechanics will be added in future updates.</p>
            </div>

            <p class="text-center text-xs text-gray-500 mt-4">
                For now, you can only view your character's stats and inventory from the intro.
            </p>
        </div>

        <div class="mt-4 text-center">
            <button
                id="travel-close-btn"
                class="px-6 py-2 bg-red-600 hover:bg-red-500 text-white font-bold rounded-lg transition-colors"
                style="font-size: 0.875rem;">
                Okay
            </button>
        </div>
    `;

    backdrop.appendChild(modal);
    document.body.appendChild(backdrop);

    // Close button handler
    document.getElementById('travel-close-btn').onclick = () => {
        backdrop.remove();
    };

    // Close on backdrop click
    backdrop.onclick = (e) => {
        if (e.target === backdrop) {
            backdrop.remove();
        }
    };
}

logger.debug('Game mechanics module loaded');
