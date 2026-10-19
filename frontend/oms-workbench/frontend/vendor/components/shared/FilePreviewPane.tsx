import { useMemo } from 'react'
import { Button, Empty, Space, Typography, message } from 'antd'
import { CloseOutlined, DownloadOutlined, LinkOutlined } from '@ant-design/icons'
import type { UploadFile } from 'antd'
import { fileService } from '@/services/file'
import type { File as ProjectFile } from '@/types'
import { useFilePreviewSource } from './useFilePreviewSource'
import './FilePreviewPane.css'

interface FilePreviewPaneProps {
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
  onClose?: () => void
}

const IMAGE_EXTENSIONS = new Set(['png', 'jpg', 'jpeg', 'webp', 'gif', 'bmp'])

const getFileName = ({
  file,
  uploadFile,
}: {
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
}) => file?.original_name || uploadFile?.name || ''

const getMimeType = ({
  file,
  uploadFile,
}: {
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
}) => file?.mime_type || uploadFile?.type || uploadFile?.originFileObj?.type || ''

const getFileExtension = ({
  file,
  uploadFile,
}: {
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
}) => getFileName({ file, uploadFile }).split('.').pop()?.toLowerCase() || ''

const isPdfFile = ({
  file,
  uploadFile,
}: {
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
}) => {
  const mimeType = getMimeType({ file, uploadFile })
  return mimeType.includes('pdf') || getFileExtension({ file, uploadFile }) === 'pdf'
}

const isImageFile = ({
  file,
  uploadFile,
}: {
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
}) => {
  const mimeType = getMimeType({ file, uploadFile })
  return (
    mimeType.startsWith('image/') ||
    IMAGE_EXTENSIONS.has(getFileExtension({ file, uploadFile }))
  )
}

export const FilePreviewPane = ({
  file,
  uploadFile,
  onClose,
}: FilePreviewPaneProps) => {
  const localFile = uploadFile?.originFileObj as globalThis.File | undefined
  const displayName = getFileName({ file, uploadFile })

  const previewMode = useMemo<'pdf' | 'image' | 'unsupported'>(() => {
    if (isPdfFile({ file, uploadFile })) return 'pdf'
    if (isImageFile({ file, uploadFile })) return 'image'
    return 'unsupported'
  }, [file, uploadFile])
  const { previewUrl, loading, previewNotice } = useFilePreviewSource({
    file,
    uploadFile,
    previewKind: previewMode,
  })

  const previewDisplayUrl = useMemo(() => {
    if (!previewUrl || previewMode !== 'pdf') {
      return previewUrl
    }

    const hashJoiner = previewUrl.includes('#') ? '&' : '#'
    return `${previewUrl}${hashJoiner}navpanes=0&view=FitH&zoom=page-width`
  }, [previewMode, previewUrl])

  const handleDownload = async () => {
    if (localFile) {
      const url = previewUrl || URL.createObjectURL(localFile)
      const link = document.createElement('a')
      link.href = url
      link.download = displayName || '预览文件'
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      if (!previewUrl) {
        URL.revokeObjectURL(url)
      }
      return
    }

    if (!file) return
    try {
      await fileService.downloadFile(file.id, file.original_name)
    } catch (error: any) {
      message.error(error.message || '文件下载失败')
    }
  }

  const handleOpenNewWindow = () => {
    if (!previewDisplayUrl) return
    window.open(previewDisplayUrl, '_blank', 'noopener,noreferrer')
  }

  return (
    <section className="file-preview-pane">
      <header className="file-preview-pane__header">
        <div className="file-preview-pane__heading">
          <Typography.Text className="file-preview-pane__eyebrow">
            源文件预览
          </Typography.Text>
          <Typography.Title level={5} className="file-preview-pane__title">
            {displayName || '未选择文件'}
          </Typography.Title>
        </div>
        <Space wrap>
          <Button icon={<LinkOutlined />} onClick={handleOpenNewWindow} disabled={!previewUrl}>
            新窗口打开
          </Button>
          <Button icon={<DownloadOutlined />} onClick={handleDownload}>
            下载
          </Button>
          {onClose && (
            <Button icon={<CloseOutlined />} onClick={onClose}>
              收起
            </Button>
          )}
        </Space>
      </header>

      <div className="file-preview-pane__body">
        {loading ? (
          <div className="file-preview-pane__state">
            <Typography.Text>正在准备预览，请稍候…</Typography.Text>
          </div>
        ) : previewMode === 'pdf' && previewUrl ? (
          <iframe
            src={previewDisplayUrl}
            title={displayName || 'PDF 预览'}
            className="file-preview-pane__frame"
          />
        ) : previewMode === 'image' && previewUrl ? (
          <div className="file-preview-pane__image-shell">
            <img
              src={previewUrl}
              alt={displayName || '图片预览'}
              className="file-preview-pane__image"
            />
          </div>
        ) : (
          <div className="file-preview-pane__state">
            <Empty
              description={previewNotice || '当前仅支持 PDF 和图片预览，请下载后查看原文件。'}
            />
          </div>
        )}
      </div>
    </section>
  )
}

export default FilePreviewPane

