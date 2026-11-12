#!/usr/bin/env node

import assert from 'node:assert/strict'
import { spawn, execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const repoDir = join(scriptDir, '../..')
const runtime = mkdtempSync(join(tmpdir(), 'oms-p9-c11-test.'))
const fixture = join(runtime, 'fixture')
const evidence = join(fixture, 'c11')
const feedbackSource = readFileSync(join(repoDir, 'frontend/oms-workbench/internal/server/oms_phase1_feedback.js'), 'utf8')
const sha256 = (value) => createHash('sha256').update(value).digest('hex')

const send = (response, status, type, body, headers = {}) => {
  response.writeHead(status, { 'Content-Type': type, 'Cache-Control': 'no-store', ...headers })
  response.end(body)
}
const sendJSON = (response, status, payload, headers = {}) => send(response, status, 'application/json; charset=utf-8', status === 204 ? '' : `${JSON.stringify(payload)}\n`, headers)
const readJSON = async (request) => {
  const chunks = []
  for await (const chunk of request) chunks.push(chunk)
  return chunks.length ? JSON.parse(Buffer.concat(chunks).toString('utf8')) : {}
}
const listen = (server, port = 0) => new Promise((resolve, reject) => {
  server.once('error', reject)
  server.listen(port, '127.0.0.1', () => resolve())
})
const close = (server) => new Promise((resolve) => server.close(() => resolve()))
const actorFromCookie = (request) => String(request.headers.cookie || '').includes('session=secondary') ? 'secondary' : 'primary'

const csrf = 'csrf-token-0123456789'
let hostOrigin = ''
let sdkOrigin = ''
let c1

const sdkServer = createServer((request, response) => {
  const url = new URL(request.url || '/', sdkOrigin || 'http://127.0.0.1')
  if (url.pathname !== '/embed' || url.searchParams.get('parentOrigin') !== hostOrigin) return send(response, 404, 'text/plain; charset=utf-8', 'not found')
  return send(response, 200, 'text/html; charset=utf-8', `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>Feedback fixture</title></head><body><script>
  (() => {
    const parentOrigin = new URLSearchParams(location.search).get('parentOrigin');
    const send = (type, extra) => parent.postMessage(Object.assign({source:'feedback-sdk-web', type}, extra || {}), parentOrigin);
    addEventListener('message', event => {
      if (event.origin !== parentOrigin || event.source !== parent) return;
      const message = event.data;
      if (!message || message.source !== 'feedback-sdk-host') return;
      if (message.type === 'feedback-sdk:configure') send('feedback-sdk:configured', {request_id: message.request_id});
      if (message.type === 'feedback-sdk:get-unread-count') send('feedback-sdk:unread-count', {request_id: message.request_id, unread_count: 0});
    });
    send('feedback-sdk:ready', { supported_contracts: ['v3'], sdk_version: '3.0.1', build_id: 'acceptance' });
  })();
  </script></body></html>`)
})

const hostDocument = (actor) => {
  const tenant = c1.tenants[actor === 'primary' ? 0 : 1]
  const sdkURL = `${sdkOrigin}/embed?parentOrigin=${encodeURIComponent(hostOrigin)}`
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>OMS P9 browser fixture</title><style>[hidden]{display:none!important}.oms-v1__feedback-layer{position:fixed;inset:0}.oms-v1__feedback-frame{width:100%;height:100%;border:0}</style></head><body>
  <main data-oms-v1 data-feedback-sdk-url="${sdkURL}" data-tenant-id="${tenant.id}" data-subject-id="${tenant.administrator_subject_id}">
    <button type="button" data-oms-v1-nav="overview">总览</button><button type="button" data-oms-v1-nav="internal">公司内部管理</button><button type="button" data-oms-v1-nav="feedback">产品反馈</button>
    <section class="is-active" data-oms-v1-page="overview">总览页</section><section data-oms-v1-page="internal">公司内部管理页</section>
    <section data-oms-v1-page="feedback" data-feedback-page><div data-feedback-notice><p>正在连接产品反馈。</p></div><button type="button" data-feedback-mode="submit">提交反馈</button><button type="button" data-feedback-mode="status">我的反馈</button></section>
  </main>
  <script>document.querySelectorAll('[data-oms-v1-nav]').forEach(button=>button.addEventListener('click',()=>{document.querySelectorAll('[data-oms-v1-page]').forEach(page=>page.classList.toggle('is-active',page.dataset.omsV1Page===button.dataset.omsV1Nav))}))</script>
  <script src="/assets/oms-phase1-feedback.js"></script></body></html>`
}

const hostServer = createServer(async (request, response) => {
  try {
    const url = new URL(request.url || '/', hostOrigin || 'http://127.0.0.1')
    const actor = actorFromCookie(request)
    if (request.method === 'GET' && url.pathname === '/auth/api/v1/csrf') return sendJSON(response, 200, { csrf_token: csrf }, { 'Set-Cookie': `oms_iam_csrf=${csrf}; Path=/; SameSite=Lax` })
    if (request.method === 'POST' && url.pathname === '/auth/api/v1/login') {
      const body = await readJSON(request)
      const selected = body.username === 'secondary-admin' ? 'secondary' : 'primary'
      return sendJSON(response, 200, { status: 'authenticated' }, { 'Set-Cookie': `session=${selected}; Path=/; HttpOnly; SameSite=Lax` })
    }
    if (request.method === 'GET' && url.pathname === '/auth/api/v1/session') {
      if (!String(request.headers.cookie || '').includes('session=')) return sendJSON(response, 401, { code: 'UNAUTHENTICATED' })
      const tenant = c1.tenants[actor === 'primary' ? 0 : 1]
      return sendJSON(response, 200, { principal: { tenant_id: tenant.id, subject_id: tenant.administrator_subject_id } })
    }
    if (request.method === 'POST' && url.pathname === '/auth/api/v1/logout') return sendJSON(response, 204, {}, { 'Set-Cookie': 'session=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax' })
    if (request.method === 'POST' && url.pathname === '/gateway/feedback/config') {
      if (!String(request.headers.cookie || '').includes('session=')) return sendJSON(response, 401, { code: 'UNAUTHENTICATED' })
      return sendJSON(response, 200, { config: { feedbackV2Enabled: true, feedbackV2NotificationsEnabled: true, feedbackV2AuthMode: 'apiKey', apiKey: 'ak_fixture_0123456789abcdef0123456789abcdef', projectId: 115, customUserId: `oms1_v1_${(actor === 'primary' ? 'A' : 'B').repeat(43)}`, gatewayUrl: sdkOrigin, theme: 'system', locale: 'zh-CN' } })
    }
    if (request.method === 'GET' && url.pathname === '/workbench/') return send(response, 200, 'text/html; charset=utf-8', hostDocument(actor))
    if (request.method === 'GET' && url.pathname === '/assets/oms-phase1-feedback.js') return send(response, 200, 'text/javascript; charset=utf-8', feedbackSource)
    if (request.method === 'GET' && url.pathname === '/favicon.ico') return send(response, 204, 'image/x-icon', '')
    return sendJSON(response, 404, { code: 'NOT_FOUND' })
  } catch (error) {
    return sendJSON(response, 500, { code: 'ERROR', message: error.message })
  }
})

const runDriver = () => new Promise((resolve) => {
  const child = spawn(process.execPath, [join(scriptDir, 'verify-c11-browser.mjs')], {
    env: {
      ...process.env,
      P9_RUN_EVIDENCE_ROOT: fixture,
      P9_C11_EVIDENCE_DIR: evidence,
      P9_PRIMARY_ADMIN_USERNAME: 'primary-admin',
      P9_PRIMARY_ADMIN_PASSWORD_FILE: join(fixture, 'primary.password'),
      P9_SECONDARY_ADMIN_USERNAME: 'secondary-admin',
      P9_SECONDARY_ADMIN_PASSWORD_FILE: join(fixture, 'secondary.password'),
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  let stdout = ''; let stderr = ''
  child.stdout.on('data', (chunk) => { stdout += chunk })
  child.stderr.on('data', (chunk) => { stderr += chunk })
  child.on('close', (code) => resolve({ code, stdout, stderr }))
})

const verifierRejects = (directory) => {
  try { execFileSync(join(scriptDir, 'verify-c11-evidence'), [directory], { stdio: 'ignore' }); return false } catch { return true }
}
const chainRejects = (directory) => {
  try { execFileSync(join(scriptDir, 'verify-run-c0-c11'), [directory], { stdio: 'ignore' }); return false } catch { return true }
}
const refreshC11StateChecksum = (directory) => {
  const statePath = join(directory, 'c11/state-after-c11.json')
  const manifestPath = join(directory, 'c11/checksums.sha256')
  writeFileSync(manifestPath, readFileSync(manifestPath, 'utf8').replace(/^[0-9a-f]{64}  state-after-c11\.json$/m, `${sha256(readFileSync(statePath))}  state-after-c11.json`))
}

try {
  execFileSync(process.execPath, [join(scriptDir, 'test-c10-feedback.mjs')], { env: { ...process.env, P9_C10_TEST_ARTIFACT_DIR: fixture }, stdio: 'ignore' })
  c1 = JSON.parse(readFileSync(join(fixture, 'c1/state-after-c1.json'), 'utf8'))
  const publicURL = new URL(JSON.parse(readFileSync(join(fixture, 'c0/state-after-c0.json'), 'utf8')).deployment.public_url)
  await listen(sdkServer)
  sdkOrigin = `http://127.0.0.1:${sdkServer.address().port}`
  await listen(hostServer, Number(publicURL.port))
  hostOrigin = publicURL.origin

  const run = await runDriver()
  if (run.code !== 0 || !run.stdout.includes('PASS P9 C11 real browser')) throw new Error(`C11 browser driver failed: ${run.code}\n${run.stderr}`)
  execFileSync(join(scriptDir, 'verify-c11-evidence'), [evidence], { stdio: 'ignore' })
  const secretDir = join(runtime, 'secrets'); mkdirSync(secretDir, { mode: 0o700 })
  const resumed = execFileSync(join(scriptDir, 'run-c0-c11'), [], { env: { ...process.env, P9_RUN_EVIDENCE_ROOT: fixture, P9_C5_SECRET_DIR: secretDir, P9_PRIMARY_ADMIN_USERNAME: 'primary-admin', P9_PRIMARY_ADMIN_PASSWORD_FILE: join(fixture, 'primary.password'), P9_SECONDARY_ADMIN_USERNAME: 'secondary-admin', P9_SECONDARY_ADMIN_PASSWORD_FILE: join(fixture, 'secondary.password') }, encoding: 'utf8' })
  assert.match(resumed, /PASS P9 continuous C0-C11 evidence chain/)
  assert.match(resumed, /no stage was executed/)

  const missingBrowser = join(runtime, 'missing-browser'); cpSync(fixture, missingBrowser, { recursive: true }); rmSync(join(missingBrowser, 'c9'), { recursive: true }); rmSync(join(missingBrowser, 'c10'), { recursive: true }); rmSync(join(missingBrowser, 'c11'), { recursive: true })
  let preflightError = ''
  try {
    execFileSync(join(scriptDir, 'run-c0-c11'), [], { env: { ...process.env, P9_RUN_EVIDENCE_ROOT: missingBrowser, P9_C5_SECRET_DIR: secretDir, P9_PRIMARY_ADMIN_USERNAME: 'primary-admin', P9_PRIMARY_ADMIN_PASSWORD_FILE: join(missingBrowser, 'primary.password'), P9_SECONDARY_ADMIN_USERNAME: 'secondary-admin', P9_SECONDARY_ADMIN_PASSWORD_FILE: join(missingBrowser, 'secondary.password'), PLAYWRIGHT_MODULE_PATH: join(runtime, 'missing-playwright.mjs') }, stdio: ['ignore', 'ignore', 'pipe'] })
  } catch (error) { preflightError = String(error.stderr || '') }
  assert.match(preflightError, /C11 browser preflight failed:/)
  assert.doesNotMatch(preflightError, /P9_C9_CONFIRM_IN_PLACE_RESTORE/, 'browser preflight must fail before the destructive C9 gate')

  const wrongActor = join(runtime, 'wrong-actor'); cpSync(fixture, wrongActor, { recursive: true })
  const wrongActorStatePath = join(wrongActor, 'c11/state-after-c11.json'); const wrongActorState = JSON.parse(readFileSync(wrongActorStatePath, 'utf8')); wrongActorState.tenants[0].tenant_id = '99999999-9999-4999-8999-999999999999'; writeFileSync(wrongActorStatePath, `${JSON.stringify(wrongActorState, null, 2)}\n`); refreshC11StateChecksum(wrongActor)
  execFileSync(join(scriptDir, 'verify-c11-evidence'), [join(wrongActor, 'c11')], { stdio: 'ignore' })
  assert.equal(chainRejects(wrongActor), true, 'C0-C11 chain accepted a browser actor outside C1')

  const sameOrigin = join(runtime, 'same-origin'); cpSync(fixture, sameOrigin, { recursive: true })
  const sameOriginStatePath = join(sameOrigin, 'c11/state-after-c11.json'); const sameOriginState = JSON.parse(readFileSync(sameOriginStatePath, 'utf8')); for (const tenant of sameOriginState.tenants) { tenant.submit_frame_origin = hostOrigin; tenant.status_frame_origin = hostOrigin } writeFileSync(sameOriginStatePath, `${JSON.stringify(sameOriginState, null, 2)}\n`); refreshC11StateChecksum(sameOrigin)
  execFileSync(join(scriptDir, 'verify-c11-evidence'), [join(sameOrigin, 'c11')], { stdio: 'ignore' })
  assert.equal(chainRejects(sameOrigin), true, 'C0-C11 chain accepted a same-origin Feedback frame')

  const tampered = join(runtime, 'tampered'); cpSync(evidence, tampered, { recursive: true }); writeFileSync(join(tampered, 'result.md'), `${readFileSync(join(tampered, 'result.md'), 'utf8')}tamper\n`)
  assert.equal(verifierRejects(tampered), true, 'C11 accepted checksum drift')

  const consoleLeak = join(runtime, 'console-leak'); cpSync(evidence, consoleLeak, { recursive: true })
  const statePath = join(consoleLeak, 'state-after-c11.json'); const state = JSON.parse(readFileSync(statePath, 'utf8')); state.tenants[0].unexpected_console_error_or_warning_count = 1; writeFileSync(statePath, `${JSON.stringify(state, null, 2)}\n`)
  const manifestPath = join(consoleLeak, 'checksums.sha256'); writeFileSync(manifestPath, readFileSync(manifestPath, 'utf8').replace(/^[0-9a-f]{64}  state-after-c11\.json$/m, `${sha256(readFileSync(statePath))}  state-after-c11.json`))
  assert.equal(verifierRejects(consoleLeak), true, 'C11 accepted a browser console warning')

  const missingDenial = join(runtime, 'missing-denial'); cpSync(evidence, missingDenial, { recursive: true })
  const httpPath = join(missingDenial, 'browser-evidence.tsv'); writeFileSync(httpPath, readFileSync(httpPath, 'utf8').split('\n').filter((line) => !line.includes('\tlogged-out-config\t')).join('\n'))
  const denialManifest = join(missingDenial, 'checksums.sha256'); writeFileSync(denialManifest, readFileSync(denialManifest, 'utf8').replace(/^[0-9a-f]{64}  browser-evidence\.tsv$/m, `${sha256(readFileSync(httpPath))}  browser-evidence.tsv`))
  assert.equal(verifierRejects(missingDenial), true, 'C11 accepted missing browser session-denial evidence')

  const secretLeak = join(runtime, 'secret-leak'); cpSync(evidence, secretLeak, { recursive: true }); const resultPath = join(secretLeak, 'result.md'); writeFileSync(resultPath, `${readFileSync(resultPath, 'utf8')}ak_leaked_0123456789abcdef\n`)
  const secretManifest = join(secretLeak, 'checksums.sha256'); writeFileSync(secretManifest, readFileSync(secretManifest, 'utf8').replace(/^[0-9a-f]{64}  result\.md$/m, `${sha256(readFileSync(resultPath))}  result.md`))
  assert.equal(verifierRejects(secretLeak), true, 'C11 accepted a Feedback API Key leak')

  console.log('PASS P9 C11 real Chromium fixture, session recovery and evidence negatives')
} finally {
  await Promise.allSettled([close(hostServer), close(sdkServer)])
  rmSync(runtime, { recursive: true, force: true })
}
