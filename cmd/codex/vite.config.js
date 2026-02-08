import { defineConfig } from 'vite';
import { resolve } from 'path';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [tailwindcss()],

  root: '.',

  // Public directory for static assets (if needed)
  publicDir: false,

  // Build configuration
  build: {
    // Output to main www/dist/codex folder
    outDir: '../../www/dist/codex',
    emptyOutDir: true,
    sourcemap: true,
    cssCodeSplit: true,

    // Multiple entry points for each CODEX tool
    rollupOptions: {
      input: {
        'home': resolve(__dirname, 'src/home.js'),
        'item-editor': resolve(__dirname, 'src/item-editor.js'),
        'starting-gear-editor': resolve(__dirname, 'src/starting-gear-editor.js'),
        'systems-editor': resolve(__dirname, 'src/systems-editor.js'),
        'database-migration': resolve(__dirname, 'src/database-migration.js'),
        'validation': resolve(__dirname, 'src/validation.js'),
      },
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: 'chunks/[name]-[hash].js',
        assetFileNames: (assetInfo) => {
          if (assetInfo.name && assetInfo.name.endsWith('.css')) {
            return '[name].css';
          }
          return 'assets/[name]-[hash][extname]';
        }
      }
    },

    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: false,
      }
    }
  },

  // Development server
  server: {
    port: 5174,
    open: false,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },

  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  }
});
