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
import { restoreSignerFromSession } from '../systems/millLogin.js';

// Session status enum
export const SessionStatus = {
    INITIALIZING: 'initializing',
    ACTIVE: 'active',
    EXPIRED: 'expired',
    ERROR: 'error',
    UNAUTHENTICATED: 'unauthenticated'
};

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
                // Rebuild the client-side signer (window.nostr) from MILL's
                // persisted state so signing survives a page reload.
                restoreSignerFromSession();
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

    handleSessionExpiry() {
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

    async performLogin(loginRequest) {
        const response = await fetch(`${API_BASE_URL}/auth/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(loginRequest)
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

// Export singleton instance
export const sessionManager = new SessionManager();

// Make available globally for compatibility with templates
if (typeof window !== 'undefined') {
    window.sessionManager = sessionManager;
}

logger.debug('SessionManager loaded and initialized');
