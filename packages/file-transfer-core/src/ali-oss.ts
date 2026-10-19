import OSS from 'ali-oss'
import { FileTransferError } from './errors.js'
import type {
  AliyunOssUploadAuthorization,
  CompletedPart,
  DirectUploader,
  UploadProgress,
} from './types.js'

const REQUIRED_MULTIPART_ACTIONS = new Set([
  'oss:PutObject',
  'oss:AbortMultipartUpload',
  'oss:ListParts',
])

interface AliOssCheckpoint {
  uploadId?: string
  doneParts?: Array<{ number?: number; partNumber?: number; etag?: string; size?: number }>
}

interface AliOssResult {
  res?: { status?: number; headers?: Record<string, string> }
}

interface AliOssClient {
  multipartUpload(
    objectKey: string,
    file: Blob,
    options: {
      partSize: number
      parallel: number
      headers?: Record<string, string>
      progress: (fraction: number, checkpoint: unknown) => void
    },
  ): Promise<AliOssResult>
  abortMultipartUpload(objectKey: string, uploadId: string): Promise<unknown>
  cancel?: () => void
}

export interface AliOssMultipartUploaderOptions {
  allowInsecureEndpoint?: boolean
  expirySkewMs?: number
  now?: () => number
}

export function createAliOssMultipartUploader(
  options: AliOssMultipartUploaderOptions = {},
): DirectUploader {
  const now = options.now ?? Date.now
  const expirySkewMs = options.expirySkewMs ?? 30_000

  return {
    async upload(input, uploadOptions = {}) {
      const authorization = input.session.authorization
      if (authorization.provider !== 'aliyun-oss') {
        throw new FileTransferError('contract_error', 'Upload authorization is not for Aliyun OSS.')
      }
      validateAuthorization(authorization, now(), expirySkewMs, options.allowInsecureEndpoint === true)
      const Client = OSS as unknown as new (options: Record<string, unknown>) => AliOssClient
      const client = new Client({
        region: authorization.region,
        endpoint: authorization.endpoint,
        cname: authorization.endpointIsCName === true,
        bucket: authorization.bucket,
        accessKeyId: authorization.accessKeyId,
        accessKeySecret: authorization.accessKeySecret,
        stsToken: authorization.securityToken,
        secure: authorization.endpoint.startsWith('https://'),
        authorizationV4: true,
      })
      let checkpoint: AliOssCheckpoint | undefined
      let abortReject: ((error: Error) => void) | undefined
      const cancelUpload = async (): Promise<void> => {
        client.cancel?.()
        if (checkpoint?.uploadId) {
          await client
            .abortMultipartUpload(authorization.objectKey, checkpoint.uploadId)
            .catch(() => undefined)
        }
      }
      const onAbort = (): void => {
        void cancelUpload()
        abortReject?.(new DOMException('File transfer was cancelled.', 'AbortError'))
      }
      if (uploadOptions.signal?.aborted) onAbort()
      uploadOptions.signal?.addEventListener('abort', onAbort, { once: true })

      const abortPromise = new Promise<never>((_, reject) => {
        abortReject = reject
        if (uploadOptions.signal?.aborted) onAbort()
      })
      const uploadPromise = client.multipartUpload(authorization.objectKey, input.file, {
        partSize: authorization.multipart.partSizeBytes,
        parallel: authorization.multipart.parallel,
        ...(authorization.headers ? { headers: authorization.headers } : {}),
        progress: (fraction, nextCheckpoint) => {
          checkpoint = nextCheckpoint as AliOssCheckpoint
          const bounded = Math.max(0, Math.min(1, fraction))
          const progress: UploadProgress = {
            fraction: bounded,
            percent: Math.round(bounded * 100),
            loadedBytes: Math.round(input.file.size * bounded),
            totalBytes: input.file.size,
            checkpoint: nextCheckpoint,
          }
          uploadOptions.onProgress?.(progress)
        },
      })

      try {
        const result = await Promise.race([uploadPromise, abortPromise])
        const status = result.res?.status
        if (status !== undefined && (status < 200 || status >= 300)) {
          throw new FileTransferError('upload_error', `Aliyun OSS returned HTTP ${status}.`, {
            status,
            retryable: status >= 500,
          })
        }
        return {
          ...(result.res?.headers?.etag
            ? { etag: result.res.headers.etag.replaceAll('"', '') }
            : {}),
          ...(checkpoint?.doneParts ? { parts: normalizeParts(checkpoint.doneParts) } : {}),
        }
      } catch (error) {
        if (!uploadOptions.signal?.aborted) await cancelUpload()
        throw error
      } finally {
        abortReject = undefined
        uploadOptions.signal?.removeEventListener('abort', onAbort)
      }
    },
  }
}

function validateAuthorization(
  authorization: AliyunOssUploadAuthorization,
  now: number,
  expirySkewMs: number,
  allowInsecureEndpoint: boolean,
): void {
  if (!allowInsecureEndpoint && !authorization.endpoint.startsWith('https://')) {
    throw new FileTransferError('contract_error', 'Aliyun OSS endpoint must use HTTPS.')
  }
  if (!authorization.securityToken) {
    throw new FileTransferError('contract_error', 'A temporary OSS security token is required.')
  }
  if (Date.parse(authorization.expiresAt) - expirySkewMs <= now) {
    throw new FileTransferError('expired_authorization', 'OSS upload authorization has expired.')
  }
  if (
    authorization.scope.bucket !== authorization.bucket ||
    authorization.scope.objectKey !== authorization.objectKey
  ) {
    throw new FileTransferError(
      'contract_error',
      'OSS authorization scope must match one exact bucket and object key.',
    )
  }
  const granted = new Set(authorization.scope.allowedActions)
  for (const action of REQUIRED_MULTIPART_ACTIONS) {
    if (!granted.has(action)) {
      throw new FileTransferError('contract_error', `OSS authorization is missing ${action}.`)
    }
  }
  for (const action of granted) {
    if (!REQUIRED_MULTIPART_ACTIONS.has(action)) {
      throw new FileTransferError(
        'contract_error',
        `OSS authorization contains unnecessary action ${action}.`,
      )
    }
  }
  if (
    authorization.multipart.partSizeBytes <= 0 ||
    authorization.multipart.parallel <= 0 ||
    authorization.multipart.parallel > 16
  ) {
    throw new FileTransferError('contract_error', 'Invalid multipart upload limits.')
  }
}

function normalizeParts(parts: NonNullable<AliOssCheckpoint['doneParts']>): CompletedPart[] {
  return parts.flatMap((part) => {
    const partNumber = part.partNumber ?? part.number
    if (!partNumber || !part.etag) return []
    return [
      {
        partNumber,
        etag: part.etag.replaceAll('"', ''),
        ...(part.size !== undefined ? { sizeBytes: part.size } : {}),
      },
    ]
  })
}
