/**
 * Shop System Module
 *
 * Handles shop UI and buy/sell transactions with merchants.
 *
 * @module systems/shopSystem
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { showMessage } from '../ui/messaging.js';
import { updateAllDisplays } from '../ui/displayCoordinator.js';
import { getItemById } from '../state/staticData.js';

// Module state
let currentMerchantID = null;
let currentShopData = null;
let currentTab = 'buy';
let shopIsOpen = false;

// Sell staging state
let sellStaging = [];  // {itemID, quantity, value, slotIndex, slotType}

/**
 * Open shop interface for a merchant
 * @param {string} merchantID - Merchant NPC ID
 */
export async function openShop(merchantID) {
    logger.debug('Opening shop for merchant:', merchantID);
    currentMerchantID = merchantID;
    currentTab = 'buy';
    shopIsOpen = true;

    // Fetch shop data from backend
    try {
        // Get npub and saveID from gameAPI (initialized at game start)
        const npub = gameAPI.npub;
        const saveID = gameAPI.saveID;
        if (!npub || !saveID) {
            throw new Error('Session not initialized');
        }

        const response = await fetch(`/api/shop/${merchantID}?npub=${encodeURIComponent(npub)}&save_id=${encodeURIComponent(saveID)}`);
        if (!response.ok) {
            throw new Error('Failed to load shop data');
        }

        currentShopData = await response.json();
        logger.debug('Shop data loaded:', currentShopData);

        // Show modal
        const modal = document.getElementById('shop-modal');
        modal.classList.remove('hidden');

        // Update header
        document.getElementById('shop-merchant-name').textContent = currentShopData.merchant_name || 'Shop';

        // Render buy tab (default)
        switchShopTab('buy');

    } catch (error) {
        logger.error('Error loading shop:', error);
        showMessage('Failed to load shop', 'error');
    }
}

/**
 * Close shop interface
 */
export async function closeShop() {
    logger.debug('Closing shop');

    // Restore any items in sell staging back to inventory
    if (sellStaging.length > 0) {
        await restoreSellStagingItems();
    }

    const modal = document.getElementById('shop-modal');
    modal.classList.add('hidden');
    currentMerchantID = null;
    currentShopData = null;
    shopIsOpen = false;
    currentTab = 'buy';
    // Clear staging when closing
    sellStaging = [];
}

/**
 * Open buy quantity modal for custom amount
 * @param {object} shopItem - Shop item data
 */
function openBuyQuantityModal(shopItem) {
    logger.debug('Opening buy quantity modal for:', shopItem.item_id);

    // Create modal backdrop
    const backdrop = document.createElement('div');
    backdrop.className = 'fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-[100]';

    // Create modal
    const modal = document.createElement('div');
    modal.style.cssText = `
        background: #1a1a1a;
        color: white;
        padding: 20px;
        min-width: 300px;
        border-top: 2px solid #4a4a4a;
        border-left: 2px solid #4a4a4a;
        border-right: 2px solid #0a0a0a;
        border-bottom: 2px solid #0a0a0a;
    `;

    // Title
    const title = document.createElement('h3');
    title.textContent = `Buy ${shopItem.name}`;
    title.className = 'text-lg font-bold mb-4 text-green-400';
    modal.appendChild(title);

    // Stock info
    const stockInfo = document.createElement('p');
    stockInfo.textContent = `Available: ${shopItem.stock}`;
    stockInfo.className = 'text-sm text-gray-400 mb-3';
    modal.appendChild(stockInfo);

    // Quantity input
    const inputLabel = document.createElement('label');
    inputLabel.textContent = 'Quantity:';
    inputLabel.className = 'block text-sm mb-1';
    modal.appendChild(inputLabel);

    const input = document.createElement('input');
    input.type = 'number';
    input.min = '1';
    input.max = shopItem.stock.toString();
    input.value = '1';
    input.className = 'w-full px-3 py-2 mb-3 text-base';
    input.style.cssText = `
        background: #0a0a0a;
        color: white;
        border-top: 2px solid #0a0a0a;
        border-left: 2px solid #0a0a0a;
        border-right: 2px solid #3a3a3a;
        border-bottom: 2px solid #3a3a3a;
    `;
    modal.appendChild(input);

    // Total cost preview
    const costPreview = document.createElement('div');
    costPreview.className = 'mb-4 p-2';
    costPreview.style.cssText = `
        background: #0a0a0a;
        border: 1px solid #4a4a4a;
    `;
    const updateCostPreview = () => {
        const qty = Math.min(Math.max(parseInt(input.value) || 1, 1), shopItem.stock);
        const total = qty * shopItem.buy_price;
        costPreview.innerHTML = `
            <div class="text-sm">
                <span class="text-gray-400">${qty}x ${shopItem.name}</span>
            </div>
            <div class="text-lg font-bold text-yellow-400 mt-1">
                Total: ${total}g
            </div>
        `;
    };
    updateCostPreview();
    input.addEventListener('input', updateCostPreview);
    modal.appendChild(costPreview);

    // Buttons container
    const buttonsContainer = document.createElement('div');
    buttonsContainer.className = 'flex gap-2';

    // Confirm button
    const confirmBtn = document.createElement('button');
    confirmBtn.textContent = 'Buy';
    confirmBtn.className = 'flex-1 px-4 py-2 font-medium';
    confirmBtn.style.cssText = `
        background: #15803d;
        color: white;
        border-top: 2px solid #22c55e;
        border-left: 2px solid #22c55e;
        border-right: 2px solid #166534;
        border-bottom: 2px solid #166534;
    `;
    confirmBtn.onclick = async () => {
        const qty = Math.min(Math.max(parseInt(input.value) || 1, 1), shopItem.stock);
        document.body.removeChild(backdrop);
        await buyItemNow(shopItem.item_id, shopItem.name, qty, shopItem.buy_price, shopItem.stock);
    };
    buttonsContainer.appendChild(confirmBtn);

    // Cancel button
    const cancelBtn = document.createElement('button');
    cancelBtn.textContent = 'Cancel';
    cancelBtn.className = 'flex-1 px-4 py-2 font-medium';
    cancelBtn.style.cssText = `
        background: #7f1d1d;
        color: white;
        border-top: 2px solid #dc2626;
        border-left: 2px solid #dc2626;
        border-right: 2px solid #991b1b;
        border-bottom: 2px solid #991b1b;
    `;
    cancelBtn.onclick = () => {
        document.body.removeChild(backdrop);
    };
    buttonsContainer.appendChild(cancelBtn);

    modal.appendChild(buttonsContainer);
    backdrop.appendChild(modal);
    document.body.appendChild(backdrop);

    // Focus input
    input.focus();
    input.select();

    // Enter key to confirm
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            confirmBtn.click();
        } else if (e.key === 'Escape') {
            cancelBtn.click();
        }
    });
}

/**
 * Switch between buy and sell tabs
 * @param {string} tab - 'buy' or 'sell'
 */
export function switchShopTab(tab) {
    logger.debug('Switching to shop tab:', tab);
    currentTab = tab;

    // Update tab button styles
    const buyTab = document.getElementById('shop-buy-tab');
    const sellTab = document.getElementById('shop-sell-tab');
    const buyContent = document.getElementById('shop-buy-content');
    const sellContent = document.getElementById('shop-sell-content');

    if (tab === 'buy') {
        // Active buy tab
        buyTab.style.background = '#15803d';
        buyTab.style.borderTop = '2px solid #22c55e';
        buyTab.style.borderLeft = '2px solid #22c55e';
        buyTab.style.borderRight = '2px solid #166534';
        buyTab.style.borderBottom = '2px solid #166534';

        // Inactive sell tab
        sellTab.style.background = '#2a2a2a';
        sellTab.style.borderTop = '2px solid #4a4a4a';
        sellTab.style.borderLeft = '2px solid #4a4a4a';
        sellTab.style.borderRight = '2px solid #1a1a1a';
        sellTab.style.borderBottom = '2px solid #1a1a1a';

        buyContent.classList.remove('hidden');
        sellContent.classList.add('hidden');

        renderBuyTab();
    } else {
        // Active sell tab
        sellTab.style.background = '#15803d';
        sellTab.style.borderTop = '2px solid #22c55e';
        sellTab.style.borderLeft = '2px solid #22c55e';
        sellTab.style.borderRight = '2px solid #166534';
        sellTab.style.borderBottom = '2px solid #166534';

        // Inactive buy tab
        buyTab.style.background = '#2a2a2a';
        buyTab.style.borderTop = '2px solid #4a4a4a';
        buyTab.style.borderLeft = '2px solid #4a4a4a';
        buyTab.style.borderRight = '2px solid #1a1a1a';
        buyTab.style.borderBottom = '2px solid #1a1a1a';

        sellContent.classList.remove('hidden');
        buyContent.classList.add('hidden');

        renderSellTab();
    }
}

/**
 * Get total gold from inventory (counts "gold-piece" items in slots)
 */
function getPlayerGold(state) {
    let totalGold = 0;

    const generalSlots = Array.isArray(state.inventory) ? state.inventory : [];
    const equipment = state.equipment || {};

    // Check general slots
    for (const slot of generalSlots) {
        if (slot && slot.item === 'gold-piece' && slot.quantity) {
            totalGold += parseInt(slot.quantity) || 0;
        }
    }

    // Check equipment/bag
    if (equipment.bag && equipment.bag.contents) {
        const contents = Array.isArray(equipment.bag.contents) ? equipment.bag.contents : [];
        for (const slot of contents) {
            if (slot && slot.item === 'gold-piece' && slot.quantity) {
                totalGold += parseInt(slot.quantity) || 0;
            }
        }
    }

    // Check if gold is the bag item itself
    if (equipment.bag && equipment.bag.item === 'gold-piece' && equipment.bag.quantity) {
        totalGold += parseInt(equipment.bag.quantity) || 0;
    }

    // Check all equipment slots in case gold is in one
    for (const slotName in equipment) {
        const slot = equipment[slotName];
        if (slot && typeof slot === 'object' && slot.item === 'gold-piece' && slot.quantity) {
            totalGold += parseInt(slot.quantity) || 0;
        }
    }

    return totalGold;
}

/**
 * Render buy tab (merchant's inventory) - Grid Layout
 */
function renderBuyTab() {
    const state = getGameStateSync();
    const playerGold = getPlayerGold(state);
    const merchantGold = currentShopData.current_gold || 0;

    // Update gold displays
    document.getElementById('shop-player-gold').textContent = playerGold;
    document.getElementById('shop-merchant-gold').textContent = merchantGold;

    const grid = document.getElementById('shop-item-grid');
    grid.innerHTML = '';

    if (!currentShopData.inventory || currentShopData.inventory.length === 0) {
        grid.innerHTML = '<p style="color: #9ca3af;">This merchant has no items for sale.</p>';
        return;
    }

    // Render each item as grid slot (5 columns, like container system)
    currentShopData.inventory.forEach(shopItem => {
        // Create slot
        const slot = document.createElement('div');
        slot.className = 'relative cursor-pointer hover:bg-gray-800 transition-colors';
        slot.style.cssText = `
            aspect-ratio: 1;
            background: #1a1a1a;
            border-top: 2px solid #3a3a3a;
            border-left: 2px solid #3a3a3a;
            border-right: 2px solid #0a0a0a;
            border-bottom: 2px solid #0a0a0a;
        `;

        // Item image
        const img = document.createElement('img');
        img.src = `/res/img/items/${shopItem.item_id}.png`;
        img.className = 'w-full h-full object-contain p-1';
        img.style.imageRendering = 'pixelated';
        img.onerror = function() {
            if (!this.dataset.fallbackAttempted) {
                this.dataset.fallbackAttempted = 'true';
                this.src = '/res/img/items/unknown.png';
            }
        };
        slot.appendChild(img);

        // Stock quantity (bottom-right)
        const stockBadge = document.createElement('div');
        stockBadge.className = 'absolute bottom-0 right-0 px-1 text-white font-bold';
        stockBadge.style.cssText = `
            font-size: 9px;
            background: rgba(0, 0, 0, 0.7);
            text-shadow: 1px 1px 2px #000;
        `;
        stockBadge.textContent = shopItem.stock;
        slot.appendChild(stockBadge);

        // Left-click: Show value in message screen
        slot.addEventListener('click', (e) => {
            e.stopPropagation();
            showItemValue(shopItem);
        });

        // Right-click: Context menu
        slot.addEventListener('contextmenu', (e) => {
            e.preventDefault();
            e.stopPropagation();
            showBuyContextMenu(e, shopItem);
        });

        grid.appendChild(slot);
    });
}

/**
 * Show item value in message screen
 */
function showItemValue(shopItem) {
    const message = `${shopItem.name}: ${shopItem.buy_price}g`;
    showMessage(message, 'info');
}

/**
 * Show context menu for buying items
 */
function showBuyContextMenu(e, shopItem) {
    closeContextMenu();

    const menu = document.createElement('div');
    menu.id = 'shop-context-menu';
    menu.className = 'absolute z-50';
    menu.style.cssText = `
        left: ${e.pageX}px;
        top: ${e.pageY}px;
        background: #2a2a2a;
        border: 2px solid #4a4a4a;
        min-width: 150px;
    `;

    // Value button
    const valueBtn = createMenuButton('Value', () => {
        showItemValue(shopItem);
        closeContextMenu();
    });
    menu.appendChild(valueBtn);

    // Buy 1 button
    const buy1Btn = createMenuButton('Buy 1', async () => {
        closeContextMenu();
        await buyItemNow(shopItem.item_id, shopItem.name, 1, shopItem.buy_price, shopItem.stock);
    });
    menu.appendChild(buy1Btn);

    // Buy X button (opens modal)
    const buyXBtn = createMenuButton('Buy X', () => {
        closeContextMenu();
        openBuyQuantityModal(shopItem);
    });
    menu.appendChild(buyXBtn);

    document.body.appendChild(menu);

    // Click outside to close
    setTimeout(() => {
        document.addEventListener('click', handleContextMenuClose, { once: true });
    }, 10);
}

/**
 * Create a context menu button
 */
function createMenuButton(text, onClick) {
    const btn = document.createElement('button');
    btn.textContent = text;
    btn.className = 'w-full px-3 py-2 text-left text-sm hover:bg-gray-700';
    btn.style.color = '#fbbf24';
    btn.onclick = onClick;
    return btn;
}

/**
 * Close context menu
 */
function closeContextMenu() {
    const existing = document.getElementById('shop-context-menu');
    if (existing) existing.remove();
}

/**
 * Handle context menu close event
 */
function handleContextMenuClose() {
    closeContextMenu();
}

/**
 * Buy item immediately (no staging)
 */
async function buyItemNow(itemID, name, quantity, price, maxStock) {
    // Validate stock
    if (quantity > maxStock) {
        showMessage('Not enough stock', 'error');
        return;
    }

    // Validate player gold
    const state = getGameStateSync();
    const playerGold = getPlayerGold(state);
    const totalCost = price * quantity;

    if (playerGold < totalCost) {
        showMessage('Not enough gold', 'error');
        return;
    }

    // Get session info from gameAPI (already initialized at game start)
    const npub = gameAPI.npub;
    const saveID = gameAPI.saveID;

    if (!npub || !saveID) {
        showMessage('Session error', 'error');
        logger.error('GameAPI not initialized properly:', { npub, saveID });
        return;
    }

    try {
        const response = await fetch('/api/shop/buy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                npub: npub,
                save_id: saveID,
                merchant_id: currentMerchantID,
                item_id: itemID,
                quantity: quantity,
                action: 'buy'
            })
        });

        const result = await response.json();

        if (!response.ok || !result.success) {
            showMessage(result.error || result.message || 'Purchase failed', 'error');
            return;
        }

        showMessage(`Bought ${quantity}x ${name}!`, 'success');

        // Refresh shop and game state
        await refreshGameState();
        await openShop(currentMerchantID);
        updateAllDisplays();
    } catch (error) {
        logger.error('Buy transaction error:', error);
        showMessage('Failed to purchase items', 'error');
    }
}

/**
 * Render sell tab - NO LONGER RENDERS INVENTORY GRID
 * Player inventory remains visible in main UI, clicking items adds them to sell staging
 */
function renderSellTab() {
    if (!currentShopData) {
        logger.error('renderSellTab called but currentShopData is null!');
        return;
    }

    const state = getGameStateSync();
    const playerGold = getPlayerGold(state);
    const merchantGold = currentShopData.current_gold || 0;

    // Update gold displays
    document.getElementById('shop-player-gold').textContent = playerGold;
    document.getElementById('shop-merchant-gold').textContent = merchantGold;

    // Show message if merchant doesn't buy items
    if (!currentShopData.buys_items) {
        const sellContent = document.getElementById('shop-sell-content');
        sellContent.innerHTML = '<p class="text-center text-gray-400" style="font-size: 8px; padding: 20px;">This merchant doesn\'t buy items.</p>';
        return;
    }

    // Render the staging area (initially empty, items added by clicking inventory)
    renderSellStaging();
}

/**
 * Add item to sell staging
 */
function addToSellStaging(itemID, name, quantity, value, slotIndex, slotType) {
    // Validate merchant has gold
    const totalValue = value * quantity;
    const merchantGold = currentShopData.current_gold || currentShopData.starting_gold || 0;

    if (merchantGold < totalValue) {
        showMessage("Merchant doesn't have enough gold", 'error');
        return;
    }

    sellStaging.push({ itemID, name, quantity, value, slotIndex, slotType });
    renderSellStaging();
    // Note: sell-staging div is always visible now, no need to show/hide
}

/**
 * Render sell staging area
 */
function renderSellStaging() {
    const container = document.getElementById('sell-staged-items');
    container.innerHTML = '';

    let totalValue = 0;
    sellStaging.forEach((item, index) => {
        totalValue += item.value * item.quantity;

        // Create staged item badge (same as buy staging)
        const badge = document.createElement('div');
        badge.className = 'relative';
        badge.style.cssText = `
            width: 36px;
            height: 36px;
            background: #1a1a1a;
            border: 1px solid #4a4a4a;
        `;

        const img = document.createElement('img');
        img.src = `/res/img/items/${item.itemID}.png`;
        img.className = 'w-full h-full object-contain';
        img.style.imageRendering = 'pixelated';
        img.onerror = function() {
            if (!this.dataset.fallbackAttempted) {
                this.dataset.fallbackAttempted = 'true';
                this.src = '/res/img/items/unknown.png';
            }
        };
        badge.appendChild(img);

        // Quantity
        const qty = document.createElement('div');
        qty.className = 'absolute bottom-0 right-0 text-white font-bold';
        qty.style.cssText = 'font-size: 8px; text-shadow: 1px 1px 2px #000;';
        qty.textContent = item.quantity;
        badge.appendChild(qty);

        // Remove button
        const removeBtn = document.createElement('button');
        removeBtn.className = 'absolute -top-1 -right-1 text-white';
        removeBtn.style.cssText = `
            width: 14px;
            height: 14px;
            background: #dc2626;
            font-size: 10px;
            line-height: 1;
        `;
        removeBtn.textContent = '✕';
        removeBtn.onclick = async () => await removeStagedSell(index);
        badge.appendChild(removeBtn);

        container.appendChild(badge);
    });

    document.getElementById('sell-total-value').textContent = totalValue;
}

/**
 * Remove staged sell item (restore to inventory)
 */
async function removeStagedSell(index) {
    const item = sellStaging[index];

    // Restore item to inventory
    try {
        const result = await gameAPI.sendAction('add_item', {
            item_id: item.itemID,
            quantity: item.quantity
        });

        if (!result.success) {
            logger.error('Failed to restore item to inventory:', item.itemID, result.error);
            showMessage('Failed to restore item', 'error');
            return;
        }

        // Remove from staging
        sellStaging.splice(index, 1);
        renderSellStaging();

        // Refresh displays to show restored item
        await refreshGameState();
        await updateAllDisplays();

    } catch (error) {
        logger.error('Error restoring item:', error);
        showMessage('Failed to restore item', 'error');
    }
}

/**
 * Clear sell staging
 */
async function clearSellStaging() {
    // Restore items to inventory before clearing
    if (sellStaging.length > 0) {
        await restoreSellStagingItems();
    }
    sellStaging = [];
    renderSellStaging(); // Re-render to show empty staging area
    // Note: sell-staging div is always visible now, no need to hide
}

/**
 * Restore all sell staging items back to inventory
 */
async function restoreSellStagingItems() {
    logger.debug('Restoring sell staging items to inventory');

    for (const item of sellStaging) {
        try {
            const result = await gameAPI.sendAction('add_item', {
                item_id: item.itemID,
                quantity: item.quantity,
                to_slot_type: 'auto' // Auto-find available slot
            });

            if (!result.success) {
                logger.error('Failed to restore item to inventory:', item.itemID, result.error);
            }
        } catch (error) {
            logger.error('Error restoring item:', error);
        }
    }

    // Refresh displays to show restored items
    await refreshGameState();
    await updateAllDisplays();
}

/**
 * Confirm sell transaction
 */
async function confirmSellTransaction() {
    if (sellStaging.length === 0) return;

    // Get session info from gameAPI (already initialized at game start)
    const npub = gameAPI.npub;
    const saveID = gameAPI.saveID;

    if (!npub || !saveID) {
        showMessage('Session error', 'error');
        console.error('❌ GameAPI not initialized properly:', { npub, saveID });
        return;
    }

    // Process each staged item
    for (const item of sellStaging) {
        try {
            const response = await fetch('/api/shop/sell', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    npub: npub,
                    save_id: saveID,
                    merchant_id: currentMerchantID,
                    item_id: item.itemID,
                    quantity: item.quantity,
                    action: 'sell'
                })
            });

            const result = await response.json();

            if (!response.ok || !result.success) {
                showMessage(result.error || result.message || 'Sale failed', 'error');
                break;
            }
        } catch (error) {
            logger.error('Sell transaction error:', error);
            showMessage('Failed to sell items', 'error');
            break;
        }
    }

    // Clear staging and refresh
    sellStaging = [];
    clearSellStaging();
    await refreshGameState();
    await openShop(currentMerchantID);
    updateAllDisplays();
    showMessage('Sale complete!', 'success');
}

/**
 * Check if shop is open
 * @returns {boolean}
 */
export function isShopOpen() {
    return shopIsOpen;
}

/**
 * Get current shop tab
 * @returns {string} 'buy' or 'sell'
 */
export function getCurrentTab() {
    return currentTab;
}

/**
 * Add item from player inventory to sell staging
 * Called when player clicks inventory item while shop sell tab is open
 * @param {string} itemId - Item ID
 * @param {number} slotIndex - Slot index in inventory
 * @param {string} slotType - 'general' or 'inventory' (backpack)
 */
export async function addItemToSell(itemId, slotIndex, slotType) {
    if (!shopIsOpen || currentTab !== 'sell') {
        logger.warn('Attempted to add item to sell but shop is not open or not on sell tab');
        return;
    }

    if (!currentShopData || !currentShopData.buys_items) {
        showMessage("This merchant doesn't buy items", 'error');
        return;
    }

    // Get item data
    const itemData = getItemById(itemId);

    if (!itemData) {
        logger.warn('Item not found:', itemId);
        showMessage('Item not found', 'error');
        return;
    }

    // Get current state to check item exists in inventory
    const state = getGameStateSync();
    let inventorySlot = null;

    if (slotType === 'general') {
        const generalSlots = Array.isArray(state.inventory) ? state.inventory : [];
        inventorySlot = generalSlots[slotIndex];
    } else if (slotType === 'inventory') {
        const backpack = state.equipment?.bag?.contents || [];
        inventorySlot = backpack[slotIndex];
    }

    if (!inventorySlot || inventorySlot.item !== itemId) {
        showMessage('Item not found in that slot', 'error');
        return;
    }

    // Calculate sell value (what merchant pays player)
    // Price can be in itemData.price OR itemData.properties.price
    const basePrice = itemData.price || itemData.properties?.price || 0;

    // Get player charisma for pricing calculation
    const playerCharisma = state.stats?.charisma || 10;
    const shopType = currentShopData.shop_type || 'general';

    // Apply shop pricing formula from shop-pricing.json
    // Formula: base_value × (base_multiplier + (CHA - 10) × charisma_rate)
    let baseMultiplier, charismaRate;
    if (shopType === 'specialty') {
        baseMultiplier = 0.5;
        charismaRate = 0.05;
    } else {
        baseMultiplier = 0.3875;
        charismaRate = 0.05625;
    }

    const charismaBonus = (playerCharisma - 10) * charismaRate;
    const finalMultiplier = baseMultiplier + charismaBonus;
    const sellValue = Math.floor(basePrice * finalMultiplier);

    // Validate merchant has gold
    const merchantGold = currentShopData.current_gold || currentShopData.starting_gold || 0;
    if (merchantGold < sellValue) {
        showMessage("Merchant doesn't have enough gold", 'error');
        return;
    }

    // Remove 1 item from inventory immediately via backend
    try {
        const result = await gameAPI.sendAction('remove_from_inventory', {
            item_id: itemId,
            from_slot: slotIndex,
            from_slot_type: slotType,
            quantity: 1
        });

        if (!result.success) {
            showMessage(result.error || 'Failed to remove item from inventory', 'error');
            return;
        }

        // Refresh inventory display to show item removed
        await refreshGameState();
        await updateAllDisplays();

        // Add to staging list
        addToSellStaging(itemId, itemData.name, 1, sellValue, slotIndex, slotType);

    } catch (error) {
        logger.error('Error removing item from inventory:', error);
        showMessage('Failed to add item to sell', 'error');
    }
}

// Expose functions to window for onclick handlers
window.openShop = openShop;
window.closeShop = closeShop;
window.switchShopTab = switchShopTab;
window.confirmSellTransaction = confirmSellTransaction;
window.clearSellStaging = clearSellStaging;
