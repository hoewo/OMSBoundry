export type FileTransferErrorCode =
  | 'aborted'
  | 'contract_error'
  | 'expired_authorization'
  | 'http_error'
  | 'invalid_input'
  | 'network_error'
  | 'preview_not_allowed'
  | 'upload_error'

export class FileTransferError extends Error {
  readonly code: FileTransferErrorCode
  readonly retryable: boolean
  readonly status: number | undefined
  override readonly cause: unknown

  constructor(
    code: FileTransferErrorCode,
    message: string,
    options: { retryable?: boolean; status?: number; cause?: unknown } = {},
  ) {
    super(message)
    this.name = 'FileTransferError'
    this.code = code
    this.retryable = options.retryable ?? false
    this.status = options.status
    this.cause = options.cause
  }
}

export function isAbortError(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === 'AbortError') ||
    (error instanceof FileTransferError && error.code === 'aborted') ||
    (error instanceof Error && error.name === 'AbortError')
  )
}

export function normalizeTransferError(error: unknown): FileTransferError {
  if (error instanceof FileTransferError) return error
  if (isAbortError(error)) {
    return new FileTransferError('aborted', 'File transfer was cancelled.', { cause: error })
  }
  if (error instanceof Error) {
    return new FileTransferError('upload_error', error.message, { cause: error })
  }
  return new FileTransferError('upload_error', 'File transfer failed.', { cause: error })
}
