/**
 * Reading the caretaker's letter (the "caretakers-letter" item, right-click →
 * Read): the same parchment scene as the intro, with the letter written for
 * the character's background and its last line for their starting city.
 *
 * @module ui/letterReader
 */
import { logger } from '../lib/logger.js';
import { getGameStateSync } from '../state/gameState.js';
import { gameAPI } from '../lib/api.js';
import { keepOnParchment } from '../lib/letterLayout.js';

let _el = null;
let _stopFit = null;

function close() {
    _stopFit?.();
    _stopFit = null;
    _el?.remove();
    _el = null;
    document.removeEventListener('keydown', onKey, true);
}

function onKey(e) {
    if (e.key === 'Escape') { e.preventDefault(); close(); }
}

/** The letter's text: the background's letter, then the starting city's line. */
async function letterText(character) {
    const [intro, starts] = await Promise.all([
        fetch('/api/introductions').then((r) => r.json()),
        fetch('/data/systems/new-character/starting-locations.json').then((r) => r.json()),
    ]);
    const body = intro.background_letters?.find((e) => e.backgrounds.includes(character.background))?.text ?? '';
    const city = starts.racial_starting_cities?.[character.race] ?? 'kingdom';
    const closing = intro.letter_closings?.[city] ?? '';
    return closing ? `${body}<br><br>${closing}` : body;
}

/** Open the letter over the screen. */
export async function readLetter() {
    close();
    const character = getGameStateSync()?.character ?? {};
    let text = '';
    try {
        text = await letterText(character);
    } catch (err) {
        logger.error('Could not load the letter:', err);
    }
    if (!text) text = 'The ink has run too badly to read.';

    _el = document.createElement('div');
    _el.id = 'letter-reader';
    _el.innerHTML = `
      <style>
        #letter-reader { position: fixed; inset: 0; z-index: 160; background: #000 url(/res/img/scene/letter-page.png) center / cover no-repeat; image-rendering: pixelated; }
        #letter-reader .lr-text { position: absolute; top: 23vh; left: 50%; transform: translateX(-50%);
          width: 21vw; min-width: 220px; max-width: 360px; height: 46vh; max-height: 540px; overflow-y: auto;
          padding: 1.2rem 1.6rem; box-sizing: border-box; color: #1a1a1a; text-align: left;
          font-family: 'Caveat', cursive; font-size: 1.45rem; line-height: 1.05; }
        #letter-reader .lr-close { position: absolute; bottom: 6vh; left: 50%; transform: translateX(-50%);
          font-family: 'Dogica', monospace; font-size: 0.8rem; padding: 0.5rem 1.2rem; cursor: pointer; color: #000; background: #ffd700;
          border: none; box-shadow: inset 2px 2px 0 #ffed4e, inset -2px -2px 0 #cc9900, 3px 3px 0 rgba(0,0,0,0.4); }
        @media (max-height: 520px) { #letter-reader .lr-text { font-size: 1.05rem; height: 64vh; } }
      </style>
      <div class="lr-text"></div>
      <button type="button" class="lr-close">Fold it away</button>`;
    _el.querySelector('.lr-text').innerHTML = text;
    // Reading it is the tutorial's first step.
    gameAPI.sendAction('read_letter', {}).then(() => window.updateQuestTracker?.()).catch(() => {});
    _el.querySelector('.lr-close').addEventListener('click', close);
    document.body.appendChild(_el);
    _stopFit = keepOnParchment(_el.querySelector('.lr-text'));
    document.addEventListener('keydown', onKey, true);
}

window.readLetter = readLetter;
