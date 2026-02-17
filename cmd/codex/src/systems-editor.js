// Systems Editor - Main JavaScript
// Manages effects, effect types, and game systems

// State
let allEffects = {};           // Map of effect ID -> Effect
let effectTypes = {};          // EffectTypeDefinition map
let encumbranceConfig = {};    // Encumbrance system config (encumbrance.json)
let currentEffectID = null;    // Selected effect for editing
let skillsConfig = {};         // Skills system config (skills.json)
let travelConfig = {};         // Travel system config (travel-config.json)
let stagingSessionID = null;
let stagingMode = null;
let pendingSliderChanges = {}; // Track unsaved slider changes: { effectID: newValue }

// Valid stats for system_check conditions with their ranges
const CONDITION_STATS = {
    'hunger': { min: 0, max: 3, description: 'Hunger level (0=Starving, 1=Hungry, 2=Well Fed, 3=Stuffed)' },
    'fatigue': { min: 0, max: 10, description: 'Fatigue level (0-5=Rested, 6=Tired, 8=Very Tired, 9=Fatigued, 10=Exhausted)' },
    'weight_percent': { min: 0, max: 300, description: 'Weight as % of capacity (0-50=Light, 101-150=Overweight, 151-200=Encumbered, 201+=Overloaded)' },
    'hp_percent': { min: 0, max: 100, description: 'HP as % of max HP (0-100%)' },
    'mana_percent': { min: 0, max: 100, description: 'Mana as % of max mana (0-100%)' }
};

const CONDITION_OPERATORS = {
    '==': 'equals',
    '!=': 'not equals',
    '<': 'less than',
    '<=': 'less than or equal',
    '>': 'greater than',
    '>=': 'greater than or equal'
};

// Initialization
document.addEventListener('DOMContentLoaded', async () => {
    await loadData();
    setupTabs();
    setupEventListeners();
});

// Load data from API
async function loadData() {
    try {
        const response = await fetch('/api/systems-data');
        const data = await response.json();

        allEffects = data.effects || {};
        effectTypes = data.effect_types || {};
        encumbranceConfig = data.encumbrance || {};
        skillsConfig = data.skills || {};
        travelConfig = data.travel_config || {};

        renderEffectList();
        await initStaging();

        console.log('✅ Systems editor data loaded:', Object.keys(allEffects).length, 'effects');
    } catch (error) {
        console.error('❌ Failed to load systems data:', error);
        showStatus('Failed to load systems data', 'error');
    }
}

// Shared staging initialization (same pattern as character editor)
async function initStaging() {
    try {
        const modeResponse = await fetch('/api/staging/mode');
        const modeData = await modeResponse.json();
        stagingMode = modeData.mode;

        if (stagingMode === 'staging') {
            // Show staging toggle button
            document.getElementById('staging-toggle').classList.remove('hidden');

            // Try to reuse existing session from localStorage
            let existingSessionID = localStorage.getItem('codex_staging_session');
            if (existingSessionID) {
                const verifyResponse = await fetch(`/api/staging/changes?session_id=${existingSessionID}`);
                if (verifyResponse.ok) {
                    const session = await verifyResponse.json();
                    stagingSessionID = existingSessionID;
                    updateChangeCount(session.changes ? session.changes.length : 0);
                    console.log('✅ Reusing existing staging session:', stagingSessionID);
                    return;
                }
            }

            // Create new session
            const initResponse = await fetch('/api/staging/init', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ npub: localStorage.getItem('codex_npub') || 'anonymous' })
            });
            const session = await initResponse.json();
            stagingSessionID = session.session_id;
            localStorage.setItem('codex_staging_session', stagingSessionID);
            console.log('✅ Created new staging session:', stagingSessionID);
        }
    } catch (error) {
        console.error('❌ Failed to initialize staging:', error);
    }
}

// Setup tabs
function setupTabs() {
    document.querySelectorAll('.codex-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            const tabName = tab.dataset.tab;

            // Update tab buttons
            document.querySelectorAll('.codex-tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');

            // Update tab content
            document.querySelectorAll('.tab-content').forEach(content => {
                content.style.display = 'none';
            });
            const targetTab = document.getElementById(`tab-${tabName}`);
            if (targetTab) {
                targetTab.style.display = 'block';
            }

            // Update sidebar for current tab
            renderEffectList();

            // Special handling for different tabs
            if (tabName === 'fatigue') {
                renderFatigueSystem();
            } else if (tabName === 'hunger') {
                renderHungerSystem();
            } else if (tabName === 'weight') {
                renderWeightSystem();
            } else if (tabName === 'effect-types') {
                renderEffectTypesEditor();
            } else if (tabName === 'skills') {
                renderSkillsEditor();
            } else if (tabName === 'travel') {
                renderTravelSystem();
            }
        });
    });

    // Render initial tab (effects list is already active in HTML)
    renderEffectList();
}

// Setup event listeners
function setupEventListeners() {
    // New effect button
    document.getElementById('new-effect-btn').addEventListener('click', createNewEffect);

    // Effect types save button
    document.getElementById('save-effect-types-btn').addEventListener('click', saveEffectTypes);

    // Staging panel
    document.getElementById('view-changes-btn').addEventListener('click', viewChanges);
    document.getElementById('submit-pr-btn').addEventListener('click', submitPR);
    document.getElementById('clear-staging-btn').addEventListener('click', clearStaging);

    // Staging toggle
    const stagingToggle = document.getElementById('staging-toggle');
    if (stagingToggle) {
        stagingToggle.addEventListener('click', toggleStagingPanel);
    }
}

// Toggle staging panel
function toggleStagingPanel() {
    const panel = document.getElementById('staging-panel');
    if (!panel.classList.contains('visible')) {
        panel.style.display = 'block';
        setTimeout(() => {
            panel.classList.add('visible');
        }, 10);
    } else {
        panel.classList.remove('visible');
    }
}

// Close staging panel
window.closeStagingPanel = function() {
    const panel = document.getElementById('staging-panel');
    panel.classList.remove('visible');
};

// Render effect list (filtered by current tab)
function renderEffectList() {
    const effects = Object.values(allEffects);
    const activeTab = document.querySelector('.codex-tab.active')?.dataset.tab;

    let html = '';
    let filteredEffects = [];

    // Filter effects based on active tab
    // System tabs (fatigue, hunger, weight) don't show effects in sidebar - they're managed in the tab UI
    if (activeTab === 'fatigue' || activeTab === 'hunger' || activeTab === 'weight' || activeTab === 'skills') {
        html += '<div style="padding: 1rem; text-align: center; color: var(--color-textMuted); font-size: 12px;">';
        html += `${activeTab.toUpperCase()} settings are managed in the tab view →`;
        html += '</div>';
        document.getElementById('effect-list').innerHTML = html;
        return;
    } else {
        // For "effects" tab, show only applied effects (not system_ticker or system_status)
        const appliedEffects = effects.filter(e => e.source_type === 'applied');

        if (appliedEffects.length > 0) {
            html += '<div class="effect-group-header">APPLIED EFFECTS</div>';
            appliedEffects.sort((a, b) => a.name.localeCompare(b.name)).forEach(effect => {
                html += renderEffectListItem(effect, 'applied');
            });
        } else {
            html += '<div style="padding: 1rem; text-align: center; color: var(--color-textMuted); font-size: 12px;">';
            html += 'No applied effects. Create one with the + NEW EFFECT button.';
            html += '</div>';
        }

        document.getElementById('effect-list').innerHTML = html;
        return;
    }

    // Render filtered effects for system tabs
    filteredEffects.sort((a, b) => a.name.localeCompare(b.name)).forEach(effect => {
        const type = effect.source_type === 'system_ticker' ? 'system' : 'status';
        html += renderEffectListItem(effect, type);
    });

    document.getElementById('effect-list').innerHTML = html;
}

// Render individual effect list item
function renderEffectListItem(effect, type) {
    const tagColor = type === 'system' ? 'var(--codex-cyan)' :
                     (type === 'status' ? 'var(--codex-yellow)' : 'var(--codex-purple)');

    return `
        <li class="codex-list-item ${currentEffectID === effect.id ? 'active' : ''}"
            onclick="selectEffect('${effect.id}')" style="cursor: pointer;">
            <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
                <span style="color: var(--codex-text-primary);">
                    ${effect.name || effect.id}
                </span>
                <span class="codex-tag" style="font-size: 10px; background-color: ${tagColor};">
                    ${type.toUpperCase()}
                </span>
            </div>
        </li>
    `;
}

// Select effect
window.selectEffect = function(effectID) {
    currentEffectID = effectID;
    renderEffectList();
    populateEffectForm(effectID);
};

// Populate effect form
function populateEffectForm(effectID) {
    const effect = allEffects[effectID];
    if (!effect) return;

    currentEffectID = effectID;

    let html = `
        <div class="codex-section win95-inset pixel-clip">
            <h3 class="codex-section-title">EFFECT: ${effect.name}</h3>
            <div class="mb-15" style="color: var(--codex-text-secondary); font-size: 12px;">
                Source: ${effect.source_type || 'unknown'} | Category: ${effect.category || 'none'}
            </div>

            <label class="codex-label">EFFECT ID</label>
            <input type="text" class="codex-input" value="${effect.id}" disabled />

            <label class="codex-label">NAME</label>
            <input type="text" class="codex-input" value="${effect.name || ''}"
                   onchange="updateEffectField('name', this.value)" />

            <label class="codex-label">DESCRIPTION</label>
            <textarea class="codex-textarea"
                      onchange="updateEffectField('description', this.value)">${effect.description || ''}</textarea>

            <div class="grid grid-cols-2 gap-4 mb-15">
                <div>
                    <label class="codex-label">SOURCE TYPE</label>
                    <select class="codex-select" onchange="updateEffectField('source_type', this.value)">
                        <option value="system_ticker" ${effect.source_type === 'system_ticker' ? 'selected' : ''}>System Ticker</option>
                        <option value="system_status" ${effect.source_type === 'system_status' ? 'selected' : ''}>System Status</option>
                        <option value="applied" ${effect.source_type === 'applied' ? 'selected' : ''}>Applied</option>
                    </select>
                </div>
                <div>
                    <label class="codex-label">CATEGORY</label>
                    <select class="codex-select" onchange="updateEffectField('category', this.value)">
                        <option value="buff" ${effect.category === 'buff' ? 'selected' : ''}>Buff</option>
                        <option value="debuff" ${effect.category === 'debuff' ? 'selected' : ''}>Debuff</option>
                        <option value="status" ${effect.category === 'status' ? 'selected' : ''}>Status</option>
                    </select>
                </div>
            </div>

            <div class="grid grid-cols-2 gap-4 mb-15">
                <div>
                    <label class="codex-label">VISIBLE</label>
                    <select class="codex-select" onchange="updateEffectField('visible', this.value === 'true')">
                        <option value="true" ${effect.visible === true ? 'selected' : ''}>Yes (show in UI)</option>
                        <option value="false" ${effect.visible === false ? 'selected' : ''}>No (silent)</option>
                    </select>
                </div>
                <div>
                    <label class="codex-label">MESSAGE (shown when applied)</label>
                    <input type="text" class="codex-input" value="${effect.message || ''}"
                           placeholder="Optional message"
                           onchange="updateEffectField('message', this.value)" />
                </div>
            </div>

            ${effect.source_type === 'system_status' ? `
                <h4 class="codex-subsection-title">SYSTEM CHECK (Activation Condition)</h4>
                <div class="win95-inset pixel-clip mb-15" style="padding: 1rem;">
                    ${renderSystemCheckEditor(effect.system_check || null)}
                </div>
            ` : ''}

            <h4 class="codex-subsection-title">REMOVAL CONDITION</h4>
            <div class="win95-inset pixel-clip mb-15" style="padding: 1rem;">
                ${renderRemovalEditor(effect.removal || {})}
            </div>

            <h4 class="codex-subsection-title">MODIFIERS</h4>
            <div id="modifiers-container">
                ${renderModifiers(effect.modifiers || [])}
            </div>
            <button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm"
                    onclick="addModifier()">+ ADD MODIFIER</button>

            <div class="codex-btn-group mt-20">
                <button class="codex-btn codex-btn-primary pixel-clip-sm" onclick="saveEffect()">💾 SAVE EFFECT</button>
                <button class="codex-btn pixel-clip-sm" onclick="deleteEffect()">🗑️ DELETE</button>
            </div>
        </div>
    `;

    document.getElementById('effect-form').innerHTML = html;
}

// Render removal editor
function renderRemovalEditor(removal) {
    const removalType = removal.type || 'permanent';
    const showTimer = removalType === 'timed';

    return `
        <div class="grid grid-cols-1 gap-4 mb-10">
            <div>
                <label class="codex-label">REMOVAL TYPE</label>
                <select class="codex-select" onchange="updateRemovalType(this.value)">
                    <option value="permanent" ${removalType === 'permanent' ? 'selected' : ''}>Permanent (managed by system)</option>
                    <option value="timed" ${removalType === 'timed' ? 'selected' : ''}>Timed (duration only)</option>
                    <option value="action" ${removalType === 'action' ? 'selected' : ''}>Action (removed by specific action)</option>
                    <option value="equipment" ${removalType === 'equipment' ? 'selected' : ''}>Equipment (removed when unequipped)</option>
                </select>
            </div>
        </div>

        ${showTimer ? `
            <div class="grid grid-cols-1 gap-4 mb-10">
                <div>
                    <label class="codex-label">TIMER (minutes)</label>
                    <input type="number" class="codex-input" value="${removal.timer || ''}"
                           placeholder="Duration in minutes"
                           min="1" step="1"
                           onchange="updateRemovalField('timer', parseInt(this.value) || 0)" />
                    <small style="color: var(--codex-text-muted);">How long the effect lasts</small>
                </div>
            </div>
        ` : ''}

        ${removalType === 'action' ? `
            <div class="grid grid-cols-1 gap-4">
                <div>
                    <label class="codex-label">ACTION ID</label>
                    <input type="text" class="codex-input" value="${removal.action || ''}"
                           placeholder="e.g., rest, sleep, consume-antidote"
                           onchange="updateRemovalField('action', this.value)" />
                    <small style="color: var(--codex-text-muted);">Action that removes this effect</small>
                </div>
            </div>
        ` : ''}
    `;
}

// Render system check editor for system_status effects
function renderSystemCheckEditor(systemCheck) {
    const check = systemCheck || { stat: 'hunger', operator: '==', value: 0 };

    return `
        <div class="mb-10">
            <p style="color: var(--codex-text-secondary); font-size: 12px; margin-bottom: 1rem;">
                This effect will be <strong>active</strong> when the condition below is TRUE
            </p>
        </div>

        <div class="grid grid-cols-4 gap-4">
            <div>
                <label class="codex-label">STAT</label>
                <select id="system-check-stat" class="codex-select" onchange="updateSystemCheck()">
                    ${Object.entries(CONDITION_STATS).map(([stat, info]) => `
                        <option value="${stat}" ${check.stat === stat ? 'selected' : ''}>
                            ${stat}
                        </option>
                    `).join('')}
                </select>
            </div>
            <div>
                <label class="codex-label">OPERATOR</label>
                <select id="system-check-operator" class="codex-select" onchange="updateSystemCheck()">
                    ${Object.entries(CONDITION_OPERATORS).map(([op, label]) => `
                        <option value="${op}" ${check.operator === op ? 'selected' : ''}>
                            ${op} (${label})
                        </option>
                    `).join('')}
                </select>
            </div>
            <div>
                <label class="codex-label">VALUE</label>
                <input id="system-check-value" type="number" class="codex-input"
                       value="${check.value ?? ''}"
                       min="${CONDITION_STATS[check.stat]?.min || 0}"
                       max="${CONDITION_STATS[check.stat]?.max || 100}"
                       onchange="updateSystemCheck()" />
            </div>
            <div style="display: flex; align-items: flex-end;">
                <button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="clearSystemCheck()">CLEAR</button>
            </div>
        </div>

        <div style="margin-top: 0.5rem;">
            <small style="color: var(--codex-text-secondary);">
                ${CONDITION_STATS[check.stat]?.description || ''}
                (Valid range: ${CONDITION_STATS[check.stat]?.min}-${CONDITION_STATS[check.stat]?.max})
            </small>
        </div>

        <div style="margin-top: 0.5rem;">
            <small style="color: var(--codex-cyan);">
                Active when: <strong>${check.stat} ${check.operator} ${check.value}</strong>
            </small>
        </div>
    `;
}

// Update system check from inputs
window.updateSystemCheck = function() {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    const stat = document.getElementById('system-check-stat').value;
    const operator = document.getElementById('system-check-operator').value;
    const value = parseInt(document.getElementById('system-check-value').value);

    // Validate value is in range
    const range = CONDITION_STATS[stat];
    if (value < range.min || value > range.max) {
        alert(`Value must be between ${range.min} and ${range.max}`);
        return;
    }

    // Update effect
    effect.system_check = { stat, operator, value };

    // Re-render to update preview
    populateEffectForm(currentEffectID);
};

// Clear system check
window.clearSystemCheck = function() {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    effect.system_check = null;
    populateEffectForm(currentEffectID);
};

// Update removal type (triggers re-render to show/hide appropriate fields)
window.updateRemovalType = function(type) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    if (!effect.removal) {
        effect.removal = {};
    }

    effect.removal.type = type;

    // Clear fields that don't apply to this type
    if (type === 'permanent' || type === 'equipment') {
        delete effect.removal.timer;
        delete effect.removal.action;
    } else if (type === 'timed') {
        delete effect.removal.action;
    } else if (type === 'action') {
        delete effect.removal.timer;
    }

    populateEffectForm(currentEffectID);
};

// Render modifiers
function renderModifiers(modifiers) {
    if (!modifiers || modifiers.length === 0) {
        return '<p style="color: var(--codex-text-muted); text-align: center; padding: 1rem;">No modifiers. Add one to get started.</p>';
    }

    return modifiers.map((modifier, index) => `
        <div class="modifier-item win95-inset pixel-clip" style="margin-bottom: 1rem; padding: 1rem;">
            <div class="grid grid-cols-3 gap-4 mb-10">
                <div>
                    <label class="codex-label">STAT</label>
                    <select class="codex-select" onchange="updateModifier(${index}, 'stat', this.value)">
                        ${Object.keys(effectTypes.effect_types || {}).map(statID => `
                            <option value="${statID}" ${modifier.stat === statID ? 'selected' : ''}>
                                ${statID} - ${effectTypes.effect_types[statID].description}
                            </option>
                        `).join('')}
                    </select>
                </div>
                <div>
                    <label class="codex-label">VALUE</label>
                    <input type="number" class="codex-input" value="${modifier.value || 0}"
                           onchange="updateModifier(${index}, 'value', parseInt(this.value))" />
                </div>
                <div>
                    <label class="codex-label">TYPE</label>
                    <select class="codex-select" onchange="updateModifierType(${index}, this.value)">
                        <option value="instant" ${modifier.type === 'instant' ? 'selected' : ''}>Instant (apply once)</option>
                        <option value="constant" ${modifier.type === 'constant' ? 'selected' : ''}>Constant (while active)</option>
                        <option value="periodic" ${modifier.type === 'periodic' ? 'selected' : ''}>Periodic (repeating)</option>
                    </select>
                </div>
            </div>

            <div class="grid grid-cols-2 gap-4">
                <div>
                    <label class="codex-label">DELAY (minutes)</label>
                    <input type="number" class="codex-input" value="${modifier.delay || 0}"
                           onchange="updateModifier(${index}, 'delay', parseInt(this.value) || 0)" />
                </div>
                ${modifier.type === 'periodic' ? `
                <div>
                    <label class="codex-label">TICK INTERVAL (minutes)</label>
                    <input type="number" class="codex-input" value="${modifier.tick_interval || ''}"
                           placeholder="Required for periodic"
                           onchange="updateModifier(${index}, 'tick_interval', parseInt(this.value) || 0)" />
                </div>
                ` : '<div></div>'}
            </div>

            <button class="codex-btn codex-btn-sm pixel-clip-sm mt-10"
                    onclick="removeModifier(${index})">REMOVE</button>
        </div>
    `).join('');
}

// Update effect field
window.updateEffectField = function(field, value) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    effect[field] = value;
};

// Update removal field
window.updateRemovalField = function(field, value) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    if (!effect.removal) {
        effect.removal = {};
    }

    effect.removal[field] = value;
};

// Update modifier
window.updateModifier = function(index, field, value) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect || !effect.modifiers) return;

    effect.modifiers[index][field] = value;
};

// Update modifier type (triggers re-render to show/hide tick_interval)
window.updateModifierType = function(index, type) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect || !effect.modifiers) return;

    effect.modifiers[index].type = type;

    // Clear tick_interval if not periodic
    if (type !== 'periodic') {
        effect.modifiers[index].tick_interval = 0;
    }

    // Re-render to update UI
    populateEffectForm(currentEffectID);
};

// Add modifier
window.addModifier = function() {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    if (!effect.modifiers) {
        effect.modifiers = [];
    }

    // Get first effect type or empty
    const firstType = Object.keys(effectTypes.effect_types || {})[0] || '';

    effect.modifiers.push({
        stat: firstType,
        value: 0,
        type: 'instant',
        delay: 0,
        tick_interval: 0
    });

    populateEffectForm(currentEffectID);
};

// Remove modifier
window.removeModifier = function(index) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect || !effect.modifiers) return;

    effect.modifiers.splice(index, 1);
    populateEffectForm(currentEffectID);
};

window.saveEffect = saveEffect;
window.deleteEffect = deleteEffect;

// Create new effect
function createNewEffect() {
    const effectID = prompt('Enter new effect ID (lowercase, hyphens only):');
    if (!effectID) return;

    // Validate ID
    if (!/^[a-z0-9-]+$/.test(effectID)) {
        showStatus('Invalid effect ID. Use lowercase letters, numbers, and hyphens only.', 'error');
        return;
    }

    if (allEffects[effectID]) {
        showStatus('Effect already exists!', 'error');
        return;
    }

    // Create new effect with new structure
    const newEffect = {
        id: effectID,
        name: effectID.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase()),
        description: '',
        source_type: 'applied',
        category: 'buff',
        removal: {
            type: 'timed',
            timer: 60
        },
        modifiers: [],
        message: '',
        visible: true
    };

    allEffects[effectID] = newEffect;
    renderEffectList();
    selectEffect(effectID);
}

// Save effect
async function saveEffect() {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    try {
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch(`/api/effects/${currentEffectID}`, {
            method: 'PUT',
            headers: headers,
            body: JSON.stringify(effect)
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();

        if (result.mode === 'staging') {
            showStatus(`✅ Effect staged (${result.changes} total changes)`, 'success');
            updateChangeCount(result.changes);
        } else {
            showStatus('✅ Effect saved successfully', 'success');
        }
    } catch (error) {
        console.error('❌ Save failed:', error);
        showStatus(`❌ ${error.message}`, 'error');
    }
}

// Delete effect
async function deleteEffect() {
    if (!currentEffectID) return;

    if (!confirm(`Are you sure you want to delete effect "${currentEffectID}"?`)) {
        return;
    }

    try {
        const headers = {};
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch(`/api/effects/${currentEffectID}`, {
            method: 'DELETE',
            headers: headers
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();

        delete allEffects[currentEffectID];
        currentEffectID = null;
        renderEffectList();
        document.getElementById('effect-form').innerHTML = '<p style="text-align: center; color: var(--color-textMuted);">Effect deleted. Select another effect or create a new one.</p>';

        if (result.mode === 'staging') {
            showStatus(`✅ Effect deletion staged (${result.changes} total changes)`, 'success');
            updateChangeCount(result.changes);
        } else {
            showStatus('✅ Effect deleted successfully', 'success');
        }
    } catch (error) {
        console.error('❌ Delete failed:', error);
        showStatus(`❌ ${error.message}`, 'error');
    }
}

// ====================
// SYSTEM STATUS EDITORS
// ====================

// Helper: Save effect to backend (reusable)
async function saveEffectToBackend(effectID, effectData) {
    const headers = { 'Content-Type': 'application/json' };
    if (stagingSessionID) {
        headers['X-Session-ID'] = stagingSessionID;
    }

    const response = await fetch(`/api/effects/${effectID}`, {
        method: 'PUT',
        headers: headers,
        body: JSON.stringify(effectData)
    });

    if (!response.ok) {
        const error = await response.text();
        throw new Error(error);
    }

    const result = await response.json();

    if (result.mode === 'staging') {
        updateChangeCount(result.changes);
    }

    return result;
}

// Render Skill Scaling section for an effect
function renderSkillScalingSection(effect) {
    if (!effect) return '';

    const scaling = effect.skill_scaling || {};
    const hasScaling = !!effect.skill_scaling;
    const skillOptions = Object.entries(skillsConfig).sort((a, b) => a[1].name.localeCompare(b[1].name));

    return `
        <div class="mb-20 win95-inset pixel-clip" style="padding: 1rem;">
            <div style="display: flex; align-items: center; gap: 1rem; margin-bottom: 0.75rem;">
                <h4 class="codex-subsection-title" style="margin: 0;">SKILL SCALING</h4>
                <label style="color: var(--codex-text-muted); font-size: 12px; cursor: pointer;">
                    <input type="checkbox" ${hasScaling ? 'checked' : ''}
                           onchange="toggleSkillScaling('${effect.id}', this.checked)" />
                    Enable
                </label>
            </div>
            ${hasScaling ? `
                <div class="grid grid-cols-2 gap-4">
                    <div>
                        <label class="codex-label">SKILL</label>
                        <select class="codex-input" onchange="updateSkillScaling('${effect.id}', 'skill', this.value)">
                            ${skillOptions.map(([id, skill]) =>
                                `<option value="${id}" ${id === scaling.skill ? 'selected' : ''}>${skill.name}</option>`
                            ).join('')}
                        </select>
                    </div>
                    <div>
                        <label class="codex-label">BASE LEVEL (no bonus at or below)</label>
                        <input type="number" class="codex-input" value="${scaling.base_level || 10}"
                               min="1" max="20" step="1"
                               onchange="updateSkillScaling('${effect.id}', 'base_level', parseInt(this.value))" />
                    </div>
                    <div>
                        <label class="codex-label">BONUS PER LEVEL (%)</label>
                        <input type="number" class="codex-input" value="${((scaling.bonus_per_level || 0.05) * 100).toFixed(0)}"
                               min="1" max="50" step="1"
                               onchange="updateSkillScaling('${effect.id}', 'bonus_per_level', parseInt(this.value) / 100)" />
                        <small style="color: var(--codex-text-muted); display: block; margin-top: 0.25rem; font-size: 10px;">
                            % slower per skill level above base
                        </small>
                    </div>
                    <div>
                        <label class="codex-label">MAX BONUS LEVELS</label>
                        <input type="number" class="codex-input" value="${scaling.max_bonus_levels || 10}"
                               min="1" max="20" step="1"
                               onchange="updateSkillScaling('${effect.id}', 'max_bonus_levels', parseInt(this.value))" />
                        <small style="color: var(--codex-text-muted); display: block; margin-top: 0.25rem; font-size: 10px;">
                            Cap: ${((scaling.bonus_per_level || 0.05) * (scaling.max_bonus_levels || 10) * 100).toFixed(0)}% max slowdown
                        </small>
                    </div>
                </div>
            ` : `
                <small style="color: var(--codex-text-muted);">
                    When enabled, a character skill can slow this effect's tick rate.
                </small>
            `}
        </div>
    `;
}

// Render Fatigue System Editor
function renderFatigueSystem() {
    // Get all fatigue-related effects (INCLUDING accumulation, tired, very-tired, exhaustion)
    const fatigueEffects = Object.values(allEffects).filter(e =>
        e.source_type === 'system_status' && e.id &&
        (e.id.includes('fatigue') || e.id.includes('tired') || e.id === 'exhaustion')
    ).sort((a, b) => (a.system_check?.value || 0) - (b.system_check?.value || 0));

    // Get accumulation effect
    const accumulationEffect = allEffects['fatigue-accumulation'];
    const accumulationRate = accumulationEffect?.modifiers?.[0]?.tick_interval || 60;

    let html = `
        <div class="codex-section win95-inset pixel-clip">
            <h3 class="codex-section-title">⏱️ FATIGUE SYSTEM</h3>

            ${renderSaveButton('fatigue')}

            <div class="mb-20">
                <label class="codex-label">ACCUMULATION RATE (minutes per fatigue point)</label>
                <input type="number" class="codex-input" value="${accumulationRate}"
                       min="1" step="1"
                       onchange="updateFatigueAccumulationRate(this.value)"
                       style="max-width: 200px;" />
                <small style="color: var(--codex-text-muted); display: block; margin-top: 0.5rem;">
                    How often fatigue increases by 1 point
                </small>
            </div>

            ${renderSkillScalingSection(accumulationEffect)}

            <div class="mb-20">
                <h4 class="codex-subsection-title">FATIGUE LEVELS (drag dots to adjust thresholds)</h4>
                <div class="fatigue-slider-container" style="position: relative; height: 140px; margin: 2rem 0; padding: 0 30px;">
                    <div class="slider-track" style="
                        position: absolute;
                        top: 50%;
                        left: 30px;
                        right: 30px;
                        height: 10px;
                        background: linear-gradient(to right, #2a9d8f, #e9c46a, #f4a261, #e76f51);
                        border: 3px solid #8ecae6;
                        transform: translateY(-50%);
                        border-radius: 5px;
                        box-shadow: inset 0 2px 4px rgba(0,0,0,0.3);
                    "></div>
                    <div class="slider-labels" style="position: absolute; top: 90px; left: 30px; right: 30px; display: flex; justify-content: space-between; font-size: 11px; color: var(--color-textPrimary);">
                        ${Array.from({length: 11}, (_, i) => `<span style="width: 20px; text-align: center;">${i}</span>`).join('')}
                    </div>
                    ${fatigueEffects.filter(e => e.system_check).map((effect, idx) => {
                        const value = effect.system_check?.value || 0;
                        const position = value / 10; // 0.0 to 1.0
                        const isEven = idx % 2 === 0;
                        const topPos = isEven ? '-45px' : '-70px';
                        return `
                            <div class="breakpoint"
                                 data-effect-id="${effect.id}"
                                 data-value="${value}"
                                 data-system="fatigue"
                                 data-max="10"
                                 draggable="true"
                                 style="
                                     position: absolute;
                                     top: 50%;
                                     left: calc(30px + (100% - 60px) * ${position});
                                     transform: translate(-50%, -50%);
                                     cursor: grab;
                                     z-index: ${10 + idx};
                                 "
                                 ondragstart="handleBreakpointDragStart(event)"
                                 ondragend="handleBreakpointDragEnd(event)"
                                 onclick="toggleInlineEditor('${effect.id}')">
                                <div style="
                                    width: 18px;
                                    height: 18px;
                                    background: #ffd60a;
                                    border: 4px solid var(--color-bgPrimary);
                                    box-shadow: 0 0 0 2px #8ecae6, 0 2px 4px rgba(0,0,0,0.5);
                                    border-radius: 50%;
                                    pointer-events: none;
                                "></div>
                                <div style="
                                    position: absolute;
                                    top: ${topPos};
                                    left: 50%;
                                    transform: translateX(-50%);
                                    white-space: nowrap;
                                    font-size: 9px;
                                    font-weight: bold;
                                    color: #ffd60a;
                                    background: var(--color-bgPrimary);
                                    padding: 4px 8px;
                                    border: 2px solid #8ecae6;
                                    border-radius: 4px;
                                    pointer-events: none;
                                    box-shadow: 0 2px 4px rgba(0,0,0,0.5);
                                ">${effect.name} (${value})</div>
                            </div>
                        `;
                    }).join('')}
                </div>
            </div>

            <div>
                <h4 class="codex-subsection-title">EFFECTS</h4>
                ${fatigueEffects.map(effect => `
                    <div class="effect-item win95-inset pixel-clip mb-10" style="padding: 1rem;">
                        <div style="display: flex; justify-content: space-between; align-items: center; cursor: pointer;"
                             onclick="toggleInlineEditor('${effect.id}')">
                            <div>
                                <strong style="color: var(--codex-yellow);">${effect.name}</strong>
                                <span style="color: var(--codex-text-muted); margin-left: 1rem;">
                                    @ Fatigue ${effect.system_check?.operator || '=='} ${effect.system_check?.value || 0}
                                </span>
                            </div>
                            <div>
                                <span style="color: var(--codex-text-secondary); font-size: 12px; margin-right: 0.5rem;">
                                    ${formatModifiers(effect.modifiers || [])}
                                </span>
                                <span style="color: var(--codex-text-muted); font-size: 14px;">
                                    ${expandedEffectID === effect.id ? '▼' : '▶'}
                                </span>
                            </div>
                        </div>
                        ${expandedEffectID === effect.id ? renderInlineModifierEditor(effect) : ''}
                    </div>
                `).join('')}
            </div>
        </div>
    `;

    document.getElementById('fatigue-editor').innerHTML = html;
}

// Render Hunger System Editor
function renderHungerSystem() {
    // Get all hunger effects
    const hungerEffects = Object.values(allEffects).filter(e =>
        e.source_type === 'system_status' && e.id && (e.id === 'starving' || e.id === 'hungry' || e.id === 'stuffed')
    ).sort((a, b) => (a.system_check?.value || 0) - (b.system_check?.value || 0));

    // Get accumulation effects
    const accStuffed = allEffects['hunger-accumulation-stuffed'];
    const accWellFed = allEffects['hunger-accumulation-wellfed'];
    const accHungry = allEffects['hunger-accumulation-hungry'];

    let html = `
        <div class="codex-section win95-inset pixel-clip">
            <h3 class="codex-section-title">🍖 HUNGER SYSTEM</h3>

            ${renderSaveButton('hunger')}

            <div class="mb-20">
                <h4 class="codex-subsection-title">TIME AT EACH LEVEL (minutes before hunger decreases)</h4>
                <div class="grid grid-cols-3 gap-4">
                    <div>
                        <label class="codex-label">Time at Stuffed (3)</label>
                        <input type="number" class="codex-input" value="${accStuffed?.modifiers?.[0]?.tick_interval || 120}"
                               min="1" onchange="updateHungerAccumulationRate('stuffed', this.value)" />
                        <small style="color: var(--codex-text-muted); display: block; margin-top: 0.25rem; font-size: 10px;">
                            Before dropping to Well Fed (2)
                        </small>
                    </div>
                    <div>
                        <label class="codex-label">Time at Well Fed (2)</label>
                        <input type="number" class="codex-input" value="${accWellFed?.modifiers?.[0]?.tick_interval || 180}"
                               min="1" onchange="updateHungerAccumulationRate('wellfed', this.value)" />
                        <small style="color: var(--codex-text-muted); display: block; margin-top: 0.25rem; font-size: 10px;">
                            Before dropping to Hungry (1)
                        </small>
                    </div>
                    <div>
                        <label class="codex-label">Time at Hungry (1)</label>
                        <input type="number" class="codex-input" value="${accHungry?.modifiers?.[0]?.tick_interval || 240}"
                               min="1" onchange="updateHungerAccumulationRate('hungry', this.value)" />
                        <small style="color: var(--codex-text-muted); display: block; margin-top: 0.25rem; font-size: 10px;">
                            Before dropping to Starving (0)
                        </small>
                    </div>
                </div>
            </div>

            <div class="mb-20">
                <h4 class="codex-subsection-title">HUNGER LEVELS (Fixed positions - click to edit modifiers)</h4>
                <div class="hunger-slider-container" style="position: relative; height: 120px; margin: 2rem 0; padding: 0 40px;">
                    <div class="slider-track" style="
                        position: absolute;
                        top: 50%;
                        left: 40px;
                        right: 40px;
                        height: 10px;
                        background: linear-gradient(to right, #e63946, #f77f00, #06a77d, #2a9d8f);
                        border: 3px solid #9d4edd;
                        transform: translateY(-50%);
                        border-radius: 5px;
                        box-shadow: inset 0 2px 4px rgba(0,0,0,0.3);
                    "></div>
                    <div class="slider-labels" style="position: absolute; top: 80px; left: 40px; right: 40px; display: flex; justify-content: space-between; font-size: 10px; color: var(--color-textPrimary); text-align: center;">
                        <span style="width: 60px;">0<br/><strong>Starving</strong></span>
                        <span style="width: 60px;">1<br/><strong>Hungry</strong></span>
                        <span style="width: 60px;">2<br/><strong>Well Fed</strong></span>
                        <span style="width: 60px;">3<br/><strong>Stuffed</strong></span>
                    </div>
                    ${hungerEffects.filter(e => e.system_check).map(effect => {
                        const value = effect.system_check?.value || 0;
                        const position = value / 3; // 0.0 to 1.0
                        return `
                            <div class="breakpoint"
                                 data-effect-id="${effect.id}"
                                 data-value="${value}"
                                 style="
                                     position: absolute;
                                     top: 50%;
                                     left: calc(40px + (100% - 80px) * ${position});
                                     transform: translate(-50%, -50%);
                                     cursor: pointer;
                                     z-index: 10;
                                 "
                                 onclick="toggleInlineEditor('${effect.id}')">
                                <div style="
                                    width: 18px;
                                    height: 18px;
                                    background: #9d4edd;
                                    border: 4px solid var(--color-bgPrimary);
                                    box-shadow: 0 0 0 2px #9d4edd, 0 2px 4px rgba(0,0,0,0.5);
                                    border-radius: 50%;
                                    pointer-events: none;
                                "></div>
                            </div>
                        `;
                    }).join('')}
                </div>
            </div>

            <div>
                <h4 class="codex-subsection-title">EFFECTS</h4>
                ${hungerEffects.map(effect => `
                    <div class="effect-item win95-inset pixel-clip mb-10" style="padding: 1rem;">
                        <div style="display: flex; justify-content: space-between; align-items: center; cursor: pointer;"
                             onclick="toggleInlineEditor('${effect.id}')">
                            <div>
                                <strong style="color: var(--codex-purple);">${effect.name}</strong>
                                <span style="color: var(--codex-text-muted); margin-left: 1rem;">
                                    @ Hunger ${effect.system_check?.operator || '=='} ${effect.system_check?.value || 0}
                                </span>
                            </div>
                            <div>
                                <span style="color: var(--codex-text-secondary); font-size: 12px; margin-right: 0.5rem;">
                                    ${formatModifiers(effect.modifiers || [])}
                                </span>
                                <span style="color: var(--codex-text-muted); font-size: 14px;">
                                    ${expandedEffectID === effect.id ? '▼' : '▶'}
                                </span>
                            </div>
                        </div>
                        ${expandedEffectID === effect.id ? renderInlineModifierEditor(effect) : ''}
                    </div>
                `).join('')}
            </div>
        </div>
    `;

    document.getElementById('hunger-editor').innerHTML = html;
}

// Render Weight System Editor
function renderWeightSystem() {
    // Get all weight effects
    const weightEffects = Object.values(allEffects).filter(e =>
        e.source_type === 'system_status' && e.id && e.id.includes('encumbrance')
    ).sort((a, b) => (a.system_check?.value || 0) - (b.system_check?.value || 0));

    let html = `
        <div class="codex-section win95-inset pixel-clip">
            <h3 class="codex-section-title">⚖️ WEIGHT SYSTEM</h3>

            ${renderSaveButton('weight')}

            <div class="mb-20" style="background: var(--color-bgSecondary); padding: 1rem; border-radius: 4px;">
                <strong style="color: #8ecae6;">Weight Capacity Formula:</strong>
                <div style="margin-top: 0.5rem; display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
                    <input type="number" id="weightMultiplierInput"
                        value="${encumbranceConfig?.encumbrance_system?.weight_calculation?.base_weight_multiplier ?? 5}"
                        min="0.5" max="50" step="0.5"
                        style="width: 70px; padding: 4px 6px; background: var(--color-bgPrimary); border: 1px solid #8ecae6; color: var(--color-textPrimary); font-family: inherit;"
                        oninput="updateWeightFormulaPreview()" />
                    <span style="color: var(--color-textSecondary);">× Strength + Effects Bonus</span>
                    <button type="button" class="codex-btn codex-btn-secondary" style="padding: 4px 10px; font-size: 12px;" onclick="saveWeightMultiplier()">Save</button>
                </div>
                <div id="weightFormulaPreview" style="margin-top: 0.5rem; color: var(--color-textSecondary); font-size: 12px;">
                    Example: STR 14 × ${encumbranceConfig?.encumbrance_system?.weight_calculation?.base_weight_multiplier ?? 5} = ${14 * (encumbranceConfig?.encumbrance_system?.weight_calculation?.base_weight_multiplier ?? 5)} lbs base capacity
                </div>
            </div>

            <div class="mb-20">
                <h4 class="codex-subsection-title">WEIGHT PERCENTAGE THRESHOLDS (drag dots to adjust)</h4>
                <div class="weight-slider-container" style="position: relative; height: 140px; margin: 2rem 0; padding: 0 30px;">
                    <div class="slider-track" style="
                        position: absolute;
                        top: 50%;
                        left: 30px;
                        right: 30px;
                        height: 10px;
                        background: linear-gradient(to right, #2a9d8f, #52b788, #f4a261, #e76f51, #e63946);
                        border: 3px solid #8ecae6;
                        transform: translateY(-50%);
                        border-radius: 5px;
                        box-shadow: inset 0 2px 4px rgba(0,0,0,0.3);
                    "></div>
                    <div class="slider-labels" style="position: absolute; top: 90px; left: 30px; right: 30px; display: flex; justify-content: space-between; font-size: 11px; color: var(--color-textPrimary);">
                        <span>0%</span>
                        <span>50%</span>
                        <span>100%</span>
                        <span>150%</span>
                        <span>200%</span>
                    </div>
                    ${weightEffects.filter(e => e.system_check).map((effect, idx) => {
                        const value = effect.system_check?.value || 0;
                        const position = value / 200; // 0.0 to 1.0
                        const isEven = idx % 2 === 0;
                        const topPos = isEven ? '-45px' : '-70px';
                        const color = effect.category === 'buff' ? '#52b788' : '#e63946';
                        return `
                            <div class="breakpoint"
                                 data-effect-id="${effect.id}"
                                 data-value="${value}"
                                 data-system="weight"
                                 data-max="200"
                                 draggable="true"
                                 style="
                                     position: absolute;
                                     top: 50%;
                                     left: calc(30px + (100% - 60px) * ${position});
                                     transform: translate(-50%, -50%);
                                     cursor: grab;
                                     z-index: ${10 + idx};
                                 "
                                 ondragstart="handleBreakpointDragStart(event)"
                                 ondragend="handleBreakpointDragEnd(event)"
                                 onclick="toggleInlineEditor('${effect.id}')">
                                <div style="
                                    width: 18px;
                                    height: 18px;
                                    background: ${color};
                                    border: 4px solid var(--color-bgPrimary);
                                    box-shadow: 0 0 0 2px #8ecae6, 0 2px 4px rgba(0,0,0,0.5);
                                    border-radius: 50%;
                                    pointer-events: none;
                                "></div>
                                <div style="
                                    position: absolute;
                                    top: ${topPos};
                                    left: 50%;
                                    transform: translateX(-50%);
                                    white-space: nowrap;
                                    font-size: 9px;
                                    font-weight: bold;
                                    color: ${color};
                                    background: var(--color-bgPrimary);
                                    padding: 4px 8px;
                                    border: 2px solid #8ecae6;
                                    border-radius: 4px;
                                    pointer-events: none;
                                    box-shadow: 0 2px 4px rgba(0,0,0,0.5);
                                ">${effect.name} (${value}%)</div>
                            </div>
                        `;
                    }).join('')}
                </div>
            </div>

            <div>
                <h4 class="codex-subsection-title">EFFECTS</h4>
                ${weightEffects.map(effect => `
                    <div class="effect-item win95-inset pixel-clip mb-10" style="padding: 1rem;">
                        <div style="display: flex; justify-content: space-between; align-items: center; cursor: pointer;"
                             onclick="toggleInlineEditor('${effect.id}')">
                            <div>
                                <strong style="color: ${effect.category === 'buff' ? 'var(--codex-green)' : 'var(--codex-red)'};">${effect.name}</strong>
                                <span style="color: var(--codex-text-muted); margin-left: 1rem;">
                                    @ Weight ${effect.system_check?.operator || '<='} ${effect.system_check?.value || 0}%
                                </span>
                            </div>
                            <div>
                                <span style="color: var(--codex-text-secondary); font-size: 12px; margin-right: 0.5rem;">
                                    ${formatModifiers(effect.modifiers || [])}
                                </span>
                                <span style="color: var(--codex-text-muted); font-size: 14px;">
                                    ${expandedEffectID === effect.id ? '▼' : '▶'}
                                </span>
                            </div>
                        </div>
                        ${expandedEffectID === effect.id ? renderInlineModifierEditor(effect) : ''}
                    </div>
                `).join('')}
            </div>
        </div>
    `;

    document.getElementById('weight-editor').innerHTML = html;
}

// Update the formula preview when multiplier input changes
window.updateWeightFormulaPreview = function() {
    const input = document.getElementById('weightMultiplierInput');
    const preview = document.getElementById('weightFormulaPreview');
    if (!input || !preview) return;
    const mult = parseFloat(input.value) || 5;
    preview.textContent = `Example: STR 14 × ${mult} = ${14 * mult} lbs base capacity`;
};

// Save the base weight multiplier to encumbrance.json
window.saveWeightMultiplier = async function() {
    const input = document.getElementById('weightMultiplierInput');
    if (!input) return;
    const newMultiplier = parseFloat(input.value);
    if (isNaN(newMultiplier) || newMultiplier <= 0) {
        showStatus('Invalid multiplier value', 'error');
        return;
    }

    // Deep clone config and update the value
    const updatedConfig = JSON.parse(JSON.stringify(encumbranceConfig));
    if (!updatedConfig.encumbrance_system) updatedConfig.encumbrance_system = {};
    if (!updatedConfig.encumbrance_system.weight_calculation) updatedConfig.encumbrance_system.weight_calculation = {};
    updatedConfig.encumbrance_system.weight_calculation.base_weight_multiplier = newMultiplier;

    try {
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) headers['X-Session-ID'] = stagingSessionID;

        const response = await fetch('/api/systems/encumbrance', {
            method: 'PUT',
            headers,
            body: JSON.stringify(updatedConfig)
        });

        if (response.ok) {
            const result = await response.json();
            encumbranceConfig = updatedConfig;
            if (result.mode === 'staging') {
                updateChangeCount(result.changes);
                showStatus(`Weight multiplier staged (${result.changes} changes)`, 'success');
            } else {
                showStatus('✅ Weight multiplier saved', 'success');
            }
        } else {
            showStatus('❌ Failed to save weight multiplier', 'error');
        }
    } catch (error) {
        showStatus('❌ Error: ' + error.message, 'error');
    }
};

// Helper: Format modifiers for display
function formatModifiers(modifiers) {
    if (!modifiers || modifiers.length === 0) return 'No modifiers';
    return modifiers.map(m => {
        const sign = m.value >= 0 ? '+' : '';
        return `${sign}${m.value} ${m.stat}`;
    }).join(', ');
}

// Helper: Render inline modifier editor
function renderInlineModifierEditor(effect) {
    const modifiers = effect.modifiers || [];

    return `
        <div class="inline-modifier-editor" style="margin-top: 1rem; padding: 1rem; background: var(--color-bgPrimary); border-radius: 4px;">
            <h5 style="color: var(--color-textHighlighted); margin-bottom: 0.5rem; font-size: 12px;">EDIT MODIFIERS</h5>
            ${modifiers.map((modifier, index) => `
                <div class="modifier-block" style="background: var(--color-bgSecondary); padding: 0.75rem; margin-bottom: 0.75rem; border-radius: 4px; border: 1px solid var(--color-bgTertiary);">
                    <div style="display: grid; grid-template-columns: 1fr 100px 60px; gap: 0.5rem; margin-bottom: 0.5rem; align-items: center;">
                        <select class="codex-select" style="font-size: 11px; padding: 4px;"
                                onchange="updateInlineModifier('${effect.id}', ${index}, 'stat', this.value)">
                            ${Object.keys(effectTypes.effect_types || {}).map(statID => `
                                <option value="${statID}" ${modifier.stat === statID ? 'selected' : ''}>
                                    ${statID}
                                </option>
                            `).join('')}
                        </select>
                        <input type="number" class="codex-input" style="font-size: 11px; padding: 4px;"
                               value="${modifier.value || 0}"
                               placeholder="Amount"
                               onchange="updateInlineModifier('${effect.id}', ${index}, 'value', parseInt(this.value))" />
                        <button class="codex-btn codex-btn-sm pixel-clip-sm" style="font-size: 10px; padding: 2px 6px;"
                                onclick="removeInlineModifier('${effect.id}', ${index})">✕</button>
                    </div>
                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem;">
                        <div>
                            <label style="display: block; font-size: 10px; color: var(--color-textMuted); margin-bottom: 2px;">Type</label>
                            <select class="codex-select" style="font-size: 11px; padding: 4px;"
                                    onchange="updateInlineModifierType('${effect.id}', ${index}, this.value)">
                                <option value="instant" ${modifier.type === 'instant' ? 'selected' : ''}>Instant (apply once)</option>
                                <option value="constant" ${modifier.type === 'constant' ? 'selected' : ''}>Constant (while active)</option>
                                <option value="periodic" ${modifier.type === 'periodic' ? 'selected' : ''}>Periodic (repeating)</option>
                            </select>
                        </div>
                        ${modifier.type === 'periodic' ? `
                        <div>
                            <label style="display: block; font-size: 10px; color: var(--color-textMuted); margin-bottom: 2px;">Interval (min)</label>
                            <input type="number" class="codex-input" style="font-size: 11px; padding: 4px;"
                                   value="${modifier.tick_interval || 60}"
                                   placeholder="Minutes"
                                   min="1"
                                   onchange="updateInlineModifier('${effect.id}', ${index}, 'tick_interval', parseInt(this.value))" />
                        </div>
                        ` : '<div></div>'}
                    </div>
                </div>
            `).join('')}
            <div style="display: flex; gap: 0.5rem; margin-top: 0.5rem;">
                <button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" style="font-size: 11px; flex: 1;"
                        onclick="addInlineModifier('${effect.id}')">+ ADD MODIFIER</button>
                <button class="codex-btn codex-btn-sm codex-btn-primary pixel-clip-sm" style="font-size: 11px;"
                        onclick="saveInlineEffect('${effect.id}')">💾 SAVE</button>
            </div>
        </div>
    `;
}

// Helper: Edit effect inline (opens effect in sidebar)
window.editEffectInline = function(effectID) {
    // Switch to effects list tab
    document.querySelector('.codex-tab[data-tab="effects"]').click();
    // Select the effect
    selectEffect(effectID);
};

// Update fatigue accumulation rate
window.updateFatigueAccumulationRate = async function(rate) {
    const accEffect = allEffects['fatigue-accumulation'];
    if (!accEffect) {
        showStatus('❌ Fatigue accumulation effect not found', 'error');
        return;
    }

    accEffect.modifiers[0].tick_interval = parseInt(rate);

    try {
        await saveEffectToBackend('fatigue-accumulation', accEffect);
        showStatus('✅ Fatigue accumulation rate updated', 'success');
    } catch (error) {
        showStatus('❌ Failed to update rate', 'error');
    }
};

// Toggle skill scaling on/off for an effect
window.toggleSkillScaling = async function(effectID, enabled) {
    const effect = allEffects[effectID];
    if (!effect) return;

    if (enabled) {
        effect.skill_scaling = {
            skill: 'athletics',
            base_level: 10,
            bonus_per_level: 0.05,
            max_bonus_levels: 10
        };
    } else {
        delete effect.skill_scaling;
    }

    try {
        await saveEffectToBackend(effectID, effect);
        showStatus(`✅ Skill scaling ${enabled ? 'enabled' : 'disabled'}`, 'success');
        renderFatigueSystem();
    } catch (error) {
        showStatus('❌ Failed to update skill scaling', 'error');
    }
};

// Update a skill scaling field for an effect
window.updateSkillScaling = async function(effectID, field, value) {
    const effect = allEffects[effectID];
    if (!effect || !effect.skill_scaling) return;

    effect.skill_scaling[field] = value;

    try {
        await saveEffectToBackend(effectID, effect);
        showStatus('✅ Skill scaling updated', 'success');
        renderFatigueSystem();
    } catch (error) {
        showStatus('❌ Failed to update skill scaling', 'error');
    }
};

// Update hunger accumulation rates
window.updateHungerAccumulationRate = async function(level, rate) {
    const effectID = `hunger-accumulation-${level}`;
    const accEffect = allEffects[effectID];
    if (!accEffect) {
        showStatus(`❌ Hunger accumulation effect not found: ${effectID}`, 'error');
        return;
    }

    accEffect.modifiers[0].tick_interval = parseInt(rate);

    try {
        await saveEffectToBackend(effectID, accEffect);
        showStatus(`✅ Hunger accumulation rate updated (${level})`, 'success');
    } catch (error) {
        showStatus('❌ Failed to update rate', 'error');
    }
};

// ====================
// DRAGGABLE BREAKPOINTS
// ====================

let draggedBreakpoint = null;
let dragStartX = 0;
let isDragging = false;

// Helper: Check if system has pending changes
function hasPendingChanges(system) {
    return Object.values(pendingSliderChanges).some(change => change.system === system);
}

// Helper: Render save button if there are pending changes
function renderSaveButton(system) {
    if (!hasPendingChanges(system)) return '';

    const changes = Object.entries(pendingSliderChanges)
        .filter(([_, change]) => change.system === system);

    const changeText = changes.map(([id, change]) =>
        `${change.effectName}: ${change.oldValue} → ${change.newValue}`
    ).join(', ');

    return `
        <div style="background: #f4a261; padding: 1rem; margin-bottom: 1rem; border-radius: 4px; border: 2px solid #e76f51;">
            <div style="display: flex; justify-content: space-between; align-items: center; gap: 1rem;">
                <div>
                    <strong style="color: #000;">⚠️ Unsaved Changes:</strong>
                    <div style="font-size: 11px; color: #333; margin-top: 0.25rem;">${changeText}</div>
                </div>
                <div style="display: flex; gap: 0.5rem;">
                    <button class="codex-btn codex-btn-primary pixel-clip-sm"
                            onclick="saveSliderChanges('${system}')"
                            style="white-space: nowrap;">💾 SAVE CHANGES</button>
                    <button class="codex-btn pixel-clip-sm"
                            onclick="discardSliderChanges('${system}')"
                            style="white-space: nowrap;">✕ DISCARD</button>
                </div>
            </div>
        </div>
    `;
}

// Save all pending slider changes for a system
window.saveSliderChanges = async function(system) {
    const changes = Object.entries(pendingSliderChanges)
        .filter(([_, change]) => change.system === system);

    if (changes.length === 0) return;

    let savedCount = 0;
    let failedCount = 0;

    for (const [effectID, change] of changes) {
        const effect = allEffects[effectID];
        if (!effect) continue;

        try {
            await saveEffectToBackend(effectID, effect);
            delete pendingSliderChanges[effectID];
            savedCount++;
        } catch (error) {
            console.error(`Failed to save ${effectID}:`, error);
            failedCount++;
        }
    }

    if (failedCount === 0) {
        showStatus(`✅ Saved ${savedCount} threshold change${savedCount !== 1 ? 's' : ''}`, 'success');
    } else {
        showStatus(`⚠️ Saved ${savedCount}, failed ${failedCount}`, 'warning');
    }

    // Re-render to hide save button
    if (system === 'fatigue') {
        renderFatigueSystem();
    } else if (system === 'hunger') {
        renderHungerSystem();
    } else if (system === 'weight') {
        renderWeightSystem();
    }
};

// Discard pending slider changes for a system
window.discardSliderChanges = function(system) {
    const changes = Object.entries(pendingSliderChanges)
        .filter(([_, change]) => change.system === system);

    for (const [effectID, change] of changes) {
        const effect = allEffects[effectID];
        if (effect && effect.system_check) {
            // Revert to old value
            effect.system_check.value = change.oldValue;
        }
        delete pendingSliderChanges[effectID];
    }

    showStatus('Changes discarded', 'info');

    // Re-render to show original positions
    if (system === 'fatigue') {
        renderFatigueSystem();
    } else if (system === 'hunger') {
        renderHungerSystem();
    } else if (system === 'weight') {
        renderWeightSystem();
    }
};

window.handleBreakpointDragStart = function(event) {
    draggedBreakpoint = event.target;
    dragStartX = event.clientX;
    isDragging = false;
    draggedBreakpoint.style.opacity = '0.5';
    draggedBreakpoint.style.cursor = 'grabbing';
    event.dataTransfer.effectAllowed = 'move';
};

window.handleBreakpointDragEnd = function(event) {
    if (!draggedBreakpoint) return;

    const dragEndX = event.clientX;
    const dragDistance = Math.abs(dragEndX - dragStartX);

    // If dragged more than 5px, consider it a drag (not a click)
    if (dragDistance > 5) {
        isDragging = true;
        event.preventDefault();
        event.stopPropagation();

        // Get slider container
        const slider = draggedBreakpoint.parentElement;
        const sliderRect = slider.getBoundingClientRect();

        // Get padding based on system
        const system = draggedBreakpoint.dataset.system;
        const padding = (system === 'hunger') ? 40 : 30;

        // Calculate new position (accounting for padding)
        const relativeX = event.clientX - sliderRect.left - padding;
        const trackWidth = sliderRect.width - (padding * 2);
        const percentage = Math.max(0, Math.min(1, relativeX / trackWidth));

        // Get system info
        const maxValue = parseInt(draggedBreakpoint.dataset.max);
        const effectID = draggedBreakpoint.dataset.effectId;

        // Calculate new value and SNAP to integer
        let newValue = Math.round(percentage * maxValue);

        // Clamp to valid range
        newValue = Math.max(0, Math.min(maxValue, newValue));

        // Stage the change (don't save yet)
        const effect = allEffects[effectID];
        if (effect && effect.system_check) {
            const oldValue = effect.system_check.value;

            if (newValue !== oldValue) {
                // Store pending change
                pendingSliderChanges[effectID] = {
                    oldValue: oldValue,
                    newValue: newValue,
                    system: system,
                    effectName: effect.name
                };

                // Update in memory (for immediate visual feedback)
                effect.system_check.value = newValue;

                // Re-render to show snapped position and save button
                if (system === 'fatigue') {
                    renderFatigueSystem();
                } else if (system === 'hunger') {
                    renderHungerSystem();
                } else if (system === 'weight') {
                    renderWeightSystem();
                }

                showStatus(`📝 ${effect.name} threshold changed: ${oldValue} → ${newValue} (not saved yet)`, 'info');
            }
        }
    }

    // Reset drag state
    if (draggedBreakpoint) {
        draggedBreakpoint.style.opacity = '1';
        draggedBreakpoint.style.cursor = 'grab';
    }
    draggedBreakpoint = null;

    // Prevent click event if we dragged
    setTimeout(() => {
        isDragging = false;
    }, 10);
};

// Override editEffectInline to check if dragging
const originalEditEffectInline = window.editEffectInline;
window.editEffectInline = function(effectID) {
    if (isDragging) {
        return; // Don't open effect if we're dragging
    }
    originalEditEffectInline(effectID);
};

// ====================
// INLINE MODIFIER EDITING
// ====================

let expandedEffectID = null;

window.toggleInlineEditor = function(effectID) {
    if (expandedEffectID === effectID) {
        expandedEffectID = null;
    } else {
        expandedEffectID = effectID;
    }

    // Re-render current system to show/hide editor
    const activeTab = document.querySelector('.codex-tab.active')?.dataset.tab;
    if (activeTab === 'fatigue') {
        renderFatigueSystem();
    } else if (activeTab === 'hunger') {
        renderHungerSystem();
    } else if (activeTab === 'weight') {
        renderWeightSystem();
    }
};

window.updateInlineModifier = function(effectID, index, field, value) {
    const effect = allEffects[effectID];
    if (!effect || !effect.modifiers || !effect.modifiers[index]) return;
    effect.modifiers[index][field] = value;
};

window.updateInlineModifierType = function(effectID, index, type) {
    const effect = allEffects[effectID];
    if (!effect || !effect.modifiers || !effect.modifiers[index]) return;

    effect.modifiers[index].type = type;

    // Clear tick_interval if not periodic
    if (type !== 'periodic') {
        effect.modifiers[index].tick_interval = 0;
    } else if (!effect.modifiers[index].tick_interval) {
        // Set default tick_interval for periodic
        effect.modifiers[index].tick_interval = 60;
    }

    // Re-render to show/hide tick_interval field
    const activeTab = document.querySelector('.codex-tab.active')?.dataset.tab;
    if (activeTab === 'fatigue') {
        renderFatigueSystem();
    } else if (activeTab === 'hunger') {
        renderHungerSystem();
    } else if (activeTab === 'weight') {
        renderWeightSystem();
    }
};

window.addInlineModifier = function(effectID) {
    const effect = allEffects[effectID];
    if (!effect) return;

    if (!effect.modifiers) {
        effect.modifiers = [];
    }

    const firstType = Object.keys(effectTypes.effect_types || {})[0] || '';
    effect.modifiers.push({
        stat: firstType,
        value: 0,
        type: 'constant',
        delay: 0,
        tick_interval: 0
    });

    // Re-render to show new modifier
    const activeTab = document.querySelector('.codex-tab.active')?.dataset.tab;
    if (activeTab === 'fatigue') {
        renderFatigueSystem();
    } else if (activeTab === 'hunger') {
        renderHungerSystem();
    } else if (activeTab === 'weight') {
        renderWeightSystem();
    }
};

window.removeInlineModifier = function(effectID, index) {
    const effect = allEffects[effectID];
    if (!effect || !effect.modifiers) return;

    effect.modifiers.splice(index, 1);

    // Re-render to update display
    const activeTab = document.querySelector('.codex-tab.active')?.dataset.tab;
    if (activeTab === 'fatigue') {
        renderFatigueSystem();
    } else if (activeTab === 'hunger') {
        renderHungerSystem();
    } else if (activeTab === 'weight') {
        renderWeightSystem();
    }
};

window.saveInlineEffect = async function(effectID) {
    const effect = allEffects[effectID];
    if (!effect) return;

    try {
        await saveEffectToBackend(effectID, effect);
        showStatus('✅ Effect modifiers saved', 'success');

        // Collapse editor after save
        expandedEffectID = null;

        // Re-render to collapse
        const activeTab = document.querySelector('.codex-tab.active')?.dataset.tab;
        if (activeTab === 'fatigue') {
            renderFatigueSystem();
        } else if (activeTab === 'hunger') {
            renderHungerSystem();
        } else if (activeTab === 'weight') {
            renderWeightSystem();
        }
    } catch (error) {
        showStatus('❌ Failed to save modifiers', 'error');
    }
};

// Render effect types editor
function renderEffectTypesEditor() {
    const types = effectTypes.effect_types || {};

    let html = `
        <div id="effect-types-list">
            ${Object.entries(types).map(([id, type]) => `
                <div class="effect-type-item win95-inset pixel-clip" style="margin-bottom: 1rem; padding: 1rem;">
                    <div class="grid grid-cols-3 gap-4">
                        <div>
                            <label class="codex-label">TYPE ID</label>
                            <input type="text" class="codex-input" value="${id}" disabled />
                        </div>
                        <div>
                            <label class="codex-label">PROPERTY PATH</label>
                            <input type="text" class="codex-input" value="${type.property || ''}"
                                   onchange="updateEffectType('${id}', 'property', this.value)" />
                        </div>
                        <div>
                            <label class="codex-label">DESCRIPTION</label>
                            <input type="text" class="codex-input" value="${type.description || ''}"
                                   onchange="updateEffectType('${id}', 'description', this.value)" />
                        </div>
                    </div>
                </div>
            `).join('')}
        </div>
        <button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm mt-10"
                onclick="addEffectType()">+ ADD EFFECT TYPE</button>
    `;

    document.getElementById('effect-types-editor').innerHTML = html;
}

// Update effect type
window.updateEffectType = function(id, field, value) {
    if (!effectTypes.effect_types[id]) return;
    effectTypes.effect_types[id][field] = value;
};

// Add effect type
window.addEffectType = function() {
    const typeID = prompt('Enter new effect type ID (lowercase, hyphens only):');
    if (!typeID) return;

    if (!/^[a-z0-9-]+$/.test(typeID)) {
        showStatus('Invalid type ID. Use lowercase letters, numbers, and hyphens only.', 'error');
        return;
    }

    if (effectTypes.effect_types[typeID]) {
        showStatus('Effect type already exists!', 'error');
        return;
    }

    effectTypes.effect_types[typeID] = {
        id: typeID,
        property: '',
        description: ''
    };

    renderEffectTypesEditor();
};

// Save effect types
async function saveEffectTypes() {
    try {
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch('/api/effect-types', {
            method: 'PUT',
            headers: headers,
            body: JSON.stringify(effectTypes)
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();

        if (result.mode === 'staging') {
            showStatus(`✅ Effect types staged (${result.changes} total changes)`, 'success');
            updateChangeCount(result.changes);
        } else {
            showStatus('✅ Effect types saved successfully', 'success');
        }
    } catch (error) {
        console.error('❌ Save failed:', error);
        showStatus(`❌ ${error.message}`, 'error');
    }
}

// Staging panel functions
async function viewChanges() {
    if (!stagingSessionID) return;

    try {
        const response = await fetch(`/api/staging/changes?session_id=${stagingSessionID}`);
        const session = await response.json();

        if (!session.changes || session.changes.length === 0) {
            showStatus('No changes to view', 'info');
            return;
        }

        let changeText = `STAGING SESSION: ${session.session_id}\n\n`;
        changeText += `${session.changes.length} file(s) changed:\n\n`;

        session.changes.forEach((change, i) => {
            changeText += `${i + 1}. [${change.type.toUpperCase()}] ${change.file_path}\n`;
        });

        alert(changeText);
    } catch (error) {
        console.error('❌ Failed to view changes:', error);
        showStatus('Failed to load changes', 'error');
    }
}

async function submitPR() {
    if (!stagingSessionID) return;

    if (!confirm('Submit all staged changes as a pull request?')) {
        return;
    }

    try {
        const response = await fetch('/api/staging/submit', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ session_id: stagingSessionID })
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();

        showStatus(`✅ PR submitted: ${result.pr_url}`, 'success');
        updateChangeCount(0);

        // Clear session
        localStorage.removeItem('codex_staging_session');
        stagingSessionID = null;

        // Reload to create new session
        setTimeout(() => location.reload(), 2000);
    } catch (error) {
        console.error('❌ PR submission failed:', error);
        showStatus(`❌ ${error.message}`, 'error');
    }
}

async function clearStaging() {
    if (!stagingSessionID) return;

    if (!confirm('Clear all staged changes? This cannot be undone.')) {
        return;
    }

    try {
        const response = await fetch(`/api/staging/clear?session_id=${stagingSessionID}`, {
            method: 'DELETE'
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        showStatus('✅ Staging cleared', 'success');
        updateChangeCount(0);

        // Clear session
        localStorage.removeItem('codex_staging_session');
        stagingSessionID = null;

        // Reload to create new session
        setTimeout(() => location.reload(), 1000);
    } catch (error) {
        console.error('❌ Clear failed:', error);
        showStatus(`❌ ${error.message}`, 'error');
    }
}

// Update change count
function updateChangeCount(count) {
    const badge = document.getElementById('staging-badge');
    const counter = document.getElementById('change-count');

    if (badge) badge.textContent = count;
    if (counter) counter.textContent = `${count} change${count !== 1 ? 's' : ''}`;
}

// Show status message
function showStatus(message, type = 'info') {
    const container = document.getElementById('status-container');
    const statusDiv = document.createElement('div');
    statusDiv.className = `status-message status-${type}`;
    statusDiv.textContent = message;

    container.appendChild(statusDiv);

    setTimeout(() => {
        statusDiv.classList.add('fade-out');
        setTimeout(() => statusDiv.remove(), 300);
    }, 3000);
}

// ============================================================================
// Skills Editor
// ============================================================================

const ABILITY_SCORES = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'];
const ABILITY_LABELS = { strength: 'STR', dexterity: 'DEX', constitution: 'CON', intelligence: 'INT', wisdom: 'WIS', charisma: 'CHA' };
const ABILITY_COLORS = { strength: '#ef4444', dexterity: '#22c55e', constitution: '#f97316', intelligence: '#3b82f6', wisdom: '#a855f7', charisma: '#ec4899' };

let currentSkillID = null;

function renderSkillsEditor() {
    const container = document.getElementById('skills-editor');
    if (!container) return;

    const skills = Object.entries(skillsConfig).sort((a, b) => a[1].name.localeCompare(b[1].name));

    let html = '<div class="codex-section win95-inset pixel-clip">';
    html += '<h3 class="codex-section-title">SKILL DEFINITIONS</h3>';
    html += '<p class="text-muted mb-10">Skills are calculated from weighted ability score ratios. Click a skill to edit its ratios.</p>';

    // Skills grid - overview
    html += '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 16px;">';
    for (const [id, skill] of skills) {
        const isSelected = id === currentSkillID;
        html += `<div class="skill-card ${isSelected ? 'selected' : ''}" data-skill-id="${id}"
                      style="background: ${isSelected ? 'var(--codex-bg-highlight, #2a2a3a)' : 'var(--codex-bg-card, #1e1e2e)'};
                             border: 1px solid ${isSelected ? 'var(--codex-yellow, #ffd700)' : '#333'};
                             padding: 8px; cursor: pointer; border-radius: 2px;">`;
        html += `<div style="font-weight: bold; color: var(--codex-yellow, #ffd700); font-size: 12px; margin-bottom: 4px;">${skill.name}</div>`;
        html += `<div style="font-size: 10px; color: var(--codex-text-muted, #888); margin-bottom: 6px;">${skill.description}</div>`;

        // Ratio bar visualization
        html += renderRatioBar(skill.ratio);
        html += '</div>';
    }
    html += '</div>';

    // Detail editor (if skill selected)
    if (currentSkillID && skillsConfig[currentSkillID]) {
        html += renderSkillDetailEditor(currentSkillID, skillsConfig[currentSkillID]);
    }

    html += '</div>';
    container.innerHTML = html;

    // Bind click events on skill cards
    container.querySelectorAll('.skill-card').forEach(card => {
        card.addEventListener('click', () => {
            currentSkillID = card.dataset.skillId;
            renderSkillsEditor();
        });
    });

    // Bind ratio input events
    container.querySelectorAll('.skill-ratio-input').forEach(input => {
        input.addEventListener('change', handleSkillRatioChange);
        input.addEventListener('input', handleSkillRatioPreview);
    });

    // Bind save button
    const saveBtn = container.querySelector('#save-skills-btn');
    if (saveBtn) {
        saveBtn.addEventListener('click', saveSkills);
    }
}

function renderRatioBar(ratio) {
    let html = '<div style="display: flex; height: 6px; border-radius: 1px; overflow: hidden;">';
    for (const stat of ABILITY_SCORES) {
        const weight = ratio[stat] || 0;
        if (weight > 0) {
            const pct = (weight * 100).toFixed(0);
            html += `<div style="width: ${pct}%; background: ${ABILITY_COLORS[stat]};" title="${ABILITY_LABELS[stat]}: ${pct}%"></div>`;
        }
    }
    html += '</div>';

    // Labels below bar
    html += '<div style="display: flex; gap: 4px; margin-top: 2px; font-size: 9px;">';
    for (const stat of ABILITY_SCORES) {
        const weight = ratio[stat] || 0;
        if (weight > 0) {
            const pct = (weight * 100).toFixed(0);
            html += `<span style="color: ${ABILITY_COLORS[stat]};">${ABILITY_LABELS[stat]} ${pct}%</span>`;
        }
    }
    html += '</div>';
    return html;
}

function renderSkillDetailEditor(id, skill) {
    let html = '<div class="codex-section win95-inset pixel-clip" style="margin-top: 8px;">';
    html += `<h3 class="codex-section-title">EDITING: ${skill.name.toUpperCase()}</h3>`;

    // Ratio inputs
    html += '<div style="display: grid; grid-template-columns: repeat(6, 1fr); gap: 8px; margin-bottom: 12px;">';
    for (const stat of ABILITY_SCORES) {
        const weight = skill.ratio[stat] || 0;
        const pct = Math.round(weight * 100);
        html += `<div style="text-align: center;">`;
        html += `<div style="color: ${ABILITY_COLORS[stat]}; font-weight: bold; font-size: 11px; margin-bottom: 2px;">${ABILITY_LABELS[stat]}</div>`;
        html += `<input type="number" class="skill-ratio-input" data-skill="${id}" data-stat="${stat}"
                        value="${pct}" min="0" max="100" step="5"
                        style="width: 50px; text-align: center; background: #111; color: white; border: 1px solid #444; padding: 2px; font-size: 11px;">`;
        html += `<div style="font-size: 9px; color: #666;">%</div>`;
        html += '</div>';
    }
    html += '</div>';

    // Total indicator
    const total = ABILITY_SCORES.reduce((sum, stat) => sum + Math.round((skill.ratio[stat] || 0) * 100), 0);
    const totalColor = total === 100 ? '#4ade80' : '#f87171';
    html += `<div style="text-align: center; margin-bottom: 12px;">`;
    html += `<span style="color: ${totalColor}; font-size: 12px; font-weight: bold;">Total: <span id="skill-ratio-total">${total}</span>%</span>`;
    if (total !== 100) {
        html += `<span style="color: #f87171; font-size: 10px; margin-left: 8px;">(must equal 100%)</span>`;
    }
    html += '</div>';

    // Preview bar
    html += '<div style="margin-bottom: 12px;">';
    html += renderRatioBar(skill.ratio);
    html += '</div>';

    // Save button
    html += '<div class="codex-btn-group">';
    html += `<button id="save-skills-btn" class="codex-btn codex-btn-primary pixel-clip-sm" ${total !== 100 ? 'disabled style="opacity: 0.5;"' : ''}>SAVE SKILLS</button>`;
    html += '</div>';

    html += '</div>';
    return html;
}

function handleSkillRatioChange(e) {
    const skillId = e.target.dataset.skill;
    const stat = e.target.dataset.stat;
    const pct = parseInt(e.target.value) || 0;

    if (!skillsConfig[skillId]) return;

    // Update the ratio
    if (pct === 0) {
        delete skillsConfig[skillId].ratio[stat];
    } else {
        skillsConfig[skillId].ratio[stat] = pct / 100;
    }

    // Re-render to update total and bar
    renderSkillsEditor();
}

function handleSkillRatioPreview(e) {
    // Live-update the total as user types
    const skillId = e.target.dataset.skill;
    if (!skillsConfig[skillId]) return;

    let total = 0;
    document.querySelectorAll(`.skill-ratio-input[data-skill="${skillId}"]`).forEach(input => {
        total += parseInt(input.value) || 0;
    });

    const totalEl = document.getElementById('skill-ratio-total');
    if (totalEl) {
        totalEl.textContent = total;
        totalEl.parentElement.style.color = total === 100 ? '#4ade80' : '#f87171';
    }
}

async function saveSkills() {
    // Validate all skills total to 100%
    for (const [id, skill] of Object.entries(skillsConfig)) {
        const total = Object.values(skill.ratio).reduce((sum, v) => sum + v, 0);
        if (Math.abs(total - 1.0) > 0.01) {
            showStatus(`${skill.name} ratios must sum to 100% (currently ${Math.round(total * 100)}%)`, 'error');
            return;
        }
    }

    try {
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch('/api/systems/skills', {
            method: 'PUT',
            headers,
            body: JSON.stringify(skillsConfig)
        });

        if (!response.ok) {
            throw new Error(await response.text());
        }

        const result = await response.json();

        if (result.mode === 'staging') {
            updateChangeCount(result.changes);
            showStatus('Skills staged for review', 'success');
        } else {
            showStatus('Skills saved', 'success');
        }
    } catch (error) {
        console.error('Failed to save skills:', error);
        showStatus('Failed to save skills: ' + error.message, 'error');
    }
}

// ============================================================
// TRAVEL SYSTEM
// ============================================================

function renderTravelSystem() {
    const container = document.getElementById('travel-editor');
    if (!container) return;

    const scaling = travelConfig.skill_scaling || { skill: 'athletics', base_level: 10, bonus_per_level: 0.02, max_bonus_levels: 5 };

    // Build skill options from skillsConfig
    const skillOptions = Object.entries(skillsConfig)
        .sort((a, b) => a[1].name.localeCompare(b[1].name))
        .map(([id, s]) => `<option value="${id}" ${id === scaling.skill ? 'selected' : ''}>${s.name}</option>`)
        .join('');

    const maxBonus = (scaling.bonus_per_level * scaling.max_bonus_levels * 100).toFixed(0);

    let html = '<div class="codex-section win95-inset pixel-clip">';
    html += '<h3 class="codex-section-title">TRAVEL SPEED SCALING</h3>';
    html += '<p class="text-muted mb-10">Configure how skills affect travel speed through environments. Higher skill = faster travel.</p>';

    // Formula explanation
    html += '<div style="background: #1a1a2e; border: 1px solid #333; padding: 12px; margin-bottom: 16px; border-radius: 2px;">';
    html += '<div style="font-size: 11px; color: var(--codex-yellow, #ffd700); margin-bottom: 6px; font-weight: bold;">FORMULA</div>';
    html += '<div style="font-size: 12px; color: #ccc; font-family: monospace;">';
    html += `speed_multiplier = 1 + min(skill - base_level, max_bonus_levels) × bonus_per_level`;
    html += '</div>';
    html += `<div style="font-size: 10px; color: #888; margin-top: 6px;">Example: ${scaling.skill} ${scaling.base_level + scaling.max_bonus_levels} → ${(1 + scaling.max_bonus_levels * scaling.bonus_per_level).toFixed(2)}x speed (${maxBonus}% faster)</div>`;
    html += '</div>';

    // Settings grid
    html += '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 16px;">';

    // Skill selector
    html += '<div>';
    html += '<label style="display: block; font-size: 11px; color: var(--codex-yellow, #ffd700); margin-bottom: 4px; font-weight: bold;">SKILL</label>';
    html += `<select id="travel-skill" style="width: 100%; background: #111; color: white; border: 1px solid #444; padding: 4px; font-size: 12px;">`;
    html += skillOptions;
    html += '</select>';
    html += '</div>';

    // Base level
    html += '<div>';
    html += '<label style="display: block; font-size: 11px; color: var(--codex-yellow, #ffd700); margin-bottom: 4px; font-weight: bold;">BASE LEVEL (no bonus at or below)</label>';
    html += `<input type="number" id="travel-base-level" value="${scaling.base_level}" min="1" max="30" style="width: 100%; background: #111; color: white; border: 1px solid #444; padding: 4px; font-size: 12px;">`;
    html += '</div>';

    // Bonus per level
    html += '<div>';
    html += '<label style="display: block; font-size: 11px; color: var(--codex-yellow, #ffd700); margin-bottom: 4px; font-weight: bold;">BONUS PER LEVEL (%)</label>';
    html += `<input type="number" id="travel-bonus-per-level" value="${(scaling.bonus_per_level * 100).toFixed(0)}" min="1" max="50" step="1" style="width: 100%; background: #111; color: white; border: 1px solid #444; padding: 4px; font-size: 12px;">`;
    html += '<div style="font-size: 9px; color: #666; margin-top: 2px;">Speed increase per skill level above base</div>';
    html += '</div>';

    // Max bonus levels
    html += '<div>';
    html += '<label style="display: block; font-size: 11px; color: var(--codex-yellow, #ffd700); margin-bottom: 4px; font-weight: bold;">MAX BONUS LEVELS</label>';
    html += `<input type="number" id="travel-max-bonus-levels" value="${scaling.max_bonus_levels}" min="1" max="20" style="width: 100%; background: #111; color: white; border: 1px solid #444; padding: 4px; font-size: 12px;">`;
    html += '<div style="font-size: 9px; color: #666; margin-top: 2px;">Caps how many levels above base count</div>';
    html += '</div>';

    html += '</div>';

    // Preview table
    html += '<div style="margin-bottom: 16px;">';
    html += '<div style="font-size: 11px; color: var(--codex-yellow, #ffd700); margin-bottom: 6px; font-weight: bold;">PREVIEW</div>';
    html += '<div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 4px;">';
    for (let lvl = scaling.base_level; lvl <= scaling.base_level + scaling.max_bonus_levels + 1; lvl++) {
        const bonus = Math.min(lvl - scaling.base_level, scaling.max_bonus_levels);
        const mult = bonus > 0 ? (1 + bonus * scaling.bonus_per_level) : 1.0;
        const isCapped = lvl > scaling.base_level + scaling.max_bonus_levels;
        const color = bonus <= 0 ? '#888' : isCapped ? '#f97316' : '#4ade80';
        html += `<div style="background: #111; border: 1px solid #333; padding: 4px 8px; font-size: 11px; border-radius: 2px;">`;
        html += `<span style="color: ${color};">Lvl ${lvl}</span>`;
        html += `<span style="color: #ccc; float: right;">${mult.toFixed(2)}x</span>`;
        if (isCapped) html += `<div style="font-size: 9px; color: #f97316;">capped</div>`;
        html += '</div>';
    }
    html += '</div>';
    html += '</div>';

    // Save button
    html += '<div class="codex-btn-group">';
    html += '<button id="save-travel-config-btn" class="codex-btn codex-btn-primary pixel-clip-sm">SAVE TRAVEL CONFIG</button>';
    html += '</div>';

    html += '</div>';
    container.innerHTML = html;

    // Bind input change events to live-update preview
    const inputs = ['travel-skill', 'travel-base-level', 'travel-bonus-per-level', 'travel-max-bonus-levels'];
    inputs.forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            el.addEventListener('change', () => {
                updateTravelConfigFromInputs();
                renderTravelSystem();
            });
        }
    });

    // Bind save button
    const saveBtn = document.getElementById('save-travel-config-btn');
    if (saveBtn) {
        saveBtn.addEventListener('click', saveTravelConfig);
    }
}

function updateTravelConfigFromInputs() {
    if (!travelConfig.skill_scaling) {
        travelConfig.skill_scaling = {};
    }
    travelConfig.skill_scaling.skill = document.getElementById('travel-skill').value;
    travelConfig.skill_scaling.base_level = parseInt(document.getElementById('travel-base-level').value) || 10;
    travelConfig.skill_scaling.bonus_per_level = (parseInt(document.getElementById('travel-bonus-per-level').value) || 2) / 100;
    travelConfig.skill_scaling.max_bonus_levels = parseInt(document.getElementById('travel-max-bonus-levels').value) || 5;
}

async function saveTravelConfig() {
    updateTravelConfigFromInputs();

    try {
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch('/api/systems/travel-config', {
            method: 'PUT',
            headers,
            body: JSON.stringify(travelConfig)
        });

        if (!response.ok) {
            throw new Error(await response.text());
        }

        const result = await response.json();

        if (result.mode === 'staging') {
            updateChangeCount(result.changes);
            showStatus('Travel config staged for review', 'success');
        } else {
            showStatus('Travel config saved', 'success');
        }
    } catch (error) {
        console.error('Failed to save travel config:', error);
        showStatus('Failed to save travel config: ' + error.message, 'error');
    }
}
