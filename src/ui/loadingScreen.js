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
    setTimeout(() => {
        screen.style.transition = 'opacity 0.25s ease';
        screen.style.opacity = '0';
        setTimeout(() => screen.remove(), 260);
    }, 150);
}

/** Show why loading stopped (the caller decides where to go next). */
export function failLoading(message) {
    clearInterval(noteTimer);
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
