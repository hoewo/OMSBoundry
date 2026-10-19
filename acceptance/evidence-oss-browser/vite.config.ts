import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'
import { defineConfig } from 'vite'

const currentDirectory = fileURLToPath(new URL('.', import.meta.url))
const repositoryRoot = resolve(currentDirectory, '../..')
const evidenceEnvFile = resolve(
  repositoryRoot,
  process.env.EVIDENCE_ENV_FILE ?? 'backend/services/evidence-service/.env.oss-validation',
)
const evidenceBaseUrl = process.env.EVIDENCE_BASE_URL ?? 'http://127.0.0.1:8085'
const templateAccessToken =
  process.env.TEMPLATE_ACCESS_TOKEN ?? readRequiredEnvValue(evidenceEnvFile, 'TEMPLATE_ACCESS_TOKEN')

export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/evidence': {
        target: evidenceBaseUrl,
        changeOrigin: false,
        rewrite: (path) => path.replace(/^\/evidence/, ''),
        configure(proxy) {
          proxy.on('proxyReq', (request) => {
            request.setHeader('Authorization', `Bearer ${templateAccessToken}`)
          })
        },
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})

function readRequiredEnvValue(path: string, key: string): string {
  let content: string
  try {
    content = readFileSync(path, 'utf8')
  } catch (error) {
    throw new Error(`Unable to read Evidence validation environment file at ${path}.`, { cause: error })
  }
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim()
    if (!line || line.startsWith('#')) continue
    const separator = line.indexOf('=')
    if (separator < 1 || line.slice(0, separator).trim() !== key) continue
    const value = stripMatchingQuotes(line.slice(separator + 1).trim())
    if (value && !value.startsWith('replace-with-')) return value
  }
  throw new Error(`${key} must be configured in ${path}.`)
}

function stripMatchingQuotes(value: string): string {
  if (value.length < 2) return value
  const first = value[0]
  const last = value[value.length - 1]
  return (first === '"' && last === '"') || (first === "'" && last === "'")
    ? value.slice(1, -1)
    : value
}
