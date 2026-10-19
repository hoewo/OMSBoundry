export const MAX_UPLOAD_SIZE_MB = 500
export const MAX_UPLOAD_SIZE_BYTES = MAX_UPLOAD_SIZE_MB * 1024 * 1024
export const MAX_UPLOAD_SIZE_LABEL = `${MAX_UPLOAD_SIZE_MB}MB`
export const CAD_FILE_ACCEPT = '.dwg,.dxf'

export const isCadFileName = (fileName?: string | null): boolean => {
  const normalized = fileName?.trim().toLowerCase() || ''
  return normalized.endsWith('.dwg') || normalized.endsWith('.dxf')
}

export const isWithinUploadLimit = (
  file?: Pick<globalThis.File, 'size'> | null
) => !!file && file.size <= MAX_UPLOAD_SIZE_BYTES
