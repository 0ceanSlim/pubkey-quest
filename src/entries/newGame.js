/**
 * New Game Entry Point
 *
 * New game character creation page bundle.
 * This replaces the individual script tags in new-game.html.
 */

import { logger } from '../lib/logger.js';
import '../lib/session.js'; // Auto-initializes as window.sessionManager
import { NostrCharacterGenerator } from '../logic/characterGenerator.js';
import { getItemById } from '../state/staticData.js';
import { getDisplayNamesForLocation } from '../data/characters.js';
import { createInventoryFromItems, addItemWithStacking } from '../data/inventory.js';
import * as newGame from '../pages/newGame.js';

// Make functions globally available
window.characterGenerator = new NostrCharacterGenerator();
window.getItemById = getItemById;
window.getDisplayNamesForLocation = getDisplayNamesForLocation;
window.createInventoryFromItems = createInventoryFromItems;
window.addItemWithStacking = addItemWithStacking;

// Export new game functions globally
window.showIntroduction = newGame.showIntroduction;
window.showEquipmentSelection = newGame.showEquipmentSelection;
window.startAdventure = newGame.startAdventure;
window.regenerateCharacter = newGame.regenerateCharacter;
window.goToSaves = newGame.goToSaves;
window.editCharacterName = newGame.editCharacterName;

logger.info('✨ New game bundle loaded');
