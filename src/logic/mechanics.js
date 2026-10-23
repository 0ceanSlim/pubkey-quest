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
        const result = await gameAPI.move(cityId, districtKey, '');

        // Something was waiting for you here: the encounter overlay is already up
        // (it lives in the scene speech box), so don't rebuild the scene over it.
        if (result?.handledWorldHandoff) return;

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

    // The popup lives inside the game window, like the game's other popups, so
    // it scales with the game and always fits: on a phone held sideways a
    // page-level popup at real pixel sizes was taller than the screen, and its
    // buttons fell off the bottom.
    const host = document.getElementById('game-window') || document.body;
    const backdrop = document.createElement('div');
    backdrop.id = 'travel-confirm-backdrop';
    backdrop.style.cssText = `position:${host === document.body ? 'fixed' : 'absolute'};inset:0;z-index:70;` +
        'background:rgba(0,0,0,0.7);display:flex;align-items:center;justify-content:center;' +
        'font-family:"Dogica",monospace;';

    const modal = document.createElement('div');
    modal.style.cssText = 'width:290px;max-width:calc(100% - 16px);max-height:calc(100% - 16px);overflow-y:auto;' +
        'box-sizing:border-box;background:#1f1f1f;color:#d1d5db;font-size:7px;line-height:1.6;' +
        'border:2px solid #9e8b6b;box-shadow:0 0 10px rgba(158,139,107,0.5);';

    const diffColors = { easy: '#22c55e', moderate: '#eab308', hard: '#ef4444', very_hard: '#dc2626' };
    const diffColor = diffColors[difficulty] || '#9ca3af';
    const esc = (t) => String(t ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

    modal.innerHTML = `
        <div style="padding:3px 6px;background:linear-gradient(90deg,#78350f,#a16207);color:#fff;font-weight:bold;font-size:8px;">🧭 Travel to ${esc(destName)}?</div>
        <div style="padding:6px 8px;">
            <p style="margin:0 0 5px;color:#9ca3af;text-align:center;">${esc(environmentData.description || '')}</p>
            <div style="background:#111;border:1px solid #4a4a4a;padding:4px 6px;margin-bottom:5px;">
                <div>Route: <span style="color:#fde047;">${esc(envName)}</span></div>
                <div>Journey: <span style="color:#fff;">${esc(timeEstimate)}</span></div>
                <div>Difficulty: <span style="color:${diffColor};">${esc(difficulty.replace('_', ' '))}</span></div>
            </div>
            <p style="margin:0;color:#6b7280;text-align:center;">Hunger and fatigue will tick during travel. You can stop and rest mid-journey.</p>
            <div style="display:flex;justify-content:center;gap:6px;margin-top:7px;">
                <button id="travel-cancel-btn" style="font:inherit;font-size:8px;color:#fff;cursor:pointer;padding:3px 10px;background:#3f3f46;border:2px solid;border-color:#71717a #27272a #27272a #71717a;">Cancel</button>
                <button id="travel-confirm-btn" style="font:inherit;font-size:8px;font-weight:bold;color:#fff;cursor:pointer;padding:3px 10px;background:#a16207;border:2px solid;border-color:#facc15 #713f12 #713f12 #facc15;">Begin Journey</button>
            </div>
        </div>
    `;

    backdrop.appendChild(modal);
    host.appendChild(backdrop);

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

            // Refresh state and displays (backend has already unlocked music in save data)
            await refreshGameState();
            await updateAllDisplays();

            // Show music unlock notifications
            if (result.data?.music_unlocked?.length > 0) {
                for (const track of result.data.music_unlocked) {
                    showMessage(`🎵 Music unlocked: ${track}`, 'info');
                }
            }
        } else {
            showMessage(result.message || result.error || 'Failed to start travel', 'error');
        }
    } catch (error) {
        logger.error('Failed to start travel:', error);
        showMessage('Failed to start travel: ' + error.message, 'error');
    }
}

logger.debug('Game mechanics module loaded');
