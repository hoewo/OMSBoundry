import { AccessGrantCache } from './cache.js'
import {
  FileTransferError,
  isAbortError,
  normalizeTransferError,
} from './errors.js'
import type {
  AccessDeliveryMode,
  AccessDisposition,
  AccessGrant,
  DownloadSink,
  EvidenceVersion,
  FileDescriptor,
  FileTransferClientConfig,
  FileTransferClientOptions,
  ObjectUrlFactory,
  PreviewPolicy,
  UploadFileInput,
  UploadFileResult,
  UploadSession,
} from './types.js'

export const DEFAULT_MAX_BLOB_PREVIEW_BYTES = 50 * 1024 * 1024
export const DEFAULT_MAX_DIRECT_PREVIEW_BYTES = 200 * 1024 * 1024

const DEFAULT_CONFIG: FileTransferClientConfig = {
  completeRetryDelaysMs: [0, 500, 1_000, 2_000],
  accessGrantSkewMs: 30_000,
  maxCachedAccessGrants: 8,
  maxCachedFileBytes: 15 * 1024 * 1024,
  maxBlobPreviewBytes: DEFAULT_MAX_BLOB_PREVIEW_BYTES,
  maxDirectPreviewBytes: DEFAULT_MAX_DIRECT_PREVIEW_BYTES,
}

const IMAGE_EXTENSIONS = new Set(['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp'])

export class FileTransferClient {
  private readonly config: FileTransferClientConfig
  private readonly cache: AccessGrantCache
  private readonly objectUrls: ObjectUrlFactory
  private readonly downloadSink: DownloadSink
  private readonly fetcher: typeof fetch
  private readonly now: () => number
  private readonly createId: () => string
  private readonly previewObjectUrls = new Map<string, string>()

  constructor(private readonly options: FileTransferClientOptions) {
    this.config = { ...DEFAULT_CONFIG, ...options.config }
    if (this.config.completeRetryDelaysMs.length === 0) {
      this.config.completeRetryDelaysMs = [0]
    }
    this.now = options.now ?? Date.now
    // Calling native Window.fetch as this.fetcher gives it the SDK instance
    // as receiver; real browsers reject that with Illegal invocation.
    const fetcher = options.fetcher ?? globalThis.fetch
    this.fetcher = (input, init) => fetcher.call(globalThis, input, init)
    this.objectUrls = options.objectUrls ?? browserObjectUrls()
    this.downloadSink = options.downloadSink ?? browserDownloadSink()
    this.createId = options.createId ?? defaultCreateId
    this.cache = new AccessGrantCache(
      this.config.maxCachedAccessGrants,
      this.config.maxCachedFileBytes,
      this.config.accessGrantSkewMs,
      this.now,
    )
  }

  async uploadFile(input: UploadFileInput): Promise<UploadFileResult> {
    validateUploadInput(input)
    ensureNotAborted(input.signal)
    const createIdempotencyKey = input.createIdempotencyKey ?? this.createId()
    const completeIdempotencyKey = input.completeIdempotencyKey ?? this.createId()
    const abortIdempotencyKey = input.abortIdempotencyKey ?? this.createId()
    const fileName = inferFileName(input.file, input.fileName)
    const mediaType = input.mediaType ?? (input.file.type || 'application/octet-stream')
    let session: UploadSession | undefined

    try {
      const createInput = {
        fileName,
        sizeBytes: input.file.size,
        mediaType,
        idempotencyKey: createIdempotencyKey,
        ...(input.sha256 ? { sha256: input.sha256 } : {}),
        ...(input.clientContext ? { clientContext: input.clientContext } : {}),
      }
      session = await this.options.api.createUploadSession(createInput)
      validateSessionForUpload(session, input.file.size, this.now())
      ensureNotAborted(input.signal)

      const uploadOptions = {
        ...(input.signal ? { signal: input.signal } : {}),
        ...(input.onProgress ? { onProgress: input.onProgress } : {}),
      }
      const receipt = await this.options.uploader.upload(
        { file: input.file, session },
        uploadOptions,
      )
      ensureNotAborted(input.signal)
      const version = await this.completeWithRetry(
        session,
        {
          idempotencyKey: completeIdempotencyKey,
          observedSizeBytes: input.file.size,
          ...(input.sha256 ? { observedSha256: input.sha256 } : {}),
          ...(receipt.etag ? { etag: receipt.etag } : {}),
          ...(receipt.parts ? { parts: receipt.parts } : {}),
        },
        input.signal,
      )
      return { session, receipt, version }
    } catch (error) {
      if (session) {
        await this.options.api
          .abortUpload(session.sessionId, {
            idempotencyKey: abortIdempotencyKey,
            reason: isAbortError(error) || input.signal?.aborted ? 'user_cancelled' : 'transfer_failed',
          })
          .catch(() => undefined)
      }
      throw normalizeTransferError(error)
    }
  }

  getPreviewPolicy(file: FileDescriptor): PreviewPolicy {
    const base = {
      sizeBytes: file.sizeBytes,
      maxBlobBytes: this.config.maxBlobPreviewBytes,
      maxDirectBytes: this.config.maxDirectPreviewBytes,
    }
    if (!isPdf(file) && !isImage(file)) {
      return { ...base, mode: 'download', reason: 'unsupported' }
    }
    if (file.sizeBytes > this.config.maxDirectPreviewBytes) {
      return { ...base, mode: 'download', reason: 'size-limit' }
    }
    if (file.sizeBytes > this.config.maxBlobPreviewBytes) {
      if (isPdf(file)) return { ...base, mode: 'direct', reason: 'memory-threshold' }
      return { ...base, mode: 'download', reason: 'large-image' }
    }
    return { ...base, mode: 'blob' }
  }

  async resolvePreviewUrl(
    file: FileDescriptor,
    requestedMode?: AccessDeliveryMode,
  ): Promise<string> {
    const policy = this.getPreviewPolicy(file)
    const mode = requestedMode ?? (policy.mode === 'download' ? undefined : policy.mode)
    if (!mode || (mode === 'blob' && policy.mode !== 'blob') || policy.mode === 'download') {
      throw new FileTransferError(
        'preview_not_allowed',
        previewPolicyMessage(policy),
      )
    }

    const grant = await this.resolveAccessGrant(file, 'inline', mode)
    if (mode === 'direct') return grant.url

    const response = await this.fetcher(grant.url, { method: grant.method })
    if (!response.ok) {
      throw new FileTransferError(
        'http_error',
        `Preview download failed with HTTP ${response.status}.`,
        { status: response.status, retryable: response.status >= 500 },
      )
    }
    const objectUrl = this.objectUrls.create(await response.blob())
    const key = fileKey(file)
    const previous = this.previewObjectUrls.get(key)
    if (previous) this.objectUrls.revoke(previous)
    this.previewObjectUrls.set(key, objectUrl)
    return objectUrl
  }

  async downloadFile(file: FileDescriptor, fileName = file.fileName): Promise<void> {
    const grant = await this.resolveAccessGrant(file, 'attachment', 'direct', fileName)
    await this.downloadSink.save(grant.url, fileName)
  }

  revokeFile(file: FileDescriptor): void {
    const identity = `${file.fileId}:${file.version}:`
    this.cache.deleteMatching((key) => key.includes(`:${identity}`))
    const key = fileKey(file)
    const objectUrl = this.previewObjectUrls.get(key)
    if (objectUrl) this.objectUrls.revoke(objectUrl)
    this.previewObjectUrls.delete(key)
  }

  clearCache(): void {
    this.cache.clear()
    for (const objectUrl of this.previewObjectUrls.values()) this.objectUrls.revoke(objectUrl)
    this.previewObjectUrls.clear()
  }

  private async completeWithRetry(
    session: UploadSession,
    input: Parameters<FileTransferClientOptions['api']['completeUpload']>[1],
    signal?: AbortSignal,
  ): Promise<EvidenceVersion> {
    let lastError: unknown
    for (const delayMs of this.config.completeRetryDelaysMs) {
      ensureNotAborted(signal)
      if (delayMs > 0) await delay(delayMs, signal)
      try {
        return await this.options.api.completeUpload(session.sessionId, input)
      } catch (error) {
        lastError = error
        if (!(error instanceof FileTransferError) || !error.retryable) throw error
      }
    }
    throw lastError ?? new FileTransferError('upload_error', 'Upload completion failed.')
  }

  private async resolveAccessGrant(
    file: FileDescriptor,
    disposition: AccessDisposition,
    deliveryMode: AccessDeliveryMode,
    fileName?: string,
  ): Promise<AccessGrant> {
    const scope = this.options.getCacheScope?.()
    const cacheKey = scope
      ? `${scope}:${file.fileId}:${file.version}:${disposition}:${deliveryMode}`
      : undefined
    if (cacheKey) {
      const cached = this.cache.get(cacheKey)
      if (cached) return cached
    }
    const grant = await this.options.api.createAccessGrant(file.fileId, file.version, {
      disposition,
      deliveryMode,
      ...(fileName ? { fileName } : {}),
    })
    validateAccessGrant(grant, file, disposition, deliveryMode, this.now())
    if (cacheKey) this.cache.set(cacheKey, grant, file.sizeBytes)
    return grant
  }
}

function validateUploadInput(input: UploadFileInput): void {
  if (!(input.file instanceof Blob)) {
    throw new FileTransferError('invalid_input', 'file must be a Blob or File.')
  }
  if (input.file.size <= 0) {
    throw new FileTransferError('invalid_input', 'Empty files are not accepted.')
  }
  if (input.sha256 && !/^[a-f0-9]{64}$/i.test(input.sha256)) {
    throw new FileTransferError('invalid_input', 'sha256 must contain 64 hexadecimal characters.')
  }
}

function inferFileName(file: Blob, explicit?: string): string {
  const candidate = explicit ?? ('name' in file && typeof file.name === 'string' ? file.name : '')
  const fileName = candidate.trim()
  if (!fileName || fileName === '.' || fileName === '..' || /[/\\]/.test(fileName)) {
    throw new FileTransferError('invalid_input', 'A plain file name is required.')
  }
  return fileName
}

function validateSessionForUpload(session: UploadSession, expectedSize: number, now: number): void {
  if (session.status !== 'pending') {
    throw new FileTransferError('contract_error', 'Created upload session is not pending.')
  }
  if (Date.parse(session.expiresAt) <= now) {
    throw new FileTransferError('expired_authorization', 'Upload session has expired.')
  }
  if (!session.authorization.objectKey || session.authorization.objectKey !== session.authorization.scope.objectKey) {
    throw new FileTransferError('contract_error', 'Upload authorization is not bound to one object key.')
  }
  if (
    session.authorization.provider === 'aliyun-oss' &&
    session.authorization.bucket !== session.authorization.scope.bucket
  ) {
    throw new FileTransferError('contract_error', 'Upload authorization bucket scope does not match.')
  }
  if (expectedSize <= 0) {
    throw new FileTransferError('invalid_input', 'Upload size must be positive.')
  }
}

function validateAccessGrant(
  grant: AccessGrant,
  file: FileDescriptor,
  disposition: AccessDisposition,
  deliveryMode: AccessDeliveryMode,
  now: number,
): void {
  if (
    grant.fileId !== file.fileId ||
    grant.version !== file.version ||
    grant.disposition !== disposition ||
    grant.deliveryMode !== deliveryMode ||
    grant.method !== 'GET'
  ) {
    throw new FileTransferError('contract_error', 'Access grant does not match the requested file.')
  }
  if (Date.parse(grant.expiresAt) <= now) {
    throw new FileTransferError('contract_error', 'Evidence service returned an expired access grant.')
  }
}

function ensureNotAborted(signal?: AbortSignal): void {
  if (signal?.aborted) throw new DOMException('File transfer was cancelled.', 'AbortError')
}

function delay(milliseconds: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const onAbort = (): void => {
      clearTimeout(timer)
      reject(new DOMException('File transfer was cancelled.', 'AbortError'))
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, milliseconds)
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

function fileKey(file: Pick<FileDescriptor, 'fileId' | 'version'>): string {
  return `${file.fileId}:${file.version}`
}

function extension(fileName: string): string {
  return fileName.split('.').pop()?.toLowerCase() ?? ''
}

function isPdf(file: FileDescriptor): boolean {
  return file.mediaType?.toLowerCase() === 'application/pdf' || extension(file.fileName) === 'pdf'
}

function isImage(file: FileDescriptor): boolean {
  return file.mediaType?.toLowerCase().startsWith('image/') === true || IMAGE_EXTENSIONS.has(extension(file.fileName))
}

function previewPolicyMessage(policy: PreviewPolicy): string {
  switch (policy.reason) {
    case 'large-image':
      return 'Large images must be downloaded to avoid excessive browser memory use.'
    case 'size-limit':
      return 'The file exceeds the configured direct preview limit and must be downloaded.'
    case 'unsupported':
      return 'This file type is not eligible for browser preview.'
    default:
      return 'The file is not eligible for the requested preview mode.'
  }
}

function browserObjectUrls(): ObjectUrlFactory {
  return {
    create: (blob) => URL.createObjectURL(blob),
    revoke: (url) => URL.revokeObjectURL(url),
  }
}

function browserDownloadSink(): DownloadSink {
  return {
    save(url, fileName) {
      if (typeof document === 'undefined') {
        throw new FileTransferError('invalid_input', 'A download sink is required outside a browser.')
      }
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = fileName
      anchor.rel = 'noopener'
      anchor.style.display = 'none'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
    },
  }
}

function defaultCreateId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  throw new FileTransferError('invalid_input', 'A secure idempotency key generator is required.')
}
