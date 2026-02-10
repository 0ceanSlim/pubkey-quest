/**
 * Character Display UI Module
 *
 * Handles all character display updates including stats, equipment, inventory,
 * HP/mana/XP bars, fatigue, hunger, and weight calculations.
 *
 * @module ui/characterDisplay
 */

import { logger } from '../lib/logger.js';
import { getGameStateSync } from '../state/gameState.js';
import { loadItemsFromDatabase } from '../data/items.js';
import { eventBus } from '../lib/events.js';
import {
    getClassResource,
    calculateMaxResource,
    getCurrentResource,
    getResourceGradient,
    loadClassResources
} from '../data/classResources.js';

// Advancement data cache
let advancementDataCache = null;

/**
 * Load advancement data from JSON
 * @returns {Promise<Array>} Advancement data array
 */
async function loadAdvancementData() {
    if (advancementDataCache) {
        return advancementDataCache;
    }

    try {
        const response = await fetch('/data/systems/advancement.json');
        if (!response.ok) {
            throw new Error('Failed to fetch advancement data');
        }
        advancementDataCache = await response.json();
        return advancementDataCache;
    } catch (error) {
        logger.error('Error loading advancement data:', error);
        return [];
    }
}

/**
 * Get item data from database cache by ID
 * @param {string} itemId - Item ID to look up
 * @returns {Promise<Object|null>} Item data or null
 */
async function getItemByIdAsync(itemId) {
    try {
        const items = await loadItemsFromDatabase();
        const item = items.find(i => i.id === itemId);
        return item || null;
    } catch (error) {
        logger.error(`Error getting item ${itemId}:`, error);
        return null;
    }
}

/**
 * Calculate max weight capacity based on strength and equipped items
 * @param {Object} character - Character data
 * @returns {Promise<number>} Max capacity in lbs
 */
export async function calculateMaxCapacity(character) {
    // Check if backend provided pre-calculated capacity
    if (character.weight_capacity !== undefined) {
        return character.weight_capacity;
    }

    // Fallback to basic calculation (for old saves or if backend didn't provide)
    const strength = character.stats?.strength || 10;
    return strength * 5;
}

/**
 * Calculate total weight of all items in inventory
 * @param {Object} character - Character data
 * @returns {Promise<number>} Total weight in lbs
 */
export async function calculateAndDisplayWeight(character) {
    // Check if backend provided pre-calculated weight
    if (character.total_weight !== undefined) {
        return character.total_weight;
    }

    // Fallback: No calculation needed - backend should always provide this
    // Return 0 if not available (shouldn't happen in normal operation)
    return 0;
}

/**
 * Update the resource bar (mana/stamina/rage/ki/cunning) based on character class
 * @param {Object} character - Character data
 */
async function updateResourceBar(character) {
    const resourceConfig = await getClassResource(character.class);

    const currentResourceEl = document.getElementById('current-mana');
    const maxResourceEl = document.getElementById('max-mana');
    const resourceBarEl = document.getElementById('mana-bar');
    const resourceLabelEl = document.getElementById('resource-label');

    // Calculate values
    const maxResource = calculateMaxResource(resourceConfig, character);
    const currentResource = getCurrentResource(resourceConfig, character);
    const percentage = maxResource > 0 ? (currentResource / maxResource * 100) : 0;

    // Update values
    if (currentResourceEl) currentResourceEl.textContent = currentResource;
    if (maxResourceEl) maxResourceEl.textContent = maxResource;

    // Update bar width and color
    if (resourceBarEl) {
        resourceBarEl.style.width = percentage + '%';
        resourceBarEl.style.background = getResourceGradient(resourceConfig);
    }

    // Update label (if element exists)
    if (resourceLabelEl) {
        resourceLabelEl.textContent = resourceConfig.short_label || 'MP';
    }

    logger.debug(`Resource bar updated for ${character.class}: ${currentResource}/${maxResource} (${resourceConfig.type})`);
}

/**
 * Update full character display from save data
 */
export async function updateCharacterDisplay() {
    logger.debug('📊 updateCharacterDisplay called');
    const state = getGameStateSync();
    const character = state.character;

    if (!character) {
        logger.error('❌ No character data found!');
        return;
    }

    logger.debug('✓ Character data:', {
        fatigue: character.fatigue,
        hunger: character.hunger,
        hp: character.hp
    });

    // Update character info
    if (document.getElementById('char-name')) {
        document.getElementById('char-name').textContent = character.name || '-';
    }
    if (document.getElementById('char-race')) {
        document.getElementById('char-race').textContent = character.race || '-';
    }
    if (document.getElementById('char-class')) {
        document.getElementById('char-class').textContent = character.class || '-';
    }
    if (document.getElementById('char-background')) {
        document.getElementById('char-background').textContent = character.background || '-';
    }
    if (document.getElementById('char-alignment')) {
        document.getElementById('char-alignment').textContent = character.alignment || '-';
    }
    if (document.getElementById('char-level')) {
        document.getElementById('char-level').textContent = character.level || 1;
    }
    if (document.getElementById('char-gold')) {
        document.getElementById('char-gold').textContent = character.gold || 0;
    }

    // Update XP bar
    const currentExpEl = document.getElementById('current-exp');
    const expToNextEl = document.getElementById('exp-to-next');
    const expBarEl = document.getElementById('exp-bar');

    const advancementData = await loadAdvancementData();
    const currentXP = character.experience || 0;
    const currentLevel = character.level || 1;

    // Find current and next level data
    const currentLevelData = advancementData.find(l => l.Level === currentLevel);
    const nextLevelData = advancementData.find(l => l.Level === currentLevel + 1);

    if (currentExpEl && expToNextEl && expBarEl && currentLevelData) {
        const currentLevelXP = currentLevelData.ExperiencePoints;
        const nextLevelXP = nextLevelData ? nextLevelData.ExperiencePoints : currentLevelXP;
        const xpIntoLevel = currentXP - currentLevelXP;
        const xpNeededForLevel = nextLevelXP - currentLevelXP;

        currentExpEl.textContent = xpIntoLevel.toLocaleString();
        expToNextEl.textContent = xpNeededForLevel.toLocaleString();

        const expPercentage = xpNeededForLevel > 0 ? (xpIntoLevel / xpNeededForLevel * 100) : 0;
        expBarEl.style.width = Math.min(100, Math.max(0, expPercentage)) + '%';
    }

    // Update HP display
    const currentHpEl = document.getElementById('current-hp');
    const maxHpEl = document.getElementById('max-hp');
    const hpBarEl = document.getElementById('hp-bar');
    if (currentHpEl) currentHpEl.textContent = character.hp || 0;
    if (maxHpEl) maxHpEl.textContent = character.max_hp || 0;
    if (hpBarEl) {
        const hpPercentage = (character.max_hp > 0) ? (character.hp / character.max_hp * 100) : 0;
        hpBarEl.style.width = hpPercentage + '%';
    }

    // Update resource display (mana for casters, stamina/rage/ki/cunning for martials)
    await updateResourceBar(character);

    // Update quick status (main bar - icons with radial progress)
    const fatigue = Math.min(character.fatigue || 0, 10);
    const hunger = Math.max(0, Math.min(character.hunger !== undefined ? character.hunger : 2, 3));

    // Fatigue number
    const fatigueLevelEl = document.getElementById('fatigue-level');
    if (fatigueLevelEl) fatigueLevelEl.textContent = fatigue;

    // Update fatigue icon via effectsDisplay
    try {
        const { effectsDisplay } = await import('./effectsDisplay.js');
        const accumulators = effectsDisplay.getAccumulatorValues(character.active_effects);
        effectsDisplay.updateFatigueIcon(fatigue, accumulators.fatigueAccumulator, accumulators.fatigueInterval);
        effectsDisplay.updateHungerIcon(hunger, accumulators.hungerAccumulator, accumulators.hungerInterval);

        // Render active effects in the effects display area
        effectsDisplay.renderEffects(character.active_effects);
    } catch (e) {
        logger.debug('EffectsDisplay not yet available:', e);
    }

    // Hunger number
    const hungerLevelEl = document.getElementById('hunger-level');
    if (hungerLevelEl) hungerLevelEl.textContent = hunger;

    // Weight numbers (integers only, no decimals)
    const weightEl = document.getElementById('char-weight');
    const maxWeightEl = document.getElementById('max-weight');

    if (weightEl || maxWeightEl) {
        Promise.all([
            calculateAndDisplayWeight(character),
            calculateMaxCapacity(character)
        ]).then(([weight, maxCapacity]) => {
            if (weightEl) weightEl.textContent = Math.round(weight);
            if (maxWeightEl) maxWeightEl.textContent = Math.round(maxCapacity);
        });
    }

    // Update detailed stats tab (if visible)
    updateStatsTab(character);

    // Update stats
    if (character.stats) {
        const statStrEl = document.getElementById('stat-str');
        const statDexEl = document.getElementById('stat-dex');
        const statConEl = document.getElementById('stat-con');
        const statIntEl = document.getElementById('stat-int');
        const statWisEl = document.getElementById('stat-wis');
        const statChaEl = document.getElementById('stat-cha');

        if (statStrEl) statStrEl.textContent = character.stats.strength || 10;
        if (statDexEl) statDexEl.textContent = character.stats.dexterity || 10;
        if (statConEl) statConEl.textContent = character.stats.constitution || 10;
        if (statIntEl) statIntEl.textContent = character.stats.intelligence || 10;
        if (statWisEl) statWisEl.textContent = character.stats.wisdom || 10;
        if (statChaEl) statChaEl.textContent = character.stats.charisma || 10;
    }

    // Update equipment slots
    if (character.inventory && character.inventory.gear_slots) {
        const gear = character.inventory.gear_slots;
        const slots = ['neck', 'head', 'ammo', 'mainhand', 'chest', 'offhand', 'ring1', 'legs', 'ring2', 'gloves', 'boots', 'bag'];

        // Use for...of instead of forEach to properly handle async
        for (const slotName of slots) {
            const slotEl = document.querySelector(`[data-slot="${slotName}"]`);
            if (slotEl) {
                const itemId = gear[slotName]?.item;
                const quantity = gear[slotName]?.quantity || 1;

                if (itemId) {
                    // Add data-item-id attribute to the slot for interaction system
                    slotEl.setAttribute('data-item-id', itemId);

                    // Fetch item data
                    const itemData = await getItemByIdAsync(itemId);

                    if (itemData) {
                        // Replace placeholder with item image
                        const imageContainer = slotEl.querySelector('.w-10.h-10');
                        if (imageContainer) {
                            const img = document.createElement('img');
                            img.src = `/res/img/items/${itemId}.png`;
                            img.alt = itemData.name;
                            img.className = 'w-full h-full object-contain';
                            img.style.imageRendering = 'pixelated';
                            img.onerror = function() {
                                if (!this.dataset.fallbackAttempted) {
                                    this.dataset.fallbackAttempted = 'true';
                                    this.src = '/res/img/items/unknown.png';
                                }
                            };
                            imageContainer.innerHTML = '';
                            imageContainer.appendChild(img);
                        }

                        // Add quantity label if > 1 (for ammunition, potions, etc.)
                        // First remove any existing quantity label
                        const existingLabel = slotEl.querySelector('.equipment-quantity-label');
                        if (existingLabel) {
                            existingLabel.remove();
                        }

                        if (quantity > 1) {
                            const quantityLabel = document.createElement('div');
                            quantityLabel.className = 'equipment-quantity-label absolute bottom-0 right-0 text-white';
                            quantityLabel.style.fontSize = '10px';
                            quantityLabel.textContent = `${quantity}`;
                            slotEl.appendChild(quantityLabel);
                        }
                    }
                } else {
                    // Remove data-item-id attribute if slot is empty
                    slotEl.removeAttribute('data-item-id');

                    // Remove quantity label if present
                    const existingLabel = slotEl.querySelector('.equipment-quantity-label');
                    if (existingLabel) {
                        existingLabel.remove();
                    }

                    // Reset to placeholder if empty
                    const imageContainer = slotEl.querySelector('.w-10.h-10');
                    if (imageContainer) {
                        // Check if placeholder exists
                        let placeholderIcon = slotEl.querySelector('.placeholder-icon');
                        if (placeholderIcon) {
                            // Placeholder exists, make sure it's visible
                            placeholderIcon.style.display = 'block';
                            // Remove any item image
                            const itemImg = imageContainer.querySelector('img');
                            if (itemImg) {
                                itemImg.remove();
                            }
                        } else {
                            // No placeholder found, just clear the image
                            const itemImg = imageContainer.querySelector('img');
                            if (itemImg) {
                                itemImg.remove();
                            }
                        }
                    }
                }
            }
        }
    }

    // Update general slots (4x1 grid) - ALWAYS create slots even if empty
    const generalSlotsDiv = document.getElementById('general-slots');
    if (generalSlotsDiv) {
        generalSlotsDiv.innerHTML = '';
        // Set grid styles directly
        generalSlotsDiv.style.cssText = 'display: grid; grid-template-columns: repeat(4, 28px); grid-template-rows: 28px; gap: 2px; justify-content: center;';

        // Ensure inventory structure exists
        if (!character.inventory) {
            character.inventory = {};
        }
        if (!character.inventory.general_slots) {
            character.inventory.general_slots = [];
        }

        // Create a map of slot index to item data (respecting the "slot" field)
        const slotMap = {};
        character.inventory.general_slots.forEach(item => {
            if (item && item.item) {
                const slotIndex = item.slot;
                // Only use valid slot indices (0-3)
                if (slotIndex >= 0 && slotIndex < 4) {
                    slotMap[slotIndex] = item;
                }
            }
        });

        // Create all 4 general slots
        for (let i = 0; i < 4; i++) {
            const slot = slotMap[i];
            const slotDiv = document.createElement('div');
            slotDiv.className = 'relative cursor-pointer hover:bg-gray-600 flex items-center justify-center';
            slotDiv.style.cssText = `width: 100%; height: 100%; box-sizing: border-box; overflow: hidden; background: #2a2a2a; border-top: 2px solid #1a1a1a; border-left: 2px solid #1a1a1a; border-right: 2px solid #4a4a4a; border-bottom: 2px solid #4a4a4a; clip-path: polygon(3px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0 calc(100% - 3px), 0 3px);`;

            // Add data attributes for interaction system
            slotDiv.setAttribute('data-item-slot', i);

            if (slot && slot.item) {
                slotDiv.setAttribute('data-item-id', slot.item);

                // Create image container
                const imgDiv = document.createElement('div');
                imgDiv.className = 'w-full h-full flex items-center justify-center p-1';
                const img = document.createElement('img');
                img.src = `/res/img/items/${slot.item}.png`;
                img.alt = slot.item;
                img.className = 'w-full h-full object-contain';
                img.style.imageRendering = 'pixelated';
                img.onerror = function() {
                    if (!this.dataset.fallbackAttempted) {
                        this.dataset.fallbackAttempted = 'true';
                        this.src = '/res/img/items/unknown.png';
                    }
                };
                imgDiv.appendChild(img);
                slotDiv.appendChild(imgDiv);

                // Add quantity label if > 1
                if (slot.quantity > 1) {
                    const quantityLabel = document.createElement('div');
                    quantityLabel.className = 'absolute bottom-0 right-0 text-white';
                    quantityLabel.style.fontSize = '10px';
                    quantityLabel.textContent = `${slot.quantity}`;
                    slotDiv.appendChild(quantityLabel);
                }
            }

            generalSlotsDiv.appendChild(slotDiv);
        }
    }

    // Update backpack items (4x5 grid = 20 slots) - ONLY show if bag is equipped
    const backpackDiv = document.getElementById('backpack-slots');
    if (backpackDiv) {
        backpackDiv.innerHTML = '';
        // Set grid styles directly
        backpackDiv.style.cssText = 'display: grid; grid-template-columns: repeat(4, 28px); grid-template-rows: repeat(5, 28px); gap: 2px; justify-content: center;';

        // Check if a bag is actually equipped
        const bagEquipped = character.inventory?.gear_slots?.bag?.item;

        if (!bagEquipped) {
            // No bag equipped - hide the backpack div
            if (backpackDiv.parentElement) {
                backpackDiv.parentElement.style.display = 'none';
            }
            // Don't return early - continue to bindInventoryEvents() at end of function
        } else {
            // Bag is equipped - show the backpack div
            if (backpackDiv.parentElement) {
                backpackDiv.parentElement.style.display = 'grid';
            }

            // Get or initialize contents
            if (!character.inventory.gear_slots.bag.contents) {
                character.inventory.gear_slots.bag.contents = [];
            }

            const contents = character.inventory.gear_slots.bag.contents;

            // Create a map of slot index to item data (respecting the "slot" field)
            const bagSlotMap = {};
            contents.forEach(item => {
                if (item && item.item) {
                    const slotIndex = item.slot;
                    // Only use valid slot indices (0-19 for 20-slot backpack)
                    if (slotIndex >= 0 && slotIndex < 20) {
                        bagSlotMap[slotIndex] = item;
                    }
                }
            });

            let itemCount = 0;

            // Create all 20 backpack slots
            for (let i = 0; i < 20; i++) {
                const slot = bagSlotMap[i];
                const slotDiv = document.createElement('div');
                slotDiv.className = 'relative cursor-pointer hover:bg-gray-800 flex items-center justify-center';
                slotDiv.style.cssText = `width: 100%; height: 100%; box-sizing: border-box; overflow: hidden; background: #1a1a1a; border-top: 2px solid #000000; border-left: 2px solid #000000; border-right: 2px solid #3a3a3a; border-bottom: 2px solid #3a3a3a; clip-path: polygon(3px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0 calc(100% - 3px), 0 3px);`;

                // Add data attributes for interaction system
                slotDiv.setAttribute('data-item-slot', i);

                if (slot && slot.item) {
                    itemCount++;
                    slotDiv.setAttribute('data-item-id', slot.item);

                    // Create image container
                    const imgDiv = document.createElement('div');
                    imgDiv.className = 'w-full h-full flex items-center justify-center p-1';
                    const img = document.createElement('img');
                    img.src = `/res/img/items/${slot.item}.png`;
                    img.alt = slot.item;
                    img.className = 'w-full h-full object-contain';
                    img.style.imageRendering = 'pixelated';
                    img.onerror = function() {
                        if (!this.dataset.fallbackAttempted) {
                            this.dataset.fallbackAttempted = 'true';
                            this.src = '/res/img/items/unknown.png';
                        }
                    };
                    imgDiv.appendChild(img);
                    slotDiv.appendChild(imgDiv);

                    // Add quantity label if > 1
                    if (slot.quantity > 1) {
                        const quantityLabel = document.createElement('div');
                        quantityLabel.className = 'absolute bottom-0 right-0 text-white';
                        quantityLabel.style.fontSize = '10px';
                        quantityLabel.textContent = `${slot.quantity}`;
                        slotDiv.appendChild(quantityLabel);
                    }
                }

                backpackDiv.appendChild(slotDiv);
            }

            const bagCountEl = document.getElementById('bag-count');
            if (bagCountEl) {
                bagCountEl.textContent = itemCount;
            }
        }
    }

    // Rebind inventory interactions after rendering slots
    // This ensures events are always attached, regardless of where updateCharacterDisplay() is called from
    if (window.inventoryInteractions && window.inventoryInteractions.bindInventoryEvents) {
        window.inventoryInteractions.bindInventoryEvents();
    }
}

/**
 * Update detailed stats tab
 * @param {Object} character - Character data
 */
async function updateStatsTab(character) {
    // Character info
    const raceEl = document.getElementById('stats-char-race');
    const classEl = document.getElementById('stats-char-class');
    const backgroundEl = document.getElementById('stats-char-background');
    const alignmentEl = document.getElementById('stats-char-alignment');

    if (raceEl) raceEl.textContent = character.race || '-';
    if (classEl) classEl.textContent = character.class || '-';
    if (backgroundEl) backgroundEl.textContent = character.background || '-';
    if (alignmentEl) alignmentEl.textContent = character.alignment || '-';

    // Ability scores with modifiers
    if (character.stats) {
        const stats = ['str', 'dex', 'con', 'int', 'wis', 'cha'];
        const statNames = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'];

        stats.forEach((stat, index) => {
            const valueEl = document.getElementById(`stats-${stat}`);
            const modEl = document.getElementById(`stats-${stat}-mod`);

            const value = character.stats[statNames[index]] || 10;
            const modifier = Math.floor((value - 10) / 2);

            if (valueEl) valueEl.textContent = value;
            if (modEl) modEl.textContent = modifier >= 0 ? `(+${modifier})` : `(${modifier})`;
        });
    }

    // Render detailed effects list
    renderStatsEffectsList(character.active_effects || []);
}

/**
 * Render detailed effects list in stats tab
 * @param {Array} activeEffects - Array of active effect objects
 */
function renderStatsEffectsList(activeEffects) {
    const container = document.getElementById('stats-effects-list');
    const noEffectsMsg = document.getElementById('stats-no-effects');

    if (!container) return;

    // Hidden system effects that shouldn't be displayed
    const HIDDEN_EFFECTS = [
        'fatigue-accumulation',
        'hunger-accumulation-stuffed',
        'hunger-accumulation-wellfed',
        'hunger-accumulation-hungry'
    ];

    // Filter and deduplicate effects
    const visibleEffects = (activeEffects || []).filter(e => !HIDDEN_EFFECTS.includes(e.effect_id));
    const uniqueEffects = [];
    const seenIds = new Set();
    for (const effect of visibleEffects) {
        if (!seenIds.has(effect.effect_id)) {
            seenIds.add(effect.effect_id);
            uniqueEffects.push(effect);
        }
    }

    // Clear container (keep noEffectsMsg element)
    container.innerHTML = '';

    if (uniqueEffects.length === 0) {
        const noEffects = document.createElement('div');
        noEffects.className = 'text-gray-500 italic';
        noEffects.id = 'stats-no-effects';
        noEffects.textContent = 'No active effects';
        container.appendChild(noEffects);
        return;
    }

    // Render each effect with details
    uniqueEffects.forEach(effect => {
        const effectEl = document.createElement('div');
        effectEl.className = 'mb-2 pb-1';
        effectEl.style.borderBottom = '1px solid #2a2a2a';

        // Effect name and category color
        const categoryColors = {
            buff: '#4ade80',     // green
            debuff: '#f87171',  // red
            modifier: '#60a5fa', // blue
            system: '#a78bfa'   // purple
        };
        const color = categoryColors[effect.category] || '#888';
        const effectName = effect.name || effect.effect_id.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase());

        let html = `<div style="color: ${color}; font-weight: bold; font-size: 8px;">${effectName}</div>`;

        // Stat modifiers
        if (effect.stat_modifiers && Object.keys(effect.stat_modifiers).length > 0) {
            const mods = [];
            for (const [stat, value] of Object.entries(effect.stat_modifiers)) {
                const statAbbr = stat.substring(0, 3).toUpperCase();
                const sign = value >= 0 ? '+' : '';
                const modColor = value >= 0 ? '#4ade80' : '#f87171';
                mods.push(`<span style="color: ${modColor}">${sign}${value} ${statAbbr}</span>`);
            }
            html += `<div style="font-size: 7px; margin-top: 1px;">${mods.join(' ')}</div>`;
        }

        // Show description from effect data
        if (effect.description) {
            html += `<div style="color: #888; font-size: 6px; margin-top: 1px;">${effect.description}</div>`;
        }

        // Duration info if applicable
        if (effect.duration_remaining > 0) {
            const hours = Math.floor(effect.duration_remaining / 60);
            const mins = Math.round(effect.duration_remaining % 60);
            let timeStr = hours > 0 ? `${hours}h ${mins}m` : `${mins}m`;
            html += `<div style="color: #f59e0b; font-size: 6px;">${timeStr} remaining</div>`;
        } else if (effect.delay_remaining > 0) {
            const hours = Math.floor(effect.delay_remaining / 60);
            const mins = Math.round(effect.delay_remaining % 60);
            let timeStr = hours > 0 ? `${hours}h ${mins}m` : `${mins}m`;
            html += `<div style="color: #f59e0b; font-size: 6px;">Starts in ${timeStr}</div>`;
        }

        effectEl.innerHTML = html;
        container.appendChild(effectEl);
    });
}

// Listen for game state changes and auto-update display
eventBus.on('gameStateChange', (data) => {
    logger.info('🔔 gameStateChange event received!', data);
    logger.info('🎨 Updating character display from event...');
    updateCharacterDisplay();
});

// Listen for character stats updates (from tick responses) to update stats tab
// This provides enriched effect data (name, description, stat_modifiers) from the backend
eventBus.on('character:statsUpdated', (data) => {
    logger.debug('📊 character:statsUpdated received for stats tab');
    if (data.active_effects) {
        // Update stats tab with enriched effects data
        renderStatsEffectsList(data.active_effects);
    }
});

logger.debug('Character display module loaded');
