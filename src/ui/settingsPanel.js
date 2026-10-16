/**
 * Settings tab: fills the About section and the save-location note.
 *
 * Refreshed when the game state loads and whenever the tab is opened.
 *
 * @module ui/settingsPanel
 */

import { eventBus } from '../lib/events.js';
import { gameAPI } from '../lib/api.js';
import { getGameStateSync } from '../state/gameState.js';

function setText(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

/** Fill the About section and save note from the current state. */
export function refreshSettingsPanel() {
    const panel = document.getElementById('settings-panel');
    if (!panel) return;

    const c = getGameStateSync()?.character ?? {};
    const who = [c.name, [c.race, c.class].filter(Boolean).join(' ')].filter(Boolean).join(' · ');
    setText('settings-character', who || '—');

    const npub = gameAPI.npub ?? '';
    setText('settings-npub', npub ? `${npub.slice(0, 12)}…${npub.slice(-6)}` : '—');
    const copy = document.getElementById('settings-npub-copy');
    if (copy && !copy.dataset.wired) {
        copy.dataset.wired = '1';
        copy.addEventListener('click', async () => {
            try {
                await navigator.clipboard.writeText(gameAPI.npub ?? '');
                window.showMessage?.('npub copied', 'success');
            } catch (_) {
                window.showMessage?.("Couldn't copy — select it from your profile instead.", 'error');
            }
        });
    }

    setText('settings-save-id', gameAPI.saveID ?? '—');
    setText('settings-version', panel.dataset.version || '—');

    const relay = panel.dataset.relaySaves === 'true';
    setText('settings-save-where', relay
        ? 'Saves are signed by you and stored on your Nostr relays.'
        : 'Saves are kept on this server.');
}

eventBus.on('gameStateLoaded', refreshSettingsPanel);
window.refreshSettingsPanel = refreshSettingsPanel;
