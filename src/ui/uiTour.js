/**
 * "Show me around": a guided tour of the game screen. It spotlights each part
 * of the UI in turn — the header, every tab, the scene, the action bar and the
 * text box — with a line on what it is and what it's for.
 *
 * It runs once for a brand-new character (no XP yet, day 1) and any time from
 * Settings → Show me around. Time stops while it runs and picks up again after.
 * Whether this browser has seen it is a per-browser convenience (localStorage),
 * not game state, so it never touches the save.
 *
 * The tour lives inside #game-window and works in its unscaled 756×503 space,
 * so the spotlight and the card scale with the game (wide or tall screens
 * stretch the window) and the card can never land off the game screen.
 *
 * @module ui/uiTour
 */
import { eventBus } from '../lib/events.js';
import { smoothClock } from '../systems/smoothClock.js';

const SEEN_KEY = 'pq_ui_tour_seen';
const AFTER_LOAD_MS = 2200; // after the loading fade and the what's-new check
const CARD_W = 230;         // in game-window pixels
const EDGE = 4;             // keep the card this far inside the window

/**
 * The stops, in screen order. `sel` is what gets the spotlight; a stop whose
 * element isn't showing (a hidden chip, a tab a class doesn't have) is skipped.
 */
const STEPS = [
    { sel: null, title: 'Welcome', text: 'A quick look around the screen: what each part is for. It takes about a minute. You can leave any time, and Settings → Show me around brings it back.' },

    // Header
    { sel: '#char-portrait-thumb', title: 'Your portrait', text: 'Until you make one, your crest stands in: it\'s drawn from your key, so no one else has it. Click to make a portrait.' },
    { sel: '.pq-name-row', title: 'Name and level', text: 'Click your level for the level guide: what each level brings, and when.' },
    { sel: '.pq-clock-row', title: 'Time', text: 'The sun or moon, the time and the day. Time passes slowly in town and faster on the road. The game pauses itself when something needs you (a fight, a find, a conversation) and runs again after. ▶ / ⏸ starts and stops it yourself; 🕐 lets hours pass on purpose, to rest or wait for a shop to open. Shops and people keep their hours, and nights are more dangerous.' },
    { sel: '#exp-bar', parent: true, title: 'Experience', text: 'XP toward your next level, from fights, quests and discoveries. Click it for the level guide.' },
    { sel: '#hp-bar', parent: true, title: 'Health', text: 'At 0 in a fight you fall and make death saves. Rest, potions and healing bring it back.' },
    { sel: '#mana-bar', parent: true, title: 'Your class resource', text: 'Mana for spellcasters: every spell costs mana (and some, runes). Fighters have Stamina, barbarians Rage, monks Ki and rogues Cunning: the fuel for class abilities in combat.' },
    { sel: '#fatigue-icon-container', title: 'Fatigue', text: '0 is fresh, 10 is exhausted. It climbs as the hours pass, faster on the road, and a tired hero fights worse. Waiting holds it where it is; sleep clears it, in an inn bed or camped out on the road. Inns serve breakfast, so a night in one also wakes you Well Fed.' },
    { sel: '#hunger-icon-container', title: 'Hunger', text: 'Stuffed, Well Fed, Hungry, Starving. It drops every few hours: eat from your bag (right-click food → Eat). Starving hurts.' },
    { sel: '#char-weight', parent: true, title: 'Weight', text: 'What you carry, out of what you can carry. Strength and a backpack raise the limit; going over slows and weakens you.' },
    { sel: '#active-effects-display', title: 'Effects', text: 'Buffs and ailments on you right now. Hover one for what it does and how long it lasts.' },

    // Top tabs
    { sel: '#tab-questlog', title: 'Quest Log', text: 'Your quests, the step you\'re on and where to go next. The tracker over the scene shows the current step at a glance.' },
    { sel: '#tab-stats', title: 'Stats', text: 'Ability scores, skills, saving throws and the feats you\'ve taken, plus the full detail of every effect on you: what it changes and how long it lasts.' },
    { sel: '#tab-inventory', title: 'Inventory', text: 'Your gold and your bag. The 4 general slots are your belt: they\'re what you can reach in a fight. The darker slots are inside the backpack you\'re wearing; take the backpack off and it goes into a general slot with everything still inside. Right-click (or long-press) an item for what you can do with it: equip, eat, use, read, drop.' },
    { sel: '#tab-equipment', title: 'Equipment', text: 'What you\'re wearing and wielding, and what it does for your armor and damage.' },
    { sel: '#tab-spells', title: 'Spells & Abilities', text: 'Spellcasters: the spells you know, which are prepared, and casting outside a fight. Everyone else: your class abilities, what they do and what they cost.' },

    // Bottom row
    { sel: '#btn-friends', title: 'Friends', text: 'Other players: friends, whispers, and forming a party.' },
    { sel: '#tab-music', title: 'Music', text: 'Every town has its own music. The tracks you\'ve heard on your travels are here to play again.' },
    { sel: '#btn-settings', title: 'Settings', text: 'Save, reload your last save, quit, and text speed. This tour lives here too.' },
    { sel: 'button[title="Report Bug"]', title: 'Report a bug', text: 'Something broken or odd? Tell us here. It sends what went wrong along with the game\'s state, which helps a lot.' },
    { sel: 'button[title="Quit"]', title: 'Quit', text: 'Leave the game. It asks first whether to save.' },

    // Scene
    { sel: '#city-name', title: 'Where you are', text: 'The town (or road) and, below it, the district. Each town has a few districts with their own buildings and people.' },
    { sel: '#headcount-chip', title: 'Who\'s here', text: 'Other players in the same place as you. Click to see who.' },
    { sel: '#quest-tracker', title: 'Quest tracker', text: 'Your current quest step. Click it to open the journal. A 💡 beside it explains how that step works, whenever you want it.' },

    // Action bar
    { sel: '#navigation-buttons', title: 'Travel', text: 'Walk between districts by compass direction. Gates at the edge of town lead onto the roads, where time passes faster and the wilds are dangerous.' },
    { sel: '#building-buttons', title: 'Buildings', text: 'Shops, inns, temples and your people\'s vault. Some keep hours and close at night.' },
    { sel: '#npc-buttons', title: 'People', text: 'Who\'s here to talk to. They have homes and schedules, so who you find depends on the hour.' },

    // Text box
    { sel: '#game-text', title: 'The story', text: 'What happens, as it happens: what people say, what you find, how a fight goes.' },
    { sel: '#chat-row', title: 'Chat', text: 'Talk to players nearby. Click the channel to switch between Say (this spot), Area (this town) and Party.' },

    { sel: '#manual-save-btn', title: 'Saving', openSettings: true, text: 'There\'s no autosave. Save when you\'re somewhere safe (Settings → Save game, or Ctrl+S): your save is signed with your own key, so it\'s yours. Close without saving and you go back to your last save.' },
    { sel: null, title: 'That\'s the tour', text: 'Good luck out there. Settings → Show me around brings this back any time.' },
];

let _root = null;
let _host = null;
let _i = 0;
let _steps = [];
let _wasRunning = false;

function esc(s) {
    return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

function targetOf(step) {
    if (!step.sel) return null;
    const el = document.querySelector(step.sel);
    if (!el) return null;
    const t = step.parent ? el.parentElement : el;
    const r = t?.getBoundingClientRect();
    return r && r.width > 0 && r.height > 0 ? t : null;
}

/**
 * The host's frame: its size in its own (unscaled) pixels, and a function
 * turning a screen rect into those pixels. #game-window is scaled with a CSS
 * transform, so screen rects are divided back down by that scale.
 */
function frame() {
    if (_host === document.body) {
        return { w: window.innerWidth, h: window.innerHeight, toLocal: (r) => r };
    }
    const hr = _host.getBoundingClientRect();
    const w = _host.offsetWidth, h = _host.offsetHeight;
    const sx = hr.width / w || 1, sy = hr.height / h || 1;
    return {
        w, h,
        toLocal: (r) => ({
            left: (r.left - hr.left) / sx, top: (r.top - hr.top) / sy,
            width: r.width / sx, height: r.height / sy,
            right: (r.right - hr.left) / sx, bottom: (r.bottom - hr.top) / sy,
        }),
    };
}

function setSeen() {
    try { localStorage.setItem(SEEN_KEY, '1'); } catch (_) { /* storage blocked */ }
}

function hasSeen() {
    try { return localStorage.getItem(SEEN_KEY) === '1'; } catch (_) { return true; }
}

/** End the tour, put time back how it was. */
export function endUITour() {
    _root?.remove();
    _root = null;
    window.removeEventListener('resize', render);
    document.removeEventListener('keydown', onKey, true);
    setSeen();
    window.pqTourPending = false; // tutorial tips may show now
    if (_wasRunning) window.timeClock?.play?.();
}

function onKey(e) {
    if (!_root) return;
    if (e.key === 'Escape') { e.preventDefault(); endUITour(); }
    else if (e.key === 'ArrowRight' || e.key === 'Enter') { e.preventDefault(); go(1); }
    else if (e.key === 'ArrowLeft') { e.preventDefault(); go(-1); }
}

function go(d) {
    const n = _i + d;
    if (n < 0) return;
    if (n >= _steps.length) { endUITour(); return; }
    _i = n;
    render();
}

function render() {
    if (!_root) return;
    const step = _steps[_i];
    if (step.openSettings) window.openSettings?.();
    const el = targetOf(step);
    const spot = _root.querySelector('.tour-spot');
    const card = _root.querySelector('.tour-card');
    const pad = 3;
    const f = frame();

    let r = null;
    if (el) {
        el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        r = f.toLocal(el.getBoundingClientRect());
        Object.assign(spot.style, {
            display: 'block',
            left: `${r.left - pad}px`, top: `${r.top - pad}px`,
            width: `${r.width + pad * 2}px`, height: `${r.height + pad * 2}px`,
        });
        _root.style.background = 'transparent'; // the spotlight's shadow dims the rest
    } else {
        spot.style.display = 'none';
        _root.style.background = 'rgba(0,0,0,0.72)';
    }

    const last = _i === _steps.length - 1;
    card.innerHTML = `
        <div class="tour-head"><span>🧭 ${esc(step.title)}</span><span class="tour-count">${_i + 1} / ${_steps.length}</span></div>
        <div class="tour-text">${esc(step.text)}</div>
        <div class="tour-btns">
            <button class="tour-btn alt" data-act="skip">${last ? 'Close' : 'Skip tour'}</button>
            <span style="flex:1"></span>
            ${_i > 0 ? '<button class="tour-btn alt" data-act="back">◀ Back</button>' : ''}
            <button class="tour-btn" data-act="next">${last ? 'Done' : _i === 0 ? 'Start ▶' : 'Next ▶'}</button>
        </div>`;

    // Place the card beside the spotlight: below it if there's room, else
    // above, else to the side, else centred; always inside the window.
    const cw = Math.min(CARD_W, f.w - EDGE * 2);
    card.style.width = `${cw}px`;
    const ch = card.offsetHeight;
    const clampX = (x) => Math.min(Math.max(EDGE, x), f.w - cw - EDGE);
    const clampY = (y) => Math.min(Math.max(EDGE, y), f.h - ch - EDGE);
    let left, top;
    if (r) {
        const gap = pad + 4;
        if (r.bottom + gap + ch <= f.h - EDGE) {
            left = clampX(r.left + r.width / 2 - cw / 2); top = r.bottom + gap;
        } else if (r.top - gap - ch >= EDGE) {
            left = clampX(r.left + r.width / 2 - cw / 2); top = r.top - gap - ch;
        } else if (r.left - gap - cw >= EDGE) {
            left = r.left - gap - cw; top = clampY(r.top + r.height / 2 - ch / 2);
        } else if (r.right + gap + cw <= f.w - EDGE) {
            left = r.right + gap; top = clampY(r.top + r.height / 2 - ch / 2);
        } else {
            left = clampX((f.w - cw) / 2); top = clampY((f.h - ch) / 2);
        }
    } else {
        left = clampX((f.w - cw) / 2);
        top = clampY((f.h - ch) / 2);
    }
    card.style.left = `${left}px`;
    card.style.top = `${top}px`;
    card.querySelector('[data-act="next"]')?.focus({ preventScroll: true });
}

/** Start the tour from the first stop. */
export function startUITour() {
    if (_root) endUITour();
    _steps = STEPS.filter((s) => !s.sel || s.openSettings || targetOf(s));
    _i = 0;
    _wasRunning = smoothClock.isRunning?.() ?? false;
    if (_wasRunning) window.timeClock?.pause?.();

    _host = document.getElementById('game-window') || document.body;
    const pos = _host === document.body ? 'fixed' : 'absolute';
    _root = document.createElement('div');
    _root.id = 'ui-tour';
    _root.innerHTML = `
      <style>
        #ui-tour { position: ${pos}; inset: 0; z-index: 9000; overflow: hidden; }
        #ui-tour .tour-spot { position: absolute; border: 1px solid #facc15; box-shadow: 0 0 0 9999px rgba(0,0,0,0.72), 0 0 6px #facc15; pointer-events: none; transition: all 0.2s ease; }
        #ui-tour .tour-card { position: absolute; box-sizing: border-box; background: #1f1f1f; color: #e5e7eb; font-family: 'Dogica', monospace; font-size: 7px; line-height: 1.6;
          border-top: 2px solid #6b7280; border-left: 2px solid #6b7280; border-right: 2px solid #000; border-bottom: 2px solid #000; box-shadow: 0 2px 8px rgba(0,0,0,0.6); }
        #ui-tour .tour-head { display: flex; align-items: center; gap: 4px; padding: 2px 4px; background: linear-gradient(90deg, #854d0e, #a16207); color: #fff; font-weight: bold; }
        #ui-tour .tour-count { margin-left: auto; color: #fde68a; font-weight: normal; }
        #ui-tour .tour-text { padding: 5px 6px; }
        #ui-tour .tour-btns { display: flex; gap: 3px; padding: 0 6px 6px; }
        #ui-tour .tour-btn { font: inherit; color: #fff; cursor: pointer; padding: 2px 6px; background: #0e7490; border: 2px solid; border-color: #22d3ee #155e75 #155e75 #22d3ee; }
        #ui-tour .tour-btn.alt { background: #3f3f46; border-color: #71717a #27272a #27272a #71717a; }
      </style>
      <div class="tour-spot"></div>
      <div class="tour-card" role="dialog" aria-label="Tour of the game screen"></div>`;
    _host.appendChild(_root);
    _root.addEventListener('click', (e) => {
        const act = e.target.closest('[data-act]')?.dataset.act;
        if (act === 'next') go(1);
        else if (act === 'back') go(-1);
        else if (act === 'skip') endUITour();
    });
    window.addEventListener('resize', render);
    document.addEventListener('keydown', onKey, true);
    render();
}

// A brand-new character gets the tour once, when nothing else is on screen
// (the first-save prompt and the what's-new popup go first).
eventBus.on('gameStateLoaded', (state) => {
    const ch = state?.character;
    if (!ch || hasSeen() || (ch.experience ?? 0) > 0 || (ch.current_day ?? 1) > 1) return;
    window.pqTourPending = true; // tutorial tips wait for the tour
    let tries = 0;
    const tryStart = () => {
        const busy = document.getElementById('whats-new')
            || !document.getElementById('save-modal')?.classList.contains('hidden');
        if (busy && tries++ < 240) { setTimeout(tryStart, 500); return; }
        if (!busy) startUITour();
        else window.pqTourPending = false;
    };
    setTimeout(tryStart, AFTER_LOAD_MS);
});

window.startUITour = startUITour;
