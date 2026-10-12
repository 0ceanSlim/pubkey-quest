/**
 * Social over Nostr (docs/draft/social-plan.md §5, §7, §8): the Friends tab
 * (mutual follows), Follow / Unfollow, Mute, and Whispers (NIP-17 DMs).
 *
 * Lists are edited in the browser and signed with the player's own signer; the
 * server checks each edit changes exactly one person before publishing, so a
 * follow made here never wipes follows made in other clients. Whispers are
 * sealed and gift-wrapped here (the server never sees the text) and relayed to
 * the recipient's DM inbox.
 *
 * @module ui/socialNostr
 */

import { nip19, getEventHash, verifyEvent } from 'nostr-tools';
import { createWrap } from 'nostr-tools/nip59';
import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { getActiveSigner } from '../systems/millLogin.js';
import { socialHooks, appendChatLine, startWhisper, noteWhisperFrom, playerRow, openPlayerMenu } from './socialUI.js';
import { crestSVG } from './crest.js';

const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

const hexOf = (npub) => {
    try { return nip19.decode(npub).data; } catch (_) { return null; }
};
const npubOf = (hex) => {
    try { return nip19.npubEncode(hex); } catch (_) { return hex; }
};

function ago(sec) {
    if (!sec) return '';
    const d = Math.max(0, Date.now() / 1000 - sec);
    if (d < 90) return 'just now';
    if (d < 3600) return `${Math.round(d / 60)} min ago`;
    if (d < 86400) return `${Math.round(d / 3600)} h ago`;
    return `${Math.round(d / 86400)} d ago`;
}

// ─── Friends + follows ───────────────────────────────────────────────────────

let _follows = null;   // Set of hex pubkeys, null until loaded
let _friends = [];
let _friendsAt = 0;
let _loading = null;

async function loadFriends(refresh = false) {
    if (_loading) return _loading;
    if (!refresh && _follows && Date.now() - _friendsAt < 60000) return;
    _loading = (async () => {
        try {
            const q = new URLSearchParams({ npub: gameAPI.npub ?? '', save_id: gameAPI.saveID ?? '' });
            if (refresh) q.set('refresh', 'true');
            const resp = await fetch(`/api/friends?${q}`);
            if (!resp.ok) return;
            const out = await resp.json();
            _follows = new Set((out.follows ?? []).map(hexOf).filter(Boolean));
            _friends = out.friends ?? [];
            _friendsAt = Date.now();
        } catch (err) {
            logger.debug('friends load failed:', err);
        } finally {
            _loading = null;
        }
    })();
    return _loading;
}

/** Edit one person in one of your lists (contacts / mutes), signed here. */
async function editList(endpoint, kind, hex, add) {
    const signer = getActiveSigner();
    if (!signer?.signEvent) throw new Error('Your signer isn\'t ready yet.');
    const cur = await (await fetch(endpoint)).json();
    const ev = cur?.event ?? null;
    let tags = (ev?.tags ?? []).map(t => [...t]);
    const has = tags.some(t => t[0] === 'p' && t[1] === hex);
    if (add === has) return false; // nothing to change
    if (add) tags.push(['p', hex]);
    else {
        const i = tags.findIndex(t => t[0] === 'p' && t[1] === hex);
        tags.splice(i, 1);
    }
    const now = Math.floor(Date.now() / 1000);
    const signed = await signer.signEvent({
        kind,
        created_at: Math.max(now, (ev?.created_at ?? 0) + 1),
        tags,
        content: ev?.content ?? '',
    });
    const resp = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ event: signed }),
    });
    const out = await resp.json().catch(() => ({}));
    if (!resp.ok || !out.success) throw new Error(out.error ?? `HTTP ${resp.status}`);
    return true;
}

socialHooks.isFollowing = (npub) => {
    if (!_follows) { loadFriends(); return null; }
    return _follows.has(hexOf(npub));
};

socialHooks.setFollow = async (npub, follow) => {
    const hex = hexOf(npub);
    if (!hex) return;
    try {
        await editList('/api/friends/contacts', 3, hex, follow);
        if (follow) _follows?.add(hex); else _follows?.delete(hex);
        appendChatLine({ channel: 'system', text: follow ? '➕ Followed. If they follow you back, you\'re friends.' : '➖ Unfollowed.' });
        loadFriends(true);
    } catch (err) {
        appendChatLine({ channel: 'system', text: `Couldn't update your follows: ${err.message}` });
    }
};

socialHooks.renderFriendsTab = async (list, foot, count) => {
    if (!_follows) list.innerHTML = '<div class="fp-empty">Finding your mutual follows…</div>';
    await loadFriends();
    const online = _friends.filter(f => f.in_game);
    const rest = _friends.filter(f => !f.in_game);
    count.textContent = _friends.length ? `${online.length} in game` : '';
    list.innerHTML = _friends.length ? '' :
        '<div class="fp-empty">No friends yet. Follow players you meet — when they follow you back, they show up here.</div>';
    for (const f of online) {
        list.appendChild(playerRow({ ...f.in_game, npub: f.npub }));
    }
    for (const f of rest) {
        const sub = f.client
            ? `active on ${f.client} · ${ago(f.last_event_at)}`
            : f.last_event_at ? `last seen ${ago(f.last_event_at)}` : 'no recent activity';
        const row = document.createElement('div');
        row.className = 'fp-row';
        const face = f.picture
            ? `<img src="${esc(f.picture)}" alt="" style="width:12px;height:12px;object-fit:cover">`
            : crestSVG(f.npub, f.name, 12);
        row.innerHTML = `<div class="fp-face">${face}</div><div class="fp-body">
            <div class="fp-line"><span class="fp-name"><span class="fp-dot off">○</span> ${esc(f.name)}</span></div>
            <div class="fp-sub">${esc(sub)}</div></div>`;
        const player = { npub: f.npub, name: f.name, here: false };
        row.addEventListener('contextmenu', (e) => { e.preventDefault(); openPlayerMenu(player, e.clientX, e.clientY); });
        row.addEventListener('click', (e) => openPlayerMenu(player, e.clientX, e.clientY));
        list.appendChild(row);
    }
    foot.innerHTML = '<span>Friends follow you back.</span>';
    const btn = document.createElement('button');
    btn.className = 'fp-btn';
    btn.textContent = '↻';
    btn.title = 'Refresh from Nostr';
    btn.addEventListener('click', async () => { await loadFriends(true); socialHooks.renderFriendsTab(list, foot, count); });
    foot.appendChild(btn);
};

// ─── Mute ────────────────────────────────────────────────────────────────────

let _mutes = new Set();

async function loadMutes() {
    try {
        const out = await (await fetch('/api/mutes')).json();
        _mutes = new Set((out?.event?.tags ?? []).filter(t => t[0] === 'p').map(t => t[1]));
    } catch (_) { /* keep what we have */ }
}

socialHooks.isMuted = (npub) => _mutes.has(hexOf(npub));
socialHooks.setMute = async (npub, mute) => {
    const hex = hexOf(npub);
    if (!hex) return;
    try {
        await editList('/api/mutes', 10000, hex, mute);
        if (mute) _mutes.add(hex); else _mutes.delete(hex);
        appendChatLine({ channel: 'system', text: mute ? '🔇 Muted — you won\'t see their chat or whispers.' : '🔈 Unmuted.' });
    } catch (err) {
        appendChatLine({ channel: 'system', text: `Couldn't update your mute list: ${err.message}` });
    }
};

// ─── Whispers (NIP-17) ───────────────────────────────────────────────────────

function canWhisper() {
    const signer = getActiveSigner();
    const method = window.sessionManager?.getSigningMethod?.();
    return typeof signer?.nip44?.encrypt === 'function' && method !== 'amber';
}

const UNAVAILABLE = 'Whispers need a sign-in that supports encrypted messages (key, bunker, Google, or a NIP-44 extension).';

function myHex() {
    return hexOf(gameAPI.npub ?? '') ?? window.sessionManager?.getPublicKey?.();
}

/** Seal a rumor to one recipient (NIP-59) and gift-wrap it. */
async function sealAndWrap(signer, rumor, toHex) {
    const content = await signer.nip44.encrypt(toHex, JSON.stringify(rumor));
    const twoDays = 2 * 24 * 60 * 60;
    const seal = await signer.signEvent({
        kind: 13,
        created_at: Math.floor(Date.now() / 1000) - Math.floor(Math.random() * twoDays),
        tags: [],
        content,
    });
    return createWrap(seal, toHex);
}

const _sentRumors = new Set();

async function sendWhisper(to, text) {
    if (!to?.npub) return;
    if (!canWhisper()) { appendChatLine({ channel: 'system', text: UNAVAILABLE }); return; }
    const signer = getActiveSigner();
    const me = myHex(), toHex = hexOf(to.npub);
    if (!me || !toHex) return;
    try {
        const rumor = { kind: 14, pubkey: me, created_at: Math.floor(Date.now() / 1000), tags: [['p', toHex]], content: text };
        rumor.id = getEventHash(rumor);
        _sentRumors.add(rumor.id);
        const wraps = [await sealAndWrap(signer, rumor, toHex), await sealAndWrap(signer, rumor, me)];
        const resp = await fetch('/api/dm/send', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ wraps }),
        });
        const out = await resp.json().catch(() => ({}));
        if (!resp.ok || !out.success) throw new Error(out.error ?? `HTTP ${resp.status}`);
        appendChatLine({ channel: 'whisper', from_npub: gameAPI.npub, to_name: to.name, text });
    } catch (err) {
        appendChatLine({ channel: 'system', text: `Whisper not sent: ${err.message}` });
    }
}

const _seenWraps = new Set();
const _seenRumors = new Set();
const _loadedAt = Math.floor(Date.now() / 1000);
let _inboxSince = 0;
const _names = new Map();

async function nameFor(hex) {
    if (_names.has(hex)) return _names.get(hex);
    const f = _friends.find(x => hexOf(x.npub) === hex);
    let name = f?.name;
    if (!name) {
        try {
            const out = await (await fetch(`/api/profile?npub=${encodeURIComponent(npubOf(hex))}`)).json();
            name = out?.profile?.display_name || out?.profile?.name;
        } catch (_) { /* fall back */ }
    }
    name = name || `${npubOf(hex).slice(0, 12)}…`;
    _names.set(hex, name);
    return name;
}

// NIP-42: answer the AUTH challenges of DM relays that want to know it's you
// before they hand over your whispers (once per session; a relay that won't
// take it is skipped and noted once).
let _authDone = false;
async function authDMRelays() {
    if (_authDone) return;
    _authDone = true;
    const signer = getActiveSigner();
    try {
        const out = await (await fetch('/api/dm/auth')).json();
        for (const ch of (out?.challenges ?? [])) {
            try {
                const ev = await signer.signEvent({
                    kind: 22242,
                    created_at: Math.floor(Date.now() / 1000),
                    tags: [['relay', ch.relay], ['challenge', ch.challenge]],
                    content: '',
                });
                const resp = await fetch('/api/dm/auth', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ relay: ch.relay, event: ev }),
                });
                if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
            } catch (err) {
                appendChatLine({ channel: 'system', text: `Couldn't sign in to ${ch.relay.replace(/^wss?:\/\//, '')} — some whispers there may not arrive.` });
            }
        }
    } catch (err) {
        logger.debug('DM relay auth failed:', err);
    }
}

async function pollInbox() {
    if (!canWhisper()) return;
    await authDMRelays();
    const signer = getActiveSigner();
    const me = myHex();
    try {
        const q = _inboxSince ? `?since=${_inboxSince}` : '';
        const out = await (await fetch(`/api/dm/inbox${q}`)).json();
        _inboxSince = Math.floor(Date.now() / 1000);
        for (const wrap of (out?.wraps ?? [])) {
            if (_seenWraps.has(wrap.id)) continue;
            _seenWraps.add(wrap.id);
            try {
                const seal = JSON.parse(await signer.nip44.decrypt(wrap.pubkey, wrap.content));
                if (!verifyEvent(seal)) continue;
                const rumor = JSON.parse(await signer.nip44.decrypt(seal.pubkey, seal.content));
                if (rumor.pubkey !== seal.pubkey || rumor.kind !== 14) continue;
                if (_seenRumors.has(rumor.id) || _sentRumors.has(rumor.id)) continue;
                _seenRumors.add(rumor.id);
                // Only whispers from this session show in the log, not your whole history.
                if ((rumor.created_at ?? 0) < _loadedAt - 60) continue;
                if (seal.pubkey === me) continue; // your own copy from another client
                if (_mutes.has(seal.pubkey)) continue;
                const fromNpub = npubOf(seal.pubkey);
                const name = await nameFor(seal.pubkey);
                appendChatLine({ id: undefined, channel: 'whisper', from_npub: fromNpub, from_name: name, text: rumor.content });
                noteWhisperFrom(fromNpub, name);
            } catch (_) { /* not for us / undecryptable */ }
        }
    } catch (err) {
        logger.debug('inbox poll failed:', err);
    }
}

socialHooks.whisper = (npub, name) => {
    if (!canWhisper()) { appendChatLine({ channel: 'system', text: UNAVAILABLE }); return; }
    startWhisper(npub, name);
};
socialHooks.sendWhisper = sendWhisper;
socialHooks.whisperUnavailable = UNAVAILABLE;

// ─── Start-up ────────────────────────────────────────────────────────────────

function init() {
    // Background: your follows (for the menu) and mutes (to hide chat), a moment
    // after the game loads; then the whisper inbox every 15 s.
    setTimeout(() => { loadFriends(); loadMutes(); }, 4000);
    setTimeout(() => { pollInbox(); setInterval(pollInbox, 15000); }, 6000);
}

if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
else init();
