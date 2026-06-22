/**
 * Level Guide Modal (slideshow)
 *
 * A read-only "what do I get at each level" preview, one level per slide. Opens
 * from the XP bar (top-right) or the level-up modal, fetches the server-assembled
 * guide (/api/progression/guide), and opens on the player's current level. Arrow
 * buttons / arrow keys step through levels, showing each level's XP distance and
 * what it unlocks. All progression rules live server-side; this only renders.
 *
 * @module systems/levelGuideModal
 */

import { logger } from '../lib/logger.js';
import { gameAPI } from '../lib/api.js';

const num = (n) => (typeof n === 'number' ? n.toLocaleString() : n);
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

let guideData = null;
let slideIndex = 0;

/** Open the modal, load the guide, and jump to the current level. */
export async function openLevelGuide() {
    const modal = document.getElementById('level-guide-modal');
    if (!modal) {
        logger.error('Level guide modal element not found');
        return;
    }

    const slideEl = document.getElementById('level-guide-slide');
    if (slideEl) {
        slideEl.innerHTML = '<div style="font-size:7px;color:#888;padding:18px;text-align:center;">Loading…</div>';
    }
    modal.classList.remove('hidden');
    document.addEventListener('keydown', onKey);

    try {
        guideData = await gameAPI.getLevelGuide();
        slideIndex = clampIndex((guideData.current_level || 1) - 1);
        render();
    } catch (error) {
        logger.error('Failed to load level guide:', error);
        if (slideEl) {
            slideEl.innerHTML = '<div style="font-size:7px;color:#f88;padding:18px;text-align:center;">Could not load the progression guide.</div>';
        }
    }
}

/** Close the modal. */
export function closeLevelGuide() {
    document.getElementById('level-guide-modal')?.classList.add('hidden');
    document.removeEventListener('keydown', onKey);
}

/** Step the slideshow by delta levels (clamped to 1–20). */
export function navLevelGuide(delta) {
    if (!guideData) return;
    slideIndex = clampIndex(slideIndex + delta);
    render();
}

/** Jump back to the player's current level. */
export function jumpToCurrentLevel() {
    if (!guideData) return;
    slideIndex = clampIndex((guideData.current_level || 1) - 1);
    render();
}

function clampIndex(i) {
    const max = guideData ? guideData.levels.length - 1 : 0;
    return Math.min(max, Math.max(0, i));
}

function onKey(e) {
    if (document.getElementById('level-guide-modal')?.classList.contains('hidden')) return;
    if (e.key === 'ArrowRight') { navLevelGuide(1); e.preventDefault(); }
    else if (e.key === 'ArrowLeft') { navLevelGuide(-1); e.preventDefault(); }
    else if (e.key === 'Escape') { closeLevelGuide(); }
}

function render() {
    const slideEl = document.getElementById('level-guide-slide');
    if (!slideEl || !guideData) return;

    const rows = guideData.levels;
    const row = rows[slideIndex];
    const prev = rows[slideIndex - 1];

    // Status line: where this level sits relative to the player.
    let status, statusColor;
    if (row.is_current) {
        status = '★ You are here';
        statusColor = '#22d3ee';
    } else if (row.reached) {
        status = '✓ Reached';
        statusColor = '#86efac';
    } else {
        const away = Math.max(0, row.xp_required - guideData.xp_current);
        status = `${num(away)} XP away`;
        statusColor = '#fcd34d';
    }

    // Unlocks: what's new at this level.
    const unlocks = [];
    if (row.ability_point) {
        unlocks.push(unlockRow('✦', row.feat_eligible
            ? '+1 Ability point <span style="color:#9ca3af;">— or take a feat (planned)</span>'
            : '+1 Ability point', '#fcd34d'));
    }
    const xpPct = row.xp_multiplier ? Math.round((row.xp_multiplier - 1) * 100) : 0;
    if (xpPct > 0) {
        unlocks.push(unlockRow('📈', `+${xpPct}% XP from all sources`, '#fdba74'));
    }
    if (row.level === 1) {
        unlocks.push(unlockRow('❤', `${row.max_hp} starting HP`, '#fca5a5'));
        if (row.max_mana) unlocks.push(unlockRow('🔷', `${row.max_mana} starting Mana`, '#93c5fd'));
    } else {
        if (row.hp_gain > 0) unlocks.push(unlockRow('❤', `+${row.hp_gain} Max HP <span style="color:#9ca3af;">(→ ${row.max_hp})</span>`, '#fca5a5'));
        const manaGain = (row.max_mana || 0) - (prev ? prev.max_mana || 0 : 0);
        if (manaGain > 0) unlocks.push(unlockRow('🔷', `+${manaGain} Max Mana <span style="color:#9ca3af;">(→ ${row.max_mana})</span>`, '#93c5fd'));
        const resGain = (row.resource_max || 0) - (prev ? prev.resource_max || 0 : 0);
        if (row.resource_label && resGain > 0) unlocks.push(unlockRow('🔋', `+${resGain} ${row.resource_label} <span style="color:#9ca3af;">(→ ${row.resource_max})</span>`, '#67e8f9'));
    }
    // Starting resource at level 1 (flat resources have nothing to show otherwise).
    if (row.level === 1 && row.resource_label) {
        unlocks.push(unlockRow('🔋', `${row.resource_max} starting ${row.resource_label}`, '#67e8f9'));
    }
    if (row.proficiency > (prev ? prev.proficiency : 0)) {
        unlocks.push(unlockRow('🎯', `Proficiency bonus → +${row.proficiency}`, '#fde68a'));
    }
    if (row.new_spell_tier > 0) {
        unlocks.push(unlockRow('✨', `New spell level ${row.new_spell_tier} unlocked`, '#c4b5fd'));
    }
    (row.new_abilities || []).forEach((a) => unlocks.push(
        unlockRow('🗡', `<b>${esc(a.name)}</b>${a.summary ? ` — ${esc(a.summary)}` : ''}`, '#86efac')));
    (row.ability_upgrades || []).forEach((a) => unlocks.push(
        unlockRow('🔼', `<b>${esc(a.name)}</b>${a.summary ? `: ${esc(a.summary)}` : ''}`, '#7dd3fc')));

    const unlocksHTML = unlocks.length
        ? unlocks.join('')
        : '<div style="font-size:7px;color:#888;text-align:center;padding:6px;">Just XP toward the next level.</div>';

    // Banked ability points are a current resource — offer to allocate them from
    // the slide the player is on.
    const allocateBanner = (row.is_current && guideData.unspent > 0)
        ? `<div style="text-align:center; margin-bottom:7px;">
             <button onclick="window.openAbilityAllocate && window.openAbilityAllocate()" style="font-size:8px; font-weight:bold; color:#1a1a1a; background:#fcd34d; border:none; padding:3px 8px; cursor:pointer;">✦ ${guideData.unspent} point${guideData.unspent === 1 ? '' : 's'} to spend — Allocate</button>
           </div>`
        : '';

    const atFirst = slideIndex === 0;
    const atLast = slideIndex === rows.length - 1;

    slideEl.innerHTML =
        `<div style="display:flex; align-items:center; justify-content:space-between; padding:6px 8px; border-bottom:1px solid #444;">
            ${navButton(-1, '‹', atFirst)}
            <div style="text-align:center;">
                <div style="font-size:15px; font-weight:bold; color:${row.is_current ? '#22d3ee' : '#ffffff'};">LEVEL ${row.level}</div>
                <div style="font-size:6px; color:#9ca3af; margin-top:1px;">${slideIndex + 1} / ${rows.length}</div>
            </div>
            ${navButton(1, '›', atLast)}
        </div>
        <div style="padding:6px 10px 8px;">
            <div style="font-size:8px; text-align:center; margin-bottom:7px; color:${statusColor}; font-weight:bold;">${status}</div>
            ${allocateBanner}
            <div style="display:flex; flex-direction:column; gap:5px;">${unlocksHTML}</div>
        </div>
        <div style="padding:4px 8px; border-top:1px solid #444; display:flex; justify-content:space-between; align-items:center;">
            <button onclick="window.jumpToCurrentLevel()" style="font-size:6px; color:#22d3ee; background:none; border:none; cursor:pointer; text-decoration:underline;">↩ jump to current</button>
            <span style="font-size:6px; color:#666;">← → keys to flip</span>
        </div>`;
}

function navButton(dir, label, disabled) {
    const color = disabled ? '#3a3a3a' : '#22d3ee';
    const cursor = disabled ? 'default' : 'pointer';
    const onclick = disabled ? '' : `onclick="window.navLevelGuide(${dir})"`;
    return `<button ${onclick} ${disabled ? 'disabled' : ''} style="font-size:18px; width:26px; height:26px; line-height:1; cursor:${cursor}; color:${color}; background:#15323d; border-top:1px solid #2b6577; border-left:1px solid #2b6577; border-right:1px solid #0a1a20; border-bottom:1px solid #0a1a20;">${label}</button>`;
}

function unlockRow(icon, html, color) {
    return `<div style="display:flex; align-items:flex-start; gap:5px; font-size:7px; line-height:1.45;">` +
        `<span style="flex-shrink:0; color:${color};">${icon}</span>` +
        `<span style="color:#fff;">${html}</span></div>`;
}

// Export to window for inline onclick handlers (XP bar, level-up link, nav buttons).
if (typeof window !== 'undefined') {
    window.openLevelGuide = openLevelGuide;
    window.closeLevelGuide = closeLevelGuide;
    window.navLevelGuide = navLevelGuide;
    window.jumpToCurrentLevel = jumpToCurrentLevel;
}
