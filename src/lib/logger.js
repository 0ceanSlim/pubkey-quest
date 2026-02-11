/**
 * Smart Logging System
 *
 * Provides environment-aware logging:
 * - Development: Shows all logs (debug, info, warn, error)
 * - Production: Shows only errors
 *
 * Usage:
 *   import { logger } from '@lib/logger.js';
 *   logger.debug('Detailed info');
 *   logger.info('General info');
 *   logger.warn('Warning');
 *   logger.error('Error');
 */

const LogLevel = {
  DEBUG: 0,
  INFO: 1,
  WARN: 2,
  ERROR: 3,
  NONE: 4
};

class Logger {
  constructor() {
    // In production (__PROD__ is true), only show errors
    // In development (__DEV__ is true), show all logs
    this.level = __PROD__ ? LogLevel.ERROR : LogLevel.DEBUG;
    this.prefix = '[Pubkey Quest]';
  }

  /**
   * Debug logging - detailed information for development
   * Hidden in production
   */
  debug(...args) {
    if (this.level <= LogLevel.DEBUG) {
      console.log(`${this.prefix} 🐛`, ...args);
    }
  }

  /**
   * Info logging - general information
   * Hidden in production
   */
  info(...args) {
    if (this.level <= LogLevel.INFO) {
      console.log(`${this.prefix} ℹ️`, ...args);
    }
  }

  /**
   * Warning logging - potential issues
   * Hidden in production
   */
  warn(...args) {
    if (this.level <= LogLevel.WARN) {
      console.warn(`${this.prefix} ⚠️`, ...args);
    }
  }

  /**
   * Error logging - critical errors
   * Always shown (even in production)
   */
  error(...args) {
    if (this.level <= LogLevel.ERROR) {
      console.error(`${this.prefix} ❌`, ...args);
    }
  }

  /**
   * Create a collapsible log group
   * Hidden in production
   */
  group(label) {
    if (this.level <= LogLevel.DEBUG) {
      console.group(`${this.prefix} ${label}`);
    }
  }

  /**
   * End a log group
   * Hidden in production
   */
  groupEnd() {
    if (this.level <= LogLevel.DEBUG) {
      console.groupEnd();
    }
  }

  /**
   * Start a performance timer
   * Hidden in production
   */
  time(label) {
    if (this.level <= LogLevel.DEBUG) {
      console.time(`${this.prefix} ${label}`);
    }
  }

  /**
   * End a performance timer
   * Hidden in production
   */
  timeEnd(label) {
    if (this.level <= LogLevel.DEBUG) {
      console.timeEnd(`${this.prefix} ${label}`);
    }
  }

  /**
   * Set log level manually (useful for debugging)
   * @param {number} level - LogLevel constant
   */
  setLevel(level) {
    this.level = level;
  }
}

// Export singleton instance
export const logger = new Logger();

// Export LogLevel for manual level setting
export { LogLevel };
