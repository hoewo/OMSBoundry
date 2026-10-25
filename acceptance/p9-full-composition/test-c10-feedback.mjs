#!/usr/bin/env node

import { createServer } from 'node:http'
import { spawn } from 'node:child_process'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { chmodSync, cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = dirname(fileURLToPath(import.meta.url))
const runtime = mkdtempSync(join(tmpdir(), 'oms-p9-c10-test.'))
const fixture = join(runtime, 'fixture')
const evidence = join(runtime, 'c10')
const artifactDir = String(process.env.P9_C10_TEST_ARTIFACT_DIR || '').trim()
const sha256 = (value) => createHash('sha256').update(value).digest('hex')
const refreshManifest = (directory, artifacts) => {
  const manifest = join(directory, 'checksums.sha256')
  const selected = new Set(artifacts)
  const lines = readFileSync(manifest, 'utf8').trimEnd().split('\n').map((line) => {
    const match = line.match(/^([0-9a-f]{64})  (.+)$/)
    if (!match) throw new Error(`invalid fixture manifest: ${manifest}`)
    return `${selected.has(match[2]) ? sha256(readFileSync(join(directory, match[2]))) : match[1]}  ${match[2]}`
  })
  writeFileSync(manifest, `${lines.join('\n')}\n`)
  return sha256(readFileSync(manifest))
}
const rewriteState = (directory, stateName, mutate) => {
  const path = join(directory, stateName)
  const state = JSON.parse(readFileSync(path, 'utf8'))
  mutate(state)
  writeFileSync(path, `${JSON.stringify(state, null, 2)}\n`)
  return refreshManifest(directory, [stateName])
}

const send = (response, status, payload, headers = {}) => {
  const body = status === 204 ? '' : `${JSON.stringify(payload)}\n`
  response.writeHead(status, { 'Content-Type': 'application/json', ...headers })
  response.end(body)
}
const bodyJSON = async (request) => {
  const chunks = []
  for await (const chunk of request) chunks.push(chunk)
  return chunks.length ? JSON.parse(Buffer.concat(chunks).toString('utf8')) : {}
}

let nextFeedback = 100
let nextMessage = 1000
let nextNotification = 5000
const feedbacks = []
const messages = new Map()
const notifications = []
let serverOrigin = ''
const actors = {
  primary: { custom: 'oms1_v1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' },
  secondary: { custom: 'oms1_v1_BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB' },
}
const actorFromCookie = (request) => String(request.headers.cookie || '').includes('session=secondary') ? 'secondary' : 'primary'

function addDeveloperReplies() {
  for (const feedback of feedbacks) {
    const list = messages.get(feedback.id)
    if (list.some((message) => message.sender_type === 'developer')) continue
    const message = { id: nextMessage++, feedback_id: feedback.id, project_id: 115, sender_type: 'developer', message_type: 'text', content: '已收到并完成 P9 C10 验收回复', attachments: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    list.push(message)
    notifications.push({ id: nextNotification++, project_id: 115, feedback_id: feedback.id, message_id: message.id, sender_type: 'developer', type: 'developer_message', message_preview: message.content, created_at: message.created_at, read_at: null, custom_user_id: feedback.custom_user_id })
  }
}

const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url, serverOrigin || 'http://127.0.0.1')
    if (request.method === 'GET' && url.pathname === '/auth/api/v1/csrf') return send(response, 200, { csrf_token: 'csrf-token-0123456789' }, { 'Set-Cookie': 'oms_iam_csrf=csrf-token-0123456789; Path=/; HttpOnly' })
    if (request.method === 'POST' && url.pathname === '/auth/api/v1/login') {
      const body = await bodyJSON(request)
      const actor = body.username === 'secondary-admin' ? 'secondary' : 'primary'
      return send(response, 200, { status: 'authenticated' }, { 'Set-Cookie': `session=${actor}; Path=/; HttpOnly` })
    }
    if (request.method === 'GET' && url.pathname === '/auth/api/v1/session') {
      const actor = actorFromCookie(request)
      const c1 = JSON.parse(readFileSync(join(fixture, 'c1/state-after-c1.json'), 'utf8'))
      const tenant = c1.tenants[actor === 'primary' ? 0 : 1]
      return send(response, 200, { principal: { tenant_id: tenant.id, subject_id: tenant.administrator_subject_id } })
    }
    if (request.method === 'POST' && url.pathname === '/auth/api/v1/admin/grants') return send(response, 204, {})
    if (request.method === 'POST' && url.pathname === '/gateway/feedback/config') {
      const actor = actorFromCookie(request)
      return send(response, 200, { config: { feedbackV2Enabled: true, feedbackV2NotificationsEnabled: true, feedbackV2AuthMode: 'apiKey', apiKey: 'ak_test_0123456789abcdef0123456789abcdef', projectId: 115, customUserId: actors[actor].custom, gatewayUrl: serverOrigin, theme: 'system' } })
    }
    if (!url.pathname.startsWith('/workshop/v2/apikey/')) return send(response, 404, { code: 'NOT_FOUND' })
    if (request.headers.authorization !== 'Bearer ak_test_0123456789abcdef0123456789abcdef') return send(response, 401, { code: 'UNAUTHORIZED' })

    if (request.method === 'GET' && url.pathname === '/workshop/v2/apikey/feedbacks') {
      const custom = url.searchParams.get('custom_user_id')
      return send(response, 200, { code: 'OK', data: feedbacks.filter((item) => item.custom_user_id === custom), meta: { total: feedbacks.filter((item) => item.custom_user_id === custom).length } })
    }
    if (request.method === 'POST' && url.pathname === '/workshop/v2/apikey/feedbacks') {
      const body = await bodyJSON(request)
      const item = { id: nextFeedback++, project_id: body.project_id, short_id: `FB${nextFeedback}`, title: body.title, content: body.content, custom_user_id: body.custom_user_id, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
      feedbacks.push(item)
      messages.set(item.id, [{ id: nextMessage++, feedback_id: item.id, project_id: 115, sender_type: 'customer', message_type: 'text', content: body.content, attachments: [], created_at: item.created_at, updated_at: item.updated_at }])
      return send(response, 201, { code: 'OK', data: item })
    }
    const messageMatch = url.pathname.match(/^\/workshop\/v2\/apikey\/feedbacks\/(\d+)\/messages$/)
    if (messageMatch && request.method === 'POST') {
      const feedbackID = Number(messageMatch[1]); const body = await bodyJSON(request); const list = messages.get(feedbackID) || []
      let item = list.find((message) => message.client_message_id === body.client_message_id)
      let status = 200
      if (!item) {
        status = 201
        item = { id: nextMessage++, feedback_id: feedbackID, project_id: 115, sender_type: 'customer', sender_custom_user_id: body.custom_user_id, client_message_id: body.client_message_id, message_type: 'text', content: body.content, attachments: body.attachments || [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
        list.push(item); messages.set(feedbackID, list)
      }
      return send(response, status, { code: 'OK', data: item })
    }
    if (messageMatch && request.method === 'GET') return send(response, 200, { code: 'OK', data: messages.get(Number(messageMatch[1])) || [] })
    if (request.method === 'GET' && url.pathname === '/workshop/v2/apikey/feedback-notifications') {
      const custom = url.searchParams.get('custom_user_id'); const feedbackID = Number(url.searchParams.get('feedback_id'))
      const items = notifications.filter((item) => item.custom_user_id === custom && (!feedbackID || item.feedback_id === feedbackID) && (url.searchParams.get('unread_only') !== 'true' || item.read_at == null)).map(({ custom_user_id: _, ...item }) => item)
      return send(response, 200, { code: 'OK', data: { notifications: items, unread_count: notifications.filter((item) => item.custom_user_id === custom && item.read_at == null).length } })
    }
    if (request.method === 'POST' && url.pathname === '/workshop/v2/apikey/feedback-notifications/read') {
      const body = await bodyJSON(request)
      let marked = 0
      for (const item of notifications) if (item.custom_user_id === body.custom_user_id && item.feedback_id === body.feedback_id && item.read_at == null) { item.read_at = new Date().toISOString(); marked++ }
      return send(response, 200, { code: 'OK', data: { marked_count: marked } })
    }
    return send(response, 404, { code: 'NOT_FOUND' })
  } catch (error) {
    return send(response, 500, { code: 'ERROR', error: { message: error.message } })
  }
})

const runDriver = () => new Promise((resolve) => {
  const child = spawn(process.execPath, [join(scriptDir, 'verify-c10-feedback.mjs')], {
    env: { ...process.env, P9_C0_EVIDENCE_DIR: join(fixture, 'c0'), P9_C1_EVIDENCE_DIR: join(fixture, 'c1'), P9_C9_EVIDENCE_DIR: join(fixture, 'c9'), P9_C10_EVIDENCE_DIR: evidence, P9_PRIMARY_ADMIN_USERNAME: 'primary-admin', P9_PRIMARY_ADMIN_PASSWORD_FILE: join(fixture, 'primary.password'), P9_SECONDARY_ADMIN_USERNAME: 'secondary-admin', P9_SECONDARY_ADMIN_PASSWORD_FILE: join(fixture, 'secondary.password') },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  let stdout = ''; let stderr = ''
  child.stdout.on('data', (chunk) => { stdout += chunk })
  child.stderr.on('data', (chunk) => { stderr += chunk })
  child.on('close', (code) => resolve({ code, stdout, stderr }))
})

try {
  execFileSync(join(scriptDir, 'test-c9-recovery'), [], { env: { ...process.env, P9_C9_TEST_ARTIFACT_DIR: fixture }, stdio: 'ignore' })
  await new Promise((resolve, reject) => server.listen(0, '127.0.0.1', (error) => error ? reject(error) : resolve()))
  serverOrigin = `http://127.0.0.1:${server.address().port}`
  const origin = serverOrigin
  const phaseDigests = []
  for (let phase = 0; phase <= 8; phase++) {
    phaseDigests[phase] = rewriteState(join(fixture, `c${phase}`), `state-after-c${phase}.json`, (state) => {
      state.deployment.public_url = origin
      for (let prior = 0; prior < phase; prior++) state.deployment[`c${prior}_checksums_sha256`] = phaseDigests[prior]
    })
  }
  const post7Digest = rewriteState(join(fixture, 'c9/post-c7'), 'state-after-c7.json', (state) => {
    state.deployment.public_url = origin
    for (let phase = 0; phase <= 6; phase++) state.deployment[`c${phase}_checksums_sha256`] = phaseDigests[phase]
  })
  const post8Digest = rewriteState(join(fixture, 'c9/post-c8'), 'state-after-c8.json', (state) => {
    state.deployment.public_url = origin
    for (let phase = 0; phase <= 6; phase++) state.deployment[`c${phase}_checksums_sha256`] = phaseDigests[phase]
    state.deployment.c7_checksums_sha256 = post7Digest
  })
  const c9StatePath = join(fixture, 'c9/state-after-c9.json')
  const c9State = JSON.parse(readFileSync(c9StatePath, 'utf8'))
  c9State.deployment.public_url = origin
  for (let phase = 0; phase <= 8; phase++) c9State.deployment[`c${phase}_checksums_sha256`] = phaseDigests[phase]
  c9State.deployment.post_c7_checksums_sha256 = post7Digest
  c9State.deployment.post_c8_checksums_sha256 = post8Digest
  writeFileSync(c9StatePath, `${JSON.stringify(c9State, null, 2)}\n`)
  refreshManifest(join(fixture, 'c9'), ['post-c7/checksums.sha256', 'post-c8/checksums.sha256', 'state-after-c9.json'])
  const pending = await runDriver()
  if (pending.code !== 3 || !pending.stderr.includes('PENDING:') || existsSync(evidence) || feedbacks.length !== 4) throw new Error(`C10 did not stop at the developer-reply gate: ${pending.code} ${pending.stderr}`)
  addDeveloperReplies()
  const completed = await runDriver()
  if (completed.code !== 0 || !completed.stdout.includes('PASS P9 C10') || feedbacks.length !== 4) throw new Error(`C10 did not resume to completion: ${completed.code} ${completed.stderr}`)
  execFileSync(join(scriptDir, 'verify-c10-evidence'), [evidence], { stdio: 'ignore' })

  const tampered = join(runtime, 'tampered'); cpSync(evidence, tampered, { recursive: true }); writeFileSync(join(tampered, 'result.md'), `${readFileSync(join(tampered, 'result.md'), 'utf8')}tamper\n`)
  let rejected = false
  try { execFileSync(join(scriptDir, 'verify-c10-evidence'), [tampered], { stdio: 'ignore' }) } catch { rejected = true }
  if (!rejected) throw new Error('P9 C10 accepted checksum drift')

  const broken = join(runtime, 'broken'); cpSync(evidence, broken, { recursive: true })
  const statePath = join(broken, 'state-after-c10.json'); const state = JSON.parse(readFileSync(statePath, 'utf8')); state.tenants[0].feedbacks[0].developer_message_ids = []; writeFileSync(statePath, `${JSON.stringify(state, null, 2)}\n`)
  const manifestPath = join(broken, 'checksums.sha256'); writeFileSync(manifestPath, readFileSync(manifestPath, 'utf8').replace(/^[0-9a-f]{64}  state-after-c10\.json$/m, `${sha256(readFileSync(statePath))}  state-after-c10.json`))
  rejected = false
  try { execFileSync(join(scriptDir, 'verify-c10-evidence'), [broken], { stdio: 'ignore' }) } catch { rejected = true }
  if (!rejected) throw new Error('P9 C10 accepted missing developer evidence')

  const leaked = join(runtime, 'leaked'); cpSync(evidence, leaked, { recursive: true }); const resultPath = join(leaked, 'result.md'); writeFileSync(resultPath, `${readFileSync(resultPath, 'utf8')}ak_leaked_0123456789abcdef\n`)
  const leakedManifest = join(leaked, 'checksums.sha256'); writeFileSync(leakedManifest, readFileSync(leakedManifest, 'utf8').replace(/^[0-9a-f]{64}  result\.md$/m, `${sha256(readFileSync(resultPath))}  result.md`))
  rejected = false
  try { execFileSync(join(scriptDir, 'verify-c10-evidence'), [leaked], { stdio: 'ignore' }) } catch { rejected = true }
  if (!rejected) throw new Error('P9 C10 accepted a Feedback credential leak')

  if (artifactDir) {
    if (existsSync(artifactDir)) throw new Error(`P9 C10 test artifact directory already exists: ${artifactDir}`)
    cpSync(fixture, artifactDir, { recursive: true }); cpSync(evidence, join(artifactDir, 'c10'), { recursive: true })
    chmodSync(join(artifactDir, 'primary.password'), 0o600); chmodSync(join(artifactDir, 'secondary.password'), 0o600)
  }
  console.log('PASS P9 C10 offline pending/resume, two-tenant notifications and evidence negatives')
} finally {
  await new Promise((resolve) => server.close(() => resolve()))
  rmSync(runtime, { recursive: true, force: true })
}
