import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
import solidPlugin from 'vite-plugin-solid';

export default defineConfig({
  plugins: [tailwindcss(), solidPlugin()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/webhooks': { target: 'http://localhost:8080', changeOrigin: true },
      '/swagger': { target: 'http://localhost:8080', changeOrigin: true },
      '/sse': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
  build: { target: 'esnext', outDir: 'dist' },
});
