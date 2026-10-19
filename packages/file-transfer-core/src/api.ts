import { FileTransferError } from './errors.js'
import type {
  AbortUploadInput,
  AccessGrant,
  AccessGrantInput,
  CompleteUploadInput,
  CreateUploadSessionInput,
  EvidenceVersion,
  FetchFileTransferApiOptions,
  FileTransferApi,
  UploadAuthorization,
  UploadSession,
} from './types.js'

type JsonObject = Record<string, unknown>

export function createFetchFileTransferApi(options: FetchFileTransferApiOptions): FileTransferApi {
  const baseUrl = options.baseUrl.replace(/\/+$/, '')
  const fetcher = options.fetcher ?? fetch

  const request = async <T>(path: string, init: RequestInit, allowEmpty = false): Promise<T> => {
    const headers = new Headers(await options.getHeaders?.())
    new Headers(init.headers).forEach((value, key) => headers.set(key, value))
    if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
    try {
      const response = await fetcher(baseUrl + path, { ...init, headers })
      if (!response.ok) {
        let detail = response.statusText
        try {
          const payload = (await response.json()) as JsonObject
          const error = asObject(payload.error)
          detail = asOptionalString(error?.message) ?? asOptionalString(payload.message) ?? detail
        } catch {
          // Error bodies are not required to be JSON.
        }
        throw new FileTransferError('http_error', detail || `HTTP ${response.status}`, {
          status: response.status,
          retryable: response.status === 408 || response.status === 429 || response.status >= 500,
        })
      }
      if (allowEmpty || response.status === 204) return undefined as T
      return (await response.json()) as T
    } catch (error) {
      if (error instanceof FileTransferError) throw error
      throw new FileTransferError('network_error', 'Evidence service is unreachable.', {
        retryable: true,
        cause: error,
      })
    }
  }

  return {
    async createUploadSession(input) {
      const payload = await request<JsonObject>('/v1/upload-sessions', {
        method: 'POST',
        headers: { 'Idempotency-Key': input.idempotencyKey },
        body: JSON.stringify({
          file_name: input.fileName,
          size_bytes: input.sizeBytes,
          media_type: input.mediaType,
          sha256: input.sha256,
          client_context: input.clientContext
            ? {
                owner_ref: input.clientContext.ownerRef,
                purpose: input.clientContext.purpose,
                metadata: input.clientContext.metadata,
              }
            : undefined,
        }),
      })
      return parseUploadSession(payload)
    },

    async completeUpload(sessionId, input) {
      const payload = await request<JsonObject>(
        `/v1/upload-sessions/${encodeURIComponent(sessionId)}/complete`,
        {
          method: 'POST',
          headers: { 'Idempotency-Key': input.idempotencyKey },
          body: JSON.stringify({
            observed_size_bytes: input.observedSizeBytes,
            observed_sha256: input.observedSha256,
            etag: input.etag,
            parts: input.parts?.map((part) => ({
              part_number: part.partNumber,
              etag: part.etag,
              size_bytes: part.sizeBytes,
            })),
          }),
        },
      )
      return parseEvidenceVersion(payload)
    },

    async abortUpload(sessionId, input) {
      await request<void>(
        `/v1/upload-sessions/${encodeURIComponent(sessionId)}/abort`,
        {
          method: 'POST',
          headers: { 'Idempotency-Key': input.idempotencyKey },
          body: JSON.stringify({ reason: input.reason }),
        },
        true,
      )
    },

    async createAccessGrant(fileId, version, input) {
      const payload = await request<JsonObject>(
        `/v1/files/${encodeURIComponent(fileId)}/versions/${encodeURIComponent(String(version))}/access-grants`,
        {
          method: 'POST',
          body: JSON.stringify({
            disposition: input.disposition,
            delivery_mode: input.deliveryMode,
            file_name: input.fileName,
          }),
        },
      )
      return parseAccessGrant(payload)
    },
  }
}

function parseUploadSession(payload: JsonObject): UploadSession {
  const authorizationPayload = requiredObject(payload, 'authorization')
  const scopePayload = requiredObject(authorizationPayload, 'scope')
  const provider = requiredString(authorizationPayload, 'provider')
  const objectKey = requiredString(authorizationPayload, 'object_key')
  const scopeBucket = asOptionalString(scopePayload.bucket)
  const region = asOptionalString(authorizationPayload.region)
  const headers = asStringRecord(authorizationPayload.headers)
  const scope = {
    objectKey: requiredString(scopePayload, 'object_key'),
    allowedActions: requiredStringArray(scopePayload, 'allowed_actions'),
    ...(scopeBucket ? { bucket: scopeBucket } : {}),
  }
  let authorization: UploadAuthorization
  if (provider === 'aliyun-oss') {
    const multipartPayload = requiredObject(authorizationPayload, 'multipart')
    authorization = {
      provider,
      endpoint: requiredString(authorizationPayload, 'endpoint'),
      ...(authorizationPayload.endpoint_is_cname === undefined ? {} : {
        endpointIsCName: parseEndpointIsCName(authorizationPayload.endpoint_is_cname),
      }),
      bucket: requiredString(authorizationPayload, 'bucket'),
      objectKey,
      accessKeyId: requiredString(authorizationPayload, 'access_key_id'),
      accessKeySecret: requiredString(authorizationPayload, 'access_key_secret'),
      securityToken: requiredString(authorizationPayload, 'security_token'),
      expiresAt: requiredString(authorizationPayload, 'expires_at'),
      multipart: {
        thresholdBytes: requiredNumber(multipartPayload, 'threshold_bytes'),
        partSizeBytes: requiredNumber(multipartPayload, 'part_size_bytes'),
        parallel: requiredNumber(multipartPayload, 'parallel'),
      },
      scope,
      ...(region ? { region } : {}),
      ...(headers ? { headers } : {}),
    }
  } else if (provider === 'local-http') {
    const method = requiredString(authorizationPayload, 'method')
    if (method !== 'PUT') throw contractError('method', 'PUT')
    authorization = {
      provider,
      uploadUrl: requiredString(authorizationPayload, 'upload_url'),
      method,
      objectKey,
      expiresAt: requiredString(authorizationPayload, 'expires_at'),
      scope,
      ...(headers ? { headers } : {}),
    }
  } else {
    throw new FileTransferError(
      'contract_error',
      `Evidence service returned unsupported upload provider ${provider}.`,
    )
  }
  return {
    sessionId: requiredString(payload, 'session_id'),
    fileId: requiredString(payload, 'file_id'),
    version: requiredNumber(payload, 'version'),
    status: requiredString(payload, 'status') as UploadSession['status'],
    expiresAt: requiredString(payload, 'expires_at'),
    authorization,
  }
}

function parseEndpointIsCName(value: unknown): boolean {
  if (typeof value !== 'boolean') throw contractError('endpoint_is_cname', 'boolean')
  return value
}

function parseEvidenceVersion(payload: JsonObject): EvidenceVersion {
  return {
    fileId: requiredString(payload, 'file_id'),
    version: requiredNumber(payload, 'version'),
    status: requiredString(payload, 'status') as EvidenceVersion['status'],
    fileName: requiredString(payload, 'file_name'),
    sizeBytes: requiredNumber(payload, 'size_bytes'),
    mediaType: requiredString(payload, 'media_type'),
    sha256: requiredString(payload, 'sha256'),
    createdAt: requiredString(payload, 'created_at'),
  }
}

function parseAccessGrant(payload: JsonObject): AccessGrant {
  return {
    fileId: requiredString(payload, 'file_id'),
    version: requiredNumber(payload, 'version'),
    url: requiredString(payload, 'url'),
    method: requiredString(payload, 'method') as 'GET',
    disposition: requiredString(payload, 'disposition') as AccessGrant['disposition'],
    deliveryMode: requiredString(payload, 'delivery_mode') as AccessGrant['deliveryMode'],
    expiresAt: requiredString(payload, 'expires_at'),
    refreshAfter: requiredString(payload, 'refresh_after'),
  }
}

function asObject(value: unknown): JsonObject | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? (value as JsonObject)
    : undefined
}

function requiredObject(payload: JsonObject, key: string): JsonObject {
  const value = asObject(payload[key])
  if (!value) throw contractError(key, 'object')
  return value
}

function requiredString(payload: JsonObject, key: string): string {
  const value = payload[key]
  if (typeof value !== 'string' || value.length === 0) throw contractError(key, 'string')
  return value
}

function requiredNumber(payload: JsonObject, key: string): number {
  const value = payload[key]
  if (typeof value !== 'number' || !Number.isFinite(value)) throw contractError(key, 'number')
  return value
}

function requiredStringArray(payload: JsonObject, key: string): string[] {
  const value = payload[key]
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) {
    throw contractError(key, 'string[]')
  }
  return value
}

function asOptionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

function asStringRecord(value: unknown): Record<string, string> | undefined {
  const object = asObject(value)
  if (!object) return undefined
  const entries = Object.entries(object)
  if (entries.some(([, item]) => typeof item !== 'string')) return undefined
  return Object.fromEntries(entries) as Record<string, string>
}

function contractError(field: string, expected: string): FileTransferError {
  return new FileTransferError(
    'contract_error',
    `Evidence service response field ${field} must be ${expected}.`,
  )
}
