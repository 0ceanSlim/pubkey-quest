/**
 * Index/Home Page Entry Point
 *
 * Login page bundle - imports and initializes authentication systems.
 * This replaces the individual script tags in index.html.
 */

// Import global styles (only imported here, shared across all bundles)
import '../styles/main.css';

// Core libraries
import { logger } from '../lib/logger.js';
import '../lib/session.js'; // Auto-initializes as window.sessionManager

// Systems
import { themeManager } from '../systems/themeManager.js';
import { profileManager } from '../systems/profileManager.js';
import '../systems/auth.js'; // Auto-initializes authentication

// Pages
import '../pages/tabs.js'; // Tab navigation

logger.info('🏠 Index page bundle loaded');
