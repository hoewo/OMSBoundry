import { useEffect, useMemo, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import { Button, Upload, message } from 'antd'
import {
  CheckCircleFilled,
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  InboxOutlined,
  ReloadOutlined,
  StopOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { biddingService } from '@/services/bidding'
import { fileService } from '@/services/file'
import { oa2026Service } from '@/services/oa2026'
import { useProjectPermissions } from '@/hooks/useProjectPermissions'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { FilePreviewDrawer } from '@/components/shared/FilePreviewDrawer'
import type { File as ProjectFile, FileCategory } from '@/types'
import { getErrorMessage } from '@/utils/error'
import { isWithinUploadLimit, MAX_UPLOAD_SIZE_LABEL } from '@/utils/upload'
import './BiddingFileList.css'

interface BiddingFileListProps {
  projectId: string
}

type BiddingFileType = 'tender' | 'bid' | 'award'

const FILE_CATEGORY_MAP: Record<BiddingFileType, FileCategory> = {
  tender: 'tender',
  bid: 'bid',
  award: 'award_notice',
}

const FILE_TYPES: BiddingFileType[] = ['tender', 'bid', 'award']

const FILE_META: Record<
  BiddingFileType,
  {
    title: string
    hint: string
    accent: string
    accentText: string
    soft: string
  }
> = {
  tender: {
    title: '招标文件',
    hint: '待上传招标材料',
    accent: '#2563eb',
    accentText: '#1d4ed8',
    soft: 'rgba(37, 99, 235, 0.1)',
  },
  bid: {
    title: '投标文件',
    hint: '待上传投标版本',
    accent: '#d97706',
    accentText: '#b45309',
    soft: 'rgba(245, 158, 11, 0.12)',
  },
  award: {
    title: '中标通知书',
    hint: '待上传中标通知',
    accent: '#059669',
    accentText: '#047857',
    soft: 'rgba(16, 185, 129, 0.12)',
  },
}

const buildUpdatePayload = (type: BiddingFileType, fileIds: string[]) => {
  if (type === 'tender') {
    return { tender_file_ids: fileIds }
  }

  if (type === 'bid') {
    return { bid_file_ids: fileIds }
  }

  return { award_notice_file_ids: fileIds }
}

const buildRequirementPayload = (
  type: BiddingFileType,
  notRequired: boolean
) => {
  if (type === 'tender') {
    return { tender_file_not_required: notRequired }
  }

  if (type === 'bid') {
    return { bid_file_not_required: notRequired }
  }

  return { award_notice_file_not_required: notRequired }
}

const formatFileSize = (bytes?: number) => {
  if (!bytes || Number.isNaN(bytes)) {
    return '大小未知'
  }

  if (bytes < 1024) {
    return `${bytes} B`
  }

  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }

  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const getFileName = (file?: ProjectFile) => {
  if (!file) {
    return ''
  }

  return file.original_name || file.file_name
}

const getFileBadge = (file?: ProjectFile) => {
  const fileName = getFileName(file)
  const ext = fileName.split('.').pop()?.toUpperCase()

  if (!ext) {
    return 'FILE'
  }

  return ext.slice(0, 4)
}

const getTypeLabel = (type: BiddingFileType) => FILE_META[type].title

export const BiddingFileList = ({ projectId }: BiddingFileListProps) => {
  const queryClient = useQueryClient()
  const [uploadingType, setUploadingType] = useState<BiddingFileType | null>(
    null
  )
  const [deletingType, setDeletingType] = useState<BiddingFileType | null>(null)
  const [downloadingType, setDownloadingType] =
    useState<BiddingFileType | null>(null)
  const [updatingRequirementType, setUpdatingRequirementType] =
    useState<BiddingFileType | null>(null)
  const [previewFile, setPreviewFile] = useState<ProjectFile | null>(null)
  const pendingFileIdsRef = useRef<Record<BiddingFileType, string[]>>({
    tender: [],
    bid: [],
    award: [],
  })

  const projectPermissions = useProjectPermissions(projectId)
  const canUpload = projectPermissions.canUploadBusinessFile
  const canDelete = projectPermissions.canDeleteBusinessFile

  const { data: biddingInfo } = useQuery({
    queryKey: ['biddingInfo', projectId],
    queryFn: () => biddingService.getBiddingInfo(projectId),
    enabled: !!projectId,
  })
  const { data: automaticBidding } = useQuery({
    queryKey: ['oa2026', 'bidding', projectId],
    queryFn: () => oa2026Service.getBidding(projectId),
    enabled: !!projectId,
  })

  const requiredFileTypes = useMemo<BiddingFileType[]>(() => {
    if (!automaticBidding?.engagement_method) {
      return []
    }
    return automaticBidding.engagement_method === 'tender'
      ? FILE_TYPES
      : ['bid', 'award']
  }, [automaticBidding?.engagement_method])

  const filesByType = useMemo<Record<BiddingFileType, ProjectFile[]>>(
    () => ({
      tender: biddingInfo?.tender_files || [],
      bid: biddingInfo?.bid_files || [],
      award: biddingInfo?.award_notice_files || [],
    }),
    [biddingInfo]
  )

  useEffect(() => {
    pendingFileIdsRef.current = {
      tender: filesByType.tender.map((file) => file.id),
      bid: filesByType.bid.map((file) => file.id),
      award: filesByType.award.map((file) => file.id),
    }
  }, [filesByType])

  const currentFiles = useMemo<
    Record<BiddingFileType, ProjectFile | undefined>
  >(() => {
    return {
      tender: filesByType.tender[filesByType.tender.length - 1],
      bid: filesByType.bid[filesByType.bid.length - 1],
      award: filesByType.award[filesByType.award.length - 1],
    }
  }, [filesByType])

  const uploadedCount = useMemo(
    () => requiredFileTypes.filter((type) => !!currentFiles[type]).length,
    [currentFiles, requiredFileTypes]
  )

  // OA 2026 uses the selected engagement method as the only source of truth:
  // comparison/competitive negotiation do not require a tender file, while
  // bid and award files are always required. Legacy per-file skip flags remain
  // in the old API for compatibility, but must not change this checklist.
  const notRequiredTypes = useMemo<BiddingFileType[]>(() => [], [])

  const missingTypes = useMemo(
    () =>
      requiredFileTypes.filter(
        (type) => !currentFiles[type] && !notRequiredTypes.includes(type)
      ),
    [currentFiles, notRequiredTypes, requiredFileTypes]
  )

  const handledCount = uploadedCount + notRequiredTypes.length

  const latestUpdated = useMemo(() => {
    return requiredFileTypes.map((type) => ({
      type,
      file: currentFiles[type],
    }))
      .filter(
        (item): item is { type: BiddingFileType; file: ProjectFile } =>
          !!item.file
      )
      .sort(
        (a, b) =>
          dayjs(b.file.created_at).valueOf() -
          dayjs(a.file.created_at).valueOf()
      )[0]
  }, [currentFiles, requiredFileTypes])

  const uploadMutation = useMutation({
    mutationFn: async ({
      type,
      file,
    }: {
      type: BiddingFileType
      file: globalThis.File
    }) => {
      const uploadedFile = await fileService.uploadFile(
        projectId,
        file,
        FILE_CATEGORY_MAP[type],
        FILE_META[type].title
      )

      const nextFileIDs = Array.from(new Set([
        ...pendingFileIdsRef.current[type],
        uploadedFile.id,
      ]))
      pendingFileIdsRef.current[type] = nextFileIDs
      await biddingService.appendUploadedFile(projectId, type, uploadedFile.id)

      return { type }
    },
    onSuccess: ({ type }) => {
      message.success(`${FILE_META[type].title}上传成功`)
      setPreviewFile(null)
      queryClient.invalidateQueries({ queryKey: ['biddingInfo', projectId] })
      queryClient.invalidateQueries({
        queryKey: ['projectProgressAssistant', String(projectId)],
      })
      queryClient.invalidateQueries({ queryKey: ['projectFiles'] })
      queryClient.invalidateQueries({ queryKey: ['files'] })
    },
    onError: (error: any, variables) => {
      message.error(
        getErrorMessage(error, `${FILE_META[variables.type].title}上传失败`)
      )
    },
    onSettled: (_data, _error, variables) => {
      if (variables?.type) {
        setUploadingType((current) =>
          current === variables.type ? null : current
        )
      }
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async ({
      type,
      fileId,
    }: {
      type: BiddingFileType
      fileId: string
    }) => {
      // Ending a BE reference must never delete Evidence bytes.
      const nextFileIDs = pendingFileIdsRef.current[type].filter(
        (item) => item !== fileId
      )
      pendingFileIdsRef.current[type] = nextFileIDs
      await biddingService.removeMaterialReference(projectId, type, fileId)
      return { type }
    },
    onSuccess: ({ type }, variables) => {
      message.success(`${FILE_META[type].title}已移除`)
      if (previewFile?.id === variables.fileId) {
        setPreviewFile(null)
      }
      queryClient.invalidateQueries({ queryKey: ['biddingInfo', projectId] })
      queryClient.invalidateQueries({
        queryKey: ['projectProgressAssistant', String(projectId)],
      })
      queryClient.invalidateQueries({ queryKey: ['projectFiles'] })
      queryClient.invalidateQueries({ queryKey: ['files'] })
    },
    onError: (error: any, variables) => {
      message.error(
        getErrorMessage(error, `${FILE_META[variables.type].title}删除失败`)
      )
    },
    onSettled: (_data, _error, variables) => {
      if (variables?.type) {
        setDeletingType((current) =>
          current === variables.type ? null : current
        )
      }
    },
  })

  const requirementMutation = useMutation({
    mutationFn: async ({
      type,
      notRequired,
    }: {
      type: BiddingFileType
      notRequired: boolean
    }) => {
      await biddingService.createOrUpdateBiddingInfo(
        projectId,
        buildRequirementPayload(type, notRequired)
      )
      return { type, notRequired }
    },
    onSuccess: ({ type, notRequired }) => {
      message.success(
        notRequired
          ? `${FILE_META[type].title}已标记为不需要`
          : `${FILE_META[type].title}已恢复为需要`
      )
      queryClient.invalidateQueries({ queryKey: ['biddingInfo', projectId] })
      queryClient.invalidateQueries({
        queryKey: ['projectProgressAssistant', String(projectId)],
      })
    },
    onError: (error: any, variables) => {
      message.error(
        getErrorMessage(
          error,
          variables.notRequired ? '标记不需要失败' : '恢复需要失败'
        )
      )
    },
    onSettled: (_data, _error, variables) => {
      if (variables?.type) {
        setUpdatingRequirementType((current) =>
          current === variables.type ? null : current
        )
      }
    },
  })

  const validateBeforeUpload = (file: globalThis.File) => {
    if (!isWithinUploadLimit(file)) {
      message.error(`文件大小不能超过${MAX_UPLOAD_SIZE_LABEL}`)
      return false
    }
    return true
  }

  const handleUploadSelect =
    (type: BiddingFileType) => async (file: globalThis.File) => {
      if (!validateBeforeUpload(file)) {
        return Upload.LIST_IGNORE
      }

      setUploadingType(type)
      await uploadMutation.mutateAsync({
        type,
        file,
      })

      return Upload.LIST_IGNORE
    }

  const handleDownload = async (type: BiddingFileType, selectedFile?: ProjectFile) => {
    const currentFile = selectedFile || currentFiles[type]
    if (!currentFile) {
      return
    }

    setDownloadingType(type)
    try {
      await fileService.downloadFile(currentFile.id, getFileName(currentFile))
    } catch (error: any) {
      message.error(getErrorMessage(error, '下载失败'))
    } finally {
      setDownloadingType((current) => (current === type ? null : current))
    }
  }

  const handleDelete = async (type: BiddingFileType, selectedFile?: ProjectFile) => {
    const currentFile = selectedFile || currentFiles[type]
    if (!currentFile) {
      return
    }

    setDeletingType(type)
    await deleteMutation.mutateAsync({ type, fileId: currentFile.id })
  }

  const handleToggleRequirement = async (
    type: BiddingFileType,
    notRequired: boolean
  ) => {
    setUpdatingRequirementType(type)
    await requirementMutation.mutateAsync({ type, notRequired })
  }

  return (
    <>
      <section className="bidding-files">
        <div className="bidding-files__header">
          <SectionTitle icon={<FileSearchOutlined />}>招投标信息</SectionTitle>
          <span className="bidding-files__summary">
            已处理 {handledCount} / {requiredFileTypes.length}
          </span>
        </div>

        {!automaticBidding?.engagement_method ? (
          <div className="bidding-files__overview">
            <div className="bidding-files__overview-item">
              <span className="bidding-files__overview-label">文件清单</span>
              <strong className="bidding-files__overview-value">请先选择项目承接方式</strong>
              <span className="bidding-files__overview-note">选择后系统自动确定招标文件是否必传</span>
            </div>
          </div>
        ) : (
          <>

        <div className="bidding-files__overview">
          <div className="bidding-files__overview-item">
            <span className="bidding-files__overview-label">当前状态</span>
            <strong className="bidding-files__overview-value">
              {missingTypes.length === 0
                ? '资料已处理'
                : `还缺 ${missingTypes.length} 份`}
            </strong>
            {notRequiredTypes.length > 0 && (
              <span className="bidding-files__overview-note">
                {notRequiredTypes.length} 份已标记不需要
              </span>
            )}
          </div>
          <div className="bidding-files__overview-item">
            <span className="bidding-files__overview-label">最近更新</span>
            <strong className="bidding-files__overview-value">
              {latestUpdated ? getTypeLabel(latestUpdated.type) : '暂无记录'}
            </strong>
            <span className="bidding-files__overview-note">
              {latestUpdated
                ? dayjs(latestUpdated.file.created_at).format('YYYY-MM-DD')
                : notRequiredTypes.length > 0
                  ? '暂无文件更新'
                  : '等待首次上传'}
            </span>
          </div>
          <div className="bidding-files__overview-item bidding-files__overview-item--action">
            <span className="bidding-files__overview-label">下一步</span>
            <strong className="bidding-files__overview-value">
              {missingTypes.length > 0
                ? `补传${getTypeLabel(missingTypes[0])}`
                : notRequiredTypes.length > 0
                  ? '如需上传先恢复需要'
                  : '如需更新可直接覆盖上传'}
            </strong>
          </div>
        </div>

        <div className="bidding-files__grid">
          {requiredFileTypes.map((type) => {
            const meta = FILE_META[type]
            const file = currentFiles[type]
            const files = filesByType[type]
            const style = {
              '--bidding-slot-accent': meta.accent,
              '--bidding-slot-accent-text': meta.accentText,
              '--bidding-slot-soft': meta.soft,
            } as CSSProperties
            const isFilled = !!file
            const isUploading = uploadingType === type
            const isDownloading = downloadingType === type
            const isDeleting = deletingType === type
            const isUpdatingRequirement = updatingRequirementType === type
            const isNotRequired = !isFilled && notRequiredTypes.includes(type)

            return (
              <article
                key={type}
                className={`bidding-file-slot ${
                  isFilled
                    ? 'bidding-file-slot--filled'
                    : isNotRequired
                      ? 'bidding-file-slot--not-required'
                      : 'bidding-file-slot--empty'
                }`}
                style={style}
              >
                <div className="bidding-file-slot__head">
                  <div className="bidding-file-slot__title-group">
                    <span className="bidding-file-slot__dot" />
                    <div className="bidding-file-slot__title-copy">
                      <h3 className="bidding-file-slot__title">{meta.title}</h3>
                    </div>
                  </div>
                  <span
                    className={`bidding-file-slot__status ${
                      isFilled ? 'bidding-file-slot__status--filled' : ''
                    } ${
                      isNotRequired
                        ? 'bidding-file-slot__status--not-required'
                        : ''
                    }`}
                  >
                    {isFilled && <CheckCircleFilled />}
                    {isNotRequired && <StopOutlined />}
                    {isUploading
                      ? '上传中'
                      : isFilled
                        ? '已上传'
                        : isNotRequired
                          ? '不需要'
                          : canUpload
                            ? '待补齐'
                            : '暂无文件'}
                  </span>
                </div>

                {isFilled ? (
                  <div className="bidding-file-slot__body bidding-file-slot__body--filled">
                    <div
                      className="bidding-file-slot__file-mark"
                      aria-hidden="true"
                    >
                      <FileTextOutlined />
                      <span>{getFileBadge(file)}</span>
                    </div>
                    <div className="bidding-file-slot__content">
                      {files.map((item) => (
                        <div key={item.id} className="bidding-file-slot__file-entry">
                          <button
                            type="button"
                            className="bidding-file-slot__name"
                            title={getFileName(item)}
                            onClick={() => setPreviewFile(item)}
                          >
                            {getFileName(item)}
                          </button>
                          <div className="bidding-file-slot__meta">
                            <span>{formatFileSize(item.file_size)}</span>
                            <span className="bidding-file-slot__meta-separator">·</span>
                            <span>{dayjs(item.created_at).format('YYYY-MM-DD')}</span>
                            <Button type="link" size="small" onClick={() => handleDownload(type, item)}>
                              下载
                            </Button>
                            {canDelete && (
                              <DeleteConfirmButton buttonProps={{ type: "link", size: "small", danger: true, disabled: isDeleting }} buttonText="移除" confirmText="确认移除" title="确认移除材料" description="只移除本项目材料引用，文件与历史保留。" loading={isDeleting} onConfirm={() => handleDelete(type, item)} />
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <div
                    className={`bidding-file-slot__body ${
                      isNotRequired
                        ? 'bidding-file-slot__body--not-required'
                        : 'bidding-file-slot__body--empty'
                    }`}
                  >
                    <div
                      className="bidding-file-slot__file-mark"
                      aria-hidden="true"
                    >
                      {isNotRequired ? <StopOutlined /> : <InboxOutlined />}
                      <span>{isNotRequired ? '免传' : '空位'}</span>
                    </div>
                    <div className="bidding-file-slot__content">
                      <div className="bidding-file-slot__empty-title">
                        {isNotRequired ? '无需上传' : '暂未上传'}
                      </div>
                      <div className="bidding-file-slot__empty-text">
                        {isNotRequired
                          ? `该项目已标记无需${meta.title}`
                          : meta.hint}
                      </div>
                    </div>
                  </div>
                )}

                <div className="bidding-file-slot__actions">
                  {isNotRequired ? (
                    <Button
                      size="small"
                      className="bidding-file-slot__action-button bidding-file-slot__action-button--primary"
                      icon={<ReloadOutlined />}
                      loading={isUpdatingRequirement}
                      disabled={!canUpload || isUpdatingRequirement}
                      onClick={() => handleToggleRequirement(type, false)}
                    >
                      恢复需要
                    </Button>
                  ) : (
                    <Upload
                      showUploadList={false}
                      beforeUpload={handleUploadSelect(type)}
                      multiple
                      disabled={!canUpload || isUploading}
                      accept=".pdf,.doc,.docx,.xls,.xlsx,.zip,.rar,.jpg,.jpeg,.png"
                    >
                      <Button
                        size="small"
                        className="bidding-file-slot__action-button bidding-file-slot__action-button--primary"
                        icon={<UploadOutlined />}
                        loading={isUploading}
                        disabled={!canUpload || isUploading}
                      >
                        {isFilled ? '继续上传' : '上传文件'}
                      </Button>
                    </Upload>
                  )}
                  <div className="bidding-file-slot__actions-secondary">
                    {isFilled ? (
                      <>
                        <Button
                          size="small"
                          className="bidding-file-slot__action-button bidding-file-slot__action-button--secondary"
                          icon={<EyeOutlined />}
                          onClick={() => file && setPreviewFile(file)}
                          disabled={isUploading}
                        >
                          预览
                        </Button>
                        <Button
                          size="small"
                          className="bidding-file-slot__action-button bidding-file-slot__action-button--secondary"
                          icon={<DownloadOutlined />}
                          onClick={() => handleDownload(type)}
                          disabled={isUploading || isDownloading}
                        >
                          下载
                        </Button>
                        {canDelete && (
                          <DeleteConfirmButton
                            title={`确认移除${meta.title}`}
                            description="只移除本项目的材料引用，文件与历史保留。"
                            details={[
                              { label: '文件位', value: meta.title },
                              {
                                label: '文件',
                                value: file?.original_name || '-',
                              },
                            ]}
                            buttonProps={{
                              size: 'small',
                              type: 'text',
                              className:
                                'bidding-file-slot__action-button bidding-file-slot__action-button--delete',
                              icon: <DeleteOutlined />,
                              disabled: isUploading || isDeleting,
                            }}
                            loading={isDeleting}
                            buttonText="移除"
                            confirmText="确认移除"
                            onConfirm={() => handleDelete(type)}
                          />
                        )}
                      </>
                    ) : null}
                  </div>
                </div>
              </article>
            )
          })}
        </div>
          </>
        )}
      </section>
      <FilePreviewDrawer
        open={!!previewFile}
        file={previewFile}
        onClose={() => setPreviewFile(null)}
        mask
      />
    </>
  )
}
