/**
 * Authentication Module
 *
 * Wires the session manager's lifecycle events to the UI and renders the
 * logged-out "Connect" screen. All signing-method selection and signing is
 * handled by MILL (see systems/millLogin.js) — this module only opens it and
 * reacts to the resulting session events.
 *
 * @module systems/auth
 */

import { logger } from '../lib/logger.js';
// openMillLogin is registered on window by systems/millLogin.js (imported via
// lib/session.js). Reference it lazily so load order doesn't matter.
const openLogin = () => window.openMillLogin?.();
// UI helpers assumed globally available (defined in nav-play.html template).
const showMessage = (...args) => window.showMessage?.(...args);
const showLoadingModal = (...args) => window.showLoadingModal?.(...args);
const hideLoadingModal = () => window.hideLoadingModal?.();

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
 * Renders the logged-out screen in the #game-app container. A single button
 * opens the MILL login modal, which offers every signing method.
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
                <div class="max-w-md mx-auto">
                    <button onclick="openMillLogin()"
                            class="w-full bg-purple-600 hover:bg-purple-700 text-white px-6 py-3 rounded-lg font-medium transition-colors">
                        🔑 Connect Nostr Account
                    </button>
                    <p class="text-gray-500 text-sm mt-4">Extension, mobile signer, private key, or a brand-new identity — all handled in your browser.</p>
                </div>
            </div>
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
 * Shows a popup modal for whitelist denial with an in-app access-request form.
 * The denied npub is captured automatically; the player just adds an optional
 * contact + note. Submissions POST to /api/report/access (logged server-side and
 * mirrored to a pinned GitHub issue) — no GitHub account required.
 * @param {string} errorMessage - The error message to display
 * @param {string} npub - The denied npub (captured at denial time)
 */
function showWhitelistDenialPopup(errorMessage, npub) {
    // Remove existing popup if any
    const existingPopup = document.getElementById('whitelist-denial-popup');
    if (existingPopup) {
        existingPopup.remove();
    }

    // Best-effort recovery of the denied npub if the caller didn't pass one.
    if (!npub) {
        try {
            npub = window.sessionManager?.getSession?.()?.npub || '';
        } catch (_) { /* not logged in yet — fine, form works without it */ }
    }

    // Create popup element
    const popup = document.createElement('div');
    popup.id = 'whitelist-denial-popup';
    popup.dataset.npub = npub || '';
    popup.className = 'fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4';
    const npubDisplay = npub
        ? `<code class="block w-full mb-4 px-3 py-2 bg-gray-800 text-green-400 rounded border border-gray-700 text-xs break-all">${npub}</code>`
        : `<p class="mb-4 text-xs text-red-400">Could not read your public key — please close this and log in again.</p>`;

    popup.innerHTML = `
        <div class="bg-gray-900 border-4 border-red-600 rounded-lg w-full max-w-lg text-left flex flex-col" style="max-height: 90vh;">
            <div class="p-6 overflow-y-auto">
                <div class="text-center mb-5">
                    <div class="text-5xl mb-2">🔑</div>
                    <h2 class="text-2xl font-bold text-red-500">Test Server Access</h2>
                    <p class="text-gray-400 text-sm mt-2">${errorMessage || 'Your public key is not whitelisted for this test server yet.'}</p>
                </div>

                <div class="bg-gray-800 border border-yellow-600 rounded p-3 mb-5 text-xs text-gray-300">
                    <p class="text-yellow-500 font-bold mb-2">⚠️ Before you request access — read this</p>
                    <ul class="list-disc list-inside space-y-1">
                        <li>This server runs the <b>latest commit</b> and auto-deploys — things break often, on purpose.</li>
                        <li>Character saves <b>may be wiped without notice</b> while I test database changes.</li>
                        <li>The server may restart mid-session during a deploy.</li>
                        <li>Want stability? Use the production server instead once it's live.</li>
                    </ul>
                </div>

                <div id="access-request-form">
                    <label class="block text-xs font-bold text-gray-400 mb-1">Your Nostr public key</label>
                    ${npubDisplay}

                    <label class="block text-xs font-bold text-gray-400 mb-1">What do you want to test? <span class="font-normal text-gray-600">(optional)</span></label>
                    <textarea id="access-interest" rows="2" maxlength="4000"
                        placeholder="Combat, items, UI/UX, Nostr integration, performance, balance…"
                        class="w-full mb-4 px-3 py-2 bg-gray-800 text-white rounded border border-gray-700 text-sm"></textarea>

                    <label class="block text-xs font-bold text-gray-400 mb-1">Anything else? <span class="font-normal text-gray-600">(optional)</span></label>
                    <textarea id="access-message" rows="2" maxlength="4000"
                        placeholder="Testing/QA experience, how often you'll play, or just say hi."
                        class="w-full mb-4 px-3 py-2 bg-gray-800 text-white rounded border border-gray-700 text-sm"></textarea>

                    <label class="block text-xs font-bold text-gray-400 mb-1">Contact <span class="font-normal text-gray-600">(optional)</span></label>
                    <input id="access-contact" type="text" maxlength="200"
                        placeholder="Nostr, email, or GitHub username"
                        class="w-full mb-4 px-3 py-2 bg-gray-800 text-white rounded border border-gray-700 text-sm" />

                    <label class="flex items-start gap-2 mb-4 text-sm text-gray-200 cursor-pointer">
                        <input id="access-ack" type="checkbox" class="mt-1 flex-shrink-0"
                            onchange="const b=document.getElementById('access-submit'); b.disabled=!this.checked; b.style.opacity=this.checked?'1':'0.5';" />
                        <span>I understand the test server is unstable and my character data may be wiped without notice.</span>
                    </label>

                    <button id="access-submit" onclick="submitAccessRequest()" disabled style="opacity:0.5;"
                        class="block w-full bg-green-600 hover:bg-green-700 text-white px-6 py-3 rounded-lg font-medium transition-colors mb-3">
                        📝 Request Access
                    </button>
                </div>

                <button onclick="closeWhitelistDenialPopup()"
                        class="w-full bg-gray-600 hover:bg-gray-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors">
                    Close
                </button>
            </div>
        </div>
    `;

    document.body.appendChild(popup);
    logger.info('Whitelist denial popup shown');
}

/**
 * Submits the in-app access request to the server.
 */
async function submitAccessRequest() {
    const popup = document.getElementById('whitelist-denial-popup');
    const form = document.getElementById('access-request-form');
    const submitBtn = document.getElementById('access-submit');
    const npub = popup?.dataset.npub || '';
    const contact = document.getElementById('access-contact')?.value.trim() || '';
    const interest = document.getElementById('access-interest')?.value.trim() || '';
    const message = document.getElementById('access-message')?.value.trim() || '';
    const acknowledged = document.getElementById('access-ack')?.checked || false;

    if (!npub) {
        showMessage('❌ Could not determine your npub — please log in again.', 'error');
        return;
    }
    if (!acknowledged) {
        showMessage('⚠️ Please acknowledge the test-server terms first.', 'error');
        return;
    }

    if (submitBtn) { submitBtn.disabled = true; submitBtn.textContent = 'Sending…'; }

    try {
        const response = await fetch('/api/report/access', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ npub, contact, interest, message, acknowledged })
        });
        const data = await response.json();

        if (response.ok && data.success) {
            if (form) {
                form.innerHTML = `<p class="text-green-400 text-sm mb-2">✅ ${data.message || 'Request received — you\'ll be notified once approved.'}</p>`;
            }
        } else {
            showMessage('❌ ' + (data.error || 'Failed to send request.'), 'error');
            if (submitBtn) { submitBtn.disabled = false; submitBtn.textContent = '📝 Request Access'; }
        }
    } catch (error) {
        logger.error('Failed to submit access request:', error);
        showMessage('❌ Failed to send request: ' + error.message, 'error');
        if (submitBtn) { submitBtn.disabled = false; submitBtn.textContent = '📝 Request Access'; }
    }
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


// For compatibility with inline onclick handlers, attach functions to window.
if (typeof window !== 'undefined') {
    window.showLoginInterface = showLoginInterface;
    window.hideLoginInterface = hideLoginInterface;
    window.openLogin = openLogin;
    window.logout = logout;
    window.showWhitelistDenialPopup = showWhitelistDenialPopup;
    window.submitAccessRequest = submitAccessRequest;
    window.closeWhitelistDenialPopup = closeWhitelistDenialPopup;
}

logger.debug('🔐 Authentication system module loaded.');
