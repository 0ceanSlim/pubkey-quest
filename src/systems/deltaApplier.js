/**
 * DeltaApplier - Surgical DOM update system
 *
 * Handles delta updates from the backend and applies them surgically to the DOM.
 * This is the ONLY place that modifies DOM in response to state changes,
 * eliminating full re-renders and preventing UI flickering.
 *
 * Delta format from backend:
 * {
 *   character: { hp, max_hp, fatigue, hunger, gold, time_of_day, current_day },
 *   npcs: { added: [], removed: [] },
 *   buildings: { state_changed: { building_id: isOpen } },
 *   inventory: { general_slots: {...}, backpack_slots: {...} },
 *   equipment: { changed: { slot_name: item_id } },
 *   location: { city, district, building },
 *   effects: { added: [], removed: [], updated: [] }
 * }
 */

import { logger } from '../lib/logger.js';
import { smoothClock } from './smoothClock.js';
import { eventBus } from '../lib/events.js';
import { getGameStateSync } from '../state/gameState.js';
import { effectsDisplay } from '../ui/effectsDisplay.js';
import { buildingButton } from '../ui/locationDisplay.js';
import { attachConditionBar, attachLightBar, attachWearBar, attachChargeBar } from '../ui/conditionBar.js';

class DeltaApplier {
    constructor() {
        // DOM element cache for faster updates
        this.elementCache = new Map();
        logger.debug('DeltaApplier initialized');
    }

    /**
     * Apply a delta object to the DOM
     * @param {object} delta - Delta object from backend
     */
    applyDelta(delta) {
        if (!delta) {
            logger.debug('No delta to apply');
            return;
        }

        logger.debug('Applying delta:', delta);

        try {
            if (delta.character) {
                this.applyCharacterDelta(delta.character);
            }
        } catch (e) {
            logger.error('Error applying character delta:', e);
        }

        try {
            if (delta.npcs) {
                this.applyNPCDelta(delta.npcs);
            }
        } catch (e) {
            logger.error('Error applying NPC delta:', e);
        }

        try {
            if (delta.buildings) {
                this.applyBuildingDelta(delta.buildings);
            }
        } catch (e) {
            logger.error('Error applying building delta:', e);
        }

        try {
            if (delta.inventory) {
                this.applyInventoryDelta(delta.inventory);
            }
        } catch (e) {
            logger.error('Error applying inventory delta:', e);
        }

        try {
            if (delta.equipment) {
                this.applyEquipmentDelta(delta.equipment);
            }
        } catch (e) {
            logger.error('Error applying equipment delta:', e);
        }

        try {
            if (delta.location) {
                this.applyLocationDelta(delta.location);
            }
        } catch (e) {
            logger.error('Error applying location delta:', e);
        }

        try {
            if (delta.effects) {
                this.applyEffectsDelta(delta.effects);
            }
        } catch (e) {
            logger.error('Error applying effects delta:', e);
        }

        try {
            if (delta.show_ready) {
                this.applyShowReadyDelta(delta.show_ready);
            }
        } catch (e) {
            logger.error('Error applying show_ready delta:', e);
        }
    }

    /**
     * Apply character stat changes
     * @param {object} charDelta
     */
    applyCharacterDelta(charDelta) {
        // HP
        if (charDelta.hp !== undefined) {
            this.updateElement('current-hp', charDelta.hp);
            this.updateHPBar(charDelta.hp, charDelta.max_hp);
        }
        if (charDelta.max_hp !== undefined) {
            this.updateElement('max-hp', charDelta.max_hp);
        }

        // Mana
        if (charDelta.mana !== undefined) {
            this.updateElement('current-mana', charDelta.mana);
        }
        if (charDelta.max_mana !== undefined) {
            this.updateElement('max-mana', charDelta.max_mana);
        }

        // Fatigue
        if (charDelta.fatigue !== undefined) {
            this.updateElement('fatigue-level', charDelta.fatigue);
            // Get accumulator from state for radial progress
            const state = getGameStateSync();
            const accumulators = effectsDisplay.getAccumulatorValues(state?.character?.active_effects);
            effectsDisplay.updateFatigueIcon(charDelta.fatigue, accumulators.fatigueAccumulator, accumulators.fatigueInterval);
        }

        // Hunger
        if (charDelta.hunger !== undefined) {
            this.updateElement('hunger-level', charDelta.hunger);
            // Get accumulator from state for radial progress
            const state = getGameStateSync();
            const accumulators = effectsDisplay.getAccumulatorValues(state?.character?.active_effects);
            effectsDisplay.updateHungerIcon(charDelta.hunger, accumulators.hungerAccumulator, accumulators.hungerInterval);
        }

        // Gold
        if (charDelta.gold !== undefined) {
            this.updateElement('gold-amount', charDelta.gold);
        }

        // XP
        // The XP bar, level and "+N XP" flair are drawn from cached state by the
        // character display, so update the cache and redraw: a discovery on the
        // road shows at once, not at the next full refresh (stopping to rest).
        if (charDelta.xp !== undefined) {
            const character = getGameStateSync()?.character;
            if (character && character.experience !== charDelta.xp) {
                character.experience = charDelta.xp;
                import('../ui/characterDisplay.js').then((m) => m.updateCharacterDisplay());
            }
        }

        // Time - sync to smooth clock
        if (charDelta.time_of_day !== undefined || charDelta.current_day !== undefined) {
            const timeOfDay = charDelta.time_of_day ?? smoothClock.getCurrentTime().timeOfDay;
            const currentDay = charDelta.current_day ?? smoothClock.getCurrentTime().currentDay;
            smoothClock.syncFromBackend(timeOfDay, currentDay);

            // Emit time changed event for other systems
            eventBus.emit('time:changed', { timeOfDay, currentDay });
        }
    }

    /**
     * Apply NPC changes (add/remove NPC buttons)
     * @param {object} npcDelta
     */
    applyNPCDelta(npcDelta) {
        logger.debug('applyNPCDelta called:', npcDelta);

        const container = document.querySelector('#npc-buttons > div');
        if (!container) {
            logger.warn('NPC container not found for delta update');
            return;
        }

        // Remove NPCs
        if (npcDelta.removed && npcDelta.removed.length > 0) {
            logger.debug(`Removing ${npcDelta.removed.length} NPCs:`, npcDelta.removed);
            npcDelta.removed.forEach(npcId => {
                const button = container.querySelector(`[data-npc-id="${npcId}"]`);
                if (button) {
                    button.remove();
                    logger.debug(`Removed NPC button: ${npcId}`);
                } else {
                    logger.warn(`NPC button not found to remove: ${npcId}`);
                }
            });
        }

        // Add NPCs
        if (npcDelta.added && npcDelta.added.length > 0) {
            logger.debug(`Adding ${npcDelta.added.length} NPCs:`, npcDelta.added);

            // Remove empty state message first (handles multiple possible formats)
            const emptyMsg = container.querySelector('.empty-message') ||
                             container.querySelector('.text-gray-400.italic');
            if (emptyMsg) {
                emptyMsg.remove();
                logger.debug('Removed empty message before adding NPCs');
            }

            npcDelta.added.forEach(npcId => {
                // Check if NPC button already exists (avoid duplicates)
                const existing = container.querySelector(`[data-npc-id="${npcId}"]`);
                if (existing) {
                    logger.debug(`NPC button already exists, skipping: ${npcId}`);
                    return;
                }

                // Try to get NPC name from static data, fallback to formatting the ID
                let displayName = npcId.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
                try {
                    // Use the getNPCById function if available
                    const getNPCById = window.getNPCById;
                    if (getNPCById) {
                        const npcData = getNPCById(npcId);
                        if (npcData && npcData.name) {
                            displayName = npcData.name;
                        }
                    }
                } catch (e) {
                    // Fallback to formatted ID
                }

                // Create NPC button using the same style as locationDisplay.js
                const button = this.createActionButton(displayName, () => {
                    if (window.talkToNPC) {
                        window.talkToNPC(npcId);
                    }
                }, 'npc');
                button.dataset.npcId = npcId;
                container.appendChild(button);
                logger.debug(`Added NPC button: ${npcId} (${displayName})`);
            });
        }

        // Update empty state after all changes
        this.updateEmptyState(container, 'No one here.');
    }

    /**
     * Apply building state changes (open/closed)
     * @param {object} buildingDelta
     */
    applyBuildingDelta(buildingDelta) {
        if (!buildingDelta.state_changed) return;

        for (const [buildingId, isOpen] of Object.entries(buildingDelta.state_changed)) {
            const button = document.querySelector(`[data-building-id="${buildingId}"]`);
            if (button?._building) {
                // A fresh button: the right label, and only its one handler
                // (patching the old one left its "closed" click in place).
                button.replaceWith(buildingButton(button._building, isOpen));
                logger.debug(`Building ${buildingId}: ${isOpen ? 'OPEN' : 'CLOSED'}`);
            }
        }
    }

    /**
     * Apply inventory slot changes
     * @param {object} inventoryDelta
     */
    applyInventoryDelta(inventoryDelta) {
        if (inventoryDelta.general_slots) {
            for (const [slotIndex, slotDelta] of Object.entries(inventoryDelta.general_slots)) {
                this.updateInventorySlot('general', parseInt(slotIndex), slotDelta);
            }
        }

        if (inventoryDelta.backpack_slots) {
            for (const [slotIndex, slotDelta] of Object.entries(inventoryDelta.backpack_slots)) {
                this.updateInventorySlot('backpack', parseInt(slotIndex), slotDelta);
            }
        }
    }

    /**
     * Update a single inventory slot
     * @param {string} type - 'general' or 'backpack'
     * @param {number} slotIndex - Slot index
     * @param {object} slotDelta - Slot changes
     */
    updateInventorySlot(type, slotIndex, slotDelta) {
        const selector = type === 'general'
            ? `[data-item-slot="${slotIndex}"]`
            : `[data-backpack-slot="${slotIndex}"]`;

        const slotDiv = document.querySelector(selector);
        if (!slotDiv) return;

        // Empty slot
        if (slotDelta.empty || (!slotDelta.item_id && !slotDelta.quantity)) {
            const imgContainer = slotDiv.querySelector('.item-image-container');
            const qtyLabel = slotDiv.querySelector('.quantity-label');
            imgContainer?.remove();
            qtyLabel?.remove();
            delete slotDiv.dataset.itemId;
            attachConditionBar(slotDiv, null);
            attachLightBar(slotDiv, null, null);
            attachWearBar(slotDiv, null, null);
            attachChargeBar(slotDiv, null, null);
            return;
        }

        // Get current item image
        const existingImg = slotDiv.querySelector('img');
        const existingQty = slotDiv.querySelector('.quantity-label');

        // Update or create image
        if (slotDelta.item_id) {
            const newSrc = `/res/img/items/${slotDelta.item_id}.png`;

            if (existingImg) {
                const currentSrc = existingImg.getAttribute('src');
                if (currentSrc !== newSrc) {
                    existingImg.src = newSrc;
                }
            } else {
                const imgDiv = document.createElement('div');
                imgDiv.className = 'item-image-container w-full h-full flex items-center justify-center p-1';
                const img = document.createElement('img');
                img.src = newSrc;
                img.className = 'w-full h-full object-contain';
                img.style.imageRendering = 'pixelated';
                imgDiv.appendChild(img);
                slotDiv.appendChild(imgDiv);
            }

            slotDiv.dataset.itemId = slotDelta.item_id;
            // Its condition bars: food, a lamp's oil, wear, charges.
            attachConditionBar(slotDiv, slotDelta.item_id, slotDelta.t);
            attachLightBar(slotDiv, slotDelta.item_id, slotDelta);
            attachWearBar(slotDiv, slotDelta.item_id, slotDelta);
            attachChargeBar(slotDiv, slotDelta.item_id, slotDelta);
        }

        // Update quantity
        if (slotDelta.quantity !== undefined) {
            if (slotDelta.quantity > 1) {
                if (existingQty) {
                    existingQty.textContent = slotDelta.quantity;
                } else {
                    const qtyLabel = document.createElement('div');
                    qtyLabel.className = 'quantity-label absolute bottom-0 right-0 text-white text-xs px-1 bg-black bg-opacity-50';
                    qtyLabel.textContent = slotDelta.quantity;
                    slotDiv.appendChild(qtyLabel);
                }
            } else {
                existingQty?.remove();
            }
        }
    }

    /**
     * Apply equipment slot changes
     * @param {object} equipmentDelta
     */
    applyEquipmentDelta(equipmentDelta) {
        if (!equipmentDelta.changed) return;

        for (const [slotName, itemId] of Object.entries(equipmentDelta.changed)) {
            const slotDiv = document.querySelector(`[data-slot="${slotName}"]`);
            if (!slotDiv) continue;

            const existingImg = slotDiv.querySelector('img');

            if (!itemId) {
                // Remove equipment
                existingImg?.parentElement?.remove();
                delete slotDiv.dataset.itemId;
            } else {
                // Add/update equipment
                const newSrc = `/res/img/items/${itemId}.png`;

                if (existingImg) {
                    existingImg.src = newSrc;
                } else {
                    const imgDiv = document.createElement('div');
                    imgDiv.className = 'w-full h-full flex items-center justify-center p-1';
                    const img = document.createElement('img');
                    img.src = newSrc;
                    img.className = 'w-full h-full object-contain';
                    img.style.imageRendering = 'pixelated';
                    imgDiv.appendChild(img);
                    slotDiv.appendChild(imgDiv);
                }

                slotDiv.dataset.itemId = itemId;
            }
        }
    }

    /**
     * Apply location changes
     * @param {object} locationDelta
     */
    applyLocationDelta(locationDelta) {
        // Location changes typically require more complex UI updates
        // Emit an event for the location system to handle
        eventBus.emit('location:changed', locationDelta);
    }

    /**
     * Apply effects changes
     * @param {object} effectsDelta
     */
    applyEffectsDelta(effectsDelta) {
        // Effects changes affect the effects display
        eventBus.emit('effects:changed', effectsDelta);
    }

    /**
     * Apply show ready changes (show/hide Play Show button)
     * @param {object} showReadyDelta - { is_ready: boolean, building_id: string }
     */
    applyShowReadyDelta(showReadyDelta) {
        const isReady = showReadyDelta.is_ready;
        const buildingId = showReadyDelta.building_id;

        logger.info('Applying show_ready delta:', { isReady, buildingId });

        // Find existing Play Show button (look for button with "Play Show" text)
        const existingButton = document.querySelector('[data-action="play-show"]') ||
                               Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Play Show');

        logger.debug('Existing Play Show button:', { exists: !!existingButton });

        if (isReady) {
            // Need to show the button - only if player is in the correct building
            // Get current building from game state
            const state = getGameStateSync();
            let currentBuilding = state.location?.building || '';

            // Find the Exit Building button - used both to check if in building and to find container
            const exitBtn = Array.from(document.querySelectorAll('button')).find(b =>
                b.textContent === 'Exit Building');
            const isInAnyBuilding = !!exitBtn;

            logger.info('Show ready check:', {
                currentBuilding,
                buildingId,
                isInBuilding: currentBuilding === buildingId,
                exitButtonFound: isInAnyBuilding,
                fullLocation: state.location
            });

            // If we're in a building (Exit button exists) but game state doesn't have it, use the buildingId
            // This handles cases where the state might be stale
            if (isInAnyBuilding && !currentBuilding && buildingId) {
                logger.info('Using buildingId from delta as current building (state was stale)');
                currentBuilding = buildingId;
            }

            if (currentBuilding !== buildingId) {
                logger.debug('Player not in show building:', { currentBuilding, buildingId });
                // If there's an existing button but we're not in the right building, remove it
                if (existingButton) {
                    existingButton.remove();
                }
                return;
            }

            // Only add button if it doesn't exist
            if (!existingButton) {
                logger.debug('Adding Play Show button to building');

                // Use the exit button we already found
                const container = exitBtn?.parentElement;

                logger.debug('Exit button search:', { found: !!exitBtn, exitText: exitBtn?.textContent, containerFound: !!container });

                if (container) {
                    const button = document.createElement('button');
                    // Match Win95-style from createLocationButton with 'action' type
                    button.className = 'text-white transition-all leading-tight text-center overflow-hidden';
                    button.style.fontSize = '7px';
                    button.style.background = '#9e8b6b'; // Muted gold for action buttons
                    button.style.color = '#ffffff';
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
                    button.textContent = 'Play Show';
                    button.dataset.action = 'play-show';
                    button.addEventListener('click', async () => {
                        // Auto-play time when taking action
                        if (window.timeClock && window.timeClock.play) {
                            window.timeClock.play();
                        }
                        // Import and call performShow
                        const { performShow } = await import('../ui/locationDisplay.js');
                        performShow();
                    });
                    container.appendChild(button);
                    logger.info('Play Show button added via delta');
                } else {
                    logger.warn('Could not find container for Play Show button (no Exit button found)');
                }
            } else {
                logger.debug('Play Show button already exists');
            }
        } else {
            // Need to hide the button
            if (existingButton) {
                logger.info('Removing Play Show button via delta');
                existingButton.remove();
            }
        }
    }

    // --- Helper Methods ---

    /**
     * Update a DOM element by ID
     * @param {string} id - Element ID
     * @param {*} value - New value
     */
    updateElement(id, value) {
        let el = this.elementCache.get(id);
        if (!el) {
            el = document.getElementById(id);
            if (el) {
                this.elementCache.set(id, el);
            }
        }
        if (el && el.textContent !== String(value)) {
            el.textContent = value;
        }
    }

    /**
     * Update HP bar width
     * @param {number} hp
     * @param {number} maxHp
     */
    updateHPBar(hp, maxHp) {
        const hpBar = document.getElementById('hp-bar');
        if (!hpBar) return;

        // Get max HP from DOM if not provided
        if (maxHp === undefined) {
            const maxHpEl = document.getElementById('max-hp');
            maxHp = maxHpEl ? parseInt(maxHpEl.textContent) : 10;
        }

        const percentage = Math.max(0, Math.min(100, (hp / maxHp) * 100));
        hpBar.style.width = `${percentage}%`;

        // Update color based on percentage
        if (percentage <= 25) {
            hpBar.className = hpBar.className.replace(/bg-\w+-\d+/g, '') + ' bg-red-600';
        } else if (percentage <= 50) {
            hpBar.className = hpBar.className.replace(/bg-\w+-\d+/g, '') + ' bg-yellow-500';
        } else {
            hpBar.className = hpBar.className.replace(/bg-\w+-\d+/g, '') + ' bg-green-600';
        }
    }

    /**
     * Update fatigue icon and radial progress
     * @param {number} fatigue
     * @param {Array} activeEffects - Active effects for accumulator lookup
     */
    updateFatigueDisplay(fatigue, activeEffects) {
        const accumulators = effectsDisplay.getAccumulatorValues(activeEffects);
        effectsDisplay.updateFatigueIcon(fatigue, accumulators.fatigueAccumulator, accumulators.fatigueInterval);
    }

    /**
     * Update hunger icon and radial progress
     * @param {number} hunger
     * @param {Array} activeEffects - Active effects for accumulator lookup
     */
    updateHungerDisplay(hunger, activeEffects) {
        const accumulators = effectsDisplay.getAccumulatorValues(activeEffects);
        effectsDisplay.updateHungerIcon(hunger, accumulators.hungerAccumulator, accumulators.hungerInterval);
    }

    /**
     * Update empty state message for a container
     * @param {Element} container
     * @param {string} message
     */
    updateEmptyState(container, message) {
        // Find empty message (handles multiple possible class formats)
        const emptyMsg = container.querySelector('.empty-message') ||
                         container.querySelector('.text-gray-400.italic');

        // Count real children (excluding empty message)
        const realChildren = Array.from(container.children).filter(child => {
            return !child.classList.contains('empty-message') &&
                   !(child.classList.contains('text-gray-400') && child.classList.contains('italic'));
        });
        const hasChildren = realChildren.length > 0;

        if (hasChildren && emptyMsg) {
            emptyMsg.remove();
        } else if (!hasChildren && !emptyMsg) {
            const msgDiv = document.createElement('div');
            // Match the style from locationDisplay.js for consistency
            msgDiv.className = 'empty-message text-gray-400 text-xs p-2 text-center italic';
            msgDiv.textContent = message;
            container.appendChild(msgDiv);
        }
    }

    /**
     * Create an action button (consistent with locationDisplay.js createLocationButton style)
     * @param {string} text
     * @param {function} onClick
     * @param {string} type - 'npc', 'building', 'action'
     * @returns {HTMLButtonElement}
     */
    createActionButton(text, onClick, type = 'action') {
        const button = document.createElement('button');

        // Match locationDisplay.js createLocationButton styling exactly
        const typeStyles = {
            navigation: '#6b7a9e',      // City districts - muted blue
            environment: '#9e6b6b',     // Outside city - muted red
            building: '#6b8e6b',        // Open buildings - muted green
            'building-closed': '#808080', // Closed buildings - grey
            npc: '#8b6b9e',             // NPCs - muted purple
            action: '#9e8b6b'           // Special actions - muted gold
        };

        const bgColor = typeStyles[type] || typeStyles.action;
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
        button.onclick = onClick;
        return button;
    }

    /**
     * Clear the element cache (call when DOM is rebuilt)
     */
    clearCache() {
        this.elementCache.clear();
    }
}

// Create singleton instance
export const deltaApplier = new DeltaApplier();

logger.debug('DeltaApplier module loaded');
