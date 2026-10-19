import { useState, useEffect } from 'react'
import {
  Drawer,
  Form,
  InputNumber,
  Button,
  message,
  Typography,
  Space,
  Alert,
} from 'antd'
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query'
import { useProductionStageService, useProductionStagePort } from '@/production-stage-ports'
import { fileService } from '@/services/file'
import { InlineFilePreviewLayout } from '@/components/shared/InlineFilePreviewLayout'
import { ProductionStageFileField } from './ProductionStageFileField'
import { getErrorMessage } from '@/utils/error'
import type { UploadFile } from 'antd'
import type { File as ProjectFile, StageFileInfo } from '@/types'
import './ProductionStageEditModal.css'

const { Text } = Typography

interface NativeStageEditModalProps {
  projectId: string | number
  visible: boolean
  stageInfo?: StageFileInfo
  canDeleteExisting?: boolean
  onCancel: () => void
  onSuccess: () => void
}

export const NativeStageEditModal = ({
  projectId,
  visible,
  stageInfo,
  canDeleteExisting = true,
  onCancel,
  onSuccess,
  stage,
  drawerTitle,
  mainFileLabel,
}: NativeStageEditModalProps & {stage:'scheme'|'preliminary'|'construction';drawerTitle:string;mainFileLabel:string}) => {
  const productionService = useProductionStageService(projectId)
  const stagePort = useProductionStagePort(projectId)
  if (stagePort.stage !== stage) throw Error('原阶段表单与原生端口不匹配')
  const maxSizeLabel = stagePort.maxUploadBytes ? `${stagePort.maxUploadBytes / (1024 * 1024)}MB` : '500MB'
  const [form] = Form.useForm()
  const [mainFileList, setMainFileList] = useState<UploadFile[]>([])
  const [reviewSheetFileList, setReviewSheetFileList] = useState<UploadFile[]>([])
  const [previewFile, setPreviewFile] = useState<ProjectFile | null>(null)
  const [previewUploadFile, setPreviewUploadFile] = useState<UploadFile | null>(null)
  const [uploading, setUploading] = useState(false)
  const queryClient = useQueryClient()
  const hasPreview = Boolean(previewFile || previewUploadFile)

  // 在弹窗内重新获取最新数据（类似 ApprovalAuditModal 的做法）
  const { data: latestStageInfo, isLoading: isLoadingLatest, error: latestStageError, refetch: refetchLatest } = useQuery<StageFileInfo>({
    queryKey: ['productionFilesByStage', projectId, stage],
    queryFn: () => productionService.getProductionFilesByStage(projectId, stage),
    enabled: !!projectId && visible,
  })

  // 使用最新获取的数据，如果没有则使用传入的 stageInfo（向后兼容）
  const currentStageInfo = latestStageInfo || stageInfo
  
  // 初始化表单数据
  useEffect(() => {
    if (visible && currentStageInfo) {
      form.setFieldsValue({
        score: currentStageInfo.score,
      })
    } else if (visible) {
      form.resetFields()
      setMainFileList([])
      setReviewSheetFileList([])
    }
  }, [visible, currentStageInfo, form])

  // One explicit stage intent replaces the monolith's partial mutation loop.
  // Native upload/command/replay semantics belong to the injected owner port.
  const deleteFileMutation = useMutation({
    mutationFn: (id: string) => productionService.deleteProductionFile(projectId, id),
    onSuccess: () => { message.success('已移出阶段资料，历史保留'); onSuccess() },
    onError: (error: unknown) => message.error(getErrorMessage(error, '移出未确认')),
  })
  const handleSubmit = async () => {
    if (uploading || isLoadingLatest || latestStageError || !latestStageInfo) return
    try {
      const values = await form.validateFields()
      const newMainFiles = mainFileList.filter(file => file.originFileObj && !file.status)
      const newReviewSheets = reviewSheetFileList.filter(file => file.originFileObj && !file.status)
      if (!(currentStageInfo?.main_files?.length || 0) && !newMainFiles.length) {
        message.error(`请至少上传一个${mainFileLabel}`)
        return
      }
      setUploading(true)
      await stagePort.saveStage({
        mainFiles: newMainFiles.map(file => file.originFileObj as globalThis.File),
        reviewSheets: newReviewSheets.map(file => file.originFileObj as globalThis.File),
        // Empty input preserves the existing score; 0 is an explicit value.
        ...(values.score !== undefined && values.score !== null ? {score: values.score} : {}),
      })
      await queryClient.invalidateQueries({queryKey: ['productionFilesByStage', projectId, stage]})
      const refreshed = await refetchLatest()
      if (refreshed.isError || !refreshed.data) throw Error('阶段保存后的最终回读未确认，请复核原命令')
      message.success('保存成功')
      form.resetFields()
      setMainFileList([])
      setReviewSheetFileList([])
      setPreviewFile(null)
      setPreviewUploadFile(null)
      onSuccess()
    } catch (error) {
      message.error(getErrorMessage(error, '阶段保存未确认'))
    } finally {
      setUploading(false)
    }
  }

  const handleCancel = () => {
    if (uploading) return
    form.resetFields()
    setMainFileList([])
    setReviewSheetFileList([])
    setPreviewFile(null)
    setPreviewUploadFile(null)
    onCancel()
  }

  const handlePreviewFile = (file: ProjectFile) => {
    setPreviewFile(file)
    setPreviewUploadFile(null)
  }

  const handlePreviewUploadFile = (file: UploadFile) => {
    setPreviewUploadFile(file)
    setPreviewFile(null)
  }

  const handleClosePreview = () => {
    setPreviewFile(null)
    setPreviewUploadFile(null)
  }

  const handleDownloadFile = async (file: ProjectFile) => {
    try {
      await fileService.downloadFile(file.id, file.original_name)
    } catch (error: any) {
      message.error(error?.message || '下载失败')
    }
  }

  const mainFiles = currentStageInfo?.main_files || []
  const reviewSheets = currentStageInfo?.review_sheets ||
    (currentStageInfo?.review_sheet ? [currentStageInfo.review_sheet] : [])

  return (
    <Drawer
      title={drawerTitle}
      open={visible}
      onClose={handleCancel}
      width={hasPreview ? 'calc(100vw - 24px)' : 800}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={handleCancel}>取消</Button>
          <Button type="primary" onClick={handleSubmit} loading={uploading} disabled={isLoadingLatest || !!latestStageError || !latestStageInfo}>
            保存
          </Button>
        </Space>
      }
    >
      <InlineFilePreviewLayout
        className="production-stage-edit__layout"
        contentClassName="production-stage-edit__form-column"
        file={previewFile}
        uploadFile={previewUploadFile}
        onClosePreview={handleClosePreview}
        defaultPreviewWidth={820}
        minPreviewWidth={780}
        minContentWidth={460}
        minHeight="calc(100vh - 170px)"
        previewHeight="calc(100vh - 170px)"
      >
          <Form form={form} layout="vertical" className="production-stage-edit__form">
        {latestStageError && <Alert type="error" showIcon message="阶段最新资料未确认，未启用保存" description="读取失败不表示空资料，不使用旧表单覆盖事实。" action={<Button onClick={() => void refetchLatest()}>重试</Button>}/>} 
        {/* 方案文件上传区域 */}
        <Form.Item>
          <ProductionStageFileField
            label={mainFileLabel}
            required
            fileList={mainFileList}
            onFileListChange={setMainFileList}
            onRemovePendingFile={(uid) =>
              setMainFileList((files) => files.filter((file) => file.uid !== uid))
            }
            multiple
            existingFiles={mainFiles}
            helper={`必填，至少保留一个${mainFileLabel}。`}
            maxSizeBytes={stagePort.maxUploadBytes}
            uploadHint={`支持 PDF、DOC、DOCX、PNG、JPG 及 CAD（DWG、DXF），最大 ${maxSizeLabel}，可上传多个文件`}
            deleteTitle={`确认删除${mainFileLabel}`}
            deleteDescription="移出阶段资料并保留引用历史，不删除文件字节。"
            disabled={uploading}
            deleteLoading={deleteFileMutation.isPending}
            canDeleteExisting={canDeleteExisting}
            onPreviewFile={handlePreviewFile}
            onPreviewUploadFile={handlePreviewUploadFile}
            onDownloadFile={handleDownloadFile}
            onDeleteExistingFile={(file) => {
              if (previewFile?.id === file.file?.id) {
                handleClosePreview()
              }
              deleteFileMutation.mutate(file.id)
            }}
          />
        </Form.Item>

        {/* 校审单上传区域 */}
        <Form.Item>
          <ProductionStageFileField
            label="校审单（可选）"
            fileList={reviewSheetFileList}
            onFileListChange={setReviewSheetFileList}
            onRemovePendingFile={(uid) =>
              setReviewSheetFileList((files) => files.filter((file) => file.uid !== uid))
            }
            multiple
            maxCount={20}
            existingFiles={reviewSheets}
            maxSizeBytes={stagePort.maxUploadBytes}
            uploadHint={`支持 PDF、DOC、DOCX、PNG、JPG 及 CAD（DWG、DXF），最大 ${maxSizeLabel}，可上传多个文件`}
            deleteTitle="确认删除校审单"
            deleteDescription="移出校审单引用并保留历史，不删除文件字节。"
            disabled={uploading}
            deleteLoading={deleteFileMutation.isPending}
            canDeleteExisting={canDeleteExisting}
            onPreviewFile={handlePreviewFile}
            onPreviewUploadFile={handlePreviewUploadFile}
            onDownloadFile={handleDownloadFile}
            onDeleteExistingFile={(file) => {
              if (previewFile?.id === file.file?.id) {
                handleClosePreview()
              }
              deleteFileMutation.mutate(file.id)
            }}
          />
        </Form.Item>

        {/* 评分输入区域 */}
        <Form.Item
          name="score"
          label="评分（可选）"
          rules={[
            { validator: async (_, value) => {
              if (value == null || value === '') return
              const text = String(value)
              if (text.length > 128 || !/^[0-9]+(?:\.[0-9]+)?$/.test(text)) throw Error('评分必须在0-100之间')
              const [whole, fraction=''] = text.split('.')
              if (BigInt(whole) > 100n || (BigInt(whole) === 100n && /[1-9]/.test(fraction))) throw Error('评分必须在0-100之间')
            } },
          ]}
        >
          <InputNumber
            stringMode
            placeholder="请输入评分（0-100）"
            min={0}
            max={100}
            style={{ width: '100%', maxWidth: 200 }}
          />
        </Form.Item>
        <Text type="secondary" className="production-stage-edit__score-help">
          可选，如填写请输入0-100之间的分数
        </Text>
          </Form>
      </InlineFilePreviewLayout>
    </Drawer>
  )
}

export const SchemeStageEditModal = (props: NativeStageEditModalProps) => <NativeStageEditModal {...props} stage="scheme" drawerTitle="编辑方案阶段文件" mainFileLabel="方案文件" />
