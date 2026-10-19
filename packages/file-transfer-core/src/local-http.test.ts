import { describe, expect, it, vi } from 'vitest'
import { createDirectUploaderRouter, createLocalHttpUploader } from './local-http.js'
import type { DirectUploader, UploadSession } from './types.js'

function localSession(): UploadSession {
  return {
    sessionId: 'session-1',
    fileId: 'file-1',
    version: 1,
    status: 'pending',
    expiresAt: '2030-01-01T00:00:00Z',
    authorization: {
      provider: 'local-http',
      uploadUrl: 'http://127.0.0.1:8085/v1/local-uploads/session-1?token=bound',
      method: 'PUT',
      objectKey: 'tenants/t1/files/file-1/versions/1',
      expiresAt: '2030-01-01T00:00:00Z',
      headers: { 'Content-Type': 'application/pdf' },
      scope: {
        objectKey: 'tenants/t1/files/file-1/versions/1',
        allowedActions: ['evidence:PutObject'],
      },
    },
  }
}

describe('local HTTP adapter', () => {
  it('uploads to the exact signed URL and reports completion', async () => {
    const fetcher = vi.fn(async () =>
      new Response(null, { status: 204, headers: { ETag: '"etag-1"' } }),
    )
    const progress = vi.fn()
    const uploader = createLocalHttpUploader({ fetcher: fetcher as typeof fetch, now: () => 0 })
    const file = new Blob(['content'], { type: 'application/pdf' })

    await expect(
      uploader.upload({ file, session: localSession() }, { onProgress: progress }),
    ).resolves.toEqual({ etag: 'etag-1' })
    expect(fetcher).toHaveBeenCalledWith(
      'http://127.0.0.1:8085/v1/local-uploads/session-1?token=bound',
      expect.objectContaining({ method: 'PUT', body: file }),
    )
    expect(progress).toHaveBeenLastCalledWith(
      expect.objectContaining({ percent: 100, loadedBytes: 7 }),
    )
  })

  it('fails closed on broader local actions and routes by provider', async () => {
    const local = createLocalHttpUploader({ now: () => 0 })
    const invalid = localSession()
    invalid.authorization.scope.allowedActions.push('evidence:DeleteObject')
    await expect(
      local.upload({ file: new Blob(['x']), session: invalid }),
    ).rejects.toMatchObject({ code: 'contract_error' })

    const aliyunOss: DirectUploader = { upload: vi.fn(async () => ({ etag: 'oss' })) }
    const localHttp: DirectUploader = { upload: vi.fn(async () => ({ etag: 'local' })) }
    const router = createDirectUploaderRouter({ aliyunOss, localHttp })
    await expect(
      router.upload({ file: new Blob(['x']), session: localSession() }),
    ).resolves.toEqual({ etag: 'local' })
    expect(localHttp.upload).toHaveBeenCalledOnce()
    expect(aliyunOss.upload).not.toHaveBeenCalled()
  })
})
