/**
 * Ground Items Modal UI Module
 *
 * Handles the ground items modal overlay.
 * Displays items on the ground and allows picking them up.
 *
 * @module ui/groundItems
 */

import { logger } from '../lib/logger.js';
import { refreshGameState, getGroundItems, setGroundItems } from '../state/gameState.js';
import { showMessage, showActionText } from './messaging.js';
import { getItemById } from '../state/staticData.js';

/**
 * Open the ground items modal
 * Shows all items on the ground at the current location
 */
export function openGroundModal() {
    // Get ground items at current location
    const groundItems = getGroundItems();

    // Create modal backdrop (only over scene area)
    const modal = document.createElement('div');
    modal.id = 'ground-modal';
    modal.style.position = 'absolute';
    modal.style.top = '0';
    modal.style.left = '0';
    modal.style.width = '100%';
    modal.style.height = '100%';
    modal.style.zIndex = '50';
    modal.style.background = 'rgba(0, 0, 0, 0.95)';
    modal.style.border = '2px solid #4a4a4a';
    modal.style.boxShadow = 'inset 1px 1px 0 #3a3a3a, inset -1px -1px 0 #000000';

    // Header with title and close button
    const header = document.createElement('div');
    header.className = 'flex items-center justify-between p-2';
    header.style.background = '#2a2a2a';
    header.style.borderBottom = '2px solid #4a4a4a';
    header.innerHTML = `
        <span class="text-white font-bold" style="font-size: 10px;">ITEMS ON GROUND</span>
        <button id="close-ground-modal-btn" class="text-white font-bold px-2 hover:bg-gray-700" style="font-size: 12px;">✕</button>
    `;

    // Content wrapper with centered grid
    const contentWrapper = document.createElement('div');
    contentWrapper.style.height = 'calc(100% - 36px)';
    contentWrapper.style.display = 'flex';
    contentWrapper.style.alignItems = 'flex-start';
    contentWrapper.style.justifyContent = 'center';
    contentWrapper.style.padding = '20px';
    contentWrapper.style.overflowY = 'auto';
    contentWrapper.style.boxSizing = 'border-box';

    // Grid container (4 columns, max width to match inventory sizing)
    const gridContainer = document.createElement('div');
    gridContainer.style.display = 'grid';
    gridContainer.style.gridTemplateColumns = 'repeat(4, 1fr)';
    gridContainer.style.gap = '4px';
    gridContainer.style.maxWidth = '200px';
    gridContainer.style.width = '100%';

    if (groundItems.length === 0) {
        const emptyMsg = document.createElement('div');
        emptyMsg.className = 'text-center text-gray-500 mt-8';
        emptyMsg.style.fontSize = '10px';
        emptyMsg.style.gridColumn = '1 / -1';
        emptyMsg.textContent = 'Nothing on the ground';
        gridContainer.appendChild(emptyMsg);
    } else {
        // Render each ground item (matching inventory slot style)
        groundItems.forEach(ground => {
            const itemSlot = document.createElement('div');
            itemSlot.className = 'relative cursor-pointer hover:bg-gray-600 flex items-center justify-center';
            itemSlot.style.cssText = `aspect-ratio: 1; background: #2a2a2a; border-top: 2px solid #1a1a1a; border-left: 2px solid #1a1a1a; border-right: 2px solid #4a4a4a; border-bottom: 2px solid #4a4a4a; clip-path: polygon(3px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 3px), calc(100% - 3px) 100%, 3px 100%, 0 calc(100% - 3px), 0 3px);`;
            itemSlot.onclick = () => pickupGroundItem(ground.item);

            // Create image container
            const imgDiv = document.createElement('div');
            imgDiv.className = 'w-full h-full flex items-center justify-center p-1';
            const img = document.createElement('img');
            img.src = `/res/img/items/${ground.item}.png`;
            img.alt = ground.item;
            img.className = 'w-full h-full object-contain';
            img.style.imageRendering = 'pixelated';
            img.onerror = function() {
                if (!this.dataset.fallbackAttempted) {
                    this.dataset.fallbackAttempted = 'true';
                    this.src = '/res/img/items/unknown.png';
                }
            };
            imgDiv.appendChild(img);
            itemSlot.appendChild(imgDiv);

            // Add quantity label if > 1
            if (ground.quantity > 1) {
                const quantityLabel = document.createElement('div');
                quantityLabel.className = 'absolute bottom-0 right-0 text-white';
                quantityLabel.style.fontSize = '10px';
                quantityLabel.textContent = `${ground.quantity}`;
                itemSlot.appendChild(quantityLabel);
            }

            gridContainer.appendChild(itemSlot);
        });
    }

    // Assemble modal structure
    contentWrapper.appendChild(gridContainer);
    modal.appendChild(header);
    modal.appendChild(contentWrapper);

    // Add close button event listener
    setTimeout(() => {
        const closeBtn = document.getElementById('close-ground-modal-btn');
        if (closeBtn) {
            closeBtn.addEventListener('click', closeGroundModal);
        }
    }, 0);

    // Add to scene container (not body)
    const sceneContainer = document.querySelector('#game-window .flex.flex-1 > div[style*="width: 556px"] > div[style*="height: 347px"] > div[style*="width: 347px"]');
    if (sceneContainer) {
        sceneContainer.style.position = 'relative';
        sceneContainer.appendChild(modal);
    } else {
        // Fallback: add to body if scene container not found
        document.body.appendChild(modal);
    }

    logger.debug('Ground modal opened:', groundItems.length, 'items');
}

/**
 * Close ground items modal
 */
export function closeGroundModal() {
    const modal = document.getElementById('ground-modal');
    if (modal) {
        modal.remove();
    }
}

/**
 * Refresh ground modal if it's open
 */
export function refreshGroundModal() {
    const modal = document.getElementById('ground-modal');
    if (modal) {
        // Modal is open, refresh it
        closeGroundModal();
        openGroundModal();
    }
}

/**
 * Pick up an item from the ground
 * @param {string} itemId - Item ID to pick up
 */
export async function pickupGroundItem(itemId) {
    if (!window.gameAPI || !window.gameAPI.initialized) {
        logger.error('Game API not initialized');
        showMessage('❌ Game not initialized', 'error');
        return;
    }

    const itemData = getItemById(itemId);

    try {
        // Server-authoritative pickup: the backend validates the item is actually on
        // the ground here and moves it into the inventory (no quantity → the whole
        // pile). Nothing is removed client-side until the server confirms.
        const result = await window.gameAPI.sendAction('pickup_item', { item_id: itemId });

        if (result.success) {
            showActionText(result.message || `Picked up ${itemData?.name || itemId}`, 'green');
            if (result.data && result.data.ground !== undefined) {
                setGroundItems(result.data.ground);
            }
            await refreshGameState(); // inventory changed
            closeGroundModal();
            openGroundModal();        // re-render with the updated ground
        } else {
            showMessage(result.error || 'Failed to pick up item', 'error');
        }
    } catch (error) {
        showMessage('Error picking up item: ' + error.message, 'error');
    }
}

logger.debug('Ground items module loaded');
