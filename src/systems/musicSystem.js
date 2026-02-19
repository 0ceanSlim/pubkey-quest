/**
 * Music System Module
 *
 * Handles music playback with auto/manual modes and loop controls.
 * Tracks unlocked music and provides UI integration.
 *
 * @module systems/musicSystem
 */

import { logger } from '../lib/logger.js';
import { getGameStateSync } from '../state/gameState.js';

// Music system state
let currentAudio = null;
let currentTrack = null;
let playbackMode = 'auto'; // 'auto' or 'manual'
let loopEnabled = true;
let allTracks = []; // Loaded from music.json
let manuallyPaused = false; // Track if user manually paused
let currentVolume = 0.5; // Default volume

/**
 * Initialize the music system
 * @param {Array} tracks - Music tracks from game data API
 */
export function initMusicSystem(tracks = []) {
    // Store tracks
    allTracks = tracks;

    // Load playback preferences from localStorage
    const savedMode = localStorage.getItem('musicMode');
    const savedLoop = localStorage.getItem('musicLoop');
    const savedVolume = localStorage.getItem('musicVolume');

    if (savedMode) playbackMode = savedMode;
    if (savedLoop !== null) loopEnabled = savedLoop === 'true';
    if (savedVolume !== null) currentVolume = parseFloat(savedVolume);
}

/**
 * Get all music tracks with unlock status
 * @returns {Array} Array of tracks with unlocked status
 */
export function getAllTracks() {
    const state = getGameStateSync();
    const unlockedTracks = state.character?.music_tracks_unlocked || [];

    logger.debug('Getting all tracks:', {
        totalTracks: allTracks.length,
        unlockedTracks: unlockedTracks.length
    });

    return allTracks.map(track => ({
        ...track,
        unlocked: track.auto_unlock || unlockedTracks.includes(track.title)
    }));
}

/**
 * Get currently playing track info
 * @returns {Object|null} Current track info or null
 */
export function getCurrentTrack() {
    return currentTrack;
}

/**
 * Get playback mode
 * @returns {string} 'auto' or 'manual'
 */
export function getPlaybackMode() {
    return playbackMode;
}

/**
 * Get loop status
 * @returns {boolean} Whether loop is enabled
 */
export function isLoopEnabled() {
    return loopEnabled;
}

/**
 * Set playback mode
 * @param {string} mode - 'auto' or 'manual'
 */
export function setPlaybackMode(mode) {
    playbackMode = mode;
    localStorage.setItem('musicMode', mode);
    logger.debug('Playback mode set to:', mode);

    // Trigger mode change event
    document.dispatchEvent(new CustomEvent('musicModeChange', {
        detail: { mode, loop: loopEnabled }
    }));

    // If switching to auto, play location music
    if (mode === 'auto') {
        playLocationMusic();
    }
}

/**
 * Toggle loop on/off
 */
export function toggleLoop() {
    loopEnabled = !loopEnabled;
    localStorage.setItem('musicLoop', loopEnabled.toString());
    logger.debug('Loop toggled:', loopEnabled);

    // Update current audio if playing
    if (currentAudio) {
        currentAudio.loop = loopEnabled;
    }

    // Trigger loop change event
    document.dispatchEvent(new CustomEvent('musicLoopChange', {
        detail: { mode: playbackMode, loop: loopEnabled }
    }));
}

/**
 * Play a specific track
 * @param {Object} track - Track object from getAllTracks()
 * @param {boolean} forcePlay - Force play even if manually paused
 */
export function playTrack(track, forcePlay = false) {
    if (!track.unlocked) {
        logger.warn('Cannot play locked track:', track.title);
        return;
    }

    const isDifferentTrack = !currentTrack || currentTrack.title !== track.title;

    // Don't restart if same track is already playing and not paused
    if (currentTrack && currentTrack.title === track.title && currentAudio && !currentAudio.paused) {
        logger.debug('Track already playing:', track.title);
        return;
    }

    // If same track but paused, and not force playing, keep it paused
    if (currentTrack && currentTrack.title === track.title && currentAudio && currentAudio.paused && !forcePlay) {
        logger.debug('Track paused, not auto-resuming:', track.title);
        return;
    }

    // If manually paused and not forcing, only block if it's the SAME track
    // Allow new tracks to play even if manually paused
    if (manuallyPaused && !forcePlay && !isDifferentTrack) {
        logger.debug('Manually paused, not auto-playing same track');
        return;
    }

    // Stop current audio
    stopMusic();

    // Create new audio
    currentAudio = new Audio(track.file);
    currentAudio.loop = loopEnabled;
    currentAudio.volume = currentVolume;
    currentTrack = track;

    // Clear manual pause flag when starting a new track
    manuallyPaused = false;

    // Play
    currentAudio.play().catch(err => {
        logger.debug('Music autoplay prevented:', err);
    });

    logger.debug('Playing track:', track.title);

    // Trigger track change event
    document.dispatchEvent(new CustomEvent('musicTrackChange', {
        detail: { track, mode: playbackMode, loop: loopEnabled }
    }));
}

/**
 * Play the current location's music (auto mode)
 */
export function playLocationMusic() {
    const state = getGameStateSync();
    const currentLocation = state.location?.current;

    if (!currentLocation) {
        return;
    }

    // Only play in auto mode
    if (playbackMode !== 'auto') {
        return;
    }

    // Find the track that unlocks at this location
    const locationTrack = allTracks.find(t =>
        t.unlocks_at === currentLocation &&
        (t.auto_unlock || state.character?.music_tracks_unlocked?.includes(t.title))
    );

    if (locationTrack) {
        // Add unlocked property since we already verified it should be unlocked
        const unlockedTrack = { ...locationTrack, unlocked: true };
        playTrack(unlockedTrack);
    } else {
        // Play a default track if no location-specific track
        const defaultTrack = allTracks.find(t => t.auto_unlock);
        if (defaultTrack) {
            // Add unlocked property for auto-unlock tracks
            const unlockedTrack = { ...defaultTrack, unlocked: true };
            playTrack(unlockedTrack);
        }
    }
}

/**
 * Stop current music
 */
export function stopMusic() {
    if (currentAudio) {
        currentAudio.pause();
        currentAudio.currentTime = 0;
        currentAudio = null;
    }
    currentTrack = null;
}

/**
 * Pause current music
 */
export function pauseMusic() {
    if (currentAudio) {
        currentAudio.pause();
        manuallyPaused = true; // Mark as manually paused
    }
}

/**
 * Resume current music
 */
export function resumeMusic() {
    if (currentAudio) {
        currentAudio.play().catch(err => {
            logger.debug('Music play prevented:', err);
        });
        manuallyPaused = false; // Clear manual pause flag
    }
}

/**
 * Toggle play/pause
 */
export function togglePlayPause() {
    if (!currentAudio) {
        return;
    }

    if (currentAudio.paused) {
        resumeMusic();
    } else {
        pauseMusic();
    }

    // Trigger event to update UI
    document.dispatchEvent(new CustomEvent('musicPlayPauseToggle', {
        detail: { paused: currentAudio.paused }
    }));
}

/**
 * Set volume
 * @param {number} volume - Volume level (0.0 to 1.0)
 */
export function setVolume(volume) {
    currentVolume = Math.max(0, Math.min(1, volume));
    localStorage.setItem('musicVolume', currentVolume.toString());

    if (currentAudio) {
        currentAudio.volume = currentVolume;
    }

    // Trigger event to update UI
    document.dispatchEvent(new CustomEvent('musicVolumeChange', {
        detail: { volume: currentVolume }
    }));
}

/**
 * Get current volume
 * @returns {number} Current volume (0.0 to 1.0)
 */
export function getVolume() {
    return currentVolume;
}

/**
 * Check if a track should be unlocked at current location
 * and unlock it if needed
 */
export function checkAndUnlockLocationMusic() {
    const state = getGameStateSync();
    const currentLocation = state.location?.current;
    const unlockedTracks = state.character?.music_tracks_unlocked || [];

    if (!currentLocation) return;

    // Find track for this location
    const locationTrack = allTracks.find(t => t.unlocks_at === currentLocation);

    if (locationTrack && !locationTrack.auto_unlock && !unlockedTracks.includes(locationTrack.title)) {
        // Unlock the track
        state.character.music_tracks_unlocked = [...unlockedTracks, locationTrack.title];
        logger.info('Unlocked music track:', locationTrack.title);

        // Trigger unlock event
        document.dispatchEvent(new CustomEvent('musicUnlocked', {
            detail: { track: locationTrack }
        }));
    }
}

// Export for global access
window.musicSystem = {
    playTrack,
    playLocationMusic,
    checkAndUnlockLocationMusic,
    stopMusic,
    pauseMusic,
    resumeMusic,
    togglePlayPause,
    setPlaybackMode,
    toggleLoop,
    setVolume,
    getVolume,
    getPlaybackMode,
    isLoopEnabled,
    getCurrentTrack,
    getAllTracks
};

logger.debug('Music system module loaded');
