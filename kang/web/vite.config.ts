import path from 'path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  build: {
    outDir: '../server/cmd/server/dist',
    emptyOutDir: true,
  },
  server: {
    port: 9872,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:9871',
        changeOrigin: true,
      },
    },
  },
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '.'),
    },
  },
});
