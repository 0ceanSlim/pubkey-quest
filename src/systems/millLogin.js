/**
 * MILL login integration for Pubkey Quest.
 *
 * MILL (Multi-Interface Login Layer) is a self-contained Web Component that
 * handles every Nostr signing method client-side (NIP-07 extension, NIP-46
 * bunker, NIP-55 Amber, private key, new identity, Google "Secure login" via
 * pomegranate/FROST). It replaces the hand-rolled login modals that used to
 * live in nav-play.html / auth.js. Modeled on grain's mill-bridge.js.
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

// MILL method id → server signing_method value (session.SigningMethod, grain
// client/session/types.go). google/pomegranate get their own enum values so
// MILL.restore's aliases (google→privatekey, pomegranate→nip46) rebuild the
// right signer after a reload.
const METHOD_MAP = {
    nip07: 'browser_extension',
    nip46: 'bunker',
    nip55: 'amber',
    privatekey: 'encrypted_key',
    newkey: 'encrypted_key',
    readonly: 'none',
    google: 'google',
    pomegranate: 'pomegranate',
};

// Picker layout, mirroring grain's mill-bridge. The "I'm new here" callout
// (newkey) is pulled out to the top card — it must stay in `methods` or the
// callout silently doesn't render. Google "Secure login" (pomegranate) + the
// platform's natural signer are the main tiles; the rest collapse under
// "More options". Read-only is omitted: playing requires write mode. `google`
// (Drive+PIN) is never listed, so that PIN login stays hidden.
const LOGIN_LAYOUT = {
    methods: ['newkey', 'pomegranate', 'nip07'],
    moreMethods: ['nip46', 'privatekey'],
    platforms: {
        // Android: Amber (NIP-55 intent) — the same-device path, where NIP-46
        // over relays stalls on a backgrounded signer.
        android: {
            methods: ['newkey', 'pomegranate', 'nip55'],
            moreMethods: ['nip46', 'privatekey'],
        },
        // iOS: no NIP-07 extensions in iOS browsers.
        ios: {
            methods: ['newkey', 'pomegranate', 'privatekey'],
            moreMethods: ['nip46'],
        },
    },
};

// The signer MILL produced (or restored) for this page. Kept separately from
// window.nostr so logout can disconnect it without poking a NIP-07 extension.
let activeSigner = null;

/**
 * Build a MILL theme object from the currently-active Pubkey Quest palette.
 * Pubkey has a theme switcher, so we read the live --color-* custom properties
 * at open() time and map them onto MILL's --mill-* tokens. Square corners, no
 * glow/blur/shadow, and the pixel font give it the win95 look; anything we
 * don't set keeps MILL's default.
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
        '--mill-shadow': 'none',
        '--mill-glow': 'transparent',
        '--mill-overlay-blur': '0',
        '--mill-font': "'Dogica Pixel', 'Dogica', monospace",
        '--mill-font-mono': "'Dogica Pixel', 'Dogica', monospace",
    };
}

/**
 * Install a signer as the page-wide window.nostr (extension is already global;
 * bunker / private-key / new-key / pomegranate are not).
 * @param {object} signer
 */
function installSigner(signer) {
    activeSigner = signer;
    try {
        MILL.installAsWindowNostr(signer);
    } catch (err) {
        logger.warn('Failed to install signer as window.nostr:', err);
    }
}

/**
 * Complete login after MILL produces a signer + pubkey. Installs the signer as
 * window.nostr and defers to the session manager for the /api/auth/login POST
 * (which handles the whitelist gate and fires authentication events).
 * @param {{ method: string, pubkey: string, signer?: object }} result
 */
async function finishLogin(result) {
    logger.info(`MILL connected via ${result.method}`);

    if (result.signer) installSigner(result.signer);

    const signingMethod = METHOD_MAP[result.method] || 'none';

    // Close MILL right away; the login POST can take a moment and the session
    // manager drives the logged-in UI from here.
    MILL.close();

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
 * MILL's modal is sized for a proportional font; with our wide pixel font it
 * overflows on phones. MILL mounts one persistent <nostr-signer> element with
 * an OPEN shadow root, so inject a small mobile stylesheet there once. The
 * @media rules re-evaluate on resize/rotate, so this adapts even if the modal
 * was opened on desktop first.
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
        // Name the remote signer / bunker shows when authorizing.
        appName: 'Pubkey Quest',
        theme: pubkeyQuestTheme(),
        // Grid picker: main methods render as tiles. MILL does its own platform
        // detection and merges the matching `platforms` block over the base.
        layout: 'grid',
        ...LOGIN_LAYOUT,
        callout: 'newkey',
        density: isMobile ? 'compact' : 'comfortable',
        header: {
            logo: '/res/img/static/logo.png',
            logoHeight: 64,
            eyebrow: false,
            title: 'Pubkey Quest',
            message: 'Choose how to connect your Nostr identity.',
            align: 'center',
        },
        // Google "Secure login" (pomegranate/FROST) via the njump ecosystem
        // defaults — the same key the player gets in any njump-based client.
        pomegranate: true,
        // MILL's default tip recommends NIP-07; off-message when leading with Google.
        tip: false,
        amberCallback: `${window.location.origin}/api/auth/amber-callback`,
        onConnected: (result) => finishLogin(result),
    });

    injectMillResponsiveStyles();
}

/**
 * Rebuild the signer from MILL's persisted state (sessionStorage). MILL.restore
 * accepts grain's SigningMethod enum directly.
 * @param {{ publicKey: string, signingMethod: string }} session
 * @returns {Promise<boolean>} true if a signer was installed
 */
async function tryRestore(session) {
    try {
        const signer = await MILL.restore({
            method: session.signingMethod,
            pubkey: session.publicKey,
        });
        if (!signer || typeof signer.signEvent !== 'function') return false;
        installSigner(signer);
        return true;
    } catch (err) {
        logger.debug('Signer restore skipped:', err?.message || err);
        return false;
    }
}

/**
 * After a page reload with an active server session, rebuild window.nostr so
 * signing keeps working without re-opening the picker. The session cookie
 * survives the reload and knows the signing method + pubkey. NIP-07 extensions
 * inject window.nostr asynchronously, so that method retries for ~3s; every
 * other method restores in one attempt (retrying could spin up duplicate bunker
 * connections). No-op if there's nothing to restore.
 * @param {{ publicKey?: string, signingMethod?: string } | null} session
 */
export async function restoreSignerFromSession(session) {
    if (!session || !session.publicKey || !session.signingMethod) return;
    if (session.signingMethod === 'none') return;

    if (await tryRestore(session)) {
        logger.debug('Restored window.nostr signer from session');
        return;
    }
    if (session.signingMethod !== 'browser_extension') return;

    for (const delay of [100, 200, 400, 800, 1500]) {
        await new Promise((r) => setTimeout(r, delay));
        if (await tryRestore(session)) {
            logger.debug('Restored NIP-07 signer after extension injected');
            return;
        }
    }
}

/**
 * On logout, drop the signer and wipe MILL's persisted restore state
 * (encrypted nsec, signing grants, bunker connection) so the next page load
 * can't silently restore the account we just logged out of.
 */
export function clearMillSigner() {
    try {
        activeSigner?.disconnect?.();
    } catch (_) { /* best effort */ }
    activeSigner = null;
    try {
        MILL.clearRestoreState();
    } catch (_) { /* best effort */ }
    // Restore key used before the server session carried the method.
    try { localStorage.removeItem('pubkey_quest_signer'); } catch (_) { /* private mode */ }
}

// Expose for inline template handlers (nav-play.html, auth.js login screen).
if (typeof window !== 'undefined') {
    window.openMillLogin = openMillLogin;
    window.MILL = MILL;
}
