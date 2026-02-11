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

    async loginWithExtension() {
        try {
            if (!window.nostr) {
                throw new Error('No Nostr extension found. Please install Alby or nos2x.');
            }

            this.emit('authenticationStarted', 'extension');

            // Add timeout for extension prompt (30 seconds)
            const publicKeyPromise = window.nostr.getPublicKey();
            const timeoutPromise = new Promise((_, reject) => {
                setTimeout(() => reject(new Error('Extension request timed out. Please try again.')), 30000);
            });

            const publicKey = await Promise.race([publicKeyPromise, timeoutPromise]);
            if (!publicKey) {
                throw new Error('Failed to get public key from extension');
            }

            const loginRequest = {
                public_key: publicKey,
                signing_method: 'browser_extension',
                mode: 'write'
            };

            return await this.performLogin(loginRequest);
        } catch (error) {
            this.emit('authenticationFailed', { method: 'extension', error: error.message });
            throw error;
        }
    }

    async loginWithPrivateKey(privateKey) {
        try {
            if (!privateKey) {
                throw new Error('Private key is required');
            }

            this.emit('authenticationStarted', 'private_key');

            const loginRequest = {
                private_key: privateKey,
                signing_method: 'encrypted_key',
                mode: 'write'
            };

            return await this.performLogin(loginRequest);
        } catch (error) {
            this.emit('authenticationFailed', { method: 'private_key', error: error.message });
            throw error;
        }
    }

    async loginWithAmber() {
        try {
            this.emit('authenticationStarted', 'amber');

            // Set up callback listener BEFORE opening Amber
            this.setupAmberCallbackListener();

            // Use proper NIP-55 nostrsigner URL format
            const amberUrl = this.createAmberLoginURL();

            logger.debug('Opening Amber with URL:', amberUrl);

            // Try multiple approaches for opening the nostrsigner protocol (mobile-first)
            let protocolOpened = false;

            // Method 1: Create anchor element and click it (most reliable on mobile)
            try {
                const anchor = document.createElement('a');
                anchor.href = amberUrl;
                anchor.target = '_blank';
                anchor.style.display = 'none';
                document.body.appendChild(anchor);

                anchor.click();
                protocolOpened = true;

                setTimeout(() => {
                    if (document.body.contains(anchor)) {
                        document.body.removeChild(anchor);
                    }
                }, 100);

                logger.debug('Amber protocol opened via anchor click');
            } catch (anchorError) {
                logger.warn('Anchor method failed:', anchorError);
            }

            // Method 2: Fallback to window.location.href
            if (!protocolOpened) {
                try {
                    window.location.href = amberUrl;
                    protocolOpened = true;
                    logger.debug('Amber protocol opened via window.location.href');
                } catch (locationError) {
                    logger.warn('Window location method failed:', locationError);
                }
            }

            if (!protocolOpened) {
                throw new Error('Unable to open Amber protocol');
            }

            return new Promise((resolve, reject) => {
                // Store resolve/reject for callback handler
                window._amberLoginPromise = { resolve, reject };

                // Set timeout in case user doesn't complete the flow
                setTimeout(() => {
                    if (window._amberLoginPromise) {
                        window._amberLoginPromise = null;
                        reject(new Error('Amber connection timed out. Make sure Amber is installed and try again.'));
                    }
                }, 60000); // 60 seconds timeout
            });
        } catch (error) {
            this.emit('authenticationFailed', { method: 'amber', error: error.message });
            throw error;
        }
    }

    setupAmberCallbackListener() {
        const handleVisibilityChange = () => {
            if (!document.hidden) {
                setTimeout(() => this.checkForAmberCallback(), 500);
            }
        };

        const handleFocus = () => {
            setTimeout(() => this.checkForAmberCallback(), 500);
        };

        document.addEventListener('visibilitychange', handleVisibilityChange);
        window.addEventListener('focus', handleFocus);

        setTimeout(() => this.checkForAmberCallback(), 1000);

        // Clean up listeners after timeout
        setTimeout(() => {
            document.removeEventListener('visibilitychange', handleVisibilityChange);
            window.removeEventListener('focus', handleFocus);
        }, 65000);
    }

    async checkForAmberCallback() {
        const currentUrl = new URL(window.location.href);

        // Check if this is the amber-callback page or has event parameter
        if (currentUrl.pathname === '/api/auth/amber-callback' || currentUrl.searchParams.has('event')) {
            try {
                const isActive = await this.checkExistingSession();
                if (isActive && window._amberLoginPromise) {
                    this.currentStatus = SessionStatus.ACTIVE;
                    this.startSessionMonitoring();
                    this.emit('authenticationSuccess', {
                        method: 'amber',
                        npub: this.sessionData.npub,
                        pubkey: this.sessionData.publicKey,
                        isNewAccount: false
                    });
                    window._amberLoginPromise.resolve(this.sessionData);
                    window._amberLoginPromise = null;
                }
            } catch (error) {
                if (window._amberLoginPromise) {
                    window._amberLoginPromise.reject(error);
                    window._amberLoginPromise = null;
                }
            }
        }

        // Check localStorage for callback result
        const amberResult = localStorage.getItem('amber_callback_result');
        if (amberResult && window._amberLoginPromise) {
            try {
                localStorage.removeItem('amber_callback_result');
                const data = JSON.parse(amberResult);

                if (data.error) {
                    throw new Error(data.error);
                }

                const isActive = await this.checkExistingSession();
                if (isActive) {
                    this.currentStatus = SessionStatus.ACTIVE;
                    this.startSessionMonitoring();
                    this.emit('authenticationSuccess', {
                        method: 'amber',
                        npub: this.sessionData.npub,
                        pubkey: this.sessionData.publicKey,
                        isNewAccount: false
                    });
                    window._amberLoginPromise.resolve(this.sessionData);
                    window._amberLoginPromise = null;
                } else {
                    throw new Error('Amber login succeeded but session not found');
                }
            } catch (error) {
                if (window._amberLoginPromise) {
                    window._amberLoginPromise.reject(error);
                    window._amberLoginPromise = null;
                }
            }
        }
    }

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
                window.showWhitelistDenialPopup(result.error, result.form_url);
            }
            // Throw error with special marker so auth.js can handle it
            const error = new Error(result.error || 'Access denied');
            error.whitelistDenial = true;
            error.formUrl = result.form_url;
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

    async generateKeys() {
        try {
            const response = await fetch(`${API_BASE_URL}/auth/generate-keys`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                }
            });

            if (!response.ok) {
                throw new Error(`Key generation failed: ${response.status}`);
            }

            const result = await response.json();

            if (!result.success) {
                throw new Error(result.error || 'Key generation failed');
            }

            return result.key_pair;
        } catch (error) {
            logger.error('Key generation error:', error);
            throw error;
        }
    }

    createAmberLoginURL() {
        const callbackUrl = encodeURIComponent(`${window.location.origin}/api/auth/amber-callback`);
        return `intent://get_public_key?callback_url=${callbackUrl}#Intent;scheme=nostrsigner;package=com.greenart7c3.nostrsigner;end`;
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
