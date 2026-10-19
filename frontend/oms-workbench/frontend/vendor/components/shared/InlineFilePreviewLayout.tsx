import { useCallback, useEffect, useRef, useState } from 'react'
import type { CSSProperties, ReactNode } from 'react'
import type { UploadFile } from 'antd'
import type { File as ProjectFile } from '@/types'
import { FilePreviewPane } from './FilePreviewPane'
import { useResizableWidth } from './useResizableWidth'
import './InlineFilePreviewLayout.css'

interface InlineFilePreviewLayoutProps {
  children: ReactNode
  file?: ProjectFile | null
  uploadFile?: UploadFile | null
  onClosePreview?: () => void
  className?: string
  contentClassName?: string
  defaultPreviewWidth?: number
  minPreviewWidth?: number
  minContentWidth?: number
  contentWidth?: number
  pinContentToRight?: boolean
  minHeight?: string
  previewHeight?: string
}

export const InlineFilePreviewLayout = ({
  children,
  file,
  uploadFile,
  onClosePreview,
  className,
  contentClassName,
  defaultPreviewWidth = 820,
  minPreviewWidth = 780,
  minContentWidth = 460,
  contentWidth,
  pinContentToRight = false,
  minHeight = 'calc(100vh - 180px)',
  previewHeight,
}: InlineFilePreviewLayoutProps) => {
  const shellRef = useRef<HTMLDivElement | null>(null)
  const [hasManualPreviewWidth, setHasManualPreviewWidth] = useState(false)
  const hasPreview = Boolean(file || uploadFile)
  const remainingContentWidth = contentWidth ?? minContentWidth
  const {
    width: previewWidth,
    isResizing,
    setWidth: setPreviewWidth,
    handleResizeStart,
    handleResizeKeyDown,
  } = useResizableWidth({
    containerRef: shellRef,
    defaultWidth: defaultPreviewWidth,
    minWidth: minPreviewWidth,
    minRemainingWidth: remainingContentWidth,
    enabled: hasPreview,
    invertDelta: true,
  })

  const handlePinnedResizeStart = useCallback(
    (event: Parameters<typeof handleResizeStart>[0]) => {
      setHasManualPreviewWidth(true)
      handleResizeStart(event)
    },
    [handleResizeStart]
  )

  const handlePinnedResizeKeyDown = useCallback(
    (event: Parameters<typeof handleResizeKeyDown>[0]) => {
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        setHasManualPreviewWidth(true)
      }
      handleResizeKeyDown(event)
    },
    [handleResizeKeyDown]
  )

  useEffect(() => {
    if (!hasPreview) {
      setHasManualPreviewWidth(false)
    }
  }, [hasPreview])

  useEffect(() => {
    if (
      !hasPreview ||
      !contentWidth ||
      !pinContentToRight ||
      hasManualPreviewWidth ||
      !shellRef.current
    ) {
      return
    }

    const shell = shellRef.current
    const syncPreviewWidth = () => {
      const containerWidth = shell.getBoundingClientRect().width
      if (!Number.isFinite(containerWidth) || containerWidth <= 0) {
        return
      }

      // 固定右侧内容列时，预览区域吃掉剩余空间。
      setPreviewWidth(Math.max(minPreviewWidth, containerWidth - contentWidth - 18))
    }

    syncPreviewWidth()
    const frameId = window.requestAnimationFrame(syncPreviewWidth)
    const observer = new ResizeObserver(syncPreviewWidth)
    observer.observe(shell)
    window.addEventListener('resize', syncPreviewWidth)

    return () => {
      window.cancelAnimationFrame(frameId)
      observer.disconnect()
      window.removeEventListener('resize', syncPreviewWidth)
    }
  }, [
    contentWidth,
    hasManualPreviewWidth,
    hasPreview,
    minPreviewWidth,
    pinContentToRight,
    setPreviewWidth,
  ])

  return (
    <div
      ref={shellRef}
      className={[
        'inline-file-preview-layout',
        hasPreview ? 'inline-file-preview-layout--with-preview' : '',
        pinContentToRight ? 'inline-file-preview-layout--pin-content' : '',
        className || '',
      ]
        .filter(Boolean)
        .join(' ')}
      style={
        {
          '--inline-preview-min-height': minHeight,
          '--inline-preview-pane-height': previewHeight || minHeight,
          '--inline-preview-content-min-width': `${minContentWidth}px`,
          '--inline-preview-content-width': contentWidth
            ? `${contentWidth}px`
            : undefined,
        } as CSSProperties
      }
    >
      {hasPreview && (
        <div
          className={[
            'inline-file-preview-layout__resize-handle',
            isResizing ? 'inline-file-preview-layout__resize-handle--active' : '',
          ]
            .filter(Boolean)
            .join(' ')}
          role="separator"
          aria-label="拖动调整预览宽度"
          aria-orientation="vertical"
          tabIndex={0}
          onMouseDown={pinContentToRight ? handlePinnedResizeStart : handleResizeStart}
          onKeyDown={pinContentToRight ? handlePinnedResizeKeyDown : handleResizeKeyDown}
        />
      )}

      {hasPreview && (
        <aside
          className="inline-file-preview-layout__preview"
          style={{ width: previewWidth, flexBasis: previewWidth }}
        >
          <FilePreviewPane
            file={file}
            uploadFile={uploadFile}
            onClose={onClosePreview}
          />
        </aside>
      )}

      <div
        className={[
          'inline-file-preview-layout__content',
          contentWidth ? 'inline-file-preview-layout__content--fixed' : '',
          pinContentToRight ? 'inline-file-preview-layout__content--pinned' : '',
          contentClassName || '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {children}
      </div>
    </div>
  )
}

export default InlineFilePreviewLayout

