/**
 * Inventory Interaction System Module
 *
 * Handles drag-and-drop, context menus, item tooltips, and all inventory interactions.
 * Includes equipment management, vault operations, and item actions.
 *
 * @module systems/inventoryInteractions
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getItemById } from '../state/staticData.js';
import { updateCharacterDisplay } from '../ui/characterDisplay.js';
import { showMessage } from '../ui/messaging.js';
import { showVaultUI } from '../ui/locationDisplay.js';
import { openContainer } from './containers.js';
import { isShopOpen, getCurrentTab, addItemToSell } from './shopSystem.js';
import { deltaApplier } from './deltaApplier.js';

// State for drag-and-drop
let draggedItem = null;
let draggedFromSlot = null;
let draggedFromType = null; // 'inventory', 'equipment', 'general'

// Expose drag state for container system (temporary compatibility)
export const inventoryDragState = {
    itemId: null,
    fromSlot: null,
    fromType: null,
    vaultBuilding: null
};

// Context menu state
let activeContextMenu = null;

// Vault open state (set by external code)
export let vaultOpen = false;
export function setVaultOpen(isOpen) {
    vaultOpen = isOpen;
}

/**
 * Initialize inventory interaction system
 */
export function initializeInventoryInteractions() {
    logger.info('Initializing inventory interactions');

    // NOTE: We don't bind on gameStateChange here anymore
    // The binding now happens in game-state.js AFTER updateCharacterDisplay() completes
    // This prevents race conditions where we try to bind before items are rendered

    // Close context menu when clicking elsewhere
    document.addEventListener('click', (e) => {
        // Don't close if clicking on container context menu
        if (e.target.closest('#container-context-menu')) {
            return;
        }

        if (activeContextMenu && !e.target.closest('.context-menu')) {
            closeContextMenu();
        }
    });

    // Prevent default context menu (but not for container slots)
    document.addEventListener('contextmenu', (e) => {
        // Don't interfere with container modal context menus
        if (e.target.closest('#container-modal')) {
            return; // Let container handle its own context menu
        }

        if (e.target.closest('[data-item-slot]') || e.target.closest('[data-slot]')) {
            e.preventDefault();
        }
    });
}

/**
 * Bind drag-drop and click events to inventory slots.
 *
 * DEPRECATED no-op. Slot interaction now lives in `slotInteractions.js`, which
 * uses ONE delegated set of pointer-event listeners (mouse + touch) instead of
 * per-slot HTML5-drag bindings. Delegation handles re-rendered DOM with no
 * per-render rebinding, so this is kept only as a harmless entry point for the
 * existing `window.inventoryInteractions.bindInventoryEvents()` callers.
 * The per-slot bind* / handleDrag* / handleDrop* / handleLeftClick functions
 * below are no longer wired up and will be removed in cleanup.
 */
export function bindInventoryEvents() {
    /* no-op — see slotInteractions.js */
}

/**
 * Bind events to an inventory slot
 */
function bindSlotEvents(slotElement, slotType, slotIndex) {
    const itemId = slotElement.getAttribute('data-item-id');

    if (!itemId) {
        // Empty slot - only allow dropping
        slotElement.addEventListener('dragover', handleDragOver);
        slotElement.addEventListener('drop', (e) => handleDrop(e, slotType, slotIndex, showMessage, showVaultUI));
        return;
    }

    // Make slot draggable
    slotElement.setAttribute('draggable', 'true');

    // Drag events
    slotElement.addEventListener('dragstart', (e) => handleDragStart(e, itemId, slotType, slotIndex));
    slotElement.addEventListener('dragend', handleDragEnd);
    slotElement.addEventListener('dragover', handleDragOver);
    slotElement.addEventListener('drop', (e) => handleDrop(e, slotType, slotIndex, showMessage, showVaultUI));

    // Click events
    slotElement.addEventListener('click', (e) => handleLeftClick(e, itemId, slotType, slotIndex, openContainer));
    slotElement.addEventListener('contextmenu', (e) => handleRightClick(e, itemId, slotType, slotIndex));

    // Hover events for tooltip
    slotElement.addEventListener('mouseenter', (e) => showItemTooltip(e, itemId, slotType));
    slotElement.addEventListener('mouseleave', hideItemTooltip);
}

/**
 * Bind events to an equipment slot
 */
function bindEquipmentSlotEvents(slotElement, slotName) {
    // Always bind click events to equipment slots (even when empty)
    // The handlers will dynamically check if there's an item at click time
    slotElement.addEventListener('click', (e) => {
        const itemId = slotElement.getAttribute('data-item-id');
        if (itemId) {
            handleLeftClick(e, itemId, 'equipment', slotName, openContainer);
        }
    });

    slotElement.addEventListener('contextmenu', (e) => {
        const itemId = slotElement.getAttribute('data-item-id');
        if (itemId) {
            handleRightClick(e, itemId, 'equipment', slotName);
        }
    });

    // Bind drag events dynamically
    slotElement.addEventListener('dragstart', (e) => {
        const itemId = slotElement.getAttribute('data-item-id');
        if (itemId) {
            slotElement.setAttribute('draggable', 'true');
            handleDragStart(e, itemId, 'equipment', slotName);
        } else {
            e.preventDefault();
        }
    });
    slotElement.addEventListener('dragend', handleDragEnd);

    // Hover events (dynamic check)
    slotElement.addEventListener('mouseenter', (e) => {
        const itemId = slotElement.getAttribute('data-item-id');
        if (itemId) {
            showItemTooltip(e, itemId, 'equipment');
        }
    });
    slotElement.addEventListener('mouseleave', hideItemTooltip);

    // Always allow dropping onto equipment slots
    slotElement.addEventListener('dragover', handleDragOver);
    slotElement.addEventListener('drop', (e) => handleDropOnEquipment(e, slotName, showMessage, showVaultUI));
}

/**
 * Bind events to a vault slot
 */
function bindVaultSlotEvents(slotElement, slotIndex, buildingId) {
    const itemId = slotElement.getAttribute('data-item-id');

    if (!itemId) {
        // Empty slot - only allow dropping
        slotElement.addEventListener('dragover', handleDragOver);
        slotElement.addEventListener('drop', (e) => handleDropOnVault(e, slotIndex, buildingId));
        return;
    }

    // Make slot draggable
    slotElement.setAttribute('draggable', 'true');

    // Drag events
    slotElement.addEventListener('dragstart', (e) => handleDragStart(e, itemId, 'vault', slotIndex, buildingId));
    slotElement.addEventListener('dragend', handleDragEnd);
    slotElement.addEventListener('dragover', handleDragOver);
    slotElement.addEventListener('drop', (e) => handleDropOnVault(e, slotIndex, buildingId));

    // Hover events
    slotElement.addEventListener('mouseenter', (e) => showItemTooltip(e, itemId, 'vault'));
    slotElement.addEventListener('mouseleave', hideItemTooltip);

    // Click events
    slotElement.addEventListener('click', (e) => handleLeftClick(e, itemId, 'vault', slotIndex, openContainer));
    slotElement.addEventListener('contextmenu', (e) => {
        e.preventDefault();
        handleRightClick(e, itemId, 'vault', slotIndex);
    });
}

/**
 * Handle drag start
 */
function handleDragStart(e, itemId, slotType, slotIndex, buildingId = null) {
    draggedItem = itemId;
    draggedFromSlot = slotIndex;
    draggedFromType = slotType;

    // Store buildingId for vault operations
    if (slotType === 'vault') {
        inventoryDragState.vaultBuilding = buildingId;
    }

    // Update global state for container system
    inventoryDragState.itemId = itemId;
    inventoryDragState.fromSlot = slotIndex;
    inventoryDragState.fromType = slotType;

    e.target.style.opacity = '0.5';
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', itemId);
}

/**
 * Handle drag end
 */
function handleDragEnd(e) {
    e.target.style.opacity = '1';
    draggedItem = null;
    draggedFromSlot = null;
    draggedFromType = null;

    // Clear global state
    inventoryDragState.itemId = null;
    inventoryDragState.fromSlot = null;
    inventoryDragState.fromType = null;
}

/**
 * Handle drag over (allow drop)
 */
function handleDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
}

/**
 * Handle drop on inventory slot
 * @param {Function} showMessage - UI message callback
 * @param {Function} showVaultUI - Vault UI callback
 */
async function handleDrop(e, toSlotType, toSlotIndex, showMessage, showVaultUI) {
    e.preventDefault();

    if (!draggedItem) return;

    // Check if dropping on the same slot (do nothing)
    if (draggedFromType === toSlotType && draggedFromSlot === toSlotIndex) {
        return;
    }

    // If dragging from vault to inventory, handle specially with surgical updates
    if (draggedFromType === 'vault') {
        const vaultSlots = document.querySelectorAll('[data-vault-slot]');
        const buildingId = vaultSlots[0]?.getAttribute('data-vault-building');

        // Map UI slot types to backend slot types
        let backendSlotType = toSlotType === 'general' ? 'general' : 'inventory';

        try {
            const result = await gameAPI.sendAction('move_item', {
                item_id: draggedItem,
                from_slot: draggedFromSlot,
                from_slot_type: 'vault',
                to_slot: toSlotIndex,
                to_slot_type: backendSlotType,
                vault_building: buildingId
            });

            if (result.success) {
                // Log the full response for debugging
                logger.debug('Vault drag-withdraw response:', JSON.stringify(result, null, 2));

                // Apply delta for surgical inventory updates (no full refresh)
                if (result.delta) {
                    logger.debug('Applying delta:', Object.keys(result.delta));
                    deltaApplier.applyDelta(result.delta);
                }

                // Silent refresh to update local cache without triggering location rebuild
                await refreshGameState(true);

                // Update character display (for gold changes etc)
                await updateCharacterDisplay();

                // Show updated vault directly (use imported function)
                const vaultData = result.delta?.vault_data;
                logger.debug('Vault data from delta:', vaultData ? 'present' : 'missing');
                if (vaultData) {
                    logger.debug('Updating vault UI via drag-withdraw, slots:', vaultData.slots?.length || 0);
                    showVaultUI(vaultData);
                } else {
                    logger.warn('No vault_data in response delta - vault UI will not update');
                }
            } else {
                showMessage(result.error || 'Failed to withdraw from vault', 'error');
            }
        } catch (error) {
            logger.error('Error withdrawing from vault:', error);
            showMessage('Failed to withdraw from vault', 'error');
        }
        return;
    }

    // If dragging from equipment to inventory
    if (draggedFromType === 'equipment') {
        await performAction('unequip', draggedItem, draggedFromSlot, undefined, 'equipment', showMessage, showVaultUI);
    }
    // If dropping on an inventory slot
    else {
        // Check if destination slot has an item
        const state = getGameStateSync();
        let destItem = null;

        // Get destination slot item using CORRECT state structure
        if (toSlotType === 'general') {
            // state.inventory is the array of general slots
            const generalSlots = Array.isArray(state.inventory) ? state.inventory : [];
            if (generalSlots[toSlotIndex]) {
                destItem = generalSlots[toSlotIndex];
            }
        } else if (toSlotType === 'inventory') {
            // state.equipment.bag.contents is the backpack
            const backpack = state.equipment?.bag?.contents || [];
            if (backpack[toSlotIndex]) {
                destItem = backpack[toSlotIndex];
            }
        }

        console.log('🎯 Stacking check:', {
            destItem,
            draggedItem,
            canStack: destItem && destItem.item === draggedItem,
            toSlotType,
            toSlotIndex
        });

        // If destination has an item and it's the same type, try to stack
        if (destItem && destItem.item === draggedItem) {
            console.log('🎯 Attempting stack:', {
                action: 'stack',
                itemId: draggedItem,
                fromSlot: draggedFromSlot,
                toSlot: toSlotIndex,
                fromSlotType: draggedFromType,
                toSlotType: toSlotType
            });
            await performAction('stack', draggedItem, draggedFromSlot, toSlotIndex, draggedFromType, toSlotType, showMessage, showVaultUI);
        }
        // Otherwise, move/swap as normal
        else {
            await performAction('move', draggedItem, draggedFromSlot, toSlotIndex, draggedFromType, toSlotType, showMessage, showVaultUI);
        }
    }
}

/**
 * Handle drop on equipment slot
 */
async function handleDropOnEquipment(e, equipSlotName, showMessage, showVaultUI) {
    e.preventDefault();

    if (!draggedItem) return;

    // Only allow equipping from inventory
    if (draggedFromType === 'inventory' || draggedFromType === 'general') {
        await performAction('equip', draggedItem, draggedFromSlot, equipSlotName, draggedFromType, showMessage, showVaultUI);
    }
}

/**
 * Handle drop on vault slot
 * Uses surgical updates to avoid rebuilding the scene (which would destroy vault overlay)
 */
async function handleDropOnVault(e, toSlotIndex, buildingId) {
    e.preventDefault();

    if (!draggedItem) return;

    try {
        const result = await gameAPI.sendAction('move_item', {
            item_id: draggedItem,
            from_slot: draggedFromSlot,
            to_slot: toSlotIndex,
            from_slot_type: draggedFromType,
            to_slot_type: 'vault',
            vault_building: buildingId
        });

        if (result.success) {
            // Log the full response for debugging
            logger.debug('Vault drag-drop response:', JSON.stringify(result, null, 2));

            // Apply delta for surgical inventory updates (no full refresh)
            if (result.delta) {
                logger.debug('Applying delta:', Object.keys(result.delta));
                deltaApplier.applyDelta(result.delta);
            }

            // Silent refresh to update local cache without triggering location rebuild
            await refreshGameState(true);

            // Update character display (for gold changes etc)
            await updateCharacterDisplay();

            // Show updated vault directly (use imported function)
            const vaultData = result.delta?.vault_data;
            logger.debug('Vault data from delta:', vaultData ? 'present' : 'missing');
            if (vaultData) {
                logger.debug('Updating vault UI via drag-drop, slots:', vaultData.slots?.length || 0);
                showVaultUI(vaultData);
            } else {
                logger.warn('No vault_data in response delta - vault UI will not update');
            }
        } else {
            showMessage(result.error || 'Failed to move item to vault', 'error');
        }
    } catch (error) {
        logger.error('Error in handleDropOnVault:', error);
        showMessage('Failed to move item to vault', 'error');
    }
}

/**
 * Handle left click (default action)
 * @param {Function} openContainer - Container opening callback
 */
async function handleLeftClick(e, itemId, slotType, slotIndex, openContainer = null, storeInVault = null, withdrawFromVault = null) {
    e.stopPropagation();

    // Check if vault is open - change default behavior
    if (vaultOpen) {
        if (slotType === 'vault') {
            // Clicking vault item -> withdraw to inventory
            await withdrawFromVault(itemId, slotIndex);
        } else if (slotType === 'general' || slotType === 'inventory') {
            // Clicking inventory item -> store in vault
            await storeInVault(itemId, slotIndex, slotType);
        }
        return;
    }

    // Check if shop is open on sell tab - change default behavior
    if (isShopOpen() && getCurrentTab() === 'sell') {
        if (slotType === 'general' || slotType === 'inventory') {
            // Clicking inventory item -> add to sell staging
            addItemToSell(itemId, slotIndex, slotType);
        }
        return;
    }

    // Normal behavior (vault and shop not affecting behavior)
    // Get item data to determine default action
    const itemData = getItemById(itemId);
    if (!itemData) {
        logger.warn(`Item ${itemId} not found`);
        return;
    }

    // Determine default action based on item data
    const action = getDefaultAction(itemData, slotType === 'equipment');

    // Perform the default action
    if (action === 'open') {
        // Open container
        if (openContainer) {
            await openContainer(itemId, slotIndex, slotType);
        } else {
            logger.error('openContainer function not provided');
        }
    } else if (action === 'equip' && itemData.gear_slot) {
        // For equip action, pass the gear_slot as the target
        await performAction(action, itemId, slotIndex, itemData.gear_slot, slotType);
    } else if (action) {
        await performAction(action, itemId, slotIndex, undefined, slotType);
    }
}

/**
 * Handle right click (context menu)
 */
function handleRightClick(e, itemId, slotType, slotIndex) {
    logger.debug(`Right click: ${itemId} in ${slotType}[${slotIndex}]`);
    e.preventDefault();
    e.stopPropagation();

    // Get item data template
    const itemData = getItemById(itemId);
    if (!itemData) {
        logger.warn(`Item ${itemId} not found`);
        return;
    }

    // Get actual inventory item to check current quantity
    const state = getGameStateSync();
    let inventoryItem = null;

    if (slotType === 'general') {
        const generalSlots = Array.isArray(state.inventory) ? state.inventory : [];
        inventoryItem = generalSlots[slotIndex];
    } else if (slotType === 'inventory') {
        const backpack = state.equipment?.bag?.contents || [];
        inventoryItem = backpack[slotIndex];
    }

    // Merge template data with actual inventory quantity
    const itemWithQuantity = {
        ...itemData,
        quantity: inventoryItem?.quantity || 1
    };

    // Get available actions
    const isEquipped = slotType === 'equipment';
    const actions = getItemActions(itemWithQuantity, isEquipped);

    logger.debug(`Showing context menu with ${actions.length} actions`);

    // Show context menu
    showContextMenu(e.clientX, e.clientY, itemId, slotIndex, slotType, actions);
}

/**
 * Get default action for an item
 * @param {Object} itemData - Item data
 * @param {boolean} isEquipped - Whether item is equipped
 * @returns {string} Action name
 */
export function getDefaultAction(itemData, isEquipped) {
    if (isEquipped) {
        return 'unequip';
    }

    // Check if item is a container - containers should be opened by default
    if (itemData.tags && itemData.tags.includes('container')) {
        return 'open';
    }

    // Check if item has "equipment" tag - this is the primary indicator
    if (itemData.tags && itemData.tags.includes('equipment')) {
        return 'equip';
    }

    // Fallback: Check item type for backwards compatibility
    const itemType = itemData.type || itemData.item_type;
    const weaponTypes = ['Weapon', 'Melee Weapon', 'Ranged Weapon', 'Simple Weapon', 'Martial Weapon'];
    const armorTypes = ['Armor', 'Light Armor', 'Medium Armor', 'Heavy Armor', 'Shield'];
    const wearableTypes = ['Ring', 'Necklace', 'Amulet', 'Cloak', 'Boots', 'Gloves', 'Helmet', 'Hat'];
    const ammunitionTypes = ['Ammunition', 'Ammo'];
    const consumableTypes = ['Potion', 'Consumable', 'Food'];

    if (weaponTypes.includes(itemType) || armorTypes.includes(itemType) || wearableTypes.includes(itemType) || ammunitionTypes.includes(itemType)) {
        return 'equip';
    }

    if (consumableTypes.includes(itemType)) {
        return 'use';
    }

    return 'examine';
}

/**
 * Get available actions for an item
 * @param {Object} itemData - Item data
 * @param {boolean} isEquipped - Whether item is equipped
 * @returns {Array<Object>} Array of action objects {action, label}
 */
export function getItemActions(itemData, isEquipped) {
    const actions = [];

    if (isEquipped) {
        actions.push({ action: 'unequip', label: 'Unequip' });
    } else {
        // Check for container tag first
        if (itemData.tags && itemData.tags.includes('container')) {
            actions.push({ action: 'open', label: 'Open' });
        }

        // Check for equipment tag
        if (itemData.tags && itemData.tags.includes('equipment')) {
            actions.push({ action: 'equip', label: 'Equip' });
        } else {
            // Fallback to type checking
            const itemType = itemData.type || itemData.item_type;
            const weaponTypes = ['Weapon', 'Melee Weapon', 'Ranged Weapon', 'Simple Weapon', 'Martial Weapon'];
            const armorTypes = ['Armor', 'Light Armor', 'Medium Armor', 'Heavy Armor', 'Shield'];
            const wearableTypes = ['Ring', 'Necklace', 'Amulet', 'Cloak', 'Boots', 'Gloves', 'Helmet', 'Hat'];
            const ammunitionTypes = ['Ammunition', 'Ammo'];

            if (weaponTypes.includes(itemType) || armorTypes.includes(itemType) || wearableTypes.includes(itemType) || ammunitionTypes.includes(itemType)) {
                actions.push({ action: 'equip', label: 'Equip' });
            }
        }

        // Check for consumables
        const consumableTypes = ['Potion', 'Consumable', 'Food'];
        const itemType = itemData.type || itemData.item_type;
        if (consumableTypes.includes(itemType)) {
            actions.push({ action: 'use', label: 'Use' });
        }

        // Add split action for stackable items (quantity > 1)
        if (itemData.quantity && itemData.quantity > 1) {
            actions.push({ action: 'split', label: 'Split' });
        }
    }

    // Examine is always available
    actions.push({ action: 'examine', label: 'Examine' });

    // Drop is always last
    actions.push({ action: 'drop', label: 'Drop' });

    return actions;
}

/**
 * Show context menu
 */
export function showContextMenu(x, y, itemId, slotIndex, slotType, actions) {
    // Close existing menu
    closeContextMenu();

    // Create context menu
    const menu = document.createElement('div');
    menu.className = 'context-menu fixed bg-gray-800 border-2 border-gray-600 shadow-lg z-50';
    menu.style.left = `${x}px`;
    menu.style.top = `${y}px`;
    menu.style.minWidth = '120px';

    // Add actions
    actions.forEach(({ action, label }) => {
        const item = document.createElement('div');
        item.className = 'context-menu-item px-3 py-2 hover:bg-gray-700 cursor-pointer text-sm text-white';
        item.textContent = label;
        item.addEventListener('click', async () => {
            // Pass parameters correctly: action, itemId, fromSlot, toSlotOrType, fromSlotType, toSlotType
            // For equip action, we need to pass the gear_slot from item data
            let toSlotOrType = undefined;
            if (action === 'equip') {
                const itemData = getItemById(itemId);
                toSlotOrType = itemData?.gear_slot || undefined;
            }

            await performAction(action, itemId, slotIndex, toSlotOrType, slotType, undefined);
            closeContextMenu();
        });
        menu.appendChild(item);
    });

    document.body.appendChild(menu);
    activeContextMenu = menu;

    // Adjust position if menu goes off screen
    const rect = menu.getBoundingClientRect();
    if (rect.right > window.innerWidth) {
        menu.style.left = `${window.innerWidth - rect.width - 5}px`;
    }
    if (rect.bottom > window.innerHeight) {
        menu.style.top = `${window.innerHeight - rect.height - 5}px`;
    }
}

/**
 * Close context menu
 */
function closeContextMenu() {
    if (activeContextMenu) {
        activeContextMenu.remove();
        activeContextMenu = null;
    }
}

/**
 * Perform an item action
 * @param {string} action - Action to perform
 * @param {string} itemId - Item ID
 * @param {number} fromSlot - Source slot
 * @param {*} toSlotOrType - Destination slot or type
 * @param {string} fromSlotType - Source slot type
 * @param {string} toSlotType - Destination slot type (optional)
 * @param {Function} showMessage - UI message callback
 * @param {Function} showVaultUI - Vault UI callback
 * @param {Function} showActionText - Action text callback
 * @param {Function} addItemToGround - Ground items callback
 * @param {Function} refreshGroundModal - Ground modal callback
 */
export async function performAction(action, itemId, fromSlot, toSlotOrType, fromSlotType, toSlotType, showMessage, showVaultUI, showActionText, addItemToGround, refreshGroundModal) {
    // Special case: examine (no backend call needed)
    if (action === 'examine') {
        showItemDetails(itemId);
        return;
    }

    // Special case: open container (no backend call needed - just show UI)
    if (action === 'open') {
        logger.error('openContainer callback not implemented yet');
        if (showMessage) showMessage('Cannot open container', 'error');
        return;
    }

    // Special case: split stack (handle client-side for now)
    if (action === 'split') {
        await handleSplitStack(itemId, fromSlot, fromSlotType, showMessage, showActionText);
        return;
    }

    // Check if Game API is initialized
    if (!gameAPI.initialized) {
        logger.error('Game API not initialized');
        if (showMessage) showMessage('Game not initialized', 'error');
        return;
    }

    // Prepare parameters for the new Game API
    const params = {
        item_id: itemId,
        from_slot: fromSlotType === 'equipment' ? -1 : (typeof fromSlot === 'number' ? fromSlot : -1),
        to_slot: typeof toSlotOrType === 'number' ? toSlotOrType : -1,
        from_slot_type: fromSlotType || '',
        to_slot_type: toSlotType || '',
        from_equip: fromSlotType === 'equipment' ? fromSlot : '',
        to_equip: typeof toSlotOrType === 'string' ? toSlotOrType : '',
        equipment_slot: typeof toSlotOrType === 'string' ? toSlotOrType : '',
        quantity: 1
    };

    // Special case: drop - prompt for quantity, but only add to ground after API success
    let dropInfo = null;  // Store drop details for later
    if (action === 'drop') {
        // Get item data from inventory to check current quantity
        const state = getGameStateSync();
        let inventoryItem = null;

        // Get the item at the specific slot using CORRECT state structure
        if (fromSlotType === 'general') {
            const generalSlots = Array.isArray(state.inventory) ? state.inventory : [];
            inventoryItem = generalSlots[fromSlot];
        } else if (fromSlotType === 'inventory') {
            const backpack = state.equipment?.bag?.contents || [];
            inventoryItem = backpack[fromSlot];
        }

        const currentQuantity = inventoryItem?.quantity || 1;
        const itemData = getItemById(itemId);

        // If quantity > 1, show prompt for how many to drop
        if (currentQuantity > 1) {
            const dropQuantity = await promptDropQuantity(itemData?.name || itemId, currentQuantity);

            if (dropQuantity === null || dropQuantity <= 0) {
                // User cancelled or entered invalid amount
                return;
            }

            // Set the drop quantity in the params
            params.quantity = dropQuantity;

            // Store drop info for after successful API call
            dropInfo = {
                itemId: itemId,
                quantity: dropQuantity,
                itemName: itemData?.name || itemId
            };
        } else {
            // Single item
            params.quantity = 1;

            // Store drop info for after successful API call
            dropInfo = {
                itemId: itemId,
                quantity: 1,
                itemName: itemData?.name || itemId
            };
        }
    }

    try {
        // Map old action names to new game action types
        const actionMap = {
            'equip': 'equip_item',
            'unequip': 'unequip_item',
            'use': 'use_item',
            'drop': 'drop_item',
            'move': 'move_item',
            'stack': 'stack_item',
            'add': 'add_item'
        };

        const gameAction = actionMap[action] || action;

        logger.debug(`Sending action: ${gameAction}`, params);

        // Extra logging for stack action
        if (action === 'stack') {
            console.log('🎯 STACK ACTION - Params being sent:', params);
        }

        // Send action to Go backend
        const result = await gameAPI.sendAction(gameAction, params);

        console.log('🎯 Backend response:', result);

        if (result.success) {
            logger.info('Action successful:', result.message);

            // Show message with color from API response (or default to green for success)
            if (showActionText && result.message) {
                showActionText(result.message, result.color || 'green', 4000);
            }

            // Silent refresh FIRST to update cached state
            await refreshGameState(true);

            // Update character display to sync all inventory/equipment visuals
            // This handles all DOM updates correctly (no need for delta in this case)
            await updateCharacterDisplay();

            // Update calculated values from response.data (weight/capacity)
            // Do this AFTER updateCharacterDisplay so backend values take precedence
            if (result.data) {
                if (result.data.total_weight !== undefined) {
                    const state = getGameStateSync();
                    state.character.total_weight = result.data.total_weight;
                    // Update weight display
                    const weightEl = document.getElementById('char-weight');
                    if (weightEl) weightEl.textContent = Math.round(result.data.total_weight);
                }
                if (result.data.weight_capacity !== undefined) {
                    const state = getGameStateSync();
                    state.character.weight_capacity = result.data.weight_capacity;
                    // Update capacity display
                    const maxWeightEl = document.getElementById('max-weight');
                    if (maxWeightEl) maxWeightEl.textContent = Math.round(result.data.weight_capacity);
                }
            }

            // If this was a drop action, NOW add to ground (after successful API call)
            if (action === 'drop' && dropInfo && addItemToGround) {
                addItemToGround(dropInfo.itemId, dropInfo.quantity);

                // Refresh ground modal if it's open
                if (refreshGroundModal) {
                    refreshGroundModal();
                }
            }
        } else {
            logger.error('Action failed:', result.error);

            // Show error message with color from API response (or default to red for errors)
            if (showActionText && result.error) {
                showActionText(result.error, result.color || 'red', 4000);
            }
        }
    } catch (error) {
        logger.error('Error performing action:', error);
        if (showActionText) {
            showActionText(`Failed to perform action: ${error.message}`, 'red', 4000);
        }
    }
}

/**
 * Prompt user for quantity to drop
 */
async function promptDropQuantity(itemName, maxQuantity) {
    return new Promise((resolve) => {
        // Create modal backdrop
        const modal = document.createElement('div');
        modal.style.position = 'fixed';
        modal.style.top = '0';
        modal.style.left = '0';
        modal.style.width = '100%';
        modal.style.height = '100%';
        modal.style.background = 'rgba(0, 0, 0, 0.8)';
        modal.style.zIndex = '100';
        modal.style.display = 'flex';
        modal.style.alignItems = 'center';
        modal.style.justifyContent = 'center';

        // Create dialog box
        const dialog = document.createElement('div');
        dialog.style.background = '#2a2a2a';
        dialog.style.border = '2px solid #4a4a4a';
        dialog.style.padding = '20px';
        dialog.style.minWidth = '300px';
        dialog.style.boxShadow = 'inset 1px 1px 0 #3a3a3a, inset -1px -1px 0 #000000';

        dialog.innerHTML = `
            <div style="color: white; font-size: 12px;">
                <h3 style="margin: 0 0 15px 0; font-weight: bold;">Drop ${itemName}</h3>
                <p style="margin: 0 0 10px 0; color: #ccc;">How many do you want to drop? (Max: ${maxQuantity})</p>
                <input type="number" id="drop-quantity-input" min="1" max="${maxQuantity}" value="${maxQuantity}"
                    style="width: 100%; padding: 5px; background: #1a1a1a; color: white; border: 2px solid #4a4a4a; font-size: 12px;" />
                <div style="margin-top: 15px; display: flex; gap: 10px; justify-content: flex-end;">
                    <button id="drop-cancel-btn" style="padding: 5px 15px; background: #3a3a3a; color: white; border: 2px solid #4a4a4a; cursor: pointer; font-size: 11px;">Cancel</button>
                    <button id="drop-confirm-btn" style="padding: 5px 15px; background: #4a4a4a; color: white; border: 2px solid #6a6a6a; cursor: pointer; font-size: 11px;">Drop</button>
                </div>
            </div>
        `;

        modal.appendChild(dialog);
        document.body.appendChild(modal);

        const input = document.getElementById('drop-quantity-input');
        const cancelBtn = document.getElementById('drop-cancel-btn');
        const confirmBtn = document.getElementById('drop-confirm-btn');

        // Focus and select input
        input.focus();
        input.select();

        // Event handlers
        const cleanup = () => {
            modal.remove();
        };

        cancelBtn.onclick = () => {
            cleanup();
            resolve(null);
        };

        confirmBtn.onclick = () => {
            const quantity = parseInt(input.value);
            if (quantity > 0 && quantity <= maxQuantity) {
                cleanup();
                resolve(quantity);
            } else {
                input.style.borderColor = '#ff0000';
            }
        };

        input.onkeydown = (e) => {
            if (e.key === 'Enter') {
                confirmBtn.click();
            } else if (e.key === 'Escape') {
                cancelBtn.click();
            }
        };

        // Click outside to cancel
        modal.onclick = (e) => {
            if (e.target === modal) {
                cancelBtn.click();
            }
        };
    });
}

/**
 * Handle splitting a stack into two stacks
 */
async function handleSplitStack(itemId, fromSlot, fromSlotType, showMessage, showActionText) {
    // Get item data from inventory to check current quantity
    const state = getGameStateSync();
    let inventoryItem = null;

    // Find the item in inventory to get actual quantity
    if (fromSlotType === 'general' && state.character.inventory?.general_slots) {
        inventoryItem = state.character.inventory.general_slots[fromSlot];
    } else if (fromSlotType === 'inventory' && state.character.inventory?.gear_slots?.bag?.contents) {
        inventoryItem = state.character.inventory.gear_slots.bag.contents[fromSlot];
    }

    if (!inventoryItem || inventoryItem.quantity <= 1) {
        if (showActionText) {
            showActionText('Cannot split a stack of 1 item.', 'yellow');
        }
        return;
    }

    const currentQuantity = inventoryItem.quantity;
    const itemData = getItemById(itemId);

    // Show prompt for how many to split
    const splitQuantity = await promptSplitQuantity(itemData?.name || itemId, currentQuantity);

    if (splitQuantity === null || splitQuantity <= 0 || splitQuantity >= currentQuantity) {
        // User cancelled or entered invalid amount
        return;
    }

    // Find an empty slot in inventory
    let emptySlotIndex = -1;
    let emptySlotType = '';

    // Check backpack first (more space)
    if (state.character.inventory?.gear_slots?.bag?.contents) {
        const backpackSlots = state.character.inventory.gear_slots.bag.contents;

        // Build a set of used slot numbers by checking the 'slot' field of each item
        const usedSlots = new Set();
        backpackSlots.forEach(slot => {
            if (slot && slot.slot !== undefined && slot.item !== null && slot.item !== '') {
                usedSlots.add(slot.slot);
            }
        });

        // Find first unused slot number (0-19)
        for (let i = 0; i < 20; i++) {
            if (!usedSlots.has(i)) {
                emptySlotIndex = i;
                emptySlotType = 'inventory';
                break;
            }
        }
    }

    // If no empty backpack slot, check general slots
    if (emptySlotIndex === -1 && state.character.inventory?.general_slots) {
        const generalSlots = state.character.inventory.general_slots;

        // Build a set of used slot numbers
        const usedSlots = new Set();
        generalSlots.forEach(slot => {
            if (slot && slot.slot !== undefined && slot.item !== null && slot.item !== '') {
                usedSlots.add(slot.slot);
            }
        });

        // Find first unused slot number (0-3)
        for (let i = 0; i < 4; i++) {
            if (!usedSlots.has(i)) {
                emptySlotIndex = i;
                emptySlotType = 'general';
                break;
            }
        }
    }

    // If no empty slot, show error
    if (emptySlotIndex === -1) {
        if (showActionText) {
            showActionText('Inventory full - cannot split stack', 'red');
        }
        return;
    }

    // Call new in-memory game action system
    if (!gameAPI.initialized) {
        logger.error('Game API not initialized');
        if (showMessage) showMessage('Game not initialized', 'error');
        return;
    }

    try {
        // Send split action to Go backend (in-memory)
        const result = await gameAPI.sendAction('split_item', {
            item_id: itemId,
            from_slot: fromSlot,
            to_slot: emptySlotIndex,
            from_slot_type: fromSlotType,
            to_slot_type: emptySlotType,
            quantity: splitQuantity
        });

        // Silent refresh FIRST to update cached state
        await refreshGameState(true);

        // Update character display to sync all inventory/equipment visuals
        await updateCharacterDisplay();

        // Update calculated values from response.data (weight/capacity)
        if (result.data) {
            if (result.data.total_weight !== undefined) {
                const state = getGameStateSync();
                state.character.total_weight = result.data.total_weight;
                const weightEl = document.getElementById('char-weight');
                if (weightEl) weightEl.textContent = Math.round(result.data.total_weight);
            }
            if (result.data.weight_capacity !== undefined) {
                const state = getGameStateSync();
                state.character.weight_capacity = result.data.weight_capacity;
                const maxWeightEl = document.getElementById('max-weight');
                if (maxWeightEl) maxWeightEl.textContent = Math.round(result.data.weight_capacity);
            }
        }

        if (showActionText) {
            showActionText(`Split ${splitQuantity} from stack of ${itemData?.name || itemId}`, 'green');
        }

    } catch (error) {
        logger.error('Failed to split stack:', error);
        if (showMessage) showMessage('Error splitting stack: ' + error.message, 'error');
    }
}

/**
 * Prompt user for quantity to split from stack
 */
async function promptSplitQuantity(itemName, maxQuantity) {
    const maxSplit = maxQuantity - 1; // Can't split all items
    const defaultSplit = Math.floor(maxQuantity / 2);

    return new Promise((resolve) => {
        // Create modal backdrop
        const modal = document.createElement('div');
        modal.style.position = 'fixed';
        modal.style.top = '0';
        modal.style.left = '0';
        modal.style.width = '100%';
        modal.style.height = '100%';
        modal.style.background = 'rgba(0, 0, 0, 0.8)';
        modal.style.zIndex = '100';
        modal.style.display = 'flex';
        modal.style.alignItems = 'center';
        modal.style.justifyContent = 'center';

        // Create dialog box
        const dialog = document.createElement('div');
        dialog.style.background = '#2a2a2a';
        dialog.style.border = '2px solid #4a4a4a';
        dialog.style.padding = '20px';
        dialog.style.minWidth = '300px';
        dialog.style.boxShadow = 'inset 1px 1px 0 #3a3a3a, inset -1px -1px 0 #000000';

        dialog.innerHTML = `
            <div style="color: white; font-size: 12px;">
                <h3 style="margin: 0 0 15px 0; font-weight: bold;">Split ${itemName}</h3>
                <p style="margin: 0 0 10px 0; color: #ccc;">How many to split into new stack? (Max: ${maxSplit})</p>
                <input type="number" id="split-quantity-input" min="1" max="${maxSplit}" value="${defaultSplit}"
                    style="width: 100%; padding: 5px; background: #1a1a1a; color: white; border: 2px solid #4a4a4a; font-size: 12px;" />
                <div style="margin-top: 15px; display: flex; gap: 10px; justify-content: flex-end;">
                    <button id="split-cancel-btn" style="padding: 5px 15px; background: #3a3a3a; color: white; border: 2px solid #4a4a4a; cursor: pointer; font-size: 11px;">Cancel</button>
                    <button id="split-confirm-btn" style="padding: 5px 15px; background: #4a4a4a; color: white; border: 2px solid #6a6a6a; cursor: pointer; font-size: 11px;">Split</button>
                </div>
            </div>
        `;

        modal.appendChild(dialog);
        document.body.appendChild(modal);

        const input = document.getElementById('split-quantity-input');
        const cancelBtn = document.getElementById('split-cancel-btn');
        const confirmBtn = document.getElementById('split-confirm-btn');

        // Focus and select input
        input.focus();
        input.select();

        // Event handlers
        const cleanup = () => {
            modal.remove();
        };

        cancelBtn.onclick = () => {
            cleanup();
            resolve(null);
        };

        confirmBtn.onclick = () => {
            const quantity = parseInt(input.value);
            if (quantity > 0 && quantity <= maxSplit) {
                cleanup();
                resolve(quantity);
            } else {
                input.style.borderColor = '#ff0000';
            }
        };

        input.onkeydown = (e) => {
            if (e.key === 'Enter') {
                confirmBtn.click();
            } else if (e.key === 'Escape') {
                cancelBtn.click();
            }
        };

        // Click outside to cancel
        modal.onclick = (e) => {
            if (e.target === modal) {
                cancelBtn.click();
            }
        };
    });
}

// ── Item-info panel presentation ────────────────────────────────────────────
const ITEM_RARITY_COLORS = {
    common: '#c2bbb2', uncommon: '#336b3e', rare: '#567e9d',
    legendary: '#4f3663', mythic: '#cdad36',
};
const ITEM_RARITY_GLOW = new Set(['legendary', 'mythic']);

// What each item tag actually does — surfaced on hover (title) and on click
// (inline, so it doesn't open a second stacked modal). Unknown tags show as a
// plain chip with no description.
const TAG_DESCRIPTIONS = {
    light: 'Small and easy to wield — usable in your off-hand for two-weapon fighting.',
    heavy: 'Large and unwieldy — small creatures attack with it at disadvantage.',
    'two-handed': 'Requires two hands to wield.',
    versatile: 'Can be used one- or two-handed, dealing more damage with two.',
    finesse: 'Use Strength or Dexterity for attack and damage rolls.',
    reach: 'Adds 5 ft to your reach when you attack.',
    thrown: 'Can be thrown to make a ranged attack.',
    ammunition: 'Fires ammunition; you draw a piece as part of the attack.',
    loading: 'Fires only once per action, however many attacks you have.',
    topple: 'On a hit, can knock the target prone (a save resists).',
    'improvised-weapon': 'Not a true weapon — improvised, with a flat damage die.',
    'light-source': 'Sheds light, illuminating the area around you.',
    'oil-burning': 'Burns as fuel or a fire source.',
    restraint: 'Can bind or restrain a creature.',
    poison: 'Coated with or delivers poison.',
    focus: 'Can serve as a spellcasting focus.',
    focus_provided: 'Provides a spellcasting focus.',
    spell_component: 'A material component for casting spells.',
    container: 'Holds other items.',
    pack: 'A bundle of adventuring gear.',
    'armor-set': 'Part of a matching armor set.',
    set: 'Part of a matching set.',
    consumable: 'Used up when activated.',
    healing: 'Restores hit points when consumed.',
    medium: 'Medium armor — adds up to +2 Dexterity to AC.',
    directional: 'Has a directional effect.',
};
const HIDDEN_ITEM_TAGS = new Set(['equipment']);

/**
 * Show item details modal — a sectioned, rarity-aware panel sized to the scene.
 */
export function showItemDetails(itemId) {
    const itemData = getItemById(itemId);
    if (!itemData) {
        logger.warn(`Item ${itemId} not found`);
        return;
    }

    // Find the scene container
    const sceneImage = document.getElementById('scene-image');
    const sceneContainer = sceneImage ? sceneImage.parentElement : null;

    if (!sceneContainer) {
        logger.warn('Scene container not found');
        return;
    }

    // Single-modal: don't let item-info panels stack on themselves.
    sceneContainer.querySelectorAll('.item-detail-modal').forEach((m) => m.remove());

    const props = itemData.properties || {};
    const getProp = (name) => itemData[name] ?? props[name];
    const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
    const has = (v) => v !== undefined && v !== null && v !== '' && v !== 'null';

    const rarity = String(itemData.rarity || 'common').toLowerCase();
    const rColor = ITEM_RARITY_COLORS[rarity] || ITEM_RARITY_COLORS.common;
    const glow = ITEM_RARITY_GLOW.has(rarity);

    // Modal overlay within the scene — sized to the scene, scrolls only if needed.
    const modal = document.createElement('div');
    modal.className = 'item-detail-modal absolute inset-0 flex items-center justify-center z-50';
    modal.style.background = 'rgba(0,0,0,0.8)';
    modal.addEventListener('click', (e) => { if (e.target === modal) modal.remove(); });

    const content = document.createElement('div');
    content.style.cssText = [
        'background:#23252b', 'width:92%', 'max-width:280px', 'max-height:88%',
        'overflow-y:auto', 'font-size:10px', 'color:#dddddd',
        `border:2px solid ${glow ? rColor : '#4b5563'}`,
        glow ? `box-shadow:0 0 8px 1px ${rColor}` : '',
    ].filter(Boolean).join(';');

    const row = (label, value, labelColor) =>
        `<div style="display:flex; justify-content:space-between; gap:8px; padding:1px 0;"><span style="color:${labelColor || '#9ca3af'};">${label}</span><span style="color:#eee; text-align:right;">${value}</span></div>`;
    const section = (title, rows) => rows.length
        ? `<div style="padding:0 8px 6px;"><div style="font-size:8px; color:#9ca3af; text-transform:uppercase; letter-spacing:0.5px; border-bottom:1px solid #3a3a3a; padding-bottom:2px; margin-bottom:4px;">${title}</div>${rows.join('')}</div>`
        : '';

    // Header: image + rarity-colored name, rarity badge, type · equip slot.
    const typeBits = [esc(itemData.type || 'Item')];
    const gearSlot = getProp('gear_slot');
    if (gearSlot) typeBits.push(esc(String(gearSlot).replace(/_/g, ' ')));
    const header = `
        <div style="display:flex; gap:8px; padding:8px; background:#1a1a1a; border-bottom:1px solid ${glow ? rColor : '#3a3a3a'};">
            ${itemData.image ? `<img src="${esc(itemData.image)}" alt="" style="width:40px; height:40px; flex-shrink:0; image-rendering:pixelated; background:#111; border:1px solid #333; object-fit:contain;">` : ''}
            <div style="min-width:0;">
                <div style="color:${rColor}; font-weight:bold; font-size:13px; line-height:1.1;">${esc(itemData.name)}</div>
                <div style="font-size:9px; margin-top:2px;"><span style="color:${rColor}; text-transform:capitalize;">◆ ${esc(rarity)}</span> <span style="color:#777;">·</span> <span style="color:#9ca3af; text-transform:capitalize;">${typeBits.join(' · ')}</span></div>
            </div>
        </div>`;

    // Description + lore (notes).
    let lore = `<div style="padding:8px; line-height:1.4;"><div style="color:#d1d5db;">${esc(itemData.description || itemData.ai_description || 'No description available.')}</div>`;
    if (itemData.notes) lore += `<div style="color:#8b8b8b; font-style:italic; margin-top:3px;">${esc(itemData.notes)}</div>`;
    lore += `</div>`;

    // Combat.
    const combatRows = [];
    if (has(getProp('damage'))) {
        let dmg = esc(getProp('damage'));
        const dtype = getProp('damage_type') || getProp('damage-type');
        if (has(dtype)) dmg += ` ${esc(dtype)}`;
        combatRows.push(row('Damage', dmg, '#f87171'));
    }
    if (has(getProp('range'))) {
        const long = getProp('range_long') || getProp('range-long');
        combatRows.push(row('Range', has(long) ? `${esc(getProp('range'))}/${esc(long)} ft` : `${esc(getProp('range'))} ft`, '#93c5fd'));
    }
    if (has(getProp('ac'))) combatRows.push(row('Armor Class', esc(getProp('ac')), '#93c5fd'));

    // Consumable.
    const consumeRows = [];
    if (has(getProp('heal'))) consumeRows.push(row('Healing', esc(getProp('heal')), '#86efac'));
    const effects = props.effects || itemData.effects;
    if (Array.isArray(effects) && effects.length) {
        consumeRows.push(row('Effects', effects.map((e) => `${esc(e.type)} ${e.value > 0 ? '+' : ''}${esc(e.value)}`).join(', '), '#86efac'));
    }

    // Container.
    const containRows = [];
    if (has(getProp('container_slots'))) containRows.push(row('Capacity', `${esc(getProp('container_slots'))} slots`, '#c4b5fd'));
    const allowed = getProp('allowed_types');
    if (Array.isArray(allowed) && allowed.length) containRows.push(row('Holds', allowed.map(esc).join(', '), '#c4b5fd'));

    // Details.
    const detailRows = [];
    const value = getProp('value') ?? getProp('price');
    if (value && value > 0) detailRows.push(row('💰 Value', `${Number(value).toLocaleString()} gp`, '#fbbf24'));
    if (getProp('weight') > 0) detailRows.push(row('⚖️ Weight', `${esc(getProp('weight'))} lb`, '#9ca3af'));
    if (getProp('stack') > 1) detailRows.push(row('Stack', `up to ${esc(getProp('stack'))}`, '#9ca3af'));

    // Focus — the spell component this focus provides an unlimited amount of while
    // equipped (elemental-staff style). `provides` is the component's item id.
    const focusRows = [];
    const provides = getProp('provides');
    if (has(provides)) {
        const compName = getItemById(provides)?.name || String(provides).replace(/[-_]/g, ' ');
        focusRows.push(row('Provides', `∞ ${esc(compName)} (unlimited when equipped)`, '#c4b5fd'));
    }

    // Tags as describable chips.
    const tags = (itemData.tags || props.tags || []).filter((t) => typeof t === 'string' && !HIDDEN_ITEM_TAGS.has(t));
    let tagsHtml = '';
    if (tags.length) {
        const chips = tags.map((t) => {
            const desc = TAG_DESCRIPTIONS[t] || '';
            return `<span class="item-tag-chip" data-tag="${esc(t)}" title="${esc(desc)}" style="font-size:8px; background:#3a3a3a; color:#cbd5e1; padding:1px 6px; border-radius:6px; text-transform:capitalize; cursor:${desc ? 'help' : 'default'};">${esc(t.replace(/[-_]/g, ' '))}</span>`;
        }).join(' ');
        tagsHtml = `<div style="padding:0 8px 4px; display:flex; gap:3px; flex-wrap:wrap;">${chips}</div>
            <div id="item-tag-desc" style="padding:0 8px 6px; font-size:9px; color:#9ca3af;"></div>`;
    }

    content.innerHTML = header + lore
        + section('Combat', combatRows)
        + section('Consumable', consumeRows)
        + section('Container', containRows)
        + section('Focus', focusRows)
        + section('Details', detailRows)
        + tagsHtml
        + `<button class="item-detail-close" style="width:100%; padding:5px; background:#0e7490; color:#fff; font-size:10px; border:none; border-top:1px solid #155e75; cursor:pointer;">Close</button>`;

    content.querySelector('.item-detail-close').addEventListener('click', () => modal.remove());

    // Tag click → show its description inline (no second modal).
    const descEl = content.querySelector('#item-tag-desc');
    content.querySelectorAll('.item-tag-chip').forEach((chip) => {
        chip.addEventListener('click', () => {
            const d = TAG_DESCRIPTIONS[chip.dataset.tag];
            if (descEl) descEl.innerHTML = d
                ? `<strong style="color:#cbd5e1; text-transform:capitalize;">${esc(chip.dataset.tag.replace(/[-_]/g, ' '))}:</strong> ${esc(d)}`
                : '';
        });
    });

    modal.appendChild(content);
    sceneContainer.appendChild(modal);
}

/**
 * Show action text on hover (in bottom-right of game scene)
 */
function showItemTooltip(e, itemId, slotType) {
    // Get item data
    const itemData = getItemById(itemId);
    if (!itemData) {
        logger.warn(`Item ${itemId} not found for tooltip`);
        return;
    }

    // Determine default action
    const isEquipped = slotType === 'equipment';
    const defaultAction = getDefaultAction(itemData, isEquipped);

    // Action display names (simple text)
    const actionNames = {
        'equip': 'Equip',
        'unequip': 'Unequip',
        'use': 'Use',
        'open': 'Open',
        'examine': 'Info',
        'drop': 'Drop'
    };

    // Action colors
    const actionColors = {
        'equip': '#4a9eff',      // Blue for equip
        'unequip': '#ff8c00',    // Orange for unequip
        'use': '#00ff00',        // Green for use
        'open': '#00ff00',       // Green for open
        'examine': '#ff8c00',    // Orange for info
        'drop': '#ff0000'        // Red for drop
    };

    const actionName = actionNames[defaultAction] || 'Info';
    const actionColor = actionColors[defaultAction] || '#ff8c00';

    // Update action text in bottom-right corner
    const actionText = document.getElementById('action-text');
    if (actionText) {
        actionText.textContent = actionName;
        actionText.style.color = actionColor;
        actionText.style.display = 'block';
    } else {
        logger.warn('action-text element not found!');
    }
}

/**
 * Hide action text
 */
function hideItemTooltip() {
    const actionText = document.getElementById('action-text');
    if (actionText) {
        actionText.style.display = 'none';
    }
}

/**
 * Store item from inventory into vault (when vault is open)
 * Uses surgical updates to avoid rebuilding the scene
 */
export async function storeInVault(itemId, fromSlot, fromSlotType) {
    // Get vault building ID from the vault overlay
    const vaultSlots = document.querySelectorAll('[data-vault-slot]');
    if (vaultSlots.length === 0) return;

    // May be empty for some vaults (legacy location-format); the backend
    // resolves it from the player's current building, so don't bail here.
    const buildingId = vaultSlots[0].getAttribute('data-vault-building') || '';

    // Find first free vault slot
    let freeVaultSlot = null;
    for (let i = 0; i < vaultSlots.length; i++) {
        const slot = vaultSlots[i];
        const slotItemId = slot.getAttribute('data-item-id');
        if (!slotItemId) {
            freeVaultSlot = parseInt(slot.getAttribute('data-vault-slot'));
            break;
        }
    }

    if (freeVaultSlot === null) {
        if (showMessage) showMessage('Vault is full', 'error');
        return;
    }

    // Perform move action
    try {
        const result = await gameAPI.sendAction('move_item', {
            from_slot: fromSlot,
            from_slot_type: fromSlotType,
            to_slot: freeVaultSlot,
            to_slot_type: 'vault',
            vault_building: buildingId
        });

        if (result.success) {
            showMessage('Item stored in vault', 'success');

            // Log the full response for debugging
            logger.debug('Vault store response:', JSON.stringify(result, null, 2));

            // Apply delta for surgical inventory updates (no full refresh)
            if (result.delta) {
                logger.debug('Applying delta:', Object.keys(result.delta));
                deltaApplier.applyDelta(result.delta);
            }

            // Silent refresh to update local cache without triggering location rebuild
            await refreshGameState(true);

            // Update character display (for gold changes etc)
            await updateCharacterDisplay();

            // Show updated vault directly (use imported function)
            const vaultData = result.delta?.vault_data;
            logger.debug('Vault data from delta:', vaultData ? 'present' : 'missing');
            if (vaultData) {
                logger.debug('Updating vault UI after store, slots:', vaultData.slots?.length || 0);
                showVaultUI(vaultData);
            } else {
                logger.warn('No vault_data in response delta - vault UI will not update');
            }
        }
    } catch (error) {
        logger.error('Error storing in vault:', error);
        showMessage('Failed to store item', 'error');
    }
}

/**
 * Withdraw item from vault to inventory (when vault is open)
 * Priority: backpack slots first, then general slots
 * Uses surgical updates to avoid rebuilding the scene
 */
export async function withdrawFromVault(itemId, vaultSlot) {
    const state = getGameStateSync();
    let targetSlot = null;
    let targetSlotType = null;

    // First try to find free backpack slot
    if (state.character?.inventory?.gear_slots?.bag?.contents) {
        const backpackContents = state.character.inventory.gear_slots.bag.contents;
        for (let i = 0; i < 20; i++) {
            const existingItem = backpackContents.find(item => item.slot === i);
            if (!existingItem || !existingItem.item) {
                targetSlot = i;
                targetSlotType = 'inventory'; // Backend expects 'inventory' for backpack
                break;
            }
        }
    }

    // If no backpack slot, try general slots
    if (targetSlot === null && state.character?.inventory?.general_slots) {
        for (let i = 0; i < state.character.inventory.general_slots.length; i++) {
            const slot = state.character.inventory.general_slots[i];
            if (!slot.item) {
                targetSlot = i;
                targetSlotType = 'general'; // Backend expects 'general' not 'general_slots'
                break;
            }
        }
    }

    if (targetSlot === null) {
        if (showMessage) showMessage('No free inventory space', 'error');
        return;
    }

    // Get building ID from vault
    const vaultSlots = document.querySelectorAll('[data-vault-slot]');
    const buildingId = vaultSlots[0]?.getAttribute('data-vault-building');

    // Perform move action
    try {
        const result = await gameAPI.sendAction('move_item', {
            item_id: itemId,
            from_slot: vaultSlot,
            from_slot_type: 'vault',
            to_slot: targetSlot,
            to_slot_type: targetSlotType,
            vault_building: buildingId
        });

        if (result.success) {
            showMessage('Item withdrawn from vault', 'success');

            // Log the full response for debugging
            logger.debug('Vault withdraw response:', JSON.stringify(result, null, 2));

            // Apply delta for surgical inventory updates (no full refresh)
            if (result.delta) {
                logger.debug('Applying delta:', Object.keys(result.delta));
                deltaApplier.applyDelta(result.delta);
            }

            // Silent refresh to update local cache without triggering location rebuild
            await refreshGameState(true);

            // Update character display (for gold changes etc)
            await updateCharacterDisplay();

            // Show updated vault directly (use imported function)
            const vaultData = result.delta?.vault_data;
            logger.debug('Vault data from delta:', vaultData ? 'present' : 'missing');
            if (vaultData) {
                logger.debug('Updating vault UI after withdrawal, slots:', vaultData.slots?.length || 0);
                showVaultUI(vaultData);
            } else {
                logger.warn('No vault_data in response delta - vault UI will not update');
            }
        }
    } catch (error) {
        logger.error('Error withdrawing from vault:', error);
        showMessage('Failed to withdraw item', 'error');
    }
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeInventoryInteractions);
} else {
    initializeInventoryInteractions();
}

logger.debug('Inventory interactions module loaded');
