/**
 * Tab System Module
 *
 * Handles tab switching and URL hash routing for the homepage.
 * Manages tab navigation with hash-based routing.
 *
 * @module pages/tabs
 */

import { logger } from '../lib/logger.js';

/**
 * Tab Manager Class
 * Manages tab switching and URL hash routing
 */
class TabManager {
    constructor() {
        this.tabs = ['home', 'updates', 'roadmap', 'about', 'contribute', 'wiki'];
        this.currentTab = 'home';
        this.init();
    }

    /**
     * Initialize tab manager
     */
    init() {
        // Load tab from URL hash if present. "#updates/<id>" opens a tab and
        // scrolls to one entry in it (the in-game "what's new" links).
        const [hash, anchor] = window.location.hash.slice(1).split('/');
        if (hash && this.tabs.includes(hash)) {
            this.currentTab = hash;
        }

        // Show initial tab
        this.showTab(this.currentTab);
        this.scrollToEntry(anchor);

        // Listen for hash changes
        window.addEventListener('hashchange', () => {
            const [hash, anchor] = window.location.hash.slice(1).split('/');
            if (hash && this.tabs.includes(hash)) {
                this.showTab(hash);
                this.scrollToEntry(anchor);
            }
        });

        // Add click handlers to tab buttons
        this.tabs.forEach(tabName => {
            const button = document.getElementById(`tab-btn-${tabName}`);
            if (button) {
                button.addEventListener('click', () => {
                    this.switchTab(tabName);
                });
            }
        });
    }

    /**
     * Scroll to an entry inside the open tab ("#updates/<id>" → #update-<id>).
     * @param {string} [id]
     */
    scrollToEntry(id) {
        if (!id) return;
        const el = document.getElementById(`update-${id}`);
        if (el) setTimeout(() => el.scrollIntoView({ behavior: 'smooth', block: 'start' }), 50);
    }

    /**
     * Switch to a specific tab
     * @param {string} tabName - Name of tab to show
     */
    switchTab(tabName) {
        if (!this.tabs.includes(tabName)) {
            logger.error(`Tab "${tabName}" not found`);
            return;
        }

        // Update URL hash
        window.location.hash = tabName;

        // Show tab
        this.showTab(tabName);
    }

    /**
     * Show a specific tab
     * @param {string} tabName - Name of tab to show
     */
    showTab(tabName) {
        this.currentTab = tabName;

        // Hide all tab content
        this.tabs.forEach(tab => {
            const content = document.getElementById(`tab-${tab}`);
            const button = document.getElementById(`tab-btn-${tab}`);

            if (content) {
                content.classList.add('hidden');
            }

            if (button) {
                button.classList.remove('tab-active');
                button.classList.add('tab-inactive');
            }
        });

        // Show active tab content
        const activeContent = document.getElementById(`tab-${tabName}`);
        const activeButton = document.getElementById(`tab-btn-${tabName}`);

        if (activeContent) {
            activeContent.classList.remove('hidden');
        }

        if (activeButton) {
            activeButton.classList.remove('tab-inactive');
            activeButton.classList.add('tab-active');
        }

        // Dispatch tab change event
        window.dispatchEvent(new CustomEvent('tab-changed', {
            detail: { tab: tabName }
        }));

        // Scroll to top of page
        window.scrollTo({ top: 0, behavior: 'smooth' });
    }

    /**
     * Get current active tab
     * @returns {string} Current tab name
     */
    getCurrentTab() {
        return this.currentTab;
    }
}

// Create and export tab manager instance
export const tabManager = new TabManager();

// Initialize tab manager when DOM is ready
if (typeof window !== 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            // Already initialized in constructor
        });
    }

    // Make globally available for compatibility
    window.tabManager = tabManager;
}

logger.debug('Tab system loaded');
