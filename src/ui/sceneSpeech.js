/**
 * Scene speech box (M6) — the single on-scene narrative surface for BOTH NPC
 * dialogue and POI / encounter nodes. A JRPG-style box pinned to the scene's
 * lower third: an icon + speaker name plate, readable body text, optional amber
 * outcome lines, and optional in-box choice buttons.
 *
 * Two callers, one renderer:
 *   - NPC dialogue passes { speaker, text, portrait:true } and keeps its option
 *     buttons in the bottom action strip (no `buttons` here).
 *   - POI / encounter nodes pass their narrative + `buttons` (choices), since they
 *     have no strip to live in.
 *
 * Styled in the win95 language (dark bevel, Dogica) to match the combat overlay
 * panels it sits beside. Replaces the old top-of-log speech and the centered
 * #poi-modal text box.
 *
 * @module ui/sceneSpeech
 */

const esc = (s) =>
    String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

let _el = null;

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
    scene.appendChild(box);
    _el = box;
    return box;
}

/**
 * Show / refresh the scene speech box.
 * @param {Object}   o
 * @param {string}   o.speaker      - name-plate text (NPC name, or a node title)
 * @param {string}   [o.icon='💬']  - emoji before the name
 * @param {string}   o.text         - body text
 * @param {string[]} [o.outcomes]   - amber result lines (POI/encounter)
 * @param {boolean}  [o.portrait]   - show a framed portrait placeholder (NPCs)
 * @param {Array<{label:string,onClick:Function}>} [o.buttons] - in-box choices; omit for NPC (options live in the strip)
 */
export function showSceneSpeech({ speaker = '', icon = '💬', text = '', outcomes = [], portrait = false, buttons = null } = {}) {
    const box = ensureBox();
    if (!box) return;

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
        <div style="display:flex;gap:6px;padding:6px 8px;max-height:120px;overflow-y:auto;">
            ${portraitHTML}
            <div style="flex:1;min-width:0;">
                <div style="color:#e5e7eb;font-size:10px;line-height:1.5;">${esc(text)}</div>
                ${outcomeHTML}
            </div>
        </div>
        <div id="scene-speech-buttons" style="display:flex;flex-direction:column;gap:2px;padding:0 8px 6px;"></div>
    `;

    const btnWrap = box.querySelector('#scene-speech-buttons');
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
}

/** Hide the speech box (dialogue closed, POI walk ended, or handing off to combat). */
export function hideSceneSpeech() {
    if (_el) _el.style.display = 'none';
}
