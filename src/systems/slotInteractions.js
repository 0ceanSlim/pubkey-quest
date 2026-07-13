/**
 * Unified slot interaction core (pointer events: mouse AND touch).
 *
 * One delegated set of listeners drives click-to-act, drag-to-move, and drop
 * across every inventory surface — replacing the old per-surface HTML5-drag
 * bindings (which never fired on touch) and the three separate drop handlers in
 * inventoryInteractions.js. Because drop targets are resolved with
 * `elementFromPoint`, any slot carrying the right data-* attributes
 * participates, so adding a surface is just markup + a routeDrop branch.
 *
 * Gesture model (shared by mouse + touch):
 *   - press + release without moving        → click  (routeClick)
 *   - press + move past threshold + release  → drag   (routeDrop)
 *   - right-click / touch long-press         → context menu
 *
 * Surfaces handled here: general grid, backpack, equipment, vault, plus
 * drop-onto-a-closed-container in a general slot. Containers/shop/ground keep
 * their own handlers until Phase 4 migrates them onto this core.
 *
 * @module systems/slotInteractions
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getItemById } from '../state/staticData.js';
import { updateCharacterDisplay } from '../ui/characterDisplay.js';
import { showMessage, showActionText } from '../ui/messaging.js';
import { showVaultUI } from '../ui/locationDisplay.js';
import { openContainer, getOpenContainer, addToOpenContainer, removeFromContainer } from './containers.js';
import { isShopOpen, getCurrentTab, addItemToSell } from './shopSystem.js';
import { deltaApplier } from './deltaApplier.js';
import {
    performAction,
    getDefaultAction,
    getItemActions,
    showContextMenu,
    storeInVault,
    withdrawFromVault,
    inventoryDragState,
    vaultOpen,
} from './inventoryInteractions.js';

const SLOT_SELECTOR = '[data-item-slot], [data-slot], [data-vault-slot], [data-container-slot]';
const DRAG_THRESHOLD = 6;   // px of movement before a press becomes a drag
const LONG_PRESS_MS = 500;  // touch long-press → context menu

let pointerState = null; // { pointerId, startX, startY, src, dragging, ghost, longPressTimer }
let ready = false;

/**
 * Initialize the delegated pointer listeners. Idempotent — safe to call from
 * every former bindInventoryEvents() site.
 */
export function initSlotInteractions() {
    if (ready) return;
    ready = true;

    // Slots own their touch gestures so a drag doesn't scroll/zoom the page.
    const style = document.createElement('style');
    style.textContent = `${SLOT_SELECTOR} { touch-action: none; }`;
    document.head.appendChild(style);

    // Kill native HTML5 drag on slots — pointer events own dragging now.
    document.addEventListener('dragstart', (e) => {
        if (e.target.closest(SLOT_SELECTOR)) e.preventDefault();
    });

    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('pointermove', onPointerMove, { passive: false });
    document.addEventListener('pointerup', onPointerUp);
    document.addEventListener('pointercancel', cancelPointer);

    // Right-click context menu (desktop). Touch uses long-press (see below).
    document.addEventListener('contextmenu', (e) => {
        const el = e.target.closest(SLOT_SELECTOR);
        if (!el) return;
        const d = readDescriptor(el);
        if (!d || !d.itemId) return;
        if (d.surface === 'container') return; // container modal owns its own menu
        openContextMenu(d, e.clientX, e.clientY, e);
    });

    // Hover label that follows the cursor (mouse only — touch has no hover).
    document.addEventListener('mousemove', onHoverMove);
    document.addEventListener('mouseleave', hideTip);

    logger.debug('Slot interactions (pointer core) initialized');
}

/** Read a slot's descriptor from its data-* attributes. */
function readDescriptor(el) {
    if (!el) return null;
    const itemId = el.getAttribute('data-item-id') || '';
    if (el.hasAttribute('data-vault-slot')) {
        return {
            surface: 'vault',
            index: parseInt(el.getAttribute('data-vault-slot'), 10),
            buildingId: el.getAttribute('data-vault-building'),
            itemId, el,
        };
    }
    if (el.hasAttribute('data-container-slot')) {
        return { surface: 'container', index: parseInt(el.getAttribute('data-container-slot'), 10), itemId, el };
    }
    if (el.hasAttribute('data-slot')) {
        return { surface: 'equipment', slotName: el.getAttribute('data-slot'), itemId, el };
    }
    if (el.hasAttribute('data-item-slot')) {
        const surface = el.closest('#backpack-slots') ? 'inventory' : 'general';
        return { surface, index: parseInt(el.getAttribute('data-item-slot'), 10), itemId, el };
    }
    return null;
}

// --- pointer gesture handling -------------------------------------------

function onPointerDown(e) {
    if (e.button && e.button !== 0) return; // primary button only
    const el = e.target.closest(SLOT_SELECTOR);
    if (!el) return;
    const src = readDescriptor(el);
    if (!src) return;

    pointerState = {
        pointerId: e.pointerId,
        startX: e.clientX, startY: e.clientY,
        src, dragging: false, ghost: null, longPressTimer: null,
    };

    // Touch long-press opens the context menu (desktop has real right-click).
    if (e.pointerType === 'touch' && src.itemId) {
        const x = e.clientX, y = e.clientY;
        pointerState.longPressTimer = setTimeout(() => {
            if (pointerState && !pointerState.dragging) {
                openContextMenu(src, x, y, null);
                cancelPointer();
            }
        }, LONG_PRESS_MS);
    }
}

function onPointerMove(e) {
    if (!pointerState || e.pointerId !== pointerState.pointerId) return;
    const dx = e.clientX - pointerState.startX;
    const dy = e.clientY - pointerState.startY;

    if (!pointerState.dragging) {
        if (!pointerState.src.itemId) return;          // empty slot: nothing to drag
        if (Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
        beginDrag(pointerState.src);
    }
    if (pointerState.dragging && pointerState.ghost) {
        moveGhost(pointerState.ghost, e.clientX, e.clientY);
        e.preventDefault(); // stop touch scroll while dragging
    }
}

async function onPointerUp(e) {
    if (!pointerState || e.pointerId !== pointerState.pointerId) return;
    clearTimeout(pointerState.longPressTimer);
    const st = pointerState;
    pointerState = null;

    if (st.dragging) {
        destroyGhost(st.ghost);
        clearLegacyDragState();
        const targetEl = document.elementFromPoint(e.clientX, e.clientY);
        const tgt = readDescriptor(targetEl?.closest(SLOT_SELECTOR));
        if (tgt) await routeDrop(st.src, tgt);
    } else {
        await routeClick(st.src);
    }
}

function cancelPointer() {
    if (!pointerState) return;
    clearTimeout(pointerState.longPressTimer);
    destroyGhost(pointerState.ghost);
    clearLegacyDragState();
    pointerState = null;
}

function beginDrag(src) {
    pointerState.dragging = true;
    clearTimeout(pointerState.longPressTimer);
    pointerState.ghost = makeGhost(src);
    // Legacy compat: containers.js still reads this until Phase 4.
    inventoryDragState.itemId = src.itemId;
    inventoryDragState.fromSlot = src.surface === 'equipment' ? src.slotName : src.index;
    inventoryDragState.fromType = src.surface;
    if (src.surface === 'vault') inventoryDragState.vaultBuilding = src.buildingId;
}

function clearLegacyDragState() {
    inventoryDragState.itemId = null;
    inventoryDragState.fromSlot = null;
    inventoryDragState.fromType = null;
}

// --- drag ghost ----------------------------------------------------------

function makeGhost(src) {
    const itemData = getItemById(src.itemId);
    const ghost = document.createElement('div');
    ghost.className = 'drag-ghost';
    ghost.style.cssText =
        'position:fixed;z-index:2000;pointer-events:none;width:40px;height:40px;' +
        'opacity:0.85;image-rendering:pixelated;transform:translate(-50%,-50%);';
    if (itemData?.image) {
        ghost.style.backgroundImage = `url(${itemData.image})`;
        ghost.style.backgroundSize = 'contain';
        ghost.style.backgroundRepeat = 'no-repeat';
        ghost.style.backgroundPosition = 'center';
    } else {
        ghost.style.background = '#4a4a4a';
        ghost.style.border = '2px solid #6a6a6a';
    }
    document.body.appendChild(ghost);
    return ghost;
}

function moveGhost(ghost, x, y) {
    ghost.style.left = `${x}px`;
    ghost.style.top = `${y}px`;
}

function destroyGhost(ghost) {
    if (ghost && ghost.parentNode) ghost.parentNode.removeChild(ghost);
}

// --- routing -------------------------------------------------------------

/** Default action on a tap/click, honoring vault-open and shop-sell modes. */
async function routeClick(src) {
    if (!src.itemId) return;
    const ref = src.surface === 'equipment' ? src.slotName : src.index;

    // Clicking an item inside the open container modal removes it.
    if (src.surface === 'container') {
        await removeFromContainer(src.index);
        return;
    }

    // A container modal is open: clicking an inventory item adds it in.
    if (getOpenContainer() && (src.surface === 'general' || src.surface === 'inventory')) {
        await addToOpenContainer(src.itemId, src.index, src.surface);
        return;
    }

    // Vault open: click moves between inventory and vault.
    if (vaultOpen) {
        if (src.surface === 'vault') await withdrawFromVault(src.itemId, src.index);
        else if (src.surface === 'general' || src.surface === 'inventory') await storeInVault(src.itemId, src.index, src.surface);
        return;
    }

    // Shop open: inventory clicks must never fall through to open/equip/use (which
    // would open a container behind the shop, or equip mid-trade). On the sell tab
    // a click stages the item for sale; on the buy tab (the default, and the state
    // after every buy/sell) it's a no-op.
    if (isShopOpen()) {
        if (getCurrentTab() === 'sell' && (src.surface === 'general' || src.surface === 'inventory')) {
            addItemToSell(src.itemId, src.index, src.surface);
        }
        return;
    }

    const itemData = getItemById(src.itemId);
    if (!itemData) {
        logger.warn(`Item ${src.itemId} not found`);
        return;
    }
    const action = getDefaultAction(itemData, src.surface === 'equipment');
    if (action === 'open') {
        await openContainer(src.itemId, ref, src.surface);
    } else if (action === 'equip' && itemData.gear_slot) {
        await performAction('equip', src.itemId, ref, itemData.gear_slot, src.surface, undefined, showMessage, showVaultUI, showActionText);
    } else {
        await performAction(action, src.itemId, ref, undefined, src.surface, undefined, showMessage, showVaultUI, showActionText);
    }
}

/** Resolve a drop of `src` onto `tgt`. */
async function routeDrop(src, tgt) {
    if (!src.itemId) return;
    // No-op drop on the same slot.
    if (src.surface === tgt.surface && src.index === tgt.index && src.slotName === tgt.slotName) return;

    // Into the open container modal: add an inventory item.
    if (tgt.surface === 'container') {
        if (src.surface === 'general' || src.surface === 'inventory') {
            await addToOpenContainer(src.itemId, src.index, src.surface, tgt.index);
        }
        return; // container→container reorder isn't supported by the backend yet
    }
    // Out of the open container modal: remove to inventory.
    if (src.surface === 'container') {
        await removeFromContainer(src.index);
        return;
    }

    // Equip by dropping on an equipment slot (only from inventory/general).
    if (tgt.surface === 'equipment') {
        if (src.surface === 'general' || src.surface === 'inventory') {
            await performAction('equip', src.itemId, src.index, tgt.slotName, src.surface, undefined, showMessage, showVaultUI, showActionText);
        }
        return;
    }

    // Vault deposit / withdraw both run through move_item.
    if (tgt.surface === 'vault') { await vaultMove(src, tgt.index, tgt.buildingId, 'vault'); return; }
    if (src.surface === 'vault') { await vaultMove(src, tgt.index, src.buildingId, tgt.surface); return; }

    // Unequip by dropping equipment onto an inventory slot.
    if (src.surface === 'equipment') {
        await performAction('unequip', src.itemId, src.slotName, undefined, 'equipment', undefined, showMessage, showVaultUI, showActionText);
        return;
    }

    // Drop onto a CLOSED container sitting in a general slot → add into it.
    if ((tgt.surface === 'general' || tgt.surface === 'inventory') && tgt.itemId && tgt.itemId !== src.itemId) {
        const tgtData = getItemById(tgt.itemId);
        if (tgtData?.tags?.includes('container')) {
            await addToClosedContainer(src, tgt);
            return;
        }
    }

    // Same item → stack; otherwise move/swap within general/backpack.
    if (tgt.itemId && tgt.itemId === src.itemId) {
        await performAction('stack', src.itemId, src.index, tgt.index, src.surface, tgt.surface, showMessage, showVaultUI, showActionText);
    } else {
        await performAction('move', src.itemId, src.index, tgt.index, src.surface, tgt.surface, showMessage, showVaultUI, showActionText);
    }
}

/** Vault transfer via move_item, with surgical delta + vault UI refresh. */
async function vaultMove(src, toIndex, buildingId, toSurface) {
    const params = src.surface === 'vault'
        ? { item_id: src.itemId, from_slot: src.index, from_slot_type: 'vault', to_slot: toIndex, to_slot_type: toSurface === 'inventory' ? 'inventory' : 'general', vault_building: buildingId }
        : { item_id: src.itemId, from_slot: src.index, from_slot_type: src.surface, to_slot: toIndex, to_slot_type: 'vault', vault_building: buildingId };
    try {
        const result = await gameAPI.sendAction('move_item', params);
        if (result.success) {
            if (result.delta) deltaApplier.applyDelta(result.delta);
            await refreshGameState(true);
            await updateCharacterDisplay();
            const vaultData = result.delta?.vault_data;
            if (vaultData) showVaultUI(vaultData);
        } else {
            showMessage(result.error || result.message || 'Failed to move item', 'error');
        }
    } catch (err) {
        logger.error('vaultMove failed:', err);
        showMessage('Failed to move item', 'error');
    }
}

/** Add a dragged item into a container that's sitting (closed) in a general slot. */
async function addToClosedContainer(src, tgt) {
    try {
        const result = await gameAPI.sendAction('add_to_container', {
            item_id: src.itemId,
            from_slot: src.index,
            from_slot_type: src.surface,
            container_slot: tgt.index,
            container_slot_type: tgt.surface,
        });
        if (result.success) {
            await refreshGameState(true);
            await updateCharacterDisplay();
            if (result.message) showActionText(result.message, result.color || 'green', 3000);
        } else {
            showMessage(result.error || result.message || 'Cannot add to container', 'error');
        }
    } catch (err) {
        logger.error('addToClosedContainer failed:', err);
        showMessage('Cannot add to container', 'error');
    }
}

// --- context menu --------------------------------------------------------

function openContextMenu(desc, x, y, ev) {
    if (ev) { ev.preventDefault(); ev.stopPropagation(); }
    const itemData = getItemById(desc.itemId);
    if (!itemData) return;
    const quantity = readSlotQuantity(desc);
    const actions = getItemActions({ ...itemData, quantity }, desc.surface === 'equipment');
    const ref = desc.surface === 'equipment' ? desc.slotName : desc.index;
    showContextMenu(x, y, desc.itemId, ref, desc.surface, actions);
}

/** Read the live stack quantity at a slot (for the Split action gate). */
function readSlotQuantity(desc) {
    const s = getGameStateSync();
    if (desc.surface === 'general') {
        const arr = Array.isArray(s.inventory) ? s.inventory : [];
        return arr[desc.index]?.quantity || 1;
    }
    if (desc.surface === 'inventory') {
        const bp = s.equipment?.bag?.contents || [];
        return bp[desc.index]?.quantity || 1;
    }
    return 1;
}

// --- hover label (cursor-following, mouse only) --------------------------

let tipEl = null;

function ensureTip() {
    if (tipEl) return tipEl;
    tipEl = document.createElement('div');
    tipEl.className = 'slot-hover-tip';
    tipEl.style.cssText =
        'position:fixed;z-index:2100;pointer-events:none;display:none;' +
        'background:#1a1a1a;color:#fff;border:1px solid #6a6a6a;' +
        'padding:2px 6px;font-size:10px;white-space:nowrap;' +
        'box-shadow:1px 1px 0 #000;';
    document.body.appendChild(tipEl);
    return tipEl;
}

// Default left-click/tap action shown before the item name, matching the old
// bottom-corner action hint: label + color per action.
const ACTION_LABELS = {
    equip: 'Equip', unequip: 'Unequip', use: 'Use', open: 'Open',
    examine: 'Info', drop: 'Drop', remove: 'Remove',
};
const ACTION_COLORS = {
    equip: '#4a9eff',   // blue
    unequip: '#ff8c00', // orange
    use: '#00ff00',     // green
    open: '#00ff00',    // green
    examine: '#ff8c00', // orange
    drop: '#ff0000',    // red
    remove: '#ff8c00',  // orange
};

function onHoverMove(e) {
    // Don't fight a drag, and don't show on touch-driven synthetic mouse moves.
    if (pointerState && pointerState.dragging) { hideTip(); return; }
    const desc = readDescriptor(e.target.closest && e.target.closest(SLOT_SELECTOR));
    if (!desc || !desc.itemId) { hideTip(); return; }
    const itemData = getItemById(desc.itemId);
    if (!itemData) { hideTip(); return; }

    // What a tap will do: container slot → remove; otherwise the item's default.
    const action = desc.surface === 'container'
        ? 'remove'
        : getDefaultAction(itemData, desc.surface === 'equipment');
    const label = ACTION_LABELS[action] || 'Info';
    const color = ACTION_COLORS[action] || '#ff8c00';

    const tip = ensureTip();
    tip.textContent = '';
    const verb = document.createElement('span');
    verb.textContent = label;
    verb.style.color = color;
    verb.style.fontWeight = 'bold';
    tip.appendChild(verb);
    tip.appendChild(document.createTextNode(' ' + (itemData.name || desc.itemId)));

    tip.style.display = 'block';
    tip.style.left = `${e.clientX + 14}px`;
    tip.style.top = `${e.clientY + 14}px`;
}

function hideTip() {
    if (tipEl) tipEl.style.display = 'none';
}

// Initialize as soon as the module loads (DOM-ready aware).
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initSlotInteractions);
} else {
    initSlotInteractions();
}
