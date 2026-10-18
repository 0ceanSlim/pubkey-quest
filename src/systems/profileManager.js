/**
 * Profile Manager Module
 *
 * Fetches and manages Nostr profile metadata (kind 0 events).
 * Handles profile caching and relay communication.
 *
 * @module systems/profileManager
 */

import { logger } from '../lib/logger.js';

class ProfileManager {
  constructor() {
    this.profile = null;
    this.pubkey = null;
    this.npub = null;
    // The game's default relays (api.DefaultRelays on the server).
    this.relays = [
      'wss://offchain.pub',
      'wss://relay.ditto.pub',
      'wss://relay.nostr.net',
      'wss://nos.lol'
    ];
  }

  /**
   * Initialize profile manager with user's pubkey
   * @param {string} pubkey - Hex public key
   * @param {string} npub - Bech32 npub
   * @param {boolean} skipRelayFetch - Skip relay fetching (for new accounts)
   */
  async init(pubkey, npub, skipRelayFetch = false) {
    this.pubkey = pubkey;
    this.npub = npub;

    // Check cache first
    const cached = this.getCachedProfile(pubkey);
    if (cached) {
      this.profile = cached;
      this.dispatchProfileUpdate();
      return cached;
    }

    // Skip relay fetch for new accounts (they have no events yet)
    if (skipRelayFetch) {
      logger.info('Skipping relay fetch for new account');
      this.profile = {
        npub,
        pubkey,
        display_name: npub.slice(0, 10) + '...',
        name: null,
        picture: null,
        about: null
      };
      this.dispatchProfileUpdate();
      return this.profile;
    }

    // Fetch from relays
    try {
      const profile = await this.fetchProfile(pubkey);
      if (profile) {
        this.profile = { ...profile, npub, pubkey };
        this.cacheProfile(pubkey, this.profile);
        this.dispatchProfileUpdate();
        return this.profile;
      }
    } catch (error) {
      logger.error('Failed to fetch profile:', error);
    }

    // Return minimal profile if fetch fails
    this.profile = {
      npub,
      pubkey,
      display_name: npub.slice(0, 10) + '...',
      name: null,
      picture: null,
      about: null
    };
    this.dispatchProfileUpdate();
    return this.profile;
  }

  /**
   * Fetch profile from backend API (with caching)
   * @param {string} pubkey - Hex public key
   * @returns {Promise<Object>} Profile metadata
   */
  async fetchProfile(pubkey) {
    try {
      // Use backend API which has caching
      const response = await fetch(`/api/profile?npub=${this.npub}`);

      if (!response.ok) {
        logger.warn('Failed to fetch profile from backend:', response.status);
        return null;
      }

      const data = await response.json();

      if (data.profile) {
        logger.info(`${data.cached ? 'Profile loaded from cache' : 'Profile fetched from relays'}`);
        return {
          display_name: data.profile.display_name || data.profile.DisplayName,
          name: data.profile.name || data.profile.Name,
          picture: data.profile.picture || data.profile.Picture,
          about: data.profile.about || data.profile.About,
          nip05: data.profile.nip05 || data.profile.Nip05,
          lud16: data.profile.lud16 || data.profile.Lud16
        };
      }

      return null;
    } catch (error) {
      logger.error('Error fetching profile from backend:', error);
      return null;
    }
  }

  /**
   * Cache profile data in sessionStorage
   * @param {string} pubkey - Hex public key
   * @param {Object} profile - Profile data
   */
  cacheProfile(pubkey, profile) {
    try {
      sessionStorage.setItem(`profile_${pubkey}`, JSON.stringify({
        data: profile,
        timestamp: Date.now()
      }));
    } catch (error) {
      logger.error('Failed to cache profile:', error);
    }
  }

  /**
   * Get cached profile from sessionStorage
   * @param {string} pubkey - Hex public key
   * @returns {Object|null} Cached profile or null
   */
  getCachedProfile(pubkey) {
    try {
      const cached = sessionStorage.getItem(`profile_${pubkey}`);
      if (cached) {
        const { data, timestamp } = JSON.parse(cached);
        // Cache valid for 1 hour
        if (Date.now() - timestamp < 3600000) {
          return data;
        }
      }
    } catch (error) {
      logger.error('Failed to get cached profile:', error);
    }
    return null;
  }

  /**
   * Dispatch profile update event
   */
  dispatchProfileUpdate() {
    window.dispatchEvent(new CustomEvent('profile-updated', {
      detail: this.profile
    }));
  }

  /**
   * Clear profile data
   */
  clear() {
    this.profile = null;
    this.pubkey = null;
    this.npub = null;
  }

  /**
   * Get current profile
   * @returns {Object|null} Current profile data
   */
  getProfile() {
    return this.profile;
  }
}

// Create and export singleton instance
export const profileManager = new ProfileManager();

// Make available globally for template compatibility
if (typeof window !== 'undefined') {
  window.profileManager = profileManager;

  // Listen for session ready event to load profile
  window.addEventListener('sessionReady', async (event) => {
    if (event.detail && event.detail.npub && event.detail.pubkey) {
      const skipRelayFetch = event.detail.isNewAccount || false;
      await profileManager.init(event.detail.pubkey, event.detail.npub, skipRelayFetch);
      // Update login button after profile loads
      if (typeof window.updateLoginButton === 'function') {
        window.updateLoginButton();
      }
    }
  });

  // Listen for authentication success
  window.addEventListener('authenticationSuccess', async (event) => {
    if (event.detail && event.detail.npub && event.detail.pubkey) {
      const skipRelayFetch = event.detail.isNewAccount || false;
      await profileManager.init(event.detail.pubkey, event.detail.npub, skipRelayFetch);
      // Update login button after profile loads
      if (typeof window.updateLoginButton === 'function') {
        window.updateLoginButton();
      }
    }
  });

  // Clear profile on logout
  window.addEventListener('loggedOut', () => {
    profileManager.clear();
    // Update login button after logout
    if (typeof window.updateLoginButton === 'function') {
      window.updateLoginButton();
    }
  });
}

logger.debug('Profile manager module loaded');
