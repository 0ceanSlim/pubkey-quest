/**
 * Game Intro Module
 *
 * The opening story: the rainy night, the caretaker, their letter, the road.
 * It's story only; a new character's gear is their inheritance, taken up in
 * game from their people's vault keeper (the tutorial quest, see
 * docs/draft/intro-and-tutorial.md). The letter's last line, and the arrival
 * screen, are the character's starting city's.
 *
 * @module pages/gameIntro
 */

import { logger } from '../lib/logger.js';
import { keepOnParchment } from '../lib/letterLayout.js';
import {
  createContinueButton,
  waitForButtonClick,
} from '../components/continueButton.js';

// Module state
// GLOBAL STATE
// ============================================================================
let generatedCharacter = null;
let introData = null;
let playerName = "";
let stopLetterFit = null;

// ============================================================================
// MUSIC PLAYLIST SYSTEM
// ============================================================================
let currentTrack = 0;
const tracks = ["intro-music", "game-music"];

function setupMusicPlaylist() {
  tracks.forEach((trackId, index) => {
    const audio = document.getElementById(trackId);
    audio.volume = 0.3;

    audio.addEventListener("ended", () => {
      // Move to next track
      currentTrack = (currentTrack + 1) % tracks.length;
      const nextAudio = document.getElementById(tracks[currentTrack]);
      nextAudio.currentTime = 0;
      nextAudio.play().catch((e) => logger.debug("Music autoplay blocked:", e));
    });
  });
}

function startMusic() {
  const firstTrack = document.getElementById(tracks[0]);
  firstTrack.play().catch((e) => logger.debug("Music autoplay blocked:", e));
}

// ============================================================================
// SCENE DISPLAY SYSTEM
// ============================================================================

/**
 * Display a scene with background image and text, with Continue button
 * @param {Object} config - Scene configuration
 * @param {string} config.text - Text to display
 * @param {string} config.image - Background image filename (optional)
 * @param {boolean} config.isQuote - Large centered quote style
 * @param {boolean} config.isLetter - Letter styling
 * @param {number} config.buttonDelay - Delay before showing Continue button (ms, default 7000)
 */
async function showScene(config) {
  const container = document.getElementById("scene-container");
  const background = document.getElementById("scene-background");
  const content = document.getElementById("scene-content");

  // Set up background
  if (config.image) {
    const src = config.image.startsWith('/') ? config.image : `/res/img/scene/${config.image}`;
    background.style.backgroundImage = `url(${src})`;
  } else {
    background.style.backgroundImage = "";
  }

  // Clear and set up content
  content.innerHTML = "";

  // Add skip button (top right)
  const skipBtn = document.createElement("button");
  skipBtn.className = "pixel-skip-btn";
  skipBtn.textContent = "Skip Scene";
  skipBtn.onclick = () => {
    continueBtn.click();
  };
  container.appendChild(skipBtn);

  if (config.isLetter) {
    // Letter scene - special styling positioned on background letter
    const letterDiv = document.createElement("div");
    letterDiv.className = "letter-container";
    letterDiv.innerHTML = `
      <div>${config.text}</div>
    `;
    content.appendChild(letterDiv);
    // On the page, at any window shape (the scene is cover-fitted).
    letterDiv.style.position = 'fixed';
    stopLetterFit = keepOnParchment(letterDiv);
  } else {
    // Regular scene
    const textElement = document.createElement("div");
    textElement.className = "scene-text";

    if (config.isQuote) {
      // Quote styling - large, centered, light purple with responsive sizing
      textElement.className +=
        " text-2xl md:text-3xl lg:text-4xl font-bold text-purple-500 leading-loose";
      textElement.style.textShadow =
        "3px 3px 6px rgba(0, 0, 0, 1), 0 0 12px rgba(0, 0, 0, 0.95), 1px 1px 3px rgba(0, 0, 0, 1)";
      textElement.style.lineHeight = "1.8";
      textElement.style.maxWidth = "90vw";
      textElement.style.maxHeight = "80vh";
      textElement.style.overflow = "visible";
      textElement.style.display = "flex";
      textElement.style.flexDirection = "column";
      textElement.style.alignItems = "center";
      textElement.style.justifyContent = "center";

      // Split text into sentences and animate each one
      const sentences = config.text.match(/[^.!?]+[.!?]+/g) || [config.text];
      textElement.innerHTML = ""; // Clear content

      // Calculate timing: divide buttonDelay by (number of sentences - 1) so last sentence appears when button shows
      const totalDuration = (config.buttonDelay || 7000) / 1000; // Convert ms to seconds
      const delayPerSentence =
        sentences.length > 1 ? totalDuration / (sentences.length - 1) : 0;

      sentences.forEach((sentence, index) => {
        const sentenceSpan = document.createElement("span");
        sentenceSpan.textContent = sentence.trim() + " ";
        sentenceSpan.style.opacity = "0";
        sentenceSpan.style.display = "block";
        sentenceSpan.style.marginBottom = "0.5em";
        sentenceSpan.style.animation = `fadeInSentence 0.8s ease-out forwards`;
        sentenceSpan.style.animationDelay = `${index * delayPerSentence}s`;
        textElement.appendChild(sentenceSpan);
      });
    } else {
      // Normal scene text, on a dark panel so it reads over the art
      textElement.className += " scene-story text-xl md:text-2xl leading-relaxed";
      textElement.style.textShadow =
        "3px 3px 6px rgba(0, 0, 0, 1), 0 0 12px rgba(0, 0, 0, 0.95), 1px 1px 3px rgba(0, 0, 0, 1)";
      textElement.textContent = config.text;
    }

    if (!config.isQuote) {
      textElement.textContent = config.text;
    }
    content.appendChild(textElement);
  }

  // Add Continue button using component
  const buttonDelay =
    config.buttonDelay !== undefined ? config.buttonDelay : 7000;
  const continueBtn = createContinueButton(buttonDelay);

  // For letter scenes, position button below the letter area
  if (config.isLetter) {
    continueBtn.style.cssText = `
      position: fixed !important;
      bottom: 10vh !important;
      left: 50% !important;
      right: auto !important;
      transform: translateX(-50%) !important;
      z-index: 100 !important;
      margin: 0 !important;
    `;
  }

  content.appendChild(continueBtn);

  // Show container with fade-in
  container.classList.remove("hidden", "fade-out");
  // First ensure we're not in fade-in state
  container.classList.remove("fade-in");
  // Force reflow to ensure classes are applied
  void container.offsetHeight;
  // Now add fade-in to trigger transition
  container.classList.add("fade-in");

  // Wait for user to click Continue
  await waitForButtonClick(continueBtn);

  // Remove skip button
  skipBtn.remove();
  stopLetterFit?.();
  stopLetterFit = null;

  // For letter scenes, just fade out without wipe animation
  if (config.isLetter) {
    // Fade out the letter container and button
    const textElements = content.querySelectorAll(
      ".letter-container, .pixel-continue-btn"
    );
    textElements.forEach((el) => {
      el.style.transition = "opacity 0.6s ease-out";
      el.style.opacity = "0";
    });

    // Wait for fade animation to complete
    await new Promise((resolve) => setTimeout(resolve, 600));
  } else {
    // Animate text out first (wipe down) for regular scenes
    const textElements = content.querySelectorAll(
      ".scene-text, .pixel-continue-btn"
    );
    textElements.forEach((el) => {
      el.style.animation = "wipeOut 0.6s ease-in forwards";
    });

    // Wait for text animation to complete
    await new Promise((resolve) => setTimeout(resolve, 600));
  }

  // Clear content
  content.innerHTML = "";

  // Then fade out the scene
  container.classList.remove("fade-in");
  container.classList.add("fade-out");
  await new Promise((resolve) => setTimeout(resolve, 800));

  // Fully reset container for next scene
  container.classList.remove("fade-in", "fade-out");
  container.classList.add("hidden");
}

/**
 * Show final scene with button
 * @param {string} text - Text to display
 * @param {string} buttonText - Button label
 * @param {Function} buttonAction - Function to call on click (NOT a string!)
 */
async function showFinalScene(text, buttonText, buttonAction) {
  const container = document.getElementById("scene-container");
  const background = document.getElementById("scene-background");
  const content = document.getElementById("scene-content");

  background.style.backgroundImage = "";
  background.style.backgroundColor = "#000000";

  // Create text element
  const textElement = document.createElement("div");
  textElement.className = "scene-text text-2xl md:text-4xl font-bold text-yellow-400 leading-relaxed mb-8";
  textElement.textContent = text;

  // Create button with delay using the continue button component
  const button = createContinueButton(7000, buttonText);
  button.onclick = buttonAction; // Direct function reference, no eval!

  content.innerHTML = "";
  content.appendChild(textElement);
  content.appendChild(button);

  container.classList.remove("hidden", "fade-out");
  // Force reflow to ensure hidden is removed before fade-in
  void container.offsetHeight;
  container.classList.add("fade-in");
}

// ============================================================================
// INTRO SEQUENCE
// ============================================================================

/**
 * The story, scene by scene; ends on the Begin Journey button.
 */
async function startIntroSequence() {
  playerName = document.getElementById("character-name").value.trim();

  if (!playerName) {
    alert("Please enter your name");
    return;
  }

  // Hide name screen
  document.getElementById("name-screen").classList.add("hidden");

  // Load intro data
  const response = await fetch("/api/introductions");
  introData = await response.json();
  const city = generatedCharacter.city || "kingdom";

  // 1. The rainy night
  await showScene({
    text: introData.scene1.text,
    image: introData.scene1.image,
    buttonDelay: introData.scene1.duration,
  });

  // 2. The caretaker
  await showScene({
    text: introData.scene2.text,
    image: introData.scene2.image,
    buttonDelay: introData.scene2.duration,
  });

  // 3. Their final words
  await showScene({
    text: introData.final_words.text,
    isQuote: true,
    buttonDelay: introData.final_words.duration,
  });

  // 4. How they raised you (by background)
  const bgIntro = introData.background_intros.find((entry) =>
    entry.backgrounds.includes(generatedCharacter.background)
  );
  if (bgIntro) {
    await showScene({
      text: bgIntro.text,
      image: bgIntro.image,
      buttonDelay: bgIntro.duration,
    });
  } else {
    logger.warn("⚠️ No background intro found for:", generatedCharacter.background);
  }

  // 5. The letter they left
  await showScene({
    text: introData.letter_intro.text,
    image: introData.letter_intro.image,
    buttonDelay: introData.letter_intro.duration,
  });

  // 6. The letter itself (by background), ending with who to find in your
  //    starting city: your people's vault keeper.
  const bgLetter = introData.background_letters.find((entry) =>
    entry.backgrounds.includes(generatedCharacter.background)
  );
  if (bgLetter) {
    const closing = introData.letter_closings?.[city] || "";
    await showScene({
      text: closing ? `${bgLetter.text}<br><br>${closing}` : bgLetter.text,
      image: bgLetter.image,
      isLetter: true,
      buttonDelay: bgLetter.duration,
    });
  } else {
    logger.warn("⚠️ No background letter found for:", generatedCharacter.background);
  }

  // 7. Departure
  await showScene({
    text: introData.departure.text,
    image: introData.departure.image,
    buttonDelay: introData.departure.duration,
  });

  // 8. Arrival in your starting city
  const arrival = introData.arrivals?.[city];
  if (arrival) {
    await showScene({
      text: arrival.text,
      image: arrival.image,
      buttonDelay: arrival.duration,
    });
  }

  // 9. Final text + Begin Journey
  await showFinalScene(
    introData.final_text.text,
    "Begin Journey",
    () => startAdventure() // Pass function, not string
  );
}

// ============================================================================
// SAVE AND START ADVENTURE
// ============================================================================

/**
 * Start the adventure - save character and redirect
 */
// Guards against the "Begin Journey" button being clicked more than once — each
// click used to POST /api/character/create-save and mint a brand-new save file,
// so an impatient double-click during the (~1 min) game build made duplicate saves.
let adventureStarting = false;

async function startAdventure() {
  if (adventureStarting) return; // one save, no matter how many clicks
  adventureStarting = true;
  showJourneyLoader();

  try {
    logger.debug("🎮 Starting adventure...");
    logger.debug("playerName:", playerName);

    // Get the final character name (either edited or generated)
    const finalName = playerName || "0";

    // Send to new Go endpoint
    const session = window.sessionManager.getSession();
    // Gear is chosen in game, from the vault keeper (the tutorial quest).
    const requestData = {
      npub: session.npub,
      name: finalName,
    };

    logger.debug("📤 Sending character creation request:", requestData);

    const response = await fetch("/api/character/create-save", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(requestData),
    });

    if (response.ok) {
      const result = await response.json();
      logger.debug("✅ Character created successfully:", result);

      // Play departure music and transition
      const introMusic = document.getElementById("intro-music");
      const gameMusic = document.getElementById("game-music");
      introMusic.pause();
      gameMusic.volume = 0.3;
      gameMusic
        .play()
        .catch((e) => logger.debug("Game music autoplay blocked:", e));

      // Fill the loading bar, then fade into the game.
      completeJourneyLoader();
      setTimeout(() => {
        window.location.href = "/game?save=" + result.save_id;
      }, 1400);
    } else {
      const error = await response.json();
      throw new Error(error.error || "Failed to create save file");
    }
  } catch (error) {
    logger.error("❌ Failed to start adventure:", error);
    adventureStarting = false; // let them retry after a failure
    hideJourneyLoader();
    alert("Failed to start adventure: " + error.message);
  }
}

// ── Begin-Journey loading overlay ───────────────────────────────────────────
// A full-screen bar + fade that covers the ~minute of game build, so the player
// gets feedback (and can't re-click) instead of a dead button that invites spam.

function showJourneyLoader() {
  let el = document.getElementById("journey-loader");
  if (!el) {
    el = document.createElement("div");
    el.id = "journey-loader";
    el.style.cssText =
      "position:fixed;inset:0;z-index:100;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:20px;background:#000;opacity:0;transition:opacity 0.6s ease;font-family:inherit;";

    const label = document.createElement("div");
    label.textContent = "Beginning your journey…";
    label.style.cssText =
      "color:#facc15;font-size:20px;font-weight:800;letter-spacing:0.04em;text-align:center;padding:0 16px;";

    const track = document.createElement("div");
    track.style.cssText =
      "width:min(70vw,360px);height:16px;background:#1a1a1a;overflow:hidden;border-top:2px solid #000;border-left:2px solid #000;border-right:2px solid #3a3a3a;border-bottom:2px solid #3a3a3a;";

    const fill = document.createElement("div");
    fill.id = "journey-loader-fill";
    fill.style.cssText = "width:4%;height:100%;background:#facc15;transition:width 2.8s ease-out;";

    track.appendChild(fill);
    el.appendChild(label);
    el.appendChild(track);
    document.body.appendChild(el);
  }
  requestAnimationFrame(() => {
    el.style.opacity = "1";
    const fill = document.getElementById("journey-loader-fill");
    if (fill) requestAnimationFrame(() => { fill.style.width = "88%"; });
  });
}

function completeJourneyLoader() {
  const fill = document.getElementById("journey-loader-fill");
  if (fill) {
    fill.style.transition = "width 0.4s ease-out";
    fill.style.width = "100%";
  }
}

function hideJourneyLoader() {
  const el = document.getElementById("journey-loader");
  if (el) {
    el.style.opacity = "0";
    setTimeout(() => el.remove(), 600);
  }
}

// ============================================================================
// INITIALIZATION
// ============================================================================

/**
 * Request fullscreen mode (mobile optimization)
 */
function requestFullscreen() {
  const elem = document.documentElement;

  if (elem.requestFullscreen) {
    elem.requestFullscreen().catch(err => {
      logger.debug("Fullscreen request failed:", err);
    });
  } else if (elem.webkitRequestFullscreen) { // Safari
    elem.webkitRequestFullscreen();
  } else if (elem.mozRequestFullScreen) { // Firefox
    elem.mozRequestFullScreen();
  } else if (elem.msRequestFullscreen) { // IE11
    elem.msRequestFullscreen();
  }
}

/**
 * Check if device is mobile
 */
function isMobileDevice() {
  return /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(navigator.userAgent) ||
         (window.innerWidth <= 768);
}

document.addEventListener("DOMContentLoaded", async function () {
  logger.debug("🎮 Initializing game intro...");

  if (!window.sessionManager) {
    logger.error("❌ Session manager not available");
    window.location.href = "/";
    return;
  }

  try {
    await window.sessionManager.init();

    if (!window.sessionManager.isAuthenticated()) {
      logger.debug("❌ Not authenticated, redirecting");
      window.location.href = "/";
      return;
    }

    const session = window.sessionManager.getSession();
    logger.debug("✅ Authenticated:", session.npub);

    // Initialize character generator
    logger.debug("🎲 Initializing character generator...");
    await window.characterGenerator.initialize();

    // Generate character
    logger.debug("🎲 Generating character...");
    generatedCharacter = await window.characterGenerator.generateCharacter(
      session.npub
    );
    logger.debug("✅ Character generated:", generatedCharacter);


    // Set up music playlist
    setupMusicPlaylist();

    // Start music when name input is interacted with
    const nameInput = document.getElementById("character-name");
    nameInput.addEventListener(
      "focus",
      () => {
        startMusic();
      },
      { once: true }
    );

    // Request fullscreen on mobile when user interacts with name input
    if (isMobileDevice()) {
      nameInput.addEventListener(
        "click",
        () => {
          requestFullscreen();
        },
        { once: true }
      );

      // Also try on Begin Your Story button
      const continueBtn = document.getElementById("continue-btn");
      continueBtn.addEventListener("click", () => {
        requestFullscreen();
      });
    }
  } catch (error) {
    logger.error("❌ Initialization failed:", error);
    alert("Failed to initialize: " + error.message);
  }
});
// Export module state and functions
export {
  startIntroSequence,
  startAdventure
};



logger.debug('Game intro module loaded');
