import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    host: 'localhost',
    port: 5176,
    proxy: {
      '/api': {
        target: 'http://localhost:8094',
        changeOrigin: true
      }
    }
  }
});
