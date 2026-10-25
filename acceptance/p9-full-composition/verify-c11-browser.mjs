#!/usr/bin/env node

import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { chmodSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, rmdirSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const required = (name) => {
  const value = String(process.env[name] || '').trim()
  if (!value) throw new Error(`set ${name}`)
  return value
}
const sha256 = (value) => createHash('sha256').update(value).digest('hex')
const fileSha256 = (path) => sha256(readFileSync(path))
const jsonFile = (path) => JSON.parse(readFileSync(path, 'utf8'))
const allowedURL = (value) => {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' || (parsed.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(parsed.hostname))
  } catch { return false }
}
const ensurePasswordFile = (path) => {
  const stat = statSync(path)
  if (!stat.isFile() || (stat.mode & 0o777) !== 0o600) throw new Error(`protected input must be a regular file with mode 0600: ${path}`)
}
const runVerifier = (name, args) => {
  try { execFileSync(join(scriptDir, name), args, { stdio: ['ignore', 'ignore', 'pipe'] }) }
  catch (error) { throw new Error(`${name} failed: ${String(error?.stderr || '').trim() || 'unknown verifier error'}`) }
}

const root = resolve(required('P9_RUN_EVIDENCE_ROOT'))
const evidenceDir = resolve(required('P9_C11_EVIDENCE_DIR'))
const primaryUsername = required('P9_PRIMARY_ADMIN_USERNAME')
const primaryPasswordFile = resolve(required('P9_PRIMARY_ADMIN_PASSWORD_FILE'))
const secondaryUsername = required('P9_SECONDARY_ADMIN_USERNAME')
const secondaryPasswordFile = resolve(required('P9_SECONDARY_ADMIN_PASSWORD_FILE'))
if (!process.env.P9_RUN_EVIDENCE_ROOT.startsWith('/') || !process.env.P9_C11_EVIDENCE_DIR.startsWith('/')) throw new Error('P9 evidence paths must be absolute')
if (existsSync(evidenceDir) && (!statSync(evidenceDir).isDirectory() || readdirSync(evidenceDir).length)) throw new Error(`P9 C11 evidence directory must be absent or empty: ${evidenceDir}`)
ensurePasswordFile(primaryPasswordFile)
ensurePasswordFile(secondaryPasswordFile)
runVerifier('verify-run-c0-c10', [root])

const playwrightModule = process.env.PLAYWRIGHT_MODULE_PATH || 'playwright'
let chromium
try { ({ chromium } = await import(playwrightModule)) }
catch { throw new Error('Playwright is required; set PLAYWRIGHT_MODULE_PATH or install playwright') }

const c0 = jsonFile(join(root, 'c0/state-after-c0.json'))
const c1 = jsonFile(join(root, 'c1/state-after-c1.json'))
const c9 = jsonFile(join(root, 'c9/state-after-c9.json'))
const c10 = jsonFile(join(root, 'c10/state-after-c10.json'))
const baseURL = String(c0.deployment?.public_url || '').replace(/\/+$/, '')
if (!allowedURL(baseURL)) throw new Error('C0 public URL is not allowed')
const workbenchURL = `${baseURL}/workbench/?view=oms-v1`
const actors = [
  { label: 'primary', username: primaryUsername, passwordFile: primaryPasswordFile, tenant: c1.tenants[0] },
  { label: 'secondary', username: secondaryUsername, passwordFile: secondaryPasswordFile, tenant: c1.tenants[1] },
]
if (actors.some((actor) => !actor.tenant?.id || !actor.tenant?.administrator_subject_id)) throw new Error('C1 tenant administrators are incomplete')

const runtime = mkdtempSync(join(tmpdir(), 'oms-p9-c11.'))
chmodSync(runtime, 0o700)
const rows = [['actor', 'check', 'result']]
const tenantResults = []
let browser

const requestJSON = async (request, url, options, expected, label) => {
  const response = await request.fetch(url, { ...options, failOnStatusCode: false })
  rows.push([label.split(' ', 1)[0], label.slice(label.indexOf(' ') + 1), String(response.status())])
  if (!expected.includes(response.status())) throw new Error(`${label} returned HTTP ${response.status()}, expected ${expected.join('/')}`)
  if (response.status() === 204) return {}
  try { return await response.json() } catch { throw new Error(`${label} returned non-JSON HTTP ${response.status()}`) }
}

const authenticate = async (context, actor) => {
  const csrf = await requestJSON(context.request, `${baseURL}/auth/api/v1/csrf`, { method: 'GET' }, [200], `${actor.label} csrf`)
  const token = String(csrf.csrf_token || '')
  if (!token) throw new Error(`${actor.label} CSRF token missing`)
  await requestJSON(context.request, `${baseURL}/auth/api/v1/login`, {
    method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': token },
    data: { username: actor.username, password: readFileSync(actor.passwordFile, 'utf8').replace(/[\r\n]+$/, '') },
  }, [200], `${actor.label} login`)
  const session = await requestJSON(context.request, `${baseURL}/auth/api/v1/session`, { method: 'GET' }, [200], `${actor.label} session`)
  assert.equal(session.principal?.tenant_id, actor.tenant.id, `${actor.label} session Tenant differs from C1`)
  assert.equal(session.principal?.subject_id, actor.tenant.administrator_subject_id, `${actor.label} session Subject differs from C1`)
  return token
}

const changeGrant = async (context, actor, operation) => {
  const csrf = await requestJSON(context.request, `${baseURL}/auth/api/v1/csrf`, { method: 'GET' }, [200], `${actor.label} ${operation}-csrf`)
  await requestJSON(context.request, `${baseURL}/auth/api/v1/admin/grants${operation === 'revoke' ? '/revoke' : ''}`, {
    method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf.csrf_token },
    data: { subject_id: actor.tenant.administrator_subject_id, audience: 'feedback', action: 'feedback.use' },
  }, [204], `${actor.label} ${operation}-feedback`)
}

const openMode = async (page, mode, expectedOrigin) => {
  await page.locator('[data-feedback-toggle]').click()
  await page.locator(`[data-feedback-mode="${mode}"]`).first().click()
  const layer = page.locator('[data-feedback-layer]')
  await layer.waitFor({ state: 'visible', timeout: 15000 })
  const frame = layer.locator('iframe')
  await frame.waitFor({ state: 'attached' })
  await page.waitForFunction(() => {
    const value = document.querySelector('[data-feedback-layer] iframe')?.src || ''
    return value && value !== 'about:blank'
  })
  const frameURL = await frame.getAttribute('src')
  assert(frameURL && allowedURL(frameURL), `${mode} iframe URL is invalid`)
  assert.equal(new URL(frameURL).origin, expectedOrigin, `${mode} iframe origin differs from configured SDK`)
  assert.notEqual(new URL(frameURL).origin, new URL(baseURL).origin, `${mode} iframe must be cross-origin`)
  await page.evaluate(() => { document.querySelector('[data-feedback-layer]').hidden = true })
  return new URL(frameURL).origin
}

try {
  const launchOptions = { headless: true }
  if (process.env.BROWSER_EXECUTABLE) launchOptions.executablePath = process.env.BROWSER_EXECUTABLE
  browser = await chromium.launch(launchOptions)
  for (const actor of actors) {
    const context = await browser.newContext()
    let revoked = false
    try {
      await authenticate(context, actor)
      const configEnvelope = await requestJSON(context.request, `${baseURL}/gateway/feedback/config`, { method: 'POST', headers: { 'X-CSRF-Token': (await requestJSON(context.request, `${baseURL}/auth/api/v1/csrf`, { method: 'GET' }, [200], `${actor.label} config-csrf`)).csrf_token } }, [200], `${actor.label} config`)
      const gatewayURL = String(configEnvelope.config?.gatewayUrl || '')
      if (!allowedURL(gatewayURL) || configEnvelope.config?.projectId !== 115) throw new Error(`${actor.label} Feedback runtime config is invalid`)
      const expectedOrigin = new URL(gatewayURL).origin
      const page = await context.newPage()
      const consoleProblems = []
      const pageErrors = []
      let intentionalDenialWindow = false
      page.on('console', (message) => { if (message.type() === 'error' || message.type() === 'warning') consoleProblems.push({ type: message.type(), text: message.text(), intentionalDenialWindow }) })
      page.on('pageerror', (error) => pageErrors.push(error.name || 'Error'))
      await page.goto(workbenchURL, { waitUntil: 'domcontentloaded' })
      const rootNode = page.locator('[data-oms-v1]')
      await rootNode.waitFor({ state: 'visible' })
      assert.equal(await rootNode.getAttribute('data-tenant-id'), actor.tenant.id)
      assert.equal(await rootNode.getAttribute('data-subject-id'), actor.tenant.administrator_subject_id)
      await page.locator('[data-feedback-entry]').waitFor({ state: 'visible', timeout: 15000 })
      const submitOrigin = await openMode(page, 'submit', expectedOrigin)
      const statusOrigin = await openMode(page, 'status', expectedOrigin)

      if (actor.label === 'primary') {
        await changeGrant(context, actor, 'revoke')
        revoked = true
        intentionalDenialWindow = true
        const denied = page.waitForResponse((response) => response.url() === `${baseURL}/gateway/feedback/config` && response.status() === 403, { timeout: 15000 })
        await page.reload({ waitUntil: 'domcontentloaded' })
        await denied
        await page.waitForTimeout(100)
        intentionalDenialWindow = false
        await page.locator('[data-oms-v1]').waitFor({ state: 'visible' })
        assert.equal(await page.locator('[data-feedback-entry]').isHidden(), true, 'revoked Feedback entry must remain hidden')
        await page.locator('[data-oms-v1-nav="internal"]').click()
        await page.locator('[data-oms-v1-page="internal"].is-active').waitFor({ state: 'visible' })
        await changeGrant(context, actor, 'grant')
        revoked = false
        await page.reload({ waitUntil: 'domcontentloaded' })
        await page.locator('[data-feedback-entry]').waitFor({ state: 'visible', timeout: 15000 })
      }

      const expectedAuthorizationErrors = actor.label === 'primary' ? consoleProblems.filter((problem) => problem.intentionalDenialWindow && problem.type === 'error' && /^Failed to load resource: the server responded with a status of 403(?: \([^)]*\))?$/.test(problem.text)) : []
      const unexpectedConsoleProblems = consoleProblems.filter((problem) => !expectedAuthorizationErrors.includes(problem))
      assert.deepEqual(unexpectedConsoleProblems, [], `${actor.label} browser console contains unexpected warning/error`)
      assert.deepEqual(pageErrors, [], `${actor.label} browser page contains errors`)
      tenantResults.push({
        tenant_id: actor.tenant.id,
        subject_id: actor.tenant.administrator_subject_id,
        submit_frame_origin: submitOrigin,
        status_frame_origin: statusOrigin,
        expected_authorization_console_error_count: expectedAuthorizationErrors.length,
        unexpected_console_error_or_warning_count: 0,
        page_error_count: 0,
        capability_revocation_checked: actor.label === 'primary',
      })
    } finally {
      try {
        if (revoked) await changeGrant(context, actor, 'grant')
      } finally {
        await context.close()
      }
    }
  }

  const state = {
    schema_version: 1,
    scenario_id: 'oms-v1-p9-full-composition-v1',
    phase: 'C11',
    result: 'pass',
    deployment: {
      profile: 'D1', compose_project: c0.deployment.compose_project, public_url: baseURL,
      c0_checksums_sha256: fileSha256(join(root, 'c0/checksums.sha256')),
      c1_checksums_sha256: fileSha256(join(root, 'c1/checksums.sha256')),
      c9_checksums_sha256: fileSha256(join(root, 'c9/checksums.sha256')),
      c10_checksums_sha256: fileSha256(join(root, 'c10/checksums.sha256')),
    },
    browser: { engine: 'chromium', version: browser.version(), headless: true },
    tenants: tenantResults,
    checks: {
      two_tenant_sessions: 'pass', submit_and_status_open: 'pass', cross_origin_sdk: 'pass',
      capability_revocation_hidden: 'pass', non_feedback_navigation_during_revocation: 'pass',
      capability_regrant_restored: 'pass', unexpected_console_clean: 'pass', credentials_recorded: false,
    },
  }
  writeFileSync(join(runtime, 'browser-evidence.tsv'), `${rows.map((row) => row.join('\t')).join('\n')}\n`, { mode: 0o600 })
  writeFileSync(join(runtime, 'state-after-c11.json'), `${JSON.stringify(state, null, 2)}\n`, { mode: 0o600 })
  writeFileSync(join(runtime, 'result.md'), '# P9 C11 browser result\n\nPASS: two Tenant sessions opened Feedback submit/status in the configured cross-origin frame; primary capability revocation hid Feedback while OMS navigation remained usable, and regrant restored it. Unexpected Console warnings/errors and page errors: 0. The expected authorization-denial resource error is counted separately.\n', { mode: 0o600 })
  const stage = `${evidenceDir}.stage-${process.pid}`
  mkdirSync(stage, { mode: 0o700 })
  for (const file of ['browser-evidence.tsv', 'result.md', 'state-after-c11.json']) copyFileSync(join(runtime, file), join(stage, file))
  writeFileSync(join(stage, 'checksums.sha256'), `${['browser-evidence.tsv', 'result.md', 'state-after-c11.json'].map((file) => `${fileSha256(join(stage, file))}  ${file}`).join('\n')}\n`, { mode: 0o600 })
  runVerifier('verify-c11-evidence', [stage])
  if (existsSync(evidenceDir)) rmdirSync(evidenceDir)
  renameSync(stage, evidenceDir)
  console.log(`PASS P9 C11 real browser, cross-origin Feedback and revocation recovery: ${evidenceDir}`)
} finally {
  if (browser) await browser.close()
  rmSync(runtime, { recursive: true, force: true })
}
