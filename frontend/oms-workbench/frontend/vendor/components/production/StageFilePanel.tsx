import { useState, type ComponentType, type ReactNode } from 'react'
import { Alert, Button, Empty, Spin, message } from 'antd'
import {
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useProductionStageService } from '@/production-stage-ports'
import { fileService } from '@/services/file'
import { useProductionStagePermissions } from '@/production-stage-ports'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import { FilePreviewDrawer } from '@/components/shared/FilePreviewDrawer'
import { isCadFileName } from '@/utils/upload'
import type { File as ProjectFile, ProductionFile, ProductionStage, StageFileInfo } from '@/types'
import dayjs from 'dayjs'
import '../project/ProjectWorkspace.css'
import './StageFilePanel.css'

type StageEditComponent = ComponentType<{
  projectId: string | number
  visible: boolean
  stageInfo?: StageFileInfo
  canDeleteExisting?: boolean
  onCancel: () => void
  onSuccess: () => void
}>

interface StageFilePanelProps {
  projectId: string | number
  stage: ProductionStage
  title: string
  mainFileLabel: string
  icon: ReactNode
  tone: 'scheme' | 'preliminary' | 'construction'
  EditComponent: StageEditComponent
}

const formatFileSize = (bytes?: number): string => {
  if (!bytes) return '-'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${Math.round((bytes / Math.pow(k, i)) * 100) / 100} ${sizes[i]}`
}

const getFileBadge = (fileName: string): string =>
  fileName.split('.').pop()?.slice(0, 4).toUpperCase() || 'FILE'

export const StageFilePanel = ({
  projectId,
  stage,
  title,
  mainFileLabel,
  icon,
  tone,
  EditComponent,
}: StageFilePanelProps) => {
  const productionService = useProductionStageService(projectId)
  const [editDrawerOpen, setEditDrawerOpen] = useState(false)
  const [previewFile, setPreviewFile] = useState<ProjectFile | null>(null)
  const queryClient = useQueryClient()
  const projectPermissions = useProductionStagePermissions(projectId)
  const canUpload = projectPermissions.canUploadProductionFile
  const canDownload = projectPermissions.canDownloadProductionFile
  const canDelete = projectPermissions.canDeleteProductionFile

  const {
    data: stageInfo,
    isLoading,
    error,
    refetch,
  } = useQuery<StageFileInfo>({
    queryKey: ['productionFilesByStage', projectId, stage],
    queryFn: () => productionService.getProductionFilesByStage(projectId, stage),
    enabled: !!projectId,
  })

  const deleteFileMutation = useMutation({
    mutationFn: (productionFileId: string) =>
      productionService.deleteProductionFile(projectId, productionFileId),
    onSuccess: () => {
      message.success('删除成功')
      queryClient.invalidateQueries({
        queryKey: ['productionFilesByStage', projectId, stage],
      })
      queryClient.invalidateQueries({
        queryKey: ['projectProgressAssistant', String(projectId)],
      })
    },
    onError: (error: any) => {
      message.error(error?.message || '删除失败')
    },
  })

  const handleDownload = async (fileId: string, fileName?: string) => {
    try {
      await fileService.downloadFile(fileId, fileName)
    } catch (error: any) {
      message.error(error?.message || '下载失败')
    }
  }

  const mainFiles = stageInfo?.main_files || []
  const reviewSheet = stageInfo?.review_sheet
  const score = stageInfo?.score
  const isEmpty = mainFiles.length === 0 && !reviewSheet && score == null
  const fileProgress = mainFiles.length > 0 ? 1 : 0
  const requiredMissingItems = [
    mainFiles.length === 0 ? mainFileLabel : '',
  ].filter(Boolean)
  const latestUpdatedAt = [...mainFiles, ...(reviewSheet ? [reviewSheet] : [])]
    .map((item) => item.created_at)
    .filter(Boolean)
    .sort((a, b) => dayjs(b).valueOf() - dayjs(a).valueOf())[0]

  // Keep the same mounted editor through a failed/refetching latest read.
  // A source error must not discard selected files or close a pending intent.
  const editor = <EditComponent
    projectId={projectId}
    visible={editDrawerOpen}
    stageInfo={stageInfo}
    canDeleteExisting={canDelete && !error && !isLoading}
    onCancel={() => setEditDrawerOpen(false)}
    onSuccess={async () => {
      setEditDrawerOpen(false)
      await queryClient.invalidateQueries({queryKey: ['productionFilesByStage', projectId, stage]})
      await queryClient.invalidateQueries({queryKey: ['projectProgressAssistant', String(projectId)]})
      await refetch()
    }}
  />

  const renderFileList = (files: ProductionFile[], emptyText: string) => {
    if (files.length === 0) {
      return (
        <div className="stage-file-panel__empty-block">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
        </div>
      )
    }

    return (
      <div className="stage-file-panel__file-list">
        {files.map((item) => {
          const file = item.file
          if (!file) return null
          const isCad = isCadFileName(file.original_name)

          return (
            <article className="stage-file-panel__file" key={item.id}>
              <div className="stage-file-panel__file-main">
                <span className="stage-file-panel__file-icon">
                  {getFileBadge(file.original_name)}
                </span>
                <div>
                  {isCad ? (
                    <strong className="stage-file-panel__file-name stage-file-panel__file-name--static">
                      {file.original_name}
                    </strong>
                  ) : (
                    <button
                      type="button"
                      className="stage-file-panel__file-name"
                      onClick={() => setPreviewFile(file)}
                    >
                      {file.original_name}
                    </button>
                  )}
                  <div className="stage-file-panel__file-meta">
                    {formatFileSize(file.file_size)} ·{' '}
                    {dayjs(item.created_at).format('YYYY-MM-DD HH:mm')}
                  </div>
                </div>
              </div>
              <div className="stage-file-panel__file-actions">
                {!isCad && (
                  <Button
                    type="text"
                    size="small"
                    icon={<EyeOutlined />}
                    onClick={() => setPreviewFile(file)}
                  >
                    预览
                  </Button>
                )}
                {canDownload && (
                  <Button
                    type="text"
                    size="small"
                    icon={<DownloadOutlined />}
                    onClick={() => handleDownload(file.id, file.original_name)}
                  >
                    下载
                  </Button>
                )}
                {canDelete && (
                  <DeleteConfirmButton
                    title="确认删除文件"
                    description="移出阶段资料并保留引用历史，不删除文件字节。"
                    details={[
                      { label: '文件', value: file.original_name },
                      {
                        label: '上传时间',
                        value: dayjs(item.created_at).format('YYYY-MM-DD'),
                      },
                    ]}
                    buttonProps={{ size: 'small' }}
                    loading={deleteFileMutation.isPending}
                    onConfirm={() => deleteFileMutation.mutate(item.id)}
                  />
                )}
              </div>
            </article>
          )
        })}
      </div>
    )
  }

  if (isLoading) {
    return (
      <>
      <section className={`project-workspace__card stage-file-panel stage-file-panel--${tone}`}>
        <div className="stage-file-panel__loading">
          <Spin />
          <span>正在加载{title}资料...</span>
        </div>
      </section>
      {editor}
      </>
    )
  }

  if (error) {
    return (
      <>
      <section className={`project-workspace__card stage-file-panel stage-file-panel--${tone}`}>
        <Alert
          message="加载失败"
          description={error instanceof Error ? error.message : '加载阶段文件失败'}
          type="error"
          showIcon
          action={<Button onClick={() => refetch()}>重试</Button>}
        />
      </section>
      {editor}
      </>
    )
  }

  return (
    <>
      <section className={`project-workspace__card stage-file-panel stage-file-panel--${tone}`}>
        <header className="stage-file-panel__header">
          <div>
            <SectionTitle icon={icon}>{title}</SectionTitle>
            <p className="stage-file-panel__subtitle">
              维护{mainFileLabel}，校审单与评分可按实际情况补充。
            </p>
          </div>
          {canUpload && (
            isEmpty ? (
              <CreateActionButton
                size="small"
                compact
                onClick={() => setEditDrawerOpen(true)}
              >
                新建阶段资料
              </CreateActionButton>
            ) : (
              <Button
                size="small"
                icon={<EditOutlined />}
                onClick={() => setEditDrawerOpen(true)}
              >
                编辑阶段资料
              </Button>
            )
          )}
        </header>

        <div className="stage-file-panel__summary">
          <div className="stage-file-panel__summary-item stage-file-panel__summary-item--state">
            <span>阶段状态</span>
            <strong>{requiredMissingItems.length === 0 ? '主文件已就位' : '待上传主文件'}</strong>
            <em>
              {requiredMissingItems.length === 0
                ? '校审单与评分为可选项'
                : requiredMissingItems.join('、')}
            </em>
          </div>
          <div className="stage-file-panel__summary-item stage-file-panel__summary-item--progress">
            <span>文件进度</span>
            <strong>{fileProgress}/1 项</strong>
            <em>{fileProgress === 1 ? '主文件已保存' : '先上传阶段主文件'}</em>
          </div>
          <div className="stage-file-panel__summary-item stage-file-panel__summary-item--time">
            <span>最近维护</span>
            <strong>{latestUpdatedAt ? dayjs(latestUpdatedAt).format('MM-DD HH:mm') : '-'}</strong>
            <em>{latestUpdatedAt ? '来自阶段文件更新' : '暂无维护记录'}</em>
          </div>
        </div>

        {isEmpty ? (
          <div className="stage-file-panel__empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={canUpload ? '暂无阶段资料，请先新建' : '暂无阶段资料'}
            />
          </div>
        ) : (
          <div className="stage-file-panel__body">
            <div className="stage-file-panel__files-stack">
              <section className="stage-file-panel__section stage-file-panel__section--main">
                <div className="stage-file-panel__section-title">
                  <span>{mainFileLabel}</span>
                  <em>{mainFiles.length} 份</em>
                </div>
                {renderFileList(mainFiles, `暂无${mainFileLabel}`)}
              </section>
              <section className="stage-file-panel__section stage-file-panel__section--review">
                <div className="stage-file-panel__section-title">
                  <span>校审单</span>
                  <em>{reviewSheet ? '已上传' : '可选'}</em>
                </div>
                {reviewSheet
                  ? renderFileList([reviewSheet], '暂无校审单')
                  : renderFileList([], '暂无校审单')}
              </section>
            </div>
            <aside className="stage-file-panel__score-card">
              <div className="stage-file-panel__score-label">阶段评分</div>
              <div className="stage-file-panel__score">
                <strong>{score != null ? score : '-'}</strong>
                <span>分</span>
              </div>
              <p>可选，校审完成后按需录入。</p>
            </aside>
          </div>
        )}
      </section>

      {editor}
      <FilePreviewDrawer
        open={!!previewFile}
        file={previewFile}
        onClose={() => setPreviewFile(null)}
      />
    </>
  )
}
