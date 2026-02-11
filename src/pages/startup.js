/**
 * Pubkey Quest Startup Module
 *
 * Comprehensive initialization sequence for the application.
 * Follows a step-by-step initialization pattern with error handling.
 *
 * @module pages/startup
 */

import { logger } from '../lib/logger.js';
import { showActionText, showMessage } from '../ui/messaging.js';
import { initializeAuthentication } from '../systems/auth.js';

/**
 * Pubkey Quest Startup Class
 * Manages application initialization sequence
 */
class PubkeyQuestStartup {
    constructor() {
        this.initializationSteps = [
            { name: 'Session Manager', fn: this.initSessionManager },
            { name: 'Authentication', fn: this.initAuthentication },
            { name: 'Game Systems', fn: this.initGameSystems },
            { name: 'UI Components', fn: this.initUIComponents }
        ];
        this.currentStep = 0;
        this.isInitialized = false;
    }

    /**
     * Main initialization sequence
     */
    async initialize() {
        console.log('🎯 initialize() called!');
        console.log('🔍 logger object:', logger);
        console.log('🔍 initializationSteps:', this.initializationSteps);
        console.log('🔍 initializationSteps length:', this.initializationSteps.length);

        logger.info('Starting Pubkey Quest initialization sequence...');
        console.log('✅ logger.info completed');

        try {
            console.log('🔄 Entering try block, starting for loop...');
            for (let i = 0; i < this.initializationSteps.length; i++) {
                console.log(`🔢 Loop iteration ${i}`);
                const step = this.initializationSteps[i];
                console.log(`📝 Step ${i}:`, step);
                this.currentStep = i;

                console.log(`📢 About to log step ${i + 1}/${this.initializationSteps.length}: ${step.name}`);
                logger.info(`Step ${i + 1}/${this.initializationSteps.length}: ${step.name}`);
                console.log('📢 logger.info completed');

                console.log('🔄 About to call updateLoadingIndicator...');
                this.updateLoadingIndicator(step.name, (i + 1) / this.initializationSteps.length);
                console.log('✅ updateLoadingIndicator completed');

                console.log(`🚀 About to execute step function: ${step.name}`);
                await step.fn.call(this);
                console.log(`✅ Step ${i + 1} function completed: ${step.name}`);

                logger.info(`Step ${i + 1} complete: ${step.name}`);
            }

            console.log('🎉 All steps completed, exited for loop');
            console.log('🔧 Setting isInitialized to true...');
            this.isInitialized = true;
            console.log('📢 Calling logger.info...');
            logger.info('Pubkey Quest initialization complete!');
            console.log('🎊 About to call onInitializationComplete()...');
            this.onInitializationComplete();
            console.log('✅ onInitializationComplete() finished');

        } catch (error) {
            logger.error('Initialization failed at step:', this.initializationSteps[this.currentStep]?.name, error);
            this.onInitializationFailed(error);
        }
    }

    /**
     * Wait for session manager to be ready
     */
    async initSessionManager() {
        return new Promise((resolve, reject) => {
            const checkSessionManager = () => {
                if (window.sessionManager) {
                    logger.info('SessionManager ready');
                    resolve();
                } else {
                    logger.debug('Waiting for SessionManager...');
                    setTimeout(checkSessionManager, 50);
                }
            };
            checkSessionManager();

            // Timeout after 5 seconds
            setTimeout(() => {
                if (!window.sessionManager) {
                    reject(new Error('SessionManager failed to load within 5 seconds'));
                }
            }, 5000);
        });
    }

    /**
     * Initialize authentication system
     */
    async initAuthentication() {
        try {
            initializeAuthentication();
            logger.info('Authentication system initialized');
        } catch (error) {
            throw new Error('Authentication initialization failed: ' + error.message);
        }
    }

    /**
     * Verify game systems are ready
     */
    async initGameSystems() {
        // Game systems are initialized when the game starts after authentication
        // Just verify the functions are available
        const requiredGameFunctions = [
            'getGameState',
            'initializeGame'
        ];

        for (const funcName of requiredGameFunctions) {
            if (typeof window[funcName] !== 'function') {
                throw new Error(`Required game function ${funcName} not found`);
            }
        }

        logger.info('Game systems ready');
    }

    /**
     * Initialize UI components and event listeners
     */
    async initUIComponents() {
        // Check that required DOM elements exist
        const requiredElements = [
            'game-app'
        ];

        for (const elementId of requiredElements) {
            const element = document.getElementById(elementId);
            if (!element) {
                throw new Error(`Required DOM element #${elementId} not found`);
            }
        }

        // Initialize UI event listeners
        this.setupGlobalEventListeners();
        logger.info('UI components initialized');
    }

    /**
     * Setup global event listeners
     */
    setupGlobalEventListeners() {
        // Global error handler
        window.addEventListener('error', (event) => {
            logger.error('Global error:', event.error);
            showMessage('❌ An error occurred: ' + event.error.message, 'error');
        });

        // Unhandled promise rejection handler
        window.addEventListener('unhandledrejection', (event) => {
            logger.error('Unhandled promise rejection:', event.reason);
            showMessage('❌ An error occurred: ' + (event.reason?.message || event.reason), 'error');
        });

        // Session storage events
        window.addEventListener('storage', (event) => {
            if (event.key === 'pubkey_quest_session_meta') {
                logger.debug('Session storage changed, refreshing session');
                if (window.sessionManager) {
                    window.sessionManager.checkExistingSession();
                }
            }
        });

        // Visibility change handler for session monitoring
        document.addEventListener('visibilitychange', () => {
            if (!document.hidden && window.sessionManager) {
                // Check session when tab becomes visible
                window.sessionManager.checkExistingSession();
            }
        });

        // Before unload handler for cleanup
        window.addEventListener('beforeunload', () => {
            logger.debug('Cleaning up before page unload');
            // Any cleanup logic here
        });
    }

    /**
     * Update loading indicator (stub)
     */
    updateLoadingIndicator(stepName, progress) {
        // Don't show loading indicator - the game HTML is already rendered
        // This prevents the loading screen from overwriting the game UI
    }

    /**
     * Called when initialization completes successfully
     */
    onInitializationComplete() {
        console.log('🎊 onInitializationComplete() started');

        // Clear any loading indicators
        console.log('🧹 Calling hideLoadingIndicator()...');
        this.hideLoadingIndicator();
        console.log('✅ hideLoadingIndicator() done');

        // Emit initialization complete event
        console.log('📡 Dispatching pubkeyQuestReady event...');
        window.dispatchEvent(new CustomEvent('pubkeyQuestReady', {
            detail: { timestamp: Date.now() }
        }));
        console.log('✅ Event dispatched');

        console.log('📢 Calling logger.info...');
        logger.info('Pubkey Quest is ready to play!');
        console.log('✅ logger.info done');

        // Show welcome message in action text (purple)
        console.log('💬 Calling showActionText...');
        showActionText('Welcome to Pubkey Quest! Your adventure begins...', 'purple');
        console.log('✅ showActionText done');

        // Now initialize the actual game (load save data and render UI)
        if (typeof window.initializeGame === 'function') {
            window.initializeGame()
                .catch(error => {
                    logger.error('Failed to initialize game:', error);
                });
        } else {
            console.error('❌ initializeGame function not found on window!');
        }
    }

    /**
     * Show welcome popup modal
     */
    showWelcomePopup() {
        // Create modal backdrop
        const backdrop = document.createElement('div');
        backdrop.id = 'welcome-popup-backdrop';
        backdrop.className = 'fixed inset-0 bg-black bg-opacity-80 flex items-center justify-center z-[9999]';
        backdrop.style.fontFamily = '"Dogica", monospace';

        // Create modal content
        const modal = document.createElement('div');
        modal.className = 'bg-gray-800 rounded-lg p-8 max-w-2xl mx-4 relative';
        modal.style.border = '4px solid #ffd700';
        modal.style.boxShadow = '0 0 30px rgba(255, 215, 0, 0.5)';

        modal.innerHTML = `
            <h2 class="text-2xl font-bold text-yellow-400 mb-6 text-center">Welcome to Pubkey Quest!</h2>

            <div class="text-gray-300 space-y-4 text-sm leading-relaxed">
                <p class="text-center text-lg text-yellow-300">
                    I hope you enjoyed the intro!
                </p>

                <div class="bg-gray-900 border-2 border-yellow-600 rounded p-4 my-4">
                    <p class="text-yellow-200 font-bold mb-2">⚠️ Work in Progress</p>
                    <p>This is a work-in-progress game UI that only serves to pull data from your save and is <strong class="text-red-400">not interactable at the moment</strong>.</p>
                </div>

                <p>
                    Please <span class="text-green-400 font-bold">share your experience</span> with others and see the differences in your introductions!
                </p>

                <div class="bg-gray-900 border border-gray-600 rounded p-3 text-xs">
                    <p class="text-gray-400 mb-1">📍 The game UI is not functional except to travel different parts of the city</p>
                    <p class="text-gray-400">🏗️ NPC and building locations are just placeholders</p>
                </div>
            </div>

            <div class="mt-6 text-center">
                <button
                    id="welcome-close-btn"
                    class="px-8 py-3 bg-yellow-600 hover:bg-yellow-500 text-black font-bold rounded-lg transition-colors"
                    style="font-size: 1rem;">
                    Got it, let's explore!
                </button>
            </div>
        `;

        backdrop.appendChild(modal);
        document.body.appendChild(backdrop);

        // Close button handler
        document.getElementById('welcome-close-btn').onclick = () => {
            backdrop.remove();
        };

        // Close on backdrop click
        backdrop.onclick = (e) => {
            if (e.target === backdrop) {
                backdrop.remove();
            }
        };
    }

    /**
     * Called when initialization fails
     */
    onInitializationFailed(error) {
        const gameContainer = document.getElementById('game-app');
        if (gameContainer) {
            gameContainer.innerHTML = `
                <div class="flex items-center justify-center min-h-screen">
                    <div class="text-center max-w-md mx-auto p-6">
                        <div class="mb-6">
                            <h1 class="text-4xl font-bold text-red-400 mb-2">⚠️ Initialization Failed</h1>
                            <p class="text-gray-400 mb-4">Failed to start Pubkey Quest</p>
                        </div>

                        <div class="bg-red-900 bg-opacity-50 border border-red-600 rounded-lg p-4 mb-6">
                            <p class="text-red-200 text-sm">${error.message}</p>
                        </div>

                        <button onclick="window.location.reload()"
                                class="bg-yellow-600 hover:bg-yellow-700 text-gray-900 px-6 py-3 rounded-lg font-medium">
                            🔄 Retry
                        </button>

                        <div class="mt-6 text-xs text-gray-500">
                            <p>If this problem persists, please check:</p>
                            <ul class="mt-2 text-left">
                                <li>• JavaScript is enabled</li>
                                <li>• No browser extensions are blocking scripts</li>
                                <li>• Your internet connection is stable</li>
                            </ul>
                        </div>
                    </div>
                </div>
            `;
        }

        logger.error('Pubkey Quest initialization failed:', error);
    }

    /**
     * Hide loading indicator (stub)
     */
    hideLoadingIndicator() {
        // Loading indicator will be replaced by game interface or login interface
        // This is handled by the authentication system
    }

    /**
     * Check if initialization is complete
     * @returns {boolean} True if initialized
     */
    isReady() {
        return this.isInitialized;
    }

    /**
     * Get current initialization step
     * @returns {number} Current step index
     */
    getCurrentStep() {
        return this.currentStep;
    }

    /**
     * Get total number of initialization steps
     * @returns {number} Total steps
     */
    getTotalSteps() {
        return this.initializationSteps.length;
    }
}

// Create and export global startup instance
export const pubkeyQuestStartup = new PubkeyQuestStartup();

// Export public API functions
export function isPubkeyQuestReady() {
    return pubkeyQuestStartup.isReady();
}

// Auto-initialize when DOM is ready
if (typeof document !== 'undefined') {
    console.log('🔧 Startup.js: document exists, readyState =', document.readyState);

    // Check if DOM is already loaded
    if (document.readyState === 'loading') {
        // DOM hasn't loaded yet, wait for it
        console.log('⏳ Waiting for DOMContentLoaded...');
        document.addEventListener('DOMContentLoaded', function() {
            console.log('✅ DOMContentLoaded fired!');
            logger.info('DOM loaded, starting Pubkey Quest initialization...');
            pubkeyQuestStartup.initialize();
        });
    } else {
        // DOM is already loaded, initialize immediately
        console.log('✅ DOM already loaded, initializing immediately...');
        logger.info('DOM already loaded, starting Pubkey Quest initialization immediately...');
        pubkeyQuestStartup.initialize();
    }
} else {
    console.error('❌ document is undefined!');
}

// Make startup instance available globally for compatibility
if (typeof window !== 'undefined') {
    window.pubkeyQuestStartup = pubkeyQuestStartup;
    window.isPubkeyQuestReady = isPubkeyQuestReady;
}

logger.debug('Pubkey Quest startup system loaded');
