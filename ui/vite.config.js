import { defineConfig } from 'vite'

// The build lands inside the Go package so `//go:embed ui_assets/dist` can bake
// it into the binary. Paths are relative because the Go file server mounts the
// bundle at the server root and nothing guarantees a leading slash resolves.
export default defineConfig({
  base: './',
  build: {
    outDir: '../command/ui_assets/dist',
    emptyOutDir: true,
    // The UI is small and served from memory; one request beats three.
    assetsInlineLimit: 0,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
      },
    },
  },
  server: {
    // `pnpm dev` talks to a `tomato ui` already running on its own port.
    proxy: {
      '/api': 'http://localhost:7788',
      '/ws': { target: 'ws://localhost:7788', ws: true },
    },
  },
})
