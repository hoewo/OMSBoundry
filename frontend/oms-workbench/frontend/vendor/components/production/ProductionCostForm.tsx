import {
  Form,
  Input,
  Select,
  DatePicker,
  InputNumber,
  Button,
  message,
  Space,
} from 'antd'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { productionService } from '@/services/production'
import { fileService } from '@/services/file'
import { AttachmentUploader } from '@/components/shared/AttachmentUploader'
import type {
  CostScope,
  CreateProductionCostRequest,
  File as ProjectFile,
  ProductionCost,
  ProductionCostType,
} from '@/types'
import type { UploadFile } from 'antd'
import dayjs from 'dayjs'
import { getErrorMessage } from '@/utils/error'
import { isWithinUploadLimit, MAX_UPLOAD_SIZE_LABEL } from '@/utils/upload'

interface ProductionCostFormProps {
  projectId: string | number
  scope?: CostScope
  initialData?: ProductionCost
  readOnly?: boolean
  canManageFiles?: boolean
  onSuccess?: () => void
  onCancel?: () => void
  onPreviewFile?: (file: ProjectFile) => void
  onPreviewUploadFile?: (file: UploadFile | null) => void
  onDownloadFile?: (file: ProjectFile) => void
  onClearPreview?: () => void
  onExistingInvoiceRemoved?: () => void
  onExistingInvoiceRestored?: () => void
}

const COST_TYPE_LABELS: Record<ProductionCostType, string> = {
  vehicle: '用车',
  accommodation: '住宿',
  transport: '交通',
  expert_fee: '专家费',
  bid_signup_fee: '投标报名费',
  bid_service_fee: '中标服务费',
  other: '其他',
}

const COST_TYPE_OPTIONS: ProductionCostType[] = [
  'vehicle',
  'accommodation',
  'transport',
  'expert_fee',
  'bid_signup_fee',
  'bid_service_fee',
  'other',
]

const BUSINESS_ONLY_COST_TYPES = new Set<ProductionCostType>([
  'bid_signup_fee',
  'bid_service_fee',
])

const getCostTypeLabel = (costType: ProductionCostType, scope: CostScope) => {
  if (costType === 'expert_fee') {
    return scope === 'business' ? '评标专家费' : '方案评审专家费'
  }

  return COST_TYPE_LABELS[costType]
}

export const ProductionCostForm = ({
  projectId,
  scope = 'production',
  initialData,
  readOnly = false,
  canManageFiles = false,
  onSuccess,
  onCancel,
  onPreviewFile,
  onPreviewUploadFile,
  onDownloadFile,
  onClearPreview,
  onExistingInvoiceRemoved,
  onExistingInvoiceRestored,
}: ProductionCostFormProps) => {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()
  const [invoiceFileList, setInvoiceFileList] = useState<UploadFile[]>([])
  const [uploadingInvoice, setUploadingInvoice] = useState(false)
  const existingInvoices = useMemo<ProjectFile[]>(() => initialData
    ? initialData.invoice_files || (initialData.invoice_file ? [initialData.invoice_file] : [])
    : productionService.stagedFiles(projectId, scope), [projectId,scope,initialData?.id,initialData?.revision])

  const existingInvoiceFileList = useMemo<UploadFile[]>(() => {
    return existingInvoices.map((file) => ({
        uid: file.id,
        name: file.original_name || file.file_name || '发票文件',
        status: 'done',
        size: file.file_size,
        type: file.mime_type,
        response: file,
      }))
  }, [existingInvoices])

  useEffect(() => {
    form.setFieldsValue({
      invoice_file_id: initialData?.invoice_file_id,
    })
    setInvoiceFileList(existingInvoiceFileList)
  }, [existingInvoiceFileList, form, initialData?.invoice_file_id])

  const createMutation = useMutation({
    mutationFn: (data: CreateProductionCostRequest) =>
      productionService.createCost(projectId, data, scope),
    onSuccess: () => {
      message.success(scope === 'business' ? '经营成本创建成功' : '生产成本创建成功')
      form.resetFields()
      queryClient.invalidateQueries({
        queryKey: ['costs', projectId, scope],
      })
      queryClient.invalidateQueries({
        queryKey: ['costStatistics', projectId, scope],
      })
      queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
      onSuccess?.()
    },
    onError: (error: any) => {
      message.error(getErrorMessage(error, '创建失败'))
    },
  })

  const updateMutation = useMutation({
    mutationFn: (data: CreateProductionCostRequest) => {
      if (!initialData?.id) {
        throw new Error('缺少成本记录ID')
      }
      return productionService.updateCost(projectId, initialData.id, data, scope)
    },
    onSuccess: () => {
      message.success(scope === 'business' ? '经营成本更新成功' : '生产成本更新成功')
      queryClient.invalidateQueries({
        queryKey: ['costs', projectId, scope],
      })
      queryClient.invalidateQueries({
        queryKey: ['costStatistics', projectId, scope],
      })
      queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
      onSuccess?.()
    },
    onError: (error: any) => {
      message.error(getErrorMessage(error, '更新失败'))
    },
  })

  const isSubmitting =
    uploadingInvoice || createMutation.isPending || updateMutation.isPending

  const handleFinish = async (values: any) => {
    let mutationStarted = false
    try {
      const invoiceFileIds: string[] = []
      for (const item of invoiceFileList) {
        const persistedFile = item.response as ProjectFile | undefined
        const file = item.originFileObj as globalThis.File | undefined
        if (file && item.status !== 'done') {
          setUploadingInvoice(true)
          const uploadedFile = await fileService.uploadCostFile(
            String(projectId),
            scope,
            file
          )
          invoiceFileIds.push(uploadedFile.id)
          // Keep the completed Evidence identity if the owner command fails.
          item.response = uploadedFile
          item.status = 'done'
          setInvoiceFileList((files) => [...files])
        } else {
          invoiceFileIds.push(persistedFile?.id || String(item.uid))
        }
      }

      const data: CreateProductionCostRequest = {
        cost_type: values.cost_type,
        cost_scope: scope,
        amount: values.amount,
        description: values.description,
        incurred_at: values.incurred_at
          ? dayjs(values.incurred_at).format('YYYY-MM-DD')
          : undefined,
        invoice_file_id: invoiceFileIds[0],
        invoice_file_ids: invoiceFileIds,
        reason: values.reason,
      }

      if (initialData) {
        mutationStarted = true
        await updateMutation.mutateAsync(data)
      } else {
        mutationStarted = true
        await createMutation.mutateAsync(data)
      }
    } catch (error: any) {
      // A failed business command never deletes already stored Evidence bytes.
      if (!mutationStarted) {
        message.error(getErrorMessage(error, '操作失败'))
      }
    } finally {
      setUploadingInvoice(false)
    }
  }

  const beforeUpload = (file: globalThis.File) => {
    if (!isWithinUploadLimit(file)) {
      message.error(`文件大小不能超过${MAX_UPLOAD_SIZE_LABEL}`)
      return false
    }
    return false
  }

  const handlePreviewInvoice = (file: UploadFile) => {
    if (file.originFileObj) {
      onPreviewUploadFile?.(file)
      return
    }
    const persisted = file.response as ProjectFile | undefined
    if (persisted) onPreviewFile?.(persisted)
  }

  const handleDownloadInvoice = (file: UploadFile) => {
    const persisted = file.response as ProjectFile | undefined
    if (persisted) onDownloadFile?.(persisted)
  }

  return (
    <Form
      form={form}
      layout="vertical"
      onFinish={handleFinish}
      disabled={readOnly}
      initialValues={
        initialData
          ? {
              cost_type: initialData.cost_type,
              amount: initialData.amount,
              description: initialData.description,
              incurred_at: initialData.incurred_at
                ? dayjs(initialData.incurred_at)
                : undefined,
              invoice_file_id: initialData.invoice_file_id,
            }
          : undefined
      }
    >
      <Form.Item
        name="cost_type"
        label="成本类型"
        rules={[{ required: true, message: '请选择成本类型' }]}
      >
        <Select placeholder="请选择成本类型">
          {COST_TYPE_OPTIONS.filter(
            (value) => scope === 'business' || !BUSINESS_ONLY_COST_TYPES.has(value)
          ).map((value) => (
            <Select.Option key={value} value={value}>
              {getCostTypeLabel(value, scope)}
            </Select.Option>
          ))}
        </Select>
      </Form.Item>

      <Form.Item
        name="amount"
        label="金额"
        rules={[{ required: true, message: '请输入金额' }]}
      >
        <InputNumber<string>
          stringMode
          placeholder="请输入金额"
          min="0"
          style={{ width: '100%' }}
          precision={2}
        />
      </Form.Item>

      <Form.Item name="description" label="描述">
        <Input.TextArea rows={3} placeholder="请输入描述信息（可选）" />
      </Form.Item>

      <Form.Item name="incurred_at" label="发生时间">
        <DatePicker style={{ width: '100%' }} />
      </Form.Item>
      {initialData && !readOnly && <Form.Item name="reason" label="纠正原因" rules={[{required:true,message:'请说明纠正原因，原版本会保留'}]}><Input.TextArea maxLength={500} rows={2} /></Form.Item>}

      <Form.Item name="invoice_file_id" hidden>
        <Input />
      </Form.Item>

      <Form.Item label="上传发票（可选）">
        <AttachmentUploader
          helper={`支持 PDF、PNG、JPG，单个文件最大 ${MAX_UPLOAD_SIZE_LABEL}`}
          fileList={invoiceFileList}
          accept=".pdf,.png,.jpg,.jpeg"
          beforeUpload={beforeUpload}
          onChange={(files) => {
            setInvoiceFileList(files)
            if (files.length === 0) {
              onExistingInvoiceRemoved?.()
              onClearPreview?.()
            } else {
              onExistingInvoiceRestored?.()
            }
          }}
          disabled={readOnly || isSubmitting || !canManageFiles}
          onPreview={handlePreviewInvoice}
          onDownload={handleDownloadInvoice}
        />
      </Form.Item>

      <Form.Item>
        <Space>
          {!readOnly && <Button type="primary" htmlType="submit" loading={isSubmitting}>
            {initialData ? '追加纠正' : '提交'}
          </Button>}
          {onCancel && <Button onClick={onCancel}>取消</Button>}
        </Space>
      </Form.Item>
    </Form>
  )
}
