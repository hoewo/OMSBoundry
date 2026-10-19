import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { UploadSession } from './types.js'

const mockState = vi.hoisted(() => ({
  constructedWith: undefined as Record<string, unknown> | undefined,
  objectKey: undefined as string | undefined,
  cancel: vi.fn(),
  abortMultipartUpload: vi.fn(async () => undefined),
  uploadImpl: undefined as
    | ((
        objectKey: string,
        file: Blob,
        options: { progress: (fraction: number, checkpoint: unknown) => void },
      ) => Promise<unknown>)
    | undefined,
}))

vi.mock('ali-oss', () => ({
  default: class MockAliOss {
    constructor(options: Record<string, unknown>) {
      mockState.constructedWith = options
    }

    cancel = mockState.cancel
    abortMultipartUpload = mockState.abortMultipartUpload

    multipartUpload(
      objectKey: string,
      file: Blob,
      options: { progress: (fraction: number, checkpoint: unknown) => void },
    ) {
      mockState.objectKey = objectKey
      return mockState.uploadImpl?.(objectKey, file, options)
    }
  },
}))

import { createAliOssMultipartUploader } from './ali-oss.js'

const actions = [
  'oss:PutObject',
  'oss:AbortMultipartUpload',
  'oss:ListParts',
]

function session(extraActions: string[] = []): UploadSession {
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
      bucket: 'private-bucket',
      objectKey: 'tenant/t1/file-1/v1',
      accessKeyId: 'STS.example',
      accessKeySecret: 'temporary-secret',
      securityToken: 'temporary-token',
      expiresAt: '2030-01-01T00:00:00Z',
      multipart: {
        thresholdBytes: 20,
        partSizeBytes: 8,
        parallel: 3,
      },
      scope: {
        bucket: 'private-bucket',
        objectKey: 'tenant/t1/file-1/v1',
        allowedActions: [...actions, ...extraActions],
      },
    },
  }
}

describe('Aliyun OSS multipart adapter', () => {
  beforeEach(() => {
    mockState.constructedWith = undefined
    mockState.objectKey = undefined
    mockState.cancel.mockClear()
    mockState.abortMultipartUpload.mockClear()
    mockState.uploadImpl = async (_objectKey, _file, options) => {
      options.progress(1, {
        uploadId: 'upload-1',
        doneParts: [{ partNumber: 1, etag: '"part-etag"', size: 7 }],
      })
      return { res: { status: 200, headers: { etag: '"object-etag"' } } }
    }
  })

  it('uploads only to the exact object key supplied by the server', async () => {
    const progress = vi.fn()
    const uploader = createAliOssMultipartUploader({ now: () => 0 })

    const receipt = await uploader.upload(
      { file: new Blob(['content']), session: session() },
      { onProgress: progress },
    )

    expect(mockState.objectKey).toBe('tenant/t1/file-1/v1')
    expect(mockState.constructedWith).toMatchObject({
      bucket: 'private-bucket',
      stsToken: 'temporary-token',
      secure: true,
    })
    expect(receipt).toEqual({
      etag: 'object-etag',
      parts: [{ partNumber: 1, etag: 'part-etag', sizeBytes: 7 }],
    })
    expect(progress).toHaveBeenCalledWith(
      expect.objectContaining({ fraction: 1, percent: 100, loadedBytes: 7 }),
    )
  })

  it('rejects credentials that contain broader actions than multipart upload needs', async () => {
    const uploader = createAliOssMultipartUploader({ now: () => 0 })

    await expect(
      uploader.upload({ file: new Blob(['content']), session: session(['oss:ListObjects']) }),
    ).rejects.toMatchObject({ code: 'contract_error' })
    expect(mockState.constructedWith).toBeUndefined()
  })

  it('uses the complete custom-domain endpoint only when explicitly authorized', async () => {
    const custom = session()
    if (custom.authorization.provider !== 'aliyun-oss') throw new Error('fixture provider')
    custom.authorization.endpoint = 'https://files.example.test'
    custom.authorization.endpointIsCName = true
    await createAliOssMultipartUploader({ now: () => 0 }).upload({ file: new Blob(['content']), session: custom })
    expect(mockState.constructedWith).toMatchObject({ endpoint: 'https://files.example.test', cname: true, bucket: 'private-bucket' })
    custom.authorization.endpointIsCName = false
    await createAliOssMultipartUploader({ now: () => 0 }).upload({ file: new Blob(['content']), session: custom })
    expect(mockState.constructedWith?.cname).toBe(false)
  })

  it('aborts the OSS multipart upload when the browser signal is cancelled', async () => {
    mockState.uploadImpl = async () => new Promise(() => undefined)
    const uploader = createAliOssMultipartUploader({ now: () => 0 })
    const controller = new AbortController()
    const pending = uploader.upload(
      { file: new Blob(['content']), session: session() },
      { signal: controller.signal },
    )
    controller.abort()

    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    expect(mockState.cancel).toHaveBeenCalledOnce()
  })
})
