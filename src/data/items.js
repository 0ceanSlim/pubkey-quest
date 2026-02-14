/**
 * Item Data Module
 *
 * Handles loading and caching item data from the database API.
 * Provides ASYNC utilities for item lookups and transformations.
 *
 * Use this module for database queries. For synchronous DOM-cached lookups,
 * see state/staticData.js (transitional).
 *
 * @module data/items
 */

import { logger } from '../lib/logger.js';
import { API_BASE_URL } from '../config/constants.js';

// Shared cache for item database
let itemsDatabaseCache = null;

/**
 * Load all items from the database API
 * Uses caching to avoid repeated API calls
 * @returns {Promise<Array>} Array of item objects
 */
export async function loadItemsFromDatabase() {
    if (itemsDatabaseCache) {
        return itemsDatabaseCache;
    }

    try {
        const response = await fetch(`${API_BASE_URL}/items`);
        if (response.ok) {
            itemsDatabaseCache = await response.json();
            logger.info(`Loaded ${itemsDatabaseCache.length} items from database`);
            return itemsDatabaseCache;
        }
    } catch (error) {
        logger.warn("Could not load items from database:", error);
    }
    return [];
}

/**
 * Get item data from database cache by ID
 * @param {string} itemId - The item ID to look up
 * @returns {Promise<Object|null>} Item object or null if not found
 */
export async function getItemById(itemId) {
    try {
        const items = await loadItemsFromDatabase();

        // Find item by ID (exact match)
        const item = items.find((i) => i.id === itemId);

        if (item) {
            // Convert database format to expected frontend format
            return {
                id: item.id,
                name: item.name,
                description: item.description,
                type: item.item_type,
                tags: item.tags || [],
                rarity: item.rarity,
                gear_slot: item.properties?.gear_slot,
                slots: item.properties?.slots,
                contents: item.properties?.contents,
                stack: item.properties?.stack,
                ...item.properties, // Spread all other properties
            };
        } else {
            logger.warn(`Item ID "${itemId}" not found in database`);
        }
    } catch (error) {
        logger.warn(`Could not load item data for ID: ${itemId}`, error);
    }
    return null;
}

/**
 * Get formatted HTML stats for an item
 * @param {string} itemName - The item name to fetch stats for
 * @returns {Promise<string>} HTML string with formatted item stats
 */
export async function getItemStats(itemName) {
    try {
        const response = await fetch(
            `${API_BASE_URL}/items?name=${encodeURIComponent(itemName)}`
        );

        if (!response.ok) {
            return `<div class="font-bold text-yellow-400 mb-1">${itemName}</div><div class="text-gray-400 text-xs">No details available</div>`;
        }

        const items = await response.json();

        if (!items || items.length === 0) {
            return `<div class="font-bold text-yellow-400 mb-1">${itemName}</div><div class="text-gray-400 text-xs">No details available</div>`;
        }

        const itemData = items[0];
        const props = itemData.properties || {};

        let statsHTML = `<div class="font-bold text-yellow-400 mb-2 text-xl">${itemData.name}</div>`;

        if (itemData.item_type) {
            statsHTML += `<div class="text-green-400 text-sm font-semibold mb-3">${itemData.item_type}</div>`;
        }

        // Check if this is a focus item
        const isFocus =
            itemData.item_type === "Arcane Focus" ||
            itemData.item_type === "Druidic Focus" ||
            itemData.item_type === "Holy Symbol";

        // If it's a focus, show the component it provides prominently
        if (isFocus && props.provides) {
            statsHTML += `<div class="bg-purple-900 bg-opacity-40 border-2 border-purple-500 rounded-lg p-3 mb-3">`;
            statsHTML += `<div class="text-purple-300 text-xs font-semibold mb-2">✨ Provides Unlimited:</div>`;
            statsHTML += `<div class="flex items-center gap-2">`;
            statsHTML += `<img src="/res/img/items/${props.provides}.png" class="w-8 h-8 object-contain" style="image-rendering: pixelated; image-rendering: -moz-crisp-edges; image-rendering: crisp-edges;" onerror="this.style.display='none'">`;
            statsHTML += `<div class="text-white font-semibold">${props.provides
                .split("-")
                .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
                .join(" ")}</div>`;
            statsHTML += `</div>`;
            statsHTML += `</div>`;
        }

        // Stats section
        statsHTML += `<div class="space-y-1 mb-3">`;

        if (props.damage) {
            statsHTML += `<div class="text-gray-300 text-sm">⚔️ Damage: ${props.damage
                } ${props["damage-type"] || ""}</div>`;
        }

        if (props.ac) {
            statsHTML += `<div class="text-gray-300 text-sm">🛡️ AC: ${props.ac}</div>`;
        }

        if (props.weight) {
            statsHTML += `<div class="text-gray-300 text-sm">⚖️ Weight: ${props.weight} lb</div>`;
        }

        statsHTML += `</div>`;

        // Add tags
        if (itemData.tags && itemData.tags.length > 0) {
            statsHTML += `<div class="flex flex-wrap gap-1 mb-3">`;
            statsHTML += itemData.tags
                .map(
                    (tag) =>
                        `<span class="bg-gray-700 px-2 py-1 rounded text-xs text-gray-300">${tag}</span>`
                )
                .join("");
            statsHTML += `</div>`;
        }

        // Add full description
        if (itemData.description) {
            statsHTML += `<div class="text-gray-300 text-sm mt-3 leading-relaxed border-t border-gray-600 pt-3">${itemData.description}</div>`;
        }

        // Add notes for focuses
        if (isFocus && props.notes && props.notes.length > 0) {
            statsHTML += `<div class="text-purple-300 text-xs mt-3 leading-relaxed border-t border-purple-600 pt-3">`;
            props.notes.forEach((note) => {
                statsHTML += `<div class="mb-1">• ${note}</div>`;
            });
            statsHTML += `</div>`;
        }

        return statsHTML;
    } catch (error) {
        logger.error("Error fetching item:", itemName, error);
        return `<div class="font-bold text-yellow-400 mb-1">${itemName}</div><div class="text-gray-400 text-xs">Error loading details</div>`;
    }
}

logger.debug('Items data module loaded');
