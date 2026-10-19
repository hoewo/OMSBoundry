import { Button, Progress, Upload, message } from 'antd'
import type { ReactNode } from 'react'
import type { UploadFile, UploadProps } from 'antd'
import {
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileTextOutlined,
  InboxOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import { isCadFileName, isWithinUploadLimit, MAX_UPLOAD_SIZE_LABEL } from '@/utils/upload'
import './AttachmentUploader.css'

const { Dragger } = Upload

export interface AttachmentUploaderProps {
  label?: ReactNode
  helper?: ReactNode
  fileList: UploadFile[]
  onChange: (fileList: UploadFile[]) => void
  beforeUpload?: UploadProps['beforeUpload']
  onPreview?: (file: UploadFile) => void
  onDownload?: (file: UploadFile) => void
  onRemove?: (file: UploadFile) => boolean | void | Promise<boolean | void>
  canRemove?: (file: UploadFile) => boolean
  onRetry?: (file: UploadFile) => void
  accept?: string
  maxCount?: number
  disabled?: boolean
  uploading?: boolean
  uploadProgress?: number
  multiple?: boolean
}

const formatFileSize = (size?: number) => {
  if (!size || size < 0) return '大小未知'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

const getFileBadge = (file: UploadFile) =>
  file.name.split('.').pop()?.slice(0, 4).toUpperCase() || 'FILE'

const getStatusText = (file: UploadFile, uploading: boolean) => {
  if (file.status === 'error') return '上传失败'
  if (file.status === 'uploading' || uploading) return '上传中'
  if (file.status === 'done') return '已上传'
  return '待保存'
}

export const AttachmentUploader = ({
  label,
  helper,
  fileList,
  onChange,
  beforeUpload,
  onPreview,
  onDownload,
  onRemove,
  canRemove,
  onRetry,
  accept,
  maxCount = 20,
  disabled = false,
  uploading = false,
  uploadProgress,
  multiple = true,
}: AttachmentUploaderProps) => {
  const handleChange: UploadProps['onChange'] = ({ fileList: next }) => {
    if (next.length > maxCount) {
      message.warning(`每类附件最多上传 ${maxCount} 个文件`)
    }
    onChange(next.slice(-maxCount))
  }

  const handleRemove = async (file: UploadFile) => {
    const result = await onRemove?.(file)
    if (result === false) return false
    onChange(fileList.filter((item) => item.uid !== file.uid))
    return true
  }

  const handleBeforeUpload: UploadProps['beforeUpload'] = (file, selectedFiles) => {
    if (!isWithinUploadLimit(file)) {
      message.error(`文件大小不能超过${MAX_UPLOAD_SIZE_LABEL}`)
      return Upload.LIST_IGNORE
    }
    return beforeUpload?.(file, selectedFiles) ?? false
  }

  return (
    <div className="attachment-uploader">
      {(label || helper) && (
        <div className="attachment-uploader__header">
          <div>
            {label && <div className="attachment-uploader__label">{label}</div>}
            {helper && <div className="attachment-uploader__helper">{helper}</div>}
          </div>
          <span className="attachment-uploader__count">
            {fileList.length}/{maxCount}
          </span>
        </div>
      )}

      <Dragger
        fileList={fileList}
        onChange={handleChange}
        beforeUpload={handleBeforeUpload}
        onRemove={handleRemove}
        multiple={multiple}
        maxCount={maxCount}
        accept={accept}
        disabled={disabled || uploading}
        showUploadList={false}
        className="attachment-uploader__dropzone"
      >
        <p className="attachment-uploader__drop-icon"><InboxOutlined /></p>
        <p className="attachment-uploader__drop-title">
          拖拽文件到这里，或点击选择
        </p>
        <p className="attachment-uploader__drop-hint">支持一次选择多个文件</p>
      </Dragger>

      {fileList.length > 0 && (
        <div className="attachment-uploader__list" aria-label="附件列表">
          {fileList.map((file) => {
            const percent = file.percent ?? uploadProgress
            const isBusy = file.status === 'uploading' || uploading
            return (
              <div className="attachment-uploader__row" key={file.uid}>
                <span className="attachment-uploader__badge">
                  <FileTextOutlined />
                  {getFileBadge(file)}
                </span>
                <div className="attachment-uploader__file-info">
                  <strong title={file.name}>{file.name}</strong>
                  <span>
                    {formatFileSize(file.size)} · {getStatusText(file, uploading)}
                  </span>
                  {isBusy && typeof percent === 'number' && (
                    <Progress percent={percent} size="small" showInfo={false} />
                  )}
                </div>
                <div className="attachment-uploader__actions">
                  {file.status === 'error' && onRetry && (
                    <Button
                      type="text"
                      size="small"
                      icon={<ReloadOutlined />}
                      onClick={() => onRetry(file)}
                    >
                      重试
                    </Button>
                  )}
                  {onPreview && !isCadFileName(file.name) && (
                    <Button
                      type="text"
                      size="small"
                      icon={<EyeOutlined />}
                      aria-label="预览"
                      onClick={() => onPreview(file)}
                      disabled={isBusy}
                    >
                      预览
                    </Button>
                  )}
                  {onDownload && file.status === 'done' && (
                    <Button
                      type="text"
                      size="small"
                      icon={<DownloadOutlined />}
                      aria-label="下载"
                      onClick={() => onDownload(file)}
                      disabled={isBusy}
                    >
                      下载
                    </Button>
                  )}
                  {!disabled && (canRemove?.(file) ?? true) && (
                    <Button
                      type="text"
                      size="small"
                      danger
                      icon={<DeleteOutlined />}
                      aria-label="移除"
                      onClick={() => void handleRemove(file)}
                      disabled={isBusy}
                    >
                      移除
                    </Button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

export default AttachmentUploader
