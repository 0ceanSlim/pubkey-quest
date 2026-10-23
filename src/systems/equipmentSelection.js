/**
 * Equipment Selection System
 *
 * The starting-gear choice flow, including complex weapon selections and pack
 * choices, matching the character-generator logic. It runs in game, over the
 * vault keeper's vault, when a new character takes up their inheritance
 * (ui/startingGear.js builds the overlay: #gear-container, #gear-background,
 * #gear-content).
 *
 * @module systems/equipmentSelection
 */

import {
  createContinueButton,
  waitForButtonClick
} from '../components/continueButton.js';
import {
  createBackButton
} from '../components/backButton.js';
import {
  getItemById
} from '../state/staticData.js'; // For basic item data lookup
import {
  getItemStats
} from '../data/items.js'; // For fetching detailed item stats HTML
import {
  logger
} from '../lib/logger.js'; // Assuming logger is available
import { showItemDetails } from './inventoryInteractions.js';

let currentChoiceIndex = 0;
let selectedChoices = {};
let equipmentChoices = [];
let itemStatsCache = {}; // Cache for item stats
let selectionHistory = []; // Track history for back button
let currentBackButton = null; // Current back button element
let backButtonCallback = null; // Callback to go back

/** Where the screens' Back button and item pop-up go: the inheritance overlay. */
function gearHost() {
  return document.getElementById('gear-overlay') || document.body;
}

// Moving between screens: the vault stays up behind and only the panel
// cross-fades (it used to drop to black between every screen).
const FADE_MS = 180;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/** Bring a screen in: the scene shows (once) and the panel fades up. */
function showPanel() {
  const container = document.getElementById('gear-container');
  const content = document.getElementById('gear-content');
  container.classList.remove('hidden', 'fade-out');
  container.classList.add('fade-in');
  container.style.opacity = '';
  void content.offsetHeight;
  content.classList.remove('gear-fade');
}

/** Fade the panel out and empty it, ready for the next screen. */
async function clearPanel() {
  const content = document.getElementById('gear-content');
  content.classList.add('gear-fade');
  await sleep(FADE_MS);
  content.innerHTML = '';
}

// ============================================================================
// PUBLIC API
// ============================================================================

/**
 * Start equipment selection flow (excluding pack)
 * @param {Object} equipment - The equipment choices provided by character generator.
 * @param {Array<Object>} equipment.choices - Array of choice objects.
 */
export async function startEquipmentSelection(equipment) {
  logger.debug("⚙️ Starting equipment selection with:", equipment);
  equipmentChoices = equipment.choices || [];
  currentChoiceIndex = 0;
  selectedChoices = {};
  selectionHistory = []; // Clear history on new start

  if (equipmentChoices.length === 0) {
    logger.debug("⚙️ No equipment choices, skipping selection.");
    return;
  }

  // Show each choice as a separate scene (but stop before pack)
  while (currentChoiceIndex < equipmentChoices.length) {
    logger.debug(`⚙️ Showing choice ${currentChoiceIndex + 1}/${equipmentChoices.length}`);
    const choice = equipmentChoices[currentChoiceIndex];
    const shouldGoBack = await showEquipmentChoiceScene(choice, currentChoiceIndex);

    if (shouldGoBack) {
      // Go back to previous choice
      currentChoiceIndex--;
      if (currentChoiceIndex >= 0) {
        delete selectedChoices[currentChoiceIndex];
        logger.debug(`⚙️ Went back to choice ${currentChoiceIndex + 1}. Selected choices:`, selectedChoices);
      } else {
        // If already at first choice and going back, essentially cancel selection
        logger.warn("⚙️ Attempted to go back before first choice. Resetting selection.");
        return; // Or handle as a cancellation
      }
    } else {
      // Continue to next choice
      currentChoiceIndex++;
      logger.debug(`⚙️ Proceeded to next choice. Selected choices:`, selectedChoices);
    }
  }

  // Show confirmation after all equipment choices are made
  logger.debug("⚙️ All equipment choices made. Showing final confirmation.");
  const confirmed = await showFinalConfirmation();
  if (!confirmed) {
    logger.debug("⚙️ Final confirmation rejected. Going back to last choice.");
    // Go back to last choice
    currentChoiceIndex = equipmentChoices.length - 1;
    delete selectedChoices[currentChoiceIndex];
    // Restart the selection process (might be better to have a dedicated "back to first choice" mechanism)
    // For now, re-init with the same choices.
    return startEquipmentSelection({
      choices: equipmentChoices
    });
  }
  logger.debug("⚙️ Final confirmation accepted.");
}

/**
 * Handle pack selection (called after other equipment choices)
 * @param {Object} startingEquipment - The initial equipment data from character generator.
 */
export async function handlePackSelection(startingEquipment) {
  logger.debug("📦 Handling pack selection with:", startingEquipment);
  const container = document.getElementById('gear-container');
  const background = document.getElementById('gear-background');
  const content = document.getElementById('gear-content');

  const packChoice = startingEquipment.pack_choice;
  const packGiven = startingEquipment.inventory?.find(item =>
    item.item && item.item.includes('-pack')
  );

  content.innerHTML = '';
  content.style.zIndex = '10';

  if (packChoice) {
    logger.debug("📦 Player needs to choose a pack.");
    const title = document.createElement('div');
    title.className = 'text-xl md:text-2xl font-bold text-yellow-400 mb-4';
    title.textContent = 'Choose Your Pack';
    content.appendChild(title);

    const description = document.createElement('div');
    description.className = 'text-lg mb-6';
    description.textContent = packChoice.description || 'Choose your adventuring pack';
    content.appendChild(description);

    const allPackItems = [];
    for (const option of packChoice.options) {
      const packData = await fetchPackData(option.item);
      const contents = packData?.properties?.contents || packData?.contents;
      if (contents) {
        contents.forEach(item => allPackItems.push(item[0]));
      }
    }
    await preloadItemStats(allPackItems);

    const scrollContainer = document.createElement('div');
    scrollContainer.className = 'w-full max-w-6xl mx-auto overflow-y-auto px-4 mb-4';
    scrollContainer.style.maxHeight = 'calc(100vh - 280px)';

    const optionsContainer = document.createElement('div');
    optionsContainer.className = 'gear-bundles';

    let selectedPackIndex = null;

    for (let i = 0; i < packChoice.options.length; i++) {
      const option = packChoice.options[i];
      const packName = option.item;
      const packData = await fetchPackData(packName);

      if (i > 0) optionsContainer.appendChild(orMark());
      const packContainer = document.createElement('div');
      packContainer.className = 'gear-option gear-bundle';
      packContainer.setAttribute('data-option-index', i);

      const packTitle = document.createElement('div');
      packTitle.className = 'gear-option-label';
      packTitle.textContent = getItemById(packName)?.name || packName.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
      packContainer.appendChild(packTitle);

      const contents = packData?.properties?.contents || packData?.contents;
      if (contents) {
        const itemsRow = document.createElement('div');
        itemsRow.className = 'gear-cards';

        for (const packItem of contents) {
          const itemName = packItem[0];
          const quantity = packItem[1];
          const itemCard = await createSimpleItemCard(itemName, quantity, true);

          itemCard.addEventListener('click', (e) => {
            if (!e.target.closest('.info-btn')) {
              e.stopPropagation();
              packContainer.click();
            }
          });
          itemsRow.appendChild(itemCard);
        }
        packContainer.appendChild(itemsRow);
      }

      packContainer.onclick = (e) => {
        if (e.target.closest('.info-btn')) return;

        optionsContainer.querySelectorAll('[data-option-index]').forEach(opt => {
          opt.classList.remove('selected');
        });
        packContainer.classList.add('selected');
        selectedPackIndex = i;
        confirmBtn.disabled = false;
        confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
      };
      optionsContainer.appendChild(packContainer);
    }

    scrollContainer.appendChild(optionsContainer);
    content.appendChild(scrollContainer);

    const confirmBtn = createContinueButton(0, 'Confirm Selection');
    confirmBtn.disabled = true;
    confirmBtn.classList.add('opacity-50', 'cursor-not-allowed');
    content.appendChild(confirmBtn);

    showPanel();

    await new Promise(resolve => {
      confirmBtn.onclick = () => {
        if (selectedPackIndex !== null) {
          selectedChoices['pack'] = packChoice.options[selectedPackIndex].item;
          resolve();
        }
      };
    });

  } else if (packGiven) {
    logger.debug("📦 Player has a pack already provided:", packGiven.item);
    const title = document.createElement('div');
    title.className = 'text-xl md:text-2xl font-bold text-yellow-400 mb-4';
    title.textContent = 'Your Pack';
    content.appendChild(title);

    const description = document.createElement('div');
    description.className = 'text-lg mb-4 text-center';
    description.textContent = 'You have been provided with this pack:';
    content.appendChild(description);

    const packName = packGiven.item;
    const packData = await fetchPackData(packName);

    const contents = packData?.properties?.contents || packData?.contents;
    if (contents) {
      await preloadItemStats(contents.map(item => item[0]));
    }

    const packContainer = document.createElement('div');
    packContainer.className = 'gear-option gear-bundle gear-given';

    const packTitle = document.createElement('div');
    packTitle.className = 'gear-option-label';
    packTitle.textContent = getItemById(packName)?.name || packName.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
    packContainer.appendChild(packTitle);

    if (contents) {
      const itemsRow = document.createElement('div');
      itemsRow.className = 'gear-cards';

      for (const packItem of contents) {
        const itemName = packItem[0];
        const quantity = packItem[1];
        const itemCard = await createSimpleItemCard(itemName, quantity, true);
        itemsRow.appendChild(itemCard);
      }
      packContainer.appendChild(itemsRow);
    }
    content.appendChild(packContainer);

    selectedChoices['pack'] = packName;

    const continueBtn = createContinueButton(0, 'Continue');
    continueBtn.className += ' mt-6';
    content.appendChild(continueBtn);

    showPanel();

    await new Promise(resolve => {
      continueBtn.onclick = resolve;
    });
  } else {
    logger.debug("📦 No pack choice and no pack given. Skipping pack selection.");
  }

  await clearPanel();
}

/**
 * Get all selected equipment.
 * @returns {Object} An object mapping choice index/key to selected equipment.
 */
export function getSelectedEquipment() {
  logger.debug("⚙️ Getting selected equipment:", selectedChoices);
  return selectedChoices;
}

// ============================================================================
// PRIVATE HELPER FUNCTIONS
// ============================================================================

/**
 * Show final confirmation before proceeding to given items.
 * @returns {Promise<boolean>} True if confirmed, false if back.
 */
async function showFinalConfirmation() {
  const container = document.getElementById('gear-container');
  const background = document.getElementById('gear-background');
  const content = document.getElementById('gear-content');

  content.innerHTML = '';

  const confirmDialog = document.createElement('div');
  confirmDialog.className = 'flex flex-col items-center justify-center gap-6';

  const message = document.createElement('div');
  message.className = 'text-2xl font-bold text-yellow-400 text-center';
  message.textContent = 'Is that everything?';
  confirmDialog.appendChild(message);

  const subMessage = document.createElement('div');
  subMessage.className = 'text-lg text-gray-300 text-center max-w-md';
  subMessage.textContent = 'Once you proceed, you cannot change your equipment choices.';
  confirmDialog.appendChild(subMessage);

  const note = document.createElement('div');
  note.className = 'text-sm text-gray-400 text-center max-w-md italic';
  note.textContent = 'Anything you leave, the shops can sell you later.';
  confirmDialog.appendChild(note);

  const buttonContainer = document.createElement('div');
  buttonContainer.className = 'flex flex-col gap-3 items-center';

  const confirmBtn = createContinueButton(0, 'Yes, Continue');
  buttonContainer.appendChild(confirmBtn);

  const backBtn = createContinueButton(0, 'Go Back'); // Using createContinueButton for styling consistency
  backBtn.style.background = '#6b7280';
  buttonContainer.appendChild(backBtn);

  confirmDialog.appendChild(buttonContainer);
  content.appendChild(confirmDialog);

  showPanel();

  const userChoice = await new Promise(resolve => {
    confirmBtn.onclick = () => resolve(true);
    backBtn.onclick = () => resolve(false);
  });

  await clearPanel();
  return userChoice;
}


/**
 * Show equipment choice scene.
 * @param {Object} choice - The current equipment choice object.
 * @param {number} choiceIndex - The index of the current choice.
 * @returns {Promise<boolean>} True if the user went back, false if they continued.
 */
async function showEquipmentChoiceScene(choice, choiceIndex) {
  const container = document.getElementById('gear-container');
  const background = document.getElementById('gear-background');
  const content = document.getElementById('gear-content');

  content.innerHTML = '';
  content.style.zIndex = '10';

  const title = document.createElement('div');
  title.className = 'text-xl md:text-2xl font-bold text-yellow-400 mb-4';
  title.textContent = `Choose Your Equipment (${choiceIndex + 1} of ${equipmentChoices.length})`;
  content.appendChild(title);

  const complexOptions = choice.options.filter(opt => opt.isComplexChoice);

  if (complexOptions.length > 0) {
    return await showMultiSlotChoiceSelection(content, choice, choiceIndex, complexOptions);
  } else {
    return await showRegularChoiceSelection(content, choice, choiceIndex);
  }
}

/**
 * Show regular (non-complex) choice selection.
 * @returns {Promise<boolean>} True if the user went back, false if they continued.
 */
async function showRegularChoiceSelection(content, choice, choiceIndex) {
  const container = document.getElementById('gear-container');

  const allItems = [];
  choice.options.forEach(option => {
    if (option.isBundle) {
      option.bundle.forEach(bundleItem => allItems.push(bundleItem[0]));
    } else if (!option.isComplexChoice) {
      allItems.push(option.item);
    }
  });
  await preloadItemStats(allItems);

  const scrollContainer = document.createElement('div');
  scrollContainer.className = 'w-full max-w-6xl mx-auto overflow-y-auto px-4 mb-4';
  scrollContainer.style.maxHeight = 'calc(100vh - 280px)';

  const optionsContainer = document.createElement('div');

  let selectedOption = null;
  let selectedOptionIndex = null;
  let userClickedBack = false;

  const bundles = choice.options.filter(opt => opt.isBundle);
  const simpleItems = choice.options.filter(opt => !opt.isBundle && !opt.isComplexChoice);

  // A choice is laid out in sections: the bundles side by side, then the
  // single items in a full-width grid that wraps into rows.
  optionsContainer.className = 'gear-choice';

  const createClickableOption = (option, optionIndex, containerElement) => {
    containerElement.dataset.optionIndex = optionIndex;
    containerElement.style.cursor = 'pointer';

    containerElement.onclick = (e) => {
      if (e.target.closest('.info-btn')) {
        return;
      }

      const allContainers = optionsContainer.querySelectorAll('[data-option-index]');
      allContainers.forEach(row => {
        row.classList.remove('selected');
      });

      containerElement.classList.add('selected');

      selectedOption = option;
      selectedOptionIndex = optionIndex;
      confirmBtn.disabled = false;
      confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
    };
  };

  if (bundles.length > 0) {
    const bundleRow = document.createElement('div');
    bundleRow.className = 'gear-bundles';
    for (let idx = 0; idx < bundles.length; idx++) {
      const option = bundles[idx];
      const optionIndex = choice.options.indexOf(option);
      if (idx > 0) bundleRow.appendChild(orMark());

      const bundleContainer = document.createElement('div');
      bundleContainer.className = 'gear-option gear-bundle';

      const bundleLabel = document.createElement('div');
      bundleLabel.className = 'gear-option-label';
      bundleLabel.textContent = '📦 Bundle';
      bundleContainer.appendChild(bundleLabel);

      const itemsRow = document.createElement('div');
      itemsRow.className = 'gear-cards';

      for (const bundleItem of option.bundle) {
        const itemName = bundleItem[0];
        const quantity = bundleItem[1];
        const itemCard = await createSimpleItemCard(itemName, quantity, true);

        itemCard.addEventListener('click', (e) => {
          if (!e.target.closest('.info-btn')) {
            e.stopPropagation();
            bundleContainer.click();
          }
        });
        itemsRow.appendChild(itemCard);
      }

      bundleContainer.appendChild(itemsRow);
      createClickableOption(option, optionIndex, bundleContainer);
      bundleRow.appendChild(bundleContainer);
    }
    optionsContainer.appendChild(bundleRow);

    if (simpleItems.length > 0) {
      const separator = document.createElement('div');
      separator.className = 'gear-or';
      separator.textContent = simpleItems.length > 1 ? 'or one of these' : 'or';
      optionsContainer.appendChild(separator);
    }
  }

  if (simpleItems.length > 0) {
    const gridContainer = document.createElement('div');
    gridContainer.className = 'gear-grid';

    for (let idx = 0; idx < simpleItems.length; idx++) {
      const option = simpleItems[idx];
      const optionIndex = choice.options.indexOf(option);
      const itemsRow = document.createElement('div');
      itemsRow.className = 'gear-option';

      const itemCard = await createSimpleItemCard(option.item, option.quantity, false);

      itemCard.addEventListener('click', (e) => {
        if (!e.target.closest('.info-btn')) {
          e.stopPropagation();
          itemsRow.click();
        }
      });

      itemsRow.appendChild(itemCard);

      createClickableOption(option, optionIndex, itemsRow);
      gridContainer.appendChild(itemsRow);
    }
    optionsContainer.appendChild(gridContainer);
  }

  scrollContainer.appendChild(optionsContainer);
  content.appendChild(scrollContainer);

  const confirmBtn = createContinueButton(0, 'Confirm Choice');
  confirmBtn.disabled = true;
  confirmBtn.classList.add('opacity-50', 'cursor-not-allowed');
  content.appendChild(confirmBtn);

  if (choiceIndex > 0) {
    const backBtn = createBackButton(() => {
      userClickedBack = true;
      confirmBtn.disabled = false;
      confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
      confirmBtn.click();
    });
    gearHost().appendChild(backBtn);
    currentBackButton = backBtn;
  }

  showPanel();

  await waitForButtonClick(confirmBtn);

  if (currentBackButton) {
    currentBackButton.remove();
    currentBackButton = null;
  }

  await clearPanel();
  if (userClickedBack) return true;

  selectedChoices[choiceIndex] = selectedOption;

  return false;
}

/**
 * Show multi-slot choice selection (two-step process).
 * @returns {Promise<boolean>} True if the user went back, false if they continued.
 */
async function showMultiSlotChoiceSelection(content, choice, choiceIndex, complexOptions) {
  logger.debug('🎮 showMultiSlotChoiceSelection called!');

  const container = document.getElementById('gear-container');

  const scrollContainer = document.createElement('div');
  scrollContainer.className = 'w-full max-w-6xl mx-auto overflow-y-auto px-4 mb-4';
  scrollContainer.style.maxHeight = 'calc(100vh - 280px)';

  const optionsContainer = document.createElement('div');
  optionsContainer.className = 'gear-bundles';

  let selectedConfiguration = null;
  let selectedConfigIndex = null;
  let userClickedBack = false;

  complexOptions.forEach((option, idx) => {
    const optionIndex = choice.options.indexOf(option);

    const configContainer = document.createElement('div');
    configContainer.className = 'gear-option gear-config';
    configContainer.dataset.optionIndex = optionIndex;

    const configTitle = document.createElement('div');
    configTitle.className = 'gear-option-label';
    configTitle.textContent = `Option ${idx + 1}`;
    configContainer.appendChild(configTitle);

    const slotsDesc = document.createElement('div');
    slotsDesc.className = 'gear-config-slots';

    if (!option.weaponSlots || option.weaponSlots.length === 0) {
      logger.error('❌ No weaponSlots found for complex option!');
      return;
    }

    option.weaponSlots.forEach((slot) => {
      const slotDiv = document.createElement('div');
      if (slot.type === 'weapon_choice') {
        slotDiv.innerHTML = `<span class="text-green-400">⚔️ Choose a weapon</span> from ${slot.options.length} options`;
      } else if (slot.type === 'fixed_item') {
        slotDiv.innerHTML = `<span class="text-blue-400">🛡️ ${getItemById(slot.item[0])?.name || slot.item[0]}</span> (included)`;
      }
      slotsDesc.appendChild(slotDiv);
    });

    configContainer.appendChild(slotsDesc);

    configContainer.onclick = () => {
      optionsContainer.querySelectorAll('[data-option-index]').forEach(row => {
        row.classList.remove('selected');
      });

      configContainer.classList.add('selected');
      selectedConfiguration = option;
      selectedConfigIndex = optionIndex;
      confirmBtn.disabled = false;
      confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
    };

    optionsContainer.appendChild(configContainer);

    if (idx < complexOptions.length - 1) optionsContainer.appendChild(orMark());
  });

  scrollContainer.appendChild(optionsContainer);
  content.appendChild(scrollContainer);

  const confirmBtn = createContinueButton(0, 'Confirm Configuration');
  confirmBtn.disabled = true;
  confirmBtn.classList.add('opacity-50', 'cursor-not-allowed');
  content.appendChild(confirmBtn);

  if (choiceIndex > 0) {
    const backBtn = createBackButton(() => {
      userClickedBack = true;
      confirmBtn.disabled = false;
      confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
      confirmBtn.click();
    });
    gearHost().appendChild(backBtn);
    currentBackButton = backBtn;
  }

  showPanel();

  await waitForButtonClick(confirmBtn);

  if (currentBackButton) {
    currentBackButton.remove();
    currentBackButton = null;
  }

  await clearPanel();
  if (userClickedBack) return true;

  const weaponSelections = {};
  let currentSlotIdx = 0;
  let weaponChoiceIndices = [];

  selectedConfiguration.weaponSlots.forEach((slot, idx) => {
    if (slot.type === 'weapon_choice') {
      weaponChoiceIndices.push(idx);
    }
  });

  let weaponChoicePosition = 0;

  while (weaponChoicePosition < weaponChoiceIndices.length) {
    currentSlotIdx = weaponChoiceIndices[weaponChoicePosition];
    const slot = selectedConfiguration.weaponSlots[currentSlotIdx];

    const result = await showWeaponSlotSelection(slot, weaponChoicePosition, weaponChoiceIndices.length, choiceIndex);

    if (result.shouldGoBack) {
      if (weaponChoicePosition > 0) {
        weaponChoicePosition--;
        const prevSlotIdx = weaponChoiceIndices[weaponChoicePosition];
        delete weaponSelections[prevSlotIdx];
      } else {
        return await showMultiSlotChoiceSelection(content, choice, choiceIndex, complexOptions);
      }
    } else {
      weaponSelections[currentSlotIdx] = result.selectedWeapon;
      weaponChoicePosition++;
    }
  }

  selectedConfiguration.weaponSlots.forEach((slot, idx) => {
    if (slot.type === 'fixed_item') {
      weaponSelections[idx] = slot.item;
    }
  });

  const finalSelection = {
    isComplexChoice: true,
    weapons: Object.values(weaponSelections)
  };

  selectedChoices[choiceIndex] = finalSelection;
  return false;
}

/**
 * Show weapon selection for a single slot.
 * @returns {Promise<Object>} Object with selectedWeapon and shouldGoBack.
 */
async function showWeaponSlotSelection(slot, slotIndex, totalSlots, choiceIndex) {
  const container = document.getElementById('gear-container');
  const content = document.getElementById('gear-content');

  content.innerHTML = '';

  const title = document.createElement('div');
  title.className = 'text-xl md:text-2xl font-bold text-yellow-400 mb-4';
  title.textContent = `Choose Weapon ${slotIndex + 1} of ${totalSlots}`;
  content.appendChild(title);

  await preloadItemStats(slot.options.map(w => w[0]));

  const scrollContainer = document.createElement('div');
  scrollContainer.className = 'w-full max-w-6xl mx-auto overflow-y-auto px-4 mb-4';
  scrollContainer.style.maxHeight = 'calc(100vh - 280px)';

  const gridContainer = document.createElement('div');
  gridContainer.className = 'gear-grid';

  let selectedWeapon = null;
  let userClickedBack = false;

  for (let idx = 0; idx < slot.options.length; idx++) {
    const weaponOption = slot.options[idx];
    const weaponName = weaponOption[0];
    const quantity = weaponOption[1];

    const itemContainer = document.createElement('div');
    itemContainer.className = 'gear-option';
    itemContainer.dataset.optionIndex = idx;

    const itemCard = await createSimpleItemCard(weaponName, quantity, false);

    itemCard.addEventListener('click', (e) => {
      if (!e.target.closest('.info-btn')) {
        e.stopPropagation();
        itemContainer.click();
      }
    });

    itemContainer.onclick = (e) => {
      if (e.target.closest('.info-btn')) return;

      gridContainer.querySelectorAll('[data-option-index]').forEach(row => {
        row.classList.remove('selected');
      });

      itemContainer.classList.add('selected');
      selectedWeapon = weaponOption;
      confirmBtn.disabled = false;
      confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
    };

    itemContainer.appendChild(itemCard);
    gridContainer.appendChild(itemContainer);
  }

  scrollContainer.appendChild(gridContainer);
  content.appendChild(scrollContainer);

  const confirmBtn = createContinueButton(0, 'Confirm Weapon');
  confirmBtn.disabled = true;
  confirmBtn.classList.add('opacity-50', 'cursor-not-allowed');
  content.appendChild(confirmBtn);

  const existingBackButtons = document.querySelectorAll('.pixel-back-btn');
  existingBackButtons.forEach((btn) => {
    btn.remove();
  });

  const backBtn = createBackButton(() => {
    userClickedBack = true;
    confirmBtn.disabled = false;
    confirmBtn.classList.remove('opacity-50', 'cursor-not-allowed');
    confirmBtn.click();
  });
  gearHost().appendChild(backBtn);
  currentBackButton = backBtn;

  showPanel();

  await waitForButtonClick(confirmBtn);

  if (currentBackButton) {
    currentBackButton.remove();
    currentBackButton = null;
  }

  await clearPanel();

  return {
    selectedWeapon: selectedWeapon,
    shouldGoBack: userClickedBack
  };
}

/**
 * Create a simple item card (for use in weapon slots and simple items)
 * @param {string} itemName - The ID of the item.
 * @param {number} quantity - The quantity of the item.
 * @param {boolean} isInBundle - True if the item is part of a bundle.
 * @returns {Promise<HTMLDivElement>} The item card element.
 */
async function createSimpleItemCard(itemName, quantity, isInBundle = false) {
  const itemData = getItemById(itemName);
  const card = document.createElement('div');
  card.className = 'item-card' + (isInBundle ? ' in-bundle' : '');

  const img = document.createElement('img');
  img.src = itemData?.image || `/res/img/items/${itemData?.id || itemName}.png`;
  img.alt = itemData?.name || itemName;
  img.onerror = function() {
    if (!this.dataset.fallbackAttempted) {
      this.dataset.fallbackAttempted = 'true';
      this.src = '/res/img/items/unknown.png';
    }
  };
  card.appendChild(img);

  if (quantity && quantity > 1) {
    const qtyText = document.createElement('div');
    qtyText.className = 'item-qty';
    qtyText.textContent = `×${quantity}`;
    card.appendChild(qtyText);
  }

  const name = document.createElement('div');
  name.className = 'item-name';
  name.dataset.itemId = itemName;
  name.textContent = itemData?.name || itemName;
  card.appendChild(name);

  const facts = itemFacts(itemData);
  if (!facts.length) card.classList.add('no-facts');
  if (facts.length) {
    const f = document.createElement('div');
    f.className = 'item-facts';
    f.innerHTML = facts.map((line) => `<div>${line}</div>`).join('');
    card.appendChild(f);
  }

  const infoBtn = document.createElement('button');
  infoBtn.className = 'info-btn';
  infoBtn.type = 'button';
  infoBtn.textContent = '?';
  infoBtn.title = 'Everything about it';
  infoBtn.onclick = (e) => {
    e.stopPropagation();
    const overlay = document.getElementById('gear-overlay');
    if (overlay) showItemDetails(itemName, overlay);
    else showItemModal(itemName);
  };
  card.appendChild(infoBtn);

  return card;
}

// The weapon properties worth knowing at a glance (5e's), in card order.
const CARD_TAGS = ['two-handed', 'versatile', 'finesse', 'light', 'thrown', 'reach', 'loading'];

/**
 * The facts printed on a gear card: damage dice and type for a weapon (with its
 * range when it shoots), AC for armor, and the key weapon properties.
 * @param {Object} item - static item data
 * @returns {string[]} short lines (HTML-safe: from our own data)
 */
function itemFacts(data) {
  if (!data) return [];
  // Most stats sit under `properties` in the game data.
  const item = { ...(data.properties || {}), ...data };
  let tags = item.tags || [];
  if (typeof tags === 'string') {
    try { tags = JSON.parse(tags); } catch (_) { tags = []; }
  }
  const lines = [];
  if (item.damage) {
    const [one, two] = String(item.damage).split(',');
    lines.push(`${one} ${item.damage_type || ''}`.trim());
    const props = CARD_TAGS.filter((t) => tags.includes(t))
      .map((t) => (t === 'versatile' && two ? `versatile ${two}` : t));
    if (item.range && (tags.includes('martial-ranged') || tags.includes('simple-ranged'))) {
      props.unshift(`range ${item.range}`);
    }
    if (props.length) lines.push(props.join(', '));
  } else if (item.set_bonus?.ac) {
    lines.push(`AC ${item.set_bonus.ac}`);
    const str = item.set_bonus.requirements?.strength;
    if (str) lines.push(`needs Str ${str}`);
  } else if (item.ac) {
    const ac = String(item.ac);
    lines.push(ac.startsWith('+') ? `${ac} AC` : `AC ${ac}`);
  }
  // A spellcasting focus supplies one component, endlessly: ∞ and its picture.
  if (item.provides && tags.includes('focus')) {
    const comp = getItemById(item.provides);
    const name = escapeHTML(comp?.name || String(item.provides).replace(/-/g, ' '));
    const src = comp?.image || `/res/img/items/${item.provides}.png`;
    lines.push(`<span class="item-provides" title="Supplies ${name} to any spell that needs it, at no cost"><span class="inf">∞</span><img src="${escapeHTML(src)}" alt="${name}"></span>`);
  }
  return lines;
}

function escapeHTML(s) {
  return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

/** The small "or" between options side by side. */
function orMark() {
  const or = document.createElement('div');
  or.className = 'gear-or-inline';
  or.textContent = 'or';
  return or;
}

/**
 * Show item info modal.
 * @param {string} itemName - The ID of the item to show info for.
 */
async function showItemModal(itemName) {
  hideItemModal();

  const backdrop = document.createElement('div');
  backdrop.id = 'item-modal-backdrop';
  backdrop.className = 'fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-50';
  backdrop.onclick = (e) => {
    if (e.target === backdrop) {
      hideItemModal();
    }
  };

  const modal = document.createElement('div');
  modal.className = 'bg-gray-800 border-2 border-yellow-400 rounded-lg p-6 max-w-md w-full mx-4 relative';
  modal.onclick = (e) => e.stopPropagation();

  const closeBtn = document.createElement('button');
  closeBtn.className = 'absolute top-2 right-2 w-8 h-8 bg-red-600 hover:bg-red-700 text-white rounded-full font-bold transition-colors';
  closeBtn.textContent = '✕';
  closeBtn.onclick = hideItemModal;
  modal.appendChild(closeBtn);

  modal.innerHTML += '<div class="text-center text-white">Loading...</div>';

  backdrop.appendChild(modal);
  gearHost().appendChild(backdrop);

  const statsHTML = await getItemStats(itemName);

  modal.innerHTML = '';
  modal.appendChild(closeBtn);

  const contentDiv = document.createElement('div');
  contentDiv.className = 'text-white';
  contentDiv.innerHTML = statsHTML;
  modal.appendChild(contentDiv);
}

/**
 * Hide item modal.
 */
function hideItemModal() {
  const backdrop = document.getElementById('item-modal-backdrop');
  if (backdrop) {
    backdrop.remove();
  }
}

/**
 * Preload item stats for multiple items into cache.
 * @param {Array<string>} itemNames - An array of item IDs to preload.
 */
async function preloadItemStats(itemNames) {
  // The cards read names and rarity from static data and "?" opens the game's
  // item panel, so the old stats HTML no longer needs fetching ahead.
  return itemNames;
}

/**
 * Fetch pack data from API to get contents.
 * @param {string} packName - The ID of the pack.
 * @returns {Promise<Object|null>} The pack data object, or null if not found/error.
 */
async function fetchPackData(packName) {
  try {
    logger.debug('📦 Fetching pack data for:', packName);
    const response = await fetch(`/api/items?name=${encodeURIComponent(packName)}`);
    if (!response.ok) {
      logger.warn('❌ Failed to fetch pack data for:', packName);
      return null;
    }

    const items = await response.json();

    if (!items || items.length === 0) {
      logger.warn('⚠️ No pack data found for:', packName);
      return null;
    }

    const packData = items[0];
    return packData;
  } catch (error) {
    logger.error('❌ Error fetching pack data:', packName, error);
    return null;
  }
}


// ============================================================================
// IN-GAME HELPERS (the inheritance, ui/startingGear.js)
// ============================================================================

/**
 * Show the items given with the class, before the choices (packs are shown on
 * the pack screen instead).
 * @param {Array<{item: string, quantity: number}>} givenItems
 */
export async function showGivenItems(givenItems) {
  const items = (givenItems || []).filter(item => !item.item.includes('-pack'));
  if (items.length === 0) return;

  const container = document.getElementById('gear-container');
  const content = document.getElementById('gear-content');
  content.innerHTML = '';

  const title = document.createElement('div');
  title.className = 'mb-4 text-xl font-bold text-yellow-400';
  title.textContent = 'Yours, along with the rest';
  content.appendChild(title);

  const row = document.createElement('div');
  row.className = 'gear-cards gear-given';
  for (const given of items) {
    row.appendChild(await createSimpleItemCard(given.item, given.quantity));
  }
  content.appendChild(row);

  const continueBtn = createContinueButton(0, 'Continue →');
  content.appendChild(continueBtn);

  showPanel();
  await waitForButtonClick(continueBtn);
  await clearPanel();
}

/**
 * Turn the picks into what claim_starting_gear takes: {"choice-N": item id,
 * or a JSON list of [item, quantity] for bundles and weapon picks}, plus the
 * pack.
 * @returns {{equipment_choices: Object<string,string>, pack_choice: string}}
 */
export function toChoicePayload(selected = selectedChoices) {
  const choices = {};
  for (const [key, option] of Object.entries(selected)) {
    if (key === 'pack') continue;
    const choiceKey = `choice-${key}`;
    if (option.isBundle && option.bundle?.length) {
      choices[choiceKey] = JSON.stringify(option.bundle);
    } else if (option.isComplexChoice && option.weapons) {
      choices[choiceKey] = JSON.stringify(option.weapons);
    } else if (option.item) {
      choices[choiceKey] = option.item;
    }
  }
  return { equipment_choices: choices, pack_choice: selected.pack || '' };
}
