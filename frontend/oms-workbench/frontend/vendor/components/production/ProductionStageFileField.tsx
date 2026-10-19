import { Button, Space, Typography, Upload, message } from 'antd'
import type { ReactNode } from 'react'
import {
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileDoneOutlined,
} from '@ant-design/icons'
import type { UploadFile, UploadProps } from 'antd'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { AttachmentUploader } from '@/components/shared/AttachmentUploader'
import type { File as ProjectFile, ProductionFile } from '@/types'
import { CAD_FILE_ACCEPT, isCadFileName, isWithinUploadLimit, MAX_UPLOAD_SIZE_LABEL } from '@/utils/upload'
import dayjs from 'dayjs'
import './ProductionStageFileField.css'

const { Text } = Typography

interface ProductionStageFileFieldProps {
  label: ReactNode
  required?: boolean
  helper?: ReactNode
  fileList: UploadFile[]
  existingFiles?: ProductionFile[]
  multiple?: boolean
  maxCount?: number
  maxSizeBytes?: number
  accept?: string
  uploadHint?: ReactNode
  existingTitle?: string
  deleteTitle: string
  deleteDescription: string
  disabled?: boolean
  deleteLoading?: boolean
  canDeleteExisting?: boolean
  onFileListChange: (fileList: UploadFile[]) => void
  onRemovePendingFile: (uid: string) => void
  onPreviewFile: (file: ProjectFile) => void
  onPreviewUploadFile: (file: UploadFile) => void
  onDownloadFile: (file: ProjectFile) => void
  onDeleteExistingFile: (file: ProductionFile) => void
}

const formatFileSize = (bytes?: number): string => {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export const ProductionStageFileField = ({
  label,
  required = false,
  helper,
  fileList,
  existingFiles = [],
  multiple = true,
  maxCount = 20,
  maxSizeBytes,
  accept = `.pdf,.doc,.docx,.png,.jpg,.jpeg,${CAD_FILE_ACCEPT}`,
  uploadHint,
  existingTitle = '已保存文件',
  deleteTitle,
  deleteDescription,
  disabled = false,
  deleteLoading = false,
  canDeleteExisting = true,
  onFileListChange,
  onRemovePendingFile,
  onPreviewFile,
  onPreviewUploadFile,
  onDownloadFile,
  onDeleteExistingFile,
}: ProductionStageFileFieldProps) => {
  const pendingFiles = fileList.filter((file) => file.originFileObj)
  const visibleExistingFiles = existingFiles.filter((file) => file.file)
  const hasFiles = pendingFiles.length > 0 || visibleExistingFiles.length > 0

  const beforeUpload: UploadProps['beforeUpload'] = (file) => {
    if (maxSizeBytes ? file.size > maxSizeBytes : !isWithinUploadLimit(file)) {
      message.error(`文件大小不能超过${maxSizeBytes ? `${maxSizeBytes / (1024 * 1024)}MB` : MAX_UPLOAD_SIZE_LABEL}`)
      return Upload.LIST_IGNORE
    }

    return false
  }

  return (
    <div className="production-stage-file-field">
      <div className="production-stage-file-field__head">
        <Text className="production-stage-file-field__label">
          {required && <span className="production-stage-file-field__required">*</span>}
          {label}
        </Text>
        {hasFiles && (
          <span className="production-stage-file-field__count">
            {visibleExistingFiles.length} 已保存 · {pendingFiles.length} 待保存
          </span>
        )}
      </div>

      <AttachmentUploader
        fileList={fileList}
        onChange={onFileListChange}
        beforeUpload={beforeUpload}
        multiple={multiple}
        maxCount={maxCount}
        accept={accept}
        disabled={disabled}
        helper={uploadHint || `支持 PDF、DOC、DOCX、PNG、JPG 及 CAD（DWG、DXF），最大 ${MAX_UPLOAD_SIZE_LABEL}`}
        onPreview={(file) => {
          if (isCadFileName(file.name)) {
            message.info('CAD 文件请下载后查看，暂不支持在线预览')
            return
          }
          onPreviewUploadFile(file)
        }}
        onRemove={(file) => {
          onRemovePendingFile(file.uid)
          return true
        }}
      />

      {helper && <div className="production-stage-file-field__helper">{helper}</div>}

      {visibleExistingFiles.length > 0 && (
        <div className="production-stage-file-field__group">
          <div className="production-stage-file-field__group-title">{existingTitle}</div>
          <div className="production-stage-file-field__cards">
            {visibleExistingFiles.map((productionFile) => {
              const file = productionFile.file!
              const uploadTime = productionFile.created_at
                ? dayjs(productionFile.created_at).format('YYYY-MM-DD HH:mm')
                : '-'

              return (
                <article
                  key={productionFile.id}
                  className="production-stage-file-field__card production-stage-file-field__card--saved"
                >
                  <div className="production-stage-file-field__badge production-stage-file-field__badge--saved">
                    <FileDoneOutlined />
                  </div>
                  <div className="production-stage-file-field__meta">
                    <strong title={file.original_name}>{file.original_name}</strong>
                    <span>
                      已保存 · {formatFileSize(file.file_size)} · {uploadTime}
                    </span>
                  </div>
                  <Space size={6} wrap className="production-stage-file-field__actions">
                    {!isCadFileName(file.original_name) && (
                      <Button
                        size="small"
                        icon={<EyeOutlined />}
                        disabled={disabled}
                        onClick={() => onPreviewFile(file)}
                      >
                        预览
                      </Button>
                    )}
                    <Button
                      size="small"
                      icon={<DownloadOutlined />}
                      disabled={disabled}
                      onClick={() => onDownloadFile(file)}
                    >
                      下载
                    </Button>
                    {canDeleteExisting && (
                      <DeleteConfirmButton
                        title={deleteTitle}
                        description={deleteDescription}
                        details={[
                          { label: '文件', value: file.original_name },
                          { label: '上传时间', value: uploadTime },
                        ]}
                        buttonProps={{
                          size: 'small',
                          icon: <DeleteOutlined />,
                          disabled,
                        }}
                        loading={deleteLoading}
                        buttonText="删除"
                        onConfirm={() => onDeleteExistingFile(productionFile)}
                      />
                    )}
                  </Space>
                </article>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

export default ProductionStageFileField
