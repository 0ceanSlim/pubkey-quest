/**
 * PWA glue for Pubkey Quest — registers the service worker and surfaces a
 * "Install" affordance when the browser says the app is installable. Plain static
 * JS (not bundled) so both the landing layout and the game layout can include it
 * with one <script src="/pwa.js" defer>.
 */
(function () {
  'use strict';

  if ('serviceWorker' in navigator) {
    window.addEventListener('load', function () {
      navigator.serviceWorker.register('/sw.js').catch(function (err) {
        console.warn('SW registration failed:', err);
      });
    });
  }

  // beforeinstallprompt only fires on Chromium (Android/desktop) when the app is
  // installable and not already installed. iOS Safari has no equivalent — users
  // add via Share → Add to Home Screen — so we simply don't show a button there.
  var deferredPrompt = null;
  var DISMISS_KEY = 'pq_pwa_install_dismissed';

  function isDismissed() {
    try { return localStorage.getItem(DISMISS_KEY) === '1'; } catch (_) { return false; }
  }
  function rememberDismissed() {
    try { localStorage.setItem(DISMISS_KEY, '1'); } catch (_) { /* private mode */ }
  }
  function isInstalled() {
    return (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches) ||
      window.navigator.standalone === true;
  }

  function hideBar() {
    var bar = document.getElementById('pwa-install-bar');
    if (bar) bar.style.display = 'none';
  }

  // The install affordance is an "Install" button paired with a ✕ that dismisses
  // it for good (persisted), so it never nags — the browser's own address-bar
  // install control is always still there if you change your mind.
  function installBar() {
    var existing = document.getElementById('pwa-install-bar');
    if (existing) return existing;

    var bar = document.createElement('div');
    bar.id = 'pwa-install-bar';
    bar.style.cssText = 'position:fixed;right:14px;bottom:14px;z-index:9998;display:none;font-family:inherit;';

    var install = document.createElement('button');
    install.type = 'button';
    install.textContent = '📲 Install';
    install.setAttribute('aria-label', 'Install Pubkey Quest');
    install.style.cssText = [
      'padding:9px 14px', 'font-weight:800', 'font-size:13px', 'font-family:inherit',
      'color:#291e00', 'background:#facc15', 'cursor:pointer', 'vertical-align:top',
      'border-top:3px solid #fef08a', 'border-left:3px solid #fef08a',
      'border-right:0', 'border-bottom:3px solid #ca8a04',
    ].join(';');
    install.addEventListener('click', function () {
      if (!deferredPrompt) { hideBar(); return; }
      deferredPrompt.prompt();
      deferredPrompt.userChoice.finally(function () {
        deferredPrompt = null;
        hideBar();
      });
    });

    var dismiss = document.createElement('button');
    dismiss.type = 'button';
    dismiss.textContent = '✕';
    dismiss.title = 'Dismiss';
    dismiss.setAttribute('aria-label', 'Dismiss install prompt');
    dismiss.style.cssText = [
      'padding:9px 11px', 'font-weight:800', 'font-size:13px', 'font-family:inherit',
      'color:#291e00', 'background:#eab308', 'cursor:pointer', 'vertical-align:top',
      'border-top:3px solid #fef08a', 'border-left:1px solid #ca8a04',
      'border-right:3px solid #ca8a04', 'border-bottom:3px solid #ca8a04',
    ].join(';');
    dismiss.addEventListener('click', function () {
      hideBar();
      rememberDismissed();
    });

    bar.appendChild(install);
    bar.appendChild(dismiss);
    (document.body || document.documentElement).appendChild(bar);
    return bar;
  }

  window.addEventListener('beforeinstallprompt', function (e) {
    e.preventDefault();
    deferredPrompt = e;
    if (isDismissed() || isInstalled()) return; // respect a prior dismissal / installed app
    installBar().style.display = 'block';
  });

  window.addEventListener('appinstalled', function () {
    deferredPrompt = null;
    hideBar();
    rememberDismissed();
  });
})();
