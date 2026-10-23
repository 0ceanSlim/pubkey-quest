/**
 * Quest complete: the rewards screen, OSRS-style. Every quest that finishes
 * (quests_completed on a response, with its rewards) gets one, in the game
 * window: the quest's name, "Quest complete!", and what it gave (XP, gold,
 * items, quest points). It waits while something else holds the screen (a
 * conversation, a fight, the level-up window, the tour), so a turn-in plays
 * out the giver's last lines first. One at a time, in order.
 *
 * @module ui/questComplete
 */

import { smoothClock } from '../systems/smoothClock.js';

const queue = [];
let showing = false;
let retry = null;

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

function isOpen(id) {
    const el = document.getElementById(id);
    return !!el && !el.classList.contains('hidden') && el.style.display !== 'none';
}

/**
 * Something else has the screen: the rewards wait their turn. A turn-in's own
 * conversation is closing as it shows, so it doesn't wait for that.
 */
function busy(q) {
    const holds = smoothClock.holds ?? new Set();
    return (holds.has('dialogue') && !q?.closing) || holds.has('combat') || holds.has('poi') || holds.has('encounter')
        || window.pqTourPending || document.getElementById('ui-tour') || document.getElementById('whats-new')
        || document.getElementById('gear-overlay') || document.getElementById('letter-reader')
        || document.getElementById('tutorial-tip') || isOpen('level-up-modal');
}

/**
 * Queue a finished quest's rewards screen.
 * @param {{id: string, name: string, rewards?: {xp?: number, gold?: number, quest_points?: number, items?: {id: string, name: string, quantity: number}[]}}} q
 */
export function showQuestComplete(q) {
    if (!q) return;
    queue.push(q);
    pump();
}

function pump() {
    if (showing || !queue.length) return;
    if (busy(queue[0])) {
        clearTimeout(retry);
        retry = setTimeout(pump, 600);
        return;
    }
    showing = true;
    render(queue.shift());
}

function render(q) {
    const r = q.rewards || {};
    const lines = [];
    if (r.xp) lines.push(`<li><span class="qc-ic">⭐</span>${Number(r.xp).toLocaleString()} XP</li>`);
    if (r.gold) lines.push(`<li><img src="/res/img/items/gold-piece.png" alt="">${Number(r.gold).toLocaleString()} gold</li>`);
    for (const it of r.items || []) {
        const qty = it.quantity > 1 ? `${it.quantity} × ` : '';
        lines.push(`<li><img src="/res/img/items/${esc(it.id)}.png" alt="" onerror="this.style.visibility='hidden'">${qty}${esc(it.name)}</li>`);
    }
    if (r.quest_points) lines.push(`<li><span class="qc-ic">📜</span>${r.quest_points} quest point${r.quest_points === 1 ? '' : 's'}</li>`);

    // A turn-in carries the giver's last words.
    const closing = q.closing
        ? `<div class="qc-quote">${esc(q.closing)}${q.speaker ? `<div class="qc-who">— ${esc(q.speaker)}</div>` : ''}</div>`
        : '';

    const host = document.getElementById('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'quest-complete';
    el.innerHTML = `
      <style>
        #quest-complete { position: ${host === document.body ? 'fixed' : 'absolute'}; inset: 0; z-index: 8600;
          display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,0.6);
          font-family: 'Dogica', monospace; animation: qc-in 0.25s ease-out; }
        @keyframes qc-in { from { opacity: 0; } to { opacity: 1; } }
        #quest-complete .qc-box { width: 300px; max-width: calc(100% - 16px); box-sizing: border-box;
          background: #2b2416; color: #f5e6c8; font-size: 7px; line-height: 1.7; text-align: center;
          border-top: 2px solid #a16207; border-left: 2px solid #a16207; border-right: 2px solid #000; border-bottom: 2px solid #000;
          box-shadow: 0 2px 14px rgba(0,0,0,0.8); padding: 10px 12px 9px; }
        #quest-complete .qc-title { font-size: 12px; color: #fbbf24; text-shadow: 1px 1px 0 #000; margin-bottom: 2px; }
        #quest-complete .qc-name { font-size: 8px; color: #fde68a; margin-bottom: 8px; }
        #quest-complete .qc-sub { color: #d6c7a1; margin-bottom: 4px; }
        #quest-complete .qc-quote { font-family: 'Caveat', cursive; font-size: 13px; line-height: 1.25; color: #f5e6c8;
          text-align: left; margin: 0 0 9px; padding: 5px 7px; background: rgba(0,0,0,0.25); border-left: 2px solid #a16207;
          max-height: 140px; overflow-y: auto; }
        #quest-complete .qc-who { font-family: 'Dogica', monospace; font-size: 6px; color: #d6c7a1; text-align: right; margin-top: 3px; }
        #quest-complete ul { list-style: none; margin: 0 auto 10px; padding: 0; display: inline-block; text-align: left; }
        #quest-complete li { display: flex; align-items: center; gap: 6px; }
        #quest-complete li img, #quest-complete .qc-ic { width: 16px; height: 16px; image-rendering: pixelated; text-align: center; font-size: 10px; line-height: 16px; }
        #quest-complete button { font: inherit; font-size: 8px; color: #fff; cursor: pointer; padding: 3px 14px; background: #0e7490;
          border: 2px solid; border-color: #22d3ee #155e75 #155e75 #22d3ee; }
      </style>
      <div class="qc-box" role="dialog" aria-label="Quest complete">
        <div class="qc-title">Quest complete!</div>
        <div class="qc-name">${esc(q.name)}</div>
        ${closing}
        ${lines.length ? `<div class="qc-sub">You are awarded:</div><ul>${lines.join('')}</ul>` : ''}
        <div><button type="button">Continue</button></div>
      </div>`;
    const close = () => {
        el.remove();
        showing = false;
        pump();
    };
    el.querySelector('button').addEventListener('click', close);
    host.appendChild(el);
    el.querySelector('button').focus();
}

if (typeof window !== 'undefined') {
    window.showQuestComplete = showQuestComplete;
}
