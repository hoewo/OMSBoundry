import { useEffect, useState } from 'react'
import { Modal, Typography, message } from 'antd'
import type { UploadFile } from 'antd'
import { fileService } from '@/services/file'
import type { File as ProjectFile } from '@/types'
import type {
  FilePreviewDeliveryMode,
  FilePreviewPolicy,
  FileTransferFile,
} from '@/lib/file-transfer'

export type FilePreviewKind = 'pdf' | 'image' | 'unsupported'

interface UseFilePreviewSourceOptions {
  active?: boolean
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
  previewKind: FilePreviewKind
}

const formatFileSize = (bytes: number): string => {
  const megabytes = bytes / 1024 / 1024
  return `${Math.round(megabytes * 10) / 10}MB`
}

const toLocalPreviewMetadata = (
  localFile: globalThis.File,
  uploadFile?: UploadFile | null
): FileTransferFile => ({
  original_name: uploadFile?.name || localFile.name,
  file_name: uploadFile?.name || localFile.name,
  mime_type: uploadFile?.type || localFile.type,
  file_size: localFile.size,
})

const getDownloadNotice = (
  policy: FilePreviewPolicy,
  previewKind: FilePreviewKind
): string => {
  if (policy.reason === 'large-image' || previewKind === 'image') {
    return `图片较大（${formatFileSize(policy.fileSize)}），为避免解码占用过多内存，请下载后查看原文件。`
  }
  return `文件较大（${formatFileSize(policy.fileSize)}），已超过 ${formatFileSize(policy.maxDirectSize)} 在线预览上限，请下载后查看原文件。`
}

export const useFilePreviewSource = ({
  active = true,
  file,
  uploadFile,
  previewKind,
}: UseFilePreviewSourceOptions) => {
  const [previewUrl, setPreviewUrl] = useState<string>()
  const [loading, setLoading] = useState(false)
  const [previewNotice, setPreviewNotice] = useState<string>()
  const localFile = uploadFile?.originFileObj as globalThis.File | undefined

  useEffect(() => {
    setPreviewUrl(undefined)
    setPreviewNotice(undefined)

    if (!active || previewKind === 'unsupported' || (!file && !localFile)) {
      setLoading(false)
      return
    }

    let cancelled = false
    let localObjectUrl: string | undefined
    let destroyConfirmation: (() => void) | undefined
    const previewMetadata = file || (localFile
      ? toLocalPreviewMetadata(localFile, uploadFile)
      : undefined)
    const policy = fileService.getPreviewPolicy(previewMetadata)

    const loadPreview = async (deliveryMode: FilePreviewDeliveryMode) => {
      setPreviewNotice(undefined)
      setLoading(true)
      try {
        let url: string
        if (localFile) {
          localObjectUrl = URL.createObjectURL(localFile)
          url = localObjectUrl
        } else if (file) {
          url = await fileService.resolveTemporaryUrl(file.id, file, deliveryMode)
        } else {
          return
        }

        if (cancelled) {
          if (file) fileService.revokeTemporaryUrl(file.id)
          return
        }
        setPreviewUrl(url)
      } catch (error) {
        if (!cancelled) {
          const text = error instanceof Error ? error.message : '文件预览加载失败'
          message.error(text || '文件预览加载失败')
          setPreviewUrl(undefined)
          setPreviewNotice(text)
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    if (policy.mode === 'download') {
      setLoading(false)
      setPreviewNotice(getDownloadNotice(policy, previewKind))
    } else if (policy.mode === 'direct') {
      const fileSizeLabel = formatFileSize(policy.fileSize)
      const blobLimitLabel = formatFileSize(policy.maxBlobSize)
      const directLimitLabel = formatFileSize(policy.maxDirectSize)
      setLoading(false)
      setPreviewNotice('大文件预览等待确认，可继续使用流式预览或直接下载。')

      const confirmation = Modal.confirm({
        centered: true,
        title: `使用流式方式预览 ${fileSizeLabel} 的 PDF？`,
        content: (
          <div>
            <Typography.Paragraph>
              文件已超过 {blobLimitLabel} 的内存预览阈值。继续后将
              {file ? '使用带短期签名的 Range 流式地址直接加载' : '由浏览器直接读取本地文件'}，
              不会先把整份文件复制到页面内存。
            </Typography.Paragraph>
            <Typography.Text type="secondary">
              流式预览最高支持 {directLimitLabel}，打开速度仍取决于网络和浏览器 PDF 渲染能力。
            </Typography.Text>
          </div>
        ),
        okText: '使用流式预览',
        cancelText: '暂不预览',
        onOk: () => loadPreview('direct'),
        onCancel: () => {
          if (!cancelled) {
            setPreviewNotice('已取消大文件预览，可使用上方“下载”查看原文件。')
          }
        },
      })
      destroyConfirmation = confirmation.destroy
    } else {
      void loadPreview('blob')
    }

    return () => {
      cancelled = true
      destroyConfirmation?.()
      if (localObjectUrl) URL.revokeObjectURL(localObjectUrl)
      if (file) fileService.revokeTemporaryUrl(file.id)
    }
  }, [active, file, localFile, previewKind, uploadFile])

  return { previewUrl, loading, previewNotice }
}
