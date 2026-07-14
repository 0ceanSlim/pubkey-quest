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

  function installButton() {
    var existing = document.getElementById('pwa-install-btn');
    if (existing) return existing;
    var b = document.createElement('button');
    b.id = 'pwa-install-btn';
    b.type = 'button';
    b.textContent = '📲 Install';
    b.setAttribute('aria-label', 'Install Pubkey Quest');
    b.style.cssText = [
      'position:fixed', 'right:14px', 'bottom:14px', 'z-index:9998',
      'padding:9px 15px', 'font-weight:800', 'font-size:13px', 'font-family:inherit',
      'color:#291e00', 'background:#facc15', 'cursor:pointer', 'display:none',
      'border-top:3px solid #fef08a', 'border-left:3px solid #fef08a',
      'border-right:3px solid #ca8a04', 'border-bottom:3px solid #ca8a04',
    ].join(';');
    b.addEventListener('click', function () {
      if (!deferredPrompt) return;
      deferredPrompt.prompt();
      deferredPrompt.userChoice.finally(function () {
        deferredPrompt = null;
        b.style.display = 'none';
      });
    });
    (document.body || document.documentElement).appendChild(b);
    return b;
  }

  window.addEventListener('beforeinstallprompt', function (e) {
    e.preventDefault();
    deferredPrompt = e;
    installButton().style.display = 'block';
  });

  window.addEventListener('appinstalled', function () {
    deferredPrompt = null;
    var b = document.getElementById('pwa-install-btn');
    if (b) b.style.display = 'none';
  });
})();
