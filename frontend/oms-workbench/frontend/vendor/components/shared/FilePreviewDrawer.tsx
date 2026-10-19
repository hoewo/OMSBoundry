import { useMemo } from 'react'
import { Button, Drawer, Empty, Space, Typography, message } from 'antd'
import { DownloadOutlined, LinkOutlined } from '@ant-design/icons'
import type { UploadFile } from 'antd'
import { fileService } from '@/services/file'
import type { File as ProjectFile } from '@/types'
import { useFilePreviewSource } from './useFilePreviewSource'
import { useResizableWidth } from './useResizableWidth'
import './FilePreviewDrawer.css'

interface FilePreviewDrawerProps {
  open: boolean
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
  onClose: () => void
  width?: number
  mask?: boolean
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

export const FilePreviewDrawer = ({
  open,
  file,
  uploadFile,
  onClose,
  width = 920,
  mask = false,
}: FilePreviewDrawerProps) => {
  const localFile = uploadFile?.originFileObj as globalThis.File | undefined
  const displayName = getFileName({ file, uploadFile })
  const resolvedWidth = Math.max(width, 920)
  const {
    width: drawerWidth,
    isResizing,
    handleResizeStart,
    handleResizeKeyDown,
  } = useResizableWidth({
    defaultWidth: resolvedWidth,
    minWidth: 920,
    enabled: open,
    invertDelta: true,
    getMaxWidth: () => window.innerWidth - 24,
  })

  const previewMode = useMemo<'pdf' | 'image' | 'unsupported'>(() => {
    if (isPdfFile({ file, uploadFile })) return 'pdf'
    if (isImageFile({ file, uploadFile })) return 'image'
    return 'unsupported'
  }, [file, uploadFile])
  const { previewUrl, loading, previewNotice } = useFilePreviewSource({
    active: open,
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
    <Drawer
      title={displayName || '文件预览'}
      placement="right"
      open={open}
      onClose={onClose}
      width={drawerWidth}
      mask={mask}
      push={{ distance: Math.min(drawerWidth, 480) }}
      destroyOnHidden
      extra={
        file || uploadFile ? (
          <Space>
            <Button icon={<LinkOutlined />} onClick={handleOpenNewWindow} disabled={!previewUrl}>
              新窗口打开
            </Button>
            <Button icon={<DownloadOutlined />} onClick={handleDownload}>
              下载
            </Button>
          </Space>
        ) : undefined
      }
    >
      <div className="file-preview-drawer__shell">
        <div
          className={[
            'file-preview-drawer__resize-handle',
            isResizing ? 'file-preview-drawer__resize-handle--active' : '',
          ]
            .filter(Boolean)
            .join(' ')}
          role="separator"
          aria-label="拖动调整预览抽屉宽度"
          aria-orientation="vertical"
          tabIndex={0}
          onMouseDown={handleResizeStart}
          onKeyDown={handleResizeKeyDown}
        />

        <div className="file-preview-drawer">
          {loading ? (
            <div className="file-preview-drawer__state">
              <Typography.Text>正在准备预览，请稍候…</Typography.Text>
            </div>
          ) : previewMode === 'pdf' && previewUrl ? (
            <iframe
              src={previewDisplayUrl}
              title={displayName || 'PDF 预览'}
              className="file-preview-drawer__frame"
            />
          ) : previewMode === 'image' && previewUrl ? (
            <div className="file-preview-drawer__image-shell">
              <img
                src={previewUrl}
                alt={displayName || '图片预览'}
                className="file-preview-drawer__image"
              />
            </div>
          ) : (
            <div className="file-preview-drawer__state">
              <Empty
                description={
                  previewNotice || (file
                    ? '当前仅支持 PDF 和图片预览，请下载后查看原文件。'
                    : '请选择需要预览的文件')
                }
              />
            </div>
          )}
        </div>
      </div>
    </Drawer>
  )
}

export default FilePreviewDrawer
