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
                // Surface the server's human-readable reason. Handlers put the
                // explanation in `message` (e.g. "Head to your rented room to
                // sleep."); fall back to `error`, then a generic string.
                throw new Error(result.error || result.message || 'Action failed');
            }

            logger.info(`Action completed: ${actionType}`, result.message);

            // Any action can award XP; surface a level-up moment generically so
            // every XP source (shows, quests, …) shows it without bespoke wiring.
            if (result.data?.level_up?.leveled && typeof window !== 'undefined') {
                window.showLevelUpModal?.(result.data.level_up);
            }

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

    /**
     * Fetch the level-up progression guide (1→20) for the current character.
     * @returns {Promise<Object>} { class, current_level, xp_current, xp_to_next, levels[] }
     */
    async getLevelGuide() {
        this.ensureInitialized();

        const response = await fetch(
            `${API_BASE_URL}/progression/guide?npub=${this.npub}&save_id=${this.saveID}`
        );
        if (!response.ok) {
            throw new Error(`Failed to fetch level guide: ${response.status}`);
        }
        const result = await response.json();
        if (!result.success) {
            throw new Error('Failed to fetch level guide');
        }
        return result;
    }

    /**
     * Fetch the character's ability-point allocation state.
     * @returns {Promise<Object>} { unspent, cap, scores }
     */
    async getAbilityPoints() {
        this.ensureInitialized();

        const response = await fetch(
            `${API_BASE_URL}/progression/ability-points?npub=${this.npub}&save_id=${this.saveID}`
        );
        if (!response.ok) {
            throw new Error(`Failed to fetch ability points: ${response.status}`);
        }
        const result = await response.json();
        if (!result.success) {
            throw new Error('Failed to fetch ability points');
        }
        return result;
    }

    /**
     * Spend one banked ability point into an ability.
     * @param {string} ability - Ability name or 3-letter abbrev (e.g. "Strength" / "str")
     * @returns {Promise<Object>} { ability, unspent, cap, scores, max_hp, max_mana }
     */
    async spendAbilityPoint(ability) {
        this.ensureInitialized();

        const response = await fetch(`${API_BASE_URL}/progression/spend-point`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ npub: this.npub, save_id: this.saveID, ability })
        });
        if (!response.ok) {
            // The endpoint returns a plain-text reason (e.g. "no unspent ability points").
            let reason = `spend failed: ${response.status}`;
            try { reason = (await response.text()).trim() || reason; } catch { /* ignore */ }
            throw new Error(reason);
        }
        return await response.json();
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
