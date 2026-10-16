/**
 * Social poll (docs/draft/social-plan.md §2).
 *
 * Every 3 s while the game page is open — paused, fighting or not — ask the
 * server what's going on around us. It doubles as our presence heartbeat (stop
 * polling for 20 s and other players see us go offline). Results fan out on the
 * event bus:
 *   social:here      players you can see (People column / travel screen)
 *   social:count     {count, area} for the headcount chip
 *   social:chat      new chat lines
 *   social:invites   open party invites
 *   social:party     your party (or null)
 *   social:party-sync  the leader moved you — refresh the world
 *
 * @module systems/socialPoll
 */

import { logger } from '../lib/logger.js';
import { eventBus } from '../lib/events.js';
import { gameAPI } from '../lib/api.js';

const POLL_MS = 3000;

let _timer = null;
let _inFlight = false;
let _last = { here: [], area_count: 0, area_name: '', invites: [], party: null };

/** The latest poll result. */
export function getSocialState() {
    return _last;
}

async function poll() {
    if (_inFlight) return;
    const npub = gameAPI.npub, saveID = gameAPI.saveID;
    if (!npub || !saveID) return;
    _inFlight = true;
    try {
        const q = new URLSearchParams({ npub, save_id: saveID });
        const resp = await fetch(`/api/social/poll?${q}`);
        if (!resp.ok) return;
        const out = await resp.json();
        if (!out?.success) return;
        _last = out;
        eventBus.emit('social:here', out.here ?? []);
        eventBus.emit('social:count', { count: out.area_count ?? 0, area: out.area_name ?? '' });
        if (out.chat?.length) eventBus.emit('social:chat', out.chat);
        eventBus.emit('social:invites', out.invites ?? []);
        eventBus.emit('social:party', out.party ?? null);
        // A follower's clock is the leader's: follow it between our own actions.
        if (out.party && !out.party.you_lead) eventBus.emit('clock:running', Boolean(out.clock_running));
        if (out.party_sync) eventBus.emit('social:party-sync');
        if (out.in_combat) eventBus.emit('social:in-combat');
    } catch (err) {
        logger.debug('social poll failed:', err?.message || err);
    } finally {
        _inFlight = false;
    }
}

/** Poll right away (after sending a chat line, accepting an invite, …). */
export function pollNow() {
    poll();
}

/** Start polling (idempotent). */
export function startSocialPoll() {
    if (_timer) return;
    _timer = setInterval(poll, POLL_MS);
    poll();
}
