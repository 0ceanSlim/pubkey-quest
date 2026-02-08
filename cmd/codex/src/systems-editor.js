// Systems Editor - Main JavaScript
// Manages effects, effect types, and game systems

// State
let allEffects = {};           // Map of effect ID -> Effect
let effectTypes = {};          // EffectTypeDefinition map
let currentEffectID = null;    // Selected effect for editing
let stagingSessionID = null;
let stagingMode = null;

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

            // Special handling for effect types tab
            if (tabName === 'effect-types') {
                renderEffectTypesEditor();
            }
        });
    });
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

// Render effect list
function renderEffectList() {
    const effects = Object.values(allEffects);

    // Categorize effects
    const systemTickers = effects.filter(e => e.category === 'system');
    const statuses = effects.filter(e => e.category !== 'system' && e.effects && e.effects.some(eff => eff.duration === 0));
    const conditions = effects.filter(e => e.category !== 'system' && e.effects && e.effects.every(eff => eff.duration > 0));

    let html = '';

    // System Tickers
    if (systemTickers.length > 0) {
        html += '<div class="effect-group-header">SYSTEM TICKERS</div>';
        systemTickers.sort((a, b) => a.name.localeCompare(b.name)).forEach(effect => {
            html += renderEffectListItem(effect);
        });
    }

    // Statuses (duration = 0)
    if (statuses.length > 0) {
        html += '<div class="effect-group-header">STATUSES</div>';
        statuses.sort((a, b) => a.name.localeCompare(b.name)).forEach(effect => {
            html += renderEffectListItem(effect);
        });
    }

    // Conditions (duration > 0)
    if (conditions.length > 0) {
        html += '<div class="effect-group-header">CONDITIONS</div>';
        conditions.sort((a, b) => a.name.localeCompare(b.name)).forEach(effect => {
            html += renderEffectListItem(effect);
        });
    }

    document.getElementById('effect-list').innerHTML = html;
}

// Render individual effect list item
function renderEffectListItem(effect) {
    const effectType = effect.category === 'system' ? 'system' :
                       (effect.effects && effect.effects.some(e => e.duration === 0) ? 'status' : 'condition');
    const tagColor = effectType === 'system' ? 'var(--codex-cyan)' :
                     (effectType === 'status' ? 'var(--codex-yellow)' : 'var(--codex-purple)');

    return `
        <li class="codex-list-item ${currentEffectID === effect.id ? 'active' : ''}"
            onclick="selectEffect('${effect.id}')" style="cursor: pointer;">
            <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
                <span style="color: var(--codex-text-primary);">
                    ${effect.name || effect.id}
                </span>
                <span class="codex-tag" style="font-size: 10px; background-color: ${tagColor};">
                    ${effectType.toUpperCase()}
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

    // Determine effect type for display
    const isSystemTicker = effect.category === 'system';
    const isStatus = !isSystemTicker && effect.effects && effect.effects.some(e => e.duration === 0);
    const isCondition = !isSystemTicker && !isStatus;

    let html = `
        <div class="codex-section win95-inset pixel-clip">
            <h3 class="codex-section-title">EFFECT: ${effect.name}</h3>
            <div class="mb-15" style="color: var(--codex-text-secondary); font-size: 12px;">
                Type: ${isSystemTicker ? '⚙️ System Ticker' : (isStatus ? '📌 Status (persistent)' : '⏱️ Condition (temporary)')}
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
                    <label class="codex-label">CATEGORY</label>
                    <select class="codex-select" onchange="updateEffectField('category', this.value)">
                        <option value="">None</option>
                        <option value="buff" ${effect.category === 'buff' ? 'selected' : ''}>Buff</option>
                        <option value="debuff" ${effect.category === 'debuff' ? 'selected' : ''}>Debuff</option>
                        <option value="system" ${effect.category === 'system' ? 'selected' : ''}>System</option>
                    </select>
                </div>
                <div>
                    <label class="codex-label">SILENT (system tickers)</label>
                    <select class="codex-select" onchange="updateEffectField('silent', this.value === 'true' ? true : (this.value === 'false' ? false : null))">
                        <option value="">Not set</option>
                        <option value="true" ${effect.silent === true ? 'selected' : ''}>True</option>
                        <option value="false" ${effect.silent === false ? 'selected' : ''}>False</option>
                    </select>
                </div>
            </div>

            <label class="codex-label">MESSAGE (shown when effect is applied)</label>
            <input type="text" class="codex-input mb-15" value="${effect.message || ''}"
                   placeholder="Optional message"
                   onchange="updateEffectField('message', this.value)" />

            <details style="margin-bottom: 15px;">
                <summary style="cursor: pointer; color: var(--codex-text-secondary); user-select: none;">Advanced: Icon & Color (optional)</summary>
                <div class="grid grid-cols-2 gap-4 mt-10">
                    <div>
                        <label class="codex-label">ICON (filename from /systems/effects/)</label>
                        <input type="text" class="codex-input" value="${effect.icon || ''}"
                               placeholder="e.g., fatigued, hungry, blessed"
                               onchange="updateEffectField('icon', this.value)" />
                    </div>
                    <div>
                        <label class="codex-label">COLOR</label>
                        <input type="text" class="codex-input" value="${effect.color || ''}"
                               placeholder="e.g., red, #ff0000"
                               onchange="updateEffectField('color', this.value)" />
                    </div>
                </div>
            </details>

            <h4 class="codex-subsection-title">EFFECT DETAILS</h4>
            <div id="effect-details-container">
                ${renderEffectDetails(effect.effects || [])}
            </div>
            <button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm"
                    onclick="addEffectDetail()">+ ADD EFFECT DETAIL</button>

            <div class="codex-btn-group mt-20">
                <button class="codex-btn codex-btn-primary pixel-clip-sm" onclick="saveEffect()">💾 SAVE EFFECT</button>
                <button class="codex-btn pixel-clip-sm" onclick="deleteEffect()">🗑️ DELETE</button>
            </div>
        </div>
    `;

    document.getElementById('effect-form').innerHTML = html;
}

// Render effect details
function renderEffectDetails(effectDetails) {
    if (!effectDetails || effectDetails.length === 0) {
        return '<p style="color: var(--codex-text-muted); text-align: center; padding: 1rem;">No effect details. Add one to get started.</p>';
    }

    return effectDetails.map((detail, index) => `
        <div class="effect-detail win95-inset pixel-clip" style="margin-bottom: 1rem; padding: 1rem;">
            <div class="grid grid-cols-2 gap-4 mb-10">
                <div>
                    <label class="codex-label">TYPE</label>
                    <select class="codex-select" onchange="updateEffectDetail(${index}, 'type', this.value)">
                        ${Object.keys(effectTypes.effect_types || {}).map(typeID => `
                            <option value="${typeID}" ${detail.type === typeID ? 'selected' : ''}>
                                ${typeID} - ${effectTypes.effect_types[typeID].description}
                            </option>
                        `).join('')}
                    </select>
                </div>
                <div>
                    <label class="codex-label">VALUE</label>
                    <input type="number" class="codex-input" value="${detail.value || 0}"
                           onchange="updateEffectDetail(${index}, 'value', parseInt(this.value))" />
                </div>
            </div>

            <div class="grid grid-cols-3 gap-4">
                <div>
                    <label class="codex-label">DURATION (seconds)</label>
                    <input type="number" class="codex-input" value="${detail.duration || 0}"
                           onchange="updateEffectDetail(${index}, 'duration', parseInt(this.value))" />
                </div>
                <div>
                    <label class="codex-label">DELAY (seconds)</label>
                    <input type="number" class="codex-input" value="${detail.delay || 0}"
                           onchange="updateEffectDetail(${index}, 'delay', parseInt(this.value))" />
                </div>
                <div>
                    <label class="codex-label">TICK INTERVAL (optional)</label>
                    <input type="number" class="codex-input" value="${detail.tick_interval || ''}"
                           placeholder="seconds"
                           onchange="updateEffectDetail(${index}, 'tick_interval', parseInt(this.value) || null)" />
                </div>
            </div>

            <button class="codex-btn codex-btn-sm pixel-clip-sm mt-10"
                    onclick="removeEffectDetail(${index})">REMOVE</button>
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

// Update effect detail
window.updateEffectDetail = function(index, field, value) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect || !effect.effects) return;

    effect.effects[index][field] = value;
};

// Add effect detail
window.addEffectDetail = function() {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect) return;

    if (!effect.effects) {
        effect.effects = [];
    }

    // Get first effect type or empty
    const firstType = Object.keys(effectTypes.effect_types || {})[0] || '';

    effect.effects.push({
        type: firstType,
        value: 0,
        duration: 0,
        delay: 0
    });

    populateEffectForm(currentEffectID);
};

// Remove effect detail
window.removeEffectDetail = function(index) {
    if (!currentEffectID) return;
    const effect = allEffects[currentEffectID];
    if (!effect || !effect.effects) return;

    effect.effects.splice(index, 1);
    populateEffectForm(currentEffectID);
};

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

    // Create new effect
    const newEffect = {
        id: effectID,
        name: effectID.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase()),
        description: '',
        effects: [],
        icon: '⚡',
        color: '',
        category: 'effect'
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
