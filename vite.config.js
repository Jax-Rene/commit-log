import { defineConfig } from 'vite';
import path from 'node:path';

export default defineConfig({
  base: '/static/dist/',
  build: {
    outDir: 'web/static/dist',
    emptyOutDir: true,
    manifest: false,
    rollupOptions: {
      input: {
        admin: path.resolve(__dirname, 'web/frontend/admin.js'),
        public: path.resolve(__dirname, 'web/frontend/public.js'),
      },
      output: {
        entryFileNames: 'assets/[name].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: ({ name }) => {
          if (!name) {
            return 'assets/[name][extname]';
          }
          const ext = path.extname(name);
          const baseName = path.basename(name, ext);
          return `assets/${baseName}${ext}`;
        },
      },
    },
  },
  css: {
    postcss: {
      plugins: [
        require('tailwindcss'),
        require('autoprefixer'),
      ],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'web/frontend'),
    },
  },
});
