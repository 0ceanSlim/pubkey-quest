/**
 * Game Intro Entry Point
 *
 * Intro sequence page bundle - imports intro cutscene system.
 * This replaces the individual script tags in game-intro.html.
 */

import { logger } from '../lib/logger.js';
import '../lib/session.js'; // Auto-initializes as window.sessionManager
import { NostrCharacterGenerator } from '../logic/characterGenerator.js';
import { getItemById } from '../state/staticData.js';
import { generateStartingVault, getDisplayNamesForLocation } from '../data/characters.js';
import * as gameIntro from '../pages/gameIntro.js';

// Make functions globally available
window.characterGenerator = new NostrCharacterGenerator();
window.getItemById = getItemById;
window.generateStartingVault = generateStartingVault;
window.getDisplayNamesForLocation = getDisplayNamesForLocation;

// Export intro functions globally
window.startIntroSequence = gameIntro.startIntroSequence;
window.startAdventure = gameIntro.startAdventure;

logger.info('🎬 Game intro bundle loaded');
