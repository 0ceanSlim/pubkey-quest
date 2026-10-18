/**
 * The "server is about to update" warning: a banner over the game with a
 * countdown and a Save button, shown while a deploy waits to restart the
 * server (cmd/server/maintenance). The notice arrives on the social poll.
 *
 * @module ui/restartNotice
 */

import { eventBus } from '../lib/events.js';

let banner = null;
let timer = null;
let dueAt = 0; // unix seconds
let announced = 0;

function clock(secs) {
    const s = Math.max(0, secs);
    return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

function paint() {
    if (!banner) return;
    const left = Math.round(dueAt - Date.now() / 1000);
    const text = banner.querySelector('[data-text]');
    text.textContent = left > 0
        ? `⚠️ The server updates in ${clock(left)}. Save now so nothing is lost.`
        : '🔄 The server is updating. Reload the page in a moment.';
}

function show(notice) {
    dueAt = notice.at;
    if (!banner) {
        banner = document.createElement('div');
        banner.id = 'restart-notice';
        banner.style.cssText =
            'position:fixed; top:6px; left:50%; transform:translateX(-50%); z-index:2500; ' +
            'display:flex; align-items:center; gap:8px; padding:5px 8px; max-width:92vw; ' +
            'background:#7c2d12; color:#fff; font-size:10px; font-weight:bold; ' +
            'border-top:2px solid #fb923c; border-left:2px solid #fb923c; border-right:2px solid #431407; border-bottom:2px solid #431407; ' +
            'box-shadow:0 0 12px rgba(0,0,0,0.7);';
        banner.innerHTML =
            '<span data-text></span>' +
            '<button data-save style="font-size:10px; font-weight:bold; color:#fff; cursor:pointer; padding:2px 8px; background:#166534; ' +
            'border-top:2px solid #22c55e; border-left:2px solid #22c55e; border-right:2px solid #14532d; border-bottom:2px solid #14532d;">💾 Save now</button>';
        banner.querySelector('[data-save]').addEventListener('click', () => window.openSaveModal?.());
        document.body.appendChild(banner);
        timer = setInterval(paint, 1000);
    }
    paint();
    // Say it once in the log too, per notice.
    if (announced !== notice.at) {
        announced = notice.at;
        window.showMessage?.(`⚠️ Server update${notice.version ? ` (${notice.version})` : ''} coming — save your game.`, 'warning');
    }
}

function hide() {
    clearInterval(timer);
    timer = null;
    banner?.remove();
    banner = null;
}

eventBus.on('server:restart', (notice) => (notice ? show(notice) : hide()));
