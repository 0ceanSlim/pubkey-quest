/**
 * Effects Display Module
 *
 * Renders active effects with radial cooldown indicators in the player stats area.
 * Replaces the old stats grid with a dynamic effects display.
 *
 * Effect icons are looked up in this order:
 * 1. /res/img/systems/effects/{effectId}.png (specific effect icon)
 * 2. Fallback by category: buff.png, debuff.png, modifier.png
 */

import { logger } from '../lib/logger.js';
import { eventBus } from '../lib/events.js';
import { getGameStateSync } from '../state/gameState.js';

// System effects that should NOT be displayed (internal tracking effects)
const HIDDEN_EFFECTS = [
    'fatigue-accumulation',
    'hunger-accumulation-stuffed',
    'hunger-accumulation-wellfed',
    'hunger-accumulation-hungry'
];

// Fallback icons by category
const CATEGORY_FALLBACKS = {
    buff: 'buff',
    debuff: 'debuff',
    modifier: 'modifier',
    system: 'modifier'
};

// Cache for effect icons that failed to load (avoid repeated 404s)
const failedIcons = new Set();

class EffectsDisplay {
    constructor() {
        this.container = null;
        this.noEffectsMsg = null;
        this.effectsCache = new Map(); // effectId -> effect data
        logger.debug('EffectsDisplay initialized');
    }

    /**
     * Initialize the effects display
     */
    init() {
        this.container = document.getElementById('active-effects-display');
        this.noEffectsMsg = document.getElementById('no-effects-msg');

        if (!this.container) {
            logger.warn('Effects display container not found');
            return;
        }

        // Listen for effects changes
        eventBus.on('effects:changed', (delta) => this.handleEffectsDelta(delta));

        // Listen for game state loaded to initialize radials with correct values
        eventBus.on('gameStateLoaded', (state) => {
            this.initializeFromState(state);
            // Also trigger a tick to get fresh accumulator values from backend
            if (window.tickManager && typeof window.tickManager.forceTick === 'function') {
                setTimeout(() => window.tickManager.forceTick(), 100);
            }
        });

        // Initial render from current state
        const state = getGameStateSync();
        console.log('EffectsDisplay init - state:', state);
        console.log('EffectsDisplay init - state?.character:', state?.character);
        console.log('EffectsDisplay init - active_effects:', state?.character?.active_effects || state?.active_effects);

        const activeEffects = state?.character?.active_effects || state?.active_effects;
        const fatigue = state?.character?.fatigue ?? state?.fatigue;
        const hunger = state?.character?.hunger ?? state?.hunger;

        if (activeEffects) {
            this.renderEffects(activeEffects);

            // Also initialize radial progress indicators
            const accumulators = this.getAccumulatorValues(activeEffects);
            console.log('EffectsDisplay init - accumulators:', accumulators, 'fatigue:', fatigue, 'hunger:', hunger);
            this.updateFatigueIcon(fatigue, accumulators.fatigueAccumulator, accumulators.fatigueInterval);
            this.updateHungerIcon(hunger, accumulators.hungerAccumulator, accumulators.hungerInterval);
        }

        logger.debug('EffectsDisplay initialized');
    }

    /**
     * Initialize from loaded game state (called when gameStateLoaded event fires)
     * @param {object} state - The loaded game state
     */
    initializeFromState(state) {
        console.log('EffectsDisplay initializeFromState called with state:', state);

        // Ensure container is set (in case init() hasn't run yet)
        if (!this.container) {
            this.container = document.getElementById('active-effects-display');
            this.noEffectsMsg = document.getElementById('no-effects-msg');
        }

        const activeEffects = state?.character?.active_effects || state?.active_effects;
        const fatigue = state?.character?.fatigue ?? state?.fatigue;
        const hunger = state?.character?.hunger ?? state?.hunger;

        console.log('initializeFromState - activeEffects:', activeEffects);
        console.log('initializeFromState - container:', this.container);

        if (activeEffects && activeEffects.length > 0) {
            this.renderEffects(activeEffects);

            const accumulators = this.getAccumulatorValues(activeEffects);
            console.log('initializeFromState - accumulators:', accumulators, 'fatigue:', fatigue, 'hunger:', hunger);
            this.updateFatigueIcon(fatigue, accumulators.fatigueAccumulator, accumulators.fatigueInterval);
            this.updateHungerIcon(hunger, accumulators.hungerAccumulator, accumulators.hungerInterval);
        } else {
            console.log('initializeFromState - no active effects to render');
        }
    }

    /**
     * Handle effects delta from backend
     * @param {object} delta - { added: [], removed: [], updated: [] }
     */
    handleEffectsDelta(delta) {
        if (!delta) return;

        logger.debug('Effects delta received:', delta);

        // Get current effects from state
        const state = getGameStateSync();
        if (state?.character?.active_effects) {
            this.renderEffects(state.character.active_effects);
        }
    }

    /**
     * Render all active effects (smart update to prevent flickering)
     * @param {Array} activeEffects - Array of active effect objects
     */
    renderEffects(activeEffects) {
        if (!this.container) return;

        // Filter out hidden system effects
        const visibleEffects = (activeEffects || []).filter(effect =>
            !HIDDEN_EFFECTS.includes(effect.effect_id)
        );

        // Deduplicate by effect_id (effects with multiple stat modifiers create multiple entries)
        const uniqueEffects = [];
        const seenIds = new Set();
        for (const effect of visibleEffects) {
            if (!seenIds.has(effect.effect_id)) {
                seenIds.add(effect.effect_id);
                uniqueEffects.push(effect);
            }
        }

        // Get current displayed effect IDs
        const existingIcons = this.container.querySelectorAll('.effect-icon-wrapper');
        const displayedIds = new Set();
        existingIcons.forEach(icon => displayedIds.add(icon.dataset.effectId));

        // Get new effect IDs
        const newIds = new Set(uniqueEffects.map(e => e.effect_id));

        // Check if we need a full re-render (effects added or removed)
        const needsFullRender = displayedIds.size !== newIds.size ||
            [...displayedIds].some(id => !newIds.has(id)) ||
            [...newIds].some(id => !displayedIds.has(id));

        if (needsFullRender) {
            // Full re-render: clear and recreate all icons
            existingIcons.forEach(icon => icon.remove());

            if (uniqueEffects.length === 0) {
                if (this.noEffectsMsg) {
                    this.noEffectsMsg.style.display = 'block';
                }
                return;
            }

            if (this.noEffectsMsg) {
                this.noEffectsMsg.style.display = 'none';
            }

            uniqueEffects.forEach(effect => {
                const wrapper = this.createEffectIcon(effect);
                this.container.appendChild(wrapper);
            });
        } else {
            // Just update radial progress on existing icons (no flicker)
            uniqueEffects.forEach(effect => {
                this.updateEffectRadial(effect);
            });
        }
    }

    /**
     * Update just the radial progress for an existing effect icon
     * @param {object} effect - Active effect object
     */
    updateEffectRadial(effect) {
        const progressCircle = document.getElementById(`effect-progress-${effect.effect_id}`);
        if (!progressCircle) return;

        const durationRemaining = effect.duration_remaining || 0;
        const totalDuration = effect.total_duration || durationRemaining;
        const tickInterval = effect.tick_interval || 0;
        const tickAccumulator = effect.tick_accumulator || 0;

        let progress = 0;
        if (durationRemaining > 0 && totalDuration > 0) {
            progress = durationRemaining / totalDuration;
        } else if (tickInterval > 0 && tickAccumulator > 0) {
            progress = tickAccumulator / tickInterval;
        }

        const circumference = 94.25;
        const offset = circumference * (1 - progress);
        progressCircle.setAttribute('stroke-dashoffset', String(offset));
    }

    /**
     * Create an effect icon element with radial timer
     * @param {object} effect - Active effect object
     * @returns {HTMLElement}
     */
    createEffectIcon(effect) {
        const wrapper = document.createElement('div');
        wrapper.className = 'effect-icon-wrapper';
        wrapper.dataset.effectId = effect.effect_id;
        wrapper.style.cssText = 'position: relative; width: 16px; height: 16px; display: inline-block; cursor: help;';

        // Get effect category for fallback
        const category = effect.category || 'modifier';

        // Create the icon image
        const img = document.createElement('img');
        img.className = 'effect-icon-img';
        img.style.cssText = 'width: 16px; height: 16px; image-rendering: pixelated; position: relative; z-index: 2;';
        img.alt = effect.name || effect.effect_id;

        // Try specific icon first, then fallback
        const specificIcon = `/res/img/systems/effects/${effect.effect_id}.png`;
        const fallbackIcon = `/res/img/systems/effects/${CATEGORY_FALLBACKS[category] || 'modifier'}.png`;

        if (failedIcons.has(effect.effect_id)) {
            img.src = fallbackIcon;
        } else {
            img.src = specificIcon;
            img.onerror = () => {
                failedIcons.add(effect.effect_id);
                img.src = fallbackIcon;
            };
        }

        wrapper.appendChild(img);

        // Show radial progress for:
        // 1. Effects with duration_remaining (timed buffs/debuffs like performance-high)
        // 2. Effects with tick_interval and tick_accumulator (periodic effects)
        // Condition-based effects (tired, hungry, etc.) have neither, so no radial
        const durationRemaining = effect.duration_remaining || 0;
        const totalDuration = effect.total_duration || durationRemaining; // Fallback to remaining if total not set
        const tickInterval = effect.tick_interval || 0;
        const tickAccumulator = effect.tick_accumulator || 0;

        let progress = 0;
        let showRadial = false;

        if (durationRemaining > 0 && totalDuration > 0) {
            // Duration-based effect (shows time remaining as countdown)
            progress = durationRemaining / totalDuration;
            showRadial = true;
        } else if (tickInterval > 0 && tickAccumulator > 0) {
            // Tick-based effect (shows progress toward next tick)
            progress = tickAccumulator / tickInterval;
            showRadial = true;
        }

        if (showRadial) {
            const circumference = 94.25; // 2 * π * 15
            const offset = circumference * (1 - progress);

            const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
            svg.setAttribute('viewBox', '0 0 36 36');
            svg.style.cssText = 'position: absolute; top: -4px; left: -4px; width: 24px; height: 24px; transform: rotate(-90deg); pointer-events: none; z-index: 1;';

            // Track circle (background)
            const trackCircle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
            trackCircle.setAttribute('cx', '18');
            trackCircle.setAttribute('cy', '18');
            trackCircle.setAttribute('r', '15');
            trackCircle.setAttribute('fill', 'none');
            trackCircle.setAttribute('stroke', 'rgba(255,255,255,0.2)');
            trackCircle.setAttribute('stroke-width', '3');

            // Progress circle
            const progressCircle = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
            progressCircle.setAttribute('cx', '18');
            progressCircle.setAttribute('cy', '18');
            progressCircle.setAttribute('r', '15');
            progressCircle.setAttribute('fill', 'none');
            progressCircle.setAttribute('stroke', '#00ddff');
            progressCircle.setAttribute('stroke-width', '3');
            progressCircle.setAttribute('stroke-dasharray', String(circumference));
            progressCircle.setAttribute('stroke-dashoffset', String(offset));
            progressCircle.id = `effect-progress-${effect.effect_id}`;

            svg.appendChild(trackCircle);
            svg.appendChild(progressCircle);
            wrapper.appendChild(svg);
        }

        // Add custom tooltip on hover
        wrapper.addEventListener('mouseenter', (e) => this.showTooltip(e, effect));
        wrapper.addEventListener('mouseleave', () => this.hideTooltip());

        return wrapper;
    }

    /**
     * Show custom tooltip for an effect
     * @param {MouseEvent} event - Mouse event
     * @param {object} effect - Active effect object
     */
    showTooltip(event, effect) {
        // Create or get tooltip element
        let tooltip = document.getElementById('effect-tooltip');
        if (!tooltip) {
            tooltip = document.createElement('div');
            tooltip.id = 'effect-tooltip';
            tooltip.style.cssText = `
                position: fixed;
                background: #1a1a2e;
                border: 1px solid #444;
                border-radius: 4px;
                padding: 6px 10px;
                font-size: 10px;
                color: #fff;
                z-index: 9999;
                pointer-events: none;
                box-shadow: 2px 2px 8px rgba(0,0,0,0.5);
                min-width: 80px;
            `;
            document.body.appendChild(tooltip);
        }

        // Build tooltip content
        const effectName = effect.name || effect.effect_id.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
        let html = `<div style="font-weight: bold; margin-bottom: 4px; color: #ddd;">${effectName}</div>`;

        // Add stat modifiers with color coding
        if (effect.stat_modifiers && Object.keys(effect.stat_modifiers).length > 0) {
            const mods = [];
            for (const [stat, value] of Object.entries(effect.stat_modifiers)) {
                const statName = stat.substring(0, 3).toUpperCase();
                const sign = value >= 0 ? '+' : '';
                const color = value >= 0 ? '#4ade80' : '#f87171'; // green for positive, red for negative
                mods.push(`<span style="color: ${color}">${sign}${value} ${statName}</span>`);
            }
            html += `<div style="display: flex; gap: 8px; flex-wrap: wrap;">${mods.join(' ')}</div>`;
        }

        // Add duration if not permanent
        if (effect.duration_remaining > 0) {
            const hours = Math.floor(effect.duration_remaining / 60);
            const mins = Math.round(effect.duration_remaining % 60);
            let timeStr = '';
            if (hours > 0) {
                timeStr = `${hours}h ${mins}m`;
            } else {
                timeStr = `${mins}m`;
            }
            html += `<div style="color: #888; margin-top: 4px; font-size: 9px;">${timeStr} remaining</div>`;
        }

        tooltip.innerHTML = html;
        tooltip.style.display = 'block';

        // Position tooltip above the icon
        const rect = event.target.closest('.effect-icon-wrapper').getBoundingClientRect();
        tooltip.style.left = `${rect.left + rect.width / 2 - tooltip.offsetWidth / 2}px`;
        tooltip.style.top = `${rect.top - tooltip.offsetHeight - 8}px`;

        // Keep tooltip in viewport
        const tooltipRect = tooltip.getBoundingClientRect();
        if (tooltipRect.left < 5) {
            tooltip.style.left = '5px';
        }
        if (tooltipRect.right > window.innerWidth - 5) {
            tooltip.style.left = `${window.innerWidth - tooltip.offsetWidth - 5}px`;
        }
        if (tooltipRect.top < 5) {
            // Show below instead
            tooltip.style.top = `${rect.bottom + 8}px`;
        }
    }

    /**
     * Hide the effect tooltip
     */
    hideTooltip() {
        const tooltip = document.getElementById('effect-tooltip');
        if (tooltip) {
            tooltip.style.display = 'none';
        }
    }

    /**
     * Update fatigue icon and progress
     * @param {number} fatigueLevel - Current fatigue level (0-10)
     * @param {number} accumulator - Tick accumulator progress
     * @param {number} tickInterval - Minutes until next fatigue increase
     */
    updateFatigueIcon(fatigueLevel, accumulator = 0, tickInterval = 60) {
        const iconEl = document.getElementById('fatigue-icon');
        const progressEl = document.getElementById('fatigue-progress');

        console.log('updateFatigueIcon called:', { fatigueLevel, accumulator, tickInterval, hasProgressEl: !!progressEl });

        if (iconEl) {
            // Use appropriate fatigue icon (fatigue-00.png through fatigue-10.png)
            const level = Math.min(Math.max(fatigueLevel || 0, 0), 10);
            const paddedLevel = String(level).padStart(2, '0');
            iconEl.src = `/res/img/systems/fatigue/fatigue-${paddedLevel}.png`;

            // Update tooltip
            const container = document.getElementById('fatigue-icon-container');
            if (container) {
                let status = 'Fresh';
                if (level >= 10) status = 'Exhausted';
                else if (level === 9) status = 'Fatigued';
                else if (level === 8) status = 'Very Tired';
                else if (level >= 6) status = 'Tired';
                container.title = `Fatigue: ${status} (${level}/10)`;
            }
        }

        if (progressEl && tickInterval > 0) {
            // Calculate progress to next fatigue tick using circle circumference
            // Circle radius is 15, circumference = 2 * π * 15 ≈ 94.25
            const circumference = 94.25;
            const progress = accumulator / tickInterval;
            const offset = circumference * (1 - progress);
            console.log('Fatigue radial update:', { progress, offset, circumference });
            progressEl.setAttribute('stroke-dasharray', String(circumference));
            progressEl.setAttribute('stroke-dashoffset', String(offset));
        }
    }

    /**
     * Update hunger icon and progress
     * @param {number} hungerLevel - Current hunger level (0-3)
     * @param {number} accumulator - Tick accumulator progress
     * @param {number} tickInterval - Minutes until next hunger decrease
     */
    updateHungerIcon(hungerLevel, accumulator = 0, tickInterval = 240) {
        const iconEl = document.getElementById('hunger-icon');
        const progressEl = document.getElementById('hunger-progress');

        console.log('updateHungerIcon called:', { hungerLevel, accumulator, tickInterval, hasProgressEl: !!progressEl });

        if (iconEl) {
            // Use appropriate hunger icon (hunger-00.png through hunger-03.png)
            const level = Math.min(Math.max(hungerLevel !== undefined ? hungerLevel : 2, 0), 3);
            const paddedLevel = String(level).padStart(2, '0');
            iconEl.src = `/res/img/systems/hunger/hunger-${paddedLevel}.png`;

            // Update tooltip
            const container = document.getElementById('hunger-icon-container');
            if (container) {
                const statuses = ['Starving', 'Hungry', 'Well Fed', 'Stuffed'];
                container.title = `Hunger: ${statuses[level]} (${level}/3)`;
            }
        }

        if (progressEl && tickInterval > 0 && hungerLevel > 0) {
            // Calculate progress to next hunger tick using circle circumference
            // Circle radius is 15, circumference = 2 * π * 15 ≈ 94.25
            const circumference = 94.25;
            const progress = accumulator / tickInterval;
            const offset = circumference * (1 - progress);
            console.log('Hunger radial update:', { progress, offset, circumference, hungerLevel });
            progressEl.setAttribute('stroke-dasharray', String(circumference));
            progressEl.setAttribute('stroke-dashoffset', String(offset));
        } else if (progressEl) {
            // No progress when starving
            const circumference = 94.25;
            progressEl.setAttribute('stroke-dasharray', String(circumference));
            progressEl.setAttribute('stroke-dashoffset', String(circumference));
        }
    }

    /**
     * Get accumulator values from active effects
     * @param {Array} activeEffects - Array of active effects
     * @returns {object} { fatigueAccumulator, hungerAccumulator, fatigueInterval, hungerInterval }
     */
    getAccumulatorValues(activeEffects) {
        let fatigueAccumulator = 0;
        let hungerAccumulator = 0;
        let fatigueInterval = 60; // Default 1 hour
        let hungerInterval = 240; // Default 4 hours

        if (!activeEffects) return { fatigueAccumulator, hungerAccumulator, fatigueInterval, hungerInterval };

        for (const effect of activeEffects) {
            if (effect.effect_id === 'fatigue-accumulation') {
                fatigueAccumulator = effect.tick_accumulator || 0;
            } else if (effect.effect_id === 'hunger-accumulation-stuffed') {
                hungerAccumulator = effect.tick_accumulator || 0;
                hungerInterval = 360; // 6 hours
            } else if (effect.effect_id === 'hunger-accumulation-wellfed') {
                hungerAccumulator = effect.tick_accumulator || 0;
                hungerInterval = 240; // 4 hours
            } else if (effect.effect_id === 'hunger-accumulation-hungry') {
                hungerAccumulator = effect.tick_accumulator || 0;
                hungerInterval = 240; // 4 hours
            }
        }

        return { fatigueAccumulator, hungerAccumulator, fatigueInterval, hungerInterval };
    }
}

// Create singleton instance
export const effectsDisplay = new EffectsDisplay();

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => effectsDisplay.init());
} else {
    effectsDisplay.init();
}

// Export for global access
window.effectsDisplay = effectsDisplay;

logger.debug('EffectsDisplay module loaded');
