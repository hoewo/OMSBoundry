export type UploadSessionStatus = 'pending' | 'completed' | 'aborted' | 'expired'
export type EvidenceVersionStatus = 'available' | 'quarantined' | 'deleted'
export type AccessDisposition = 'inline' | 'attachment'
export type AccessDeliveryMode = 'blob' | 'direct'
export type PreviewPolicyMode = AccessDeliveryMode | 'download'

export interface OpaqueClientContext {
  ownerRef?: string
  purpose?: string
  metadata?: Record<string, string>
}

export interface MultipartOptions {
  thresholdBytes: number
  partSizeBytes: number
  parallel: number
}

export interface UploadAuthorizationScope {
  bucket?: string
  objectKey: string
  allowedActions: string[]
}

export interface AliyunOssUploadAuthorization {
  provider: 'aliyun-oss'
  region?: string
  endpoint: string
  endpointIsCName?: boolean
  bucket: string
  objectKey: string
  accessKeyId: string
  accessKeySecret: string
  securityToken: string
  expiresAt: string
  multipart: MultipartOptions
  scope: UploadAuthorizationScope
  headers?: Record<string, string>
}

export interface LocalHttpUploadAuthorization {
  provider: 'local-http'
  uploadUrl: string
  method: 'PUT'
  objectKey: string
  expiresAt: string
  headers?: Record<string, string>
  scope: UploadAuthorizationScope
}

export type UploadAuthorization =
  | AliyunOssUploadAuthorization
  | LocalHttpUploadAuthorization

export interface CreateUploadSessionInput {
  fileName: string
  sizeBytes: number
  mediaType?: string
  sha256?: string
  idempotencyKey: string
  clientContext?: OpaqueClientContext
}

export interface UploadSession {
  sessionId: string
  fileId: string
  version: number
  status: UploadSessionStatus
  expiresAt: string
  authorization: UploadAuthorization
}

export interface CompletedPart {
  partNumber: number
  etag: string
  sizeBytes?: number
}

export interface DirectUploadReceipt {
  etag?: string
  parts?: CompletedPart[]
}

export interface CompleteUploadInput {
  idempotencyKey: string
  observedSizeBytes: number
  observedSha256?: string
  etag?: string
  parts?: CompletedPart[]
}

export interface AbortUploadInput {
  idempotencyKey: string
  reason: 'user_cancelled' | 'transfer_failed' | 'client_shutdown'
}

export interface EvidenceVersion {
  fileId: string
  version: number
  status: EvidenceVersionStatus
  fileName: string
  sizeBytes: number
  mediaType: string
  sha256: string
  createdAt: string
}

export interface AccessGrantInput {
  disposition: AccessDisposition
  deliveryMode: AccessDeliveryMode
  fileName?: string
}

export interface AccessGrant {
  fileId: string
  version: number
  url: string
  method: 'GET'
  disposition: AccessDisposition
  deliveryMode: AccessDeliveryMode
  expiresAt: string
  refreshAfter: string
}

export interface FileTransferApi {
  createUploadSession(input: CreateUploadSessionInput): Promise<UploadSession>
  completeUpload(sessionId: string, input: CompleteUploadInput): Promise<EvidenceVersion>
  abortUpload(sessionId: string, input: AbortUploadInput): Promise<void>
  createAccessGrant(fileId: string, version: number, input: AccessGrantInput): Promise<AccessGrant>
}

export interface UploadProgress {
  fraction: number
  percent: number
  loadedBytes: number
  totalBytes: number
  checkpoint?: unknown
}

export interface DirectUploadInput {
  file: Blob
  session: UploadSession
}

export interface DirectUploader {
  upload(
    input: DirectUploadInput,
    options?: {
      signal?: AbortSignal
      onProgress?: (progress: UploadProgress) => void
    },
  ): Promise<DirectUploadReceipt>
}

export interface UploadFileInput {
  file: Blob
  fileName?: string
  mediaType?: string
  sha256?: string
  clientContext?: OpaqueClientContext
  createIdempotencyKey?: string
  completeIdempotencyKey?: string
  abortIdempotencyKey?: string
  signal?: AbortSignal
  onProgress?: (progress: UploadProgress) => void
}

export interface UploadFileResult {
  session: UploadSession
  receipt: DirectUploadReceipt
  version: EvidenceVersion
}

export interface FileDescriptor {
  fileId: string
  version: number
  fileName: string
  sizeBytes: number
  mediaType?: string
}

export type PreviewPolicyReason =
  | 'memory-threshold'
  | 'size-limit'
  | 'large-image'
  | 'unsupported'

export interface PreviewPolicy {
  mode: PreviewPolicyMode
  reason?: PreviewPolicyReason
  sizeBytes: number
  maxBlobBytes: number
  maxDirectBytes: number
}

export interface FileTransferClientConfig {
  completeRetryDelaysMs: number[]
  accessGrantSkewMs: number
  maxCachedAccessGrants: number
  maxCachedFileBytes: number
  maxBlobPreviewBytes: number
  maxDirectPreviewBytes: number
}

export interface ObjectUrlFactory {
  create(blob: Blob): string
  revoke(url: string): void
}

export interface DownloadSink {
  save(url: string, fileName: string): void | Promise<void>
}

export interface FileTransferClientOptions {
  api: FileTransferApi
  uploader: DirectUploader
  config?: Partial<FileTransferClientConfig>
  getCacheScope?: () => string | undefined
  fetcher?: typeof fetch
  objectUrls?: ObjectUrlFactory
  downloadSink?: DownloadSink
  now?: () => number
  createId?: () => string
}

export interface FetchFileTransferApiOptions {
  baseUrl: string
  fetcher?: typeof fetch
  getHeaders?: () => HeadersInit | Promise<HeadersInit>
}
