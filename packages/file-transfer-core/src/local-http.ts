import { FileTransferError } from './errors.js'
import type { DirectUploader, LocalHttpUploadAuthorization } from './types.js'

export interface LocalHttpUploaderOptions {
  fetcher?: typeof fetch
  allowInsecureUrl?: boolean
  expirySkewMs?: number
  now?: () => number
}

export function createLocalHttpUploader(options: LocalHttpUploaderOptions = {}): DirectUploader {
  const fetcher = options.fetcher ?? fetch
  const now = options.now ?? Date.now
  const expirySkewMs = options.expirySkewMs ?? 30_000
  return {
    async upload(input, uploadOptions = {}) {
      const authorization = input.session.authorization
      if (authorization.provider !== 'local-http') {
        throw new FileTransferError('contract_error', 'Upload authorization is not for local HTTP.')
      }
      validateAuthorization(
        authorization,
        now(),
        expirySkewMs,
        options.allowInsecureUrl === true,
      )
      uploadOptions.onProgress?.({
        fraction: 0,
        percent: 0,
        loadedBytes: 0,
        totalBytes: input.file.size,
      })
      let response: Response
      try {
        response = await fetcher(authorization.uploadUrl, {
          method: authorization.method,
          body: input.file,
          ...(authorization.headers ? { headers: authorization.headers } : {}),
          ...(uploadOptions.signal ? { signal: uploadOptions.signal } : {}),
        })
      } catch (error) {
        if (uploadOptions.signal?.aborted) {
          throw new DOMException('File transfer was cancelled.', 'AbortError')
        }
        throw new FileTransferError('network_error', 'Local evidence upload failed.', {
          retryable: true,
          cause: error,
        })
      }
      if (!response.ok) {
        throw new FileTransferError('upload_error', `Local evidence upload returned HTTP ${response.status}.`, {
          status: response.status,
          retryable: response.status === 408 || response.status === 429 || response.status >= 500,
        })
      }
      uploadOptions.onProgress?.({
        fraction: 1,
        percent: 100,
        loadedBytes: input.file.size,
        totalBytes: input.file.size,
      })
      const etag = response.headers.get('ETag')?.replaceAll('"', '')
      return etag ? { etag } : {}
    },
  }
}

function validateAuthorization(
  authorization: LocalHttpUploadAuthorization,
  now: number,
  expirySkewMs: number,
  allowInsecureUrl: boolean,
): void {
  const url = new URL(authorization.uploadUrl)
  const localDevelopment = url.hostname === '127.0.0.1' || url.hostname === 'localhost'
  if (!allowInsecureUrl && url.protocol !== 'https:' && !localDevelopment) {
    throw new FileTransferError('contract_error', 'Local HTTP upload URL must use HTTPS.')
  }
  if (Date.parse(authorization.expiresAt) - expirySkewMs <= now) {
    throw new FileTransferError('expired_authorization', 'Local upload authorization has expired.')
  }
  if (
    authorization.objectKey !== authorization.scope.objectKey ||
    authorization.scope.allowedActions.length !== 1 ||
    authorization.scope.allowedActions[0] !== 'evidence:PutObject'
  ) {
    throw new FileTransferError(
      'contract_error',
      'Local upload authorization must be bound to one object and one write action.',
    )
  }
}

export function createDirectUploaderRouter(uploaders: {
  aliyunOss: DirectUploader
  localHttp: DirectUploader
}): DirectUploader {
  return {
    upload(input, options) {
      switch (input.session.authorization.provider) {
        case 'aliyun-oss':
          return uploaders.aliyunOss.upload(input, options)
        case 'local-http':
          return uploaders.localHttp.upload(input, options)
      }
    },
  }
}
