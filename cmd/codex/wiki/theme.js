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
      '<a class="pq-brand" href="__WIKI_BASE__" title="Wiki home">' +
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

  // ── Sortable tables: click a column header to sort by it, click again to flip.
  // Cells sort by their leading number ("1/4", "1500 gold", "12.5%", "3–5"),
  // falling back to text; empty cells always sort last.
  function sortKey(text) {
    var t = text.trim();
    if (!t) return null;
    var frac = t.match(/^(\d+)\/(\d+)/);
    if (frac) return +frac[1] / +frac[2];
    var n = t.replace(/,/g, '').match(/^-?\d+(\.\d+)?/);
    return n ? parseFloat(n[0]) : t.toLowerCase();
  }
  function compare(a, b) {
    if (a === null || b === null) return a === b ? 0 : a === null ? 1 : -1;
    if (typeof a === typeof b) return a < b ? -1 : a > b ? 1 : 0;
    return typeof a === 'number' ? -1 : 1; // numbers before words
  }
  function sortable() {
    var tables = document.querySelectorAll('.markdown-body table');
    Array.prototype.forEach.call(tables, function (table) {
      var head = table.tHead, body = table.tBodies[0];
      if (!head || !body || body.rows.length < 2) return;
      Array.prototype.forEach.call(head.rows[0].cells, function (th, col) {
        if (!th.textContent.trim()) return; // icon columns
        th.classList.add('pq-sortable');
        th.setAttribute('role', 'button');
        th.tabIndex = 0;
        function sort() {
          var dir = th.getAttribute('aria-sort') === 'ascending' ? -1 : 1;
          Array.prototype.forEach.call(head.rows[0].cells, function (c) { c.removeAttribute('aria-sort'); });
          th.setAttribute('aria-sort', dir === 1 ? 'ascending' : 'descending');
          var rows = Array.prototype.slice.call(body.rows);
          rows.sort(function (r1, r2) {
            var a = sortKey(r1.cells[col] ? r1.cells[col].textContent : '');
            var b = sortKey(r2.cells[col] ? r2.cells[col].textContent : '');
            if (a === null || b === null) return compare(a, b); // empties stay last either way
            return dir * compare(a, b);
          });
          rows.forEach(function (r) { body.appendChild(r); });
        }
        th.addEventListener('click', sort);
        th.addEventListener('keydown', function (e) {
          if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); sort(); }
        });
      });
    });
  }

  function ready() {
    header();
    sortable();
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', ready);
  } else {
    ready();
  }
})();
