/**
 * The loading screen over the game while it starts: a checklist of steps
 * (key, save, world, scene) that tick off as they finish, and a bar.
 *
 * The markup lives in www/views/game.html (#pq-loading) so it is on screen
 * before this bundle runs; this module only drives it.
 *
 * @module ui/loadingScreen
 */

const STEPS = ['key', 'save', 'world', 'scene'];
const done = new Set();
const startedAt = performance.now();
let noteTimer = null;
let quipTimer = null;

// Long enough to read a saying even when everything loads at once.
const MIN_SHOWN_MS = 1200;
const QUIP_EVERY_MS = 3000;

// Sayings shown while loading, one at a time in a random order.
const QUIPS = [
    'Your hero was rolled from your npub. Blame the hash, not the dice.',
    'Asking the relays nicely…',
    'Your save is a signed note. Nobody can forge your deeds.',
    'Not your keys, not your character.',
    'Every npub is a hero. Some just haven\'t logged in yet.',
    'Gossip travels faster than relays. Barely.',
    'Counting gold. Just gold. There is only gold.',
    'Thistle swears everything in the wagon is genuine. Mostly.',
    'Barnaby\'s pony begs you to buy something light.',
    'The watchtower fire is probably nothing. Probably.',
    'Polishing the dragonfire locks of the Ember Vault…',
    'Waking the innkeepers. They are not morning people.',
    'Teaching the cellar rats to look menacing.',
    'Rolling initiative for the mule.',
    'Telling the goblins you\'re on your way.',
    'Lighting torches in dungeons you haven\'t found yet.',
    'Sweeping the dust off the Merchant\'s Highway…',
    'Rations: an adventurer\'s least favourite, most necessary friend.',
    'Hunger and fatigue are patient. They will wait for you.',
    'A rested adventurer is a living adventurer.',
    'Night really falls out there. Pack a light.',
    'Sharpening swords you haven\'t looted yet.',
    'Somewhere, a bard is tuning a lute badly.',
    'Shuffling the deck for Three-Dragon Ante…',
];

function shuffled(list) {
    const out = [...list];
    for (let i = out.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [out[i], out[j]] = [out[j], out[i]];
    }
    return out;
}

function startQuips() {
    const box = el('pq-loading-quip');
    if (!box) return;
    const order = shuffled(QUIPS);
    let i = 0;
    box.textContent = order[0];
    quipTimer = setInterval(() => {
        i = (i + 1) % order.length;
        box.style.opacity = '0';
        setTimeout(() => {
            box.textContent = order[i];
            box.style.opacity = '1';
        }, 300);
    }, QUIP_EVERY_MS);
}

function el(id) {
    return document.getElementById(id);
}

function paint() {
    const bar = el('pq-loading-bar');
    if (bar) bar.style.width = `${Math.max(4, Math.round((100 * done.size) / STEPS.length))}%`;
}

/** Mark a step finished. */
export function stepDone(step) {
    if (done.has(step)) return;
    done.add(step);
    const li = document.querySelector(`#pq-loading-steps [data-step="${step}"]`);
    if (li) {
        li.style.color = '#e5e7eb';
        const mark = li.querySelector('[data-mark]');
        if (mark) {
            mark.textContent = '✔';
            mark.style.color = '#22c55e';
        }
    }
    paint();
}

/** Take the loading screen away (after a short beat so the full bar shows). */
export function finishLoading() {
    STEPS.forEach(stepDone);
    clearInterval(noteTimer);
    const screen = el('pq-loading');
    if (!screen) return;
    const wait = Math.max(150, MIN_SHOWN_MS - (performance.now() - startedAt));
    setTimeout(() => {
        clearInterval(quipTimer);
        screen.style.transition = 'opacity 0.25s ease';
        screen.style.opacity = '0';
        setTimeout(() => screen.remove(), 260);
    }, wait);
}

/** Show why loading stopped (the caller decides where to go next). */
export function failLoading(message) {
    clearInterval(noteTimer);
    clearInterval(quipTimer);
    const note = el('pq-loading-note');
    if (note) {
        note.style.color = '#f87171';
        note.textContent = message;
    }
}

// A slow relay can hold the save for a few seconds: say so rather than sit
// silently.
noteTimer = setInterval(() => {
    const note = el('pq-loading-note');
    if (!note || done.has('scene')) return;
    const secs = Math.round((performance.now() - startedAt) / 1000);
    if (secs >= 4) {
        note.textContent = done.has('save')
            ? `Still working… ${secs}s`
            : `Waiting on your save… ${secs}s`;
    }
}, 1000);

startQuips();
