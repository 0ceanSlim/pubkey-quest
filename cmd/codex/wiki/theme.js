// Pubkey Quest wiki theme — written to the wiki as custom.js (gollum --js) by
// codex --wiki, so edit it here. The wiki is served at /wiki on the game server, so
// it shares the game's localStorage: the theme a player picked in the game
// ('preferred-theme', src/systems/themeManager.js) applies here too, and picking one
// here changes the game's.
(function () {
  var THEMES = ['dark', 'light', 'midnight', 'lava', 'solar', 'matrix'];
  var KEY = 'preferred-theme';

  function stored() {
    try { return localStorage.getItem(KEY); } catch (e) { return null; }
  }
  function apply(name) {
    if (THEMES.indexOf(name) < 0) name = 'dark';
    document.documentElement.setAttribute('data-theme', name);
    return name;
  }
  var current = apply(stored());

  function header() {
    if (document.getElementById('pq-header')) return;
    var wrap = document.getElementById('wiki-wrapper');
    if (!wrap) return;

    var bar = document.createElement('header');
    bar.id = 'pq-header';
    bar.innerHTML =
      '<a class="pq-brand" href="/" title="Back to Pubkey Quest">' +
      '<img src="/res/img/static/logo.png" alt="">' +
      '<span>pubkey quest</span><span class="pq-tag">wiki</span></a>' +
      '<div class="pq-controls">' +
      '<a class="pq-play" href="/" title="Play Pubkey Quest"><span>Play </span>&#9654;</a>' +
      '<select aria-label="Theme" title="Change theme"></select></div>';

    var select = bar.querySelector('select');
    THEMES.forEach(function (t) {
      var o = document.createElement('option');
      o.value = t;
      o.textContent = t.charAt(0).toUpperCase() + t.slice(1);
      if (t === current) o.selected = true;
      select.appendChild(o);
    });
    select.addEventListener('change', function () {
      current = apply(select.value);
      try { localStorage.setItem(KEY, current); } catch (e) {}
    });

    wrap.parentNode.insertBefore(bar, wrap);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', header);
  } else {
    header();
  }
})();
