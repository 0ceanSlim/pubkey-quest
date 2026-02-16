/**
 * Location Display UI Module
 *
 * Handles location display, navigation, music, buildings, NPCs, and vault UI.
 * Provides the main location scene interface with district navigation.
 *
 * @module ui/locationDisplay
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getLocationById, getNPCById } from '../state/staticData.js';
import { updateTimeDisplay, formatTime } from './timeDisplay.js';
import { showActionText, showMessage } from './messaging.js';
import { moveToLocation } from '../logic/mechanics.js';
import { updateAllDisplays } from './displayCoordinator.js';
import { eventBus } from '../lib/events.js';

// Module-level state
let lastDisplayedLocation = null;
let lastBuildingState = null;
let autoUpdateInitialized = false;
let isRendering = false; // Prevent duplicate renders
let renderTimeout = null; // For debouncing

/**
 * Fetch NPCs at current location from backend
 * @param {string} location - Location ID (e.g., "kingdom")
 * @param {string} district - District key (e.g., "center")
 * @param {string} building - Building ID (optional, empty string if not in building)
 * @returns {Promise<string[]>} Array of NPC IDs at this location
 */
async function fetchNPCsAtLocation(location, district, building = '') {
    try {
        const state = getGameStateSync();
        const timeOfDay = state.character?.time_of_day !== undefined ? state.character.time_of_day : 720;
        const districtId = `${location}-${district}`;

        const params = new URLSearchParams({
            location: location,
            district: districtId,
            time: timeOfDay.toString()
        });

        if (building) {
            params.append('building', building);
        }

        const response = await fetch(`/api/npcs/at-location?${params.toString()}`);
        if (!response.ok) {
            logger.error('Failed to fetch NPCs:', response.statusText);
            return [];
        }

        const data = await response.json();
        logger.debug('Fetched NPCs at location:', data);

        // Extract NPC IDs from response
        return data.map(npc => npc.npc_id);
    } catch (error) {
        logger.error('Error fetching NPCs at location:', error);
        return [];
    }
}

/**
 * Display current location with navigation, buildings, and NPCs
 * Main location rendering function
 */
export async function displayCurrentLocation() {
    // Prevent duplicate renders - if already rendering, skip
    if (isRendering) {
        logger.debug('Location display already rendering, skipping duplicate call');
        return;
    }

    isRendering = true;

    try {
        // Initialize auto-update listeners on first call
        if (!autoUpdateInitialized) {
            initializeAutoUpdate();
            autoUpdateInitialized = true;
        }

        const state = getGameStateSync();
        const cityId = state.location?.current;
        const districtKey = state.location?.district || 'center';

        if (!cityId) return;

        // Check if player is in an environment (traveling)
        const currentLocData = getLocationById(cityId);
        if (currentLocData && currentLocData.location_type === 'environment') {
            displayTravelView(state, currentLocData);
            return;
        }

        // Restore normal grid layout if coming back from travel view
        restoreActionButtonsLayout();

        // Check if player is in a building that just closed and eject them
        await checkAndEjectFromClosedBuilding();

    // Construct full district ID from city + district (e.g., "village-west-east")
    const districtId = `${cityId}-${districtKey}`;
    const currentLocationId = districtId;  // For compatibility with rest of function

    logger.debug('Display location:', { cityId, districtKey, districtId });

    // Get the city data (for image, music)
    const cityData = getLocationById(cityId);
    if (!cityData) {
        logger.error('City not found:', cityId);
        return;
    }

    // Get the district data (for description, buildings, connections)
    const locationData = getLocationById(districtId);
    if (!locationData) {
        logger.error('District not found:', districtId);
        return;
    }

    logger.debug('City data:', cityData);
    logger.debug('District data:', locationData);

    // Update scene image (use city's image for all districts)
    const sceneImage = document.getElementById('scene-image');
    if (sceneImage && cityData.image) {
        sceneImage.src = cityData.image;
        sceneImage.alt = cityData.name;
    }

    // Update music (use city's music for all districts)
    if (cityData.music && window.musicSystem) {
        window.musicSystem.playLocationMusic();
    }

    // Update city name (top of scene)
    const cityName = document.getElementById('city-name');
    if (cityName) {
        cityName.textContent = cityData.name;
    }

    // Update district name (bottom of scene)
    const districtName = document.getElementById('district-name');
    if (districtName) {
        districtName.textContent = locationData.name;
    }

    // Update time of day display
    updateTimeDisplay();

    // Show location description in action text (white color) - only when:
    // 1. District changes (moving between districts)
    // 2. Exiting a building (going from building to outdoors)
    // NOT when entering a building
    const currentBuildingId = state.location?.building || '';
    const districtOnlyKey = `${cityId}-${districtKey}`;
    const wasInBuilding = lastBuildingState !== null && lastBuildingState !== '';
    const isInBuilding = currentBuildingId !== '';
    const exitedBuilding = wasInBuilding && !isInBuilding;
    const districtChanged = lastDisplayedLocation !== districtOnlyKey;

    if (locationData.description && (districtChanged || exitedBuilding)) {
        showActionText(locationData.description, 'white');
        lastDisplayedLocation = districtOnlyKey;
    }

    // Track building state for next time
    lastBuildingState = currentBuildingId;

    // Generate location actions based on city district structure
    const navContainer = document.getElementById('navigation-buttons');
    const buildingContainer = document.getElementById('building-buttons');
    const npcContainer = document.getElementById('npc-buttons');

    // NOTE: We no longer clear containers here - instead we build fragments and replace atomically below
    // This reduces flicker by minimizing the time containers are empty

    // Get district data from location
    let districtData = null;
    if (locationData.properties?.districts) {
        // Find the current district
        for (const district of Object.values(locationData.properties.districts)) {
            if (district.id === currentLocationId) {
                districtData = district;
                break;
            }
        }
    }

    // If we have district data, use it; otherwise fall back to location data
    const currentData = districtData || locationData;
    logger.debug('Current data for buttons:', currentData);

    // Get connections - check both direct and properties.connections
    const connections = currentData.connections || currentData.properties?.connections;
    let buildings = currentData.buildings || currentData.properties?.buildings;

    // Fetch NPCs from backend based on current location and time
    let npcs = [];
    await fetchNPCsAtLocation(cityId, districtKey, currentBuildingId).then(npcData => {
        npcs = npcData || [];
    });

    // Check if we're inside a building
    if (currentBuildingId && buildings) {
        // Find the current building data
        const currentBuilding = buildings.find(b => b.id === currentBuildingId);

        if (currentBuilding) {
            logger.debug('Inside building:', currentBuilding);

            // Override buildings to show only "Exit Building" button
            buildings = [{ id: '__exit__', name: 'Exit Building', isExit: true }];
        }
    }

    // 1. NAVIGATION BUTTONS (D-pad style with cardinal directions)
    logger.debug('Navigation connections:', connections);
    if (connections) {
        // Clear all D-pad slots first
        ['travel-n', 'travel-s', 'travel-e', 'travel-w', 'travel-center'].forEach(slotId => {
            const slot = document.getElementById(slotId);
            if (slot) slot.innerHTML = '';
        });

        Object.entries(connections).forEach(([direction, connectionId]) => {
            logger.debug(`Processing connection: ${direction} -> ${connectionId}`);
            const connectedLocation = getLocationById(connectionId);
            logger.debug(`Found location:`, connectedLocation);

            if (connectedLocation) {
                // Map direction to D-pad slot
                const slotMap = {
                    'north': 'travel-n',
                    'south': 'travel-s',
                    'east': 'travel-e',
                    'west': 'travel-w',
                    'center': 'travel-center'
                };

                const slotId = slotMap[direction.toLowerCase()];
                const slot = document.getElementById(slotId);

                if (slot) {
                    // Determine button type based on location_type
                    const buttonType = connectedLocation.location_type === 'environment' ? 'environment' : 'navigation';

                    const button = createLocationButton(
                        connectedLocation.name || direction.toUpperCase(),
                        () => moveToLocation(connectionId),
                        buttonType
                    );
                    button.className += ' w-full h-full';
                    slot.appendChild(button);
                } else {
                    logger.warn(`No slot found for ${direction} (${slotId})`);
                }
            } else {
                logger.warn(`No location found for ${direction} -> ${connectionId}`);
            }
        });
    } else {
        logger.debug('No connections found for this location');
    }

    // 2. BUILDING BUTTONS - Build in fragment first, then replace atomically to reduce flicker
    if (buildingContainer) {
        const buildingButtonContainer = buildingContainer.querySelector('div');
        const buildingFragment = document.createDocumentFragment();

        if (buildings && buildings.length > 0) {
            // Get current time of day from game state in minutes (0-1439)
            const timeInMinutes = state.character?.time_of_day !== undefined ? state.character.time_of_day : 720;

            buildings.forEach(building => {
                // Check if this is the special "Exit Building" button
                if (building.isExit) {
                    const button = createLocationButton(
                        building.name,
                        () => exitBuilding(),
                        'building'  // Use green for exit button
                    );
                    buildingFragment.appendChild(button);

                    // Check if player has a rented room here - add Sleep button
                    const hasRentedRoom = checkIfRoomRented(currentBuildingId);
                    if (hasRentedRoom) {
                        const sleepButton = createLocationButton(
                            'Sleep',
                            () => sleepInRoom(),
                            'action'  // Special color for action
                        );
                        buildingFragment.appendChild(sleepButton);
                    }

                    // Check if player has a booked show at the right time - add Play Show button
                    const hasShow = checkIfShowReady(currentBuildingId, timeInMinutes);
                    if (hasShow) {
                        const showButton = createLocationButton(
                            'Play Show',
                            () => performShow(),
                            'action'  // Special color for action
                        );
                        buildingFragment.appendChild(showButton);
                    }

                    return;
                }

                // Check if building is currently open (pass time in minutes)
                const isOpen = isBuildingOpen(building, timeInMinutes);

                if (isOpen) {
                    // Open building - normal styling
                    const button = createLocationButton(
                        building.name,
                        () => enterBuilding(building.id),
                        'building'
                    );
                    button.dataset.buildingId = building.id; // For delta applier
                    buildingFragment.appendChild(button);
                } else {
                    // Closed building - grey styling with different click handler
                    const button = createLocationButton(
                        building.name,
                        () => showBuildingClosedMessage(building),
                        'building-closed'
                    );
                    button.dataset.buildingId = building.id; // For delta applier
                    button.disabled = true;
                    button.classList.add('opacity-50', 'cursor-not-allowed');
                    buildingFragment.appendChild(button);
                }
            });
        } else {
            // No buildings in this district
            const emptyMessage = document.createElement('div');
            emptyMessage.className = 'text-gray-400 text-xs p-2 text-center italic';
            emptyMessage.textContent = 'No buildings here.';
            buildingFragment.appendChild(emptyMessage);
        }

        // Atomic replace - minimizes flicker
        buildingButtonContainer.replaceChildren(buildingFragment);
    }

    // 3. NPC BUTTONS - Build in fragment first, then replace atomically to reduce flicker
    if (npcContainer) {
        const npcButtonContainer = npcContainer.querySelector('div');
        const npcFragment = document.createDocumentFragment();

        if (npcs && npcs.length > 0) {
            npcs.forEach(npcId => {
                const npcData = getNPCById(npcId);
                const displayName = npcData ? npcData.name : npcId.replace(/_/g, ' ');
                const button = createLocationButton(
                    displayName,
                    () => talkToNPC(npcId),
                    'npc'
                );
                button.dataset.npcId = npcId; // For delta applier
                npcFragment.appendChild(button);
            });
        } else {
            // Show message when no NPCs in district (they're all in buildings)
            const emptyMessage = document.createElement('div');
            emptyMessage.className = 'empty-message text-gray-400 text-xs p-2 text-center italic';
            emptyMessage.textContent = 'No one here. Check buildings.';
            npcFragment.appendChild(emptyMessage);
        }

        // Atomic replace - minimizes flicker
        npcButtonContainer.replaceChildren(npcFragment);
    }
    } finally {
        // Always reset rendering flag
        isRendering = false;
    }
}

// ============================================================================
// TRAVEL VIEW
// ============================================================================

/**
 * Display travel view when player is in an environment
 * Shows full-width progress bar, environment info, and travel controls
 * @param {Object} state - Current game state
 * @param {Object} envData - Environment location data
 */
function displayTravelView(state, envData) {
    const envName = envData.name || state.location.current;
    const envDescription = envData.description || '';
    const travelProgress = state.location?.travel_progress || 0;
    const travelTime = envData.properties?.travel_time || 1440;
    const connects = envData.properties?.connects || envData.connections || [];

    // Parse origin/destination from connects and district
    const originDistrict = state.location?.raw_district || '';
    let originCity = '', destCity = '', originName = '', destName = '';

    for (const connectId of connects) {
        const lastHyphen = connectId.lastIndexOf('-');
        const city = lastHyphen > -1 ? connectId.substring(0, lastHyphen) : connectId;
        const cityData = getLocationById(city);
        const name = cityData ? cityData.name : city;

        if (connectId === originDistrict) {
            originCity = city;
            originName = name;
        } else {
            destCity = city;
            destName = name;
        }
    }

    // Store travel metadata for updateTravelProgress to use
    _travelViewMeta = { travelTime, originName, destName };

    const progressPct = Math.min(Math.floor(travelProgress * 100), 100);
    const timeRemainingStr = formatTravelTimeRemaining(travelTime, travelProgress);

    // Update scene image (use environment type or generic)
    const sceneImage = document.getElementById('scene-image');
    if (sceneImage) {
        sceneImage.src = envData.image || '/res/img/locations/travel.png';
        sceneImage.alt = envName;
    }

    // Update city name to show environment
    const cityNameEl = document.getElementById('city-name');
    if (cityNameEl) {
        cityNameEl.textContent = envName;
    }

    // Update district name to show progress
    const districtNameEl = document.getElementById('district-name');
    if (districtNameEl) {
        districtNameEl.textContent = `${originName} → ${destName}`;
    }

    // Update time display
    updateTimeDisplay();

    // Show environment description
    if (envDescription && lastDisplayedLocation !== state.location.current) {
        showActionText(envDescription, 'white');
        lastDisplayedLocation = state.location.current;
    }

    const isStopped = state.location?.travel_stopped || false;

    // Replace the entire action-buttons grid with a travel-specific layout
    const actionButtons = document.getElementById('action-buttons');
    if (!actionButtons) return;

    // Switch from 3-column grid to single-column travel layout
    actionButtons.innerHTML = '';
    actionButtons.style.display = 'flex';
    actionButtons.style.flexDirection = 'column';
    actionButtons.style.gap = '4px';
    actionButtons.style.padding = '4px';

    // -- Full-width progress bar section --
    const progressSection = document.createElement('div');
    progressSection.style.cssText = 'width: 100%; display: flex; flex-direction: column; gap: 2px;';

    // Origin → Destination labels
    const labels = document.createElement('div');
    labels.style.cssText = 'display: flex; justify-content: space-between; width: 100%; font-size: 7px; color: #9ca3af;';
    labels.innerHTML = `<span>${originName}</span><span>${destName}</span>`;
    progressSection.appendChild(labels);

    // Progress bar container
    const barContainer = document.createElement('div');
    barContainer.style.cssText = 'width: 100%; height: 10px; background: #1a1a1a; border: 1px solid #4a4a4a; position: relative;';

    const barFill = document.createElement('div');
    barFill.id = 'travel-progress-bar';
    barFill.style.cssText = `width: ${progressPct}%; height: 100%; background: linear-gradient(90deg, #16a34a, #22c55e); transition: width 0.4s ease;`;
    barContainer.appendChild(barFill);
    progressSection.appendChild(barContainer);

    // Progress text (percentage and time remaining)
    const progressText = document.createElement('div');
    progressText.id = 'travel-progress-text';
    progressText.style.cssText = 'font-size: 8px; color: #fbbf24; text-align: center;';
    progressText.textContent = `${progressPct}% — ${timeRemainingStr} remaining`;
    progressSection.appendChild(progressText);

    actionButtons.appendChild(progressSection);

    // -- Full-width travel control buttons --
    const controlsSection = document.createElement('div');
    controlsSection.style.cssText = 'width: 100%; display: flex; gap: 4px; flex: 1;';

    if (isStopped) {
        const continueBtn = createLocationButton('Continue Journey', () => resumeTravel(), 'building');
        continueBtn.style.flex = '1';
        continueBtn.style.fontSize = '9px';
        controlsSection.appendChild(continueBtn);
        const turnBackBtn = createLocationButton('Turn Back', () => turnBack(), 'environment');
        turnBackBtn.style.flex = '1';
        turnBackBtn.style.fontSize = '9px';
        controlsSection.appendChild(turnBackBtn);
    } else {
        const stopBtn = createLocationButton('Stop & Rest', () => stopTravel(), 'environment');
        stopBtn.style.flex = '1';
        stopBtn.style.fontSize = '9px';
        controlsSection.appendChild(stopBtn);
    }

    actionButtons.appendChild(controlsSection);

    // -- Status message --
    const statusMsg = document.createElement('div');
    statusMsg.style.cssText = 'text-align: center; color: #9ca3af; font-size: 8px; font-style: italic;';
    statusMsg.textContent = isStopped ? 'Resting in the wilderness.' : 'Traveling...';
    actionButtons.appendChild(statusMsg);
}

/**
 * Format travel time remaining as a human-readable string
 * @param {number} travelTime - Total travel time in minutes
 * @param {number} progress - Current progress 0.0-1.0
 * @returns {string} Formatted time string
 */
function formatTravelTimeRemaining(travelTime, progress) {
    const minutesRemaining = Math.ceil(travelTime * (1.0 - progress));
    const daysLeft = Math.floor(minutesRemaining / 1440);
    const hoursLeft = Math.floor((minutesRemaining % 1440) / 60);
    const minsLeft = minutesRemaining % 60;
    let timeRemaining = '';
    if (daysLeft > 0) timeRemaining += `${daysLeft}d `;
    if (hoursLeft > 0 || daysLeft > 0) timeRemaining += `${hoursLeft}h `;
    timeRemaining += `${minsLeft}m`;
    return timeRemaining;
}

// Module-level metadata for travel view updates (set by displayTravelView, read by updateTravelProgress)
let _travelViewMeta = null;

/**
 * Restore action-buttons to normal 3-column grid layout after leaving travel view.
 * Travel view replaces the innerHTML and changes display to flex, so we need to
 * rebuild the original structure when returning to a city.
 */
function restoreActionButtonsLayout() {
    const actionButtons = document.getElementById('action-buttons');
    if (!actionButtons) return;

    // Check if we need to restore (travel view sets display to flex)
    if (actionButtons.style.display === 'flex') {
        // Reset to original grid layout
        actionButtons.style.display = '';
        actionButtons.style.flexDirection = '';
        actionButtons.style.gap = '';
        actionButtons.style.padding = '';
        actionButtons.className = 'grid grid-cols-3 gap-1 h-full';

        // Rebuild the original column structure
        actionButtons.innerHTML = `
            <div id="navigation-buttons" class="flex flex-col gap-0.5 min-w-0 overflow-hidden">
                <h3 class="text-white text-[8px] font-bold uppercase mb-0.5 flex-shrink-0">Travel</h3>
                <div class="grid grid-cols-3 grid-rows-3 gap-0.5 flex-1 min-h-0">
                    <div></div>
                    <div id="travel-n" class="flex items-center justify-center min-w-0 min-h-0"></div>
                    <div></div>
                    <div id="travel-w" class="flex items-center justify-center min-w-0 min-h-0"></div>
                    <div id="travel-center" class="flex items-center justify-center min-w-0 min-h-0"></div>
                    <div id="travel-e" class="flex items-center justify-center min-w-0 min-h-0"></div>
                    <div></div>
                    <div id="travel-s" class="flex items-center justify-center min-w-0 min-h-0"></div>
                    <div></div>
                </div>
            </div>
            <div id="building-buttons" class="flex flex-col gap-0.5">
                <h3 class="text-white text-[8px] font-bold uppercase mb-0.5">Buildings</h3>
                <div class="flex flex-col gap-0.5 overflow-y-auto"></div>
            </div>
            <div id="npc-buttons" class="flex flex-col gap-0.5">
                <h3 class="text-white text-[8px] font-bold uppercase mb-0.5">People</h3>
                <div class="flex flex-col gap-0.5 overflow-y-auto"></div>
            </div>
        `;

        // Clear travel metadata
        _travelViewMeta = null;
        logger.debug('Restored action-buttons to normal grid layout');
    }
}

/**
 * Update travel progress bar and text (called from tick manager on each tick)
 * @param {number} progress - 0.0 to 1.0
 */
export function updateTravelProgress(progress) {
    const bar = document.getElementById('travel-progress-bar');
    if (bar) {
        const pct = Math.min(Math.floor(progress * 100), 100);
        bar.style.width = `${pct}%`;
    }

    // Also update the progress text (percentage and time remaining)
    const textEl = document.getElementById('travel-progress-text');
    if (textEl && _travelViewMeta) {
        const pct = Math.min(Math.floor(progress * 100), 100);
        const timeStr = formatTravelTimeRemaining(_travelViewMeta.travelTime, progress);
        textEl.textContent = `${pct}% — ${timeStr} remaining`;
    }
}

/**
 * Stop travel (player stops moving, time keeps flowing)
 */
async function stopTravel() {
    logger.info('Stopping travel');

    try {
        const result = await gameAPI.sendAction('stop_travel', {});
        if (result.success) {
            showMessage(result.message, 'info');
            await refreshGameState();
            await displayCurrentLocation();
        } else {
            showMessage(result.message || 'Failed to stop', 'error');
        }
    } catch (error) {
        logger.error('Failed to stop travel:', error);
    }
}

/**
 * Resume travel (player starts moving again)
 */
async function resumeTravel() {
    logger.info('Resuming travel');

    try {
        const result = await gameAPI.sendAction('resume_travel', {});
        if (result.success) {
            showMessage(result.message, 'info');
            await refreshGameState();
            await displayCurrentLocation();
        } else {
            showMessage(result.message || 'Failed to resume travel', 'error');
        }
    } catch (error) {
        logger.error('Failed to resume travel:', error);
    }
}

/**
 * Turn back during travel (reverse direction)
 */
async function turnBack() {
    logger.info('Turning back during travel');

    try {
        const result = await gameAPI.sendAction('turn_back', {});
        if (result.success) {
            showMessage(result.message, 'info');
            await refreshGameState();
            await displayCurrentLocation();
        } else {
            showMessage(result.message || 'Failed to turn back', 'error');
        }
    } catch (error) {
        logger.error('Failed to turn back:', error);
    }
}

/**
 * Create a location button with consistent styling
 * @param {string} text - Button text
 * @param {Function} onClick - Click handler
 * @param {string} type - Button type: 'navigation', 'environment', 'building', 'building-closed', 'npc'
 * @returns {HTMLButtonElement} Styled button element
 */
export function createLocationButton(text, onClick, type = 'navigation') {
    const button = document.createElement('button');

    // Different muted colors for different types (Win95-style)
    const typeStyles = {
        navigation: '#6b7a9e',      // City districts - muted blue
        environment: '#9e6b6b',     // Outside city - muted red
        building: '#6b8e6b',        // Open buildings - muted green
        'building-closed': '#808080', // Closed buildings - grey
        npc: '#8b6b9e',             // NPCs - muted purple
        action: '#9e8b6b'           // Special actions (sleep, play show) - muted gold
    };

    const bgColor = typeStyles[type] || typeStyles.navigation;
    const textColor = type === 'building-closed' ? '#000000' : '#ffffff';

    button.className = 'text-white transition-all leading-tight text-center overflow-hidden';
    button.style.fontSize = '7px';
    button.style.background = bgColor;
    button.style.color = textColor;
    button.style.cursor = 'pointer';
    button.style.padding = '2px 4px';
    button.style.borderTop = '1px solid #ffffff';
    button.style.borderLeft = '1px solid #ffffff';
    button.style.borderRight = '1px solid #000000';
    button.style.borderBottom = '1px solid #000000';
    button.style.boxShadow = 'inset -1px -1px 0 #404040, inset 1px 1px 0 rgba(255, 255, 255, 0.3)';
    button.style.overflowWrap = 'break-word';
    button.style.hyphens = 'none';
    button.style.display = 'flex';
    button.style.alignItems = 'center';
    button.style.justifyContent = 'center';
    button.textContent = text;
    button.addEventListener('click', () => {
        // Auto-play time when taking any action
        if (window.timeClock && window.timeClock.play) {
            window.timeClock.play();
        }
        onClick();
    });
    return button;
}

/**
 * Check if a building is currently open based on time of day
 * @param {Object} building - Building data
 * @param {number} currentTimeInMinutes - Current time in minutes (0-1439)
 * @returns {boolean} True if building is open
 */
export function isBuildingOpen(building, currentTimeInMinutes) {
    // Always open buildings
    if (building.open === 'always') {
        return true;
    }

    // Private buildings (never accessible)
    if (building.open === -1 || building.open < 0) {
        return false;
    }

    const openTime = building.open;
    const closeTime = building.close;

    // No time specified - assume always open
    if (openTime === undefined) {
        return true;
    }

    // Open rest of day (close is null)
    if (closeTime === null) {
        return currentTimeInMinutes >= openTime;
    }

    // Check if open hours wrap around midnight (overnight businesses like inns/taverns)
    if (openTime < closeTime) {
        // Normal hours (e.g., 480-1020: 8 AM to 5 PM)
        return currentTimeInMinutes >= openTime && currentTimeInMinutes < closeTime;
    } else {
        // Overnight hours (e.g., 1020-480: 5 PM through night to 8 AM)
        return currentTimeInMinutes >= openTime || currentTimeInMinutes < closeTime;
    }
}

/**
 * Show message when clicking a closed building
 * @param {Object} building - Building data
 */
export function showBuildingClosedMessage(building) {
    let openTimeName;
    if (building.open === 'always') {
        openTimeName = 'always';
    } else {
        // Convert minutes (0-1439) to hours and minutes
        const openHour = Math.floor(building.open / 60);
        const openMin = building.open % 60;
        openTimeName = formatTime(openHour, openMin);
    }

    showMessage(`🔒 ${building.name} is closed. Opens at ${openTimeName}.`, 'error');
}

/**
 * Enter a building
 * @param {string} buildingId - Building ID to enter
 */
export async function enterBuilding(buildingId) {
    logger.debug('Entering building:', buildingId);

    try {
        await gameAPI.sendAction('enter_building', { building_id: buildingId });
        await refreshGameState();
        await updateAllDisplays();
    } catch (error) {
        logger.error('Failed to enter building:', error);
        showMessage('❌ Failed to enter building', 'error');
    }
}

/**
 * Exit the current building
 */
export async function exitBuilding() {
    logger.debug('Exiting building');

    try {
        await gameAPI.sendAction('exit_building', {});
        await refreshGameState();
        await updateAllDisplays();
    } catch (error) {
        logger.error('Failed to exit building:', error);
        showMessage('❌ Failed to exit building', 'error');
    }
}

/**
 * Initiate dialogue with an NPC
 * @param {string} npcId - NPC ID to talk to
 */
export async function talkToNPC(npcId) {
    logger.debug('Initiating dialogue with NPC:', npcId);

    try {
        const result = await gameAPI.sendAction('talk_to_npc', { npc_id: npcId });

        if (result.success && result.delta?.npc_dialogue) {
            showNPCDialogue(result.delta.npc_dialogue, result.message);
        } else {
            showMessage('❌ Failed to talk to NPC', 'error');
        }
    } catch (error) {
        logger.error('Error talking to NPC:', error);
        showMessage('❌ Failed to talk to NPC', 'error');
    }
}

/**
 * Show NPC dialogue UI - replaces bottom UI with dialogue options
 * @param {Object} dialogueData - Dialogue data with options
 * @param {string} npcMessage - NPC message to display
 */
export function showNPCDialogue(dialogueData, npcMessage) {
    logger.debug('Showing NPC dialogue:', dialogueData);

    // Show NPC message in yellow
    if (npcMessage) {
        showMessage(npcMessage, 'warning'); // warning = yellow
    }

    // Get the action-buttons container (parent of all three columns)
    const actionButtonsArea = document.getElementById('action-buttons');
    if (!actionButtonsArea) {
        logger.error('action-buttons container not found!');
        return;
    }

    // Hide the normal action buttons
    actionButtonsArea.style.display = 'none';

    // Create or get dialogue overlay container
    let dialogueOverlay = document.getElementById('npc-dialogue-overlay');
    if (!dialogueOverlay) {
        dialogueOverlay = document.createElement('div');
        dialogueOverlay.id = 'npc-dialogue-overlay';
        dialogueOverlay.className = 'p-4 bg-gray-800 border-t-4 border-yellow-500';
        dialogueOverlay.style.height = '125px'; // Match action-buttons height

        // Insert right after action-buttons
        actionButtonsArea.parentNode.insertBefore(dialogueOverlay, actionButtonsArea.nextSibling);
    }

    // Clear previous content
    dialogueOverlay.innerHTML = '';

    // Create dialogue options grid
    const optionsGrid = document.createElement('div');
    optionsGrid.className = 'grid grid-cols-3 gap-1';

    // Add dialogue option buttons
    if (dialogueData.options && dialogueData.options.length > 0) {
        dialogueData.options.forEach(optionKey => {
            const button = document.createElement('button');
            button.className = 'text-white transition-all';
            button.style.fontSize = '7px';
            button.style.background = '#9e8b6b'; // Muted yellow/tan
            button.style.color = '#ffffff';
            button.style.cursor = 'pointer';
            button.style.padding = '2px 4px';
            button.style.borderTop = '1px solid #ffffff';
            button.style.borderLeft = '1px solid #ffffff';
            button.style.borderRight = '1px solid #000000';
            button.style.borderBottom = '1px solid #000000';
            button.style.boxShadow = 'inset -1px -1px 0 #404040, inset 1px 1px 0 rgba(255, 255, 255, 0.3)';

            // Format option text (convert snake_case to readable)
            const optionText = formatDialogueOption(optionKey);
            button.textContent = optionText;

            button.addEventListener('click', () => selectDialogueOption(dialogueData.npc_id, optionKey));
            optionsGrid.appendChild(button);
        });
    } else {
        logger.warn('No dialogue options provided!');
    }

    dialogueOverlay.appendChild(optionsGrid);
    dialogueOverlay.style.display = 'block';

    logger.debug('Dialogue overlay created with', dialogueData.options?.length || 0, 'options');
}

/**
 * Format dialogue option key to readable text
 * @param {string} optionKey - Option key (snake_case)
 * @returns {string} Readable option text
 */
export function formatDialogueOption(optionKey) {
    const optionNames = {
        'ask_about_fee': 'Ask about fee',
        'ask_about_tribute': 'Ask about tribute',
        'pay_fee': 'Pay the fee',
        'pay_tribute': 'Pay tribute',
        'use_storage': 'Use storage',
        'maybe_later': 'Maybe later',
        'goodbye': 'Goodbye'
    };

    return optionNames[optionKey] || optionKey.replace(/_/g, ' ');
}

/**
 * Handle dialogue option selection
 * @param {string} npcId - NPC ID
 * @param {string} choice - Selected dialogue option
 */
export async function selectDialogueOption(npcId, choice) {
    logger.debug('Selected dialogue option:', choice);

    try {
        const result = await gameAPI.sendAction('npc_dialogue_choice', {
            npc_id: npcId,
            choice: choice
        });

        if (result.success) {
            // Refresh game state after successful dialogue action
            await refreshGameState();
            await updateAllDisplays();

            // Check if vault should open (check this first before close action)
            if (result.delta?.open_vault) {
                logger.debug('Opening vault with data:', result.delta.open_vault);
                closeNPCDialogue();
                // Show message before opening vault
                if (result.message) {
                    showMessage(result.message, 'warning');
                }
                showVaultUI(result.delta.open_vault);
            }
            // Check if shop should open
            else if (result.delta?.open_shop) {
                logger.debug('Opening shop for merchant:', result.delta.open_shop);
                closeNPCDialogue();
                // Show message before opening shop
                if (result.message) {
                    showMessage(result.message, 'warning');
                }
                // Open shop with optional tab selection
                const shopTab = result.delta.shop_tab || 'buy';
                window.openShop(result.delta.open_shop);
                if (shopTab === 'sell') {
                    window.switchShopTab('sell');
                }
            }
            // Check if show booking UI should open
            else if (result.delta?.show_booking) {
                logger.debug('Opening show booking UI:', result.delta.show_booking);
                closeNPCDialogue();
                showShowBookingUI(result.delta.show_booking);
            }
            // Check if dialogue should close
            else if (result.delta?.npc_dialogue?.action === 'close') {
                closeNPCDialogue();
                // Show message when closing dialogue
                if (result.message) {
                    showMessage(result.message, 'warning');
                }
            }
            // Continue dialogue with new options
            else if (result.delta?.npc_dialogue) {
                // When continuing dialogue, only show the message once
                // The message will be shown by showNPCDialogue, so pass it there
                showNPCDialogue(result.delta.npc_dialogue, result.message);
            }
            // No delta but has message - just show the message
            else if (result.message) {
                showMessage(result.message, 'warning');
            }
        } else {
            logger.error('Dialogue option failed:', result.error);
            showMessage(result.error || 'Dialogue option failed', 'error');
        }
    } catch (error) {
        logger.error('Error selecting dialogue option:', error);
        showMessage('❌ Failed to process dialogue', 'error');
    }
}

/**
 * Close NPC dialogue and restore normal UI
 */
export function closeNPCDialogue() {
    logger.debug('Closing NPC dialogue');

    // Hide dialogue overlay
    const dialogueOverlay = document.getElementById('npc-dialogue-overlay');
    if (dialogueOverlay) {
        dialogueOverlay.style.display = 'none';
    }

    // Restore action buttons
    const actionButtonsArea = document.getElementById('action-buttons');
    if (actionButtonsArea) {
        actionButtonsArea.style.display = 'grid'; // Restore grid display
    }

    logger.debug('Dialogue closed, action buttons restored');
}

/**
 * Show booking UI overlay for selecting a show to perform
 * @param {Object} bookingData - Show booking data with available shows
 */
export function showShowBookingUI(bookingData) {
    logger.debug('Showing show booking UI with data:', bookingData);

    const sceneContainer = document.getElementById('scene-container');
    if (!sceneContainer) return;

    // Create or get show booking overlay
    let bookingOverlay = document.getElementById('show-booking-overlay');
    if (!bookingOverlay) {
        bookingOverlay = document.createElement('div');
        bookingOverlay.id = 'show-booking-overlay';
        bookingOverlay.className = 'absolute inset-0 flex items-center justify-center';
        bookingOverlay.style.cssText = 'background: rgba(0, 0, 0, 0.85); z-index: 100;';
        sceneContainer.appendChild(bookingOverlay);
    }

    // Clear previous content
    bookingOverlay.innerHTML = '';

    // Create booking container with retro window style
    const bookingContainer = document.createElement('div');
    bookingContainer.style.cssText = `
        background: #2a2a2a;
        border-top: 2px solid #4a4a4a;
        border-left: 2px solid #4a4a4a;
        border-right: 2px solid #0a0a0a;
        border-bottom: 2px solid #0a0a0a;
        clip-path: polygon(
            4px 0, calc(100% - 4px) 0,
            100% 4px, 100% calc(100% - 4px),
            calc(100% - 4px) 100%, 4px 100%,
            0 calc(100% - 4px), 0 4px
        );
        max-width: 480px;
        width: 90%;
        max-height: 85%;
        display: flex;
        flex-direction: column;
    `;

    // Title bar
    const titleBar = document.createElement('div');
    titleBar.style.cssText = `
        background: #9e8b6b;
        border-bottom: 2px solid #0a0a0a;
        padding: 4px 6px;
        display: flex;
        justify-content: space-between;
        align-items: center;
    `;

    const title = document.createElement('div');
    title.style.cssText = 'color: #ffffff; font-size: 10px; font-weight: bold;';
    title.textContent = '♫ Performance Booking';

    const closeButton = document.createElement('button');
    closeButton.style.cssText = `
        background: #dc2626;
        color: white;
        border-top: 1px solid #ef4444;
        border-left: 1px solid #ef4444;
        border-right: 1px solid #991b1b;
        border-bottom: 1px solid #991b1b;
        padding: 2px 6px;
        font-size: 8px;
        font-weight: bold;
        cursor: pointer;
    `;
    closeButton.textContent = 'X';
    closeButton.onclick = () => {
        bookingOverlay.remove();
    };

    titleBar.appendChild(title);
    titleBar.appendChild(closeButton);
    bookingContainer.appendChild(titleBar);

    // Content area
    const contentArea = document.createElement('div');
    contentArea.style.cssText = 'padding: 6px; overflow-y: auto; flex: 1;';

    // Show time info banner
    const showTimeHour = Math.floor(bookingData.show_time / 60);
    const showTimeMin = bookingData.show_time % 60;
    const infoBanner = document.createElement('div');
    infoBanner.style.cssText = `
        background: #1a1a1a;
        border-top: 1px solid #000;
        border-left: 1px solid #000;
        border-right: 1px solid #3a3a3a;
        border-bottom: 1px solid #3a3a3a;
        padding: 4px 6px;
        margin-bottom: 6px;
        font-size: 8px;
        color: #cccccc;
    `;
    infoBanner.innerHTML = `<span style="color: #fbbf24;">Show Time:</span> ${showTimeHour}:${showTimeMin.toString().padStart(2, '0')} • Select a performance:`;
    contentArea.appendChild(infoBanner);

    // Shows list
    bookingData.available_shows.forEach((show, index) => {
        const showCard = document.createElement('div');
        showCard.style.cssText = `
            background: #1f1f1f;
            border-top: 1px solid #3a3a3a;
            border-left: 1px solid #3a3a3a;
            border-right: 1px solid #0a0a0a;
            border-bottom: 1px solid #0a0a0a;
            margin-bottom: 4px;
            padding: 5px;
            cursor: pointer;
        `;

        // Format required instruments
        const instruments = show.required_instruments.map(inst => {
            return inst.charAt(0).toUpperCase() + inst.slice(1);
        }).join(', ');

        // Calculate charisma bonus display
        const charBonus = show.charisma_gold_bonus || 0;

        showCard.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 6px;">
                <div style="flex: 1; min-width: 0;">
                    <div style="color: #fbbf24; font-size: 9px; font-weight: bold; margin-bottom: 2px;">${show.name}</div>
                    <div style="color: #9ca3af; font-size: 7px; margin-bottom: 3px; line-height: 1.3;">${show.description}</div>
                    <div style="font-size: 7px; color: #6b7280; line-height: 1.4;">
                        <div>Req: ${instruments}</div>
                        <div style="color: #10b981;">Pay: ${show.base_gold}g + ${show.base_xp}xp${charBonus > 0 ? ` (+${charBonus}g/CHA)` : ''}</div>
                    </div>
                </div>
                <button class="book-show-btn" data-show-index="${index}" style="
                    background: #16a34a;
                    color: white;
                    border-top: 1px solid #22c55e;
                    border-left: 1px solid #22c55e;
                    border-right: 1px solid #15803d;
                    border-bottom: 1px solid #15803d;
                    padding: 4px 8px;
                    font-size: 7px;
                    font-weight: bold;
                    cursor: pointer;
                    white-space: nowrap;
                    flex-shrink: 0;
                ">BOOK</button>
            </div>
        `;

        // Hover effect
        showCard.addEventListener('mouseenter', () => {
            showCard.style.background = '#2a2a2a';
        });
        showCard.addEventListener('mouseleave', () => {
            showCard.style.background = '#1f1f1f';
        });

        // Handle booking
        const bookButton = showCard.querySelector('.book-show-btn');
        bookButton.addEventListener('mouseenter', () => {
            bookButton.style.background = '#15803d';
        });
        bookButton.addEventListener('mouseleave', () => {
            bookButton.style.background = '#16a34a';
        });
        bookButton.onclick = async (e) => {
            e.stopPropagation();
            await bookShow(show.id, bookingData.npc_id);
            bookingOverlay.remove();
        };

        contentArea.appendChild(showCard);
    });

    bookingContainer.appendChild(contentArea);
    bookingOverlay.appendChild(bookingContainer);
    bookingOverlay.style.display = 'flex';
}

/**
 * Book a show performance
 * @param {string} showId - The ID of the show to book
 * @param {string} npcId - The ID of the NPC booking the show
 */
async function bookShow(showId, npcId) {
    logger.debug('Booking show:', showId, 'with NPC:', npcId);

    try {
        const result = await gameAPI.sendAction('book_show', {
            show_id: showId,
            npc_id: npcId
        });

        if (result.success) {
            showMessage(result.message, 'success');

            // Update cached state with booked_shows from response if available
            if (result.data?.booked_shows) {
                const state = getGameStateSync();
                state.booked_shows = result.data.booked_shows;
                state.character.booked_shows = result.data.booked_shows;
                logger.debug('Updated booked_shows in state:', result.data.booked_shows);
            }

            // Refresh state to get updated game time, gold, etc.
            await refreshGameState();
            updateAllDisplays();
        } else {
            showMessage(result.error || 'Failed to book show', 'error');
        }
    } catch (error) {
        logger.error('Error booking show:', error);
        showMessage('❌ Failed to book show', 'error');
    }
}

/**
 * Show vault UI overlay (40 slots over main scene)
 * @param {Object} vaultData - Vault data with slots
 */
export function showVaultUI(vaultData) {
    // Get scene container to overlay on top of it
    const sceneContainer = document.getElementById('scene-container');
    if (!sceneContainer) return;

    // Create or get vault overlay
    let vaultOverlay = document.getElementById('vault-overlay');
    if (!vaultOverlay) {
        vaultOverlay = document.createElement('div');
        vaultOverlay.id = 'vault-overlay';
        vaultOverlay.className = 'absolute inset-0 bg-black bg-opacity-90 flex items-center justify-center';
        vaultOverlay.style.zIndex = '100';
        sceneContainer.appendChild(vaultOverlay);
    }

    // Clear previous content
    vaultOverlay.innerHTML = '';

    // Create vault container
    const vaultContainer = document.createElement('div');
    vaultContainer.className = 'p-2 w-full h-full flex flex-col';

    // Header
    const header = document.createElement('div');
    header.className = 'flex justify-between items-center mb-2';

    const title = document.createElement('h2');
    title.className = 'text-yellow-400 font-bold';
    title.style.fontSize = '12px';
    title.textContent = '🏦 Vault Storage';

    const closeButton = document.createElement('button');
    closeButton.className = 'text-white px-2 py-1 font-bold';
    closeButton.style.cssText = 'background: #dc2626; border-top: 2px solid #ef4444; border-left: 2px solid #ef4444; border-right: 2px solid #991b1b; border-bottom: 2px solid #991b1b; font-size: 10px;';
    closeButton.textContent = 'Close';
    closeButton.addEventListener('click', closeVaultUI);

    header.appendChild(title);
    header.appendChild(closeButton);
    vaultContainer.appendChild(header);

    // Vault slots grid (40 slots in 8x5 grid)
    const slotsGrid = document.createElement('div');
    slotsGrid.className = 'grid grid-cols-8 gap-1 flex-1';
    slotsGrid.id = 'vault-slots-grid';
    slotsGrid.style.gridAutoRows = '1fr';

    const slots = vaultData.slots || [];
    for (let i = 0; i < 40; i++) {
        const slotData = slots[i] || { slot: i, item: null, quantity: 0 };
        const slotElement = createVaultSlot(slotData, i, vaultData.building || vaultData.location);
        slotsGrid.appendChild(slotElement);
    }

    vaultContainer.appendChild(slotsGrid);

    // Instructions
    const instructions = document.createElement('div');
    instructions.className = 'text-gray-400 text-center mt-1';
    instructions.style.fontSize = '8px';
    instructions.textContent = 'Click inventory items to store. Click vault items to withdraw.';
    vaultContainer.appendChild(instructions);

    vaultOverlay.appendChild(vaultContainer);
    vaultOverlay.style.display = 'flex';

    // Mark vault as open - sync both window and module state
    window.vaultOpen = true;

    // Also set the module-level vault state in inventoryInteractions
    // This ensures the left-click handler knows the vault is open
    import('../systems/inventoryInteractions.js').then(module => {
        if (module.setVaultOpen) {
            module.setVaultOpen(true);
        }
    });

    // Force DOM to update before binding events
    requestAnimationFrame(() => {
        if (window.inventoryInteractions && window.inventoryInteractions.bindInventoryEvents) {
            window.inventoryInteractions.bindInventoryEvents();
        }
    });

    logger.debug('showVaultUI: Vault UI refreshed with', vaultData?.slots?.length || 0, 'slots');
}

/**
 * Create a single vault slot element (styled like backpack slots)
 * @param {Object} slotData - Slot data with item and quantity
 * @param {number} slotIndex - Slot index (0-39)
 * @param {string} buildingId - Building ID for vault
 * @returns {HTMLElement} Vault slot element
 */
export function createVaultSlot(slotData, slotIndex, buildingId) {
    const slot = document.createElement('div');
    slot.className = 'vault-slot relative cursor-pointer hover:bg-gray-800 flex items-center justify-center';
    // Match backpack slot styling exactly
    slot.style.cssText = `aspect-ratio: 1; background: #1a1a1a; border-top: 2px solid #000000; border-left: 2px solid #000000; border-right: 2px solid #3a3a3a; border-bottom: 2px solid #3a3a3a; clip-path: polygon(3px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0 calc(100% - 3px), 0 3px);`;

    // Data attributes for drag-and-drop
    slot.setAttribute('data-vault-slot', slotIndex);
    slot.setAttribute('data-vault-building', buildingId);
    slot.setAttribute('data-slot-type', 'vault');

    if (slotData.item && slotData.quantity > 0) {
        slot.setAttribute('data-item-id', slotData.item);

        // Create image container
        const imgDiv = document.createElement('div');
        imgDiv.className = 'w-full h-full flex items-center justify-center p-1';
        const img = document.createElement('img');
        img.src = `/res/img/items/${slotData.item}.png`;
        img.alt = slotData.item;
        img.className = 'w-full h-full object-contain';
        img.style.imageRendering = 'pixelated';
        img.onerror = function() {
            if (!this.dataset.fallbackAttempted) {
                this.dataset.fallbackAttempted = 'true';
                this.src = '/res/img/items/unknown.png';
            }
        };
        imgDiv.appendChild(img);
        slot.appendChild(imgDiv);

        // Add quantity label if > 1
        if (slotData.quantity > 1) {
            const quantityLabel = document.createElement('div');
            quantityLabel.className = 'absolute bottom-0 right-0 text-white';
            quantityLabel.style.fontSize = '10px';
            quantityLabel.textContent = `${slotData.quantity}`;
            slot.appendChild(quantityLabel);
        }
    }

    return slot;
}

/**
 * Close vault UI
 */
export function closeVaultUI() {
    const vaultOverlay = document.getElementById('vault-overlay');
    if (vaultOverlay) {
        vaultOverlay.style.display = 'none';
    }

    // Mark vault as closed - sync both window and module state
    window.vaultOpen = false;

    // Also set the module-level vault state in inventoryInteractions
    import('../systems/inventoryInteractions.js').then(module => {
        if (module.setVaultOpen) {
            module.setVaultOpen(false);
        }
    });

    // Refresh inventory display
    // Import updateCharacterDisplay dynamically to avoid circular dependency
    import('./characterDisplay.js').then(module => {
        module.updateCharacterDisplay();
    });
}

/**
 * Check if player has a rented room at the current building
 * @param {string} buildingId - Building ID to check
 * @returns {boolean} True if room is rented here
 */
export function checkIfRoomRented(buildingId) {
    const state = getGameStateSync();

    // Try multiple possible paths for rented_rooms
    const rentedRooms = state.rented_rooms || state.character?.rented_rooms || [];
    const currentDay = state.current_day || state.character?.current_day || 0;
    const currentTime = state.time_of_day || state.character?.time_of_day || 0;

    logger.debug('Checking rented room:', { buildingId, rentedRooms, currentDay, currentTime });

    // Check if there's a valid (non-expired) rented room at this building
    for (const room of rentedRooms) {
        if (room.building === buildingId) {
            const expDay = room.expiration_day || 0;
            const expTime = room.expiration_time || 0;

            logger.debug('Found rented room:', { room, expDay, expTime });

            // Check if not expired
            if (currentDay < expDay || (currentDay === expDay && currentTime <= expTime)) {
                logger.debug('Room is valid!');
                return true;
            }
        }
    }

    logger.debug('No valid room found');
    return false;
}

/**
 * Check if player has a show ready to perform at the current building
 * @param {string} buildingId - Building ID to check
 * @param {number} timeInMinutes - Current time in minutes
 * @returns {boolean} True if show is ready to perform
 */
export function checkIfShowReady(buildingId, timeInMinutes) {
    const state = getGameStateSync();

    // Try multiple possible paths for booked_shows
    const bookedShows = state.booked_shows || state.character?.booked_shows || [];
    const currentDay = state.current_day || state.character?.current_day || 0;

    logger.debug('Checking booked show:', { buildingId, bookedShows, currentDay, timeInMinutes });

    // Check if there's an unperformed show at this venue
    for (const show of bookedShows) {
        if (show.venue_id === buildingId && show.day === currentDay && !show.performed) {
            const showTime = show.show_time || 1260; // Default 9 PM
            const timeDiff = timeInMinutes - showTime;

            logger.debug('Found booked show:', { show, showTime, timeDiff });

            // Show is ready if it's between show time and 60 minutes after (9-10pm window)
            if (timeDiff >= 0 && timeDiff <= 60) {
                logger.debug('Show is ready to perform!');
                return true;
            }
        }
    }

    logger.debug('No show ready');
    return false;
}

/**
 * Sleep in rented room
 */
export async function sleepInRoom() {
    logger.debug('Sleeping in rented room');

    try {
        const result = await gameAPI.sendAction('sleep', {});

        if (result.success) {
            // Apply delta for surgical updates (HP, mana, fatigue, hunger)
            if (result.delta) {
                const { deltaApplier } = await import('../systems/deltaApplier.js');
                deltaApplier.applyDelta(result.delta);
            }

            // Force sync clock to new time (sleep is a major time jump)
            if (result.data) {
                const { smoothClock } = await import('../systems/smoothClock.js');
                const timeOfDay = result.data.time_of_day;
                const currentDay = result.data.current_day || 1;
                if (timeOfDay !== undefined) {
                    smoothClock.syncFromBackend(timeOfDay, currentDay, true); // Force sync
                    logger.debug(`Clock synced after sleep: Day ${currentDay}, time ${timeOfDay}`);
                }

                // Update local state cache with rented_rooms so sleep button is removed
                if (result.data.rented_rooms !== undefined) {
                    const state = getGameStateSync();
                    if (state) {
                        state.rented_rooms = result.data.rented_rooms;
                        if (state.character) {
                            state.character.rented_rooms = result.data.rented_rooms;
                        }
                    }
                    logger.debug('Updated rented_rooms after sleep:', result.data.rented_rooms);
                }
            }

            showMessage(result.message || 'You wake up refreshed!', 'success');

            // Refresh game state and all displays after sleep
            await refreshGameState();
            const { updateAllDisplays } = await import('./displayCoordinator.js');
            await updateAllDisplays();

            // Refresh location display to update buildings/NPCs after time change
            await displayCurrentLocation();
        } else {
            showMessage(result.error || 'Failed to sleep', 'error');
        }
    } catch (error) {
        logger.error('Error sleeping:', error);
        showMessage('❌ Failed to sleep', 'error');
    }
}

/**
 * Perform booked show
 */
export async function performShow() {
    logger.debug('Performing booked show');

    try {
        const result = await gameAPI.sendAction('play_show', {});

        if (result.success) {
            showMessage(result.message || 'Excellent performance!', 'success');
            await refreshGameState();
            await updateAllDisplays();
        } else {
            showMessage(result.error || 'Failed to perform show', 'error');
        }
    } catch (error) {
        logger.error('Error performing show:', error);
        showMessage('❌ Failed to perform show', 'error');
    }
}

/**
 * Initialize auto-update listeners for time-based UI changes
 * Sets up event listeners that update building/NPC UI when time changes
 * Uses debouncing to prevent excessive re-renders
 */
function initializeAutoUpdate() {
    logger.info('Initializing location auto-update listeners');

    // Debounced update function - waits 100ms after last event before updating
    const debouncedUpdate = () => {
        // Clear any pending update
        if (renderTimeout) {
            clearTimeout(renderTimeout);
        }

        // Schedule new update after short delay
        renderTimeout = setTimeout(async () => {
            logger.debug('⏰ Time-based location update triggered');
            await displayCurrentLocation();
        }, 100);
    };

    // Listen for time changes from the time clock (fires every 5 seconds)
    eventBus.on('gameStateChange', debouncedUpdate);

    logger.debug('Location auto-update listeners initialized (debounced 100ms)');
}

/**
 * Check if player is in a building that just closed and eject them
 * Called on every location display update to ensure player isn't in closed building
 */
async function checkAndEjectFromClosedBuilding() {
    const state = getGameStateSync();
    const currentBuildingId = state.location?.building;

    // Skip if not in a building
    if (!currentBuildingId) {
        return;
    }

    // Get current time in minutes
    const timeInMinutes = state.character?.time_of_day !== undefined ? state.character.time_of_day : 720;

    // Get current location data to find the building
    const cityId = state.location?.current;
    const districtKey = state.location?.district || 'center';
    const districtId = `${cityId}-${districtKey}`;
    const locationData = getLocationById(districtId);

    if (!locationData) {
        return;
    }

    // Get district data
    let districtData = null;
    if (locationData.properties?.districts) {
        for (const district of Object.values(locationData.properties.districts)) {
            if (district.id === districtId) {
                districtData = district;
                break;
            }
        }
    }

    const currentData = districtData || locationData;
    const buildings = currentData.buildings || currentData.properties?.buildings;

    if (!buildings) {
        return;
    }

    // Find the current building
    const currentBuilding = buildings.find(b => b.id === currentBuildingId);
    if (!currentBuilding) {
        return;
    }

    // Check if building is closed
    const isOpen = isBuildingOpen(currentBuilding, timeInMinutes);

    if (!isOpen) {
        // Building is closed! Eject player
        logger.info(`Building ${currentBuildingId} is now closed. Ejecting player...`);

        // Exit the building
        await exitBuilding();

        // Show message to player
        showMessage(`The ${currentBuilding.name} has closed for the day. You've been escorted out.`, 'warning');
    }
}

// Export functions to window for delta applier
window.enterBuilding = enterBuilding;
window.showBuildingClosedMessage = showBuildingClosedMessage;
window.talkToNPC = talkToNPC;

logger.debug('Location display module loaded');
