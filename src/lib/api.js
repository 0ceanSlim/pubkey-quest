/**
 * Game API Client - Go-First Architecture
 *
 * All game state lives in Go memory.
 * JavaScript only sends actions and renders responses.
 *
 * @module lib/api
 */

import { logger } from './logger.js';
import { API_BASE_URL } from '../config/constants.js';

class GameAPI {
    constructor() {
        this.npub = null;
        this.saveID = null;
        this.initialized = false;
    }

    /**
     * Initialize the API with user session
     * @param {string} npub - User's Nostr public key
     * @param {string} saveID - Save file ID
     */
    init(npub, saveID) {
        this.npub = npub;
        this.saveID = saveID;
        this.initialized = true;
        logger.info('Game API initialized:', { npub, saveID });
    }

    /**
     * Check if initialized
     * @throws {Error} If API not initialized
     */
    ensureInitialized() {
        if (!this.initialized) {
            throw new Error('Game API not initialized. Call init() first.');
        }
    }

    /**
     * Send a game action to the backend
     * @param {string} actionType - Type of action
     * @param {Object} params - Action parameters
     * @returns {Promise<Object>} Action result with updated state
     */
    async sendAction(actionType, params = {}) {
        this.ensureInitialized();

        logger.debug(`Sending action: ${actionType}`, params);

        try {
            const response = await fetch(`${API_BASE_URL}/game/action`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    npub: this.npub,
                    save_id: this.saveID,
                    action: {
                        type: actionType,
                        params: params
                    }
                })
            });

            if (!response.ok) {
                throw new Error(`Action failed: ${response.status}`);
            }

            const result = await response.json();

            if (!result.success) {
                throw new Error(result.error || 'Action failed');
            }

            logger.info(`Action completed: ${actionType}`, result.message);

            // Return the updated state
            return result;

        } catch (error) {
            logger.error(`Action failed: ${actionType}`, error);
            throw error;
        }
    }

    /**
     * Fetch current game state from backend
     * @returns {Promise<Object>} Current game state
     */
    async getState() {
        this.ensureInitialized();

        try {
            const response = await fetch(
                `${API_BASE_URL}/game/state?npub=${this.npub}&save_id=${this.saveID}`
            );

            if (!response.ok) {
                throw new Error(`Failed to fetch state: ${response.status}`);
            }

            const result = await response.json();

            if (!result.success) {
                throw new Error('Failed to fetch state');
            }

            return result.state;

        } catch (error) {
            logger.error('Failed to fetch game state:', error);
            throw error;
        }
    }

    /**
     * Save game to disk (manual save)
     * @returns {Promise<boolean>} Success status
     */
    async saveGame() {
        this.ensureInitialized();

        logger.info('Saving game to disk...');

        try {
            const response = await fetch(`${API_BASE_URL}/session/save`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    npub: this.npub,
                    save_id: this.saveID
                })
            });

            if (!response.ok) {
                throw new Error(`Save failed: ${response.status}`);
            }

            const result = await response.json();

            if (!result.success) {
                throw new Error(result.message || 'Save failed');
            }

            logger.info('Game saved to disk');
            return true;

        } catch (error) {
            logger.error('Save failed:', error);
            throw error;
        }
    }

    // ========================================================================
    // CONVENIENCE METHODS FOR COMMON ACTIONS
    // ========================================================================

    /**
     * Move to a new location
     * @param {string} location - Location ID
     * @param {string} district - District name (optional)
     * @param {string} building - Building name (optional)
     */
    async move(location, district = '', building = '') {
        return await this.sendAction('move', {
            location,
            district,
            building
        });
    }

}

// Export singleton instance
export const gameAPI = new GameAPI();

logger.debug('Game API client loaded');
