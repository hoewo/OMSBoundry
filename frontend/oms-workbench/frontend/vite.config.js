import { defineConfig } from 'vite'
import { fileURLToPath } from 'node:url'
const at = path => fileURLToPath(new URL(path, import.meta.url))
export default defineConfig({
  define: {'process.env.NODE_ENV': JSON.stringify('production')},
  esbuild: {jsx: 'automatic'},
  resolve: {alias: [{find: '@/components', replacement: at('./vendor/components')}, {find: '@/utils', replacement: at('./vendor/utils')}, {find: '@', replacement: at('./src')}, {find: 'ali-oss', replacement: at('./node_modules/ali-oss')}], dedupe: ['react', 'react-dom']},
  build: {
    outDir: at('../internal/server'), emptyOutDir: false,
    lib: {entry: at('./src/workspace.tsx'), name: 'ReferenceOAReactBidding', formats: ['iife'], fileName: () => 'reference_oa_react_bidding.js'},
    rollupOptions: {onwarn(warning,warn){if(warning.code!=='MODULE_LEVEL_DIRECTIVE')warn(warning)}, output: {assetFileNames: 'reference_oa_react_bidding.[ext]'}},
    sourcemap: false,
  },
})
