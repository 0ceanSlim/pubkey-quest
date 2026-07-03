/**
 * Known-spells picker — a sortable, filterable table of the player's known
 * spells, opened from an empty spell slot. Click a row to open the full spell
 * info modal (which prepares it). This is the player's spellbook view.
 *
 * @module ui/spellPicker
 */

import { logger } from '../lib/logger.js';
import { getSpellById } from '../state/staticData.js';
import { getGameStateSync, refreshGameState } from '../state/gameState.js';
import { getDamageEmoji } from '../lib/damageTypes.js';
import { gameAPI } from '../lib/api.js';
import { showActionText } from './messaging.js';

// Sort + filter state persists across opens so the player's view sticks.
let sortKey = 'name';
let sortDir = 1; // 1 asc, -1 desc
let filterText = '';
let filterSchool = '';
let filterType = '';
let openContext = null; // { slotLevel, slotIndex } the picker was opened from

const TYPE_TAGS = { damage: 'damage', heal: 'healing', buff: 'buff', utility: 'utility' };

/**
 * Open the picker over the game area. slotLevel/slotIndex is the slot the player
 * clicked — passed through to Prepare as the preferred target.
 */
export function openSpellPicker(slotLevel = null, slotIndex = null) {
    openContext = (slotLevel != null) ? { slotLevel, slotIndex } : null;
    closeSpellPicker();

    const scene = document.getElementById('scene-container') || document.body;
    const overlay = document.createElement('div');
    overlay.id = 'spell-picker-overlay';
    overlay.style.cssText = 'position:absolute; inset:0; z-index:120; background:rgba(0,0,0,0.9); display:flex; align-items:center; justify-content:center;';
    overlay.addEventListener('click', (e) => { if (e.target === overlay) closeSpellPicker(); });

    const panel = document.createElement('div');
    panel.style.cssText = 'width:96%; max-width:520px; max-height:92%; display:flex; flex-direction:column; background:#14141f; border-top:2px solid #3a3a5a; border-left:2px solid #3a3a5a; border-right:2px solid #05050a; border-bottom:2px solid #05050a; padding:8px;';
    overlay.appendChild(panel);

    // Header (scoped to the clicked slot's level)
    const lvlLabel = slotLevel === 'cantrips' ? 'Cantrips'
        : (slotLevel ? `Level ${slotLevel.split('_')[1]} Spells` : 'Spells');
    const header = document.createElement('div');
    header.className = 'flex justify-between items-center mb-2';
    header.innerHTML = `<span class="text-yellow-400 font-bold" style="font-size:11px;">📖 Known ${lvlLabel}</span>`;
    const closeBtn = document.createElement('button');
    closeBtn.textContent = 'Close';
    closeBtn.className = 'text-white px-2 py-1 font-bold';
    closeBtn.style.cssText = 'background:#dc2626;border-top:2px solid #ef4444;border-left:2px solid #ef4444;border-right:2px solid #991b1b;border-bottom:2px solid #991b1b;font-size:10px;';
    closeBtn.onclick = closeSpellPicker;
    header.appendChild(closeBtn);
    panel.appendChild(header);

    // Filter bar
    panel.appendChild(buildFilterBar());

    // Scrollable table container
    const tableWrap = document.createElement('div');
    tableWrap.id = 'spell-picker-table';
    tableWrap.style.cssText = 'overflow-y:auto; flex:1; min-height:0;';
    panel.appendChild(tableWrap);

    scene.appendChild(overlay);
    renderTable();
}

/** Prepare the chosen spell into the picker's origin slot (or first free slot of its level). */
async function prepareFromPicker(spell) {
    const slotLevel = spell.level === 0 ? 'cantrips' : `level_${spell.level}`;
    const levelLabel = spell.level === 0 ? 'cantrip' : `level ${spell.level}`;
    const slots = getGameStateSync()?.character?.spell_slots?.[slotLevel];
    if (!Array.isArray(slots) || slots.length === 0) {
        showActionText(`You have no ${levelLabel} slots`, 'red', 3000);
        return;
    }

    let target = null;
    if (openContext && openContext.slotLevel === slotLevel && openContext.slotIndex != null) {
        target = openContext.slotIndex;
    } else {
        const preparing = new Set();
        try { const q = await gameAPI.getPrepQueue(); (q.tasks || []).forEach(t => { if (t.slot_level === slotLevel) preparing.add(t.slot_index); }); } catch { /* ignore */ }
        const free = slots.find(s => !s.spell && !preparing.has(s.slot));
        if (!free) { showActionText(`No free ${levelLabel} slot`, 'red', 3000); return; }
        target = free.slot;
    }

    try {
        const res = await gameAPI.prepareSpell(spell.id, slotLevel, target);
        showActionText(res.ready_in_minutes > 0 ? `Preparing ${spell.name} — ${res.ready_in_minutes} min` : `${spell.name} prepared`,
            res.ready_in_minutes > 0 ? 'yellow' : 'green', 3000);
        closeSpellPicker();
        await refreshGameState(true);
        if (window.updateSpellsDisplay) window.updateSpellsDisplay();
    } catch (err) {
        showActionText(`Cannot prepare: ${err.message}`, 'red', 3500);
    }
}

export function closeSpellPicker() {
    const el = document.getElementById('spell-picker-overlay');
    if (el) el.remove();
}

function knownSpells() {
    const chars = getGameStateSync()?.character;
    const ids = Array.isArray(chars?.spells) ? chars.spells : [];
    return ids.map(id => getSpellById(id)).filter(Boolean);
}

function buildFilterBar() {
    const bar = document.createElement('div');
    bar.className = 'flex flex-wrap items-center gap-1 mb-2';
    bar.style.fontSize = '8px';

    const search = document.createElement('input');
    search.type = 'text';
    search.placeholder = 'Search…';
    search.value = filterText;
    search.style.cssText = 'flex:1; min-width:80px; background:#0d0d14; color:#fff; border:1px solid #333; padding:2px 4px; font-size:8px;';
    search.oninput = () => { filterText = search.value.toLowerCase(); renderTable(); };
    bar.appendChild(search);

    const schools = Array.from(new Set(knownSpells().map(s => s.school))).sort();
    bar.appendChild(makeSelect(['', ...schools], filterSchool, 'School', v => { filterSchool = v; renderTable(); }));
    bar.appendChild(makeSelect(['', 'damage', 'heal', 'buff', 'utility'], filterType, 'Type', v => { filterType = v; renderTable(); }));

    return bar;
}

function makeSelect(options, current, label, onChange) {
    const sel = document.createElement('select');
    sel.style.cssText = 'background:#0d0d14; color:#fff; border:1px solid #333; padding:2px; font-size:8px;';
    options.forEach(o => {
        const opt = document.createElement('option');
        opt.value = o;
        opt.textContent = o === '' ? `All ${label}` : (o.charAt(0).toUpperCase() + o.slice(1));
        if (o === current) opt.selected = true;
        sel.appendChild(opt);
    });
    sel.onchange = () => onChange(sel.value);
    return sel;
}

// Columns: key, label, value(spell) for sort + render.
const COLUMNS = [
    { key: 'name', label: 'Name', val: s => s.name },
    { key: 'school', label: 'School', val: s => s.school },
    { key: 'mana', label: 'MP', val: s => s.mana_cost || 0 },
    { key: 'effect', label: 'Effect', val: s => s.damage || s.heal || '' },
    { key: 'comp', label: 'Comp', val: s => (s.material_component?.required?.length || 0) },
];

/** The spell level this picker is scoped to (from the origin slot), or null. */
function targetLevel() {
    if (!openContext || !openContext.slotLevel) return null;
    return openContext.slotLevel === 'cantrips' ? 0 : parseInt(openContext.slotLevel.split('_')[1]);
}

function renderTable() {
    const wrap = document.getElementById('spell-picker-table');
    if (!wrap) return;

    let rows = knownSpells();
    const lvl = targetLevel();
    if (lvl != null) rows = rows.filter(s => s.level === lvl); // scoped to the slot's level
    if (filterText) rows = rows.filter(s => s.name.toLowerCase().includes(filterText));
    if (filterSchool) rows = rows.filter(s => s.school === filterSchool);
    if (filterType) rows = rows.filter(s => spellHasType(s, filterType));

    const col = COLUMNS.find(c => c.key === sortKey) || COLUMNS[1];
    rows.sort((a, b) => {
        let cmp = compare(col.val(a), col.val(b));
        if (cmp === 0 && sortKey !== 'name') cmp = compare(a.name, b.name);
        return cmp * sortDir;
    });

    const slotted = slottedIds();

    const table = document.createElement('table');
    table.style.cssText = 'width:100%; border-collapse:collapse; font-size:8px;';

    // Header row (click to sort)
    const thead = document.createElement('tr');
    COLUMNS.forEach(c => {
        const th = document.createElement('th');
        th.style.cssText = 'text-align:left; color:#facc15; padding:3px 4px; cursor:pointer; position:sticky; top:0; background:#14141f; border-bottom:1px solid #333; white-space:nowrap;';
        th.textContent = c.label + (sortKey === c.key ? (sortDir === 1 ? ' ▲' : ' ▼') : '');
        th.onclick = () => {
            if (sortKey === c.key) sortDir = -sortDir; else { sortKey = c.key; sortDir = 1; }
            renderTable();
        };
        thead.appendChild(th);
    });
    table.appendChild(thead);

    rows.forEach((s, i) => {
        const tr = document.createElement('tr');
        tr.style.cssText = `cursor:pointer; ${i % 2 ? 'background:rgba(255,255,255,0.03);' : ''}`;
        tr.onmouseenter = () => { tr.style.background = 'rgba(255,255,255,0.10)'; };
        tr.onmouseleave = () => { tr.style.background = i % 2 ? 'rgba(255,255,255,0.03)' : 'transparent'; };
        // Click a row → open its info (the picker stays open behind it); Prepare
        // from the info modal. Browse freely without losing the list.
        tr.onclick = () => {
            if (window.openSpellModal) {
                window.openSpellModal(s.id, { mode: 'prepare', slotLevel: openContext?.slotLevel, slotIndex: openContext?.slotIndex });
            }
        };
        tr.title = 'Click for info, then Prepare';

        tr.appendChild(nameCell(s, slotted.has(s.id)));
        tr.appendChild(td(cap(s.school)));
        tr.appendChild(td(s.mana_cost || '—', '#93c5fd'));
        tr.appendChild(effectCell(s));
        tr.appendChild(compCell(s));
        table.appendChild(tr);
    });

    wrap.innerHTML = '';
    if (rows.length === 0) {
        wrap.innerHTML = '<div class="text-gray-500 italic" style="font-size:9px; padding:8px;">No spells match.</div>';
    } else {
        wrap.appendChild(table);
    }
}

function nameCell(spell, isPrepared) {
    const cell = document.createElement('td');
    cell.style.cssText = 'padding:3px 4px;';
    const wrap = document.createElement('div');
    wrap.className = 'flex items-center gap-1';
    if (isPrepared) {
        const dot = document.createElement('span');
        dot.textContent = '●';
        dot.className = 'text-purple-400';
        dot.title = 'Prepared';
        dot.style.fontSize = '7px';
        wrap.appendChild(dot);
    }
    const icon = document.createElement('img');
    icon.src = `/res/img/spells/${spell.school}.png`;
    icon.style.cssText = 'width:14px; height:14px; image-rendering:pixelated; flex-shrink:0;';
    icon.onerror = () => { icon.style.display = 'none'; };
    wrap.appendChild(icon);
    const nm = document.createElement('span');
    nm.className = 'text-white';
    nm.textContent = spell.name;
    wrap.appendChild(nm);
    cell.appendChild(wrap);
    return cell;
}

function effectCell(spell) {
    const cell = document.createElement('td');
    cell.style.cssText = 'padding:3px 4px; white-space:nowrap;';
    if (spell.damage || spell.heal) {
        const isHeal = !!spell.heal;
        cell.className = isHeal ? 'text-green-400' : 'text-red-400';
        cell.textContent = `${getDamageEmoji(spell.damage_type, isHeal)} ${spell.damage || spell.heal}`;
    } else {
        cell.className = 'text-gray-600';
        cell.textContent = '—';
    }
    return cell;
}

function compCell(spell) {
    const cell = document.createElement('td');
    cell.style.cssText = 'padding:3px 4px;';
    const reqs = spell.material_component?.required || [];
    if (reqs.length === 0) { cell.className = 'text-gray-600'; cell.textContent = '—'; return cell; }
    const wrap = document.createElement('div');
    wrap.className = 'flex gap-0.5';
    reqs.forEach(c => {
        const ci = document.createElement('img');
        ci.src = `/res/img/items/${c.component}.png`;
        ci.title = `${c.component} x${c.quantity}`;
        ci.style.cssText = 'width:11px; height:11px; image-rendering:pixelated;';
        ci.onerror = () => { ci.style.display = 'none'; };
        wrap.appendChild(ci);
    });
    cell.appendChild(wrap);
    return cell;
}

function td(text, color) {
    const cell = document.createElement('td');
    cell.style.cssText = `padding:3px 4px; ${color ? `color:${color};` : 'color:#ddd;'}`;
    cell.textContent = text;
    return cell;
}

function slottedIds() {
    const ids = new Set();
    Object.values(getGameStateSync()?.character?.spell_slots || {}).forEach(arr => {
        if (Array.isArray(arr)) arr.forEach(s => { if (s.spell) ids.add(s.spell); });
    });
    return ids;
}

function spellHasType(spell, type) {
    const tags = spell.tags || [];
    if (type === 'damage') return !!spell.damage || tags.includes('damage');
    if (type === 'heal') return !!spell.heal || tags.includes('healing');
    if (type === 'buff') return tags.includes('buff') || tags.includes('support');
    if (type === 'utility') return tags.includes('utility') || (!spell.damage && !spell.heal && !tags.includes('buff'));
    return true;
}

function compare(a, b) {
    if (typeof a === 'number' && typeof b === 'number') return a - b;
    return String(a).localeCompare(String(b));
}

function cap(s) { return s ? s.charAt(0).toUpperCase() + s.slice(1) : ''; }

window.openSpellPicker = openSpellPicker;
window.closeSpellPicker = closeSpellPicker;

logger.debug('Spell picker module loaded');
