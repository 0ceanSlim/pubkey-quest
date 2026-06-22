/**
 * Save Ritual
 *
 * Saving is deliberate (Pokemon/Fallout-style, roadmap §4) — a win95 modal
 * confirms the save instead of a silent overwrite, and a "last saved: N min ago"
 * indicator keeps the player oriented without nagging. Triggered from the
 * Settings save button and Ctrl/Cmd+S. The actual write goes through
 * saveSystem.saveGameToLocal (memory → disk via /api/session/save).
 *
 * @module systems/saveRitual
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { saveGameToLocal } from './saveSystem.js';
import { getGameStateSync } from '../state/gameState.js';

const LS_PREFIX = 'pq_last_saved';

function lastSavedKey() {
    return `${LS_PREFIX}:${gameAPI.saveID || 'unknown'}`;
}

function getLastSaved() {
    const v = localStorage.getItem(lastSavedKey());
    const n = v ? parseInt(v, 10) : 0;
    return Number.isFinite(n) ? n : 0;
}

function setLastSaved(ts) {
    try { localStorage.setItem(lastSavedKey(), String(ts)); } catch { /* ignore */ }
}

function lastSavedText() {
    const ts = getLastSaved();
    if (!ts) return 'Last saved: never this session';
    const mins = Math.floor((Date.now() - ts) / 60000);
    if (mins <= 0) return 'Last saved: just now';
    if (mins === 1) return 'Last saved: 1 minute ago';
    if (mins < 60) return `Last saved: ${mins} minutes ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `Last saved: ${hrs} hour${hrs === 1 ? '' : 's'} ago`;
    const days = Math.floor(hrs / 24);
    return `Last saved: ${days} day${days === 1 ? '' : 's'} ago`;
}

function summaryText() {
    const name = document.getElementById('char-name')?.textContent?.trim() || 'Adventurer';
    const level = document.getElementById('char-level')?.textContent?.trim() || '?';
    let day = '';
    try {
        const s = getGameStateSync();
        const d = s?.current_day ?? s?.character?.current_day;
        if (d != null) day = ` · Day ${d}`;
    } catch { /* state not ready */ }
    return `${name} — Level ${level}${day}`;
}

function setText(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

/** Refresh every "last saved" indicator on the page (elements with data-last-saved). */
export function refreshLastSavedIndicators() {
    const text = lastSavedText();
    document.querySelectorAll('[data-last-saved]').forEach((el) => { el.textContent = text; });
}

/** Open the save ritual modal. */
export function openSaveModal() {
    const modal = document.getElementById('save-modal');
    if (!modal) return;
    setText('save-modal-summary', summaryText());
    setText('save-modal-last', lastSavedText());
    const status = document.getElementById('save-modal-status');
    if (status) { status.textContent = ''; status.classList.add('hidden'); }
    const btn = document.getElementById('save-modal-confirm');
    if (btn) { btn.disabled = false; btn.textContent = 'SAVE'; }
    modal.classList.remove('hidden');
}

/** Close the save modal. */
export function closeSaveModal() {
    document.getElementById('save-modal')?.classList.add('hidden');
}

/** Perform the deliberate save, then update the indicators. */
export async function confirmSaveGame() {
    const btn = document.getElementById('save-modal-confirm');
    const status = document.getElementById('save-modal-status');
    if (btn) { btn.disabled = true; btn.textContent = 'Saving…'; }

    let ok = false;
    try {
        ok = await saveGameToLocal(window.showMessage);
    } catch (error) {
        logger.error('Save ritual failed:', error);
        ok = false;
    }

    if (ok) {
        setLastSaved(Date.now());
        refreshLastSavedIndicators();
        setText('save-modal-last', lastSavedText());
        if (status) {
            status.textContent = '✅ Game saved';
            status.className = 'mb-2 font-bold text-green-400';
            status.classList.remove('hidden');
        }
        if (btn) { btn.disabled = false; btn.textContent = 'SAVE'; }
        setTimeout(closeSaveModal, 900);
    } else {
        if (status) {
            status.textContent = '❌ Save failed — try again';
            status.className = 'mb-2 font-bold text-red-400';
            status.classList.remove('hidden');
        }
        if (btn) { btn.disabled = false; btn.textContent = 'SAVE'; }
    }
}

// Ctrl/Cmd+S opens the save ritual (and stops the browser's "save page" dialog).
function onKey(e) {
    if ((e.ctrlKey || e.metaKey) && (e.key === 's' || e.key === 'S')) {
        e.preventDefault();
        openSaveModal();
    }
}

if (typeof window !== 'undefined') {
    window.openSaveModal = openSaveModal;
    window.closeSaveModal = closeSaveModal;
    window.confirmSaveGame = confirmSaveGame;
    document.addEventListener('keydown', onKey);
    // Keep "last saved: N min ago" current.
    refreshLastSavedIndicators();
    setInterval(refreshLastSavedIndicators, 30000);
}
