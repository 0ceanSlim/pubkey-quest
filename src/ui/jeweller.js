/**
 * The jeweller's bench (gem-system.md §3): the gems you carry, rough ones to
 * cut (a fee by size; the wheel reveals the quality) and cut ones to set in a
 * gold ring or necklace. The server builds the list and does the work
 * (the jeweller action).
 *
 * @module ui/jeweller
 */

import { gameAPI } from '../lib/api.js';
import { refreshGameState } from '../state/gameState.js';
import { showMessage } from './messaging.js';

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

const fmt = (n) => Number(n || 0).toLocaleString('en-US');

function roughRow(r, gold) {
    return `
      <div class="jw-row">
        <img src="${esc(r.image)}" alt="" onerror="this.style.visibility='hidden'">
        <div class="jw-info">
          <div>${esc(r.name)}${r.have > 1 ? ` <span class="jw-dim">×${r.have}</span>` : ''}</div>
          <div class="jw-cost">${fmt(r.fee)} gold <span class="jw-dim">· sells rough for ${fmt(r.value)}</span></div>
        </div>
        <button type="button" class="jw-btn" data-op="cut" ${gold >= r.fee ? '' : 'disabled'}>Cut</button>
      </div>`;
}

function cutRow(r, gold) {
    const pieces = (r.pieces ?? []).map((p) => {
        const total = p.gold + p.labour;
        return `<button type="button" class="jw-btn" data-op="set" data-piece="${esc(p.piece)}" ${gold >= total ? '' : 'disabled'}
          title="${esc(p.name)}: ${fmt(p.gold)} gold + ${fmt(p.labour)} for the work. Worth ${fmt(p.value)}.">${esc(p.name)} ${fmt(total)}</button>`;
    }).join('');
    return `
      <div class="jw-row">
        <img src="${esc(r.image)}" alt="" onerror="this.style.visibility='hidden'">
        <div class="jw-info">
          <div>${esc(r.name)}${r.have > 1 ? ` <span class="jw-dim">×${r.have}</span>` : ''}</div>
          <div class="jw-dim">Worth ${fmt(r.value)} gold. Set it:</div>
          <div class="jw-pieces">${pieces}</div>
        </div>
      </div>`;
}

function render(el, data) {
    el.querySelector('.jw-title').textContent = `💎 ${data.npc_name}: Gems`;
    el.querySelector('.jw-note').textContent = `You have ${fmt(data.gold)} gold.`;
    const list = el.querySelector('.jw-list');
    const rough = data.rough ?? [];
    const cut = data.cut ?? [];
    if (!rough.length && !cut.length) {
        list.innerHTML = `<div class="jw-dim" style="padding:6px">"No stones on you. Bring me something that glitters."</div>`;
        return;
    }
    let html = '';
    if (rough.length) html += `<div class="jw-head2">Rough: cut to see what it is</div>` + rough.map((r) => roughRow(r, data.gold)).join('');
    if (cut.length) html += `<div class="jw-head2">Cut: set in gold</div>` + cut.map((r) => cutRow(r, data.gold)).join('');
    list.innerHTML = html;
    const rows = [...rough, ...cut];
    list.querySelectorAll('.jw-row').forEach((rowEl, i) => {
        rowEl.querySelectorAll('.jw-btn').forEach((btn) => {
            btn.addEventListener('click', () => work(el, data.npc_id, rows[i], btn.dataset.op, btn.dataset.piece));
        });
    });
}

async function work(el, npcId, r, op, piece) {
    el.querySelectorAll('.jw-btn').forEach((b) => { b.disabled = true; });
    try {
        const result = await gameAPI.sendAction('jeweller', { npc_id: npcId, op, item_id: r.item_id, piece: piece || '' });
        showMessage(result.message || 'Done.', result.success === false ? 'warning' : 'success');
        if (result.data?.jeweller) render(el, result.data.jeweller);
    } catch (err) {
        showMessage(err?.message || "She can't do that.", 'warning');
    }
    await refreshGameState();
    window.updateCharacterDisplay?.();
}

/** Open the jeweller's bench with the server's list. */
export function openJeweller(data) {
    document.getElementById('jeweller-bench')?.remove();
    const host = document.getElementById('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'jeweller-bench';
    el.innerHTML = `
      <style>
        #jeweller-bench { position: ${host === document.body ? 'fixed' : 'absolute'}; inset: 0; z-index: 8400;
          display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.45); }
        #jeweller-bench .jw-box { width: 290px; max-width: calc(100% - 16px); background: #1f1f1f; color: #e5e7eb;
          font-family: 'Dogica', monospace; font-size: 7px;
          border-top: 2px solid #6b7280; border-left: 2px solid #6b7280; border-right: 2px solid #000; border-bottom: 2px solid #000; }
        #jeweller-bench .jw-head { padding: 2px 5px; background: linear-gradient(90deg, #4c1d95, #6d28d9); color: #fff; font-weight: bold;
          display: flex; justify-content: space-between; }
        #jeweller-bench .jw-head button { background: none; border: none; color: #fff; cursor: pointer; font: inherit; }
        #jeweller-bench .jw-note { padding: 3px 5px; color: #9ca3af; border-bottom: 1px solid #3f3f46; }
        #jeweller-bench .jw-list { padding: 3px; display: flex; flex-direction: column; gap: 2px; max-height: 260px; overflow-y: auto; }
        #jeweller-bench .jw-head2 { color: #c4b5fd; padding: 3px 2px 1px; }
        #jeweller-bench .jw-row { display: flex; align-items: center; gap: 5px; padding: 3px 4px; background: #2a2a2a; border: 1px solid #3f3f46; }
        #jeweller-bench .jw-row img { width: 22px; height: 22px; image-rendering: pixelated; flex: none; }
        #jeweller-bench .jw-info { flex: 1; display: flex; flex-direction: column; gap: 2px; min-width: 0; }
        #jeweller-bench .jw-cost { color: #fcd34d; }
        #jeweller-bench .jw-dim { color: #9ca3af; }
        #jeweller-bench .jw-pieces { display: flex; gap: 3px; flex-wrap: wrap; }
        #jeweller-bench .jw-btn { font: inherit; color: #fff; background: #6d28d9; cursor: pointer; padding: 2px 5px; flex: none;
          border-top: 1px solid #c4b5fd; border-left: 1px solid #c4b5fd; border-right: 1px solid #2e1065; border-bottom: 1px solid #2e1065; }
        #jeweller-bench .jw-btn:disabled { background: #3f3f46; color: #6b7280; cursor: default; border-color: #27272a; }
      </style>
      <div class="jw-box" role="dialog" aria-label="Jeweller">
        <div class="jw-head"><span class="jw-title"></span><button type="button" aria-label="Close">✕</button></div>
        <div class="jw-note"></div>
        <div class="jw-list"></div>
      </div>`;
    const close = () => el.remove();
    el.querySelector('.jw-head button').addEventListener('click', close);
    el.addEventListener('click', (e) => { if (e.target === el) close(); });
    host.appendChild(el);
    render(el, data);
}
