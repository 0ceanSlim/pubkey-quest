/**
 * New Game Character Generation Module
 *
 * Handles the character creation flow for new games.
 * Displays generated character, introduction, equipment selection, and creates save file.
 *
 * @module pages/newGame
 */

import { logger } from '../lib/logger.js';
import { showMessage } from '../ui/messaging.js';
import { getItemById } from '../state/staticData.js';
import { createInventoryFromItems, addItemWithStacking } from '../data/inventory.js';
import { generateStartingVault, getDisplayNamesForLocation } from '../data/characters.js';

// Module state
let generatedCharacter = null;
let characterIntroduction = null;
let startingEquipment = null;
let selectedEquipment = {};

/**
 * Initialize character generation on page load
 */
async function initializeNewGame() {
    logger.info('Initializing new game...');

    if (!window.sessionManager) {
        showMessage('❌ Session manager not available', 'error');
        goToSaves();
        return;
    }

    try {
        await window.sessionManager.init();

        if (!window.sessionManager.isAuthenticated()) {
            logger.warn('User not authenticated, redirecting to home');
            window.location.href = '/';
            return;
        }

        const session = window.sessionManager.getSession();
        logger.info('User authenticated:', session.npub);

        await generateCharacterFromNpub(session.npub);

    } catch (error) {
        logger.error('Failed to initialize new game:', error);
        showMessage('❌ Failed to create character: ' + error.message, 'error');
    }
}

/**
 * Generate character from npub
 * @param {string} npub - Nostr public key
 */
async function generateCharacterFromNpub(npub) {
    try {
        // Initialize character generator
        await window.characterGenerator.initialize();

        // Generate character from npub
        generatedCharacter = await window.characterGenerator.generateCharacter(npub);
        characterIntroduction = window.characterGenerator.generateIntroduction(generatedCharacter);
        startingEquipment = window.characterGenerator.generateStartingEquipment(generatedCharacter);

        logger.debug('Generated character:', generatedCharacter);
        logger.debug('Character introduction:', characterIntroduction);
        logger.debug('Starting equipment:', startingEquipment);

        // Display the character
        displayGeneratedCharacter();

        // Hide loading, show character
        document.getElementById('generation-loading').classList.add('hidden');
        document.getElementById('character-display').classList.remove('hidden');

    } catch (error) {
        logger.error('Character generation failed:', error);
        showMessage('❌ Character generation failed: ' + error.message, 'error');

        // Show error state
        const loadingEl = document.getElementById('generation-loading');
        loadingEl.innerHTML = '';

        const errorDiv = document.createElement('div');
        errorDiv.className = 'text-center py-8';

        const errorIcon = document.createElement('div');
        errorIcon.className = 'text-red-400 text-xl mb-2';
        errorIcon.textContent = '❌';
        errorDiv.appendChild(errorIcon);

        const errorTitle = document.createElement('p');
        errorTitle.className = 'text-red-400';
        errorTitle.textContent = 'Character generation failed';
        errorDiv.appendChild(errorTitle);

        const errorMessage = document.createElement('p');
        errorMessage.className = 'text-gray-500 text-sm';
        errorMessage.textContent = error.message;
        errorDiv.appendChild(errorMessage);

        const retryButton = document.createElement('button');
        retryButton.className = 'mt-4 bg-yellow-600 hover:bg-yellow-700 text-gray-900 px-4 py-2 rounded';
        retryButton.textContent = '🔄 Try Again';
        retryButton.onclick = function() { window.location.reload(); };
        errorDiv.appendChild(retryButton);

        loadingEl.appendChild(errorDiv);
    }
}

/**
 * Display generated character details
 */
function displayGeneratedCharacter() {
    if (!generatedCharacter) return;

    // Character icon
    const raceIcons = {
        'Human': '👤', 'Elf': '🧝', 'Dwarf': '🧔', 'Halfling': '🧒',
        'Dragonborn': '🐲', 'Gnome': '🧙', 'Half-Elf': '👨‍🎤',
        'Half-Orc': '👹', 'Tiefling': '😈', 'Orc': '👹'
    };

    document.getElementById('character-icon').textContent = raceIcons[generatedCharacter.race] || '⚔️';
    document.getElementById('character-name').textContent = generatedCharacter.name;
    document.getElementById('character-title').textContent = generatedCharacter.race + ' ' + generatedCharacter.class;

    // Character details
    document.getElementById('char-race').textContent = generatedCharacter.race;
    document.getElementById('char-class').textContent = generatedCharacter.class;
    document.getElementById('char-background').textContent = generatedCharacter.background;
    document.getElementById('char-alignment').textContent = generatedCharacter.alignment;
    document.getElementById('char-level').textContent = generatedCharacter.level;

    // Ability scores
    document.getElementById('stat-strength').textContent = generatedCharacter.stats.strength;
    document.getElementById('stat-dexterity').textContent = generatedCharacter.stats.dexterity;
    document.getElementById('stat-constitution').textContent = generatedCharacter.stats.constitution;
    document.getElementById('stat-intelligence').textContent = generatedCharacter.stats.intelligence;
    document.getElementById('stat-wisdom').textContent = generatedCharacter.stats.wisdom;
    document.getElementById('stat-charisma').textContent = generatedCharacter.stats.charisma;

    // Character stats
    document.getElementById('char-hp').textContent = generatedCharacter.hp;
    document.getElementById('char-mana').textContent = generatedCharacter.mana;
    document.getElementById('char-gold').textContent = generatedCharacter.gold;
}

/**
 * Show character introduction story
 */
function showIntroduction() {
    if (!characterIntroduction) return;

    const introContent = document.getElementById('intro-content');
    const introSection = document.getElementById('introduction-section');

    // Clear any existing content
    introContent.innerHTML = '';

    // Base introduction
    const introScene = document.createElement('div');
    introScene.className = 'intro-scene mb-6';

    const scene1 = document.createElement('p');
    scene1.className = 'mb-3';
    scene1.textContent = characterIntroduction.baseIntro.scene1;
    introScene.appendChild(scene1);

    const scene2 = document.createElement('p');
    scene2.className = 'mb-3';
    scene2.textContent = characterIntroduction.baseIntro.scene2;
    introScene.appendChild(scene2);

    const scene3 = document.createElement('p');
    scene3.className = 'mb-3';
    scene3.textContent = characterIntroduction.baseIntro.scene3;
    introScene.appendChild(scene3);

    const quote = document.createElement('blockquote');
    quote.className = 'border-l-4 border-yellow-400 pl-4 italic text-yellow-200 mb-3';
    quote.textContent = '"' + characterIntroduction.baseIntro.final_words + '"';
    introScene.appendChild(quote);

    const letterIntro = document.createElement('p');
    letterIntro.textContent = characterIntroduction.baseIntro.letter_intro;
    introScene.appendChild(letterIntro);

    introContent.appendChild(introScene);

    // Background-specific content with race elements
    if (characterIntroduction.backgroundIntro) {
        const backgroundDiv = document.createElement('div');
        backgroundDiv.className = 'background-intro mb-6';

        const title = document.createElement('h4');
        title.className = 'text-lg font-medium text-cyan-400 mb-3';
        title.textContent = 'Your ' + generatedCharacter.race + ' Heritage and ' + generatedCharacter.background + ' Past';
        backgroundDiv.appendChild(title);

        const scenePara = document.createElement('p');
        scenePara.className = 'mb-3';
        scenePara.textContent = addRaceFlavorToScene(characterIntroduction.backgroundIntro.scene, generatedCharacter.race);
        backgroundDiv.appendChild(scenePara);

        const letterDiv = document.createElement('div');
        letterDiv.className = 'bg-gray-700 rounded p-4 mt-4';

        const letterTitle = document.createElement('h5');
        letterTitle.className = 'text-sm font-medium text-gray-300 mb-2';
        letterTitle.textContent = 'The Letter:';
        letterDiv.appendChild(letterTitle);

        const letterText = document.createElement('p');
        letterText.className = 'text-sm text-gray-200 italic';
        letterText.textContent = characterIntroduction.backgroundIntro.letter;
        letterDiv.appendChild(letterText);

        backgroundDiv.appendChild(letterDiv);

        introContent.appendChild(backgroundDiv);
    }

    // Equipment introduction
    if (characterIntroduction.equipmentIntro) {
        const equipDiv = document.createElement('div');
        equipDiv.className = 'equipment-intro mb-6';

        const equipTitle = document.createElement('h4');
        equipTitle.className = 'text-lg font-medium text-purple-400 mb-3';
        equipTitle.textContent = 'Your Training';
        equipDiv.appendChild(equipTitle);

        const equipScene = document.createElement('p');
        equipScene.className = 'mb-3';
        equipScene.textContent = characterIntroduction.equipmentIntro.scene;
        equipDiv.appendChild(equipScene);

        const equipQuote = document.createElement('blockquote');
        equipQuote.className = 'border-l-4 border-purple-400 pl-4 italic text-purple-200';
        equipQuote.textContent = '"' + characterIntroduction.equipmentIntro.quote + '"';
        equipDiv.appendChild(equipQuote);

        introContent.appendChild(equipDiv);
    }

    // Final note
    const finalDiv = document.createElement('div');
    finalDiv.className = 'final-note mb-6';

    const finalText = document.createElement('p');
    finalText.className = 'mb-2';
    finalText.textContent = characterIntroduction.finalNote.text;
    finalDiv.appendChild(finalText);

    const finalQuote = document.createElement('blockquote');
    finalQuote.className = 'border-l-4 border-green-400 pl-4 italic text-green-200';
    finalQuote.textContent = '"' + characterIntroduction.finalNote.quote + '"';
    finalDiv.appendChild(finalQuote);

    introContent.appendChild(finalDiv);

    // Departure
    const departureDiv = document.createElement('div');
    departureDiv.className = 'departure text-center bg-gray-700 rounded p-4';
    const departurePara = document.createElement('p');
    departurePara.className = 'text-gray-300';
    departurePara.textContent = characterIntroduction.departure.text;
    departureDiv.appendChild(departurePara);
    introContent.appendChild(departureDiv);
    introSection.classList.remove('hidden');

    // Update buttons
    document.getElementById('show-intro-btn').style.display = 'none';
    document.getElementById('show-equipment-btn').style.display = 'block';
}

/**
 * Show equipment selection interface
 */
async function showEquipmentSelection() {
    const equipmentSection = document.getElementById('equipment-selection');
    const equipmentChoices = document.getElementById('equipment-choices');
    const startingInventory = document.getElementById('starting-inventory');

    // Show automatic equipment
    startingInventory.innerHTML = '';
    startingEquipment.inventory.forEach(item => {
        const itemDiv = document.createElement('div');
        itemDiv.className = 'bg-gray-700 rounded p-2 text-sm';

        const itemName = document.createElement('span');
        itemName.className = 'text-white';
        itemName.textContent = item.item;
        itemDiv.appendChild(itemName);

        if (item.quantity > 1) {
            const quantity = document.createElement('span');
            quantity.className = 'text-gray-400';
            quantity.textContent = ' (' + item.quantity + ')';
            itemDiv.appendChild(quantity);
        }

        startingInventory.appendChild(itemDiv);
    });

    // Show equipment choices
    equipmentChoices.innerHTML = '';
    if (startingEquipment.choices && startingEquipment.choices.length > 0) {
        startingEquipment.choices.forEach((choice, index) => {
            const choiceGroup = document.createElement('div');
            choiceGroup.className = 'choice-group bg-gray-700 rounded p-4';

            const choiceTitle = document.createElement('h5');
            choiceTitle.className = 'text-md font-medium text-yellow-400 mb-3';
            choiceTitle.textContent = 'Choose one:';
            choiceGroup.appendChild(choiceTitle);

            const optionsDiv = document.createElement('div');
            optionsDiv.className = 'space-y-2';

            choice.options.forEach((option, optionIndex) => {
                const label = document.createElement('label');
                label.className = 'flex items-center space-x-3 cursor-pointer hover:bg-gray-600 rounded p-2';

                const input = document.createElement('input');
                input.type = 'radio';
                input.name = 'choice_' + index;
                input.value = option.item;
                input.id = 'choice_' + index + '_' + optionIndex;
                input.onchange = function() { selectEquipment(index, option); };
                label.appendChild(input);

                const span = document.createElement('span');
                span.className = 'text-white';
                // Show quantity for simple items when > 1
                if (option.quantity && option.quantity > 1 && !option.isComplexChoice && !option.isBundle) {
                    span.textContent = `${option.item} x${option.quantity}`;
                } else {
                    span.textContent = option.item;
                }
                label.appendChild(span);

                // Add pack contents display for packs
                if (option.item.includes('pack')) {
                    const packContentsDiv = document.createElement('div');
                    packContentsDiv.className = 'ml-6 mt-1 text-xs text-gray-300';
                    packContentsDiv.id = 'pack_contents_' + index + '_' + optionIndex;

                    // Load and display pack contents asynchronously
                    (async () => {
                        try {
                            const packData = await getItemById(option.item);
                            if (packData && packData.contents) {
                                const contents = typeof packData.contents === 'string'
                                    ? JSON.parse(packData.contents)
                                    : packData.contents;

                                // Convert item IDs to display names
                                const contentsList = await Promise.all(contents.map(async item => {
                                    if (Array.isArray(item) && item.length === 2) {
                                        const itemId = item[0];
                                        const quantity = item[1];

                                        // Skip backpack since it becomes the container
                                        if (itemId === 'backpack') return null;

                                        // Get item data to get display name
                                        const itemData = await getItemById(itemId);
                                        const displayName = itemData ? itemData.name : itemId;

                                        return quantity > 1 ? `${displayName} x${quantity}` : displayName;
                                    }
                                    return item;
                                }));

                                // Filter out null entries (like backpack) and join
                                const filteredList = contentsList.filter(item => item !== null);
                                packContentsDiv.textContent = `Contains: ${filteredList.join(', ')}`;
                            }
                        } catch (error) {
                            logger.warn('Failed to load pack contents:', error);
                        }
                    })();

                    label.appendChild(packContentsDiv);
                }

                // Add weapon selection sub-interface for complex choices
                if (option.isComplexChoice) {
                    const weaponSelector = document.createElement('div');
                    weaponSelector.className = 'ml-6 mt-2 hidden weapon-selector';
                    weaponSelector.id = 'weapon_selector_' + index + '_' + optionIndex;

                    option.weaponSlots.forEach((slot, slotIndex) => {
                        if (slot.type === 'weapon_choice') {
                            const weaponDiv = document.createElement('div');
                            weaponDiv.className = 'mb-2';

                            const weaponLabel = document.createElement('label');
                            weaponLabel.className = 'block text-sm text-yellow-400 mb-1';
                            weaponLabel.textContent = `Choose weapon ${slotIndex + 1}:`;
                            weaponDiv.appendChild(weaponLabel);

                            const weaponSelect = document.createElement('select');
                            weaponSelect.className = 'bg-gray-600 text-white rounded px-2 py-1 text-sm';
                            weaponSelect.name = `weapon_${index}_${optionIndex}_${slotIndex}`;

                            slot.options.forEach((weapon, weaponIndex) => {
                                const weaponOption = document.createElement('option');
                                weaponOption.value = weapon[0];
                                weaponOption.textContent = `${weapon[0]} (x${weapon[1]})`;
                                if (weaponIndex === 0) weaponOption.selected = true;
                                weaponSelect.appendChild(weaponOption);
                            });

                            weaponDiv.appendChild(weaponSelect);
                            weaponSelector.appendChild(weaponDiv);
                        }
                    });

                    label.appendChild(weaponSelector);

                    // Show/hide weapon selector when radio button changes
                    input.onchange = function() {
                        // Hide all weapon selectors for this choice
                        const allSelectors = document.querySelectorAll(`[id^="weapon_selector_${index}_"]`);
                        allSelectors.forEach(s => s.classList.add('hidden'));

                        // Show this weapon selector if it's a complex choice
                        if (option.isComplexChoice) {
                            weaponSelector.classList.remove('hidden');
                        }

                        selectEquipment(index, option);
                    };
                }

                optionsDiv.appendChild(label);
            });

            choiceGroup.appendChild(optionsDiv);
            equipmentChoices.appendChild(choiceGroup);
        });
    } else {
        const noChoices = document.createElement('p');
        noChoices.className = 'text-gray-400';
        noChoices.textContent = 'No equipment choices needed - your starting gear is ready!';
        equipmentChoices.appendChild(noChoices);
    }
    equipmentSection.classList.remove('hidden');

    // Update buttons
    document.getElementById('show-equipment-btn').style.display = 'none';
    document.getElementById('start-adventure-btn').style.display = 'block';
}

/**
 * Handle equipment selection
 * @param {number} choiceIndex - Choice index
 * @param {Object} option - Selected option
 */
function selectEquipment(choiceIndex, option) {
    if (option.isComplexChoice) {
        // Store the complex choice structure for processing later
        selectedEquipment[choiceIndex] = {
            ...option,
            getSelectedWeapons: function() {
                const selectedWeapons = [];

                option.weaponSlots.forEach((slot, slotIndex) => {
                    if (slot.type === 'weapon_choice') {
                        const selectElement = document.querySelector(`select[name="weapon_${choiceIndex}_0_${slotIndex}"]`);
                        if (selectElement) {
                            selectedWeapons.push([selectElement.value, 1]);
                        }
                    } else if (slot.type === 'fixed_item') {
                        selectedWeapons.push(slot.item);
                    }
                });

                return selectedWeapons;
            }
        };
    } else {
        selectedEquipment[choiceIndex] = option;
    }

    logger.debug('Selected equipment:', selectedEquipment);
}

/**
 * Start adventure and create save file
 */
async function startAdventure() {
    try {
        showMessage('🎮 Starting your adventure...', 'info');

        // Get the final character name (either edited or generated)
        const characterNameInput = document.getElementById('character-name-input');
        const finalName = characterNameInput.style.display === 'none' ?
            generatedCharacter.name : characterNameInput.value.trim() || generatedCharacter.name;

        // Collect all items (starting + selected)
        let allItems = [];

        // Add starting equipment with proper stacking
        for (const startingItem of startingEquipment.inventory) {
            await addItemWithStacking(allItems, startingItem.item, startingItem.quantity);
        }

        // Add selected equipment to items list with proper stacking
        for (const option of Object.values(selectedEquipment)) {
            if (option.isComplexChoice) {
                // Handle complex weapon choices
                const selectedWeapons = option.getSelectedWeapons();
                for (const weapon of selectedWeapons) {
                    await addItemWithStacking(allItems, weapon[0], weapon[1]);
                }
            } else if (option.isBundle) {
                for (const bundleItem of option.bundle) {
                    await addItemWithStacking(allItems, bundleItem[0], bundleItem[1]);
                }
            } else {
                await addItemWithStacking(allItems, option.item, option.quantity);
            }
        }

        // Create proper inventory structure with dynamic equipment placement
        const inventory = await createInventoryFromItems(allItems);

        // Prepare final character data
        const finalCharacter = {
            name: finalName,
            race: generatedCharacter.race,
            class: generatedCharacter.class,
            background: generatedCharacter.background,
            alignment: generatedCharacter.alignment,
            level: generatedCharacter.level,
            experience: generatedCharacter.experience,
            stats: generatedCharacter.stats,
            hp: generatedCharacter.hp,
            max_hp: generatedCharacter.max_hp,
            mana: generatedCharacter.mana,
            max_mana: generatedCharacter.max_mana,
            fatigue: generatedCharacter.fatigue,
            gold: generatedCharacter.gold,
            inventory: inventory,
            spells: generatedCharacter.spells || {}
        };

        logger.debug('Final character:', finalCharacter);

        // Generate starting vault (40 slots, empty, city-based)
        const startingCity = generatedCharacter.city || 'kingdom';
        const startingVault = generateStartingVault(startingCity);
        logger.info('Generated vault for', startingCity, ':', startingVault);

        // Convert location IDs to display names for save file
        const displayNames = await getDisplayNamesForLocation(startingCity, 'center', '');
        logger.info('Display names:', displayNames);

        // Create new save file with FLAT structure (backend expects flat, not nested)
        const session = window.sessionManager.getSession();
        const saveData = {
            d: finalName,
            race: generatedCharacter.race,
            class: generatedCharacter.class,
            background: generatedCharacter.background,
            alignment: generatedCharacter.alignment,
            experience: generatedCharacter.experience || 0,
            hp: generatedCharacter.hp,
            max_hp: generatedCharacter.max_hp,
            mana: generatedCharacter.mana,
            max_mana: generatedCharacter.max_mana,
            fatigue: generatedCharacter.fatigue || 0,
            gold: generatedCharacter.gold,
            stats: generatedCharacter.stats,
            location: displayNames.location,
            district: displayNames.district,
            building: displayNames.building || '',
            inventory: inventory,
            vault: startingVault,
            known_spells: generatedCharacter.spells || [],
            spell_slots: {},
            locations_discovered: [startingCity],
            music_tracks_unlocked: ['character-creation', 'kingdom-theme'],
            current_day: 1,
            time_of_day: 12,  // 0=midnight, 12=noon, 23=11 PM
            movement_counter: 0  // Tracks movements for fatigue (every 4 hours = +1 fatigue)
        };

        logger.debug('Creating save with data:', saveData);

        const response = await fetch(`/api/saves/${session.npub}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(saveData)
        });

        if (response.ok) {
            const result = await response.json();
            showMessage('✅ Adventure begins!', 'success');

            setTimeout(() => {
              window.location.href = '/game?save=' + result.id;
            }, 1500);
        } else {
            throw new Error('Failed to create save file');
        }

    } catch (error) {
        logger.error('Failed to start adventure:', error);
        showMessage('❌ Failed to start adventure: ' + error.message, 'error');
    }
}

/**
 * Regenerate character with new seed
 */
function regenerateCharacter() {
    // For development - regenerate with random seed
    const session = window.sessionManager.getSession();
    const randomSeed = session.npub + Date.now().toString();
    window.location.href = '/new-game?seed=' + encodeURIComponent(randomSeed);
}

/**
 * Navigate to saves page
 */
function goToSaves() {
    window.location.href = '/saves';
}

/**
 * Add race-specific flavor to background scenes
 * @param {string} scene - Original scene text
 * @param {string} race - Character race
 * @returns {string} Modified scene with race flavor
 */
function addRaceFlavorToScene(scene, race) {
    const raceModifiers = {
        'Human': 'With the adaptability common to your human lineage, ',
        'Elf': 'Your elven grace and centuries-old wisdom guided you as ',
        'Dwarf': 'The sturdy resilience of your dwarven ancestors strengthened you while ',
        'Halfling': 'Your halfling\'s natural curiosity and comfort with simple pleasures meant ',
        'Dragonborn': 'The draconic heritage flowing in your veins instilled pride as ',
        'Gnome': 'Your gnomish ingenuity and love of tinkering made you naturally suited as ',
        'Half-Elf': 'Caught between two worlds, your half-elf nature gave you unique perspective as ',
        'Half-Orc': 'Despite the suspicious looks your orcish heritage often earned, you proved yourself as ',
        'Tiefling': 'Though others feared your infernal bloodline, you channeled that strength into becoming ',
        'Orc': 'Your orcish strength and warrior culture shaped you into '
    };

    const modifier = raceModifiers[race] || '';

    // Insert race modifier at the beginning or find a natural place to insert it
    if (scene.includes('The old caretaker')) {
        return scene.replace('The old caretaker', modifier + 'the old caretaker');
    } else if (scene.includes('They')) {
        return scene.replace('They', modifier + 'they');
    } else {
        return modifier + scene.charAt(0).toLowerCase() + scene.slice(1);
    }
}

/**
 * Enable character name editing
 */
function editCharacterName() {
    const nameDisplay = document.getElementById('character-name');
    const nameInput = document.getElementById('character-name-input');

    // Switch to input mode
    nameDisplay.style.display = 'none';
    nameInput.style.display = 'block';
    nameInput.value = nameDisplay.textContent;
    nameInput.focus();
    nameInput.select();

    // Handle saving the name
    function saveName() {
        const newName = nameInput.value.trim();
        if (newName && newName.length <= 20) {
            nameDisplay.textContent = newName;
        }
        nameInput.style.display = 'none';
        nameDisplay.style.display = 'block';
    }

    // Save on Enter or blur
    nameInput.onkeydown = function(e) {
        if (e.key === 'Enter') {
            e.preventDefault();
            saveName();
        } else if (e.key === 'Escape') {
            e.preventDefault();
            nameInput.style.display = 'none';
            nameDisplay.style.display = 'block';
        }
    };

    nameInput.onblur = saveName;
}

// Initialize on DOM ready
if (typeof document !== 'undefined') {
    document.addEventListener('DOMContentLoaded', initializeNewGame);
}

// Export functions for global access
export {
    showIntroduction,
    showEquipmentSelection,
    startAdventure,
    regenerateCharacter,
    goToSaves,
    editCharacterName
};

// Make functions globally available for compatibility
if (typeof window !== 'undefined') {
    window.showIntroduction = showIntroduction;
    window.showEquipmentSelection = showEquipmentSelection;
    window.startAdventure = startAdventure;
    window.regenerateCharacter = regenerateCharacter;
    window.goToSaves = goToSaves;
    window.editCharacterName = editCharacterName;
}

logger.debug('New game scripts loaded');
