/**
 * The smith's repair window (degradation §5): every worn item you carry or
 * wear, how worn it is, and what this smith asks to mend it (gold, sometimes a
 * material). The server builds the list and does the repair (smith_repair).
 *
 * @module ui/smithRepair
 */

import { gameAPI } from '../lib/api.js';
import { refreshGameState } from '../state/gameState.js';
import { showMessage } from './messaging.js';

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

const TIER_COLOR = { Pristine: '#4ade80', Worn: '#facc15', Damaged: '#fb923c', Ruined: '#f87171' };

function rowHTML(r) {
    const q = r.quote ?? {};
    let cost, can = false;
    if (r.reason) {
        cost = `<span class="sr-dim">${esc(r.reason)}</span>`;
    } else {
        const parts = [`${q.gold} gold`];
        if (q.material) parts.push(`${q.count} ${esc(r.material_name)} <span class="sr-dim">(have ${r.have ?? 0})</span>`);
        cost = parts.join(' + ') + (q.pristine ? '' : ' <span class="sr-dim">→ Worn</span>');
        can = !r.missing;
    }
    return `
      <div class="sr-row">
        <img src="/res/img/items/${esc(r.item_id)}.png" alt="" onerror="this.style.visibility='hidden'">
        <div class="sr-info">
          <div>${esc(r.name)} <span style="color:${TIER_COLOR[r.tier] ?? '#e5e7eb'}">${esc(r.tier)} ${r.left}%</span></div>
          <div class="sr-cost">${cost}${r.missing ? ` <span class="sr-miss">${esc(r.missing)}</span>` : ''}</div>
        </div>
        ${r.reason ? '' : `<button type="button" class="sr-btn" ${can ? '' : 'disabled'}>Repair</button>`}
      </div>`;
}

function render(el, data) {
    el.querySelector('.sr-title').textContent = `🔨 ${data.npc_name}: Repairs`;
    const notes = [];
    if (data.rough) notes.push('Rough work: only back to Worn.');
    if (data.discount) notes.push(esc(data.discount) + '.');
    notes.push(`You have ${data.gold} gold.`);
    el.querySelector('.sr-note').innerHTML = notes.join(' ');
    const list = el.querySelector('.sr-list');
    const items = data.items ?? [];
    if (!items.length) {
        list.innerHTML = `<div class="sr-dim" style="padding:6px">"Nothing of yours needs work."</div>`;
        return;
    }
    list.innerHTML = items.map(rowHTML).join('');
    list.querySelectorAll('.sr-row').forEach((rowEl, i) => {
        rowEl.querySelector('.sr-btn')?.addEventListener('click', () => repair(el, data.npc_id, items[i]));
    });
}

async function repair(el, npcId, r) {
    try {
        const result = await gameAPI.sendAction('smith_repair', {
            npc_id: npcId, slot_type: r.slot_type, slot: r.slot ?? -1, gear: r.gear ?? '',
        });
        showMessage(result.message || 'Done.', result.success === false ? 'warning' : 'success');
        if (result.data?.repair) render(el, result.data.repair);
    } catch (err) {
        showMessage(err?.message || "They can't do that.", 'warning');
    }
    await refreshGameState();
    window.updateCharacterDisplay?.();
}

/** Open the repair window with the server's list. */
export function openSmithRepair(data) {
    document.getElementById('smith-repair')?.remove();
    const host = document.getElementById('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'smith-repair';
    el.innerHTML = `
      <style>
        #smith-repair { position: ${host === document.body ? 'fixed' : 'absolute'}; inset: 0; z-index: 8400;
          display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.45); }
        #smith-repair .sr-box { width: 280px; max-width: calc(100% - 16px); background: #1f1f1f; color: #e5e7eb;
          font-family: 'Dogica', monospace; font-size: 7px;
          border-top: 2px solid #6b7280; border-left: 2px solid #6b7280; border-right: 2px solid #000; border-bottom: 2px solid #000; }
        #smith-repair .sr-head { padding: 2px 5px; background: linear-gradient(90deg, #7c2d12, #9a3412); color: #fff; font-weight: bold;
          display: flex; justify-content: space-between; }
        #smith-repair .sr-head button { background: none; border: none; color: #fff; cursor: pointer; font: inherit; }
        #smith-repair .sr-note { padding: 3px 5px; color: #9ca3af; border-bottom: 1px solid #3f3f46; }
        #smith-repair .sr-list { padding: 3px; display: flex; flex-direction: column; gap: 2px; max-height: 240px; overflow-y: auto; }
        #smith-repair .sr-row { display: flex; align-items: center; gap: 5px; padding: 3px 4px; background: #2a2a2a; border: 1px solid #3f3f46; }
        #smith-repair .sr-row img { width: 20px; height: 20px; image-rendering: pixelated; flex: none; }
        #smith-repair .sr-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
        #smith-repair .sr-cost { color: #fcd34d; }
        #smith-repair .sr-dim { color: #9ca3af; }
        #smith-repair .sr-miss { color: #f87171; }
        #smith-repair .sr-btn { font: inherit; color: #fff; background: #9a3412; cursor: pointer; padding: 2px 5px;
          border-top: 1px solid #fdba74; border-left: 1px solid #fdba74; border-right: 1px solid #431407; border-bottom: 1px solid #431407; }
        #smith-repair .sr-btn:disabled { background: #3f3f46; color: #6b7280; cursor: default; border-color: #27272a; }
      </style>
      <div class="sr-box" role="dialog" aria-label="Repairs">
        <div class="sr-head"><span class="sr-title"></span><button type="button" aria-label="Close">✕</button></div>
        <div class="sr-note"></div>
        <div class="sr-list"></div>
      </div>`;
    const close = () => el.remove();
    el.querySelector('.sr-head button').addEventListener('click', close);
    el.addEventListener('click', (e) => { if (e.target === el) close(); });
    host.appendChild(el);
    render(el, data);
}
