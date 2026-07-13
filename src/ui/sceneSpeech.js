/**
 * Scene speech box (M6) — the single on-scene narrative surface for BOTH NPC
 * dialogue and POI / encounter nodes. A JRPG-style box pinned to the scene's
 * lower third: an icon + speaker name plate, readable body text, optional amber
 * outcome lines, and optional in-box choice buttons.
 *
 * The body text types out letter-by-letter at a Settings-tunable speed. Clicking
 * the box while it's typing skips to the full text. Choices/outcomes (and, for
 * NPCs, the option strip via `onReady`) are withheld until the text finishes, so
 * you read the line before you're asked to respond.
 *
 * Two callers, one renderer:
 *   - NPC dialogue passes { speaker, text, portrait:true, onReady } and reveals
 *     its option strip in onReady; no `buttons` here.
 *   - POI / encounter nodes pass their narrative + `buttons` (choices).
 *
 * @module ui/sceneSpeech
 */

const esc = (s) =>
    String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

// Text-speed presets → milliseconds per character. Tunable from Settings → Text.
const SPEED_MS = { slow: 55, normal: 28, fast: 12, instant: 0 };
const DEFAULT_SPEED = 'normal';

/** The saved text-speed preset ('slow'|'normal'|'fast'|'instant'). */
export function getTextSpeed() {
    const pref = localStorage.getItem('textSpeed');
    return SPEED_MS[pref] !== undefined ? pref : DEFAULT_SPEED;
}

/** ms-per-character for the current text-speed preference. */
function speedMs() {
    return SPEED_MS[getTextSpeed()];
}

/** Persist the dialogue text-speed preference (called from the Settings tab). */
export function setTextSpeed(preset) {
    if (SPEED_MS[preset] === undefined) return;
    localStorage.setItem('textSpeed', preset);
}

let _el = null;
let _timer = null; // active type-out interval
let _typing = false; // true while a type-out is animating
let _finish = null; // completes the current type-out immediately (skip)

function clearTimer() {
    if (_timer) {
        clearInterval(_timer);
        _timer = null;
    }
}

/** Lazily create (once) the speech box as a lower-third child of #scene-container. */
function ensureBox() {
    if (_el && document.body.contains(_el)) return _el;
    const scene = document.getElementById('scene-container');
    if (!scene) return null;
    const box = document.createElement('div');
    box.id = 'scene-speech';
    box.style.cssText = [
        'position:absolute', 'left:0', 'right:0', 'bottom:0', 'z-index:20',
        'display:none', 'pointer-events:auto',
        'background:rgba(10,10,10,0.92)',
        'border-top:2px solid #4a4a4a', 'border-bottom:2px solid #0a0a0a',
        'box-shadow:0 -4px 16px rgba(0,0,0,0.6)',
        'font-family:"Dogica", monospace',
    ].join(';');
    // Click the box while it's typing to skip to the full text.
    box.addEventListener('click', () => {
        if (_typing && _finish) _finish();
    });
    scene.appendChild(box);
    _el = box;
    return box;
}

/**
 * Show / refresh the scene speech box.
 * @param {Object}   o
 * @param {string}   o.speaker      - name-plate text (NPC name, or a node title)
 * @param {string}   [o.icon='💬']  - emoji before the name
 * @param {string}   o.text         - body text (types out)
 * @param {string[]} [o.outcomes]   - amber result lines (POI/encounter), shown once typed
 * @param {boolean}  [o.portrait]   - show a framed portrait placeholder (NPCs)
 * @param {Array<{label:string,onClick:Function}>} [o.buttons] - in-box choices; withheld until typed
 * @param {Function} [o.onReady]    - fired when the text finishes (or is skipped); NPC reveals its strip here
 */
export function showSceneSpeech({ speaker = '', icon = '💬', text = '', outcomes = [], portrait = false, buttons = null, onReady = null } = {}) {
    const box = ensureBox();
    if (!box) return;
    clearTimer();
    _typing = false;
    _finish = null;

    const outcomeHTML = (outcomes || [])
        .map((o) => `<div style="color:#fcd34d;font-size:9px;margin-top:2px;">${esc(o)}</div>`)
        .join('');

    const initial = esc((String(speaker).trim()[0] || '?').toUpperCase());
    const portraitHTML = portrait
        ? `<div style="width:40px;height:40px;flex-shrink:0;display:flex;align-items:center;justify-content:center;
                image-rendering:pixelated;background:#1a1a1a;color:#fde047;font-size:16px;font-weight:bold;
                border-top:1px solid #4a4a4a;border-left:1px solid #4a4a4a;
                border-right:1px solid #0a0a0a;border-bottom:1px solid #0a0a0a;">${initial}</div>`
        : '';

    box.innerHTML = `
        <div style="display:flex;align-items:center;gap:4px;padding:2px 6px;
                    background:linear-gradient(90deg,#1a1a1a,#2a2a2a);border-bottom:1px solid #0a0a0a;">
            <span style="font-size:10px;">${icon}</span>
            <span style="color:#fde047;font-size:9px;font-weight:bold;text-transform:uppercase;
                         white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${esc(speaker)}</span>
        </div>
        <div id="scene-speech-body" style="display:flex;gap:6px;padding:6px 8px;max-height:120px;overflow-y:auto;">
            ${portraitHTML}
            <div style="flex:1;min-width:0;">
                <div id="scene-speech-text" style="color:#e5e7eb;font-size:10px;line-height:1.5;white-space:pre-wrap;"></div>
                <div id="scene-speech-outcome" style="display:none;">${outcomeHTML}</div>
            </div>
        </div>
        <div id="scene-speech-buttons" style="display:none;flex-direction:column;gap:2px;padding:0 8px 6px;"></div>
    `;

    const bodyEl = box.querySelector('#scene-speech-body');
    const textEl = box.querySelector('#scene-speech-text');
    const outEl = box.querySelector('#scene-speech-outcome');
    const btnWrap = box.querySelector('#scene-speech-buttons');

    // Build the choice buttons up front, but keep them hidden until the text lands.
    if (btnWrap && buttons && buttons.length) {
        buttons.forEach((b) => {
            const btn = document.createElement('button');
            btn.textContent = b.label;
            btn.style.cssText = [
                'width:100%', 'text-align:left', 'color:#fff', 'font-size:9px', 'font-weight:bold',
                'padding:3px 6px', 'cursor:pointer', 'background:#2a2a2a',
                'border-top:1px solid #4a4a4a', 'border-left:1px solid #4a4a4a',
                'border-right:1px solid #0a0a0a', 'border-bottom:1px solid #0a0a0a',
            ].join(';');
            btn.addEventListener('click', b.onClick);
            btnWrap.appendChild(btn);
        });
    }

    box.style.display = 'block';

    // reveal() completes the type-out: full text, then outcomes, choices, and the
    // caller's onReady (which surfaces the NPC option strip). Idempotent.
    let done = false;
    const reveal = () => {
        if (done) return;
        done = true;
        clearTimer();
        textEl.textContent = text;
        if (outcomeHTML) outEl.style.display = '';
        if (buttons && buttons.length) btnWrap.style.display = 'flex';
        _typing = false;
        _finish = null;
        box.style.cursor = 'default';
        if (typeof onReady === 'function') onReady();
    };

    // Instant speed (or no text) → skip the animation entirely.
    if (!text || speedMs() <= 0) {
        reveal();
        return;
    }

    _typing = true;
    _finish = reveal;
    box.style.cursor = 'pointer';
    let i = 0;
    _timer = setInterval(() => {
        i += 1;
        textEl.textContent = text.slice(0, i);
        if (bodyEl) bodyEl.scrollTop = bodyEl.scrollHeight; // keep the newest text in view
        if (i >= text.length) reveal();
    }, speedMs());
}

/** Hide the speech box (dialogue closed, POI walk ended, or handing off to combat). */
export function hideSceneSpeech() {
    clearTimer();
    _typing = false;
    _finish = null;
    if (_el) _el.style.display = 'none';
}
