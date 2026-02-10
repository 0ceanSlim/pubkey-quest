/**
 * Game State Management Module
 *
 * Manages the client-side game state cache and transformations.
 * State is authoritative on the Go backend; this module caches and transforms it for the UI.
 *
 * @module state/gameState
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { eventBus } from '../lib/events.js';

// Cache the last fetched state for immediate UI access
let cachedGameState = null;

/**
 * Get the current game state (fetches from Go if needed)
 * @param {boolean} forceRefresh - Force fetch from backend even if cached
 * @returns {Promise<Object>} Game state object
 */
export async function getGameState(forceRefresh = false) {
    if (!forceRefresh && cachedGameState) {
        return cachedGameState;
    }

    try {
        // Fetch from Go backend
        logger.debug('🔄 Fetching game state from backend...');
        const state = await gameAPI.getState();
        logger.debug('📦 Raw state from backend:', state);

        // Transform Go SaveFile format to UI format
        const uiState = transformSaveDataToUIState(state);
        logger.debug('✨ Transformed UI state:', uiState);

        // Cache it
        cachedGameState = uiState;

        return uiState;
    } catch (error) {
        logger.error('Failed to fetch game state:', error);
        // Return cached state if fetch fails
        return cachedGameState || getEmptyGameState();
    }
}

/**
 * Synchronous version for immediate access (uses cached state)
 * @returns {Object} Cached game state or empty state
 */
export function getGameStateSync() {
    if (!cachedGameState) {
        logger.warn('No cached game state, returning empty state');
        return getEmptyGameState();
    }
    return cachedGameState;
}

/**
 * Transform SaveFile (Go format) to UI state format
 * @param {Object} saveData - Save data from Go backend
 * @returns {Object} Transformed UI state
 */
export function transformSaveDataToUIState(saveData) {
    // Map display names to location/district IDs
    const locationNameMap = {
        // Cities
        'Verdant City': 'verdant',
        'Golden Haven': 'goldenhaven',
        'Goldenhaven': 'goldenhaven',

        // Towns
        'Ironpeak': 'ironpeak',
        'Iron Peak': 'ironpeak',
        'Frosthold': 'frosthold',

        // Villages
        'Millhaven Village': 'millhaven',
        'Millhaven': 'millhaven',
        'Saltwind Village': 'saltwind',
        'Saltwind': 'saltwind',
        'Marshlight Village': 'marshlight',
        'Marshlight': 'marshlight',
        'Dusthaven Village': 'dusthaven',
        'Dusthaven': 'dusthaven',

        // Kingdoms/General
        'Kingdom': 'kingdom',
        'The Royal Kingdom': 'kingdom',
        'Village': 'village',
        'Dwarven Stronghold': 'dwarven',
        'Desert Oasis': 'desert',
        'Port City': 'port'
    };

    const districtNameMap = {
        // Generic district names
        'Garden Plaza': 'center',
        'Town Square': 'center',
        'Village Center': 'center',
        'City Center': 'center',
        'Town Center': 'center',
        'Kingdom Center': 'center',

        // Directional districts
        'Market District': 'market',
        'Northern Quarter': 'north',
        'North District': 'north',
        'Southern Quarter': 'south',
        'South District': 'south',
        'Western Quarter': 'west',
        'West District': 'west',
        'Eastern Quarter': 'east',
        'East District': 'east',

        // Special districts
        'Harbor': 'harbor',
        'Residential': 'residential'
    };

    // Get location ID (handle both display names and IDs)
    let locationId = saveData.location || 'kingdom';
    if (locationNameMap[locationId]) {
        locationId = locationNameMap[locationId];
    }

    // Get district key (handle both display names and IDs)
    let districtKey = saveData.district || 'center';
    // If it's a full district ID like "verdant-center", extract the key
    if (districtKey.includes('-')) {
        districtKey = districtKey.split('-').pop();
    }
    // If it's a display name like "Garden Plaza", map it
    if (districtNameMap[districtKey]) {
        districtKey = districtNameMap[districtKey];
    }

    // Normalize stats keys to lowercase (Go sends "Strength", UI expects "strength")
    const normalizedStats = {};
    if (saveData.stats) {
        for (const [key, value] of Object.entries(saveData.stats)) {
            normalizedStats[key.toLowerCase()] = value;
        }
    }

    logger.debug('Transforming save data to UI state:', {
        savedLocation: saveData.location,
        savedDistrict: saveData.district,
        mappedLocationId: locationId,
        mappedDistrictKey: districtKey,
        hasInventory: !!saveData.inventory,
        hasGearSlots: !!saveData.inventory?.gear_slots,
        hasGeneralSlots: !!saveData.inventory?.general_slots,
        statsNormalized: normalizedStats
    });

    return {
        character: {
            name: saveData.d || 'Unknown',
            race: saveData.race,
            class: saveData.class,
            background: saveData.background,
            alignment: saveData.alignment,
            level: saveData.level || 1,
            experience: saveData.experience || 0,
            hp: saveData.hp,
            max_hp: saveData.max_hp,
            mana: saveData.mana,
            max_mana: saveData.max_mana,
            fatigue: saveData.fatigue || 0,
            fatigue_counter: saveData.fatigue_counter || 0,
            hunger: saveData.hunger !== undefined ? saveData.hunger : 1,
            hunger_counter: saveData.hunger_counter || 0,
            gold: saveData.gold || 0,
            stats: normalizedStats,
            inventory: saveData.inventory || {},
            vaults: saveData.vaults || [],
            spells: saveData.known_spells || [],
            spell_slots: saveData.spell_slots || {},
            music_tracks_unlocked: saveData.music_tracks_unlocked || [],
            current_day: saveData.current_day || 1,
            time_of_day: saveData.time_of_day !== undefined ? saveData.time_of_day : 12,
            rented_rooms: saveData.rented_rooms || [],
            booked_shows: saveData.booked_shows || [],
            performed_shows: saveData.performed_shows || [],
            active_effects: saveData.active_effects || [],
            // Include pre-calculated values from backend (NOT persisted)
            total_weight: saveData.total_weight,
            weight_capacity: saveData.weight_capacity
        },
        location: {
            current: locationId,
            district: districtKey,
            building: saveData.building || null,
            discovered: saveData.locations_discovered || []
        },
        inventory: saveData.inventory?.general_slots || [],
        equipment: saveData.inventory?.gear_slots || {},
        spells: saveData.known_spells || [],
        combat: null, // Combat state not stored in save file
        // Top-level aliases for easier access
        rented_rooms: saveData.rented_rooms || [],
        booked_shows: saveData.booked_shows || [],
        performed_shows: saveData.performed_shows || [],
        current_day: saveData.current_day || 1,
        time_of_day: saveData.time_of_day !== undefined ? saveData.time_of_day : 12
    };
}

/**
 * Get empty game state (fallback)
 * @returns {Object} Empty game state structure
 */
export function getEmptyGameState() {
    return {
        character: {},
        inventory: [],
        equipment: {},
        spells: [],
        location: {},
        combat: null
    };
}

/**
 * Refresh game state from backend and trigger UI updates
 * @param {boolean} silent - If true, don't emit events (for surgical updates)
 * @returns {Promise<Object>} Updated game state
 */
export async function refreshGameState(silent = false) {
    // Fetch fresh state from Go
    const newState = await getGameState(true);

    // Only emit events if not in silent mode
    if (!silent) {
        // Trigger UI updates via event bus
        eventBus.emit('gameStateChange', newState);

        // Also dispatch DOM event for legacy compatibility
        document.dispatchEvent(new CustomEvent('gameStateChange', { detail: newState }));
    }

    return newState;
}

// ========================================
// Ground Items System (Session-only storage)
// ========================================

// Ground items storage: { "locationId-districtKey": [{item, droppedAt, droppedDay}, ...] }
const groundItems = {};

/**
 * Get location key for ground storage
 * @returns {string} Location key in format "locationId-districtKey"
 */
function getGroundLocationKey() {
    const state = getGameStateSync();
    const cityId = state.location?.current || 'unknown';
    const districtKey = state.location?.district || 'center';
    return `${cityId}-${districtKey}`;
}

/**
 * Add item to ground at current location
 * @param {string} itemId - Item ID
 * @param {number} quantity - Quantity to drop
 */
export function addItemToGround(itemId, quantity = 1) {
    const locationKey = getGroundLocationKey();
    const state = getGameStateSync();
    const currentDay = state.character?.current_day || 1;

    if (!groundItems[locationKey]) {
        groundItems[locationKey] = [];
    }

    groundItems[locationKey].push({
        item: itemId,
        quantity: quantity,
        droppedAt: Date.now(),
        droppedDay: currentDay
    });

    logger.debug(`Item ${itemId} dropped at ${locationKey}`);
    cleanupOldGroundItems();
}

/**
 * Remove item from ground at current location
 * @param {string} itemId - Item ID to pick up
 * @returns {Object|null} Removed item object or null
 */
export function removeItemFromGround(itemId) {
    const locationKey = getGroundLocationKey();

    if (!groundItems[locationKey]) {
        return null;
    }

    const index = groundItems[locationKey].findIndex(ground => ground.item === itemId);
    if (index === -1) {
        return null;
    }

    const removed = groundItems[locationKey].splice(index, 1)[0];
    logger.debug(`Picked up ${itemId} from ${locationKey}`);
    return removed;
}

/**
 * Get all items on ground at current location
 * @returns {Array} Array of ground item objects
 */
export function getGroundItems() {
    const locationKey = getGroundLocationKey();
    cleanupOldGroundItems();
    return groundItems[locationKey] || [];
}

/**
 * Clean up items older than 1 game day
 */
function cleanupOldGroundItems() {
    const state = getGameStateSync();
    const currentDay = state.character?.current_day || 1;

    for (const locationKey in groundItems) {
        groundItems[locationKey] = groundItems[locationKey].filter(ground => {
            const daysPassed = currentDay - ground.droppedDay;
            return daysPassed < 1; // Keep items for less than 1 day
        });

        // Remove empty locations
        if (groundItems[locationKey].length === 0) {
            delete groundItems[locationKey];
        }
    }
}

/**
 * Initialize the game with fresh state or from save
 * This is the main entry point for game initialization
 */
export async function initializeGame() {
    logger.info('🎮 Initializing Nostr Hero...');

    // Wait for session manager to be ready
    if (!window.sessionManager) {
        logger.error('❌ SessionManager not available');
        throw new Error('Session manager not loaded');
    }

    // Initialize session manager and wait for result
    try {
        await window.sessionManager.init();

        if (!window.sessionManager.isAuthenticated()) {
            logger.info('🔐 User not authenticated, redirecting to login');
            window.location.href = '/';
            return;
        }

        logger.info('✅ User authenticated, loading game...');
        const session = window.sessionManager.getSession();
        logger.info(`🎮 Starting game for user: ${session.npub}`);

        // Get save ID from URL
        const urlParams = new URLSearchParams(window.location.search);
        const saveID = urlParams.get('save');

        // Initialize Game API
        if (saveID) {
            gameAPI.init(session.npub, saveID);
            logger.info('🎮 Game API initialized with save:', saveID);
        }

    } catch (error) {
        logger.error('❌ Session initialization failed:', error);
        window.location.href = '/';
        return;
    }

    try {
        // Load static game data
        logger.info('📦 Loading static game data...');
        const gameDataResponse = await fetch('/api/game-data');
        if (!gameDataResponse.ok) {
            throw new Error('Failed to load game data');
        }

        const gameData = await gameDataResponse.json();

        // Store static data in DOM (for backward compatibility)
        document.getElementById('all-items').textContent = JSON.stringify(gameData.items || []);
        document.getElementById('all-spells').textContent = JSON.stringify(gameData.spells || []);
        document.getElementById('all-monsters').textContent = JSON.stringify(gameData.monsters || []);
        document.getElementById('all-locations').textContent = JSON.stringify(gameData.locations || []);
        document.getElementById('all-packs').textContent = JSON.stringify(gameData.packs || []);
        document.getElementById('all-music-tracks').textContent = JSON.stringify(gameData.music_tracks || []);

        // Load NPCs separately
        const npcsResponse = await fetch('/api/npcs');
        if (npcsResponse.ok) {
            const npcs = await npcsResponse.json();
            document.getElementById('all-npcs').textContent = JSON.stringify(npcs || []);
            logger.info(`Loaded ${npcs?.length || 0} NPCs`);
        }

        logger.info(`Loaded game data: ${gameData.items?.length || 0} items, ${gameData.spells?.length || 0} spells, ${gameData.monsters?.length || 0} monsters, ${gameData.locations?.length || 0} locations, ${gameData.music_tracks?.length || 0} music tracks`);

        // Trigger game data loaded event for music system
        document.dispatchEvent(new Event('gameDataLoaded'));

        // Check if we're loading a specific save
        const session = window.sessionManager.getSession();
        const urlParams = new URLSearchParams(window.location.search);
        const saveID = urlParams.get('save');

        if (saveID) {
            // Load specific save into Go memory
            try {
                logger.info('🔄 Initializing session in Go memory...');

                // Initialize session in backend memory
                const initResponse = await fetch('/api/session/init', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        npub: session.npub,
                        save_id: saveID
                    })
                });

                if (!initResponse.ok) {
                    throw new Error(`Failed to initialize session: ${initResponse.status}`);
                }

                const initResult = await initResponse.json();
                logger.info('✅ Session loaded into Go memory:', initResult);

                // Fetch initial game state from Go
                await refreshGameState();

                logger.info('✅ Save file loaded successfully!');

                // Auto-play location music now that state is loaded
                if (window.musicSystem && window.musicSystem.playLocationMusic) {
                    window.musicSystem.playLocationMusic();
                }

            } catch (saveError) {
                logger.error('Failed to load specific save:', saveError);
                // Redirect back to saves page
                setTimeout(() => window.location.href = '/saves', 2000);
                return;
            }
        } else {
            // No save specified, redirect to saves page
            logger.info('🔄 No save specified, redirecting to save selection');
            window.location.href = '/saves';
            return;
        }

        // Show game UI
        const gameApp = document.getElementById('game-app');
        if (gameApp) {
            gameApp.style.display = 'block';
        }

        // Render UI - import these dynamically to avoid circular dependencies
        const { displayCurrentLocation } = await import('../ui/locationDisplay.js');
        const { updateAllDisplays } = await import('../ui/displayCoordinator.js');

        // Check if state was loaded
        const loadedState = getGameStateSync();
        logger.info('📊 State after refresh:', loadedState);

        if (!loadedState || !loadedState.character) {
            logger.error('❌ State not loaded properly!');
            throw new Error('Failed to load game state');
        }

        await displayCurrentLocation();
        await updateAllDisplays();

        // Emit event to notify systems that game state is loaded and UI is ready
        // This allows time clock to sync to the correct time from save
        eventBus.emit('gameStateLoaded', loadedState);
        logger.info('📢 Emitted gameStateLoaded event');

        logger.info('✅ Game initialized and ready to play!');

    } catch (error) {
        logger.error('Failed to initialize game:', error);
        throw error;
    }
}

logger.debug('Game state module loaded');
