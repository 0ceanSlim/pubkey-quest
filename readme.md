# ⚔️ Pubkey Quest

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/0ceanslim/pubkey-quest)

A web-based D&D-style RPG that generates your unique character from your Nostr identity and lets you adventure through a persistent world stored on Nostr relays.

## Overview

Pubkey Quest is a nostalgic RPG experience that derives a deterministic character from your Nostr public key. Your cryptographic identity becomes an adventurer with unique stats, equipment, and abilities. The game combines classic D&D 5e mechanics with Nostr's decentralized protocol to create a persistent, cross-client RPG experience.

## Current Development Status (Pre-Alpha)

The game is currently in **very early pre-alpha development**. Most systems are planned or in initial implementation.

### ✅ Implemented So Far

- **Deterministic Character Generation**:
  - Race, class, background, and alignment derived from Nostr pubkey
  - D&D 5e ability scores (STR, DEX, CON, INT, WIS, CHA)
  - Weighted distribution for realistic race/class combinations

- **Character Introduction System**:
  - Unique narrative introductions for each class/background combination
  - Contextual storytelling that reflects character origins
  - Atmospheric scene-setting with class-specific imagery

- **Starting Equipment System**:
  - Complex class-based equipment selection algorithm
  - Background-specific bonus equipment
  - Intelligent gear choices (e.g., spellcasters get component pouches, rangers get survival gear)
  - Deterministic selection ensures same character always gets same loadout

- **Starting Spell System**:
  - Class-specific spell lists for all spellcasting classes
  - Cantrip selection based on class mechanics
  - Level 1 spell loadout unique to each spellcaster
  - Spell slots properly allocated per class rules

- **Character Initialization**:
  - Starting HP calculated from class hit die + CON modifier
  - Starting gold based on class and background
  - All stats properly initialized and balanced

- **Class Resource System**:
  - Spellcasters use mana (derived from spellcasting ability + level)
  - Non-spellcasters have unique class resources (rage, ki, cunninge, stamina)
  - Class abilities that scale and grow more powerful with level

- **Game UI**:
  - Dark Win95-themed retro interface with beveled edges
  - Tabbed panels: Equipment, Inventory, Spells, Quests, Stats, Music
  - Character stat display (HP, resource, fatigue, encumbrance bars)
  - Equipment slot visualization with drag-and-drop
  - Backpack grid (20 slots) with item interactions
  - Spell/Ability slot interface
  - Mostly functional with minor edge case bugs

- **Content Database**:
  - 200+ items from D&D 5e SRD (weapons, armor, gear, tools)
  - Pixel art sprites (64x64) for items
  - Full D&D 5e spell database
  - Monster database
  - Location data
  - Item editor GUI tool for content creation

- **Save System (Basic)**:
  - Local JSON saves tied to npub
  - Save file generation from character creation
  - UI instantiation from save data

- **Authentication**:
  - Nostr login via NIP-07 browser extensions or Amber (Android)
  - Grain authentication client integration

- **Inventory System**:
  - Drag-and-drop between slots
  - Equipping/unequipping gear with validation
  - Right-click context menus (RuneScape-style)
  - Equipment slot system (10 slots)
  - Backpack grid (20 slots)
  - Item use actions (potions, food)

- **Location System**:
  - Multi-city world with districts and buildings
  - Location transitions and discovery
  - Race-based starting locations
  - Time-of-day system (day/night cycle)

- **Shop System**:
  - NPC shops with buy/sell functionality
  - Location-based shop inventories

- **Session Management**:
  - Server-side game state (Go-first architecture)
  - In-memory session with periodic saves
  - Backend-authoritative validation

- **Time & Tick System**:
  - In-game time progression (day/night cycle)
  - Tick-based game loop for scheduled events
  - Time-of-day affects NPC availability and events

- **Effects System**:
  - Extensible status effects (buffs, debuffs, conditions)
  - Duration tracking and automatic expiration
  - Stacking and interaction rules

- **Hunger & Fatigue**:
  - Hunger accumulation over time
  - Fatigue from actions and travel
  - Consequences for neglecting needs
  - Food consumption and rest mechanics

- **NPC System**:
  - NPCs with daily schedules
  - Location-based NPC availability
  - Time-aware interactions

### 🚧 Not Yet Implemented (Alpha Goals)

- **Exploration System**:
  - Monster encounters during travel
  - Random events while exploring
  - Points of Interest (POI) discovery
  - Linear and randomized dungeons
  - Static and random dungeon encounters
  - Discovered POIs can be revisited

- **Combat System**:
  - Turn-based combat encounters
  - D&D 5e dice rolling mechanics
  - Enemy AI and tactics
  - Loot drops and rewards

- **Active Spell Casting**:
  - Spell use in combat
  - Mana consumption and recovery
  - Spell effects and targeting

- **Quest System**:
  - Quest tracking and journal
  - Completion and rewards

### 📋 Development Roadmap

#### Current Phase: Pre-Alpha → Alpha

Focus: **Combat System**

- ✅ Inventory management
- ✅ Location/scene system
- ✅ Vault System
- ✅ Shop system
- ✅ Save/load functionality
- ✅ Time & tick system
- ✅ Effects system
- ✅ Hunger & fatigue
- ✅ NPC schedules
- 🚧 Exploration & POI discovery
- 🚧 Turn-based combat
- 🚧 Spell casting / Abilities in combat
- ⬚ Playtesting and balance

#### Alpha → Beta Goals

Focus: **Content & Nostr Integration**

- **Quest System**:
  - Handcrafted quest chains (all quests are manually designed)
  - Universal quests available to all players
  - Main story quests accessible regardless of character
  - Character-specific quests (race/class/background exclusive)
  - Exclusive quest rewards tailored to specific classes/roles
  - **Note**: No main story or critical content locked by character type

- **Full Nostr Integration**:
  - **Relay-based Saves**: Store save state as Nostr events (cross-client compatible)
  - **Save Validation**: Server validates saves against official list (modded vs unmodded tracking)
  - **Dungeon Master npub**: Official account for game announcements, player DMs, community engagement
  - **Nostr Badges**: Award achievements as NIP-58 badges
  - **Valid Saves List**: NIP-51 list event tracking legitimate save event IDs
  - **In-game Nostr Features**:
    - Write kind 1 notes using parchment and ink pen
    - Write long-form articles (kind 30023) using books
    - More creative integrations TBD

- **Cross-client Gameplay**:
  - Play on any Pubkey Quest client with the same character
  - Community-built clients and mods
  - Official validation against canonical ruleset

## Tech Stack

### Backend (Go) - Primary Logic Layer

- **Architecture**: Go-first design - ALL game logic lives in Go
- **Database**: SQLite for game data (migrated from JSON with CODEX)
- **Authentication**: Grain client for Nostr auth (NIP-07, Amber)
- **Session Management**: Server-side game state with in-memory sessions
- **API**: REST endpoints for game actions, data, and saves

### Frontend (Minimal JS)

- **Vanilla JavaScript**: DOM manipulation only (no game logic)
- **TailwindCSS**: Dark Win95-inspired retro theme
- **Go Templates**: Server-side HTML rendering
- **Philosophy**: Frontend cannot cheat - backend validates all actions

### Data

- **Source of Truth**: JSON files in `game-data/`
- **Runtime Cache**: SQLite database (built with CODEX)
- **Saves**: JSON files in `data/saves/{npub}/` (future: Nostr events)

## Game Data

All game content is stored as JSON in `game-data/`:

- **Items**: `game-data/items/` - 200+ individual item files (weapons, armor, gear, tools)
- **Spells**: `game-data/magic/spells/` - Full D&D 5e spell database
- **Monsters**: `game-data/monsters/` - Creature stat blocks
- **Locations**: `game-data/locations/` - Cities and environments
- **NPCs**: `game-data/npcs/` - NPC data organized by location
- **Systems**: `game-data/systems/` - Character generation tables, music config

## Contributing

Contributions are welcome! Here's how you can help:

1. **Report Bugs**: Use the [bug report template](https://github.com/0ceanSlim/pubkey-quest/issues/new?template=bug_report.md)
2. **Suggest Features**: Open an issue with your idea
3. **Add Content**: Create new items, spells, monsters, or locations (JSON format)
4. **Improve Code**: Submit PRs for bug fixes or enhancements
5. **Playtesting**: Try the game and provide feedback (when playable builds are available)

**Content Creators**: Check out `game-data/CODEX/` for the item editor and other content creation tools.

## Getting Started

See [Development Documentation](docs/development/) for setup instructions and development guides.

## License

This project is Open Source and licensed under the MIT License. See the [LICENSE](license) file for details.

---

Open Source and made with 💦 by [OceanSlim](https://njump.me/npub1zmc6qyqdfnllhnzzxr5wpepfpnzcf8q6m3jdveflmgruqvd3qa9sjv7f60)
