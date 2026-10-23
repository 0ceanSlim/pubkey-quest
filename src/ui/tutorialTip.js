/**
 * Tutorial tips: a quest stage can carry a "how it works" note (QuestStage.tip,
 * sent as stage_tip in the quest log). Tips are pulled, not pushed: the quest
 * tracker shows a 💡 while the tracked step has one, and clicking it opens the
 * note. Only a stage flagged tip_push (the vault, the shop, saving) shows its
 * tip by itself, once, and only when nothing else is on screen (a
 * conversation, a window, the tour). The NPCs say the rest in their own words.
 * Seen tips are remembered per browser (localStorage), not saved.
 *
 * @module ui/tutorialTip
 */

import { smoothClock } from '../systems/smoothClock.js';

const SEEN_KEY = 'pq_tips_seen';

function seen() {
    try { return new Set(JSON.parse(localStorage.getItem(SEEN_KEY) || '[]')); } catch (_) { return new Set(); }
}

/** Whether a quest's current-stage tip has been opened (or shown) yet. */
export function tipSeen(quest) {
    return seen().has(`${quest.id}:${quest.stage}`);
}

function markSeen(key) {
    try {
        const s = seen();
        s.add(key);
        localStorage.setItem(SEEN_KEY, JSON.stringify([...s].slice(-200)));
    } catch (_) { /* storage blocked */ }
}

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** An element is present and showing. */
function isOpen(id) {
    const el = document.getElementById(id);
    return !!el && !el.classList.contains('hidden') && el.style.display !== 'none';
}

/** Something else is on screen (the tour, a pop-up), so the tip should wait. */
function busy() {
    const holds = smoothClock.holds ?? new Set();
    return holds.has('dialogue') || holds.has('combat') || holds.has('poi') || holds.has('encounter')
        || window.pqTourPending || document.getElementById('ui-tour') || document.getElementById('whats-new')
        || document.getElementById('quest-complete')
        || document.getElementById('gear-overlay') || document.getElementById('letter-reader')
        || isOpen('vault-overlay') || isOpen('shop-modal')
        || !document.getElementById('save-modal')?.classList.contains('hidden');
}

let _pending = null;

/**
 * Show a quest's current-stage tip by itself, if the stage pushes it
 * (stage_tip_push) and it hasn't been shown before.
 * @param {{id: string, name: string, stage: number, stage_tip?: string, stage_tip_push?: boolean}} quest
 */
export function maybeShowTip(quest) {
    if (!quest?.stage_tip || !quest.stage_tip_push) return;
    const key = `${quest.id}:${quest.stage}`;
    if (seen().has(key) || document.getElementById('tutorial-tip')) return;
    if (busy()) {
        // Try again shortly (one waiting tip at a time).
        clearTimeout(_pending);
        _pending = setTimeout(() => maybeShowTip(quest), 1500);
        return;
    }
    markSeen(key);
    document.getElementById('quest-tracker-tip')?.classList.remove('fresh');
    renderTip(quest);
}

/**
 * Open a quest's current-stage tip now (the tracker's 💡), seen or not.
 * @param {{id: string, name: string, stage: number, stage_tip?: string}} quest
 */
export function showTip(quest) {
    if (!quest?.stage_tip) return;
    document.getElementById('tutorial-tip')?.remove();
    markSeen(`${quest.id}:${quest.stage}`);
    document.getElementById('quest-tracker-tip')?.classList.remove('fresh');
    renderTip(quest);
}

function renderTip(quest) {
    const host = document.getElementById('game-window') || document.body;
    const el = document.createElement('div');
    el.id = 'tutorial-tip';
    el.innerHTML = `
      <style>
        #tutorial-tip { position: ${host === document.body ? 'fixed' : 'absolute'}; left: 50%; top: 40px; transform: translateX(-50%);
          z-index: 8500; width: 300px; max-width: calc(100% - 16px); box-sizing: border-box;
          background: #1f1f1f; color: #e5e7eb; font-family: 'Dogica', monospace; font-size: 7px; line-height: 1.6;
          border-top: 2px solid #6b7280; border-left: 2px solid #6b7280; border-right: 2px solid #000; border-bottom: 2px solid #000;
          box-shadow: 0 2px 10px rgba(0,0,0,0.7); }
        #tutorial-tip .tt-head { display: flex; gap: 4px; padding: 2px 5px; background: linear-gradient(90deg, #854d0e, #a16207); color: #fff; font-weight: bold; }
        #tutorial-tip .tt-head span:last-child { margin-left: auto; color: #fde68a; font-weight: normal; }
        #tutorial-tip .tt-text { padding: 5px 7px; max-height: 260px; overflow-y: auto; }
        #tutorial-tip .tt-btns { display: flex; justify-content: flex-end; padding: 0 7px 6px; }
        #tutorial-tip button { font: inherit; color: #fff; cursor: pointer; padding: 2px 8px; background: #0e7490;
          border: 2px solid; border-color: #22d3ee #155e75 #155e75 #22d3ee; }
      </style>
      <div class="tt-head"><span>💡 How it works</span><span>${esc(quest.name)}</span></div>
      <div class="tt-text">${esc(quest.stage_tip)}</div>
      <div class="tt-btns"><button type="button">Got it</button></div>`;
    el.querySelector('button').addEventListener('click', () => el.remove());
    host.appendChild(el);
}
