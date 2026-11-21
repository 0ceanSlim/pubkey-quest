/**
 * The enchanter's table (docs/draft/enchanting.md): the plain rings and
 * necklaces you carry, what each would become, the fee and the chance. The
 * server builds the list, rolls the chance and does the work (the enchant
 * action). A failure costs the fee; the piece comes back as it was.
 *
 * @module ui/enchanter
 */

import { gameAPI } from '../lib/api.js';
import { refreshGameState } from '../state/gameState.js';
import { showMessage } from './messaging.js';

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

const fmt = (n) => Number(n || 0).toLocaleString('en-US');

function pieceRow(r, gold) {
    if (r.refusal) {
        return `
      <div class="en-row en-off">
        <img src="${esc(r.image)}" alt="" onerror="this.style.visibility='hidden'">
        <div class="en-info">
          <div>${esc(r.name)}${r.have > 1 ? ` <span class="en-dim">×${r.have}</span>` : ''}</div>
          <div class="en-dim">${esc(r.refusal)}</div>
        </div>
      </div>`;
    }
    return `
      <div class="en-row">
        <img src="${esc(r.image)}" alt="" onerror="this.style.visibility='hidden'">
        <div class="en-info">
          <div>${esc(r.name)}${r.have > 1 ? ` <span class="en-dim">×${r.have}</span>` : ''}</div>
          <div class="en-makes">→ ${esc(r.makes)}</div>
          <div class="en-effect">${esc(r.effect)}</div>
          <div><span class="en-cost">${fmt(r.fee)} gold</span> <span class="en-dim">· ${r.chance}% to take</span></div>
        </div>
        <button type="button" class="en-btn" ${gold >= r.fee ? '' : 'disabled'}>Enchant</button>
      </div>`;
}

function render(el, data) {
    el.querySelector('.en-title').textContent = `✨ ${data.npc_name}: Enchanting`;
    const works = (data.works ?? []).map((p) => `${p}s`).join(' and ');
    const what = data.gems?.length ? 'a few stones they know' : 'any stone';
    el.querySelector('.en-note').textContent = `You have ${fmt(data.gold)} gold. Works ${works}, ${what}, up to ${data.max_size}. The fee is paid either way.`;
    const list = el.querySelector('.en-list');
    const rows = data.pieces ?? [];
    if (!rows.length) {
        list.innerHTML = `<div class="en-dim" style="padding:6px">"Nothing to wake. Bring me a ring or a necklace with a stone set in it."</div>`;
        return;
    }
    list.innerHTML = rows.map((r) => pieceRow(r, data.gold)).join('');
    list.querySelectorAll('.en-row').forEach((rowEl, i) => {
        rowEl.querySelector('.en-btn')?.addEventListener('click', () => work(el, data.npc_id, rows[i]));
    });
}

async function work(el, npcId, r) {
    el.querySelectorAll('.en-btn').forEach((b) => { b.disabled = true; });
    try {
        const result = await gameAPI.sendAction('enchant', { npc_id: npcId, item_id: r.item_id });
        const tone = result.success === false || result.data?.enchant_failed ? 'warning' : 'success';
        showMessage(result.message || 'Done.', tone);
        if (result.data?.enchanter) render(el, result.data.enchanter);
    } catch (err) {
        showMessage(err?.message || "They can't do that.", 'warning');
    }
    await refreshGameState();
    window.updateCharacterDisplay?.();
}

/** Open the enchanter's table with the server's list. */
export function openEnchanter(data) {
    document.getElementById('enchanter-table')?.remove();
    const host = document.getElementById('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'enchanter-table';
    el.innerHTML = `
      <style>
        #enchanter-table { position: ${host === document.body ? 'fixed' : 'absolute'}; inset: 0; z-index: 8400;
          display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.45); }
        #enchanter-table .en-box { width: 300px; max-width: calc(100% - 16px); background: #1f1f1f; color: #e5e7eb;
          font-family: 'Dogica', monospace; font-size: 7px;
          border-top: 2px solid #6b7280; border-left: 2px solid #6b7280; border-right: 2px solid #000; border-bottom: 2px solid #000; }
        #enchanter-table .en-head { padding: 2px 5px; background: linear-gradient(90deg, #1e3a8a, #0e7490); color: #fff; font-weight: bold;
          display: flex; justify-content: space-between; }
        #enchanter-table .en-head button { background: none; border: none; color: #fff; cursor: pointer; font: inherit; }
        #enchanter-table .en-note { padding: 3px 5px; color: #9ca3af; border-bottom: 1px solid #3f3f46; line-height: 1.4; }
        #enchanter-table .en-list { padding: 3px; display: flex; flex-direction: column; gap: 2px; max-height: 280px; overflow-y: auto; }
        #enchanter-table .en-row { display: flex; align-items: center; gap: 5px; padding: 3px 4px; background: #2a2a2a; border: 1px solid #3f3f46; }
        #enchanter-table .en-off { opacity: 0.6; }
        #enchanter-table .en-row img { width: 22px; height: 22px; image-rendering: pixelated; flex: none; }
        #enchanter-table .en-info { flex: 1; display: flex; flex-direction: column; gap: 2px; min-width: 0; line-height: 1.4; }
        #enchanter-table .en-makes { color: #67e8f9; }
        #enchanter-table .en-effect { color: #d1d5db; }
        #enchanter-table .en-cost { color: #fcd34d; }
        #enchanter-table .en-dim { color: #9ca3af; }
        #enchanter-table .en-btn { font: inherit; color: #fff; background: #0e7490; cursor: pointer; padding: 2px 5px; flex: none;
          border-top: 1px solid #67e8f9; border-left: 1px solid #67e8f9; border-right: 1px solid #164e63; border-bottom: 1px solid #164e63; }
        #enchanter-table .en-btn:disabled { background: #3f3f46; color: #6b7280; cursor: default; border-color: #27272a; }
      </style>
      <div class="en-box" role="dialog" aria-label="Enchanter">
        <div class="en-head"><span class="en-title"></span><button type="button" aria-label="Close">✕</button></div>
        <div class="en-note"></div>
        <div class="en-list"></div>
      </div>`;
    const close = () => el.remove();
    el.querySelector('.en-head button').addEventListener('click', close);
    el.addEventListener('click', (e) => { if (e.target === el) close(); });
    host.appendChild(el);
    render(el, data);
}
