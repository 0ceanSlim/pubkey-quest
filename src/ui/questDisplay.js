/**
 * Quest Display — the journal tab (Quests | Badges) and the quest guide popup.
 *
 * Reads /api/quests/log, where every quest carries a status — active,
 * available, locked (a requirement unmet) or completed — and renders one list
 * that can be sorted (category / status / name / difficulty) and filtered (hide
 * unavailable, hide completed). Rows are colour-coded by status and carry the
 * next step, danger and rewards; clicking one opens the guide popup with the
 * objectives, how to start, requirements (met / unmet), recommendations and
 * rewards, plus Track for quests in progress.
 *
 * Quests are NOT accepted here — they start in the world (talk to the giver).
 * Loaded when the questlog tab opens (see the switchTab hook in game.html).
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
const DIFFICULTY_ORDER = ['novice', 'intermediate', 'moderate', 'experienced', 'master'];
const STATUS_ORDER = { active: 0, available: 1, locked: 2, completed: 3 };
const STATUS_LABEL = { active: 'In progress', available: 'Not started', locked: 'Unavailable', completed: 'Completed' };
const STATUS_COLOR = { active: '#facc15', available: '#93c5fd', locked: '#6b7280', completed: '#4ade80' };

// View preferences survive reloads (a per-browser convenience, not game state).
const PREFS_KEY = 'pq_quest_journal_prefs';
const prefs = { sort: 'category', hideLocked: false, hideDone: false, collapsed: [] };
try { Object.assign(prefs, JSON.parse(localStorage.getItem(PREFS_KEY) || '{}')); } catch (_) { /* private mode */ }
function savePrefs() {
    try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (_) { /* private mode */ }
}

let _quests = [];       // every quest in the last log, flattened, with .status
let _byId = {};

// ── tracked quest (which one the over-scene tracker follows) ────────────────
// A UI preference per save, so it lives in the browser — never in the save file.
const trackedKey = () => `pq_tracked_quest:${gameAPI.saveID}`;
export function getTrackedQuestId() {
    try { return localStorage.getItem(trackedKey()) || null; } catch (_) { return null; }
}
function setTrackedQuestId(id) {
    try {
        if (id) localStorage.setItem(trackedKey(), id);
        else localStorage.removeItem(trackedKey());
    } catch (_) { /* private mode */ }
    window.updateQuestTracker?.();
}

// ── loading ──────────────────────────────────────────────────────────────────

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
        ingest(json.data);
        renderJournal();
    } catch (err) {
        logger.error('loadQuestLog error:', err);
    }
}

function ingest(data) {
    const qp = $('quest-points-total');
    if (qp) qp.textContent = data.quest_points ?? 0;
    _quests = [
        ...(data.active || []),
        ...(data.available || []),
        ...(data.locked || []),
        ...(data.completed || []),
    ];
    _byId = Object.fromEntries(_quests.map((q) => [q.id, q]));
}

// ── the list ─────────────────────────────────────────────────────────────────

function renderJournal() {
    const journal = $('quest-journal');
    if (!journal) return;
    syncTools();

    const list = _quests.filter((q) =>
        !(prefs.hideLocked && q.status === 'locked') && !(prefs.hideDone && q.status === 'completed'));
    const byName = (a, b) => a.name.localeCompare(b.name);
    const byStatus = (a, b) => STATUS_ORDER[a.status] - STATUS_ORDER[b.status] || byName(a, b);

    let html = '';
    if (prefs.sort === 'category') {
        const cats = [...CATEGORY_ORDER, ...new Set(list.map((q) => q.category).filter((c) => !CATEGORY_ORDER.includes(c)))];
        for (const cat of cats) {
            const items = list.filter((q) => (q.category || 'side') === cat).sort(byStatus);
            if (!items.length) continue;
            const open = !prefs.collapsed.includes(cat);
            html += `<div class="qj-group" data-cat="${esc(cat)}"><span>${open ? '▾' : '▸'} ${esc(cap(cat))}</span><span>${items.length}</span></div>`;
            if (open) html += items.map(row).join('');
        }
    } else {
        const cmp = {
            status: byStatus,
            name: byName,
            difficulty: (a, b) => difficultyRank(a) - difficultyRank(b) || byName(a, b),
        }[prefs.sort] || byStatus;
        html = [...list].sort(cmp).map(row).join('');
    }

    if (!html) {
        html = _quests.length
            ? '<div class="qj-empty">Nothing to show — try clearing a filter.</div>'
            : '<div class="qj-empty">No quests yet — explore and talk to people.</div>';
    }
    journal.innerHTML = html;
}

function difficultyRank(q) {
    const i = DIFFICULTY_ORDER.indexOf(String(q.difficulty || '').toLowerCase());
    return i < 0 ? DIFFICULTY_ORDER.length : i;
}

// Short forms for the narrow rail; the popup spells them out.
const ABBR = { novice: 'Nov', intermediate: 'Int', moderate: 'Mod', experienced: 'Exp', master: 'Mas', low: 'Low', high: 'High' };
const abbr = (s) => ABBR[String(s || '').toLowerCase()] || s || '';

function row(q) {
    const tracked = q.status === 'active' && q.id === getTrackedQuestId() ? '<span class="qj-tracked" title="Tracked">📌</span>' : '';
    // Bottom line: stage (active) · difficulty · danger — e.g. "3/10 · Int · ⚔️Mod".
    const meta = [
        q.status === 'active' && q.stage_count > 1 ? `${(q.stage ?? 0) + 1}/${q.stage_count}` : '',
        abbr(q.difficulty),
        q.recommended?.danger ? `⚔️${abbr(q.recommended.danger)}` : '',
    ].filter(Boolean).join(' · ');
    const prog = q.status === 'active' && q.stage_count > 0
        ? `<div class="qj-prog"><div style="width:${Math.round((100 * (q.stage ?? 0)) / q.stage_count)}%"></div></div>`
        : '';
    return `
    <div class="qj-row qj-${q.status}" data-quest="${esc(q.id)}">
        <div class="qj-bar"></div>
        <div class="qj-body">
            <div class="qj-l1">${tracked}<span class="qj-name" title="${esc(q.name)}">${esc(q.name)}</span></div>
            <div class="qj-l2" title="${esc(detailLine(q))}">${esc(detailLine(q))}</div>
            <div class="qj-l3"><span>${esc(meta)}</span><span class="qj-reward">${esc(rewardShort(q.rewards))}</span></div>
            ${prog}
        </div>
    </div>`;
}

/** The one-line context under a quest's name, by status. */
function detailLine(q) {
    switch (q.status) {
        case 'active': {
            const objs = q.objectives || [];
            const next = objs.find((o) => !o.done) || objs[objs.length - 1];
            if (!next) return q.stage_description || 'In progress';
            return `Next: ${next.description}${next.target > 1 ? ` ${next.count}/${next.target}` : ''}`;
        }
        case 'available':
            return q.start_hint || 'Seek it out in the world.';
        case 'locked': {
            const unmet = (q.requirements || []).find((r) => !r.met);
            return `🔒 ${unmet ? unmet.description : 'Requirements not met'}`;
        }
        default:
            return 'Completed';
    }
}

// XP and gold only — the row has ~10 characters for it; QP and items are in the popup.
function rewardShort(r) {
    if (!r) return '';
    const parts = [];
    if (r.xp) parts.push(`${r.xp}xp`);
    if (r.gold) parts.push(`${r.gold}g`);
    return parts.join(' ');
}

function itemName(id) {
    return window.getItemById?.(id)?.name || cap(String(id).replace(/[-_]/g, ' '));
}

// ── tools (sort / filters / sub-tabs) ────────────────────────────────────────

function syncTools() {
    const sort = $('qj-sort');
    if (sort) sort.value = prefs.sort;
    const hl = $('qj-hide-locked');
    if (hl) hl.checked = !!prefs.hideLocked;
    const hd = $('qj-hide-done');
    if (hd) hd.checked = !!prefs.hideDone;
}

// The journal markup is server-rendered once, so wire it with delegation.
function wireJournalPanel() {
    const panel = $('questlog-panel');
    if (!panel || panel.dataset.wired) return;
    panel.dataset.wired = '1';

    panel.addEventListener('change', (e) => {
        if (e.target.id === 'qj-sort') prefs.sort = e.target.value;
        else if (e.target.id === 'qj-hide-locked') prefs.hideLocked = e.target.checked;
        else if (e.target.id === 'qj-hide-done') prefs.hideDone = e.target.checked;
        else return;
        savePrefs();
        renderJournal();
    });

    panel.addEventListener('click', (e) => {
        const sub = e.target.closest('.qj-subtab');
        if (sub) {
            const badges = sub.dataset.qjSub === 'badges';
            panel.querySelectorAll('.qj-subtab').forEach((b) => b.classList.toggle('on', b === sub));
            $('qj-quests-view')?.classList.toggle('hidden', badges);
            $('qj-badges-view')?.classList.toggle('hidden', !badges);
            return;
        }
        const group = e.target.closest('.qj-group');
        if (group) {
            const cat = group.dataset.cat;
            prefs.collapsed = prefs.collapsed.includes(cat)
                ? prefs.collapsed.filter((c) => c !== cat)
                : [...prefs.collapsed, cat];
            savePrefs();
            renderJournal();
            return;
        }
        const r = e.target.closest('.qj-row');
        if (r) openQuestModal(r.dataset.quest);
    });
}

// ── the guide popup ──────────────────────────────────────────────────────────

// The modal markup lives in the scene (game/quest-modal.html) so it draws over
// the scene and scales with it — same pattern as the level-up modal.
function openQuestModal(questId) {
    const q = _byId[questId];
    const content = $('quest-modal-content');
    const modal = $('quest-modal');
    if (!q || !content || !modal) return;
    content.innerHTML = modalBody(q);
    wireModalActions(content, q);
    modal.classList.remove('hidden');
}

function closeQuestModal() {
    $('quest-modal')?.classList.add('hidden');
}

function modalBody(q) {
    const col = STATUS_COLOR[q.status] || '#9ca3af';
    const meta = [cap(q.category || ''), q.difficulty, q.recommended?.danger ? `⚔️ ${q.recommended.danger} danger` : '']
        .filter(Boolean).map(esc).join(' · ');
    let h = `<div class="qm-name">${esc(q.name)}</div>
        <div class="qm-meta"><span class="qm-pill" style="color:${col};border-color:${col}">${STATUS_LABEL[q.status] || ''}</span>${meta}</div>`;
    if (q.description) h += `<div class="qm-desc">${esc(q.description)}</div>`;

    if (q.status === 'active') {
        if (q.stage_count > 1 || q.stage_description) {
            h += `<div class="qm-sec">Stage ${(q.stage ?? 0) + 1} of ${q.stage_count}</div>`;
            if (q.stage_description) h += `<div class="qm-desc">${esc(q.stage_description)}</div>`;
        }
        if ((q.objectives || []).length) {
            h += '<div class="qm-sec">Objectives</div>';
            h += q.objectives.map((o) =>
                `<div class="qm-obj ${o.done ? 'qm-ok' : ''}"><span>${o.done ? '☑' : '☐'} ${esc(o.description)}</span><span>${o.count}/${o.target}</span></div>`,
            ).join('');
        }
    }

    if (q.status === 'available' || q.status === 'locked') {
        h += `<div class="qm-sec">How to start</div><div class="qm-hint">${esc(q.start_hint || 'Seek it out in the world.')}</div>`;
    }

    if ((q.requirements || []).length && q.status !== 'completed') {
        h += '<div class="qm-sec">Requirements</div>';
        h += q.requirements.map((r) =>
            `<div class="${r.met ? 'qm-ok' : 'qm-bad'}">${r.met ? '✓' : '✗'} ${esc(r.description)}</div>`).join('');
    }

    if (q.recommended?.stats?.length) {
        h += `<div class="qm-sec">Recommended</div><div class="qm-desc">${q.recommended.stats.map(esc).join(' · ')}</div>`;
    }

    const r = q.rewards;
    if (r) {
        const parts = [];
        if (r.xp) parts.push(`${r.xp} XP`);
        if (r.gold) parts.push(`${r.gold} gold`);
        if (r.quest_points) parts.push(`${r.quest_points} QP`);
        h += '<div class="qm-sec">Rewards</div>';
        if (parts.length) h += `<div class="qm-reward">${parts.join(' · ')}</div>`;
        if ((r.items || []).length) {
            h += `<div class="qm-items">${r.items.map((it) =>
                `<div>${esc(itemName(it.id))}${it.quantity > 1 ? ` ×${it.quantity}` : ''}</div>`).join('')}</div>`;
        }
    }

    if (q.status === 'active') {
        const tracked = q.id === getTrackedQuestId();
        h += `<div class="qm-actions">
            <button class="qm-btn" data-qm="track">${tracked ? '📌 Untrack' : '📌 Track'}</button>
        </div>`;
    }
    return h;
}

function wireModalActions(content, q) {
    content.querySelector('[data-qm="track"]')?.addEventListener('click', () => {
        setTrackedQuestId(q.id === getTrackedQuestId() ? null : q.id);
        renderJournal();
        openQuestModal(q.id);
    });
}

/** Open the journal tab on one quest's popup — the over-scene tracker's click. */
export async function openQuestInJournal(questId) {
    window.switchTab?.('questlog');
    await loadQuestLog();
    if (questId) openQuestModal(questId);
}

if (typeof window !== 'undefined') {
    window.closeQuestModal = closeQuestModal;
    window.openQuestInJournal = openQuestInJournal;
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', wireJournalPanel);
    else wireJournalPanel();
}

export { loadQuestLog };
