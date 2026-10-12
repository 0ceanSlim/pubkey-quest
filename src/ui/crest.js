/**
 * Crests — a player's fallback face when they have no portrait.
 *
 * A tiny heraldic shield generated from the npub, so it's stable for that key
 * (one "house" for every character on it) and needs no art. Field colour ×
 * division × second colour gives ~700 distinct fields; the centre carries the
 * character's initial (a class sigil later). See docs/draft/party-combat-plan.md §2c.
 *
 * @module ui/crest
 */

// Heraldic tinctures: two metals, the rest colours.
const TINCTURES = [
    '#d4a017', // or
    '#e5e7eb', // argent
    '#b91c1c', // gules
    '#1d4ed8', // azure
    '#15803d', // vert
    '#1f2937', // sable
    '#7e22ce', // purpure
    '#c2410c', // tenné
    '#7f1d1d', // sanguine
    '#0e7490', // celeste
];

// Each division is the second-colour area, drawn over the field and clipped to
// the shield (viewBox 16 × 18).
const DIVISIONS = [
    '',                                            // plain
    '<rect x="8" y="0" width="8" height="18"/>',   // per pale
    '<rect x="0" y="9" width="16" height="9"/>',   // per fess
    '<polygon points="0,0 16,0 0,18"/>',           // per bend
    '<rect x="8" y="0" width="8" height="9"/><rect x="0" y="9" width="8" height="9"/>', // quarterly
    '<polygon points="0,18 8,8 16,18 16,13 8,3 0,13"/>', // chevron
    '<rect x="6" y="0" width="4" height="18"/><rect x="0" y="6" width="16" height="4"/>', // cross
    '<polygon points="0,0 3,0 16,15 16,18 13,18 0,3"/><polygon points="16,0 13,0 0,15 0,18 3,18 16,3"/>', // saltire
    '<rect x="2" y="0" width="3" height="18"/><rect x="8" y="0" width="3" height="18"/><rect x="14" y="0" width="2" height="18"/>', // paly
];

const SHIELD = 'M1 1 H15 V9 C15 14 8 17 8 17 C8 17 1 14 1 9 Z';

/** FNV-1a — a small, stable string hash. */
function hash(str) {
    let h = 0x811c9dc5;
    for (let i = 0; i < str.length; i++) {
        h ^= str.charCodeAt(i);
        h = Math.imul(h, 0x01000193);
    }
    return h >>> 0;
}

/** The crest's three choices for a seed (the npub). */
export function crestParts(seed) {
    const h = hash(seed || 'anon');
    const field = h % TINCTURES.length;
    const division = Math.floor(h / TINCTURES.length) % DIVISIONS.length;
    // The second colour is never the field colour.
    const offset = 1 + (Math.floor(h / (TINCTURES.length * DIVISIONS.length)) % (TINCTURES.length - 1));
    const second = (field + offset) % TINCTURES.length;
    return { field: TINCTURES[field], second: TINCTURES[second], division };
}

/**
 * The crest as an SVG string.
 * @param {string} seed    - the player's npub
 * @param {string} initial - the character's initial ('' for none)
 * @param {number} size    - rendered height in px (width is 8/9 of it)
 */
export function crestSVG(seed, initial = '', size = 18) {
    const { field, second, division } = crestParts(seed);
    const clip = `crest-${hash(seed || 'anon').toString(36)}`;
    const letter = (initial || '').trim().charAt(0).toUpperCase();
    const charge = letter
        ? `<text x="8" y="11.5" text-anchor="middle" font-size="8" font-weight="bold"
                 fill="#f9fafb" stroke="#111" stroke-width="1.6" paint-order="stroke"
                 font-family="inherit">${letter.replace(/[<>&"]/g, '')}</text>`
        : '';
    return `<svg viewBox="0 0 16 18" width="${Math.round(size * 8 / 9)}" height="${size}"
                 xmlns="http://www.w3.org/2000/svg" style="display:block;shape-rendering:crispEdges">
        <defs><clipPath id="${clip}"><path d="${SHIELD}"/></clipPath></defs>
        <g clip-path="url(#${clip})">
            <rect width="16" height="18" fill="${field}"/>
            <g fill="${second}">${DIVISIONS[division]}</g>
        </g>
        <path d="${SHIELD}" fill="none" stroke="#111" stroke-width="1"/>
        ${charge}
    </svg>`;
}
