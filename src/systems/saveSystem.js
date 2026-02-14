/**
 * Save System Module
 *
 * Handles saving game state from Go memory to disk.
 * Provides manual save functionality and Ctrl+S hotkey.
 *
 * @module systems/saveSystem
 */

import { logger } from '../lib/logger.js';
import { sessionManager } from '../lib/session.js';
import { gameAPI } from '../lib/api.js';

/**
 * Save game to local JSON file (writes from memory to disk)
 * State is already in Go memory, this just persists it to disk
 * @param {Function} showMessage - UI message callback (optional, uses window.showMessage if not provided)
 * @returns {Promise<boolean>} True if save successful
 */
export async function saveGameToLocal(showMessage) {
    // Use provided showMessage or fall back to global
    const messageFunc = showMessage || window.showMessage;

    if (!sessionManager.isAuthenticated()) {
        if (messageFunc) messageFunc('Must be logged in to save', 'error');
        return false;
    }

    if (!gameAPI.initialized) {
        if (messageFunc) messageFunc('Game not initialized', 'error');
        return false;
    }

    try {
        if (messageFunc) messageFunc('Saving game...', 'info');

        // Save from Go memory to disk
        await gameAPI.saveGame();

        if (messageFunc) messageFunc('Game saved successfully!', 'success');
        return true;

    } catch (error) {
        logger.error('Save failed:', error);
        if (messageFunc) messageFunc('Failed to save game: ' + error.message, 'error');
        return false;
    }
}

logger.debug('Save system module loaded');
