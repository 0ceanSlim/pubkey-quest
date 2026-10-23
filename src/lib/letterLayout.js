/**
 * Placing the letter's text on the parchment in letter-page.png.
 *
 * The scene is a `background-size: cover; background-position: center` image,
 * so where the page lands on screen depends on the window's shape. This works
 * it out from the image's own geometry, sizes the text box to the clear part of
 * the page (inside its ragged edges, above the thumbs at its lower corners),
 * then picks the largest font that fits the whole letter, so it reads at any
 * window size and only scrolls when even the smallest size won't fit. Used by
 * the intro's letter scene and the in-game letter reader.
 *
 * @module lib/letterLayout
 */

const IMG_W = 1408;
const IMG_H = 736;
// The page's clear writing area, in image pixels.
const PAGE = { left: 562, top: 150, right: 846, bottom: 592 };
const MAX_FONT = 34; // px on screen
const MIN_FONT = 14;

/**
 * Size and place a text box over the page for the current window.
 * @param {HTMLElement} el - fixed/absolute-positioned text box
 */
export function fitToParchment(el) {
    const fw = window.innerWidth;
    const fh = window.innerHeight;
    const scale = Math.max(fw / IMG_W, fh / IMG_H);
    const ox = (fw - IMG_W * scale) / 2;
    const oy = (fh - IMG_H * scale) / 2;
    Object.assign(el.style, {
        left: `${ox + PAGE.left * scale}px`,
        top: `${oy + PAGE.top * scale}px`,
        width: `${(PAGE.right - PAGE.left) * scale}px`,
        height: `${(PAGE.bottom - PAGE.top) * scale}px`,
        transform: 'none',
        minWidth: '0',
        maxWidth: 'none',
        maxHeight: 'none',
        padding: '0',
        boxSizing: 'border-box',
        lineHeight: '1.15',
        overflowY: 'auto',
    });
    // The largest font that fits it all (a short binary search).
    let lo = MIN_FONT, hi = MAX_FONT;
    while (hi - lo > 0.5) {
        const mid = (lo + hi) / 2;
        el.style.fontSize = `${mid}px`;
        if (el.scrollHeight <= el.clientHeight + 1) lo = mid;
        else hi = mid;
    }
    el.style.fontSize = `${lo}px`;
}

/**
 * Keep a text box on the page as the window resizes. Returns a stop function.
 * @param {HTMLElement} el
 */
export function keepOnParchment(el) {
    const fit = () => fitToParchment(el);
    fit();
    // Caveat may still be loading: fit again once fonts are ready.
    document.fonts?.ready?.then(fit);
    window.addEventListener('resize', fit);
    return () => window.removeEventListener('resize', fit);
}
