import { randomUUID } from 'node:crypto'
import OSS from 'ali-oss'

const controlBaseUrl = process.env.EVIDENCE_ACCEPTANCE_URL ?? 'http://127.0.0.1:5173/evidence'
const createKey = randomUUID()
const abortKey = randomUUID()
let session

try {
  session = await request('/v1/upload-sessions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': createKey,
    },
    body: JSON.stringify({
      file_name: 'object-boundary-probe.txt',
      size_bytes: 21,
      media_type: 'text/plain',
      client_context: {
        owner_ref: 'oss-object-boundary-acceptance',
        purpose: 'negative-authorization-test',
      },
    }),
  })

  const authorization = session.authorization
  if (authorization?.provider !== 'aliyun-oss') {
    throw new Error('Evidence did not return an Aliyun OSS authorization.')
  }
  const client = new OSS({
    region: authorization.region,
    endpoint: authorization.endpoint,
    bucket: authorization.bucket,
    accessKeyId: authorization.access_key_id,
    accessKeySecret: authorization.access_key_secret,
    stsToken: authorization.security_token,
    secure: authorization.endpoint.startsWith('https://'),
    authorizationV4: true,
  })
  const forbiddenObjectKey = `${authorization.object_key}-forbidden`
  let rejection
  try {
    await client.put(forbiddenObjectKey, Buffer.from('forbidden-key-probe'))
  } catch (error) {
    rejection = {
      status: Number(error?.status ?? error?.statusCode ?? 0),
      code: String(error?.code ?? error?.name ?? ''),
    }
  }
  if (!rejection || (rejection.status !== 403 && rejection.code !== 'AccessDenied')) {
    throw new Error('Temporary STS unexpectedly allowed a different Object Key.')
  }
  process.stdout.write(
    `${JSON.stringify({
      result: 'passed',
      expected_rejection: 'different-object-key',
      http_status: rejection.status,
      error_code: rejection.code,
    })}\n`,
  )
} finally {
  if (session?.session_id) {
    await request(`/v1/upload-sessions/${encodeURIComponent(session.session_id)}/abort`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': abortKey,
      },
      body: JSON.stringify({ reason: 'client_shutdown' }),
    }).catch(() => undefined)
  }
}

async function request(path, init) {
  const response = await fetch(controlBaseUrl + path, init)
  if (!response.ok) {
    throw new Error(`Evidence control request failed with HTTP ${response.status}.`)
  }
  if (response.status === 204) return undefined
  return response.json()
}
