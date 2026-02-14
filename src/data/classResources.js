/**
 * Class Resources Data Module
 *
 * Loads and provides access to class resource definitions (mana, stamina, rage, ki, cunning).
 * Used to determine what resource bar to display for each class.
 *
 * @module data/classResources
 */

import { logger } from '../lib/logger.js';

// Cache for class resources data
let classResourcesCache = null;

// Default mana config for casters (and fallback)
const DEFAULT_MANA_CONFIG = {
    type: 'mana',
    label: 'Mana',
    short_label: 'MP',
    color: '#2563eb',
    color_gradient: {
        start: '#2563eb',
        mid: '#1d4ed8',
        end: '#1e40af'
    }
};

/**
 * Load class resources from JSON
 * @returns {Promise<Object>} Class resources keyed by class name (lowercase)
 */
export async function loadClassResources() {
    if (classResourcesCache) {
        return classResourcesCache;
    }

    try {
        const response = await fetch('/data/systems/class-resources.json');
        if (!response.ok) {
            throw new Error(`Failed to fetch class resources: ${response.status}`);
        }
        classResourcesCache = await response.json();
        logger.debug('Class resources loaded:', Object.keys(classResourcesCache));
        return classResourcesCache;
    } catch (error) {
        logger.error('Error loading class resources:', error);
        return {};
    }
}

/**
 * Get resource config for a specific class
 * @param {string} className - The character's class (e.g., "Fighter", "Wizard")
 * @returns {Promise<Object>} Resource configuration with type, label, color, etc.
 */
export async function getClassResource(className) {
    const resources = await loadClassResources();
    const key = className?.toLowerCase();
    const config = resources[key];

    if (!config) {
        logger.warn(`No resource config for class: ${className}, using mana default`);
        return DEFAULT_MANA_CONFIG;
    }

    // If type is mana, merge with default mana config
    if (config.type === 'mana') {
        return { ...DEFAULT_MANA_CONFIG, ...config };
    }

    return config;
}

/**
 * Calculate max resource for a class based on character stats
 * @param {Object} config - Resource config from getClassResource
 * @param {Object} character - Character data with stats, level, etc.
 * @returns {number} Maximum resource value
 */
export function calculateMaxResource(config, character) {
    // If it's mana type, use max_mana from character
    if (config.type === 'mana') {
        return character.max_mana || 0;
    }

    // If there's a formula, calculate it
    if (config.max_formula) {
        if (config.max_formula === 'wisdom_mod + level') {
            const wisdom = character.stats?.wisdom || 10;
            const wisdomMod = Math.floor((wisdom - 10) / 2);
            const level = character.level || 1;
            return Math.max(1, wisdomMod + level);
        }
        // Add more formulas as needed
        logger.warn(`Unknown max formula: ${config.max_formula}`);
        return 10;
    }

    // Otherwise use static max
    return config.max || 10;
}

/**
 * Get current resource value for display
 * @param {Object} config - Resource config from getClassResource
 * @param {Object} character - Character data
 * @returns {number} Current resource value
 */
export function getCurrentResource(config, character) {
    // If it's mana type, use mana from character
    if (config.type === 'mana') {
        return character.mana || 0;
    }

    // For martial resources outside combat, show combat_start value
    const maxResource = calculateMaxResource(config, character);

    if (config.combat_start === 'max') {
        return maxResource;
    } else if (typeof config.combat_start === 'number') {
        return config.combat_start;
    }

    return maxResource;
}

/**
 * Generate CSS gradient string for resource bar
 * @param {Object} config - Resource config with color_gradient
 * @returns {string} CSS linear-gradient value
 */
export function getResourceGradient(config) {
    if (config.color_gradient) {
        const { start, mid, end } = config.color_gradient;
        return `linear-gradient(to bottom, ${start} 0%, ${mid} 50%, ${end} 100%)`;
    }
    // Fallback to solid color
    return config.color || '#2563eb';
}

// Synchronous getter for cached data (use after initial load)
export function getClassResourceSync(className) {
    if (!classResourcesCache) {
        logger.warn('Class resources not loaded yet, returning mana default');
        return DEFAULT_MANA_CONFIG;
    }

    const key = className?.toLowerCase();
    const config = classResourcesCache[key];

    if (!config) {
        return DEFAULT_MANA_CONFIG;
    }

    if (config.type === 'mana') {
        return { ...DEFAULT_MANA_CONFIG, ...config };
    }

    return config;
}

logger.debug('Class resources module loaded');
