/**
 * Settings Page Entry Point
 *
 * Settings page bundle - imports and initializes settings systems.
 * This replaces the individual script tags in settings.html.
 */

// Core libraries
import { logger } from '../lib/logger.js';
import '../lib/session.js'; // Auto-initializes as window.sessionManager

// Systems
import '../systems/themeManager.js'; // Auto-initializes as window.themeManager
import '../systems/auth.js'; // Auto-initializes authentication
import '../systems/profileManager.js'; // Auto-initializes as window.profileManager
import * as nostrIdentity from '../systems/nostrIdentity.js';

// The settings page is a Go template, so its inline script reaches this through
// window rather than importing it. (themeManager and profileManager register
// themselves.)
window.nostrIdentity = nostrIdentity;

logger.info('⚙️ Settings page bundle loaded');
