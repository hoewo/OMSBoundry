#!/usr/bin/env node

import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import {
  chmodSync,
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  renameSync,
  rmSync,
  rmdirSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { tmpdir } from 'node:os'

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
const mode = (path) => statSync(path).mode & 0o777
const ensureProtectedFile = (path) => {
  if (!statSync(path).isFile() || mode(path) !== 0o600) throw new Error(`protected input must be a regular file with mode 0600: ${path}`)
}
const runVerifier = (name, directory) => {
  try { execFileSync(join(scriptDir, name), [directory], { stdio: ['ignore', 'ignore', 'pipe'] }) }
  catch (error) { throw new Error(`${name} failed: ${String(error?.stderr || '').trim() || 'unknown verifier error'}`) }
}

class PendingFeedbackReply extends Error {
  constructor(items) {
    super('Feedback developer replies are still required')
    this.items = items
  }
}

class CookieJar {
  constructor() { this.values = new Map() }
  update(headers) {
    const lines = typeof headers.getSetCookie === 'function' ? headers.getSetCookie() : [headers.get('set-cookie')].filter(Boolean)
    for (const line of lines) {
      const pair = String(line).split(';', 1)[0]
      const separator = pair.indexOf('=')
      if (separator > 0) this.values.set(pair.slice(0, separator), pair.slice(separator + 1))
    }
  }
  header() { return [...this.values].map(([key, value]) => `${key}=${value}`).join('; ') }
}

const c0Dir = resolve(required('P9_C0_EVIDENCE_DIR'))
const c1Dir = resolve(required('P9_C1_EVIDENCE_DIR'))
const c9Dir = resolve(required('P9_C9_EVIDENCE_DIR'))
const evidenceDir = resolve(required('P9_C10_EVIDENCE_DIR'))
const primaryUsername = required('P9_PRIMARY_ADMIN_USERNAME')
const primaryPasswordFile = resolve(required('P9_PRIMARY_ADMIN_PASSWORD_FILE'))
const secondaryUsername = required('P9_SECONDARY_ADMIN_USERNAME')
const secondaryPasswordFile = resolve(required('P9_SECONDARY_ADMIN_PASSWORD_FILE'))
if (!process.env.P9_C10_EVIDENCE_DIR.startsWith('/')) throw new Error('P9_C10_EVIDENCE_DIR must be absolute')
if (existsSync(evidenceDir)) {
  if (!statSync(evidenceDir).isDirectory()) throw new Error(`P9 C10 evidence path is invalid: ${evidenceDir}`)
  if (readdirSync(evidenceDir).length) throw new Error(`P9 C10 evidence directory must be absent or empty: ${evidenceDir}`)
}
ensureProtectedFile(primaryPasswordFile)
ensureProtectedFile(secondaryPasswordFile)

runVerifier('verify-evidence', c0Dir)
runVerifier('verify-c1-evidence', c1Dir)
runVerifier('verify-c9-evidence', c9Dir)
const c0State = jsonFile(join(c0Dir, 'state-after-c0.json'))
const c1State = jsonFile(join(c1Dir, 'state-after-c1.json'))
const c9State = jsonFile(join(c9Dir, 'state-after-c9.json'))
const c0Checksum = fileSha256(join(c0Dir, 'checksums.sha256'))
const c1Checksum = fileSha256(join(c1Dir, 'checksums.sha256'))
const c9Checksum = fileSha256(join(c9Dir, 'checksums.sha256'))
if (c1State.deployment?.compose_project !== c0State.deployment?.compose_project || c9State.deployment?.compose_project !== c0State.deployment?.compose_project) throw new Error('C0, C1 and C9 Compose projects differ')
if (c9State.deployment?.c0_checksums_sha256 !== c0Checksum || c9State.deployment?.c1_checksums_sha256 !== c1Checksum) throw new Error('C9 does not bind the supplied C0 and C1 evidence')
const baseURL = String(c0State.deployment?.public_url || '').replace(/\/+$/, '')
if (!allowedURL(baseURL)) throw new Error('C0 public URL is not allowed')

const workspace = mkdtempSync(join(tmpdir(), 'oms-p9-c10.'))
chmodSync(workspace, 0o700)
const evidenceRows = [['request', 'expected_status', 'actual_status', 'time_seconds']]
const nowSeconds = () => Number(process.hrtime.bigint()) / 1e9
const record = (label, expected, actual, started) => evidenceRows.push([label, String(expected), String(actual), (nowSeconds() - started).toFixed(3)])

async function requestJSON(url, { method = 'GET', headers = {}, body, jar, expected = [200], label }) {
  const started = nowSeconds()
  const requestHeaders = { Accept: 'application/json', ...headers }
  if (jar?.header()) requestHeaders.Cookie = jar.header()
  const response = await fetch(url, { method, headers: requestHeaders, body, redirect: 'manual' })
  jar?.update(response.headers)
  record(label, expected.join('/'), response.status, started)
  const text = await response.text()
  let payload = {}
  if (text) {
    try { payload = JSON.parse(text) } catch { throw new Error(`${label} returned non-JSON HTTP ${response.status}`) }
  }
  if (!expected.includes(response.status)) throw new Error(`${label} returned HTTP ${response.status}, expected ${expected.join('/')}`)
  return payload
}

async function prepareActor(label, username, passwordFile, tenant) {
  const jar = new CookieJar()
  const csrfBody = await requestJSON(`${baseURL}/auth/api/v1/csrf`, { jar, label: `GET ${label} auth CSRF` })
  const csrf = String(csrfBody.csrf_token || '')
  if (!csrf) throw new Error(`${label} CSRF token missing`)
  await requestJSON(`${baseURL}/auth/api/v1/login`, {
    method: 'POST', jar, label: `POST ${label} login`, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
    body: JSON.stringify({ username, password: readFileSync(passwordFile, 'utf8').replace(/[\r\n]+$/, '') }),
  })
  const session = await requestJSON(`${baseURL}/auth/api/v1/session`, { jar, label: `GET ${label} session` })
  if (session.principal?.tenant_id !== tenant.id || session.principal?.subject_id !== tenant.administrator_subject_id) throw new Error(`${label} IAM session does not match C1`)
  await requestJSON(`${baseURL}/auth/api/v1/admin/grants`, {
    method: 'POST', jar, expected: [204, 409], label: `POST ${label} Feedback grant`, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
    body: JSON.stringify({ subject_id: tenant.administrator_subject_id, audience: 'feedback', action: 'feedback.use' }),
  })
  const envelope = await requestJSON(`${baseURL}/gateway/feedback/config`, {
    method: 'POST', jar, label: `POST ${label} Feedback config`, headers: { 'X-CSRF-Token': csrf },
  })
  const config = envelope.config || {}
  if (config.feedbackV2Enabled !== true || config.feedbackV2NotificationsEnabled !== true || config.projectId !== 115) throw new Error(`${label} Feedback project 115 notifications are not enabled`)
  if (!/^oms1_v[1-9][0-9]{0,3}_[A-Za-z0-9_-]{43}$/.test(String(config.customUserId || ''))) throw new Error(`${label} Feedback identity is invalid`)
  if (!allowedURL(String(config.gatewayUrl || ''))) throw new Error(`${label} Feedback gateway URL is invalid`)
  let token
  if (config.feedbackV2AuthMode === 'apiKey') {
    token = String(config.apiKey || '')
    if (!/^ak_[A-Za-z0-9_-]{16,}$/.test(token)) throw new Error(`${label} Feedback API Key config is invalid`)
  } else if (config.feedbackV2AuthMode === 'session') {
    token = String(config.feedbackSessionToken || '')
    if (!/^fbs_.{12,}$/.test(token)) throw new Error(`${label} Feedback session config is invalid`)
  } else throw new Error(`${label} Feedback auth mode is invalid`)
  return { label, tenant, config, token }
}

const apiPath = (actor, sessionPath, directPath) => `${String(actor.config.gatewayUrl).replace(/\/+$/, '')}/workshop/v2/${actor.config.feedbackV2AuthMode === 'session' ? `feedback${sessionPath}` : `apikey${directPath}`}`
const scopedQuery = (actor, values = {}) => {
  const query = new URLSearchParams(Object.fromEntries(Object.entries(values).map(([key, value]) => [key, String(value)])))
  if (actor.config.feedbackV2AuthMode === 'apiKey') {
    query.set('project_id', String(actor.config.projectId))
    query.set('custom_user_id', actor.config.customUserId)
  }
  return query
}
const feedbackRequest = (actor, sessionPath, directPath, options) => requestJSON(apiPath(actor, sessionPath, directPath), {
  ...options,
  headers: { Authorization: `Bearer ${actor.token}`, ...(options.headers || {}) },
})

async function listFeedbacks(actor) {
  const query = scopedQuery(actor, { page: 1, page_size: 200 })
  const envelope = await feedbackRequest(actor, `/feedbacks?${query}`, `/feedbacks?${query}`, { label: `GET ${actor.label} Feedback list` })
  return Array.isArray(envelope.data) ? envelope.data : []
}

async function ensureFeedback(actor, ordinal) {
  const marker = sha256(`${c0State.deployment.compose_project}\0${actor.tenant.id}`).slice(0, 16)
  const title = `P9 C10 ${marker} ${ordinal}`
  let matches = (await listFeedbacks(actor)).filter((item) => item.title === title)
  if (matches.length > 1) throw new Error(`${actor.label} has duplicate P9 C10 feedback ${ordinal}`)
  let created = false
  if (!matches.length) {
    const metadata = { source: 'oms1-p9-c10', scenario: 'full-composition-feedback-v1', ordinal }
    const direct = actor.config.feedbackV2AuthMode === 'apiKey'
      ? { project_id: actor.config.projectId, custom_user_id: actor.config.customUserId }
      : {}
    const envelope = await feedbackRequest(actor, '/feedbacks', '/feedbacks', {
      method: 'POST', expected: [201], label: `POST ${actor.label} Feedback ${ordinal}`,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...direct, title, content: `OMS1 P9 C10 ${actor.label} feedback ${ordinal}`, data: direct.project_id ? JSON.stringify(metadata) : metadata, attachments: [] }),
    })
    matches = [envelope.data]
    created = true
  }
  const item = matches[0]
  if (!Number.isInteger(item?.id) || item.id < 1 || !String(item.short_id || '')) throw new Error(`${actor.label} Feedback ${ordinal} is invalid`)
  return { id: item.id, shortId: String(item.short_id), title, created, ordinal }
}

async function ensureCustomerMessage(actor, item) {
  const clientMessageID = `p9-c10-${sha256(`${actor.tenant.id}\0${item.ordinal}`).slice(0, 24)}`
  const direct = actor.config.feedbackV2AuthMode === 'apiKey' ? { custom_user_id: actor.config.customUserId } : {}
  await feedbackRequest(actor, `/feedbacks/${item.id}/messages`, `/feedbacks/${item.id}/messages`, {
    method: 'POST', expected: [200, 201], label: `POST ${actor.label} Feedback ${item.ordinal} message`, headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...direct, content: `OMS1 P9 C10 ${actor.label} follow-up ${item.ordinal}`, client_message_id: clientMessageID, metadata: { source: 'oms1-p9-c10' }, attachments: [{ type: 'url', url: 'https://oatest.antuinfo.cn/workbench/?view=oms-v1', file_name: 'OMS1' }] }),
  })
  const query = scopedQuery(actor, { page: 1, page_size: 100 })
  query.delete('project_id')
  const envelope = await feedbackRequest(actor, `/feedbacks/${item.id}/messages?${query}`, `/feedbacks/${item.id}/messages?${query}`, { label: `GET ${actor.label} Feedback ${item.ordinal} messages` })
  const messages = Array.isArray(envelope.data) ? envelope.data : []
  const followups = messages.filter((message) => message.client_message_id === clientMessageID && message.sender_type === 'customer')
  if (followups.length !== 1 || !followups[0].attachments?.some((attachment) => attachment.type === 'url')) throw new Error(`${actor.label} Feedback ${item.ordinal} idempotent attachment message is invalid`)
  const developers = messages.filter((message) => message.sender_type === 'developer')
  return { ...item, customerMessageId: followups[0].id, developerMessageIds: developers.map((message) => message.id) }
}

async function notifications(actor, item, unreadOnly = true) {
  const query = scopedQuery(actor, { feedback_id: item.id, unread_only: unreadOnly, page: 1, page_size: 100 })
  const envelope = await feedbackRequest(actor, `/notifications?${query}`, `/feedback-notifications?${query}`, { label: `GET ${actor.label} Feedback ${item.ordinal} notifications` })
  return { notifications: Array.isArray(envelope.data?.notifications) ? envelope.data.notifications : [], unreadCount: Number(envelope.data?.unread_count || 0) }
}

async function markRead(actor, item) {
  const direct = actor.config.feedbackV2AuthMode === 'apiKey' ? { project_id: actor.config.projectId, custom_user_id: actor.config.customUserId } : {}
  await feedbackRequest(actor, '/notifications/read', '/feedback-notifications/read', {
    method: 'POST', label: `POST ${actor.label} Feedback ${item.ordinal} read`, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...direct, feedback_id: item.id }),
  })
}

async function runActor(actor) {
  const items = []
  for (const ordinal of [1, 2]) items.push(await ensureCustomerMessage(actor, await ensureFeedback(actor, ordinal)))
  const missing = items.filter((item) => !item.developerMessageIds.length)
  if (missing.length) throw new PendingFeedbackReply(missing.map((item) => ({ actor: actor.label, id: item.id, short_id: item.shortId })))
  for (const item of items) {
    const unread = await notifications(actor, item)
    if (!unread.notifications.some((notification) => notification.feedback_id === item.id && notification.sender_type === 'developer' && notification.read_at == null)) throw new Error(`${actor.label} Feedback ${item.ordinal} has no unread developer notification`)
  }
  await markRead(actor, items[0])
  if ((await notifications(actor, items[0])).notifications.length !== 0) throw new Error(`${actor.label} first Feedback was not marked read`)
  if ((await notifications(actor, items[1])).notifications.length === 0) throw new Error(`${actor.label} marking one Feedback read cleared another Feedback`)
  await markRead(actor, items[1])
  if ((await notifications(actor, items[1])).notifications.length !== 0) throw new Error(`${actor.label} second Feedback was not marked read`)
  return items
}

let stageDir = ''
try {
  const actors = [
    await prepareActor('primary', primaryUsername, primaryPasswordFile, c1State.tenants[0]),
    await prepareActor('secondary', secondaryUsername, secondaryPasswordFile, c1State.tenants[1]),
  ]
  const results = []
  const pending = []
  for (const actor of actors) {
    try { results.push({ actor, items: await runActor(actor) }) }
    catch (error) {
      if (error instanceof PendingFeedbackReply) pending.push(...error.items)
      else throw error
    }
  }
  if (pending.length) throw new PendingFeedbackReply(pending)
  for (let index = 0; index < results.length; index++) {
    const ownIDs = new Set(results[index].items.map((item) => item.id))
    const otherIDs = new Set(results[1 - index].items.map((item) => item.id))
    const visibleIDs = new Set((await listFeedbacks(results[index].actor)).map((item) => item.id))
    if (![...ownIDs].every((id) => visibleIDs.has(id)) || [...otherIDs].some((id) => visibleIDs.has(id))) throw new Error(`${results[index].actor.label} Feedback list is not identity-isolated`)
  }

  const capturedAt = new Date().toISOString().replace(/\.\d{3}Z$/, 'Z')
  const state = {
    schema_version: 1, scenario_id: 'oms-v1-p9-full-composition-v1', phase: 'C10', result: 'pass', captured_at: capturedAt,
    deployment: { profile: 'D1', compose_project: c0State.deployment.compose_project, public_url: baseURL, c0_checksums_sha256: c0Checksum, c1_checksums_sha256: c1Checksum, c9_checksums_sha256: c9Checksum },
    feedback: { project_id: 115, auth_modes: [...new Set(results.map(({ actor }) => actor.config.feedbackV2AuthMode))], notifications_enabled: true },
    tenants: results.map(({ actor, items }) => ({ tenant_id: actor.tenant.id, feedbacks: items.map((item) => ({ id: item.id, short_id: item.shortId, ordinal: item.ordinal, customer_message_id: item.customerMessageId, developer_message_ids: item.developerMessageIds, attachment_type: 'url', final_unread_count: 0 })) })),
    checks: { two_feedbacks_per_tenant: 'pass', idempotent_customer_message: 'pass', developer_reply: 'pass', self_list_isolation: 'pass', single_feedback_read_scope: 'pass', cross_tenant_identity_distinct: actors[0].config.customUserId !== actors[1].config.customUserId ? 'pass' : 'fail', credentials_recorded: false },
  }
  if (state.checks.cross_tenant_identity_distinct !== 'pass') throw new Error('Feedback identities are not separated across tenants')
  writeFileSync(join(workspace, 'state-after-c10.json'), `${JSON.stringify(state, null, 2)}\n`, { mode: 0o600 })
  writeFileSync(join(workspace, 'http-evidence.tsv'), `${evidenceRows.map((row) => row.join('\t')).join('\n')}\n`, { mode: 0o600 })
  writeFileSync(join(workspace, 'result.md'), `# P9 C10 Feedback V2 result\n\n- Result: pass\n- Captured at: ${capturedAt}\n- Checks: two Tenant identities, two feedback conversations each, idempotent customer messages with URL attachments, developer replies, unread notifications and feedback-scoped read.\n- API Key, feedback session token, IAM credential, Cookie or pseudonymous user identity recorded in evidence: no\n`, { mode: 0o600 })
  mkdirSync(dirname(evidenceDir), { recursive: true })
  stageDir = mkdtempSync(join(dirname(evidenceDir), '.p9-c10-evidence.'))
  chmodSync(stageDir, 0o700)
  for (const file of ['http-evidence.tsv', 'result.md', 'state-after-c10.json']) copyFileSync(join(workspace, file), join(stageDir, file))
  const manifest = ['http-evidence.tsv', 'result.md', 'state-after-c10.json'].map((file) => `${fileSha256(join(stageDir, file))}  ${file}`).join('\n') + '\n'
  writeFileSync(join(stageDir, 'checksums.sha256'), manifest, { mode: 0o600 })
  runVerifier('verify-c10-evidence', stageDir)
  if (existsSync(evidenceDir)) rmdirSync(evidenceDir)
  renameSync(stageDir, evidenceDir); stageDir = ''
  console.log('PASS P9 C10 Feedback V2 conversations, notifications and scoped read')
} catch (error) {
  if (error instanceof PendingFeedbackReply) {
    console.error('PENDING: reply once as a developer to each Feedback item, then rerun the same command:')
    for (const item of error.items) console.error(`${item.actor}\t${item.id}\t${item.short_id}`)
    process.exitCode = 3
  } else {
    console.error(error instanceof Error ? error.message : String(error))
    process.exitCode = 1
  }
} finally {
  if (stageDir) rmSync(stageDir, { recursive: true, force: true })
  rmSync(workspace, { recursive: true, force: true })
}
