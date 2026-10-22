import { defineConfig } from 'vite'
import { fileURLToPath } from 'node:url'

const at = path => fileURLToPath(new URL(path, import.meta.url))

export default defineConfig({
  define: {'process.env.NODE_ENV': JSON.stringify('production')},
  resolve: {
    alias: [{find: 'ali-oss', replacement: at('./node_modules/ali-oss')}],
  },
  build: {
    outDir: at('../internal/server'),
    emptyOutDir: false,
    lib: {
      entry: at('./src/evidence-transfer.ts'),
      name: 'OMSPhase1Evidence',
      formats: ['iife'],
      fileName: () => 'oms_phase1_evidence.js',
    },
    sourcemap: false,
  },
})
