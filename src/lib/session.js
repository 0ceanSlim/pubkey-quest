/**
 * Session Manager for Pubkey Quest
 *
 * Handles session initialization, persistence, and recovery.
 * Manages authentication via browser extensions, private keys, or Amber (mobile).
 *
 * @module lib/session
 */

import { logger } from './logger.js';
import { API_BASE_URL } from '../config/constants.js';
import { generateSecretKey, getPublicKey, nip19 } from 'nostr-tools';
// Registers window.openMillLogin and exposes the signer-restore helper. All
// interactive login now goes through MILL (see systems/millLogin.js).
import { restoreSignerFromSession, clearMillSigner, getActiveSigner } from '../systems/millLogin.js';

// Session status enum
export const SessionStatus = {
    INITIALIZING: 'initializing',
    ACTIVE: 'active',
    EXPIRED: 'expired',
    ERROR: 'error',
    UNAUTHENTICATED: 'unauthenticated'
};

// Events that change whether somebody is logged in. Each also raises the
// coarse `auth-changed` signal the templates listen for.
const AUTH_STATE_EVENTS = new Set([
    'sessionReady',
    'sessionRestored',
    'authenticationSuccess',
    'authenticationRequired',
    'sessionExpired',
    'loggedOut',
]);

class SessionManager {
    constructor() {
        this.sessionData = null;
        this.isInitialized = false;
        this.initPromise = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 3;
        this.sessionCheckInterval = null;
        this.eventListeners = new Map();
        this.currentStatus = SessionStatus.INITIALIZING;

        // Initialize session on construction
        this.init();
    }

    // ========================================================================
    // INITIALIZATION
    // ========================================================================

    async init() {
        if (this.initPromise) {
            return this.initPromise;
        }

        this.initPromise = this._performInit();
        return this.initPromise;
    }

    async _performInit() {
        logger.info('Initializing Pubkey Quest session...');

        try {
            await this.checkExistingSession();

            if (this.currentStatus === SessionStatus.ACTIVE) {
                logger.info('Found active session');
                // Rebuild the client-side signer from MILL's
                // persisted state so signing survives a page reload.
                restoreSignerFromSession(this.sessionData);
                this.startSessionMonitoring();
                this.isInitialized = true;
                this.emit('sessionReady', this.sessionData);
                return true;
            } else {
                logger.info('No active session, authentication required');
                this.currentStatus = SessionStatus.UNAUTHENTICATED;
                this.emit('authenticationRequired');
                return false;
            }
        } catch (error) {
            logger.error('Initialization failed:', error);
            this.currentStatus = SessionStatus.ERROR;
            this.emit('sessionError', error);
            return false;
        }
    }

    // ========================================================================
    // SESSION VALIDATION
    // ========================================================================

    async checkExistingSession() {
        try {
            const response = await fetch(`${API_BASE_URL}/auth/session`, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json'
                }
            });

            if (!response.ok) {
                throw new Error(`Session check failed: ${response.status}`);
            }

            const result = await response.json();

            if (result.success && result.is_active && result.session) {
                this.sessionData = {
                    publicKey: result.session.public_key,
                    npub: result.npub,
                    signingMethod: result.session.signing_method,
                    mode: result.session.mode,
                    isActive: true,
                    lastCheck: Date.now()
                };
                this.currentStatus = SessionStatus.ACTIVE;
                return true;
            } else {
                this.sessionData = null;
                this.currentStatus = SessionStatus.UNAUTHENTICATED;
                return false;
            }
        } catch (error) {
            logger.error('Session check error:', error);
            this.currentStatus = SessionStatus.ERROR;
            throw error;
        }
    }

    async validateSession() {
        if (!this.sessionData) return false;

        try {
            const response = await fetch(`${API_BASE_URL}/auth/session`);
            if (!response.ok) return false;

            const result = await response.json();
            return result.success && result.is_active;
        } catch (error) {
            logger.error('Session validation error:', error);
            return false;
        }
    }

    // ========================================================================
    // SESSION MONITORING
    // ========================================================================

    startSessionMonitoring() {
        if (this.sessionCheckInterval) {
            clearInterval(this.sessionCheckInterval);
        }

        // Check session every 30 seconds
        this.sessionCheckInterval = setInterval(async () => {
            try {
                const isValid = await this.validateSession();
                if (!isValid) {
                    logger.warn('Session expired or invalid');
                    this.handleSessionExpiry();
                }
            } catch (error) {
                logger.error('Session validation error:', error);
            }
        }, 30000);
    }

    async handleSessionExpiry() {
        // Usually the server just restarted and lost its in-memory sessions;
        // the signer is still here, so quietly prove the key again first.
        if (await this.reauthenticate()) return;

        this.currentStatus = SessionStatus.EXPIRED;
        this.sessionData = null;

        if (this.sessionCheckInterval) {
            clearInterval(this.sessionCheckInterval);
            this.sessionCheckInterval = null;
        }

        this.emit('sessionExpired');

        // Attempt to re-authenticate if possible
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            logger.info(`Attempting reconnection (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
            setTimeout(() => this.attemptReconnection(), 2000);
        } else {
            logger.warn('Max reconnection attempts reached');
            this.emit('authenticationRequired');
        }
    }

    async attemptReconnection() {
        try {
            const restored = await this.restoreSessionFromStorage();
            if (restored) {
                logger.info('Session restored from storage');
                this.reconnectAttempts = 0;
                this.startSessionMonitoring();
                this.emit('sessionRestored', this.sessionData);
                return true;
            }
        } catch (error) {
            logger.error('Reconnection failed:', error);
        }

        return false;
    }

    async restoreSessionFromStorage() {
        try {
            const storedSession = localStorage.getItem('pubkey_quest_session_meta');
            if (!storedSession) return false;

            const sessionMeta = JSON.parse(storedSession);

            // Validate the stored session is recent (within 1 hour)
            if (Date.now() - sessionMeta.timestamp > 3600000) {
                localStorage.removeItem('pubkey_quest_session_meta');
                return false;
            }

            const isValid = await this.checkExistingSession();
            if (isValid && this.sessionData.publicKey === sessionMeta.publicKey) {
                this.currentStatus = SessionStatus.ACTIVE;
                return true;
            }

            return false;
        } catch (error) {
            logger.error('Session restore error:', error);
            return false;
        }
    }

    // ========================================================================
    // AUTHENTICATION METHODS
    // ========================================================================

    /**
     * Prove we hold the key: fetch a single-use challenge from the server and
     * sign it as a NIP-98 HTTP-auth event (kind 27235) for POST /api/auth/login,
     * with the player's MILL signer (never window.nostr — an extension there
     * may hold a different key than the one logging in). The server refuses a
     * login without this, so nobody can claim someone else's pubkey.
     * @returns {Promise<object>} the signed proof event
     */
    async signLoginProof() {
        // After a page reload the signer is rebuilt asynchronously (and NIP-07
        // extensions inject late) — give it a few seconds to appear.
        for (const delay of [0, 250, 500, 1000, 2000]) {
            if (getActiveSigner()?.signEvent) break;
            await new Promise((r) => setTimeout(r, delay));
        }
        const signer = getActiveSigner();
        if (!signer?.signEvent) {
            throw new Error('No signer available to prove your key — log in again');
        }
        const resp = await fetch(`${API_BASE_URL}/auth/challenge`);
        const ch = await resp.json().catch(() => null);
        if (!resp.ok || !ch?.success || !ch.challenge) {
            throw new Error(ch?.error || `Could not get a login challenge (${resp.status})`);
        }
        return signer.signEvent({
            kind: ch.kind ?? 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ['u', `${window.location.origin}${API_BASE_URL}/auth/login`],
                ['method', 'POST'],
                ['challenge', ch.challenge],
            ],
            content: '',
        });
    }

    /**
     * @param {object} loginRequest  { public_key, signing_method, mode }
     * @param {{ silent?: boolean }} [opts]  silent: a background re-login — skip
     *   authenticationSuccess (whose listeners redirect or reload the page).
     */
    async performLogin(loginRequest, { silent = false } = {}) {
        const proof = await this.signLoginProof();
        const response = await fetch(`${API_BASE_URL}/auth/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ ...loginRequest, proof })
        });

        const result = await response.json().catch(() => null);

        // Check for whitelist denial before checking response.ok
        if (result?.whitelist_denial) {
            logger.warn('Whitelist denial during session manager login');
            // Show the whitelist popup if the function exists
            if (window.showWhitelistDenialPopup) {
                window.showWhitelistDenialPopup(result.error, result.npub);
            }
            // Throw error with special marker so auth.js can handle it
            const error = new Error(result.error || 'Access denied');
            error.whitelistDenial = true;
            throw error;
        }

        if (!response.ok) {
            throw new Error(result?.error || `Login failed: ${response.status}`);
        }

        if (!result.success) {
            throw new Error(result.error || result.message || 'Login failed');
        }

        logger.debug('Login response:', result);

        // Store session data - use top-level fields from backend response
        // (result.session may have different field names from grain library)
        this.sessionData = {
            publicKey: result.public_key || result.session?.public_key || result.session?.PublicKey,
            npub: result.npub,
            signingMethod: loginRequest.signing_method,
            mode: loginRequest.mode || 'write',
            isActive: true,
            lastCheck: Date.now()
        };

        this.currentStatus = SessionStatus.ACTIVE;
        this.reconnectAttempts = 0;

        this.storeSessionMetadata();
        this.startSessionMonitoring();

        if (silent) {
            this.emit('sessionRestored', this.sessionData);
            return this.sessionData;
        }

        // Check if this is a new account
        const isNewAccount = window._isNewAccount || false;
        if (isNewAccount) {
            window._isNewAccount = false;
        }

        this.emit('authenticationSuccess', {
            method: loginRequest.signing_method,
            npub: this.sessionData.npub,
            pubkey: this.sessionData.publicKey,
            isNewAccount: isNewAccount
        });

        return this.sessionData;
    }

    /**
     * The server forgot our login (it restarted, or the session aged out) but
     * this page still has its signer. Re-prove the key and recreate the session
     * without bothering the player. Concurrent callers share one attempt; after
     * a failure we stand down for a while instead of hammering the signer.
     * @returns {Promise<boolean>} true if the session is back
     */
    reauthenticate() {
        if (this._reauthPromise) return this._reauthPromise;
        if (Date.now() - (this._reauthFailedAt ?? 0) < REAUTH_COOLDOWN_MS) {
            return Promise.resolve(false);
        }
        const meta = this.sessionData ?? this._storedSessionMeta();
        if (!meta?.publicKey || !meta?.signingMethod || meta.signingMethod === 'none') {
            return Promise.resolve(false);
        }
        this._reauthPromise = this.performLogin({
            public_key: meta.publicKey,
            signing_method: meta.signingMethod,
            mode: 'write',
        }, { silent: true }).then(() => {
            logger.info('Session re-established by re-proving the key');
            return true;
        }).catch((err) => {
            // The session monitor surfaces the expiry to the player.
            logger.warn('Re-authentication failed:', err?.message || err);
            this._reauthFailedAt = Date.now();
            return false;
        }).finally(() => {
            this._reauthPromise = null;
        });
        return this._reauthPromise;
    }

    _storedSessionMeta() {
        try {
            return JSON.parse(localStorage.getItem('pubkey_quest_session_meta') || 'null');
        } catch (_) {
            return null;
        }
    }

    storeSessionMetadata() {
        try {
            const sessionMeta = {
                publicKey: this.sessionData.publicKey,
                npub: this.sessionData.npub,
                signingMethod: this.sessionData.signingMethod,
                timestamp: Date.now()
            };
            localStorage.setItem('pubkey_quest_session_meta', JSON.stringify(sessionMeta));
        } catch (error) {
            logger.warn('Failed to store session metadata:', error);
        }
    }

    async logout() {
        try {
            await fetch(`${API_BASE_URL}/auth/logout`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                }
            });
        } catch (error) {
            logger.warn('Logout request failed:', error);
        }

        this.sessionData = null;
        this.currentStatus = SessionStatus.UNAUTHENTICATED;
        this.reconnectAttempts = 0;

        if (this.sessionCheckInterval) {
            clearInterval(this.sessionCheckInterval);
            this.sessionCheckInterval = null;
        }

        localStorage.removeItem('pubkey_quest_session_meta');
        clearMillSigner();

        this.emit('loggedOut');
    }

    /**
     * Generate a fresh Nostr keypair client-side. Used by the "discover
     * character" preview flow; interactive account creation goes through MILL's
     * new-identity flow instead. Returns { npub, nsec } to match the old
     * server-generated shape.
     */
    async generateKeys() {
        try {
            const sk = generateSecretKey();
            const pk = getPublicKey(sk);
            return {
                npub: nip19.npubEncode(pk),
                nsec: nip19.nsecEncode(sk),
                pubkey: pk,
            };
        } catch (error) {
            logger.error('Key generation error:', error);
            throw error;
        }
    }

    // ========================================================================
    // EVENT SYSTEM
    // ========================================================================

    on(eventName, callback) {
        if (!this.eventListeners.has(eventName)) {
            this.eventListeners.set(eventName, []);
        }
        this.eventListeners.get(eventName).push(callback);
    }

    off(eventName, callback) {
        if (this.eventListeners.has(eventName)) {
            const callbacks = this.eventListeners.get(eventName);
            const index = callbacks.indexOf(callback);
            if (index > -1) {
                callbacks.splice(index, 1);
            }
        }
    }

    emit(eventName, data) {
        logger.debug(`SessionManager event: ${eventName}`, data);
        if (this.eventListeners.has(eventName)) {
            this.eventListeners.get(eventName).forEach(callback => {
                try {
                    callback(data);
                } catch (error) {
                    logger.error(`Error in ${eventName} event handler:`, error);
                }
            });
        }
        this.mirrorToWindow(eventName, data);
    }

    /**
     * Re-dispatch a session event on `window`.
     *
     * Half the codebase subscribes with `sessionManager.on(...)` and half with
     * `window.addEventListener(...)` — the profile manager, the nav, the
     * settings page and the home tab all use the latter. emit() only ever
     * walked its own listener map, so those never fired: the profile was never
     * fetched and the dropdown sat on "Loading…" forever. Nothing dispatched
     * `auth-changed` either, leaving four more listeners dead.
     *
     * The identity fields go out under both spellings because the two sides
     * disagree: sessionData carries `publicKey`, while listeners read `pubkey`.
     */
    mirrorToWindow(eventName, data) {
        if (typeof window === 'undefined') return;

        const detail = data && typeof data === 'object' && !Array.isArray(data)
            ? { ...data, pubkey: data.pubkey ?? data.publicKey, publicKey: data.publicKey ?? data.pubkey }
            : data;

        window.dispatchEvent(new CustomEvent(eventName, { detail }));

        if (AUTH_STATE_EVENTS.has(eventName)) {
            window.dispatchEvent(new CustomEvent('auth-changed', {
                detail: {
                    isAuthenticated: this.currentStatus === SessionStatus.ACTIVE,
                    ...(detail && typeof detail === 'object' ? detail : {}),
                },
            }));
        }
    }

    // ========================================================================
    // GETTERS
    // ========================================================================

    getSession() {
        return this.sessionData;
    }

    getStatus() {
        return this.currentStatus;
    }

    isAuthenticated() {
        return this.currentStatus === SessionStatus.ACTIVE && this.sessionData;
    }

    getPublicKey() {
        return this.sessionData?.publicKey;
    }

    getNpub() {
        return this.sessionData?.npub;
    }

    getSigningMethod() {
        return this.sessionData?.signingMethod;
    }
}

// After a failed silent re-login, wait this long before trying again.
const REAUTH_COOLDOWN_MS = 30000;


// Export singleton instance
export const sessionManager = new SessionManager();

/**
 * Player-state API routes are tied to the login session (server: auth/identity.go).
 * When one answers 401 with auth_required — typically because the server
 * restarted and lost its in-memory sessions — re-prove the key once and replay
 * the request, so a restart doesn't break the tick loop or a fight in progress.
 * Wrapping fetch here covers every caller (many modules call fetch directly).
 */
function installReauthFetch() {
    const baseFetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
        const url = typeof input === 'string' ? input : (input?.url ?? String(input));
        const isGuarded = url.includes(`${API_BASE_URL}/`) && !url.includes(`${API_BASE_URL}/auth/`);
        // A Request's body can only be read once — keep a copy for the replay.
        const replay = isGuarded && input instanceof Request ? input.clone() : input;

        const response = await baseFetch(input, init);
        if (!isGuarded || response.status !== 401) return response;

        const body = await response.clone().json().catch(() => null);
        if (!body?.auth_required) return response;
        if (!(await sessionManager.reauthenticate())) return response;
        return baseFetch(replay, init);
    };
}

// Make available globally for compatibility with templates
if (typeof window !== 'undefined') {
    window.sessionManager = sessionManager;
    installReauthFetch();
}

logger.debug('SessionManager loaded and initialized');
