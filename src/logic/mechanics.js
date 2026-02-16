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

    // Check if this is an environment - start travel
    if (locationData.location_type === 'environment') {
        showTravelConfirmation(locationId, locationData);
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
 * Show travel confirmation popup
 * @param {string} environmentId - Environment location ID
 * @param {Object} environmentData - Environment location data
 */
export function showTravelConfirmation(environmentId, environmentData) {
    const envName = environmentData.name || environmentId;

    // Get travel time from properties
    const travelTime = environmentData.properties?.travel_time || 1440;
    const days = Math.floor(travelTime / 1440);
    const hours = Math.floor((travelTime % 1440) / 60);
    let timeEstimate = '';
    if (days > 0 && hours > 0) {
        timeEstimate = `~${days} day${days > 1 ? 's' : ''} ${hours} hour${hours > 1 ? 's' : ''}`;
    } else if (days > 0) {
        timeEstimate = `~${days} day${days > 1 ? 's' : ''}`;
    } else {
        timeEstimate = `~${hours} hour${hours > 1 ? 's' : ''}`;
    }

    const difficulty = environmentData.properties?.travel_difficulty || 'unknown';

    // Determine destination city from connects[] and current location
    const state = getGameStateSync();
    const currentDistrict = `${state.location.current}-${state.location.district}`;
    const connects = environmentData.properties?.connects || environmentData.connections || [];
    const destConnectId = connects.find(c => c !== currentDistrict) || 'unknown';
    const destCityId = destConnectId.includes('-') ? destConnectId.substring(0, destConnectId.lastIndexOf('-')) : destConnectId;
    const destLocation = getLocationById(destCityId);
    const destName = destLocation ? destLocation.name : destCityId;

    // Create modal backdrop
    const backdrop = document.createElement('div');
    backdrop.id = 'travel-confirm-backdrop';
    backdrop.className = 'fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-[9999]';
    backdrop.style.fontFamily = '"Dogica", monospace';

    const modal = document.createElement('div');
    modal.className = 'bg-gray-800 rounded-lg p-6 max-w-md mx-4 relative';
    modal.style.border = '3px solid #9e8b6b';
    modal.style.boxShadow = '0 0 20px rgba(158, 139, 107, 0.5)';

    const diffColors = { easy: '#22c55e', moderate: '#eab308', hard: '#ef4444', very_hard: '#dc2626' };
    const diffColor = diffColors[difficulty] || '#9ca3af';

    modal.innerHTML = `
        <h2 class="text-lg font-bold text-yellow-400 mb-3 text-center">Travel to ${destName}?</h2>
        <div class="text-gray-300 space-y-2 text-sm leading-relaxed">
            <p class="text-center text-xs text-gray-400">${environmentData.description || ''}</p>
            <div class="bg-gray-900 border border-gray-600 rounded p-3 text-xs space-y-1">
                <div>Route: <span class="text-yellow-300">${envName}</span></div>
                <div>Journey: <span class="text-white">${timeEstimate}</span></div>
                <div>Difficulty: <span style="color: ${diffColor}">${difficulty.replace('_', ' ')}</span></div>
            </div>
            <p class="text-center text-xs text-gray-500">Hunger and fatigue will tick during travel. You can stop and rest mid-journey.</p>
        </div>
        <div class="mt-4 flex justify-center gap-3">
            <button id="travel-cancel-btn" class="px-4 py-2 bg-gray-600 hover:bg-gray-500 text-white font-bold rounded-lg transition-colors" style="font-size: 0.75rem;">Cancel</button>
            <button id="travel-confirm-btn" class="px-4 py-2 bg-yellow-600 hover:bg-yellow-500 text-white font-bold rounded-lg transition-colors" style="font-size: 0.75rem;">Begin Journey</button>
        </div>
    `;

    backdrop.appendChild(modal);
    document.body.appendChild(backdrop);

    document.getElementById('travel-cancel-btn').onclick = () => backdrop.remove();
    backdrop.onclick = (e) => { if (e.target === backdrop) backdrop.remove(); };

    document.getElementById('travel-confirm-btn').onclick = async () => {
        backdrop.remove();
        await startTravel(environmentId);
    };
}

/**
 * Start travel through an environment
 * @param {string} environmentId - Environment ID
 */
async function startTravel(environmentId) {
    logger.info('Starting travel through:', environmentId);

    try {
        const result = await gameAPI.sendAction('start_travel', {
            environment_id: environmentId
        });

        if (result.success) {
            showMessage(result.message, 'info');

            // Auto-play time when starting travel
            if (window.timeClock && window.timeClock.play) {
                window.timeClock.play();
            }

            // Refresh state and displays
            await refreshGameState();
            await updateAllDisplays();
        } else {
            showMessage(result.message || result.error || 'Failed to start travel', 'error');
        }
    } catch (error) {
        logger.error('Failed to start travel:', error);
        showMessage('Failed to start travel: ' + error.message, 'error');
    }
}

logger.debug('Game mechanics module loaded');
