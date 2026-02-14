import { defineConfig } from 'vite';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import tailwindcss from '@tailwindcss/vite';

const __dirname = dirname(fileURLToPath(import.meta.url));
const rootDir = resolve(__dirname, '..');

// Suppress Vite CSS warnings for font URLs that resolve at runtime (served by Go)
const suppressFontWarnings = () => ({
  name: 'suppress-font-warnings',
  config() {
    const originalWarn = console.warn;
    console.warn = (...args) => {
      if (typeof args[0] === 'string' && args[0].includes("didn't resolve at build time")) return;
      originalWarn.apply(console, args);
    };
  }
});

export default defineConfig({
  plugins: [suppressFontWarnings(), tailwindcss()],

  // Root directory for source files (project root)
  root: rootDir,

  // Public directory disabled - Go server handles all static file serving
  publicDir: false,

  // Build configuration
  build: {
    // Output directory
    outDir: resolve(rootDir, 'www/dist'),

    // Don't empty outDir before build (preserve other files)
    emptyOutDir: false,

    // Generate source maps for debugging
    sourcemap: true,

    // Enable CSS code splitting for separate entry point CSS files
    cssCodeSplit: true,

    // Multiple entry points
    rollupOptions: {
      // Suppress warnings for modules that are both statically and dynamically imported
      onwarn(warning, warn) {
        if (warning.code === 'MODULE_LEVEL_DIRECTIVE') return;
        if (warning.message?.includes('is dynamically imported by') &&
            warning.message?.includes('but also statically imported by')) return;
        warn(warning);
      },
      input: {
        // JavaScript entries (first one will import CSS)
        index: resolve(rootDir, 'src/entries/index.js'),
        game: resolve(rootDir, 'src/entries/game.js'),
        gameIntro: resolve(rootDir, 'src/entries/gameIntro.js'),
        newGame: resolve(rootDir, 'src/entries/newGame.js'),
        settings: resolve(rootDir, 'src/entries/settings.js'),
        discover: resolve(rootDir, 'src/entries/discover.js'),
        saves: resolve(rootDir, 'src/entries/saves.js'),
      },
      output: {
        // Output file naming pattern
        entryFileNames: '[name].js',
        chunkFileNames: 'chunks/[name]-[hash].js',
        assetFileNames: (assetInfo) => {
          // Keep CSS filenames predictable (no hash) for easy linking in Go templates
          if (assetInfo.name && assetInfo.name.endsWith('.css')) {
            // Use the entry point name for CSS files (no hash)
            return '[name].css';
          }
          return 'assets/[name]-[hash][extname]';
        }
      }
    },

    // Minification
    minify: 'terser',
    terserOptions: {
      compress: {
        // Remove console.log in production
        drop_console: false, // Keep for now during testing
      }
    }
  },

  // Development server configuration
  server: {
    port: 5173,
    open: false,
    watch: {
      // Prevent infinite rebuild loops
      ignored: ['**/www/dist/**']
    },
    proxy: {
      // Proxy API requests to Go server during development
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },

  // Define global constants
  define: {
    __DEV__: JSON.stringify(process.env.NODE_ENV !== 'production'),
    __PROD__: JSON.stringify(process.env.NODE_ENV === 'production')
  },

  // Resolve configuration
  resolve: {
    alias: {
      '@': resolve(rootDir, 'src')
    }
  }
});
