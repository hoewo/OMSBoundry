import { describe, expect, it, vi } from 'vitest'
import { createFetchFileTransferApi } from './api.js'

describe('createFetchFileTransferApi', () => {
  it('maps the versioned business-neutral HTTP contract', async () => {
    const requests: Array<{ url: string; init: RequestInit | undefined }> = []
    const fetcher = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = String(input)
      requests.push({ url, init })
      if (url.endsWith('/v1/upload-sessions')) {
        return json({
          session_id: 'session-1',
          file_id: 'file-1',
          version: 1,
          status: 'pending',
          expires_at: '2030-01-01T00:00:00Z',
          authorization: {
            provider: 'aliyun-oss',
            region: 'cn-beijing',
            endpoint: 'https://oss-cn-beijing.aliyuncs.com',
            endpoint_is_cname: true,
            bucket: 'bucket',
            object_key: 'tenant/t1/file-1/v1',
            access_key_id: 'STS.example',
            access_key_secret: 'temporary',
            security_token: 'token',
            expires_at: '2030-01-01T00:00:00Z',
            multipart: {
              threshold_bytes: 10,
              part_size_bytes: 5,
              parallel: 2,
            },
            scope: {
              bucket: 'bucket',
              object_key: 'tenant/t1/file-1/v1',
              allowed_actions: ['oss:PutObject'],
            },
          },
        })
      }
      if (url.endsWith('/complete')) {
        return json({
          file_id: 'file-1',
          version: 1,
          status: 'available',
          file_name: 'report.pdf',
          size_bytes: 7,
          media_type: 'application/pdf',
          sha256: 'a'.repeat(64),
          created_at: '2026-09-21T00:00:00Z',
        })
      }
      if (url.endsWith('/abort')) return new Response(null, { status: 204 })
      return json({
        file_id: 'file-1',
        version: 1,
        url: 'https://signed.example',
        method: 'GET',
        disposition: 'inline',
        delivery_mode: 'direct',
        expires_at: '2030-01-01T00:00:00Z',
        refresh_after: '2029-12-31T23:55:00Z',
      })
    })
    const api = createFetchFileTransferApi({
      baseUrl: 'https://suite.example/evidence/',
      fetcher: fetcher as typeof fetch,
      getHeaders: () => ({ Authorization: 'Bearer signed-context' }),
    })

    const created = await api.createUploadSession({
      fileName: 'report.pdf',
      sizeBytes: 7,
      mediaType: 'application/pdf',
      idempotencyKey: 'create-1',
      clientContext: { ownerRef: 'opaque-1', purpose: 'supporting-document' },
    })
    expect(created.authorization.provider === 'aliyun-oss' && created.authorization.endpointIsCName).toBe(true)
    await api.completeUpload('session-1', {
      idempotencyKey: 'complete-1',
      observedSizeBytes: 7,
      etag: 'etag-1',
    })
    await api.abortUpload('session-1', {
      idempotencyKey: 'abort-1',
      reason: 'client_shutdown',
    })
    await api.createAccessGrant('file-1', 1, {
      disposition: 'inline',
      deliveryMode: 'direct',
    })

    expect(requests.map((request) => request.url)).toEqual([
      'https://suite.example/evidence/v1/upload-sessions',
      'https://suite.example/evidence/v1/upload-sessions/session-1/complete',
      'https://suite.example/evidence/v1/upload-sessions/session-1/abort',
      'https://suite.example/evidence/v1/files/file-1/versions/1/access-grants',
    ])
    const createHeaders = new Headers(requests[0]?.init?.headers)
    expect(createHeaders.get('Authorization')).toBe('Bearer signed-context')
    expect(createHeaders.get('Idempotency-Key')).toBe('create-1')
    const createBody = JSON.parse(String(requests[0]?.init?.body)) as Record<string, unknown>
    expect(createBody).toEqual({
      file_name: 'report.pdf',
      size_bytes: 7,
      media_type: 'application/pdf',
      client_context: {
        owner_ref: 'opaque-1',
        purpose: 'supporting-document',
      },
    })
    expect(JSON.stringify(createBody)).not.toMatch(/project_id|category|role|permission/i)
  })

  it('marks transient HTTP failures as retryable', async () => {
    const api = createFetchFileTransferApi({
      baseUrl: 'https://suite.example/evidence',
      fetcher: vi.fn(async () =>
        json({ error: { code: 'temporarily_unavailable', message: 'try later' } }, 503),
      ) as typeof fetch,
    })

    await expect(
      api.completeUpload('session-1', {
        idempotencyKey: 'complete-1',
        observedSizeBytes: 7,
      }),
    ).rejects.toMatchObject({ code: 'http_error', status: 503, retryable: true })
  })
})

function json(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
