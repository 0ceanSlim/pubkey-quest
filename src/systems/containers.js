/**
 * Container System Module
 *
 * Handles opening containers and managing their contents.
 * Containers are items like bags, pouches, etc. that can store other items.
 *
 * @module systems/containers
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getItemById } from '../state/staticData.js';
import { showMessage, showActionText } from '../ui/messaging.js';
import { updateAllDisplays } from '../ui/displayCoordinator.js';
import { inventoryDragState } from './inventoryInteractions.js';

let currentOpenContainer = null;

/**
 * Open a container and show its contents
 * @param {string} itemId - Container item ID
 * @param {number} fromSlot - Inventory slot containing the container
 * @param {string} fromSlotType - Type of slot ('general', 'inventory', 'equipment')
 */
export async function openContainer(itemId, fromSlot, fromSlotType = 'general') {
    logger.info(`🎒 OPENING CONTAINER: ${itemId} from ${fromSlotType}[${fromSlot}]`);

    // Fetch item data to get container properties
    const itemData = getItemById(itemId);
    if (!itemData) {
        showMessage('❌ Container not found', 'error');
        return;
    }

    // Check if item is a container
    if (!itemData.tags || !itemData.tags.includes('container')) {
        showMessage('❌ This item is not a container', 'error');
        return;
    }

    // Parse container_slots from properties object or root
    let containerSlots = 10; // default
    if (itemData.container_slots) {
        containerSlots = parseInt(itemData.container_slots);
    } else if (itemData.properties && itemData.properties.container_slots) {
        containerSlots = parseInt(itemData.properties.container_slots);
    }

    // Parse allowed_types from properties object or root
    let allowedTypes = 'any'; // default
    if (itemData.allowed_types) {
        allowedTypes = itemData.allowed_types;
    } else if (itemData.properties && itemData.properties.allowed_types) {
        allowedTypes = itemData.properties.allowed_types;
    }

    // Parse allowed_items (specific item IDs that can be stored)
    let allowedItems = [];
    if (itemData.allowed_items) {
        allowedItems = Array.isArray(itemData.allowed_items) ? itemData.allowed_items : [itemData.allowed_items];
    } else if (itemData.properties && itemData.properties.allowed_items) {
        allowedItems = Array.isArray(itemData.properties.allowed_items) ? itemData.properties.allowed_items : [itemData.properties.allowed_items];
    }

    logger.debug('Container properties:', { containerSlots, allowedTypes, allowedItems });

    // Store current container info
    currentOpenContainer = {
        itemId: itemId,
        fromSlot: fromSlot,
        fromSlotType: fromSlotType,
        slots: containerSlots,
        allowedTypes: allowedTypes,
        allowedItems: allowedItems,
        contents: [] // Will be populated from save data
    };

    // Load container contents from save data
    const gameState = getGameStateSync();
    const inventory = gameState.character.inventory;

    // Find the container in inventory and get its contents
    let containerData = null;
    if (inventory && inventory.general_slots) {
        const slot = inventory.general_slots.find(s => s && s.slot === fromSlot && s.item === itemId);
        if (slot && slot.contents) {
            containerData = slot.contents;
        }
    }

    // Initialize empty contents if not found
    if (!containerData) {
        containerData = [];
    }

    currentOpenContainer.contents = containerData;

    // Update modal display
    const modal = document.getElementById('container-modal');
    const title = document.getElementById('container-title');
    const icon = document.getElementById('container-icon');
    const usedSlots = document.getElementById('container-used-slots');
    const totalSlots = document.getElementById('container-total-slots');
    const typeRestriction = document.getElementById('container-type-restriction');
    const slotsGrid = document.getElementById('container-slots-grid');

    if (!modal || !title || !icon || !usedSlots || !totalSlots || !typeRestriction || !slotsGrid) {
        logger.error('Container modal elements not found in DOM');
        showMessage('❌ Container UI not available', 'error');
        return;
    }

    // Set container info
    title.textContent = itemData.name;
    icon.textContent = itemData.name.toLowerCase().includes('pouch') ? '👝' : '🎒';

    // Count actual non-null items
    const actualUsedSlots = containerData.filter(item => item && item.item).length;
    usedSlots.textContent = actualUsedSlots;
    totalSlots.textContent = containerSlots;

    // Show type restriction if not 'any'
    if (allowedTypes !== 'any') {
        // Handle both array and string types
        const typesText = Array.isArray(allowedTypes)
            ? allowedTypes.join(', ')
            : allowedTypes.replace('-', ' ');
        typeRestriction.textContent = `Only: ${typesText}`;
    } else {
        typeRestriction.textContent = 'Any items allowed';
    }

    // Render container slots
    await renderContainerSlots(slotsGrid, containerSlots, containerData);

    // Show modal
    modal.classList.remove('hidden');
}

/**
 * Render container slots grid
 */
async function renderContainerSlots(grid, totalSlots, contents) {
    grid.innerHTML = '';

    // Fixed-size square cells (like the vault) — up to 5 across, then wrap.
    const cols = Math.min(totalSlots, 5);
    grid.style.gridTemplateColumns = `repeat(${cols}, 52px)`;
    grid.style.gridAutoRows = '52px';

    for (let i = 0; i < totalSlots; i++) {
        const slot = document.createElement('div');
        // Match inventory slot styling
        slot.className = 'relative cursor-pointer hover:bg-gray-800 flex items-center justify-center';
        slot.style.cssText = `aspect-ratio: 1; background: #1a1a1a; border-top: 2px solid #000000; border-left: 2px solid #000000; border-right: 2px solid #3a3a3a; border-bottom: 2px solid #3a3a3a; clip-path: polygon(3px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0 calc(100% - 3px), 0 3px);`;
        slot.dataset.containerSlot = i;

        // Check if slot has an item
        const slotItem = contents[i];
        if (slotItem && slotItem.item) {
            // Get item data
            const itemData = getItemById(slotItem.item);
            if (itemData) {
                slot.setAttribute('data-item-id', slotItem.item);

                // Create image container (matching inventory structure)
                const imgDiv = document.createElement('div');
                imgDiv.className = 'w-full h-full flex items-center justify-center p-1';
                const img = document.createElement('img');
                img.src = `/res/img/items/${slotItem.item}.png`;
                img.alt = itemData.name;
                img.className = 'w-full h-full object-contain';
                img.style.imageRendering = 'pixelated';
                img.onerror = function() {
                    if (!this.dataset.fallbackAttempted) {
                        this.dataset.fallbackAttempted = 'true';
                        this.src = '/res/img/items/unknown.png';
                    }
                };
                imgDiv.appendChild(img);
                slot.appendChild(imgDiv);

                // Add quantity if > 1
                if (slotItem.quantity && slotItem.quantity > 1) {
                    const qty = document.createElement('div');
                    qty.className = 'absolute bottom-0 right-0 text-white font-bold';
                    qty.style.fontSize = '9px';
                    qty.style.textShadow = '1px 1px 2px #000';
                    qty.textContent = slotItem.quantity;
                    slot.appendChild(qty);
                }

                // Click (remove) and drag in/out are handled by the unified
                // pointer core (slotInteractions.js) via the data-container-slot
                // marker — see routeClick/routeDrop's 'container' surface.

                // Bind right-click for context menu (desktop)
                slot.addEventListener('contextmenu', (e) => {
                    logger.info(`⚡ Container contextmenu event fired for slot ${i}, item: ${slotItem.item}`);
                    e.preventDefault();
                    e.stopPropagation();
                    showContainerContextMenu(e, slotItem.item, i);
                });

                slot.title = itemData.name;

                logger.debug(`✅ Bound events to container slot ${i} (${slotItem.item})`);
            }
        }
        // Empty slots stay empty beveled squares, like the vault.

        grid.appendChild(slot);
    }

    logger.info(`✅ Rendered ${totalSlots} container slots (${contents.filter(c => c && c.item).length} with items)`);
}

/**
 * Drag and drop handlers for containers
 */
let containerDraggedItem = null;
let containerDraggedFromSlot = null;

function handleContainerDragStart(e, itemId, slotIndex) {
    containerDraggedItem = itemId;
    containerDraggedFromSlot = slotIndex;
    e.target.style.opacity = '0.5';
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', itemId);
    logger.debug(`Drag start from container slot ${slotIndex}: ${itemId}`);
}

function handleContainerDragEnd(e) {
    e.target.style.opacity = '1';
    containerDraggedItem = null;
    containerDraggedFromSlot = null;
}

function handleContainerDragOver(e) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
}

async function handleContainerDrop(e, toSlotIndex) {
    e.preventDefault();

    // Check if dragging from container to container (reorder)
    if (containerDraggedItem !== null && containerDraggedFromSlot !== null) {
        if (containerDraggedFromSlot === toSlotIndex) {
            logger.debug('Cannot drop item on itself');
            return;
        }

        // Move item within container (not implemented in backend yet)
        logger.warn('Moving items within containers not yet implemented');
        return;
    }

    // Check if dragging from inventory to container
    if (inventoryDragState && inventoryDragState.itemId) {
        const itemId = inventoryDragState.itemId;
        const fromSlot = inventoryDragState.fromSlot;
        const fromType = inventoryDragState.fromType;

        logger.debug(`Adding to container: ${itemId} from ${fromType}[${fromSlot}]`);

        // Call backend API to add item to container
        try {
            const result = await gameAPI.sendAction('add_to_container', {
                item_id: itemId,
                from_slot: fromSlot,
                from_slot_type: fromType,
                container_slot: currentOpenContainer.fromSlot,
                container_slot_type: currentOpenContainer.fromSlotType,
                to_container_slot: toSlotIndex
            });

            if (result.success) {
                showActionText(result.message, result.color || 'green');

                // Refresh game state from backend
                await refreshGameState();
                await updateAllDisplays();

                // Refresh container display
                await openContainer(currentOpenContainer.itemId, currentOpenContainer.fromSlot, currentOpenContainer.fromSlotType);
            } else {
                showActionText(result.error, result.color || 'red');
            }
        } catch (error) {
            logger.error('Failed to add item to container:', error);
            showActionText('Failed to add item to container', 'red');
        }
    }
}

/** The currently open container (or null), for the pointer core. */
export function getOpenContainer() {
    return currentOpenContainer;
}

/**
 * Add an item from inventory into the currently open container. Used by the
 * pointer core for both click-to-add (when a container is open) and drag-into
 * the open-container modal.
 */
export async function addToOpenContainer(itemId, fromSlot, fromSlotType, toContainerSlot = -1) {
    if (!currentOpenContainer) return;
    try {
        const result = await gameAPI.sendAction('add_to_container', {
            item_id: itemId,
            from_slot: fromSlot,
            from_slot_type: fromSlotType,
            container_slot: currentOpenContainer.fromSlot,
            container_slot_type: currentOpenContainer.fromSlotType,
            to_container_slot: toContainerSlot,
        });
        if (result.success) {
            showActionText(result.message, result.color || 'green');
            await refreshGameState();
            await updateAllDisplays();
            await openContainer(currentOpenContainer.itemId, currentOpenContainer.fromSlot, currentOpenContainer.fromSlotType);
        } else {
            showActionText(result.error || result.message, result.color || 'red');
        }
    } catch (error) {
        logger.error('Failed to add item to container:', error);
        showActionText('Failed to add item to container', 'red');
    }
}

/**
 * Remove item from container
 */
export async function removeFromContainer(slotIndex) {
    logger.info(`🔍 removeFromContainer called - slotIndex: ${slotIndex}`);

    if (!currentOpenContainer) {
        logger.warn('No open container');
        return;
    }

    const item = currentOpenContainer.contents[slotIndex];
    if (!item || !item.item) {
        logger.warn(`No item at slot ${slotIndex}`);
        return;
    }

    logger.info(`📤 Removing ${item.item} from container at slot ${slotIndex}`);

    try {
        // Call backend API to remove item from container
        const result = await gameAPI.sendAction('remove_from_container', {
            item_id: item.item,
            container_slot: currentOpenContainer.fromSlot,
            container_slot_type: currentOpenContainer.fromSlotType,
            from_container_slot: slotIndex
        });

        logger.info('Backend response:', result);

        if (result.success) {
            showActionText(result.message, result.color || 'green');

            // Refresh game state from backend
            await refreshGameState();
            await updateAllDisplays();

            // Refresh container display
            await openContainer(currentOpenContainer.itemId, currentOpenContainer.fromSlot, currentOpenContainer.fromSlotType);
        } else {
            showActionText(result.error, result.color || 'red');
        }
    } catch (error) {
        logger.error('Failed to remove item from container:', error);
        showActionText('Failed to remove item from container', 'red');
    }
}

/**
 * Show context menu for container items
 */
function showContainerContextMenu(e, itemId, slotIndex) {
    logger.info(`🖱️ showContainerContextMenu called - itemId: ${itemId}, slot: ${slotIndex}`);

    // Close any existing menu
    const existingMenu = document.getElementById('container-context-menu');
    if (existingMenu) {
        existingMenu.remove();
    }

    // Create context menu
    const menu = document.createElement('div');
    menu.id = 'container-context-menu';
    menu.className = 'absolute bg-gray-800 border-2 border-gray-600 text-white z-50';
    menu.style.left = `${e.pageX}px`;
    menu.style.top = `${e.pageY}px`;
    menu.style.minWidth = '120px';

    // Remove option
    const removeBtn = document.createElement('button');
    removeBtn.className = 'w-full px-3 py-1 text-left hover:bg-gray-700 text-xs';
    removeBtn.textContent = 'Remove';
    removeBtn.style.fontSize = '9px';
    removeBtn.onclick = (clickEvent) => {
        clickEvent.preventDefault();
        clickEvent.stopPropagation();
        logger.info(`🔘 Remove button clicked for slot ${slotIndex}`);
        removeFromContainer(slotIndex);
        menu.remove();
    };
    menu.appendChild(removeBtn);

    // Examine option
    const examineBtn = document.createElement('button');
    examineBtn.className = 'w-full px-3 py-1 text-left hover:bg-gray-700 text-xs border-t border-gray-700';
    examineBtn.textContent = 'Examine';
    examineBtn.style.fontSize = '9px';
    examineBtn.onclick = () => {
        showMessage('📋 Item examination coming soon', 'info');
        menu.remove();
    };
    menu.appendChild(examineBtn);

    document.body.appendChild(menu);

    // Close menu when clicking outside
    setTimeout(() => {
        document.addEventListener('click', function closeMenu(e) {
            if (!menu.contains(e.target)) {
                menu.remove();
                document.removeEventListener('click', closeMenu);
            }
        });
    }, 10);
}

/**
 * Close container modal
 */
export async function closeContainer() {
    const modal = document.getElementById('container-modal');
    if (modal) {
        modal.classList.add('hidden');
    }
    currentOpenContainer = null;

    // Remove any open container context menu
    const existingMenu = document.getElementById('container-context-menu');
    if (existingMenu) {
        existingMenu.remove();
    }

    // Refresh inventory display to show updated container
    await updateAllDisplays();
}

logger.debug('Container system module loaded');
