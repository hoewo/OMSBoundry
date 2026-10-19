import type { File } from '../types'
export type FilePreviewDeliveryMode = 'blob' | 'direct'
export type FileTransferFile = Pick<File, 'original_name' | 'file_name' | 'file_size' | 'mime_type'>
export interface FilePreviewPolicy {
  mode: 'blob' | 'direct' | 'download'
  reason?: string
  fileSize: number
  maxBlobSize: number
  maxDirectSize: number
}
