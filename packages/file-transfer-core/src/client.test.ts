import { describe, expect, it, vi } from 'vitest'
import { FileTransferClient } from './client.js'
import { FileTransferError } from './errors.js'
import type {
  AccessGrant,
  DirectUploader,
  EvidenceVersion,
  FileDescriptor,
  FileTransferApi,
  UploadSession,
} from './types.js'

const uploadActions = [
  'oss:PutObject',
  'oss:AbortMultipartUpload',
  'oss:ListParts',
]

function session(): UploadSession {
  return {
    sessionId: 'session-1',
    fileId: 'file-1',
    version: 1,
    status: 'pending',
    expiresAt: '2030-01-01T00:00:00Z',
    authorization: {
      provider: 'aliyun-oss',
      endpoint: 'https://oss-cn-beijing.aliyuncs.com',
      region: 'cn-beijing',
      bucket: 'evidence-private',
      objectKey: 'tenant/t1/file-1/v1',
      accessKeyId: 'STS.example',
      accessKeySecret: 'temporary-secret',
      securityToken: 'temporary-token',
      expiresAt: '2030-01-01T00:00:00Z',
      multipart: {
        thresholdBytes: 20 * 1024 * 1024,
        partSizeBytes: 8 * 1024 * 1024,
        parallel: 3,
      },
      scope: {
        bucket: 'evidence-private',
        objectKey: 'tenant/t1/file-1/v1',
        allowedActions: uploadActions,
      },
    },
  }
}

function version(): EvidenceVersion {
  return {
    fileId: 'file-1',
    version: 1,
    status: 'available',
    fileName: 'report.pdf',
    sizeBytes: 7,
    mediaType: 'application/pdf',
    sha256: 'a'.repeat(64),
    createdAt: '2026-09-21T00:00:00Z',
  }
}

function accessGrant(overrides: Partial<AccessGrant> = {}): AccessGrant {
  return {
    fileId: 'file-1',
    version: 1,
    url: 'https://evidence.example/signed',
    method: 'GET',
    disposition: 'inline',
    deliveryMode: 'direct',
    expiresAt: '2030-01-01T00:00:00Z',
    refreshAfter: '2029-12-31T23:55:00Z',
    ...overrides,
  }
}

function api(overrides: Partial<FileTransferApi> = {}): FileTransferApi {
  return {
    createUploadSession: vi.fn(async () => session()),
    completeUpload: vi.fn(async () => version()),
    abortUpload: vi.fn(async () => undefined),
    createAccessGrant: vi.fn(async () => accessGrant()),
    ...overrides,
  }
}

const uploader: DirectUploader = {
  upload: vi.fn(async () => ({ etag: 'etag-1' })),
}

describe('native browser fetch receiver', () => {
  it('does not invoke native fetch with the SDK instance as receiver', async () => {
    const original = globalThis.fetch
    globalThis.fetch = async function (this: unknown) {
      if (this !== globalThis) throw new TypeError('Illegal invocation')
      return new Response(new Blob(['image bytes']))
    } as typeof fetch
    try {
      const client = new FileTransferClient({api: api({createAccessGrant: async () => accessGrant({deliveryMode:'blob'})}), uploader,
        objectUrls: {create: () => 'blob:fixture-preview', revoke: () => undefined}})
      expect(await client.resolvePreviewUrl({fileId:'file-1',version:1,fileName:'image.png',sizeBytes:11,mediaType:'image/png'}, 'blob')).toBe('blob:fixture-preview')
    } finally {globalThis.fetch = original}
  })
})

describe('FileTransferClient upload lifecycle', () => {
  it('forwards only technical metadata and completes the server-issued session', async () => {
    const transferApi = api()
    const client = new FileTransferClient({
      api: transferApi,
      uploader,
      createId: sequentialIds(),
    })
    const file = new Blob(['content'], { type: 'application/pdf' })

    const result = await client.uploadFile({
      file,
      fileName: 'report.pdf',
      sha256: 'a'.repeat(64),
      clientContext: {
        ownerRef: 'opaque-owner-1',
        purpose: 'opaque-purpose',
        metadata: { source: 'contract-ledger' },
      },
    })

    expect(result.version).toEqual(version())
    expect(transferApi.createUploadSession).toHaveBeenCalledWith({
      fileName: 'report.pdf',
      sizeBytes: 7,
      mediaType: 'application/pdf',
      sha256: 'a'.repeat(64),
      idempotencyKey: 'id-1',
      clientContext: {
        ownerRef: 'opaque-owner-1',
        purpose: 'opaque-purpose',
        metadata: { source: 'contract-ledger' },
      },
    })
    expect(transferApi.completeUpload).toHaveBeenCalledWith('session-1', {
      idempotencyKey: 'id-2',
      observedSizeBytes: 7,
      observedSha256: 'a'.repeat(64),
      etag: 'etag-1',
    })
    expect(transferApi.abortUpload).not.toHaveBeenCalled()
  })

  it('retries a retryable complete call with the same idempotency key', async () => {
    const completeUpload = vi
      .fn<FileTransferApi['completeUpload']>()
      .mockRejectedValueOnce(
        new FileTransferError('http_error', 'temporary', { retryable: true, status: 503 }),
      )
      .mockResolvedValue(version())
    const transferApi = api({ completeUpload })
    const client = new FileTransferClient({
      api: transferApi,
      uploader,
      createId: sequentialIds(),
      config: { completeRetryDelaysMs: [0, 0] },
    })

    await client.uploadFile({ file: new Blob(['content']), fileName: 'report.pdf' })

    expect(completeUpload).toHaveBeenCalledTimes(2)
    expect(completeUpload.mock.calls[0]?.[1].idempotencyKey).toBe('id-2')
    expect(completeUpload.mock.calls[1]?.[1].idempotencyKey).toBe('id-2')
  })

  it('best-effort aborts the server session after transfer failure', async () => {
    const transferApi = api()
    const failingUploader: DirectUploader = {
      upload: vi.fn(async () => {
        throw new Error('network lost')
      }),
    }
    const client = new FileTransferClient({
      api: transferApi,
      uploader: failingUploader,
      createId: sequentialIds(),
    })

    await expect(
      client.uploadFile({ file: new Blob(['content']), fileName: 'report.pdf' }),
    ).rejects.toMatchObject({ code: 'upload_error' })
    expect(transferApi.abortUpload).toHaveBeenCalledWith('session-1', {
      idempotencyKey: 'id-3',
      reason: 'transfer_failed',
    })
  })

  it('also aborts the server session when the user cancels', async () => {
    const controller = new AbortController()
    const transferApi = api()
    const cancellingUploader: DirectUploader = {
      upload: vi.fn(async () => {
        controller.abort()
        throw new DOMException('cancelled', 'AbortError')
      }),
    }
    const client = new FileTransferClient({
      api: transferApi,
      uploader: cancellingUploader,
      createId: sequentialIds(),
    })

    await expect(
      client.uploadFile({
        file: new Blob(['content']),
        fileName: 'report.pdf',
        signal: controller.signal,
      }),
    ).rejects.toMatchObject({ code: 'aborted' })
    expect(transferApi.abortUpload).toHaveBeenCalledWith('session-1', {
      idempotencyKey: 'id-3',
      reason: 'user_cancelled',
    })
  })
})

describe('FileTransferClient access and preview', () => {
  const smallPdf: FileDescriptor = {
    fileId: 'file-1',
    version: 1,
    fileName: 'report.pdf',
    sizeBytes: 1024,
    mediaType: 'application/pdf',
  }

  it('isolates cached grants by caller-provided identity scope', async () => {
    let scope = 'tenant-a:user-a'
    const createAccessGrant = vi.fn(async () => accessGrant())
    const client = new FileTransferClient({
      api: api({ createAccessGrant }),
      uploader,
      getCacheScope: () => scope,
    })

    await client.resolvePreviewUrl(smallPdf, 'direct')
    await client.resolvePreviewUrl(smallPdf, 'direct')
    scope = 'tenant-a:user-b'
    await client.resolvePreviewUrl(smallPdf, 'direct')

    expect(createAccessGrant).toHaveBeenCalledTimes(2)
  })

  it('does not cache access grants when no identity scope is supplied', async () => {
    const createAccessGrant = vi.fn(async () => accessGrant())
    const client = new FileTransferClient({ api: api({ createAccessGrant }), uploader })

    await client.resolvePreviewUrl(smallPdf, 'direct')
    await client.resolvePreviewUrl(smallPdf, 'direct')

    expect(createAccessGrant).toHaveBeenCalledTimes(2)
  })

  it('uses Blob preview for small files and revokes replaced object URLs', async () => {
    const create = vi.fn(() => 'blob:preview')
    const revoke = vi.fn()
    const fetcher = vi.fn(async () => new Response(new Blob(['pdf']), { status: 200 }))
    const client = new FileTransferClient({
      api: api({
        createAccessGrant: vi.fn(async () =>
          accessGrant({ deliveryMode: 'blob' }),
        ),
      }),
      uploader,
      fetcher,
      objectUrls: { create, revoke },
      getCacheScope: () => 'tenant-a:user-a',
    })

    expect(await client.resolvePreviewUrl(smallPdf)).toBe('blob:preview')
    client.revokeFile(smallPdf)

    expect(fetcher).toHaveBeenCalledWith('https://evidence.example/signed', { method: 'GET' })
    expect(revoke).toHaveBeenCalledWith('blob:preview')
  })

  it('routes medium PDFs to direct preview and large images to download', () => {
    const client = new FileTransferClient({ api: api(), uploader })
    expect(
      client.getPreviewPolicy({
        ...smallPdf,
        sizeBytes: 100 * 1024 * 1024,
      }),
    ).toMatchObject({ mode: 'direct', reason: 'memory-threshold' })
    expect(
      client.getPreviewPolicy({
        ...smallPdf,
        fileName: 'large.png',
        mediaType: 'image/png',
        sizeBytes: 100 * 1024 * 1024,
      }),
    ).toMatchObject({ mode: 'download', reason: 'large-image' })
  })
})

function sequentialIds(): () => string {
  let value = 0
  return () => `id-${++value}`
}
