/**
 * The inheritance: a new character's starting gear, taken up in game from their
 * people's vault keeper (the tutorial quest, docs/draft/intro-and-tutorial.md).
 *
 * The keeper's "I'd like to see it" line opens this. It lays the class's gear
 * choices (starting-gear.json, unchanged from the old intro) over the keeper's
 * vault: the items that come with the class, one screen per choice, the pack,
 * and "Is that everything?". The picks are claimed with claim_starting_gear,
 * which checks every one against what the class was offered.
 *
 * @module ui/startingGear
 */
import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';
import { NostrCharacterGenerator } from '../logic/characterGenerator.js';
import {
    showGivenItems,
    startEquipmentSelection,
    handlePackSelection,
    getSelectedEquipment,
    toChoicePayload,
} from '../systems/equipmentSelection.js';
import { refreshGameState, getGameStateSync } from '../state/gameState.js';
import { getNPCById } from '../state/staticData.js';
import { smoothClock } from '../systems/smoothClock.js';
import { getAllTracks, playTrack, getPlaybackMode, setPlaybackMode } from '../systems/musicSystem.js';

// The inheritance has its own music: the intro's second track, which the
// shorter intro no longer reaches.
const INHERITANCE_TRACK = 'Cycle of Life';
import { updateAllDisplays } from './displayCoordinator.js';
import { showMessage } from './messaging.js';
import { talkToNPC } from './locationDisplay.js';

const KEEPER_CITY = {
    'royal-custodian': 'kingdom',
    'warden-of-roots': 'verdant',
    'vaultwright': 'ironpeak',
    'keywarden': 'millhaven',
    'hoardkeeper': 'marshlight',
    'scalekeeper': 'goldenhaven',
};

let _open = false;

/** The keeper's vault art, or the town's while a vault has none yet. */
async function backdropFor(city) {
    const tryLoad = (src) => new Promise((resolve) => {
        const img = new Image();
        img.onload = () => resolve(src);
        img.onerror = () => resolve(null);
        img.src = src;
    });
    return (await tryLoad(`/res/img/scene/vault-${city}.png`))
        || (await tryLoad(`/res/img/locations/${city}.png`))
        || '';
}

function buildOverlay(backdrop, keeperName) {
    const el = document.createElement('div');
    el.id = 'gear-overlay';
    el.innerHTML = `
        <div id="gear-container" class="gear-scene hidden">
            <div id="gear-background" class="gear-bg"></div>
            <div id="gear-content" class="gear-panel text-center"></div>
        </div>
        <div class="gear-speaker">${keeperName ? `🗝️ ${keeperName}` : '🗝️ Your inheritance'}</div>
        <button type="button" class="gear-leave" title="Come back for it later">Not now ✕</button>`;
    document.body.appendChild(el);
    const bg = el.querySelector('#gear-background');
    if (backdrop) bg.style.backgroundImage = `url(${backdrop})`;
    return el;
}

/**
 * Open the inheritance.
 * @param {{keeper?: string, skip?: boolean}} opts - who's handing it over, and
 *   whether this is "I've done this before" (claim, then the tutorial ends)
 */
export async function openStartingGear({ keeper = '', skip = false } = {}) {
    if (_open) return;
    _open = true;
    const wasRunning = smoothClock.isRunning?.() ?? false;
    if (wasRunning) window.timeClock?.pause?.();

    const state = getGameStateSync();
    const character = state?.character ?? {};
    const city = KEEPER_CITY[keeper] || state?.location?.current || 'kingdom';
    let keeperName = '';
    try {
        keeperName = (await getNPCById(keeper))?.name ?? '';
    } catch (_) { /* name is decoration */ }

    let overlay = null;
    let backToKeeper = false; // the conversation picks up again when the screens close
    let heldMusic = false;
    try {
        const gear = await (await fetch('/api/starting-gear')).json();
        const gen = new NostrCharacterGenerator();
        gen.startingGear = gear;
        const equipment = gen.generateStartingEquipment({ class: character.class });

        overlay = buildOverlay(await backdropFor(city), keeperName);
        // Hold the town's music off (manual mode, so a location refresh can't
        // switch back) and play the inheritance's own track.
        const track = getAllTracks().find((t) => t.title === INHERITANCE_TRACK);
        if (track && getPlaybackMode() === 'auto') {
            heldMusic = true;
            setPlaybackMode('manual');
            playTrack({ ...track, unlocked: true }, true);
        }
        const leave = new Promise((resolve) => {
            overlay.querySelector('.gear-leave').addEventListener('click', () => resolve('left'));
        });

        // Walk the screens; "Not now" can end it at any point.
        const flow = (async () => {
            for (;;) {
                await showGivenItems(equipment.inventory);
                await startEquipmentSelection({ choices: equipment.choices });
                await handlePackSelection(equipment);
                const payload = toChoicePayload(getSelectedEquipment());
                const result = await gameAPI.sendAction('claim_starting_gear', { ...payload, skip });
                if (result.success) return result;
                // A refused pick (shouldn't happen from these screens): say why and go again.
                showMessage(result.message || 'That didn\'t work. Let\'s try again.', 'warning');
            }
        })();

        const outcome = await Promise.race([flow, leave]);
        backToKeeper = true;
        if (outcome === 'left') {
            showMessage(`${keeperName || 'The keeper'} will hold it for you. Come back when you're ready.`, 'info');
            return;
        }
        await refreshGameState();
        await updateAllDisplays();
        showMessage(outcome.message, 'success');
        window.updateQuestTracker?.();
    } catch (err) {
        logger.error('Inheritance failed:', err);
        showMessage(`Couldn't open your inheritance: ${err.message}`, 'error');
    } finally {
        overlay?.remove();
        // The selection screens put their Back button and item pop-up on the page.
        document.getElementById('item-modal-backdrop')?.remove();
        document.querySelectorAll('.pixel-back-btn').forEach((b) => b.remove());
        _open = false;
        if (heldMusic) setPlaybackMode('auto'); // back to the town's music
        if (wasRunning) window.timeClock?.play?.();
        // Back to the keeper you were talking to, not just the vault.
        if (backToKeeper && keeper) talkToNPC(keeper);
    }
}

window.openStartingGear = openStartingGear;
