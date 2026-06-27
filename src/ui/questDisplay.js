/**
 * Quest Display
 *
 * Renders the quest-log tab from the M3 quest API (/api/quests/log) and wires
 * the accept / abandon actions. Loaded when the questlog tab is opened (see the
 * switchTab hook in game.html) and re-rendered after accept/abandon; objective
 * progress refreshes whenever the log is reopened.
 *
 * @module ui/questDisplay
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { API_BASE_URL } from '../config/constants.js';

const $ = (id) => document.getElementById(id);

const esc = (s) =>
    String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

/** Fetch the quest log and render every section. */
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
        render(json.data);
    } catch (err) {
        logger.error('loadQuestLog error:', err);
    }
}

function render(data) {
    const qp = $('quest-points-total');
    if (qp) qp.textContent = data.quest_points ?? 0;
    renderAvailable(data.available || []);
    renderActive(data.active || []);
    renderCompleted(data.completed || []);
}

function empty(text, color = 'gray') {
    const border = color === 'green' ? 'border-green-700 text-green-500' : 'border-gray-600 text-gray-500';
    return `<div class="bg-gray-800 p-2 rounded border-l-2 ${border} text-[10px]">${esc(text)}</div>`;
}

function renderAvailable(list) {
    const el = $('available-quests');
    if (!el) return;
    if (!list.length) {
        el.innerHTML = empty('No quests available right now.');
        return;
    }
    el.innerHTML = list
        .map(
            (q) => `
        <div class="bg-gray-700 p-2 rounded border-l-2 border-blue-500">
            <div class="flex items-center justify-between">
                <span class="text-blue-300 font-bold">${esc(q.name)}</span>
                ${q.category ? `<span class="text-[9px] uppercase text-gray-400">${esc(q.category)}</span>` : ''}
            </div>
            ${q.description ? `<div class="text-gray-300 text-[10px] mt-1">${esc(q.description)}</div>` : ''}
            ${q.start_hint ? `<div class="text-gray-500 text-[10px] mt-1 italic">» ${esc(q.start_hint)}</div>` : ''}
            <button data-quest="${esc(q.id)}"
                class="quest-accept-btn mt-2 px-2 py-0.5 text-[10px] font-bold text-black bg-yellow-500 hover:bg-yellow-400 rounded">
                Accept
            </button>
        </div>`
        )
        .join('');
    el.querySelectorAll('.quest-accept-btn').forEach((b) =>
        b.addEventListener('click', () => postQuest('accept', b.dataset.quest))
    );
}

function renderActive(list) {
    const el = $('active-quests');
    if (!el) return;
    if (!list.length) {
        el.innerHTML = empty('No active quests — your adventure awaits…');
        return;
    }
    el.innerHTML = list
        .map((q) => {
            const stage =
                q.stage_count > 1
                    ? `<span class="text-[9px] text-gray-400">Stage ${(q.stage ?? 0) + 1}/${q.stage_count}</span>`
                    : '';
            const objectives = (q.objectives || [])
                .map(
                    (o) => `
                <div class="flex items-center gap-1 ${o.done ? 'text-green-400' : 'text-gray-300'}">
                    <span>${o.done ? '☑' : '☐'}</span>
                    <span class="flex-1">${esc(o.description || '')}</span>
                    <span class="text-[9px] text-gray-400">${o.count}/${o.target}</span>
                </div>`
                )
                .join('');
            return `
        <div class="bg-gray-700 p-2 rounded border-l-2 border-yellow-600">
            <div class="flex items-center justify-between">
                <span class="text-yellow-400 font-bold">${esc(q.name)}</span>${stage}
            </div>
            ${q.description ? `<div class="text-gray-400 text-[10px] mt-1 mb-1">${esc(q.description)}</div>` : ''}
            <div class="space-y-0.5 text-[10px]">${objectives}</div>
            <button data-quest="${esc(q.id)}"
                class="quest-abandon-btn mt-2 px-2 py-0.5 text-[10px] text-gray-300 bg-gray-800 hover:bg-red-900 rounded border border-gray-600">
                Abandon
            </button>
        </div>`;
        })
        .join('');
    el.querySelectorAll('.quest-abandon-btn').forEach((b) =>
        b.addEventListener('click', () => postQuest('abandon', b.dataset.quest))
    );
}

function renderCompleted(list) {
    const el = $('completed-quests');
    if (!el) return;
    if (!list.length) {
        el.innerHTML = empty('No quests completed yet.', 'green');
        return;
    }
    el.innerHTML = list
        .map(
            (q) => `
        <div class="bg-gray-800 p-2 rounded border-l-2 border-green-600 opacity-70">
            <span class="text-green-400">☑</span>
            <span class="text-green-300 font-bold">${esc(q.name)}</span>
        </div>`
        )
        .join('');
}

/** POST accept/abandon, then re-render from the returned log. */
async function postQuest(action, questID) {
    const { npub, saveID } = gameAPI;
    if (!npub || !saveID || !questID) return;
    try {
        const resp = await fetch(`${API_BASE_URL}/quests/${action}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ npub, save_id: saveID, quest_id: questID }),
        });
        const json = await resp.json();
        if (!resp.ok || !json.success) {
            window.showMessage?.(json.error ?? `Quest ${action} failed`, 'error');
            return;
        }
        if (json.message) window.showMessage?.(json.message, 'success');
        if (json.data) render(json.data);
        else loadQuestLog();
    } catch (err) {
        logger.error(`postQuest ${action} error:`, err);
        window.showMessage?.(`Quest ${action} failed`, 'error');
    }
}

export { loadQuestLog };
