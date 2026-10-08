/**
 * World Handoff
 *
 * The server can interrupt the player on the back of *any* action: a biome
 * monster while travelling, an authored encounter when you've been in a tavern
 * a while, a district pickpocket. When it does, it has already committed that
 * state server-side — combat is live, or a POI walk is active — so the client
 * must open the matching overlay or the player is left standing in a world that
 * has quietly moved on without them.
 *
 * This is the one place that handoff happens. It used to live only in the world
 * tick, so encounters fired by a `move` or `enter_building` response were
 * silently dropped: the one-shot was burnt, the cooldown spent, and the session
 * left with an invisible active encounter that blocked every later one.
 *
 * @module systems/worldHandoff
 */

import { logger } from '../lib/logger.js';
import { eventBus } from '../lib/events.js';

/**
 * Open the overlay for any fight or encounter the server just started.
 *
 * @param {Object} data - An action response's `data` payload
 * @returns {boolean} True if a handoff happened, meaning the caller should stop
 *   processing this response — whatever else it carried is now stale.
 */
export function handleWorldHandoff(data) {
    if (!data) return false;

    // A fight started (travel encounter, or a monster node inside a POI walk).
    // enterCombatMode pauses the clock.
    if (data.combat_started && data.combat) {
        logger.info('⚔️ World handoff — entering combat');
        eventBus.emit('combat:started', data.combat);
        return true;
    }

    // An authored encounter (vignette) started — open the exploration overlay on
    // its first node. The walk is already active server-side; advances flow
    // through /poi/advance.
    if (data.encounter_started && data.poi_step) {
        logger.info('✨ World handoff — opening encounter overlay');
        window.showMessage?.(`✨ ${data.encounter_name || 'Something happens…'}`, 'info');
        import('../ui/poiExplore.js')
            .then((m) => m.openFromStep(data.poi_step))
            .catch((e) => logger.error('encounter overlay open failed:', e));
        return true;
    }

    return false;
}
