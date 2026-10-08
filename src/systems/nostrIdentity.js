/**
 * Nostr Identity
 *
 * Editing the player's own Nostr identity from inside the game: their profile
 * (kind 0) and their relay list (kind 10002).
 *
 * The key never leaves the browser. MILL signs; the server verifies the
 * signature, checks the event belongs to the signed-in player, and routes it to
 * that player's own write relays. So everything here is: build an event → ask
 * MILL to sign it → POST it to /api/publish.
 *
 * @module systems/nostrIdentity
 */

import { logger } from '../lib/logger.js';
import { API_BASE_URL } from '../config/constants.js';

/** NIP-89: tells other clients what published an event. */
const CLIENT_TAG = ['client', 'Pubkey Quest'];

/** The kind-0 fields the in-game editor owns. */
export const PROFILE_FIELDS = ['display_name', 'name', 'picture', 'about', 'nip05', 'lud16', 'website'];

/**
 * Sign an event with MILL's signer.
 *
 * MILL installs itself as window.nostr, but a page reload rebuilds it
 * asynchronously and NIP-07 extensions inject late, so wait briefly rather than
 * failing on a race.
 *
 * @param {object} draft - Unsigned event (kind, content, tags)
 * @returns {Promise<object>} the signed event
 */
async function sign(draft) {
    for (const delay of [0, 250, 500, 1000, 2000]) {
        if (window.nostr?.signEvent) break;
        await new Promise((r) => setTimeout(r, delay));
    }
    if (!window.nostr?.signEvent) {
        throw new Error('No signer available — sign in again to make changes');
    }
    return window.nostr.signEvent({
        created_at: Math.floor(Date.now() / 1000),
        content: '',
        ...draft,
        // Appended last so a draft can't drop it.
        tags: [...(draft.tags || []), CLIENT_TAG],
    });
}

/**
 * Hand a signed event to the server, which verifies and routes it.
 * @param {object} event - A signed Nostr event
 * @returns {Promise<{event_id: string, relays: string[], accepted: number}>}
 */
async function publish(event) {
    const response = await fetch(`${API_BASE_URL}/publish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ event }),
    });
    const result = await response.json().catch(() => null);
    if (!response.ok || !result?.success) {
        throw new Error(result?.error || `Publish failed (${response.status})`);
    }
    if (!result.accepted) {
        throw new Error('No relay accepted the change — check your relay list');
    }
    return result;
}

/**
 * Read a player's profile. Falls back to an empty profile rather than throwing:
 * plenty of players have never published a kind 0, which is not an error.
 *
 * @param {string} npub
 * @returns {Promise<{profile: object, found: boolean}>}
 */
export async function loadProfile(npub) {
    try {
        const response = await fetch(`${API_BASE_URL}/profile?npub=${encodeURIComponent(npub)}`);
        if (!response.ok) return { profile: {}, found: false };
        const data = await response.json();
        return { profile: data.profile || {}, found: Boolean(data.found) };
    } catch (error) {
        logger.warn('Could not load profile:', error);
        return { profile: {}, found: false };
    }
}

/**
 * Publish an edited profile.
 *
 * Merges onto the profile currently published, so fields this editor does not
 * show — and fields set by some other client — survive the edit instead of
 * being wiped. kind 0 is replaceable: whatever we publish *becomes* the profile.
 *
 * @param {string} npub
 * @param {object} edits - Any of PROFILE_FIELDS
 * @returns {Promise<object>} publish result
 */
export async function saveProfile(npub, edits) {
    const { profile: existing } = await loadProfile(npub);

    const merged = { ...existing };
    for (const field of PROFILE_FIELDS) {
        if (field in edits) {
            const value = (edits[field] ?? '').trim();
            if (value) merged[field] = value;
            else delete merged[field]; // cleared on purpose
        }
    }

    const signed = await sign({ kind: 0, content: JSON.stringify(merged) });
    const result = await publish(signed);
    logger.info(`Profile published to ${result.accepted}/${result.relays?.length ?? 0} relays`);

    // Tell the rest of the UI (the dropdown, the nav, the home tab) right away
    // rather than waiting for a reload.
    window.dispatchEvent(new CustomEvent('profile-updated', {
        detail: { npub, ...merged },
    }));
    return result;
}

/**
 * Read the player's relay list, roles, and whether they pinned relays.
 * @returns {Promise<{relays: Array, pinned: boolean, connected: string[]}>}
 */
export async function loadRelays() {
    const response = await fetch(`${API_BASE_URL}/relays`);
    if (!response.ok) {
        throw new Error(response.status === 401 ? 'Sign in to manage relays' : `Could not load relays (${response.status})`);
    }
    const data = await response.json();
    return {
        relays: data.relays || [],
        pinned: Boolean(data.pinned),
        connected: data.connected || [],
    };
}

/**
 * Publish a relay list as NIP-65 (kind 10002).
 *
 * A relay the player both reads and writes gets a bare ["r", url] tag; one-way
 * relays get the "read" or "write" marker. That is the NIP-65 convention, and
 * other clients rely on it to route to you.
 *
 * @param {Array<{url: string, read: boolean, write: boolean}>} relays
 * @returns {Promise<object>} publish result
 */
export async function saveRelays(relays) {
    const usable = relays.filter((r) => r.url && (r.read || r.write));
    if (!usable.length) {
        throw new Error('Keep at least one relay you can read or write');
    }

    const tags = usable.map(({ url, read, write }) =>
        read && write ? ['r', url] : ['r', url, read ? 'read' : 'write']);

    const signed = await sign({ kind: 10002, content: '', tags });
    const result = await publish(signed);
    logger.info(`Relay list published to ${result.accepted}/${result.relays?.length ?? 0} relays`);
    return result;
}

/**
 * Turn the local "use only my relays" override on or off.
 *
 * This is not published: it tells *this game* to skip outbox routing and use
 * exactly the player's own list. Useful for keeping this game's events on a
 * chosen set of relays.
 *
 * @param {boolean} enabled
 * @returns {Promise<{relays: Array, pinned: boolean, connected: string[]}>} the new state
 */
export async function setPinnedRelays(enabled) {
    const response = await fetch(`${API_BASE_URL}/relays/pinned`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
    });
    if (!response.ok) {
        const text = await response.text().catch(() => '');
        throw new Error(text.trim() || `Could not change relay mode (${response.status})`);
    }
    const data = await response.json();
    return { relays: data.relays || [], pinned: Boolean(data.pinned), connected: data.connected || [] };
}
