/**
 * Social UI (docs/draft/social-plan.md §3–9): players in the People column and
 * on the road, the player menu and card, the headcount chip, the Friends panel,
 * chat in the log, party invites, the party HUD, and following the leader.
 *
 * Data comes from the social poll (systems/socialPoll.js) over the event bus;
 * actions go to /api/social/* and /api/party/*. Friends (S3), whispers (S5) and
 * mute (S6) plug in through the hooks at the bottom.
 *
 * @module ui/socialUI
 */

import { logger } from '../lib/logger.js';
import { eventBus } from '../lib/events.js';
import { gameAPI } from '../lib/api.js';
import { refreshGameState } from '../state/gameState.js';
import { smoothClock } from '../systems/smoothClock.js';
import { getSocialState, pollNow, startSocialPoll } from '../systems/socialPoll.js';
import { crestSVG } from './crest.js';

const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const $ = id => document.getElementById(id);

// Hooks the later slices fill in (friends, whispers, mute).
export const socialHooks = {
    /** S3: is npub someone I follow? null when unknown. */
    isFollowing: () => null,
    /** S3: follow / unfollow. */
    setFollow: null,
    /** S5: open a whisper to a player (npub, name). null = unavailable. */
    whisper: null,
    whisperUnavailable: 'Whispers arrive soon.',
    /** S6: is npub muted? / toggle mute. */
    isMuted: () => false,
    setMute: null,
    /** S3: load the Friends tab into a container. */
    renderFriendsTab: null,
};

// ─── Small helpers ───────────────────────────────────────────────────────────

/** A player's face for lists: portrait if set, else their crest. */
function faceHTML(p, size = 12) {
    if (p?.portrait_src) {
        return `<img src="${esc(p.portrait_src)}" alt="" width="${size}" height="${size}" style="image-rendering:pixelated">`;
    }
    return crestSVG(p?.npub ?? '', p?.name ?? '', size);
}

/** "40 min ahead", "2h behind", "same hour". */
function clockOffsetText(min) {
    const a = Math.abs(min ?? 0);
    if (a < 5) return 'same time';
    const span = a >= 120 ? `${Math.round(a / 60)}h` : `${a} min`;
    return `${span} ${min > 0 ? 'ahead' : 'behind'}`;
}

async function post(path, body) {
    const resp = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ npub: gameAPI.npub, save_id: gameAPI.saveID, ...body }),
    });
    let out = {};
    try { out = await resp.json(); } catch (_) { /* empty */ }
    if (!resp.ok && !out.error) out.error = `HTTP ${resp.status}`;
    return out;
}

// ─── Context menu (shared) ───────────────────────────────────────────────────

let _menu = null;

/** Show a small win95 menu at (x, y): items [{label, onClick, disabled, title}]. */
export function showMenu(x, y, items) {
    closeMenu();
    const m = document.createElement('div');
    m.style.cssText = 'position:fixed;z-index:9999;min-width:120px;background:#2a2a2a;font-size:8px;' +
        'border:2px solid;border-color:#4a4a4a #0f0f0f #0f0f0f #4a4a4a;box-shadow:2px 2px 0 rgba(0,0,0,0.5);padding:1px;';
    for (const it of items) {
        if (it.separator) {
            const hr = document.createElement('div');
            hr.style.cssText = 'height:1px;background:#4a4a4a;margin:2px 0;';
            m.appendChild(hr);
            continue;
        }
        const b = document.createElement('button');
        b.textContent = it.label;
        b.disabled = !!it.disabled;
        if (it.title) b.title = it.title;
        b.style.cssText = 'display:block;width:100%;text-align:left;font:inherit;padding:2px 6px;border:0;background:none;' +
            `color:${it.disabled ? '#6b7280' : '#e5e7eb'};cursor:${it.disabled ? 'default' : 'pointer'};white-space:nowrap;`;
        if (!it.disabled) {
            b.addEventListener('mouseenter', () => { b.style.background = '#1e3a8a'; });
            b.addEventListener('mouseleave', () => { b.style.background = 'none'; });
            b.addEventListener('click', (e) => { e.stopPropagation(); closeMenu(); it.onClick?.(); });
        }
        m.appendChild(b);
    }
    document.body.appendChild(m);
    const r = m.getBoundingClientRect();
    m.style.left = `${Math.max(2, Math.min(x, window.innerWidth - r.width - 2))}px`;
    m.style.top = `${Math.max(2, Math.min(y, window.innerHeight - r.height - 2))}px`;
    _menu = m;
    setTimeout(() => document.addEventListener('click', closeMenu, { once: true }), 0);
}

export function closeMenu() {
    _menu?.remove();
    _menu = null;
}

/** Right-click, long-press (touch) and optionally plain click open a player's menu. */
function bindPlayerMenu(el, player, { click = false } = {}) {
    el.addEventListener('contextmenu', (e) => { e.preventDefault(); openPlayerMenu(player, e.clientX, e.clientY); });
    if (click) el.addEventListener('click', (e) => { e.stopPropagation(); openPlayerMenu(player, e.clientX, e.clientY); });
    let t = null;
    el.addEventListener('touchstart', (e) => {
        const touch = e.touches[0];
        t = setTimeout(() => openPlayerMenu(player, touch.clientX, touch.clientY), 500);
    }, { passive: true });
    ['touchend', 'touchmove', 'touchcancel'].forEach(ev => el.addEventListener(ev, () => clearTimeout(t)));
}

/** The player menu: whisper, invite / party actions, follow, profile, mute. */
export function openPlayerMenu(player, x, y) {
    const st = getSocialState();
    const party = st.party;
    const inMyParty = !!party?.members?.some(m => m.npub === player.npub);
    const iLead = !!party?.you_lead;
    const canInvite = player.here && !inMyParty && (!party || iLead);
    const items = [];

    items.push(socialHooks.whisper
        ? { label: '💬 Whisper', onClick: () => socialHooks.whisper(player.npub, player.name) }
        : { label: '💬 Whisper', disabled: true, title: socialHooks.whisperUnavailable });

    if (inMyParty) {
        if (iLead) {
            items.push({ label: '👑 Make leader', onClick: () => partyAction('promote', { to: player.npub }) });
            items.push({ label: '🚪 Remove from party', onClick: () => partyAction('kick', { to: player.npub }) });
        }
    } else {
        items.push({
            label: '🎉 Invite to party', disabled: !canInvite, onClick: () => partyAction('invite', { to: player.npub }),
            title: canInvite ? '' : (player.here ? 'Only the party leader can invite.' : 'You can only invite someone standing here with you.'),
        });
    }

    const following = socialHooks.isFollowing(player.npub);
    if (socialHooks.setFollow && following !== null) {
        items.push(following
            ? { label: '➖ Unfollow', onClick: () => socialHooks.setFollow(player.npub, false) }
            : { label: '➕ Follow', onClick: () => socialHooks.setFollow(player.npub, true) });
    } else {
        items.push({ label: '➕ Follow', disabled: true, title: 'Loading your follows…' });
    }
    items.push({ label: '👁 View profile', onClick: () => openPlayerCard(player) });
    if (socialHooks.setMute) {
        const muted = socialHooks.isMuted(player.npub);
        items.push({ label: muted ? '🔈 Unmute' : '🔇 Mute', onClick: () => socialHooks.setMute(player.npub, !muted) });
    }
    showMenu(x, y, items);
}

// ─── Player card ─────────────────────────────────────────────────────────────

const _profiles = new Map();

async function fetchProfile(npub) {
    if (_profiles.has(npub)) return _profiles.get(npub);
    try {
        const resp = await fetch(`/api/profile?npub=${encodeURIComponent(npub)}`);
        const out = resp.ok ? await resp.json() : null;
        _profiles.set(npub, out);
        return out;
    } catch (_) {
        return null;
    }
}

export async function openPlayerCard(player) {
    const card = $('player-card');
    const content = $('player-card-content');
    if (!card || !content) return;
    card.classList.remove('hidden');
    const render = (prof) => {
        const p = prof?.profile ?? {};
        const pic = p.picture
            ? `<img src="${esc(p.picture)}" alt="" style="width:32px;height:32px;object-fit:cover;image-rendering:pixelated;border:1px solid #3b82f6">`
            : `<div style="width:32px;height:32px;display:flex;align-items:center;justify-content:center;border:1px solid #3b82f6">${faceHTML(player, 28)}</div>`;
        const display = p.display_name || p.name || `${player.npub.slice(0, 12)}…`;
        const charFace = `<div style="width:24px;height:24px;border:1px solid #4ade80;display:flex;align-items:center;justify-content:center">${faceHTML(player, 22)}</div>`;
        content.innerHTML = `
            <div style="display:flex;gap:6px;align-items:center">${pic}
                <div style="min-width:0"><div style="color:#fff;font-weight:bold">${esc(display)}</div>
                ${p.nip05 ? `<div class="pc-muted">${esc(p.nip05)}</div>` : ''}</div></div>
            <div style="margin-top:6px;display:flex;gap:6px;align-items:center">${charFace}
                <div><div style="color:#facc15">${esc(player.name)} · Level ${player.level ?? '?'} ${esc(player.race ?? '')} ${esc(player.class ?? '')}</div>
                <div class="pc-muted">${esc(player.where ?? '')}${player.here ? ' · here' : ` · ${clockOffsetText(player.clock_offset)}`}${player.in_combat ? ' · ⚔ in combat' : ''}</div></div></div>
            <div id="pc-actions" style="margin-top:8px;display:flex;flex-wrap:wrap;gap:3px"></div>`;
        const actions = content.querySelector('#pc-actions');
        const btn = (label, fn, disabled, title) => {
            const b = document.createElement('button');
            b.className = 'pc-btn';
            b.textContent = label;
            b.disabled = !!disabled;
            if (title) b.title = title;
            b.addEventListener('click', () => { closePlayerCard(); fn(); });
            actions.appendChild(b);
        };
        btn('💬 Whisper', () => socialHooks.whisper?.(player.npub, player.name), !socialHooks.whisper, socialHooks.whisperUnavailable);
        btn('🎉 Invite', () => partyAction('invite', { to: player.npub }), !player.here, 'Only someone standing here with you');
        const following = socialHooks.isFollowing(player.npub);
        if (socialHooks.setFollow && following !== null) {
            btn(following ? '➖ Unfollow' : '➕ Follow', () => socialHooks.setFollow(player.npub, !following));
        }
        if (socialHooks.setMute) {
            const muted = socialHooks.isMuted(player.npub);
            btn(muted ? '🔈 Unmute' : '🔇 Mute', () => socialHooks.setMute(player.npub, !muted));
        }
    };
    render(_profiles.get(player.npub));
    render(await fetchProfile(player.npub));
}

export function closePlayerCard() {
    $('player-card')?.classList.add('hidden');
}

// ─── Players here: People column and the road ────────────────────────────────

let _here = [];

/** Re-draw player buttons after the People column (or travel screen) re-renders. */
export function renderPlayersHere(list = _here) {
    _here = list ?? [];
    const visible = _here;

    // Town: the People column.
    const container = document.querySelector('#npc-buttons > div');
    if (container) {
        container.querySelectorAll('[data-player-npub]').forEach(b => b.remove());
        const empty = container.querySelector('.empty-message');
        if (empty) empty.style.display = visible.length ? 'none' : '';
        for (const p of visible) container.appendChild(playerButton(p));
    }

    // Road: a row on the travel screen.
    const actions = $('action-buttons');
    const travelling = !!$('travel-bar-container');
    let row = $('travel-people-row');
    if (travelling && actions) {
        if (!row) {
            row = document.createElement('div');
            row.id = 'travel-people-row';
            row.style.cssText = 'width:100%;display:flex;gap:3px;flex-wrap:wrap;align-items:center;font-size:7px;color:#9ca3af;';
            const poiRow = $('poi-enter-row');
            actions.insertBefore(row, poiRow ? poiRow.nextSibling : actions.children[1] ?? null);
        }
        row.innerHTML = visible.length ? '<span>Nearby:</span>' : '';
        row.style.display = visible.length ? 'flex' : 'none';
        for (const p of visible) {
            const b = playerButton(p);
            b.style.flex = '0 1 auto';
            row.appendChild(b);
        }
    } else if (row) {
        row.remove();
    }
}

function playerButton(p) {
    const b = document.createElement('button');
    b.dataset.playerNpub = p.npub;
    b.className = 'text-white leading-tight text-center overflow-hidden';
    b.style.cssText = 'font-size:7px;background:#4a7ab0;color:#fff;cursor:pointer;padding:2px 4px;' +
        'border-top:1px solid #fff;border-left:1px solid #fff;border-right:1px solid #000;border-bottom:1px solid #000;' +
        'box-shadow:inset -1px -1px 0 #404040, inset 1px 1px 0 rgba(255,255,255,0.3);display:flex;align-items:center;justify-content:center;gap:3px;';
    b.innerHTML = `<span style="display:inline-flex;width:8px;height:8px">${faceHTML(p, 8)}</span>${esc(p.name)}${p.in_combat ? ' ⚔' : ''}`;
    b.title = `${p.name} · Lv${p.level} ${p.race} ${p.class} — click for options`;
    bindPlayerMenu(b, p, { click: true });
    return b;
}

// ─── Headcount ───────────────────────────────────────────────────────────────

function renderHeadcount({ count, area }) {
    const chip = $('headcount-chip');
    if (!chip) return;
    chip.classList.toggle('hidden', !count);
    chip.textContent = `👥 ${count}`;
    chip.title = `${count} other adventurer${count === 1 ? '' : 's'} in ${area || 'this area'} — click to see who`;
}

// ─── Friends panel ───────────────────────────────────────────────────────────

let _fpTab = 'friends';
let _fpVisible = false;
let _areaTimer = null;

export function showFriendsTab(tab) {
    _fpTab = tab === 'area' ? 'area' : 'friends';
    document.querySelectorAll('.fp-subtab').forEach(b => b.classList.toggle('on', b.dataset.fpTab === _fpTab));
    loadFriendsPanel();
}

export function friendsPanelVisible(visible) {
    _fpVisible = visible;
    clearInterval(_areaTimer);
    _areaTimer = null;
    if (visible) {
        loadFriendsPanel();
        _areaTimer = setInterval(() => { if (_fpTab === 'area') loadFriendsPanel(); }, 10000);
    }
}

async function loadFriendsPanel() {
    if (!_fpVisible) return;
    const list = $('fp-list'), foot = $('fp-foot'), count = $('fp-count');
    if (!list) return;
    if (_fpTab === 'friends') {
        if (socialHooks.renderFriendsTab) {
            await socialHooks.renderFriendsTab(list, foot, count);
        } else {
            list.innerHTML = '<div class="fp-empty">Loading your friends…</div>';
            foot.innerHTML = '';
            count.textContent = '';
        }
        return;
    }
    try {
        const q = new URLSearchParams({ npub: gameAPI.npub, save_id: gameAPI.saveID });
        const resp = await fetch(`/api/social/area?${q}`);
        const out = await resp.json();
        if (_fpTab !== 'area') return;
        const players = out.players ?? [];
        count.textContent = players.length ? `${players.length} online` : '';
        list.innerHTML = players.length ? '' :
            `<div class="fp-empty">No one else is adventuring in ${esc(out.area_name || 'this area')} right now.</div>`;
        for (const p of players) list.appendChild(playerRow(p));
        foot.innerHTML = `<span>${esc(out.area_name || '')}</span>`;
    } catch (err) {
        list.innerHTML = '<div class="fp-empty">Couldn\'t load who\'s here.</div>';
    }
}

/** One row in the Friends panel for an in-game player. */
export function playerRow(p, extra = {}) {
    const row = document.createElement('div');
    row.className = 'fp-row';
    const online = extra.online ?? true;
    const sub = extra.sub ?? (p.here ? 'here' : `${p.where || ''} · ${clockOffsetText(p.clock_offset)}`);
    row.innerHTML = `
        <div class="fp-face">${faceHTML(p, 12)}</div>
        <div class="fp-body">
            <div class="fp-line"><span class="fp-name"><span class="fp-dot${online ? '' : ' off'}">${online ? '●' : '○'}</span> ${esc(p.name)}</span>
                <span class="fp-meta">${p.level ? `Lv${p.level} ${esc(p.class ?? '')}` : ''}</span></div>
            <div class="fp-sub${p.here ? ' here' : ''}">${esc(sub)}${socialHooks.isMuted(p.npub) ? ' · 🔇' : ''}</div>
        </div>`;
    row.title = 'Right-click (or long-press) for options';
    bindPlayerMenu(row, p);
    return row;
}

// ─── Chat in the log ─────────────────────────────────────────────────────────

const CHANNELS = ['say', 'area', 'party'];
const CHANNEL_LABEL = { say: 'Say', area: 'Area', party: 'Party', whisper: 'Whisper', system: '' };
const CHANNEL_COLOR = { say: '#ffffff', area: '#7dd3fc', party: '#4ade80', whisper: '#f9a8d4', system: '#fbbf24' };
let _channel = 'say';
let _whisperTo = null; // {npub, name} when the channel is a whisper
const _seen = new Set();

/** Append one chat line to the log (and the combat log mid-fight). */
export function appendChatLine(line) {
    if (_seen.has(line.id) && line.id) return;
    if (line.id) _seen.add(line.id);
    if (line.from_npub && socialHooks.isMuted(line.from_npub)) return;
    const mine = line.from_npub && line.from_npub === gameAPI.npub;
    const make = () => {
        const p = document.createElement('p');
        p.className = 'border-l-2 pl-2';
        p.style.borderColor = CHANNEL_COLOR[line.channel] ?? '#4b5563';
        if (mine) p.style.opacity = '0.7';
        if (line.channel === 'system') {
            p.style.color = CHANNEL_COLOR.system;
            p.textContent = line.text;
        } else {
            const tag = line.channel === 'whisper' && mine && line.to_name ? `To ${line.to_name}` : CHANNEL_LABEL[line.channel] ?? line.channel;
            p.innerHTML = `<span style="color:${CHANNEL_COLOR[line.channel] ?? '#fff'}">[${esc(tag)}]</span> ` +
                (line.channel === 'whisper' && mine ? '' : `<b>${esc(line.from_name || 'Someone')}:</b> `) + esc(line.text);
        }
        return p;
    };
    for (const id of ['game-text', 'combat-log']) {
        const log = $(id);
        if (!log || (id === 'game-text' && $('combat-log') && log.contains($('combat-log')))) continue;
        log.appendChild(make());
        const scroller = log.closest('.overflow-y-auto') ?? log.parentElement;
        if (scroller) scroller.scrollTop = scroller.scrollHeight;
    }
    _history.push(line);
    if (_history.length > 100) _history.shift();
}

// Lines that arrived during a fight are re-added when the normal log comes back.
const _history = [];
let _combatStartedAt = null;

function setChannel(ch, whisperTo = null) {
    _channel = ch;
    _whisperTo = whisperTo;
    const btn = $('chat-channel');
    if (btn) {
        btn.textContent = ch === 'whisper' ? `→ ${whisperTo?.name ?? '?'} ▾` : `${CHANNEL_LABEL[ch]} ▾`;
        btn.style.color = CHANNEL_COLOR[ch];
    }
}

/** Start a whisper to someone in the chat line (S5 calls this). */
export function startWhisper(npub, name) {
    setChannel('whisper', { npub, name });
    $('chat-input')?.focus();
}

async function sendChat() {
    const input = $('chat-input');
    if (!input) return;
    let text = input.value.trim();
    if (!text) return;
    let channel = _channel;
    // Shortcuts: /s /a /p /w name, /r
    const m = text.match(/^\/(s|a|p|w|r)\b\s*(.*)$/i);
    if (m) {
        const cmd = m[1].toLowerCase();
        if (cmd === 'w') {
            const [name, ...rest] = m[2].split(/\s+/);
            const target = findPlayerByName(name);
            if (!target) { appendChatLine({ channel: 'system', text: `No one called "${name}" is around.` }); return; }
            setChannel('whisper', { npub: target.npub, name: target.name });
            text = rest.join(' ');
            channel = 'whisper';
        } else if (cmd === 'r') {
            if (!_lastWhisperFrom) { appendChatLine({ channel: 'system', text: 'No one has whispered to you yet.' }); return; }
            setChannel('whisper', _lastWhisperFrom);
            text = m[2];
            channel = 'whisper';
        } else {
            channel = { s: 'say', a: 'area', p: 'party' }[cmd];
            setChannel(channel);
            text = m[2];
        }
        if (!text.trim()) { input.value = ''; return; }
    }
    input.value = '';
    if (channel === 'whisper') {
        if (!socialHooks.sendWhisper) { appendChatLine({ channel: 'system', text: socialHooks.whisperUnavailable }); return; }
        await socialHooks.sendWhisper(_whisperTo, text);
        return;
    }
    const out = await post('/api/social/say', { channel, text });
    if (!out.success) appendChatLine({ channel: 'system', text: out.error ?? 'Not sent.' });
    pollNow();
}

let _lastWhisperFrom = null;
/** S5: remember who whispered last, for /r. */
export function noteWhisperFrom(npub, name) { _lastWhisperFrom = { npub, name }; }

function findPlayerByName(name) {
    const n = (name || '').toLowerCase();
    const st = getSocialState();
    const pool = [...(st.here ?? []), ...(st.party?.members ?? []), ...(_areaCache ?? [])];
    return pool.find(p => (p.name || '').toLowerCase() === n) ?? pool.find(p => (p.name || '').toLowerCase().startsWith(n));
}
let _areaCache = [];

function initChatInput() {
    const input = $('chat-input'), btn = $('chat-channel');
    if (!input || input.dataset.bound) return;
    input.dataset.bound = '1';
    // Typing never triggers game hotkeys.
    ['keydown', 'keyup', 'keypress'].forEach(ev => input.addEventListener(ev, e => e.stopPropagation()));
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') { e.preventDefault(); sendChat(); }
        if (e.key === 'Escape') input.blur();
    });
    btn?.addEventListener('click', () => {
        const order = getSocialState().party ? CHANNELS : CHANNELS.filter(c => c !== 'party');
        const i = order.indexOf(_channel);
        setChannel(order[(i + 1) % order.length]);
    });
    setChannel('say');
}

// ─── Party: invites, HUD, follow the leader ──────────────────────────────────

const _shownInvites = new Set();

function renderInvites(invites) {
    for (const inv of invites) {
        const key = `${inv.from_npub}`;
        if (_shownInvites.has(key)) continue;
        _shownInvites.add(key);
        setTimeout(() => _shownInvites.delete(key), (inv.expires_in ?? 120) * 1000);
        const log = $('combat-log') ?? $('game-text');
        if (!log) continue;
        const p = document.createElement('p');
        p.className = 'border-l-2 pl-2';
        p.style.borderColor = '#4ade80';
        p.innerHTML = `<span style="color:#4ade80">🎉 ${esc(inv.from_name)} invites you to their party.</span> `;
        const mk = (label, fn, bg) => {
            const b = document.createElement('button');
            b.textContent = label;
            b.style.cssText = `font:inherit;color:#fff;background:${bg};padding:0 4px;margin-left:2px;cursor:pointer;border:1px solid #000;`;
            b.addEventListener('click', async () => {
                p.querySelectorAll('button').forEach(x => { x.disabled = true; });
                await fn();
            });
            p.appendChild(b);
        };
        mk('Accept', async () => {
            const out = await post('/api/party/accept', { from: inv.from_npub });
            if (!out.success) appendChatLine({ channel: 'system', text: out.error ?? 'Couldn\'t join.' });
            else onPartySync();
            pollNow();
        }, '#15803d');
        mk('Decline', async () => { await post('/api/party/decline', { from: inv.from_npub }); }, '#7f1d1d');
        log.appendChild(p);
        const scroller = log.closest('.overflow-y-auto') ?? log.parentElement;
        if (scroller) scroller.scrollTop = scroller.scrollHeight;
    }
}

let _party = null;

function renderPartyHUD(party) {
    _party = party;
    const hud = $('party-hud');
    if (!hud) return;
    if (!party || !party.members?.length) {
        hud.classList.add('hidden');
        hud.innerHTML = '';
        return;
    }
    hud.classList.remove('hidden');
    hud.innerHTML = '';
    for (const m of party.members) {
        const row = document.createElement('div');
        row.style.cssText = 'background:rgba(0,0,0,0.7);border:1px solid #14532d;padding:1px 2px;cursor:context-menu;' +
            `opacity:${m.online ? 1 : 0.5};`;
        const pct = m.max_hp > 0 ? Math.max(0, Math.min(100, (m.hp / m.max_hp) * 100)) : 0;
        row.innerHTML = `
            <div style="display:flex;align-items:center;gap:2px;font-size:6px;color:${m.you ? '#86efac' : '#e5e7eb'};white-space:nowrap;overflow:hidden">
                <span style="display:inline-flex;width:7px;height:7px">${faceHTML(m, 7)}</span>
                ${m.leader ? '👑' : ''}<span style="overflow:hidden;text-overflow:ellipsis">${esc(m.name || m.npub.slice(0, 8))}</span>${m.in_combat ? ' ⚔' : ''}
            </div>
            <div style="height:2px;background:#1a1a1a;margin-top:1px"><div style="height:100%;width:${pct}%;background:#dc2626"></div></div>`;
        row.title = `${m.name} — ${m.hp}/${m.max_hp} HP${m.leader ? ' · leader' : ''}`;
        if (!m.you) bindPlayerMenu(row, { ...m, here: true }, { click: true });
        else row.addEventListener('click', (e) => openSelfPartyMenu(e.clientX, e.clientY));
        hud.appendChild(row);
    }
}

function openSelfPartyMenu(x, y) {
    showMenu(x, y, [{ label: '🚪 Leave party', onClick: () => partyAction('leave') }]);
}

async function partyAction(action, body = {}) {
    const out = await post(`/api/party/${action}`, body);
    if (out.success) {
        if (out.message) appendChatLine({ channel: 'system', text: out.message });
        if (out.party !== undefined) renderPartyHUD(out.party);
    } else {
        appendChatLine({ channel: 'system', text: out.error ?? 'That didn\'t work.' });
    }
    pollNow();
}

/** The leader moved us: refresh the world and snap the clock. */
let _syncing = false;
async function onPartySync() {
    if (_syncing) return;
    _syncing = true;
    try {
        const st = await refreshGameState();
        const ch = st?.character;
        if (ch && ch.time_of_day !== undefined) smoothClock.syncFromBackend(ch.time_of_day, ch.current_day, true);
    } catch (err) {
        logger.warn('party sync refresh failed:', err);
    } finally {
        _syncing = false;
    }
}

// ─── Wiring ──────────────────────────────────────────────────────────────────

eventBus.on('social:here', (list) => renderPlayersHere(list));
eventBus.on('social:count', renderHeadcount);
eventBus.on('social:chat', (lines) => lines.forEach(appendChatLine));
eventBus.on('social:invites', renderInvites);
eventBus.on('social:party', renderPartyHUD);
eventBus.on('social:party-sync', onPartySync);
eventBus.on('log:replaced', () => { _combatStartedAt = _history.length; });
eventBus.on('log:restored', () => {
    if (_combatStartedAt === null) return;
    const missed = _history.slice(_combatStartedAt);
    _combatStartedAt = null;
    missed.forEach(l => { _seen.delete(l.id); });
    _history.length = Math.max(0, _history.length - missed.length);
    missed.forEach(l => appendChatLine({ ...l }));
});

function init() {
    initChatInput();
    startSocialPoll();
}

if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
else init();

window.socialState = getSocialState;
window.socialUI = {
    showFriendsTab, friendsPanelVisible, closePlayerCard, openPlayerCard, renderPlayersHere,
    openPlayerMenu, startWhisper,
};
