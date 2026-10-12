/**
 * Portrait dialog — "Make a portrait" (party-combat-plan §2c).
 *
 * A character's portrait is a picture the player chose for the game, separate
 * from their Nostr picture. This dialog:
 *   1. writes an image prompt from the save (the server builds it) that the
 *      player pastes into any AI image tool, with an optional "your look" line;
 *   2. sets the result — a pasted https link, or an upload to the player's own
 *      Blossom server (kind 10063), signed in the browser with their signer;
 *   3. previews it at token size, and can remove it.
 * The server checks and scales every portrait; the page only ever loads the
 * proxied /api/portrait copy.
 *
 * @module ui/portraitDialog
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getActiveSigner } from '../systems/millLogin.js';
import { crestSVG } from './crest.js';

const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

/** The proxied image path for a portrait URL. */
export function portraitSrc(url) {
    return url ? `/api/portrait?u=${encodeURIComponent(url)}` : '';
}

/** The character's current portrait URL ('' for none). */
function currentPortrait() {
    return window.getGameStateSync?.()?.character?.portrait ?? '';
}

/** A face for the character: portrait if set, else their crest. */
export function characterFaceHTML(size = 18) {
    const ch = window.getGameStateSync?.()?.character ?? {};
    const crest = crestSVG(gameAPI.npub ?? '', ch.name ?? '', size);
    if (!ch.portrait) return crest;
    // If the portrait fails to load, fall back to the crest in place.
    const fallback = esc(crest).replace(/'/g, '&#39;');
    return `<img src="${esc(portraitSrc(ch.portrait))}" alt="" width="${size}" height="${size}"
                 style="image-rendering:pixelated;display:block;"
                 onerror="this.outerHTML=this.dataset.fallback" data-fallback="${fallback}">`;
}

/** Redraw the small portrait next to the character's name. */
export function renderPortraitThumb() {
    const el = document.getElementById('char-portrait-thumb');
    if (el) el.innerHTML = characterFaceHTML(9);
}

let _el = null;

function close() {
    _el?.remove();
    _el = null;
}

/** Open the portrait dialog over the scene. */
export function openPortraitDialog() {
    close();
    const scene = document.getElementById('scene-container');
    if (!scene) return;
    const box = document.createElement('div');
    box.id = 'portrait-dialog';
    box.style.cssText = 'position:absolute;inset:0;z-index:60;background:rgba(0,0,0,0.8);display:flex;justify-content:center;align-items:flex-start;font-size:8px;line-height:1.5;';
    box.innerHTML = `
      <style>
        #portrait-dialog .pd-win { width: 300px; max-width: calc(100% - 16px); margin-top: 12px; background: #1f1f1f; border: 2px solid #6b21a8; box-shadow: 0 0 12px rgba(147,51,234,0.4); }
        #portrait-dialog .pd-title { display:flex; align-items:center; gap:4px; padding:2px 3px; color:#fff; font-weight:bold; background:linear-gradient(90deg,#4c1d95,#7e22ce); }
        #portrait-dialog .pd-x { margin-left:auto; font:inherit; color:#fff; background:#7f1d1d; width:14px; height:13px; line-height:9px; padding:0; cursor:pointer; border:2px solid; border-color:#dc2626 #450a0a #450a0a #dc2626; }
        #portrait-dialog .pd-body { padding:6px; color:#d1d5db; max-height:330px; overflow-y:auto; }
        #portrait-dialog .pd-sec { color:#9ca3af; text-transform:uppercase; margin:6px 0 2px; border-bottom:1px solid #333; }
        #portrait-dialog input, #portrait-dialog textarea { width:100%; box-sizing:border-box; font:inherit; color:#e5e7eb; background:#111; border:1px solid #4a4a4a; padding:2px 3px; }
        #portrait-dialog textarea { height:62px; resize:none; }
        #portrait-dialog .pd-row { display:flex; gap:4px; margin-top:3px; align-items:center; }
        #portrait-dialog .pd-btn { font:inherit; color:#fff; padding:2px 6px; cursor:pointer; background:#0e7490; border:2px solid; border-color:#22d3ee #155e75 #155e75 #22d3ee; white-space:nowrap; }
        #portrait-dialog .pd-btn.alt { background:#3f3f46; border-color:#71717a #27272a #27272a #71717a; }
        #portrait-dialog .pd-btn:disabled { opacity:0.5; cursor:default; }
        #portrait-dialog .pd-note { color:#9ca3af; }
        #portrait-dialog .pd-msg { min-height:12px; margin-top:4px; }
        #portrait-dialog .pd-prev { display:flex; gap:8px; align-items:flex-end; }
        #portrait-dialog .pd-prev > div { display:flex; flex-direction:column; align-items:center; gap:2px; color:#9ca3af; }
        #portrait-dialog .pd-frame { border:1px solid #4ade80; background:#000; display:flex; align-items:center; justify-content:center; }
      </style>
      <div class="pd-win">
        <div class="pd-title"><span>🎨 PORTRAIT</span><button class="pd-x" title="Close">✕</button></div>
        <div class="pd-body">
          <div class="pd-prev" id="pd-preview"></div>

          <div class="pd-sec">1 · Make one</div>
          <div class="pd-note">Describe your look (optional), copy the prompt, and paste it into any AI image tool.</div>
          <input id="pd-look" maxlength="300" placeholder="e.g. short red hair, scar over one eye, green cloak">
          <div class="pd-row">
            <button class="pd-btn alt" id="pd-gen">Write prompt</button>
            <button class="pd-btn" id="pd-copy" disabled>Copy</button>
          </div>
          <textarea id="pd-prompt" readonly placeholder="Your prompt appears here."></textarea>

          <div class="pd-sec">2 · Set it</div>
          <div class="pd-row">
            <input id="pd-url" placeholder="https://… link to your image">
            <button class="pd-btn" id="pd-use">Use link</button>
          </div>
          <div class="pd-row">
            <button class="pd-btn alt" id="pd-upload">Upload to my Blossom server…</button>
            <input type="file" id="pd-file" accept="image/png,image/jpeg,image/gif,image/webp" style="display:none">
          </div>
          <div class="pd-note" id="pd-blossom-note"></div>
          <div class="pd-row">
            <button class="pd-btn alt" id="pd-remove">Remove portrait</button>
          </div>
          <div class="pd-msg" id="pd-msg"></div>
          <div class="pd-note">Save your game to keep it.</div>
        </div>
      </div>`;
    scene.appendChild(box);
    _el = box;

    const $ = id => box.querySelector(`#${id}`);
    box.querySelector('.pd-x').addEventListener('click', close);
    box.addEventListener('click', e => { if (e.target === box) close(); });
    $('pd-url').value = currentPortrait();
    renderPreview();

    $('pd-gen').addEventListener('click', writePrompt);
    $('pd-copy').addEventListener('click', async () => {
        try {
            await navigator.clipboard.writeText($('pd-prompt').value);
            say('Prompt copied.', '#4ade80');
        } catch (_) {
            $('pd-prompt').select();
            say('Select and copy the prompt by hand.', '#fbbf24');
        }
    });
    $('pd-use').addEventListener('click', () => setPortrait($('pd-url').value.trim()));
    $('pd-remove').addEventListener('click', () => setPortrait(''));
    $('pd-upload').addEventListener('click', () => $('pd-file').click());
    $('pd-file').addEventListener('change', e => {
        const file = e.target.files?.[0];
        e.target.value = '';
        if (file) uploadToBlossom(file);
    });
    loadBlossomServers();
}

function say(text, color = '#d1d5db') {
    const el = _el?.querySelector('#pd-msg');
    if (el) { el.textContent = text; el.style.color = color; }
}

function renderPreview() {
    const el = _el?.querySelector('#pd-preview');
    if (!el) return;
    el.innerHTML = `
        <div><div class="pd-frame" style="width:50px;height:50px;">${characterFaceHTML(48)}</div>portrait</div>
        <div><div class="pd-frame" style="width:24px;height:24px;">${characterFaceHTML(20)}</div>on the grid</div>
        <div><div class="pd-frame" style="width:24px;height:24px;border-color:#6b7280;">${crestSVG(gameAPI.npub ?? '', window.getGameStateSync?.()?.character?.name ?? '', 20)}</div>your crest</div>`;
}

async function writePrompt() {
    const look = _el?.querySelector('#pd-look')?.value ?? '';
    try {
        const q = new URLSearchParams({ npub: gameAPI.npub ?? '', save_id: gameAPI.saveID ?? '', look });
        const resp = await fetch(`/api/game/portrait-prompt?${q}`);
        const out = await resp.json();
        if (!resp.ok || !out.success) throw new Error(out.error ?? `HTTP ${resp.status}`);
        _el.querySelector('#pd-prompt').value = out.prompt;
        _el.querySelector('#pd-copy').disabled = false;
    } catch (err) {
        say(`Couldn't write the prompt: ${err.message}`, '#f87171');
    }
}

async function setPortrait(url) {
    say(url ? 'Checking the image…' : 'Removing…');
    try {
        const resp = await fetch('/api/game/portrait', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ npub: gameAPI.npub, save_id: gameAPI.saveID, url }),
        });
        const out = await resp.json();
        if (!resp.ok || !out.success) throw new Error(out.error ?? `HTTP ${resp.status}`);
        const ch = window.getGameStateSync?.()?.character;
        if (ch) ch.portrait = out.url;
        if (_el) _el.querySelector('#pd-url').value = out.url;
        renderPreview();
        renderPortraitThumb();
        say(url ? 'Portrait set — save your game to keep it.' : 'Portrait removed — your crest shows instead.', '#4ade80');
    } catch (err) {
        say(err.message, '#f87171');
    }
}

let _blossom = [];

async function loadBlossomServers() {
    const note = _el?.querySelector('#pd-blossom-note');
    const btn = _el?.querySelector('#pd-upload');
    try {
        const resp = await fetch('/api/media-servers');
        const out = resp.ok ? await resp.json() : { blossom: [] };
        _blossom = out.blossom ?? [];
    } catch (_) {
        _blossom = [];
    }
    if (!_el) return;
    if (_blossom.length) {
        note.textContent = `Uploads go to ${_blossom[0].replace(/^https?:\/\//, '').replace(/\/$/, '')}.`;
    } else {
        btn.disabled = true;
        note.textContent = 'No Blossom servers on your Nostr profile (kind 10063) — paste a link instead.';
    }
}

async function sha256Hex(buf) {
    const digest = await crypto.subtle.digest('SHA-256', buf);
    return Array.from(new Uint8Array(digest)).map(b => b.toString(16).padStart(2, '0')).join('');
}

/** BUD-02 upload: a kind 24242 auth event signed by the player's signer, then PUT. */
async function uploadToBlossom(file) {
    const server = _blossom[0];
    if (!server) return;
    if (file.size > 8 * 1024 * 1024) { say('That image is over 8 MB.', '#f87171'); return; }
    const signer = getActiveSigner();
    if (!signer?.signEvent) { say('Your signer isn\'t ready — paste a link instead.', '#f87171'); return; }
    try {
        say('Uploading…');
        const hash = await sha256Hex(await file.arrayBuffer());
        const now = Math.floor(Date.now() / 1000);
        const auth = await signer.signEvent({
            kind: 24242,
            created_at: now,
            content: 'Upload Pubkey Quest portrait',
            tags: [['t', 'upload'], ['x', hash], ['expiration', String(now + 3600)]],
        });
        const resp = await fetch(server.replace(/\/+$/, '') + '/upload', {
            method: 'PUT',
            headers: { Authorization: 'Nostr ' + btoa(JSON.stringify(auth)), 'Content-Type': file.type || 'application/octet-stream' },
            body: file,
        });
        if (!resp.ok) throw new Error(`the server answered ${resp.status}`);
        const desc = await resp.json();
        if (!desc?.url) throw new Error('the server returned no link');
        await setPortrait(desc.url);
    } catch (err) {
        logger.error('Blossom upload failed:', err);
        say(`Upload failed: ${err.message}`, '#f87171');
    }
}

window.openPortraitDialog = openPortraitDialog;
