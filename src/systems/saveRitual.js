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
import { saveGameToLocal, lastSaveError } from './saveSystem.js';
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

/**
 * Open the save ritual modal.
 * @param {{firstSave?: boolean}} [opts] firstSave: a new character that has
 *   never been signed to relays (shown once after the intro).
 */
export function openSaveModal(opts = {}) {
    const modal = document.getElementById('save-modal');
    if (!modal) return;
    document.getElementById('save-modal-first')?.classList.toggle('hidden', !opts.firstSave);
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
            status.textContent = '❌ ' + (lastSaveError || 'Save failed — try again');
            status.className = 'mb-2 font-bold text-red-400';
            status.classList.remove('hidden');
        }
        if (btn) { btn.disabled = false; btn.textContent = 'SAVE'; }
    }
}

// ─── Quit to the title screen ────────────────────────────────────────────────

const QUIT_BTN = 'padding: 6px 10px; font-weight: bold; color: #fff; font-size: 11px; cursor: pointer; ' +
    'border-top: 2px solid #4a4a4a; border-left: 2px solid #4a4a4a; border-right: 2px solid #1a1a1a; border-bottom: 2px solid #1a1a1a;';

/** Ask whether to save, then leave the game for the home page. */
export function openQuitModal() {
    document.getElementById('quit-modal')?.remove();
    const overlay = document.createElement('div');
    overlay.id = 'quit-modal';
    overlay.style.cssText = 'position: fixed; inset: 0; background: rgba(0,0,0,0.8); z-index: 300; display: flex; align-items: center; justify-content: center;';
    overlay.innerHTML = `
        <div class="bg-win95-bgDark" style="border: 2px solid #808080; width: 340px; max-width: 92vw; box-shadow: 0 0 16px rgba(0,0,0,0.6);">
            <div class="px-3 py-1 text-white font-bold" style="font-size: 12px; background: #7f1d1d;">🚪 LEAVE THE GAME</div>
            <div class="p-4 text-center">
                <div class="text-white font-bold mb-2" style="font-size: 13px;">Save before you go?</div>
                <div class="text-gray-300 mb-3" style="font-size: 10px; line-height: 1.6;">${lastSavedText()}. Quitting without saving goes back to that save: everything since is discarded.</div>
                <div id="quit-modal-status" class="hidden mb-2 font-bold text-red-400" style="font-size: 10px;"></div>
                <div class="flex flex-col gap-1">
                    <button data-quit="save" style="${QUIT_BTN} background: #166534;">💾 Save &amp; quit</button>
                    <button data-quit="nosave" style="${QUIT_BTN} background: #7f1d1d;">Quit without saving</button>
                    <button data-quit="stay" style="${QUIT_BTN} background: #2a2a2a;">Keep playing</button>
                </div>
            </div>
        </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    overlay.querySelector('[data-quit="stay"]').onclick = () => overlay.remove();
    overlay.querySelector('[data-quit="nosave"]').onclick = () => quitToTitle();
    overlay.querySelector('[data-quit="save"]').onclick = async (e) => {
        const btn = e.currentTarget;
        const status = overlay.querySelector('#quit-modal-status');
        btn.disabled = true;
        btn.textContent = 'Saving…';
        let ok = false;
        try {
            ok = await saveGameToLocal(window.showMessage);
        } catch (error) {
            logger.error('Save before quitting failed:', error);
        }
        if (ok) {
            setLastSaved(Date.now());
            quitToTitle();
            return;
        }
        status.textContent = '❌ ' + (lastSaveError || 'Save failed. Quit without saving, or keep playing.');
        status.classList.remove('hidden');
        btn.disabled = false;
        btn.textContent = '💾 Save & quit';
    };
}

/** Let the server drop the session, then go home. */
async function quitToTitle() {
    try {
        if (gameAPI.npub && gameAPI.saveID) {
            const q = new URLSearchParams({ npub: gameAPI.npub, save_id: gameAPI.saveID });
            await fetch(`/api/session/cleanup?${q}`, { method: 'DELETE' });
        }
    } catch (error) {
        logger.warn('Session cleanup on quit failed:', error);
    }
    window.location.href = '/';
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
    window.openQuitModal = openQuitModal;
    document.addEventListener('keydown', onKey);
    // Keep "last saved: N min ago" current.
    refreshLastSavedIndicators();
    setInterval(refreshLastSavedIndicators, 30000);
}
