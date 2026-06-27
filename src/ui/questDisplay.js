/**
 * Quest Display — the quest journal tab.
 *
 * Reads /api/quests/log and renders quests grouped into collapsible category
 * sections (Main, Side, Class, Race, Daily, Weekly, then Completed). Each quest
 * is a clickable row that opens a detail modal — the "quest guide": how/where to
 * start it (available), or the current objective (in progress). Quests are NOT
 * accepted here — they start in the world (talk to the giver). Loaded when the
 * questlog tab opens (see the switchTab hook in game.html).
 *
 * @module ui/questDisplay
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';

const $ = (id) => document.getElementById(id);
const esc = (s) =>
    String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const cap = (s) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : '');

const CATEGORY_ORDER = ['main', 'side', 'class', 'race', 'daily', 'weekly'];
const CATEGORY_LABEL = { main: 'Main', side: 'Side', class: 'Class', race: 'Race', daily: 'Daily', weekly: 'Weekly' };

// Quests indexed by id for the modal, and which category sections are collapsed.
let _questsById = {};
const _collapsed = new Set();

/** Fetch the quest log and render the journal. */
async function loadQuestLog() {
    const { npub, saveID } = gameAPI;
    if (!npub || !saveID) return;
    try {
        const resp = await fetch(`${API_BASE_URL}/quests/log?npub=${npub}&save_id=${saveID}`);
        const json = await resp.json();
        if (!resp.ok || !json.success) {
            logger.warn('Quest log load failed:', json.error ?? resp.status);
            return;
        }
        renderJournal(json.data);
    } catch (err) {
        logger.error('loadQuestLog error:', err);
    }
}

function renderJournal(data) {
    const qpEl = $('quest-points-total');
    if (qpEl) qpEl.textContent = data.quest_points ?? 0;

    // Merge active + available (with a status) and index every quest for the modal.
    _questsById = {};
    const items = [];
    (data.active || []).forEach((q) => {
        const e = { ...q, status: 'active' };
        items.push(e);
        _questsById[q.id] = e;
    });
    (data.available || []).forEach((q) => {
        const e = { ...q, status: 'available' };
        items.push(e);
        _questsById[q.id] = e;
    });
    (data.completed || []).forEach((q) => {
        _questsById[q.id] = { ...q, status: 'completed' };
    });

    const byCat = {};
    items.forEach((q) => {
        const c = q.category || 'side';
        (byCat[c] = byCat[c] || []).push(q);
    });

    const journal = $('quest-journal');
    if (!journal) return;

    let html = '';
    const seen = new Set();
    CATEGORY_ORDER.forEach((cat) => {
        if (byCat[cat]?.length) {
            html += section(CATEGORY_LABEL[cat], cat, byCat[cat]);
            seen.add(cat);
        }
    });
    Object.keys(byCat).forEach((cat) => {
        if (!seen.has(cat)) html += section(cap(cat), cat, byCat[cat]);
    });
    if ((data.completed || []).length) {
        const done = data.completed.map((q) => ({ ...q, status: 'completed' }));
        html += section('Completed', 'completed', done);
    }

    journal.innerHTML = html || `<div class="text-gray-500 text-[9px] p-1">No quests yet — explore and talk to people.</div>`;
    wireJournal(journal);
}

function section(label, key, list) {
    const collapsed = _collapsed.has(key);
    const arrow = collapsed ? '▸' : '▾';
    const rows = collapsed ? '' : list.map(questRow).join('');
    return `
    <div class="quest-cat" data-cat="${esc(key)}">
        <button class="quest-cat-header w-full flex items-center justify-between text-left text-yellow-300 font-bold text-[9px] py-0.5 px-1 bg-gray-700 hover:bg-gray-600">
            <span>${arrow} ${esc(label)}</span>
            <span class="text-gray-400">${list.length}</span>
        </button>
        <div class="quest-cat-body pl-1 pt-0.5 space-y-0.5">${rows}</div>
    </div>`;
}

function questRow(q) {
    const badge =
        q.status === 'active'
            ? '<span class="text-yellow-400">◆</span>'
            : q.status === 'completed'
            ? '<span class="text-green-500">☑</span>'
            : '<span class="text-blue-300">○</span>';
    const nameColor = q.status === 'completed' ? 'text-gray-400' : 'text-gray-200';
    return `
    <button class="quest-row w-full text-left flex items-center gap-1 px-1 py-0.5 hover:bg-gray-700 text-[9px]" data-quest="${esc(q.id)}">
        ${badge}<span class="flex-1 truncate ${nameColor}">${esc(q.name)}</span>
    </button>`;
}

function wireJournal(journal) {
    journal.querySelectorAll('.quest-cat-header').forEach((h) => {
        h.addEventListener('click', () => {
            const key = h.closest('.quest-cat')?.dataset.cat;
            if (!key) return;
            if (_collapsed.has(key)) _collapsed.delete(key);
            else _collapsed.add(key);
            loadQuestLog(); // simplest: re-render from fresh data
        });
    });
    journal.querySelectorAll('.quest-row').forEach((r) => {
        r.addEventListener('click', () => openQuestModal(r.dataset.quest));
    });
}

// ── detail modal (the quest guide) ──────────────────────────────────────────

function ensureModal() {
    let overlay = $('quest-modal');
    if (overlay) return overlay;
    overlay = document.createElement('div');
    overlay.id = 'quest-modal';
    overlay.className = 'hidden';
    overlay.style.cssText =
        'position:fixed;inset:0;z-index:60;display:flex;align-items:center;justify-content:center;background:rgba(0,0,0,0.65);';
    overlay.innerHTML = `
        <div style="width:340px;max-width:90vw;max-height:80vh;overflow:auto;background:#1f2430;border:2px solid #b8860b;box-shadow:0 0 0 1px #000;padding:14px;">
            <div id="quest-modal-content"></div>
            <button id="quest-modal-close" style="margin-top:12px;width:100%;background:#3a3f4b;color:#fff;font-weight:bold;font-size:11px;padding:4px;border-top:1px solid #5a5f6b;border-left:1px solid #5a5f6b;border-right:1px solid #000;border-bottom:1px solid #000;cursor:pointer;">Close</button>
        </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', (e) => {
        if (e.target === overlay) closeQuestModal();
    });
    overlay.querySelector('#quest-modal-close').addEventListener('click', closeQuestModal);
    return overlay;
}

function openQuestModal(questId) {
    const q = _questsById[questId];
    if (!q) return;
    const overlay = ensureModal();
    overlay.querySelector('#quest-modal-content').innerHTML = modalBody(q);
    overlay.classList.remove('hidden');
}

function closeQuestModal() {
    $('quest-modal')?.classList.add('hidden');
}

function modalBody(q) {
    const meta = [cap(q.category), q.difficulty ? cap(q.difficulty) : '']
        .filter(Boolean)
        .join(' · ');
    let html = `<div style="color:#facc15;font-weight:bold;font-size:14px;">${esc(q.name)}</div>`;
    if (meta) html += `<div style="color:#9ca3af;font-size:10px;margin-bottom:8px;">${esc(meta)}</div>`;

    if (q.status === 'completed') {
        html += `<div style="color:#4ade80;font-size:12px;">☑ Completed</div>`;
        return html;
    }

    if (q.status === 'active') {
        const stage = q.stage_count > 1 ? ` (stage ${(q.stage ?? 0) + 1}/${q.stage_count})` : '';
        html += `<div style="color:#9ca3af;font-size:10px;text-transform:uppercase;margin-bottom:2px;">What to do next${stage}</div>`;
        if (q.description) html += `<div style="color:#e5e7eb;font-size:11px;margin-bottom:6px;">${esc(q.description)}</div>`;
        (q.objectives || []).forEach((o) => {
            const col = o.done ? '#4ade80' : '#d1d5db';
            html += `<div style="color:${col};font-size:11px;">${o.done ? '☑' : '☐'} ${esc(o.description)} <span style="color:#6b7280;">${o.count}/${o.target}</span></div>`;
        });
    } else {
        // available
        if (q.description) html += `<div style="color:#d1d5db;font-size:11px;margin-bottom:8px;">${esc(q.description)}</div>`;
        html += `<div style="color:#9ca3af;font-size:10px;text-transform:uppercase;margin-bottom:2px;">How to start</div>`;
        html += `<div style="color:#93c5fd;font-size:11px;">${esc(q.start_hint || 'Seek it out in the world.')}</div>`;
    }

    const r = q.rewards;
    if (r && (r.xp || r.gold || (r.items || []).length)) {
        const parts = [];
        if (r.xp) parts.push(`${r.xp} XP`);
        if (r.gold) parts.push(`${r.gold} gold`);
        (r.items || []).forEach((it) => parts.push(`${esc(it.id)}${it.quantity > 1 ? ' ×' + it.quantity : ''}`));
        html += `<div style="color:#9ca3af;font-size:10px;text-transform:uppercase;margin:10px 0 2px;">Rewards</div>`;
        html += `<div style="color:#fcd34d;font-size:11px;">${parts.join(' · ')}</div>`;
    }
    return html;
}

export { loadQuestLog };
