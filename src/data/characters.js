/**
 * Character & Vault Data Module
 *
 * Handles character-related data operations and location display name lookups.
 *
 * Vault setup is not here: the vault is one shared, server-owned store, and a
 * new character's home keeper is registered server-side at creation
 * (cmd/server/api/character/create.go).
 *
 * @module data/characters
 */

import { logger } from '../lib/logger.js';
import { API_BASE_URL } from '../config/constants.js';

/**
 * Convert location/district/building IDs to display names
 * @param {string} locationId - Location ID
 * @param {string} districtKey - District key
 * @param {string} buildingId - Building ID
 * @returns {Promise<Object>} Object with display names { location, district, building }
 */
export async function getDisplayNamesForLocation(locationId, districtKey, buildingId) {
    try {
        const response = await fetch(`${API_BASE_URL}/locations`);
        if (!response.ok) {
            logger.warn('Failed to fetch locations from API');
            return { location: locationId, district: districtKey, building: buildingId };
        }

        const allLocations = await response.json();

        // Find the location
        const location = allLocations.find(loc => loc.id === locationId);
        if (!location) {
            logger.warn(`Location not found: ${locationId}`);
            return { location: locationId, district: districtKey, building: buildingId };
        }

        // Find the district
        const district = location.properties?.districts?.[districtKey];
        if (!district) {
            logger.warn(`District not found: ${districtKey} in ${locationId}`);
            return {
                location: location.name || locationId,
                district: districtKey,
                building: buildingId
            };
        }

        // Find the building
        const building = district.buildings?.find(b => b.id === buildingId);
        if (!building) {
            logger.warn(`Building not found: ${buildingId} in ${districtKey}`);
            return {
                location: location.name || locationId,
                district: district.name || districtKey,
                building: buildingId
            };
        }

        return {
            location: location.name || locationId,
            district: district.name || districtKey,
            building: building.name || buildingId
        };

    } catch (error) {
        logger.error('Error fetching location names:', error);
        return { location: locationId, district: districtKey, building: buildingId };
    }
}

logger.debug('Character data module loaded');
