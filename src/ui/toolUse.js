/**
 * Using a tool on an item (degradation §6): a tool's "Use on…" (the tinderbox's
 * "Light…") opens a small box of the items you carry that it works on, asked
 * of the server (use_targets), so it never offers a pair that does nothing.
 * Picking one does it (use_on). The item side uses the same call: a torch's
 * "Light" is the tinderbox used on that torch.
 *
 * @module ui/toolUse
 */

import { gameAPI } from '../lib/api.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getItemById } from '../state/staticData.js';
import { showMessage } from './messaging.js';

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** An item's field, wherever the game data put it. */
export function itemField(item, name) {
    return item?.[name] ?? item?.properties?.[name];
}

const TOOL_VERBS = { light: 'Light…', sharpen: 'Sharpen…', mend: 'Mend…', salt: 'Salt…' };

/** The tool's verb: the tinderbox "Light…", the whetstone "Sharpen…", the crafting kit "Mend…", anything else "Use on…". */
export function toolLabel(item) {
    return TOOL_VERBS[itemField(item, 'use_on')] ?? 'Use on…';
}

/** Whether the player carries an item (pack, backpack, containers, hands). */
export function carries(itemId) {
    const inv = getGameStateSync()?.character?.inventory ?? {};
    const lists = [inv.general_slots ?? [], inv.gear_slots?.bag?.contents ?? []];
    for (const list of [...lists]) {
        for (const s of list) if (Array.isArray(s?.contents)) lists.push(s.contents);
    }
    if (lists.some((list) => list.some((s) => s?.item === itemId))) return true;
    return Object.values(inv.gear_slots ?? {}).some((s) => s?.item === itemId);
}

/** Whether the player carries raw food that cooks (a raw food's "cook"). */
export function carriesRawFood() {
    const inv = getGameStateSync()?.character?.inventory ?? {};
    const lists = [inv.general_slots ?? [], inv.gear_slots?.bag?.contents ?? []];
    return lists.some((list) => list.some((s) => s?.item && itemField(getItemById(s.item), 'cook')));
}

/** Whether the light in a gear slot is burning. */
export function isLitInHand(gearSlot) {
    const slot = getGameStateSync()?.character?.inventory?.gear_slots?.[gearSlot];
    return !!slot && slot.l !== undefined && slot.l !== null;
}

/** Whether a lit lantern's shutter is closed. */
export function isShutInHand(gearSlot) {
    const slot = getGameStateSync()?.character?.inventory?.gear_slots?.[gearSlot];
    return !!slot && slot.s !== undefined && slot.s !== null;
}

/** Open or close the held hooded lantern's shutter. */
export async function toggleShutter() {
    try {
        const result = await gameAPI.sendAction('shutter', {});
        showMessage(result.message || 'Done.', 'info');
    } catch (err) {
        showMessage(err?.message || "It won't budge.", 'warning');
    }
    await refreshGameState();
    window.updateCharacterDisplay?.();
}

/** A slot as the server addresses it. */
export function slotRef(fromSlot, fromSlotType) {
    if (fromSlotType === 'equipment') return { slot_type: 'equipment', gear: fromSlot, slot: -1 };
    return { slot_type: fromSlotType, slot: fromSlot };
}

/** Send a small item action (drink, fill_water) and redraw. */
export async function simpleAction(type, params) {
    try {
        const result = await gameAPI.sendAction(type, params);
        showMessage(result.message || 'Done.', result.success === false ? 'warning' : 'info');
    } catch (err) {
        showMessage(err?.message || 'Nothing happens.', 'warning');
    }
    await refreshGameState();
    window.updateCharacterDisplay?.();
}

/** Use a tool on the item in a slot. */
export async function useOn(toolId, ref) {
    try {
        const result = await gameAPI.sendAction('use_on', { item_id: toolId, ...ref });
        showMessage(result.message || 'Done.', 'info');
    } catch (err) {
        showMessage(err?.message || 'Nothing interesting happens.', 'warning');
    }
    await refreshGameState();
    window.updateCharacterDisplay?.();
}

/** Open a tool's picker: what it works on, and pick one. */
export async function openToolPicker(toolId) {
    let targets = [];
    try {
        const result = await gameAPI.sendAction('use_targets', { item_id: toolId });
        targets = result.data?.targets ?? [];
    } catch (err) {
        showMessage(err?.message || "Couldn't use that.", 'warning');
        return;
    }
    const tool = getItemById(toolId);
    if (!targets.length) {
        const use = itemField(tool, 'use_on');
        showMessage(use === 'light' ? "You've nothing to light."
            : use === 'salt' ? "You've no raw meat or fish to salt."
            : use === 'sharpen' || use === 'mend' ? `Nothing of yours needs the ${tool?.name ?? toolId}: field repair only goes up to Worn.`
            : `There's nothing to use the ${tool?.name ?? toolId} on.`, 'info');
        return;
    }

    document.getElementById('tool-picker')?.remove();
    const host = document.getElementById('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'tool-picker';
    el.innerHTML = `
      <style>
        #tool-picker { position: ${host === document.body ? 'fixed' : 'absolute'}; inset: 0; z-index: 8400;
          display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.45); }
        #tool-picker .tp-box { width: 200px; max-width: calc(100% - 16px); background: #1f1f1f; color: #e5e7eb;
          font-family: 'Dogica', monospace; font-size: 7px;
          border-top: 2px solid #6b7280; border-left: 2px solid #6b7280; border-right: 2px solid #000; border-bottom: 2px solid #000; }
        #tool-picker .tp-head { padding: 2px 5px; background: linear-gradient(90deg, #854d0e, #a16207); color: #fff; font-weight: bold;
          display: flex; justify-content: space-between; }
        #tool-picker .tp-head button { background: none; border: none; color: #fff; cursor: pointer; font: inherit; }
        #tool-picker .tp-list { padding: 3px; display: flex; flex-direction: column; gap: 2px; max-height: 220px; overflow-y: auto; }
        #tool-picker .tp-row { display: flex; align-items: center; gap: 5px; padding: 2px 4px; cursor: pointer; text-align: left;
          font: inherit; color: inherit; background: #2a2a2a; border: 1px solid #3f3f46; }
        #tool-picker .tp-row:hover { background: #3f3f46; border-color: #a16207; }
        #tool-picker .tp-row img { width: 16px; height: 16px; image-rendering: pixelated; }
      </style>
      <div class="tp-box" role="dialog" aria-label="${esc(tool?.name ?? toolId)}">
        <div class="tp-head"><span>${esc(toolLabel(tool).replace('…', ' what?'))}</span><button type="button" aria-label="Close">✕</button></div>
        <div class="tp-list"></div>
      </div>`;
    const list = el.querySelector('.tp-list');
    for (const t of targets) {
        const row = document.createElement('button');
        row.type = 'button';
        row.className = 'tp-row';
        row.innerHTML = `<img src="/res/img/items/${esc(t.item_id)}.png" alt="" onerror="this.style.visibility='hidden'"><span>${esc(t.label)}</span>`;
        row.addEventListener('click', () => {
            el.remove();
            useOn(toolId, { slot_type: t.slot_type, slot: t.slot ?? -1, gear: t.gear ?? '' });
        });
        list.appendChild(row);
    }
    const close = () => el.remove();
    el.querySelector('.tp-head button').addEventListener('click', close);
    el.addEventListener('click', (e) => { if (e.target === el) close(); });
    host.appendChild(el);
}
