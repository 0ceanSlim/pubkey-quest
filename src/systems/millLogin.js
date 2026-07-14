/**
 * MILL login integration for Pubkey Quest.
 *
 * MILL (Multi-Interface Login Layer) is a self-contained Web Component that
 * handles every Nostr signing method client-side (NIP-07 extension, NIP-46
 * bunker, NIP-55 Amber, private key, read-only, new identity). It replaces the
 * hand-rolled login modals that used to live in nav-play.html / auth.js.
 *
 * All signing happens in the browser — the server only ever receives the
 * resulting hex public key (see cmd/server/auth/grain.go). On a successful
 * connect we install the signer as window.nostr (so the rest of the app, and
 * future save-signing, can use it) and hand the pubkey to the session manager,
 * which POSTs /api/auth/login and enforces the whitelist uniformly.
 *
 * @module systems/millLogin
 */

import MILL from 'nostr-mill';
import { logger } from '../lib/logger.js';

// MILL method id → server signing_method value (session.SigningMethod).
const METHOD_MAP = {
    nip07: 'browser_extension',
    nip46: 'bunker',
    nip55: 'amber',
    privatekey: 'encrypted_key',
    newkey: 'encrypted_key',
    readonly: 'none',
};

// Which methods to offer. Read-only is omitted: playing the game requires the
// ability to sign (write mode). NIP-55 (Amber direct) is kept enabled and wired
// to our server callback route (/api/auth/amber-callback).
const LOGIN_METHODS = ['newkey', 'nip07', 'nip46', 'nip55', 'privatekey'];

/**
 * Build a MILL theme object from the currently-active Pubkey Quest palette.
 * Pubkey has a theme switcher, so we read the live --color-* custom properties
 * at open() time and map them onto MILL's --mill-* tokens. Square corners + the
 * pixel font give it the win95 look; anything we don't set keeps MILL's default.
 */
function pubkeyQuestTheme() {
    const css = getComputedStyle(document.documentElement);
    const v = (name, fallback) => css.getPropertyValue(name).trim() || fallback;

    const bgPrimary = v('--color-bgPrimary', '#1e1e1e');
    const bgSecondary = v('--color-bgSecondary', '#2d2d2d');
    const bgTertiary = v('--color-bgTertiary', '#3c3c3c');
    const textPrimary = v('--color-textPrimary', '#d4d4d4');
    const textSecondary = v('--color-textSecondary', '#cccccc');
    const textMuted = v('--color-textMuted', '#969696');
    const textHi = v('--color-textHighlighted', '#569cd6');

    return {
        '--mill-bg': bgPrimary,
        '--mill-surface': bgSecondary,
        '--mill-card': bgTertiary,
        '--mill-card-hover': bgSecondary,
        '--mill-border': textMuted,
        '--mill-border-light': textHi,
        '--mill-accent': textHi,
        '--mill-teal': textHi,
        '--mill-text': textPrimary,
        '--mill-text-secondary': textSecondary,
        '--mill-muted': textMuted,
        '--mill-radius': '0px',
        '--mill-font': "'Dogica Pixel', 'Dogica', monospace",
        '--mill-font-mono': "'Dogica Pixel', 'Dogica', monospace",
    };
}

/**
 * Complete login after MILL produces a signer + pubkey. Installs the signer as
 * window.nostr and defers to the session manager for the /api/auth/login POST
 * (which handles the whitelist gate and fires authentication events).
 * @param {{ method: string, pubkey: string, signer?: object }} result
 */
async function finishLogin(result) {
    logger.info(`MILL connected via ${result.method}`);

    // Make the chosen signer the page-wide window.nostr where MILL provides one
    // (extension is already global; bunker / private-key / new-key are not).
    if (result.signer) {
        try {
            MILL.installAsWindowNostr(result.signer);
        } catch (err) {
            logger.warn('Failed to install signer as window.nostr:', err);
        }
    }

    const signingMethod = METHOD_MAP[result.method] || 'none';

    // Remember method + pubkey so we can silently rebuild the signer after a
    // page reload (see restoreSignerFromSession).
    try {
        localStorage.setItem(
            'pubkey_quest_signer',
            JSON.stringify({ method: signingMethod, pubkey: result.pubkey })
        );
    } catch (_) { /* private mode — non-fatal */ }

    if (!window.sessionManager) {
        logger.error('SessionManager not available to complete login');
        return;
    }

    try {
        await window.sessionManager.performLogin({
            public_key: result.pubkey,
            signing_method: signingMethod,
            mode: 'write',
        });
    } catch (err) {
        // performLogin already surfaces whitelist denials + failure events.
        logger.error('Login failed after MILL connect:', err);
    }
}

/**
 * MILL ships no responsive CSS: fixed-px font sizes, a 480px modal, and no media
 * queries — all inside a shadow root, so page CSS can't reach it. With our wide
 * pixel font that overflows the modal on phones. MILL mounts one persistent
 * <nostr-signer> element with an OPEN shadow root, so inject a small mobile
 * stylesheet there once. The @media rules re-evaluate on resize/rotate, so this
 * adapts even if the modal was opened on desktop first.
 */
function injectMillResponsiveStyles() {
    const apply = () => {
        try {
            const host = document.querySelector('nostr-signer');
            const root = host && host.shadowRoot;
            if (!root || root.getElementById('pq-mill-mobile')) return;
            const style = document.createElement('style');
            style.id = 'pq-mill-mobile';
            style.textContent = `
                /* Long npub / bunker:// strings must wrap, never overflow the box. */
                .mill-modal, .mill-modal * { overflow-wrap: anywhere; }
                @media (max-width: 480px) {
                    .mill-overlay { padding: 10px !important; }
                    /* Reflow-shrink the fixed-px layout so the pixel font fits a phone. */
                    .mill-modal { zoom: 0.9; max-height: 94vh !important; }
                }
                @media (max-width: 360px) {
                    .mill-modal { zoom: 0.82; }
                }
            `;
            root.appendChild(style);
        } catch (err) {
            logger.debug('MILL responsive style injection skipped:', err?.message || err);
        }
    };
    // The element is appended synchronously by open(), but guard with rAF in case
    // a future MILL version defers the mount.
    apply();
    if (typeof requestAnimationFrame === 'function') requestAnimationFrame(apply);
}

/**
 * Open the MILL login modal, themed to the active Pubkey Quest palette.
 */
export function openMillLogin() {
    // Compact density (smaller padding, descriptions hidden) keeps the modal
    // from overflowing narrow phone screens; comfortable stays on desktop.
    const isMobile = typeof window !== 'undefined'
        && typeof window.matchMedia === 'function'
        && window.matchMedia('(max-width: 480px)').matches;

    MILL.open({
        appName: 'Pubkey Quest',
        theme: pubkeyQuestTheme(),
        methods: LOGIN_METHODS,
        density: isMobile ? 'compact' : 'comfortable',
        amberCallback: `${window.location.origin}/api/auth/amber-callback`,
        onConnected: (result) => finishLogin(result),
    });

    injectMillResponsiveStyles();
}

/**
 * After a page reload with an active server session, rebuild the client-side
 * signer (window.nostr) from MILL's persisted state so signing keeps working
 * without re-opening the picker. No-op if there's nothing to restore.
 */
export async function restoreSignerFromSession() {
    let saved;
    try {
        saved = JSON.parse(localStorage.getItem('pubkey_quest_signer') || 'null');
    } catch (_) {
        saved = null;
    }
    if (!saved || !saved.pubkey) return;

    try {
        const signer = await MILL.restore({ method: saved.method, pubkey: saved.pubkey });
        if (signer) {
            MILL.installAsWindowNostr(signer);
            logger.debug('Restored window.nostr signer from session');
        }
    } catch (err) {
        logger.debug('Signer restore skipped:', err?.message || err);
    }
}

// Expose for inline template handlers (nav-play.html, auth.js login screen).
if (typeof window !== 'undefined') {
    window.openMillLogin = openMillLogin;
    window.MILL = MILL;
}
