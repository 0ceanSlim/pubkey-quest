// Starting Gear Editor - Entry Point

// State
let allClasses = [];           // Array of StartingGearEntry
let currentClassIndex = null;  // Selected class index
let allItems = {};             // Shared with item editor (for autocomplete)
let stagingSessionID = null;
let stagingMode = null;
let characterData = {};        // All character data files

// Initialization
async function init() {
    console.log('🚀 Initializing Starting Gear Editor...');

    // Detect theme from main game
    const savedTheme = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
    console.log(`🎨 Theme: ${savedTheme}`);

    await loadData();
    setupEventListeners();

    console.log('✅ Starting Gear Editor initialized');
}

// Load all data
async function loadData() {
    try {
        // Load character data
        const charResponse = await fetch('/api/character-data');
        characterData = await charResponse.json();
        allClasses = characterData.starting_gear;

        console.log(`📦 Loaded ${allClasses.length} classes`);

        // Load items for autocomplete
        const itemsResponse = await fetch('/api/items');
        allItems = await itemsResponse.json();

        console.log(`📦 Loaded ${Object.keys(allItems).length} items`);

        // Populate item datalist
        populateItemDatalist();

        // Render class list
        renderClassList();

        // Initialize staging
        await initStaging();

    } catch (error) {
        console.error('❌ Failed to load data:', error);
        showStatus('Failed to load data: ' + error.message, 'error');
    }
}

// Populate item autocomplete datalist
function populateItemDatalist() {
    // Populate regular item datalist
    const datalist = document.getElementById('item-datalist');
    datalist.innerHTML = '';

    // Populate pack datalist (filtered for type: Pack)
    let packDatalist = document.getElementById('pack-datalist');
    if (!packDatalist) {
        packDatalist = document.createElement('datalist');
        packDatalist.id = 'pack-datalist';
        document.body.appendChild(packDatalist);
    }
    packDatalist.innerHTML = '';

    Object.keys(allItems).forEach(itemId => {
        const item = allItems[itemId];

        // Add to regular item datalist
        const option = document.createElement('option');
        option.value = itemId;
        option.textContent = item.name || itemId;
        datalist.appendChild(option);

        // Add to pack datalist if it's a Pack
        if (item.type === 'Pack') {
            const packOption = document.createElement('option');
            packOption.value = itemId;
            packOption.textContent = item.name || itemId;
            packDatalist.appendChild(packOption);
        }
    });
}

// Initialize staging system (shared session)
async function initStaging() {
    try {
        // Get staging mode
        const modeResponse = await fetch('/api/staging/mode');
        const modeData = await modeResponse.json();
        stagingMode = modeData.mode;

        console.log(`🔧 Staging mode: ${stagingMode}`);

        if (stagingMode === 'staging') {
            // Show toggle button
            const stagingToggle = document.getElementById('staging-toggle');
            if (stagingToggle) {
                stagingToggle.classList.remove('hidden');
            }

            // Try to reuse existing session from localStorage
            let existingSessionID = localStorage.getItem('codex_staging_session');

            if (existingSessionID) {
                // Verify session is still valid
                const verifyResponse = await fetch(`/api/staging/changes?session_id=${existingSessionID}`);
                if (verifyResponse.ok) {
                    stagingSessionID = existingSessionID;
                    console.log('✅ Reusing shared session:', stagingSessionID);

                    // Update change count
                    const changes = await verifyResponse.json();
                    updateChangeCount(changes.length);
                    return;
                }
            }

            // Create new session
            const initResponse = await fetch('/api/staging/init', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    npub: localStorage.getItem('codex_npub') || 'anonymous'
                })
            });

            const session = await initResponse.json();
            stagingSessionID = session.session_id;
            localStorage.setItem('codex_staging_session', stagingSessionID);

            console.log('✅ Created new session:', stagingSessionID);
        }
    } catch (error) {
        console.error('❌ Failed to initialize staging:', error);
    }
}

// Render class list in sidebar
function renderClassList() {
    const classList = document.getElementById('class-list');
    classList.innerHTML = '';

    allClasses.forEach((entry, index) => {
        const li = document.createElement('li');
        li.className = 'codex-list-item pixel-clip-sm';
        li.textContent = entry.class.toUpperCase();
        li.dataset.index = index;

        li.addEventListener('click', () => {
            selectClass(index);
        });

        classList.appendChild(li);
    });
}

// Select a class
function selectClass(index) {
    currentClassIndex = index;

    // Update UI
    document.querySelectorAll('.codex-list-item').forEach((item, i) => {
        if (i === index) {
            item.classList.add('selected');
        } else {
            item.classList.remove('selected');
        }
    });

    // Populate gear form
    populateGearForm();
}

// Populate starting gear form
function populateGearForm() {
    if (currentClassIndex === null) return;

    const entry = allClasses[currentClassIndex];
    const gearForm = document.getElementById('gear-form');

    let html = `<h2 class="codex-section-title mb-20">STARTING GEAR: ${entry.class.toUpperCase()}</h2>`;

    // Check for pack conflicts
    const hasPackInGivenItems = entry.starting_gear.given_items && entry.starting_gear.given_items.some(item => {
        const itemData = allItems[item.item];
        return itemData && itemData.type === 'Pack';
    });
    const hasPackChoice = !!entry.starting_gear.pack_choice;

    // Show warning if both exist
    if (hasPackInGivenItems && hasPackChoice) {
        html += '<div class="codex-section win95-inset pixel-clip mb-20" style="border: 2px solid var(--codex-red); background-color: rgba(255, 0, 0, 0.1);">';
        html += '<h3 style="color: var(--codex-red);">⚠️ VALIDATION ERROR</h3>';
        html += '<p style="color: var(--codex-text-primary);">Cannot have both PACK CHOICE and a pack in GIVEN ITEMS. Remove one or the other.</p>';
        html += '</div>';
    }

    // FIELD ORDER: given_items, equipment_choices, pack_choice

    // Given Items (moved to top)
    html += '<div class="codex-section win95-inset pixel-clip mb-20">';
    html += '<h3 class="codex-section-title">GIVEN ITEMS</h3>';
    html += '<p class="text-muted mb-10" style="font-size: 12px;">Items automatically given to this class at creation</p>';
    html += renderGivenItems(entry.starting_gear.given_items);
    html += '<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addGivenItem()">+ ADD ITEM</button>';
    html += '</div>';

    // Equipment Choices
    html += '<div class="codex-section win95-inset pixel-clip mb-20">';
    html += '<h3 class="codex-section-title">EQUIPMENT CHOICES</h3>';
    html += '<p class="text-muted mb-10" style="font-size: 12px;">Player chooses one option from each choice</p>';
    html += renderEquipmentChoices(entry.starting_gear.equipment_choices);
    html += '<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addEquipmentChoice()">+ ADD CHOICE</button>';
    html += '</div>';

    // Pack Choice
    html += '<div class="codex-section win95-inset pixel-clip mb-20">';
    html += '<h3 class="codex-section-title">PACK CHOICE</h3>';
    html += '<p class="text-muted mb-10" style="font-size: 12px;">Player chooses one pack (OR include a pack in Given Items above)</p>';
    if (entry.starting_gear.pack_choice) {
        html += renderPackChoice(entry.starting_gear.pack_choice);
    } else {
        html += '<p class="text-muted mb-10">No pack choice defined</p>';
        html += '<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addPackChoice()">+ ADD PACK CHOICE</button>';
    }
    html += '</div>';

    gearForm.innerHTML = html;
}

// Render equipment choices
function renderEquipmentChoices(choices) {
    let html = '';

    choices.forEach((choice, choiceIndex) => {
        html += `<div class="equipment-choice-card win95-inset pixel-clip mb-15" data-choice="${choiceIndex}">`;
        html += `<h4 class="text-highlighted mb-10">CHOICE ${choiceIndex + 1}</h4>`;

        choice.options.forEach((option, optionIndex) => {
            html += renderOption(choiceIndex, optionIndex, option);
        });

        html += '<div class="codex-btn-group">';
        html += `<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addOption(${choiceIndex})">+ ADD OPTION</button>`;
        html += `<button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removeChoice(${choiceIndex})">REMOVE CHOICE</button>`;
        html += '</div>';
        html += '</div>';
    });

    return html;
}

// Render a single option
function renderOption(choiceIndex, optionIndex, option) {
    let html = `<div class="option-card pixel-clip mb-10" data-option="${optionIndex}">`;
    html += `<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">`;
    html += `<span class="option-type-badge pixel-clip-sm">${option.type.toUpperCase().replace('_', ' ')}</span>`;
    html += `<div>`;
    html += `<button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="changeOptionType(${choiceIndex}, ${optionIndex})">CHANGE TYPE</button>`;
    html += `<button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removeOption(${choiceIndex}, ${optionIndex})" style="margin-left: 5px;">REMOVE</button>`;
    html += `</div>`;
    html += `</div>`;

    if (option.type === 'single') {
        html += renderSingleOption(choiceIndex, optionIndex, option);
    } else if (option.type === 'bundle') {
        html += renderBundleOption(choiceIndex, optionIndex, option);
    } else if (option.type === 'multi_slot') {
        html += renderMultiSlotOption(choiceIndex, optionIndex, option);
    }

    html += '</div>';
    return html;
}

// Helper: Get item image HTML
function getItemImageHTML(itemId) {
    if (!itemId) {
        return '<div class="item-image-preview pixel-clip-sm"><span style="font-size: 10px;">?</span></div>';
    }
    const imagePath = `/www/res/img/items/${itemId}.png`;
    return `<div class="item-image-preview pixel-clip-sm"><img src="${imagePath}" alt="${itemId}" onerror="this.style.display='none'; this.parentElement.innerHTML='<span style=\\'font-size: 10px;\\'>?</span>';" /></div>`;
}

// Make helper available globally
window.getItemImageHTML = getItemImageHTML;

// Render single option
function renderSingleOption(choiceIndex, optionIndex, option) {
    return `
        <div class="item-input-row">
            ${getItemImageHTML(option.item)}
            <input type="text" value="${option.item || ''}"
                   list="item-datalist"
                   placeholder="Item ID"
                   class="codex-input"
                   onchange="updateSingleOption(${choiceIndex}, ${optionIndex}, this.value, null); this.parentElement.querySelector('.item-image-preview').outerHTML = getItemImageHTML(this.value);" />
            <input type="number" value="${option.quantity || 1}"
                   min="1"
                   class="codex-input"
                   onchange="updateSingleOption(${choiceIndex}, ${optionIndex}, null, parseInt(this.value))" />
        </div>
    `;
}

// Render bundle option
function renderBundleOption(choiceIndex, optionIndex, option) {
    let html = '<div class="bundle-container">';

    (option.items || []).forEach((item, itemIndex) => {
        html += `
            <div class="bundle-item pixel-clip-sm">
                <div class="item-input-row">
                    ${getItemImageHTML(item.item)}
                    <input type="text" value="${item.item}"
                           list="item-datalist"
                           placeholder="Item ID"
                           class="codex-input"
                           onchange="updateBundleItem(${choiceIndex}, ${optionIndex}, ${itemIndex}, this.value, null); populateGearForm();" />
                    <input type="number" value="${item.quantity}"
                           min="1"
                           class="codex-input"
                           onchange="updateBundleItem(${choiceIndex}, ${optionIndex}, ${itemIndex}, null, parseInt(this.value))" />
                    <button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removeBundleItem(${choiceIndex}, ${optionIndex}, ${itemIndex})">×</button>
                </div>
            </div>
        `;
    });

    html += `<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addBundleItem(${choiceIndex}, ${optionIndex})">+ ADD ITEM TO BUNDLE</button>`;
    html += '</div>';
    return html;
}

// Render multi_slot option
function renderMultiSlotOption(choiceIndex, optionIndex, option) {
    let html = '<div class="multislot-container">';

    (option.slots || []).forEach((slot, slotIndex) => {
        html += `<div class="slot-item pixel-clip-sm">`;
        html += `<span class="option-type-badge pixel-clip-sm" style="font-size: 9px;">${slot.type === 'weapon_choice' ? 'WEAPON CHOICE' : 'FIXED ITEM'}</span>`;

        if (slot.type === 'weapon_choice') {
            html += '<div style="margin-top: 8px;">';
            (slot.options || []).forEach((weaponID, weaponIndex) => {
                html += `
                    <div class="item-input-row">
                        ${getItemImageHTML(weaponID)}
                        <input type="text" value="${weaponID}"
                               list="item-datalist"
                               placeholder="Weapon ID"
                               class="codex-input"
                               onchange="updateWeaponChoice(${choiceIndex}, ${optionIndex}, ${slotIndex}, ${weaponIndex}, this.value); populateGearForm();" />
                        <button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removeWeaponChoice(${choiceIndex}, ${optionIndex}, ${slotIndex}, ${weaponIndex})">×</button>
                    </div>
                `;
            });
            html += `<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addWeaponChoice(${choiceIndex}, ${optionIndex}, ${slotIndex})">+ ADD WEAPON OPTION</button>`;
            html += '</div>';
        } else if (slot.type === 'fixed') {
            html += `
                <div class="item-input-row" style="margin-top: 8px;">
                    ${getItemImageHTML(slot.item)}
                    <input type="text" value="${slot.item || ''}"
                           list="item-datalist"
                           placeholder="Item ID"
                           class="codex-input"
                           onchange="updateFixedSlot(${choiceIndex}, ${optionIndex}, ${slotIndex}, this.value, null); populateGearForm();" />
                    <input type="number" value="${slot.quantity || 1}"
                           min="1"
                           class="codex-input"
                           onchange="updateFixedSlot(${choiceIndex}, ${optionIndex}, ${slotIndex}, null, parseInt(this.value))" />
                </div>
            `;
        }

        html += `<button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removeSlot(${choiceIndex}, ${optionIndex}, ${slotIndex})" style="margin-top: 8px;">REMOVE SLOT</button>`;
        html += '</div>';
    });

    html += '<div class="codex-btn-group" style="margin-top: 10px;">';
    html += `<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addWeaponChoiceSlot(${choiceIndex}, ${optionIndex})">+ ADD WEAPON CHOICE SLOT</button>`;
    html += `<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addFixedSlot(${choiceIndex}, ${optionIndex})">+ ADD FIXED ITEM SLOT</button>`;
    html += '</div>';
    html += '</div>';
    return html;
}

// Render pack choice
function renderPackChoice(packChoice) {
    let html = `<label class="codex-label">Description:</label>`;
    html += `<input type="text" value="${packChoice.description}" class="codex-input mb-15" onchange="updatePackDescription(this.value)" />`;
    html += `<h4 class="text-highlighted mb-10">PACK OPTIONS:</h4>`;

    packChoice.options.forEach((packID, index) => {
        // Check if this is actually a pack
        const itemData = allItems[packID];
        const isPack = itemData && itemData.type === 'Pack';
        const warningStyle = (!isPack && packID) ? 'border: 2px solid var(--codex-red);' : '';

        html += `
            <div class="item-input-row">
                ${getItemImageHTML(packID)}
                <input type="text" value="${packID}"
                       list="pack-datalist"
                       placeholder="Pack ID (type: Pack items only)"
                       class="codex-input"
                       style="${warningStyle}"
                       onchange="updatePackOption(${index}, this.value); populateGearForm();" />
                <button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removePackOption(${index})">×</button>
                ${(!isPack && packID) ? '<span style="color: var(--codex-red); font-size: 11px; margin-left: 8px;">⚠️ Not a pack!</span>' : ''}
            </div>
        `;
    });

    html += `<button class="codex-btn codex-btn-sm codex-btn-add pixel-clip-sm" onclick="addPackOption()">+ ADD PACK</button>`;
    return html;
}

// Render given items
function renderGivenItems(items) {
    let html = '';

    items.forEach((item, index) => {
        html += `
            <div class="item-input-row">
                ${getItemImageHTML(item.item)}
                <input type="text" value="${item.item}"
                       list="item-datalist"
                       placeholder="Item ID"
                       class="codex-input"
                       onchange="updateGivenItem(${index}, this.value, null); populateGearForm();" />
                <input type="number" value="${item.quantity}"
                       min="1"
                       class="codex-input"
                       onchange="updateGivenItem(${index}, null, parseInt(this.value))" />
                <button class="codex-btn codex-btn-sm pixel-clip-sm" onclick="removeGivenItem(${index})">×</button>
            </div>
        `;
    });

    return html;
}

// Setup event listeners
function setupEventListeners() {
    // Starting Gear
    document.getElementById('save-gear-btn').addEventListener('click', saveStartingGear);
    document.getElementById('validate-gear-btn').addEventListener('click', validateGear);

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
        // Show panel - ensure display is block first, then add visible class for animation
        panel.style.display = 'block';
        // Small delay to allow display change to take effect before animation
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

// Save starting gear
async function saveStartingGear() {
    if (currentClassIndex === null) {
        showStatus('Please select a class first', 'error');
        return;
    }

    try {
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch('/api/character-data/starting-gear', {
            method: 'PUT',
            headers: headers,
            body: JSON.stringify(allClasses)
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        const result = await response.json();

        if (result.mode === 'staging') {
            showStatus(`✅ Changes staged (${result.changes} total)`, 'success');
            updateChangeCount(result.changes);
        } else {
            showStatus('✅ Saved successfully', 'success');
        }
    } catch (error) {
        console.error('❌ Save failed:', error);
        showStatus('Save failed: ' + error.message, 'error');
    }
}

// Validate gear
function validateGear() {
    if (currentClassIndex === null) {
        showStatus('Please select a class first', 'error');
        return;
    }

    const entry = allClasses[currentClassIndex];
    const issues = [];

    // Validate all item IDs
    entry.starting_gear.equipment_choices.forEach((choice, choiceIndex) => {
        choice.options.forEach((option, optionIndex) => {
            validateOptionItems(option, `Choice ${choiceIndex + 1}, Option ${optionIndex + 1}`, issues);
        });
    });

    // Validate given items
    entry.starting_gear.given_items.forEach((item, index) => {
        if (!allItems[item.item]) {
            issues.push(`Given item ${index + 1}: Unknown item ID '${item.item}'`);
        }
    });

    // Validate pack choices
    if (entry.starting_gear.pack_choice) {
        entry.starting_gear.pack_choice.options.forEach((packID, index) => {
            if (!allItems[packID]) {
                issues.push(`Pack option ${index + 1}: Unknown pack ID '${packID}'`);
            }
        });
    }

    if (issues.length > 0) {
        showStatus('❌ Validation failed:\n' + issues.join('\n'), 'error');
    } else {
        showStatus('✅ Validation passed!', 'success');
    }
}

// Validate option items
function validateOptionItems(option, context, issues) {
    if (option.type === 'single') {
        if (option.item && !allItems[option.item]) {
            issues.push(`${context}: Unknown item ID '${option.item}'`);
        }
    } else if (option.type === 'bundle') {
        (option.items || []).forEach((item, i) => {
            if (!allItems[item.item]) {
                issues.push(`${context}, Bundle item ${i + 1}: Unknown item ID '${item.item}'`);
            }
        });
    } else if (option.type === 'multi_slot') {
        (option.slots || []).forEach((slot, i) => {
            if (slot.type === 'weapon_choice') {
                (slot.options || []).forEach((weaponID, j) => {
                    if (!allItems[weaponID]) {
                        issues.push(`${context}, Slot ${i + 1}, Weapon ${j + 1}: Unknown weapon ID '${weaponID}'`);
                    }
                });
            } else if (slot.type === 'fixed') {
                if (slot.item && !allItems[slot.item]) {
                    issues.push(`${context}, Slot ${i + 1}: Unknown item ID '${slot.item}'`);
                }
            }
        });
    }
}

// Staging panel functions
async function viewChanges() {
    if (!stagingSessionID) return;

    try {
        const response = await fetch(`/api/staging/changes?session_id=${stagingSessionID}`);
        const changes = await response.json();

        console.log('📋 Staging changes:', changes);
        alert(`You have ${changes.length} staged changes. Check console for details.`);
    } catch (error) {
        console.error('❌ Failed to fetch changes:', error);
    }
}

async function submitPR() {
    if (!stagingSessionID) return;

    const title = prompt('Enter PR title:');
    if (!title) return;

    const body = prompt('Enter PR description (optional):');

    try {
        const response = await fetch('/api/staging/submit', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: stagingSessionID,
                title: title,
                body: body || ''
            })
        });

        const result = await response.json();

        if (result.pr_url) {
            showStatus(`✅ PR created: ${result.pr_url}`, 'success');
            localStorage.removeItem('codex_staging_session');
            stagingSessionID = null;
            updateChangeCount(0);
        } else {
            throw new Error(result.error || 'Failed to create PR');
        }
    } catch (error) {
        console.error('❌ Failed to submit PR:', error);
        showStatus('Failed to submit PR: ' + error.message, 'error');
    }
}

async function clearStaging() {
    if (!stagingSessionID) return;

    if (!confirm('Clear all staged changes?')) return;

    try {
        await fetch(`/api/staging/clear?session_id=${stagingSessionID}`, {
            method: 'DELETE'
        });

        localStorage.removeItem('codex_staging_session');
        stagingSessionID = null;
        updateChangeCount(0);
        showStatus('✅ Staging cleared', 'success');
    } catch (error) {
        console.error('❌ Failed to clear staging:', error);
    }
}

function updateChangeCount(count) {
    document.getElementById('change-count').textContent = `${count} change${count !== 1 ? 's' : ''}`;

    // Update badge on toggle button
    const badge = document.getElementById('staging-badge');
    if (badge) {
        badge.textContent = count;
        badge.style.display = count > 0 ? 'flex' : 'none';
    }

    // Show/hide toggle button
    const toggle = document.getElementById('staging-toggle');
    if (toggle && stagingMode === 'staging') {
        toggle.classList.remove('hidden');
    }
}

// Show status message
function showStatus(message, type) {
    const container = document.getElementById('status-container');
    const div = document.createElement('div');
    div.className = `status-message ${type} pixel-clip`;
    div.textContent = message;

    container.appendChild(div);

    setTimeout(() => {
        div.remove();
    }, 5000);
}

// Update functions (called from HTML onclick handlers)
window.addEquipmentChoice = function() {
    allClasses[currentClassIndex].starting_gear.equipment_choices.push({
        options: []
    });
    populateGearForm();
};

window.removeChoice = function(choiceIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices.splice(choiceIndex, 1);
    populateGearForm();
};

window.addOption = function(choiceIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options.push({
        type: 'single',
        item: '',
        quantity: 1
    });
    populateGearForm();
};

window.removeOption = function(choiceIndex, optionIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options.splice(optionIndex, 1);
    populateGearForm();
};

window.changeOptionType = function(choiceIndex, optionIndex) {
    const option = allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex];
    const types = ['single', 'bundle', 'multi_slot'];
    const currentIndex = types.indexOf(option.type);
    const nextType = types[(currentIndex + 1) % types.length];

    // Reset option to new type
    if (nextType === 'single') {
        allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex] = {
            type: 'single',
            item: '',
            quantity: 1
        };
    } else if (nextType === 'bundle') {
        allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex] = {
            type: 'bundle',
            items: []
        };
    } else if (nextType === 'multi_slot') {
        allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex] = {
            type: 'multi_slot',
            slots: []
        };
    }

    populateGearForm();
};

window.updateSingleOption = function(choiceIndex, optionIndex, item, quantity) {
    const option = allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex];
    if (item !== null) option.item = item;
    if (quantity !== null) option.quantity = quantity;
};

window.addBundleItem = function(choiceIndex, optionIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].items.push({
        item: '',
        quantity: 1
    });
    populateGearForm();
};

window.removeBundleItem = function(choiceIndex, optionIndex, itemIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].items.splice(itemIndex, 1);
    populateGearForm();
};

window.updateBundleItem = function(choiceIndex, optionIndex, itemIndex, item, quantity) {
    const bundleItem = allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].items[itemIndex];
    if (item !== null) bundleItem.item = item;
    if (quantity !== null) bundleItem.quantity = quantity;
};

window.addWeaponChoiceSlot = function(choiceIndex, optionIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots.push({
        type: 'weapon_choice',
        options: []
    });
    populateGearForm();
};

window.addFixedSlot = function(choiceIndex, optionIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots.push({
        type: 'fixed',
        item: '',
        quantity: 1
    });
    populateGearForm();
};

window.removeSlot = function(choiceIndex, optionIndex, slotIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots.splice(slotIndex, 1);
    populateGearForm();
};

window.addWeaponChoice = function(choiceIndex, optionIndex, slotIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots[slotIndex].options.push('');
    populateGearForm();
};

window.removeWeaponChoice = function(choiceIndex, optionIndex, slotIndex, weaponIndex) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots[slotIndex].options.splice(weaponIndex, 1);
    populateGearForm();
};

window.updateWeaponChoice = function(choiceIndex, optionIndex, slotIndex, weaponIndex, value) {
    allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots[slotIndex].options[weaponIndex] = value;
};

window.updateFixedSlot = function(choiceIndex, optionIndex, slotIndex, item, quantity) {
    const slot = allClasses[currentClassIndex].starting_gear.equipment_choices[choiceIndex].options[optionIndex].slots[slotIndex];
    if (item !== null) slot.item = item;
    if (quantity !== null) slot.quantity = quantity;
};

window.updatePackDescription = function(value) {
    allClasses[currentClassIndex].starting_gear.pack_choice.description = value;
};

window.addPackChoice = function() {
    allClasses[currentClassIndex].starting_gear.pack_choice = {
        description: '',
        options: []
    };
    populateGearForm();
};

window.addPackOption = function() {
    allClasses[currentClassIndex].starting_gear.pack_choice.options.push('');
    populateGearForm();
};

window.removePackOption = function(index) {
    allClasses[currentClassIndex].starting_gear.pack_choice.options.splice(index, 1);
    populateGearForm();
};

window.updatePackOption = function(index, value) {
    allClasses[currentClassIndex].starting_gear.pack_choice.options[index] = value;
};

window.addGivenItem = function() {
    allClasses[currentClassIndex].starting_gear.given_items.push({
        item: '',
        quantity: 1
    });
    populateGearForm();
};

window.removeGivenItem = function(index) {
    allClasses[currentClassIndex].starting_gear.given_items.splice(index, 1);
    populateGearForm();
};

window.updateGivenItem = function(index, item, quantity) {
    const givenItem = allClasses[currentClassIndex].starting_gear.given_items[index];
    if (item !== null) givenItem.item = item;
    if (quantity !== null) givenItem.quantity = quantity;
};

// Initialize on load
init();
