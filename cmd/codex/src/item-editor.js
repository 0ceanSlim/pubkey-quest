/**
 * CODEX Item Editor Entry Point
 */

// Import styles
import './styles.css';

// Detect and apply theme from main game
const savedTheme = localStorage.getItem('theme') || 'dark';
document.documentElement.setAttribute('data-theme', savedTheme);

// ===== STATE =====
let allItems = {};
let currentItem = null;
let isNewItem = false;
let currentTags = [];
let currentNotes = [];
let currentPackContents = [];
let allItemTypes = new Set();
let spellComponents = [];
let allItemIds = [];
let currentEffects = [];
let currentWornEffects = []; // from effects_when_worn (equipment passive effects)
let effectTypesData = {};  // from /api/effect-types
let namedEffectsData = {}; // from /api/effects

// ===== STAGING STATE =====
let stagingSessionID = null;
let stagingMode = null;

// ===== INITIALIZATION =====
async function loadItems() {
    try {
        const response = await fetch('/api/items');
        allItems = await response.json();

        // Extract all unique item types and spell components
        spellComponents = [];
        allItemIds = [];
        Object.entries(allItems).forEach(([id, item]) => {
            if (item.type) {
                allItemTypes.add(item.type);
            }
            // Collect spell components (items with spell_component tag)
            if (item.tags && item.tags.includes('spell_component')) {
                spellComponents.push({ id: id, name: item.name });
            }
            allItemIds.push({ id: id, name: item.name });
        });

        // Sort for better UX
        spellComponents.sort((a, b) => a.name.localeCompare(b.name));
        allItemIds.sort((a, b) => a.name.localeCompare(b.name));

        populateTypeFilter();
        populateTypeDropdown();
        populateSpellComponentsDropdown();
        populateAllowedTypesDropdown();
        populatePackItemsDropdown();
        renderItemList();
        updateItemCount();
    } catch (error) {
        console.error('Error loading items:', error);
        showStatus('Failed to load items', 'error');
    }
}

async function loadEffectsData() {
    try {
        const [typesRes, effectsRes] = await Promise.all([
            fetch('/api/effect-types'),
            fetch('/api/effects')
        ]);
        const typesJson = await typesRes.json();
        effectTypesData = typesJson.effect_types || {};
        namedEffectsData = await effectsRes.json();

        populateEffectTypeDropdown();
        populateNamedEffectDropdown();
        populateWornEffectsDropdown();
    } catch (error) {
        console.error('Error loading effects data:', error);
    }
}

function populateEffectTypeDropdown() {
    const select = document.getElementById('newEffectType');
    if (!select) return;
    select.innerHTML = '<option value="">Inline effect type...</option>';

    Object.keys(effectTypesData).sort().forEach(key => {
        const option = document.createElement('option');
        option.value = key;
        option.textContent = `${key} (${effectTypesData[key].description})`;
        select.appendChild(option);
    });
}

function populateNamedEffectDropdown() {
    const select = document.getElementById('newNamedEffect');
    if (!select) return;
    select.innerHTML = '<option value="">Named effect...</option>';

    Object.keys(namedEffectsData).sort().forEach(key => {
        const effect = namedEffectsData[key];
        const option = document.createElement('option');
        option.value = key;
        option.textContent = effect.name || key;
        select.appendChild(option);
    });
}

function populateWornEffectsDropdown() {
    const select = document.getElementById('newWornEffect');
    if (!select) return;
    select.innerHTML = '<option value="">Select effect when worn...</option>';

    Object.keys(namedEffectsData).sort().forEach(key => {
        const effect = namedEffectsData[key];
        const option = document.createElement('option');
        option.value = key;
        option.textContent = effect.name || key;
        select.appendChild(option);
    });
}

function populateTypeFilter() {
    const typeFilter = document.getElementById('typeFilter');
    typeFilter.innerHTML = '<option value="">All Types</option>';

    Array.from(allItemTypes).sort().forEach(type => {
        const option = document.createElement('option');
        option.value = type;
        option.textContent = type;
        typeFilter.appendChild(option);
    });
}

function populateTypeDropdown() {
    const typeSelect = document.getElementById('itemType');
    typeSelect.innerHTML = '<option value="">Select type...</option>';

    Array.from(allItemTypes).sort().forEach(type => {
        const option = document.createElement('option');
        option.value = type;
        option.textContent = type;
        typeSelect.appendChild(option);
    });
}

function populateSpellComponentsDropdown() {
    const providesSelect = document.getElementById('itemProvides');
    providesSelect.innerHTML = '<option value="">Select spell component...</option>';

    spellComponents.forEach(comp => {
        const option = document.createElement('option');
        option.value = comp.id;
        option.textContent = comp.name;
        providesSelect.appendChild(option);
    });
}

function populateAllowedTypesDropdown() {
    const allowedTypesSelect = document.getElementById('allowedTypes');
    allowedTypesSelect.innerHTML = '<option value="any">any (all items)</option>';

    // Add item types
    const optgroupTypes = document.createElement('optgroup');
    optgroupTypes.label = 'Item Types';
    Array.from(allItemTypes).sort().forEach(type => {
        const option = document.createElement('option');
        option.value = type;
        option.textContent = type;
        optgroupTypes.appendChild(option);
    });
    allowedTypesSelect.appendChild(optgroupTypes);

    // Add all item IDs
    const optgroupItems = document.createElement('optgroup');
    optgroupItems.label = 'Specific Items';
    allItemIds.forEach(item => {
        const option = document.createElement('option');
        option.value = item.id;
        option.textContent = `${item.name} (${item.id})`;
        optgroupItems.appendChild(option);
    });
    allowedTypesSelect.appendChild(optgroupItems);
}

function populatePackItemsDropdown() {
    const packItemSelect = document.getElementById('newPackItemSelect');
    packItemSelect.innerHTML = '<option value="">Select item to add...</option>';

    allItemIds.forEach(item => {
        const option = document.createElement('option');
        option.value = item.id;
        option.textContent = `${item.name} (${item.id})`;
        packItemSelect.appendChild(option);
    });
}

function updateItemCount() {
    const visibleItems = document.querySelectorAll('.item-row').length;
    const totalItems = Object.keys(allItems).length;
    document.getElementById('itemCount').textContent = `${visibleItems}/${totalItems}`;
}

// ===== FILTERING =====
function applyFilters() {
    const searchTerm = document.getElementById('searchBox').value.toLowerCase();
    const typeFilter = document.getElementById('typeFilter').value;

    renderItemList(searchTerm, typeFilter);
    updateItemCount();
}

function renderItemList(searchFilter = '', typeFilter = '') {
    const container = document.getElementById('itemListContainer');
    container.innerHTML = '';

    const items = Object.entries(allItems).filter(([filename, item]) => {
        // Search filter
        if (searchFilter) {
            const matches = item.name.toLowerCase().includes(searchFilter) ||
                           item.type.toLowerCase().includes(searchFilter) ||
                           filename.toLowerCase().includes(searchFilter);
            if (!matches) return false;
        }

        // Type filter
        if (typeFilter && item.type !== typeFilter) {
            return false;
        }

        return true;
    });

    items.forEach(([filename, item]) => {
        const row = document.createElement('div');
        row.className = 'item-row';
        if (currentItem === filename) {
            row.classList.add('active');
        }
        row.onclick = () => selectItem(filename);
        row.innerHTML = `
            <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-weight: bold;">${item.name}</span>
                <span style="color: #6272a4; font-size: 12px;">(${item.type})</span>
            </div>
        `;
        container.appendChild(row);
    });

    updateItemCount();
}

// ===== ITEM SELECTION =====
function selectItem(filename) {
    currentItem = filename;
    isNewItem = false;
    const item = allItems[filename];

    showEditor();
    populateForm(item);
    renderItemList(document.getElementById('searchBox').value, document.getElementById('typeFilter').value);
}

function populateForm(item) {
    document.getElementById('editorMode').textContent = 'Edit';
    document.getElementById('itemName').textContent = item.name;
    document.getElementById('deleteBtn').style.display = 'block';

    // Basic info
    document.getElementById('itemId').value = item.id || '';
    document.getElementById('itemId').disabled = true;
    document.getElementById('itemNameInput').value = item.name || '';
    document.getElementById('itemType').value = item.type || '';
    document.getElementById('itemDescription').value = item.description || '';
    document.getElementById('itemRarity').value = item.rarity || 'common';
    document.getElementById('itemPrice').value = item.price || 0;
    document.getElementById('itemWeight').value = item.weight || 1;
    document.getElementById('itemStack').value = item.stack || 1;

    // Tags
    currentTags = item.tags || [];
    renderTags();

    // Equipment
    document.getElementById('gearSlot').value = item.gear_slot || '';

    // Container
    document.getElementById('containerSlots').value = item.container_slots || '';
    const allowedTypesSelect = document.getElementById('allowedTypes');
    Array.from(allowedTypesSelect.options).forEach(opt => opt.selected = false);
    if (item.allowed_types) {
        if (item.allowed_types === 'any') {
            allowedTypesSelect.options[0].selected = true;
        } else if (Array.isArray(item.allowed_types)) {
            item.allowed_types.forEach(type => {
                Array.from(allowedTypesSelect.options).forEach(opt => {
                    if (opt.value === type) {
                        opt.selected = true;
                    }
                });
            });
        }
    }

    // Combat
    document.getElementById('itemAC').value = item.ac || '';
    document.getElementById('itemDamage').value = item.damage || '';
    document.getElementById('damageType').value = item['damage-type'] || item.damage_type || '';

    // Ranged
    document.getElementById('ammunition').value = item.ammunition || '';
    document.getElementById('range').value = item.range || '';
    document.getElementById('rangeLong').value = item['range-long'] || item.range_long || '';

    // Consumable effects
    currentEffects = Array.isArray(item.effects) ? JSON.parse(JSON.stringify(item.effects)) : [];
    // Migrate legacy heal field into effects
    if (item.heal && !currentEffects.some(e => e.type === 'hp')) {
        const healStr = String(item.heal);
        if (healStr.includes('d')) {
            currentEffects.unshift({ type: 'hp', dice: healStr, chance: 100 });
        } else {
            currentEffects.unshift({ type: 'hp', value: parseInt(healStr) || 0, chance: 100 });
        }
    }
    renderEffects();

    // Worn effects (equipment passive)
    currentWornEffects = Array.isArray(item.effects_when_worn) ? [...item.effects_when_worn] : [];
    renderWornEffects();

    // Focus
    document.getElementById('itemProvides').value = item.provides || '';

    // Pack contents
    currentPackContents = item.contents || [];
    renderPackContents();

    // Notes
    currentNotes = item.notes || [];
    renderNotes();

    // Image
    document.getElementById('itemImage').value = item.image || `/res/img/items/${item.id}.png`;
    checkImage();

    // Update conditional sections
    updateConditionalSections();
}

function showEditor() {
    document.getElementById('emptyState').style.display = 'none';
    document.getElementById('editorForm').style.display = 'block';
}

function hideEditor() {
    document.getElementById('emptyState').style.display = 'flex';
    document.getElementById('editorForm').style.display = 'none';
}

// ===== NEW ITEM =====
function createNewItem() {
    isNewItem = true;
    currentItem = null;
    currentTags = [];
    currentNotes = [];
    currentPackContents = [];

    showEditor();

    document.getElementById('editorMode').textContent = 'Create New Item';
    document.getElementById('itemName').textContent = 'New Item';
    document.getElementById('deleteBtn').style.display = 'none';

    // Clear form
    document.getElementById('itemId').value = '';
    document.getElementById('itemId').disabled = false;
    document.getElementById('itemNameInput').value = '';
    document.getElementById('itemType').value = '';
    document.getElementById('itemDescription').value = '';
    document.getElementById('itemRarity').value = 'common';
    document.getElementById('itemPrice').value = 0;
    document.getElementById('itemWeight').value = 1;
    document.getElementById('itemStack').value = 1;
    document.getElementById('gearSlot').value = '';
    document.getElementById('containerSlots').value = '';
    const allowedTypesSelect = document.getElementById('allowedTypes');
    Array.from(allowedTypesSelect.options).forEach(opt => opt.selected = false);
    document.getElementById('itemAC').value = '';
    document.getElementById('itemDamage').value = '';
    document.getElementById('damageType').value = '';
    document.getElementById('ammunition').value = '';
    document.getElementById('range').value = '';
    document.getElementById('rangeLong').value = '';
    currentEffects = [];
    currentWornEffects = [];
    document.getElementById('itemProvides').value = '';
    document.getElementById('itemImage').value = '';

    renderTags();
    renderNotes();
    renderPackContents();
    renderEffects();
    renderWornEffects();
    updateConditionalSections();

    // Clear selection
    document.querySelectorAll('.item-row').forEach(row => {
        row.classList.remove('active');
    });
}

// ===== CONDITIONAL SECTIONS =====
function updateConditionalSections() {
    const hasEquipment = currentTags.includes('equipment');
    const hasContainer = currentTags.includes('container');
    const hasConsumable = currentTags.includes('consumable');
    const hasFocus = currentTags.includes('focus');
    const hasPack = currentTags.includes('pack');
    const hasThrown = currentTags.includes('thrown');
    const hasRanged = currentTags.includes('ranged');

    const itemType = document.getElementById('itemType').value.toLowerCase();

    // Check for specific item types
    const isWeapon = itemType.includes('weapon') ||
                    itemType.includes('melee') ||
                    itemType.includes('martial') ||
                    itemType.includes('simple') ||
                    hasThrown;
    const isArmor = itemType.includes('armor');
    const isRangedWeapon = itemType.includes('ranged') || hasThrown || hasRanged;

    // Show/hide sections
    document.getElementById('equipmentSection').style.display = hasEquipment ? 'block' : 'none';
    document.getElementById('containerSection').style.display = hasContainer ? 'block' : 'none';

    // Visual indicator for required gear_slot when equipment tag is present
    const gearSlotSelect = document.getElementById('gearSlot');
    if (hasEquipment) {
        gearSlotSelect.setAttribute('required', 'required');
        gearSlotSelect.classList.add('border-yellow-500');
    } else {
        gearSlotSelect.removeAttribute('required');
        gearSlotSelect.classList.remove('border-yellow-500');
    }
    document.getElementById('consumableSection').style.display = hasConsumable ? 'block' : 'none';
    document.getElementById('focusSection').style.display = hasFocus ? 'block' : 'none';
    document.getElementById('packSection').style.display = hasPack ? 'block' : 'none';
    document.getElementById('weaponSection').style.display = isWeapon ? 'block' : 'none';
    document.getElementById('armorSection').style.display = isArmor ? 'block' : 'none';
    document.getElementById('rangedSection').style.display = isRangedWeapon ? 'block' : 'none';
}

// ===== TAGS MANAGEMENT =====
function renderTags() {
    const container = document.getElementById('tagsContainer');
    container.innerHTML = '';

    currentTags.forEach(tag => {
        const tagElement = document.createElement('div');
        tagElement.className = 'tag';
        tagElement.innerHTML = `
            ${tag}
            <button type="button" class="tag-remove" onclick="window.removeTag('${tag}')">×</button>
        `;
        container.appendChild(tagElement);
    });

    updateConditionalSections();
}

function addTag() {
    const input = document.getElementById('newTagInput');
    const tag = input.value.trim();

    if (tag && !currentTags.includes(tag)) {
        currentTags.push(tag);
        renderTags();
        input.value = '';
    }
}

function removeTag(tag) {
    currentTags = currentTags.filter(t => t !== tag);
    renderTags();
}

// ===== NOTES MANAGEMENT =====
function renderNotes() {
    const container = document.getElementById('notesContainer');
    container.innerHTML = '';

    currentNotes.forEach((note, index) => {
        const noteElement = document.createElement('div');
        noteElement.className = 'tag';
        noteElement.innerHTML = `
            ${note}
            <button type="button" class="tag-remove" onclick="window.removeNote(${index})">×</button>
        `;
        container.appendChild(noteElement);
    });
}

function addNote() {
    const input = document.getElementById('newNoteInput');
    const note = input.value.trim();

    if (note) {
        currentNotes.push(note);
        renderNotes();
        input.value = '';
    }
}

function removeNote(index) {
    currentNotes.splice(index, 1);
    renderNotes();
}

// ===== PACK CONTENTS MANAGEMENT =====
function renderPackContents() {
    const container = document.getElementById('packContentsContainer');
    container.innerHTML = '';

    currentPackContents.forEach((packItem, index) => {
        const itemId = Array.isArray(packItem) ? packItem[0] : packItem.item;
        const quantity = Array.isArray(packItem) ? packItem[1] : packItem.quantity;
        const itemData = allItems[itemId];

        const packItemElement = document.createElement('div');
        packItemElement.className = 'tag';

        const itemName = itemData ? itemData.name : '⚠️ Unknown Item';
        packItemElement.innerHTML = `
            ${itemName} (x${quantity})
            <button type="button" class="tag-remove" onclick="window.removePackItem(${index})">×</button>
        `;

        container.appendChild(packItemElement);
    });
}

function addPackItem() {
    const itemSelect = document.getElementById('newPackItemSelect');
    const quantityInput = document.getElementById('newPackItemQuantity');

    const itemId = itemSelect.value;
    const quantity = parseInt(quantityInput.value) || 1;

    if (!itemId) {
        showStatus('Please select an item to add', 'error');
        return;
    }

    if (quantity < 1) {
        showStatus('Quantity must be at least 1', 'error');
        return;
    }

    // Use object format {item, quantity}
    currentPackContents.push({ item: itemId, quantity: quantity });

    renderPackContents();
    itemSelect.value = '';
    quantityInput.value = 1;
}

function removePackItem(index) {
    currentPackContents.splice(index, 1);
    renderPackContents();
}

// ===== EFFECTS MANAGEMENT =====
function renderEffects() {
    const container = document.getElementById('effectsContainer');
    if (!container) return;
    container.innerHTML = '';

    if (currentEffects.length === 0) {
        container.innerHTML = '<span style="color: #6272a4; font-size: 12px;">No effects defined</span>';
        return;
    }

    currentEffects.forEach((effect, index) => {
        const chip = document.createElement('div');
        chip.className = 'tag';

        let label;
        if (effect.apply_effect) {
            // Named status effect
            const effectData = namedEffectsData[effect.apply_effect];
            const name = effectData ? effectData.name : effect.apply_effect;
            const chanceStr = (effect.chance || 100) < 100 ? ` ${effect.chance}%` : '';
            // Build short summary of what it does
            let summary = '';
            if (effectData && effectData.effects) {
                summary = effectData.effects.map(e => {
                    const sign = e.value >= 0 ? '+' : '';
                    return `${e.type} ${sign}${e.value}`;
                }).join(', ');
                const dur = effectData.effects[0]?.duration;
                if (dur) summary += ` (${formatDuration(dur)})`;
            }
            label = `<b>${name}</b>${chanceStr}`;
            if (summary) label += `<br><span style="font-size: 11px; color: #6272a4;">${summary}</span>`;
        } else {
            // Inline stat change
            const chanceStr = (effect.chance || 100) < 100 ? ` (${effect.chance}%)` : '';
            if (effect.dice) {
                label = `<b>${effect.type}</b> ${effect.dice}${chanceStr}`;
            } else {
                const v = effect.value;
                const display = typeof v === 'number' ? (v >= 0 ? `+${v}` : `${v}`) : v;
                label = `<b>${effect.type}</b> ${display}${chanceStr}`;
            }
        }

        chip.innerHTML = `
            <span>${label}</span>
            <button type="button" class="tag-remove" onclick="window.removeEffect(${index})">×</button>
        `;
        container.appendChild(chip);
    });
}

function formatDuration(ticks) {
    if (!ticks || ticks === 0) return 'permanent';
    if (ticks < 60) return `${ticks} ticks`;
    const mins = Math.round(ticks);
    if (mins < 60) return `${mins}m`;
    const hours = Math.floor(mins / 60);
    const rem = mins % 60;
    return rem > 0 ? `${hours}h ${rem}m` : `${hours}h`;
}

function setupEffectModeToggle() {
    const modeSelect = document.getElementById('newEffectMode');
    if (!modeSelect) return;
    modeSelect.addEventListener('change', () => {
        const flatInput = document.getElementById('newEffectValueFlat');
        const diceInput = document.getElementById('newEffectValueDice');
        if (modeSelect.value === 'dice') {
            flatInput.style.display = 'none';
            diceInput.style.display = '';
        } else {
            flatInput.style.display = '';
            diceInput.style.display = 'none';
        }
    });
}

function addInlineEffect() {
    const typeSelect = document.getElementById('newEffectType');
    const mode = document.getElementById('newEffectMode').value;
    const chanceInput = document.getElementById('newEffectChanceInline');

    const type = typeSelect.value;
    if (!type) {
        showStatus('Please select a stat to modify', 'error');
        return;
    }

    const chance = parseInt(chanceInput.value) || 100;
    if (chance < 1 || chance > 100) {
        showStatus('Chance must be between 1 and 100', 'error');
        return;
    }

    const effect = { type, chance };

    if (mode === 'dice') {
        const diceVal = document.getElementById('newEffectValueDice').value.trim();
        if (!diceVal) {
            showStatus('Please enter a dice expression (e.g. 2d4+2)', 'error');
            return;
        }
        effect.dice = diceVal;
    } else {
        const flatVal = parseInt(document.getElementById('newEffectValueFlat').value);
        if (isNaN(flatVal) || flatVal === 0) {
            showStatus('Please enter a non-zero value', 'error');
            return;
        }
        effect.value = flatVal;
    }

    currentEffects.push(effect);
    renderEffects();
    typeSelect.value = '';
    document.getElementById('newEffectValueFlat').value = '';
    document.getElementById('newEffectValueDice').value = '';
    chanceInput.value = 100;
}

function addNamedEffect() {
    const effectSelect = document.getElementById('newNamedEffect');
    const chanceInput = document.getElementById('newEffectChance');

    const effectId = effectSelect.value;
    const chance = parseInt(chanceInput.value) || 100;

    if (!effectId) {
        showStatus('Please select a status effect', 'error');
        return;
    }
    if (chance < 1 || chance > 100) {
        showStatus('Chance must be between 1 and 100', 'error');
        return;
    }

    currentEffects.push({ apply_effect: effectId, chance });
    renderEffects();
    effectSelect.value = '';
    chanceInput.value = 100;
    // Clear preview
    const preview = document.getElementById('namedEffectPreview');
    if (preview) preview.style.display = 'none';
}

function previewNamedEffect() {
    const effectId = document.getElementById('newNamedEffect').value;
    const preview = document.getElementById('namedEffectPreview');
    if (!preview) return;

    if (!effectId || !namedEffectsData[effectId]) {
        preview.style.display = 'none';
        return;
    }

    const effect = namedEffectsData[effectId];
    let html = `<div style="color: #f8f8f2; margin-bottom: 4px;"><b>${effect.name}</b>`;
    if (effect.icon) html += ` <span style="color: #6272a4;">(${effect.icon})</span>`;
    if (effect.color) html += ` <span style="display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: ${effect.color}; vertical-align: middle;"></span>`;
    html += `</div>`;
    html += `<div style="color: #6272a4; margin-bottom: 6px;">${effect.description || ''}</div>`;

    if (effect.effects && effect.effects.length > 0) {
        html += '<div style="font-family: monospace;">';
        effect.effects.forEach(e => {
            const sign = e.value >= 0 ? '+' : '';
            let line = `${e.type} ${sign}${e.value}`;
            if (e.duration) line += ` for ${formatDuration(e.duration)}`;
            if (e.tick_interval) line += ` (every ${e.tick_interval} ticks)`;
            html += `<div>${line}</div>`;
        });
        html += '</div>';
    }

    preview.innerHTML = html;
    preview.style.display = 'block';
}

function removeEffect(index) {
    currentEffects.splice(index, 1);
    renderEffects();
}

// ===== WORN EFFECTS MANAGEMENT =====
function renderWornEffects() {
    const container = document.getElementById('wornEffectsContainer');
    if (!container) return;
    container.innerHTML = '';

    if (currentWornEffects.length === 0) {
        container.innerHTML = '<span style="color: #6272a4; font-size: 12px;">No worn effects defined</span>';
        return;
    }

    currentWornEffects.forEach((effectId, index) => {
        const effectData = namedEffectsData[effectId];
        const name = effectData ? effectData.name : effectId;

        const chip = document.createElement('div');
        chip.className = 'tag';
        chip.innerHTML = `
            <span>${name}</span>
            <button type="button" class="tag-remove" onclick="window.removeWornEffect(${index})">×</button>
        `;
        container.appendChild(chip);
    });
}

function addWornEffect() {
    const select = document.getElementById('newWornEffect');
    const effectId = select.value;

    if (!effectId) {
        showStatus('Please select an effect to add', 'error');
        return;
    }

    if (currentWornEffects.includes(effectId)) {
        showStatus('This effect is already added', 'error');
        return;
    }

    currentWornEffects.push(effectId);
    renderWornEffects();
    select.value = '';
}

function removeWornEffect(index) {
    currentWornEffects.splice(index, 1);
    renderWornEffects();
}

// ===== SAVE ITEM =====
async function saveItem() {
    const itemId = document.getElementById('itemId').value.trim();

    if (!itemId) {
        showStatus('Item ID is required', 'error');
        return;
    }

    // Validate ID format (lowercase-with-hyphens)
    if (!/^[a-z0-9-]+$/.test(itemId)) {
        showStatus('Item ID must be lowercase letters, numbers, and hyphens only', 'error');
        return;
    }

    // Start with existing item data to preserve unedited fields (e.g. img)
    const existingItem = (!isNewItem && currentItem && allItems[currentItem]) ? { ...allItems[currentItem] } : {};

    // Build item object, overwriting with form values
    const item = {
        ...existingItem,
        id: itemId,
        name: document.getElementById('itemNameInput').value.trim(),
        description: document.getElementById('itemDescription').value.trim(),
        rarity: document.getElementById('itemRarity').value,
        price: parseInt(document.getElementById('itemPrice').value) || 0,
        weight: parseFloat(document.getElementById('itemWeight').value) || 1,
        stack: parseInt(document.getElementById('itemStack').value) || 1,
        type: document.getElementById('itemType').value.trim(),
        tags: currentTags,
        notes: currentNotes,
        image: document.getElementById('itemImage').value.trim() || `/res/img/items/${itemId}.png`
    };

    // Clean stale conditional fields inherited from existing item
    // Only keep fields relevant to current tags/type
    if (!currentTags.includes('equipment')) {
        delete item.gear_slot;
        delete item.effects_when_worn;
    }
    if (!currentTags.includes('container')) {
        delete item.container_slots;
        delete item.allowed_types;
    }
    if (!currentTags.includes('consumable')) delete item.effects;
    if (!currentTags.includes('focus')) delete item.provides;
    if (!currentTags.includes('pack')) delete item.contents;

    // Add conditional fields from form
    if (currentTags.includes('equipment')) {
        const gearSlot = document.getElementById('gearSlot').value;
        if (!gearSlot) {
            showStatus('Equipment items require a gear slot to be selected.', 'error');
            document.getElementById('gearSlot').focus();
            return;
        }
        item.gear_slot = gearSlot;

        if (currentWornEffects.length > 0) {
            item.effects_when_worn = currentWornEffects;
        } else {
            delete item.effects_when_worn;
        }
    }

    if (currentTags.includes('container')) {
        item.container_slots = parseInt(document.getElementById('containerSlots').value) || 20;
        const allowedTypesSelect = document.getElementById('allowedTypes');
        const selectedOptions = Array.from(allowedTypesSelect.selectedOptions).map(opt => opt.value);

        if (selectedOptions.includes('any') || selectedOptions.length === 0) {
            item.allowed_types = 'any';
        } else {
            item.allowed_types = selectedOptions;
        }
    }

    // Combat properties - clear stale then set from form
    const itemType = item.type.toLowerCase();
    const isArmor = itemType.includes('armor');
    const isWeapon = itemType.includes('melee') || itemType.includes('weapon') || itemType.includes('martial') || itemType.includes('simple');
    const isRanged = itemType.includes('ranged') || currentTags.includes('thrown') || currentTags.includes('ranged');

    if (!isArmor) delete item.ac;
    if (!isWeapon && !isRanged) { delete item.damage; delete item['damage-type']; }
    if (!isRanged) { delete item.ammunition; delete item.range; delete item['range-long']; }

    const ac = document.getElementById('itemAC').value.trim();
    if (ac) item.ac = ac;

    const damage = document.getElementById('itemDamage').value.trim();
    if (damage) item.damage = damage;

    const damageType = document.getElementById('damageType').value;
    if (damageType) item['damage-type'] = damageType;

    // Ranged properties
    const ammunition = document.getElementById('ammunition').value.trim();
    if (ammunition) item.ammunition = ammunition;

    const range = document.getElementById('range').value.trim();
    if (range) item.range = range;

    const rangeLong = document.getElementById('rangeLong').value.trim();
    if (rangeLong) item['range-long'] = rangeLong;

    // Consumable properties - heal is now just an hp effect
    delete item.heal;
    if (currentTags.includes('consumable')) {
        item.effects = currentEffects.length > 0 ? currentEffects : [];
    }

    // Focus properties
    if (currentTags.includes('focus')) {
        const provides = document.getElementById('itemProvides').value.trim();
        if (provides) item.provides = provides;
    }

    // Pack contents
    if (currentTags.includes('pack')) {
        item.contents = currentPackContents;
    }

    // Save to server
    try {
        const filename = isNewItem ? itemId : currentItem;

        // Add session header if in staging mode
        const headers = { 'Content-Type': 'application/json' };
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch(`/api/items/${filename}`, {
            method: 'PUT',
            headers: headers,
            body: JSON.stringify(item)
        });

        if (response.ok) {
            const result = await response.json();

            // Update change count if in staging mode
            if (result.mode === 'staging') {
                updateChangeCount(result.changes);
                showStatus('Item staged for PR (' + result.changes + ' changes)', 'success');
            } else {
                showStatus('Item saved successfully!', 'success');
            }

            // Update local cache
            allItems[itemId] = item;

            if (isNewItem) {
                // Add to type filter if new type
                if (!allItemTypes.has(item.type)) {
                    allItemTypes.add(item.type);
                    populateTypeFilter();
                    populateTypeDropdown();
                }

                // Add to item IDs list
                allItemIds.push({ id: itemId, name: item.name });
                allItemIds.sort((a, b) => a.name.localeCompare(b.name));

                isNewItem = false;
                currentItem = itemId;
            }

            renderItemList(document.getElementById('searchBox').value, document.getElementById('typeFilter').value);
            updateItemCount();
        } else {
            const error = await response.text();
            showStatus('Failed to save item: ' + error, 'error');
        }
    } catch (error) {
        showStatus('Error saving item: ' + error.message, 'error');
    }
}

// ===== DELETE ITEM =====
async function deleteItem() {
    if (!currentItem || isNewItem) return;

    if (!confirm(`Are you sure you want to delete "${allItems[currentItem].name}"?`)) {
        return;
    }

    try {
        // Add session header if in staging mode
        const headers = {};
        if (stagingSessionID) {
            headers['X-Session-ID'] = stagingSessionID;
        }

        const response = await fetch(`/api/items/${currentItem}`, {
            method: 'DELETE',
            headers: headers
        });

        if (response.ok) {
            const result = await response.json();

            // Update change count if in staging mode
            if (result.mode === 'staging') {
                updateChangeCount(result.changes);
                showStatus('Item deletion staged for PR (' + result.changes + ' changes)', 'success');
            } else {
                showStatus('Item deleted successfully', 'success');
            }

            delete allItems[currentItem];
            currentItem = null;

            hideEditor();
            renderItemList();
            updateItemCount();
        } else {
            showStatus('Failed to delete item', 'error');
        }
    } catch (error) {
        showStatus('Error deleting item: ' + error.message, 'error');
    }
}

// ===== VALIDATION =====
async function validateItem() {
    const itemId = document.getElementById('itemId').value.trim();
    if (!itemId) {
        showStatus('No item ID to validate', 'error');
        return;
    }

    showStatus('Validating item...', 'info');

    try {
        const response = await fetch(`/api/validation/item/${itemId}`);
        if (!response.ok) {
            const error = await response.text();
            showStatus(`Validation failed: ${error}`, 'error');
            return;
        }

        const result = await response.json();

        if (result.issues.length === 0) {
            showStatus('Validation passed - no issues found', 'success');
        } else {
            const errors = result.issues.filter(i => i.type === 'error');
            const warnings = result.issues.filter(i => i.type === 'warning');

            let msg = `Validation: ${errors.length} error(s), ${warnings.length} warning(s)`;
            if (errors.length > 0) {
                msg += '\n\nErrors:\n' + errors.map(i => `  - [${i.field || 'general'}] ${i.message}`).join('\n');
            }
            if (warnings.length > 0) {
                msg += '\n\nWarnings:\n' + warnings.map(i => `  - [${i.field || 'general'}] ${i.message}`).join('\n');
            }

            showStatus(msg, errors.length > 0 ? 'error' : 'warning');
        }
    } catch (error) {
        showStatus('Error running validation: ' + error.message, 'error');
    }
}

// ===== IMAGE HANDLING =====
async function checkImage() {
    const imagePath = document.getElementById('itemImage').value;
    const container = document.getElementById('imageContainer');

    // Clear immediately to prevent showing stale image from previous item
    container.innerHTML = '<p style="color: #6272a4;">Loading...</p>';

    if (!imagePath) {
        container.innerHTML = '<p style="color: #6272a4;">No image path set</p>';
        return;
    }

    // Try to load the image
    const img = new Image();
    img.style.width = '100%';
    img.style.imageRendering = 'pixelated';
    img.onload = function() {
        container.innerHTML = '';
        container.appendChild(img);
    };
    img.onerror = function() {
        container.innerHTML = '<p style="color: #ff5555;">Image not found</p>';
    };
    // Use /www prefix to access images from CODEX server
    img.src = imagePath.startsWith('/res/') ? `/www${imagePath}` : imagePath;
}

// ===== IMAGE GENERATION =====
let generatedImageData = null;
let pixelLabBalance = null;

async function generateImage() {
    // Deprecated - use openImageGenerator() instead
    openImageGenerator();
}

async function openImageGenerator() {
    try {
        console.log('openImageGenerator: starting');

        if (!currentItem && !isNewItem) {
            showStatus('Please select or create an item first', 'error');
            return;
        }
        console.log('openImageGenerator: item check passed');

        // Reset state
        generatedImageData = null;
        document.getElementById('generatedImagePreview').innerHTML = '<span style="color: #6272a4; font-size: 12px;">Click Generate</span>';
        document.getElementById('acceptBtn').style.display = 'none';
        document.getElementById('imageGenStatus').textContent = '';
        console.log('openImageGenerator: reset state done');

        // Load balance
        await loadPixelLabBalance();
        console.log('openImageGenerator: balance loaded');

        // Generate prompt preview
        const name = document.getElementById('itemNameInput').value || 'Unknown Item';
        const description = document.getElementById('itemDescription').value || '';
        const rarity = document.getElementById('itemRarity').value || 'common';

        const prompt = generatePromptPreview(name, description, rarity);
        document.getElementById('imageGenPrompt').value = prompt;
        console.log('openImageGenerator: prompt generated');

        // Load current image
        const itemId = document.getElementById('itemId').value;
        const currentImageDiv = document.getElementById('currentImagePreview');
        if (itemId) {
            const img = new Image();
            img.style.width = '100%';
            img.style.height = '100%';
            img.style.objectFit = 'contain';
            img.style.imageRendering = 'pixelated';
            img.onload = () => {
                currentImageDiv.innerHTML = '';
                currentImageDiv.appendChild(img);
            };
            img.onerror = () => {
                currentImageDiv.innerHTML = '<span style="color: #6272a4; font-size: 12px;">No image</span>';
            };
            img.src = `/www/res/img/items/${itemId}.png`;
        } else {
            currentImageDiv.innerHTML = '<span style="color: #6272a4; font-size: 12px;">No image</span>';
        }
        console.log('openImageGenerator: image preview set');

        // Update generate button cost
        updateGenerateButtonCost();
        console.log('openImageGenerator: button cost updated');

        // Show modal
        console.log('openImageGenerator: showing modal');
        document.getElementById('image-gen-modal').style.display = 'flex';
        console.log('openImageGenerator: modal should be visible now');
    } catch (error) {
        console.error('openImageGenerator ERROR:', error);
        showStatus('Error opening image generator: ' + error.message, 'error');
    }
}

function closeImageGenModal() {
    document.getElementById('image-gen-modal').style.display = 'none';
}

async function loadPixelLabBalance() {
    try {
        const response = await fetch('/api/balance');
        const imageGenBalanceEl = document.getElementById('imageGenBalance');

        if (response.ok) {
            const data = await response.json();
            pixelLabBalance = data.usd ?? data.balance ?? 0;

            if (imageGenBalanceEl) {
                imageGenBalanceEl.textContent = `$${pixelLabBalance.toFixed(4)}`;
                imageGenBalanceEl.style.color = pixelLabBalance > 0.1 ? 'var(--codex-green)' : 'var(--codex-orange)';
            }

            // Also update the small balance display in editor
            const balanceDisplay = document.getElementById('balanceDisplay');
            if (balanceDisplay) {
                balanceDisplay.textContent = `PixelLab Balance: $${pixelLabBalance.toFixed(4)}`;
            }
        } else {
            if (imageGenBalanceEl) {
                imageGenBalanceEl.textContent = 'API Error';
                imageGenBalanceEl.style.color = 'var(--codex-red)';
            }
        }
    } catch (error) {
        const imageGenBalanceEl = document.getElementById('imageGenBalance');
        if (imageGenBalanceEl) {
            imageGenBalanceEl.textContent = 'Not configured';
            imageGenBalanceEl.style.color = '#6272a4';
        }
        // Silently fail for balance display in editor - API may not be configured
        const balanceDisplay = document.getElementById('balanceDisplay');
        if (balanceDisplay) {
            balanceDisplay.textContent = '';
        }
    }
}

function generatePromptPreview(name, description, rarity) {
    // Match the server-side prompt generation logic
    let baseDescription = description || name;

    // Add rarity-based styling hints
    let rarityHint = '';
    switch (rarity) {
        case 'uncommon':
            rarityHint = ', subtle magical glow';
            break;
        case 'rare':
            rarityHint = ', magical blue aura, enchanted appearance';
            break;
        case 'legendary':
            rarityHint = ', radiant golden glow, legendary artifact';
            break;
        case 'mythical':
            rarityHint = ', otherworldly radiance, divine craftsmanship';
            break;
    }

    return `32x32 pixel art game item icon, ${baseDescription}${rarityHint}, centered on transparent background, highly detailed, single color black outline, fantasy RPG style`;
}

function updateGenerateButtonCost() {
    const model = document.getElementById('imageGenModel').value;
    const cost = model === 'bitforge' ? '0.03' : '0.05';
    document.getElementById('generateBtn').textContent = `🎨 Generate ($${cost})`;
}

async function generateNewImage() {
    const itemId = document.getElementById('itemId').value;
    if (!itemId) {
        showStatus('Please set an item ID first', 'error');
        return;
    }

    const model = document.getElementById('imageGenModel').value;
    const statusDiv = document.getElementById('imageGenStatus');
    const generateBtn = document.getElementById('generateBtn');
    const previewDiv = document.getElementById('generatedImagePreview');

    // Show loading state
    generateBtn.disabled = true;
    generateBtn.textContent = '⏳ Generating...';
    statusDiv.textContent = 'Generating image with PixelLab AI...';
    statusDiv.style.color = 'var(--codex-cyan)';
    previewDiv.innerHTML = '<span style="color: var(--codex-cyan);">⏳ Generating...</span>';

    try {
        const response = await fetch(`/api/items/${itemId}/generate-image`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model })
        });

        if (response.ok) {
            const data = await response.json();
            generatedImageData = data.imageData;

            // Display generated image
            const img = new Image();
            img.style.width = '100%';
            img.style.height = '100%';
            img.style.objectFit = 'contain';
            img.style.imageRendering = 'pixelated';
            img.onload = () => {
                previewDiv.innerHTML = '';
                previewDiv.appendChild(img);
            };
            img.src = `data:image/png;base64,${data.imageData}`;

            // Update status
            statusDiv.textContent = `✅ Generated successfully! Cost: $${data.cost.toFixed(4)}`;
            statusDiv.style.color = 'var(--codex-green)';

            // Show accept button
            document.getElementById('acceptBtn').style.display = 'inline-block';

            // Refresh balance
            await loadPixelLabBalance();
        } else {
            const error = await response.text();
            statusDiv.textContent = `❌ Error: ${error}`;
            statusDiv.style.color = 'var(--codex-red)';
            previewDiv.innerHTML = '<span style="color: var(--codex-red);">Generation failed</span>';
        }
    } catch (error) {
        statusDiv.textContent = `❌ Error: ${error.message}`;
        statusDiv.style.color = 'var(--codex-red)';
        previewDiv.innerHTML = '<span style="color: var(--codex-red);">Generation failed</span>';
    } finally {
        generateBtn.disabled = false;
        updateGenerateButtonCost();
    }
}

async function acceptGeneratedImage() {
    if (!generatedImageData) {
        showStatus('No generated image to accept', 'error');
        return;
    }

    const itemId = document.getElementById('itemId').value;
    const statusDiv = document.getElementById('imageGenStatus');
    const acceptBtn = document.getElementById('acceptBtn');

    acceptBtn.disabled = true;
    acceptBtn.textContent = '⏳ Saving...';

    try {
        const response = await fetch(`/api/items/${itemId}/accept-image`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ imageData: generatedImageData })
        });

        if (response.ok) {
            statusDiv.textContent = '✅ Image saved successfully!';
            statusDiv.style.color = 'var(--codex-green)';

            // Update the main image preview
            checkImage();

            // Close modal after short delay
            setTimeout(() => {
                closeImageGenModal();
                showStatus('Image generated and saved!', 'success');
            }, 1000);
        } else {
            const error = await response.text();
            statusDiv.textContent = `❌ Failed to save: ${error}`;
            statusDiv.style.color = 'var(--codex-red)';
        }
    } catch (error) {
        statusDiv.textContent = `❌ Error: ${error.message}`;
        statusDiv.style.color = 'var(--codex-red)';
    } finally {
        acceptBtn.disabled = false;
        acceptBtn.textContent = '✅ Accept & Save';
    }
}

// ===== UTILITY =====
function showStatus(message, type = 'success') {
    const statusEl = document.getElementById('statusMessage');
    statusEl.style.whiteSpace = 'pre-wrap';
    statusEl.textContent = message;
    statusEl.className = 'status-message ' + type;
    statusEl.style.display = 'block';

    // Keep validation results visible longer
    const duration = message.includes('\n') ? 15000 : 5000;
    setTimeout(() => {
        statusEl.style.display = 'none';
    }, duration);
}

function cancelEdit() {
    if (isNewItem) {
        hideEditor();
        currentItem = null;
    } else if (currentItem) {
        selectItem(currentItem); // Reload current item
    } else {
        hideEditor();
    }
}

// ===== EXPOSE FUNCTIONS TO WINDOW =====
window.createNewItem = createNewItem;
window.selectItem = selectItem;
window.applyFilters = applyFilters;
window.addTag = addTag;
window.removeTag = removeTag;
window.addNote = addNote;
window.removeNote = removeNote;
window.addPackItem = addPackItem;
window.removePackItem = removePackItem;
window.saveItem = saveItem;
window.deleteItem = deleteItem;
window.validateItem = validateItem;
window.cancelEdit = cancelEdit;
window.checkImage = checkImage;
window.addInlineEffect = addInlineEffect;
window.addNamedEffect = addNamedEffect;
window.removeEffect = removeEffect;
window.previewNamedEffect = previewNamedEffect;
window.addWornEffect = addWornEffect;
window.removeWornEffect = removeWornEffect;
window.generateImage = generateImage;
window.openImageGenerator = openImageGenerator;
window.closeImageGenModal = closeImageGenModal;
window.generateNewImage = generateNewImage;
window.acceptGeneratedImage = acceptGeneratedImage;

// ===== INIT =====
window.addEventListener('DOMContentLoaded', () => {
    loadItems();
    loadEffectsData();
    initStaging();

    // Add enter key support for tag/note input
    document.getElementById('newTagInput')?.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            addTag();
            e.preventDefault();
        }
    });

    document.getElementById('newNoteInput')?.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            addNote();
            e.preventDefault();
        }
    });

    // Update conditional sections when type changes
    document.getElementById('itemType')?.addEventListener('input', updateConditionalSections);

    // Setup effect mode toggle (flat/dice)
    setupEffectModeToggle();

    // Update generate button cost when model changes
    document.getElementById('imageGenModel')?.addEventListener('change', updateGenerateButtonCost);

    // Load PixelLab balance on startup
    loadPixelLabBalance();
});

// ===== STAGING FUNCTIONS =====

// Initialize staging system
async function initStaging() {
    try {
        // Detect mode
        const modeResponse = await fetch('/api/staging/mode');
        const modeData = await modeResponse.json();
        stagingMode = modeData.mode;

        if (stagingMode === 'staging') {
            // Show staging toggle button instead of panel
            const stagingToggle = document.getElementById('staging-toggle');
            if (stagingToggle) {
                stagingToggle.classList.remove('hidden');
            }

            // Create session
            const npub = localStorage.getItem('codex_npub') || 'anonymous';
            const initResponse = await fetch('/api/staging/init', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ npub })
            });
            const session = await initResponse.json();
            stagingSessionID = session.session_id;

            // Load npub from localStorage if exists
            const savedNpub = localStorage.getItem('codex_npub');
            if (savedNpub && document.getElementById('npub-input')) {
                document.getElementById('npub-input').value = savedNpub;
            }

            console.log('✅ Staging mode active, session:', stagingSessionID);
        } else {
            console.log('✅ Direct mode - changes save immediately');
        }
    } catch (error) {
        console.error('Failed to initialize staging:', error);
    }
}

// Update change count display
function updateChangeCount(count) {
    const countEl = document.getElementById('change-count');
    if (countEl) {
        countEl.textContent = `${count} change${count !== 1 ? 's' : ''}`;
    }

    // Update badge on toggle button
    const badge = document.getElementById('staging-badge');
    if (badge) {
        badge.textContent = count;
        badge.style.display = count > 0 ? 'flex' : 'none';
    }
}

// Toggle staging panel
window.toggleStagingPanel = function() {
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
};

// Close staging panel
window.closeStagingPanel = function() {
    const panel = document.getElementById('staging-panel');
    panel.classList.remove('visible');
};

// View staged changes
async function viewStagedChanges() {
    try {
        const response = await fetch(`/api/staging/changes?session_id=${stagingSessionID}`);
        const data = await response.json();

        const changesList = document.getElementById('changes-list');
        changesList.innerHTML = '';

        if (data.changes.length === 0) {
            changesList.innerHTML = '<p style="text-align: center; color: var(--codex-text-secondary);">No changes staged yet</p>';
        } else {
            data.changes.forEach(change => {
                const changeEl = document.createElement('div');
                changeEl.className = 'change-item';

                let typeColor = '#f1fa8c'; // yellow for update
                let typeLabel = 'UPDATE';
                if (change.type === 'create') {
                    typeColor = '#50fa7b';
                    typeLabel = 'CREATE';
                }
                if (change.type === 'delete') {
                    typeColor = '#ff5555';
                    typeLabel = 'DELETE';
                }

                changeEl.innerHTML = `
                    <div style="display: flex; align-items: center; gap: 10px;">
                        <span class="badge" style="background: ${typeColor}; color: #282a36; font-weight: bold;">
                            ${typeLabel}
                        </span>
                        <code style="color: var(--codex-cyan);">${change.file_path}</code>
                    </div>
                `;

                changesList.appendChild(changeEl);
            });
        }

        document.getElementById('changes-modal').style.display = 'flex';
    } catch (error) {
        console.error('Failed to load changes:', error);
        alert('Failed to load staged changes');
    }
}

// Close changes modal
function closeChangesModal() {
    document.getElementById('changes-modal').style.display = 'none';
}

// Submit PR
async function submitPR() {
    const npub = document.getElementById('npub-input')?.value || 'anonymous';
    localStorage.setItem('codex_npub', npub);

    await confirmSubmitPR();
}

// Confirm and submit PR
async function confirmSubmitPR() {
    closeChangesModal();

    try {
        const npub = document.getElementById('npub-input')?.value || 'anonymous';

        const response = await fetch('/api/staging/submit', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: stagingSessionID,
                npub: npub
            })
        });

        if (response.ok) {
            const data = await response.json();
            alert('✅ Pull request created!\n\nOpening PR in new tab...');
            window.open(data.pr_url, '_blank');

            // Re-initialize session
            await initStaging();
            updateChangeCount(0);
        } else if (response.status === 429) {
            // Rate limit error
            const errorData = await response.json();
            const hours = errorData.retry_after_hours || 0;
            const minutes = errorData.retry_after_minutes || 0;
            alert(`⏱️ Rate Limit Exceeded\n\nYou can only submit once every 12 hours.\n\nPlease try again in ${hours}h ${minutes}m.`);
        } else {
            const error = await response.text();
            alert('❌ Failed to create PR:\n' + error);
        }
    } catch (error) {
        console.error('Failed to submit PR:', error);
        alert('❌ Failed to create PR: ' + error.message);
    }
}

// Clear staging
async function clearStaging() {
    if (!confirm('Clear all staged changes? This cannot be undone.')) {
        return;
    }

    try {
        await fetch(`/api/staging/clear?session_id=${stagingSessionID}`, {
            method: 'DELETE'
        });

        // Re-initialize session
        await initStaging();
        updateChangeCount(0);
        alert('✅ Staging cleared');
    } catch (error) {
        console.error('Failed to clear staging:', error);
        alert('Failed to clear staging');
    }
}

// Expose staging functions to window for onclick handlers
window.viewStagedChanges = viewStagedChanges;
window.closeChangesModal = closeChangesModal;
window.submitPR = submitPR;
window.confirmSubmitPR = confirmSubmitPR;
window.clearStaging = clearStaging;

console.log('🎯 CODEX Item Editor loaded');
