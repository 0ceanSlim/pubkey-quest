/**
 * Instant, styled hover tips for anything with a `data-tip` attribute.
 *
 * The browser's own `title` tooltips come up late and unstyled (and stack on
 * top of the item-slot tip), so game UI uses `data-tip` instead. Styled to
 * match the item-slot hover tip (slotInteractions.js).
 *
 * @module ui/hoverTip
 */

let tipEl = null;

function ensureTip() {
    if (tipEl) return tipEl;
    tipEl = document.createElement('div');
    tipEl.className = 'hover-tip';
    tipEl.style.cssText =
        'position:fixed;z-index:2100;pointer-events:none;display:none;' +
        'background:#1a1a1a;color:#fff;border:1px solid #6a6a6a;' +
        'padding:2px 6px;font-size:10px;max-width:260px;white-space:normal;line-height:1.4;' +
        'box-shadow:1px 1px 0 #000;';
    document.body.appendChild(tipEl);
    return tipEl;
}

function place(e) {
    const tip = ensureTip();
    const w = tip.offsetWidth, h = tip.offsetHeight;
    let x = e.clientX + 14, y = e.clientY + 14;
    if (x + w > window.innerWidth - 4) x = Math.max(4, e.clientX - 14 - w);
    if (y + h > window.innerHeight - 4) y = Math.max(4, e.clientY - 14 - h);
    tip.style.left = `${x}px`;
    tip.style.top = `${y}px`;
}

function onMove(e) {
    const el = e.target.closest?.('[data-tip]');
    const text = el?.getAttribute('data-tip');
    if (!text) {
        if (tipEl) tipEl.style.display = 'none';
        return;
    }
    const tip = ensureTip();
    if (tip.textContent !== text) tip.textContent = text;
    tip.style.display = 'block';
    place(e);
}

document.addEventListener('mousemove', onMove, { passive: true });
document.addEventListener('mouseleave', () => { if (tipEl) tipEl.style.display = 'none'; });
document.addEventListener('pointerdown', () => { if (tipEl) tipEl.style.display = 'none'; }, { passive: true });
