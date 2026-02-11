/**
 * Test Page - Verifies build system works
 */

import { logger } from '../lib/logger.js';
import { GAME_VERSION } from '../config/constants.js';

logger.info(`Pubkey Quest v${GAME_VERSION} - Build System Test`);
logger.debug('This debug message should only appear in development');
logger.warn('This warning should only appear in development');
logger.error('This error should appear in both dev and production');

// Test DOM manipulation
document.addEventListener('DOMContentLoaded', () => {
  const testDiv = document.createElement('div');
  testDiv.textContent = `Build system test successful! Version ${GAME_VERSION}`;
  testDiv.style.cssText = 'padding: 20px; background: #4CAF50; color: white; text-align: center; font-size: 24px;';
  document.body.prepend(testDiv);

  logger.info('DOM manipulation test successful');
});

// Test that global defines work
if (__DEV__) {
  logger.debug('Running in DEVELOPMENT mode');
  logger.debug('Debug logs are enabled');
}

if (__PROD__) {
  logger.info('Running in PRODUCTION mode (this shouldnt show in prod)');
}

logger.info('Build system version:', __VERSION__);
