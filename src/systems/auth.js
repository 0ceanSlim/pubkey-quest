/**
 * Authentication Module
 *
 * Handles user authentication via Nostr (browser extension, Amber, private key),
 * session management, and related UI.
 *
 * @module systems/auth
 */

import { logger } from '../lib/logger.js';
// Assume these UI functions are globally available on the window object for now.
// A future refactor should move them into modules and import them.
const showMessage = (...args) => window.showMessage?.(...args);
const showLoadingModal = (...args) => window.showLoadingModal?.(...args);
const hideLoadingModal = () => window.hideLoadingModal?.();
const showAuthResult = (...args) => window.showAuthResult?.(...args);
const hideLoginModal = () => window.hideLoginModal?.();
const showEncryptionPasswordModal = () => window.showEncryptionPasswordModal?.();

/**
 * Initialize authentication with session manager integration.
 * This function sets up all the necessary event listeners for the session manager.
 */
export function initializeAuthentication() {
    if (!window.sessionManager) {
        logger.error('❌ SessionManager not available for authentication');
        return;
    }

    window.sessionManager.on('sessionReady', (sessionData) => {
        logger.info('✅ Session ready');
        const allowedPaths = ['/saves', '/game', '/new-game', '/settings', '/discover'];
        if (!allowedPaths.includes(window.location.pathname)) {
            logger.info('Redirecting to saves page');
            window.location.href = '/saves';
        }
    });

    window.sessionManager.on('authenticationRequired', () => {
        logger.info('🔐 Authentication required');
        if (window.location.pathname === '/game' || window.location.pathname === '/new-game') {
            showLoginInterface();
        }
    });

    window.sessionManager.on('sessionExpired', () => {
        logger.warn('⏰ Session expired, showing login interface');
        showMessage('⏰ Your session has expired. Please log in again.', 'warning');
        showLoginInterface();
    });

    window.sessionManager.on('authenticationSuccess', (data) => {
        const method = typeof data === 'string' ? data : data.method;
        const isNewAccount = typeof data === 'object' ? data.isNewAccount : false;
        logger.info(`✅ Authentication successful via ${method}${isNewAccount ? ' (new account)' : ''}`);
        showLoadingModal?.('Redirecting to saves...');

        // Hide any login interface or modals
        hideKeyLogin?.();
        hideGeneratedKeys?.();

        setTimeout(() => {
            const allowedPaths = ['/saves', '/game', '/new-game', '/settings', '/discover'];
            if (!allowedPaths.includes(window.location.pathname)) {
                logger.info(`Redirecting from ${window.location.pathname} to /saves`);
                window.location.href = '/saves';
            } else {
                logger.info(`Already on allowed path: ${window.location.pathname}`);
                hideLoadingModal?.();
                // If we're on a page that needs the user to be logged in, reload to show content
                if (window.location.pathname === '/game' || window.location.pathname === '/new-game') {
                    logger.info('Reloading page to load authenticated content');
                    window.location.reload();
                }
            }
        }, 1000);
    });

    window.sessionManager.on('authenticationFailed', ({ method, error }) => {
        logger.error(`❌ Authentication failed via ${method}:`, error);
        showMessage(`❌ Login failed via ${method}: ${error}`, 'error');
    });

    window.sessionManager.on('sessionError', (error) => {
        logger.error('❌ Session error:', error);
        showMessage('❌ Session error: ' + error.message, 'error');
        showLoginInterface();
    });

    logger.info('Authentication system listeners initialized.');
}

/**
 * Renders the main login interface in the #game-app container.
 */
function showLoginInterface() {
    const gameContainer = document.getElementById('game-app');
    if (gameContainer) {
        gameContainer.innerHTML = `
            <div class="text-center py-12">
                <h2 class="text-3xl font-bold mb-6 text-yellow-400 flex items-center justify-center gap-2">
                    <img src="/res/img/static/logo.png" alt="Pubkey Quest" class="inline-block" style="height: 1.5em; width: auto; image-rendering: pixelated;">
                    Pubkey Quest
                    <img src="/res/img/static/logo.png" alt="Pubkey Quest" class="inline-block" style="height: 1.5em; width: auto; image-rendering: pixelated;">
                </h2>
                <p class="text-gray-300 mb-8">A text-based RPG powered by Nostr</p>
                <div class="space-y-4 max-w-md mx-auto">
                    <button onclick="loginWithExtension()"
                            class="w-full bg-purple-600 hover:bg-purple-700 text-white px-6 py-3 rounded-lg font-medium transition-colors">
                        🔗 Login with Browser Extension
                    </button>
                    <button onclick="loginWithAmber()"
                            class="w-full bg-orange-600 hover:bg-orange-700 text-white px-6 py-3 rounded-lg font-medium transition-colors">
                        📱 Login with Amber
                    </button>
                    <button onclick="showKeyLogin()"
                            class="w-full bg-gray-600 hover:bg-gray-700 text-white px-6 py-3 rounded-lg font-medium transition-colors">
                        🗝️ Login with Private Key
                    </button>
                    <button onclick="generateNewKeys()"
                            class="w-full bg-green-600 hover:bg-green-700 text-white px-6 py-3 rounded-lg font-medium transition-colors">
                        ✨ Generate New Keys
                    </button>
                </div>
            </div>
            <div id="login-details" class="mt-8 hidden"></div>
        `;
    }
}

/**
 * Replaces the login interface with a loading message.
 */
function hideLoginInterface() {
    const gameContainer = document.getElementById('game-app');
    if (gameContainer) {
        gameContainer.innerHTML = `
            <div class="text-center py-12">
                <div class="spinner-border animate-spin inline-block w-8 h-8 border-4 rounded-full" role="status">
                    <span class="visually-hidden"></span>
                </div>
                <p class="text-gray-300 mt-4">🎮 Loading game...</p>
            </div>
        `;
    }
}

/**
 * Shows the private key login form.
 */
function showKeyLogin() {
    const loginDetails = document.getElementById('login-details');
    if(loginDetails) {
        loginDetails.innerHTML = `
            <div class="max-w-md mx-auto bg-gray-800 p-6 rounded-lg">
                <h3 class="text-xl font-bold mb-4 text-yellow-400">Login with Private Key</h3>
                <div class="space-y-4">
                    <div>
                        <label class="block text-sm font-medium text-gray-300 mb-2">Private Key (nsec or hex)</label>
                        <input type="password" id="auth-private-key-input"
                               class="w-full bg-gray-700 text-white px-3 py-2 rounded border border-gray-600 focus:border-yellow-400"
                               placeholder="nsec1...">
                    </div>
                    <div>
                        <label class="block text-sm font-medium text-gray-300 mb-2">Encryption Password</label>
                        <input type="password" id="auth-encryption-password-input"
                               class="w-full bg-gray-700 text-white px-3 py-2 rounded border border-gray-600 focus:border-yellow-400"
                               placeholder="Password to encrypt your key">
                    </div>
                    <div class="flex space-x-3">
                        <button onclick="loginWithPrivateKey()"
                                class="flex-1 bg-yellow-600 hover:bg-yellow-700 text-gray-900 px-4 py-2 rounded font-medium">
                            Login
                        </button>
                        <button onclick="hideKeyLogin()"
                                class="flex-1 bg-gray-600 hover:bg-gray-700 text-white px-4 py-2 rounded">
                            Cancel
                        </button>
                    </div>
                </div>
                <div class="mt-4 text-sm text-gray-400">
                    <p>⚠️ Your private key will be encrypted with your password and stored securely for this session only.</p>
                </div>
            </div>
        `;
        loginDetails.classList.remove('hidden');
    }
}

/**
 * Hides the private key login form.
 */
function hideKeyLogin() {
    const loginDetails = document.getElementById('login-details');
    if(loginDetails) {
        loginDetails.classList.add('hidden');
        loginDetails.innerHTML = '';
    }
}

/**
 * Logs in with a Nostr browser extension (e.g., Alby, nos2x).
 */
async function loginWithExtension() {
    try {
        showLoadingModal?.('Requesting access from extension...');
        showAuthResult?.('loading', 'Requesting access from extension...');

        if (!window.nostr) {
            throw new Error('Nostr extension not found');
        }
        const publicKey = await window.nostr.getPublicKey();
        if (!publicKey || publicKey.length !== 64) {
            throw new Error('Invalid public key received from extension');
        }

        showLoadingModal?.('Creating session...');
        showAuthResult?.('loading', 'Creating session with extension signing...');

        const response = await fetch('/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ public_key: publicKey, signing_method: 'browser_extension', mode: 'write' })
        });

        console.log('🔍 Login response status:', response.status);
        const result = await response.json().catch(() => null);
        console.log('🔍 Login response data:', result);

        // Check for whitelist denial
        if (result?.whitelist_denial) {
            console.log('🚫 Whitelist denial detected, showing popup');
            hideLoadingModal?.();
            showWhitelistDenialPopup(result.error, result.form_url);
            return;
        }

        if (!response.ok) {
            throw new Error(result?.error || result?.message || `HTTP ${response.status}`);
        }

        if (!result.success) {
            throw new Error(result.message || 'Login failed');
        }

        hideLoadingModal?.();
        showAuthResult?.('success', 'Connected via browser extension!');
        window.nostrExtensionConnected = true;

        setTimeout(() => {
            hideLoginModal?.();
            window.location.href = '/saves';
        }, 1000);
    } catch (error) {
        logger.error('Extension login error:', error);
        hideLoadingModal?.();
        showAuthResult?.('error', `Extension error: ${error.message}`);
    }
}

let amberCallbackReceived = false;

/**
 * Logs in with the Amber mobile app using NIP-55.
 */
function loginWithAmber() {
    if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
        showAuthResult?.('error', '⚠️ Amber login requires a local IP address (like 192.168.x.x), not localhost. Access the site from your computer\'s IP address on your local network.');
        return;
    }

    showLoadingModal?.('Opening Amber app...');
    showAuthResult?.('loading', 'Opening Amber app...');

    setupAmberCallbackListener();

    const callbackUrl = `${window.location.origin}/api/auth/amber-callback?event=`;
    const amberUrl = `nostrsigner:?compressionType=none&returnType=signature&type=get_public_key&callbackUrl=${encodeURIComponent(callbackUrl)}&appName=${encodeURIComponent('Pubkey Quest')}`;

    try {
        const anchor = document.createElement('a');
        anchor.href = amberUrl;
        document.body.appendChild(anchor);
        anchor.click();
        document.body.removeChild(anchor);
        showAuthResult?.('loading', 'Opening Amber app... If nothing happens, make sure Amber is installed and try again.');
    } catch (error) {
        logger.error('Error opening Amber:', error);
        hideLoadingModal?.();
        showAuthResult?.('error', 'Failed to open Amber app. Please ensure Amber is installed and your browser supports the nostrsigner protocol.');
    }

    setTimeout(() => {
        if (!amberCallbackReceived) {
            hideLoadingModal?.();
            showAuthResult?.('error', 'Amber connection timed out. Make sure Amber is installed and try again.');
        }
    }, 60000);
}

/**
 * Sets up listeners to detect when the user returns from Amber.
 */
function setupAmberCallbackListener() {
    const handleVisibilityChange = () => {
        if (!document.hidden && !amberCallbackReceived) setTimeout(checkForAmberCallback, 500);
    };
    const handleFocus = () => {
        if (!amberCallbackReceived) setTimeout(checkForAmberCallback, 500);
    };

    // Listen for postMessage from Amber callback page
    const handleMessage = (event) => {
        if (event.origin !== window.location.origin) return;
        if (event.data?.type === 'whitelist_denial') {
            amberCallbackReceived = true;
            hideLoadingModal?.();
            showWhitelistDenialPopup(event.data.error, event.data.formUrl);
        } else if (event.data?.type === 'amber_success') {
            amberCallbackReceived = true;
            handleAmberCallbackData({ success: true });
        } else if (event.data?.type === 'amber_error') {
            amberCallbackReceived = true;
            hideLoadingModal?.();
            showAuthResult?.('error', `Amber login failed: ${event.data.error}`);
        }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    window.addEventListener('focus', handleFocus);
    window.addEventListener('message', handleMessage);
    setTimeout(checkForAmberCallback, 1000); // Also check immediately

    setTimeout(() => { // Cleanup
        document.removeEventListener('visibilitychange', handleVisibilityChange);
        window.removeEventListener('focus', handleFocus);
        window.removeEventListener('message', handleMessage);
    }, 65000);
}

/**
 * Checks localStorage for the result from the Amber callback page.
 */
function checkForAmberCallback() {
    const amberResult = localStorage.getItem('amber_callback_result');
    if (amberResult) {
        localStorage.removeItem('amber_callback_result');
        try {
            const data = JSON.parse(amberResult);
            amberCallbackReceived = true;

            // Check if it's a whitelist denial stored in localStorage
            if (data.type === 'whitelist_denial' || data.formUrl) {
                hideLoadingModal?.();
                showWhitelistDenialPopup(data.error, data.formUrl);
                return;
            }

            handleAmberCallbackData(data);
        } catch (error) {
            logger.error('Failed to parse stored Amber result:', error);
        }
    }
}

/**
 * Handles the data returned from the Amber callback.
 */
function handleAmberCallbackData(data) {
    try {
        if (data.error) {
            // Check if this is a whitelist denial (from postMessage in Amber callback page)
            if (data.type === 'whitelist_denial' || data.formUrl) {
                hideLoadingModal?.();
                showWhitelistDenialPopup(data.error, data.formUrl);
                return;
            }
            throw new Error(data.error);
        }

        hideLoadingModal?.();
        showAuthResult?.('success', 'Connected via Amber!');
        window.amberConnected = true;

        setTimeout(() => {
            hideLoginModal?.();
            window.location.href = '/saves';
        }, 1000);
    } catch (error) {
        logger.error('Error processing Amber callback data:', error);
        hideLoadingModal?.();
        showAuthResult?.('error', `Amber login failed: ${error.message}`);
    }
}

/**
 * Logs in with a private key using the session manager.
 */
async function loginWithPrivateKey() {
    if (!window.sessionManager) {
        showMessage('❌ Session manager not available', 'error');
        return;
    }
    const privateKey = document.getElementById('auth-private-key-input')?.value?.trim();
    const password = document.getElementById('auth-encryption-password-input')?.value?.trim();
    if (!privateKey || !password) {
        showMessage('❌ Please provide both private key and password', 'error');
        return;
    }
    try {
        showMessage('🗝️ Logging in with private key...', 'info');
        console.log('🔑 About to call sessionManager.loginWithPrivateKey...');
        const result = await window.sessionManager.loginWithPrivateKey(privateKey, { password });
        console.log('🔑 loginWithPrivateKey returned:', result);
        // Login successful - redirect to saves
        console.log('🔑 Redirecting to /saves...');
        window.location.href = '/saves';
    } catch (error) {
        console.error('🔑 Private key login caught error:', error);
        logger.error('Private key login error:', error);
        // Don't show error message if it's a whitelist denial (popup already shown)
        if (!error.whitelistDenial) {
            showMessage('❌ Private key login failed: ' + error.message, 'error');
        }
    }
}

/**
 * Generates new Nostr keys using the session manager.
 */
async function generateNewKeys() {
    if (!window.sessionManager) {
        showMessage('❌ Session manager not available', 'error');
        return;
    }
    try {
        const keyPair = await window.sessionManager.generateKeys();
        if (keyPair) {
            showGeneratedKeys(keyPair);
        } else {
            showMessage('❌ Failed to generate keys', 'error');
        }
    } catch (error) {
        logger.error('Key generation error:', error);
        showMessage('❌ Key generation failed: ' + error.message, 'error');
    }
}

/**
 * Displays the newly generated keys to the user in a modal.
 * @param {Object} keyPair - The generated key pair ({ npub, nsec }).
 */
function showGeneratedKeys(keyPair) {
    window.generatedKeyPair = keyPair;
    document.getElementById('gen-npub').textContent = keyPair.npub;
    document.getElementById('gen-nsec').textContent = keyPair.nsec;
    const modal = document.getElementById('generated-keys-modal');
    if (modal) modal.classList.remove('hidden');
}

/**
 * Hides the generated keys modal.
 */
function hideGeneratedKeys() {
    const modal = document.getElementById('generated-keys-modal');
    if (modal) modal.classList.add('hidden');
}

/**
 * Proceeds to use the newly generated keys for login.
 */
async function useGeneratedKeys() {
    const privateKey = window.generatedKeyPair?.nsec;
    if (!privateKey) {
        showMessage('❌ No generated keys available to use.', 'error');
        return;
    }
    window.generatedPrivateKey = privateKey;
    hideGeneratedKeys();
    showEncryptionPasswordModal?.();
}

/**
 * Logs the user out using the session manager.
 */
async function logout() {
    if (!window.sessionManager) {
        showMessage('❌ Session manager not available', 'error');
        return;
    }
    try {
        showMessage('🚪 Logging out...', 'info');
        await window.sessionManager.logout();
        showMessage('✅ Successfully logged out', 'success');
        setTimeout(() => {
            showLoginInterface();
        }, 1000);
    } catch (error) {
        logger.error('Logout error:', error);
        showMessage('❌ Logout failed: ' + error.message, 'error');
    }
}

/**
 * Shows a popup modal for whitelist denial with request access button.
 * @param {string} errorMessage - The error message to display
 * @param {string} formUrl - URL to the access request form
 */
function showWhitelistDenialPopup(errorMessage, formUrl) {
    // Remove existing popup if any
    const existingPopup = document.getElementById('whitelist-denial-popup');
    if (existingPopup) {
        existingPopup.remove();
    }

    // Create popup element
    const popup = document.createElement('div');
    popup.id = 'whitelist-denial-popup';
    popup.className = 'fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50';
    popup.innerHTML = `
        <div class="bg-gray-900 border-4 border-red-600 rounded-lg p-8 max-w-md mx-4 text-center">
            <div class="text-6xl mb-4">🚫</div>
            <h2 class="text-2xl font-bold text-red-500 mb-4">Access Denied</h2>
            <p class="text-gray-300 mb-6">${errorMessage || 'Your public key is not whitelisted for this test server.'}</p>
            ${formUrl ? `
                <a href="${formUrl}" target="_blank"
                   class="block w-full bg-green-600 hover:bg-green-700 text-white px-6 py-3 rounded-lg font-medium transition-colors mb-3">
                    📝 Request Access
                </a>
            ` : ''}
            <button onclick="closeWhitelistDenialPopup()"
                    class="w-full bg-gray-600 hover:bg-gray-700 text-white px-6 py-3 rounded-lg font-medium transition-colors">
                Close
            </button>
        </div>
    `;

    document.body.appendChild(popup);
    logger.info('Whitelist denial popup shown');
}

/**
 * Closes the whitelist denial popup.
 */
function closeWhitelistDenialPopup() {
    const popup = document.getElementById('whitelist-denial-popup');
    if (popup) {
        popup.remove();
    }
}


// For compatibility with old HTML that uses onclick, attach functions to window
if (typeof window !== 'undefined') {
    window.showLoginInterface = showLoginInterface;
    window.hideLoginInterface = hideLoginInterface;
    window.showKeyLogin = showKeyLogin;
    window.hideKeyLogin = hideKeyLogin;
    window.loginWithExtension = loginWithExtension;
    window.loginWithAmber = loginWithAmber;
    window.loginWithPrivateKey = loginWithPrivateKey;
    window.generateNewKeys = generateNewKeys;
    window.useGeneratedKeys = useGeneratedKeys;
    window.logout = logout;
    window.showWhitelistDenialPopup = showWhitelistDenialPopup;
    window.closeWhitelistDenialPopup = closeWhitelistDenialPopup;
}

logger.debug('🔐 Authentication system module loaded.');
