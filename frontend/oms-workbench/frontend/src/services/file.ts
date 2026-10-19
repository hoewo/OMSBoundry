import type { File, FileCategory } from '../types'
import type { FilePreviewDeliveryMode, FileTransferFile, FilePreviewPolicy } from '../lib/file-transfer'
import { adapterFor, fileAdapter, costForProject } from './registry'
import { DEFAULT_MAX_BLOB_PREVIEW_BYTES, DEFAULT_MAX_DIRECT_PREVIEW_BYTES } from '../../../../../packages/file-transfer-core/src/index'
export const fileService = {
  uploadCostFile: async(projectID:string,scope:string,file:globalThis.File):Promise<File> => costForProject(projectID,scope).upload(file),
  uploadFile: async (projectID: string, file: globalThis.File, category: FileCategory, _title: string): Promise<File> => adapterFor(projectID).upload(projectID, file, category),
  downloadFile: async (viewID: string, name?: string) => {
    const adapter=fileAdapter(viewID)
    await adapter.transfer.downloadFile(adapter.descriptor(viewID),name)
  },
  getPreviewPolicy: (file?: FileTransferFile): FilePreviewPolicy => {
    const size=file?.file_size||0, mime=file?.mime_type||'', name=file?.original_name||file?.file_name||''
    const pdf=mime.includes('pdf')||/\.pdf$/i.test(name), image=mime.startsWith('image/')||/\.(png|jpg|jpeg|webp|gif|bmp)$/i.test(name)
    const base={fileSize:size,maxBlobSize:DEFAULT_MAX_BLOB_PREVIEW_BYTES,maxDirectSize:DEFAULT_MAX_DIRECT_PREVIEW_BYTES}
    if(!pdf&&!image)return {...base,mode:'download',reason:'unsupported'}
    if(size>base.maxDirectSize)return {...base,mode:'download',reason:'size-limit'}
    if(size>base.maxBlobSize)return {...base,mode:pdf?'direct':'download',reason:pdf?'memory-threshold':'large-image'}
    return {...base,mode:'blob'}
  },
  resolveTemporaryUrl: async (viewID: string, _file: File, mode: FilePreviewDeliveryMode): Promise<string> => {
    const adapter=fileAdapter(viewID)
    return adapter.transfer.resolvePreviewUrl(adapter.descriptor(viewID),mode)
  },
  revokeTemporaryUrl: (viewID: string) => {const adapter=fileAdapter(viewID);adapter.transfer.revokeFile(adapter.descriptor(viewID))},
}
