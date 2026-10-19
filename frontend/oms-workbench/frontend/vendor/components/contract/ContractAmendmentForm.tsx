import { useState, useEffect } from 'react'
import {
  Form,
  Input,
  DatePicker,
  Button,
  Space,
  message,
  Row,
  Col,
  InputNumber,
  Select,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { permissionService } from '@/services/permission'
import { fileService } from '@/services/file'
import type {
  ContractAmendment,
  ContractAmendmentChangeType,
  CreateContractAmendmentRequest,
  File as ProjectFile,
} from '@/types'
import dayjs from 'dayjs'
import type { UploadFile } from 'antd'
import { errorIncludes, getErrorMessage } from '@/utils/error'
import { isWithinUploadLimit, MAX_UPLOAD_SIZE_LABEL } from '@/utils/upload'
import { getContractEffectiveAmount } from '@/utils/contractAmounts'
import { formatMoney } from '@/utils/money'
import {sumMoney,nonnegative,rateBPS} from '@/contract-money.mjs'
import { AttachmentUploader } from '@/components/shared/AttachmentUploader'
import {ContractMaterials} from '@/contract-materials'

const { TextArea } = Input

interface ContractAmendmentFormProps {
  projectId: string
  contractId: string
  amendmentId?: string
  onSuccess?: () => void
  onCancel?: () => void
}

export const ContractAmendmentForm = ({
  projectId,
  contractId,
  amendmentId,
  onSuccess,
  onCancel,
}: ContractAmendmentFormProps) => {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()
  const [fileList, setFileList] = useState<UploadFile[]>([])

  // Check permission
  const { data: canManage } = useQuery({
    queryKey: ['canManageContractAmendments', projectId,contractId],
    queryFn: () => permissionService.canManageContractAmendments(projectId,contractId),
    enabled: !!projectId,
  })
  const { data: contract } = useQuery({
    queryKey: ['contract', contractId],
    queryFn: () => businessService.getContract(contractId),
    enabled: !!contractId,
  })

  // Load existing amendment data if editing
  const { data: existingAmendment, isLoading: isLoadingAmendment } = useQuery({
    queryKey: ['contractAmendment', amendmentId],
    queryFn: async () => {
      if (!amendmentId) return null
      const amendments = await businessService.getContractAmendments(projectId, contractId)
      return amendments.find((a) => a.id === amendmentId) || null
    },
    enabled: !!amendmentId,
  })

  // Set form values when amendment data is loaded
  useEffect(() => {
    if (existingAmendment) {
      form.setFieldsValue({
        amendment_number: existingAmendment.amendment_number,
        sign_date: existingAmendment.sign_date ? dayjs(existingAmendment.sign_date) : null,
        change_type: existingAmendment.change_type || 'adjustment',
        design_fee: existingAmendment.design_fee,
        survey_fee: existingAmendment.survey_fee,
        consultation_fee: existingAmendment.consultation_fee,
        contract_rate: existingAmendment.contract_rate,
        description: existingAmendment.description,
      })
      const files = existingAmendment.amendment_files?.length
        ? existingAmendment.amendment_files
        : existingAmendment.amendment_file
          ? [existingAmendment.amendment_file]
          : []
      setFileList(files.map((file) => ({
        uid: file.id,
        name: file.original_name,
        size: file.file_size,
        status: 'done',
        response: file,
      })))
    }
  }, [existingAmendment, form])

  const createMutation = useMutation({
    mutationFn: (data: CreateContractAmendmentRequest) =>
      businessService.createContractAmendment(projectId, contractId, data),
    onSuccess: () => {
      message.success('补充协议创建成功')
      queryClient.invalidateQueries({
        queryKey: ['contractAmendments', projectId, contractId],
      })
      queryClient.invalidateQueries({
        queryKey: ['projectAmendments', projectId],
      })
      queryClient.invalidateQueries({ queryKey: ['contracts', projectId] })
      queryClient.invalidateQueries({ queryKey: ['contract', contractId] })
      queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
      queryClient.invalidateQueries({ queryKey: ['businessStatistics', projectId] })
      queryClient.invalidateQueries({ queryKey: ['contractAmounts', projectId] })
      queryClient.invalidateQueries({ queryKey: ['companyRevenueSummary'] })
      queryClient.invalidateQueries({ queryKey: ['companyRevenueStatistics'] })
      queryClient.invalidateQueries({ queryKey: ['projectRevenueLedgersV2'] })
      form.resetFields()
      setFileList([])
      onSuccess?.()
    },
    onError: (error: any) => {
      if (errorIncludes(error, 'amendment number already exists', '补充协议编号已存在')) {
        form.setFields([{ name: 'amendment_number', errors: ['补充协议编号已存在'] }])
      }
      message.error(
        errorIncludes(error, 'negative effective amount')
          ? '补充协议生效后，合同总额及各项费用均不能小于 0'
          : getErrorMessage(error, '创建失败')
      )
    },
  })

  const updateMutation = useMutation({
    mutationFn: (data: Partial<CreateContractAmendmentRequest>) =>
      businessService.updateContractAmendment(projectId, contractId, amendmentId!, data),
    onSuccess: () => {
      message.success('补充协议更新成功')
      queryClient.invalidateQueries({
        queryKey: ['contractAmendments', projectId, contractId],
      })
      queryClient.invalidateQueries({
        queryKey: ['projectAmendments', projectId],
      })
      queryClient.invalidateQueries({
        queryKey: ['contractAmendment', amendmentId],
      })
      queryClient.invalidateQueries({ queryKey: ['contracts', projectId] })
      queryClient.invalidateQueries({ queryKey: ['contract', contractId] })
      queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
      queryClient.invalidateQueries({ queryKey: ['businessStatistics', projectId] })
      queryClient.invalidateQueries({ queryKey: ['contractAmounts', projectId] })
      queryClient.invalidateQueries({ queryKey: ['companyRevenueSummary'] })
      queryClient.invalidateQueries({ queryKey: ['companyRevenueStatistics'] })
      queryClient.invalidateQueries({ queryKey: ['projectRevenueLedgersV2'] })
      onSuccess?.()
    },
    onError: (error: any) => {
      if (errorIncludes(error, 'amendment number already exists', '补充协议编号已存在')) {
        form.setFields([{ name: 'amendment_number', errors: ['补充协议编号已存在'] }])
      }
      message.error(
        errorIncludes(error, 'negative effective amount')
          ? '补充协议生效后，合同总额及各项费用均不能小于 0'
          : getErrorMessage(error, '更新失败')
      )
    },
  })

  const beforeUpload = (file: File) => {
    if (!isWithinUploadLimit(file)) {
      message.error(`文件大小不能超过${MAX_UPLOAD_SIZE_LABEL}`)
      return false
    }
    return false // Prevent auto-upload, handle manually in form submit
  }

  const handleSubmit = async (values: any) => {
    if (!canManage && !amendmentId) {
      message.error('您没有权限创建补充协议')
      return
    }

    const fileIds: string[] = []
    const newlyUploadedFileIds: string[] = []
    const uploadedList: UploadFile[] = []

    try {
      for (const currentFile of fileList) {
        const responseFile = currentFile.response as ProjectFile | undefined
        if (!currentFile.originFileObj || currentFile.status === 'done') {
          fileIds.push(responseFile?.id || String(currentFile.uid))
          uploadedList.push(currentFile)
          continue
        }

        message.loading({ content: '正在上传补充协议附件...', key: 'uploading' })
        const uploadedFile = await fileService.uploadFile(
          projectId,
          currentFile.originFileObj,
          'contract_amendment',
          '补充协议文件'
        )
        fileIds.push(uploadedFile.id)
        newlyUploadedFileIds.push(uploadedFile.id)
        uploadedList.push({
          ...currentFile,
          uid: uploadedFile.id,
          status: 'done',
          response: uploadedFile,
        })
      }
      setFileList(uploadedList)
      if (newlyUploadedFileIds.length > 0) {
        message.success({ content: '补充协议附件上传成功', key: 'uploading' })
      }
    } catch (error: any) {
      setFileList([...uploadedList,...fileList.slice(uploadedList.length)])
      message.error({ content: getErrorMessage(error, '文件上传失败'), key: 'uploading' })
      return
    }

    // Format date as ISO 8601 string (YYYY-MM-DDTHH:mm:ssZ)
    // Go's time.Time can parse this format
    const signDate = values.sign_date
      ? values.sign_date.format('YYYY-MM-DD')
      : null

    const changeType: ContractAmendmentChangeType = values.change_type || 'adjustment'
    const noAmountChange = changeType === 'no_amount_change'
    const data: CreateContractAmendmentRequest = {
      amendment_number: values.amendment_number,
      sign_date: signDate || '',
      change_type: changeType,
      design_fee: noAmountChange ? '0.00' : String(values.design_fee || '0.00'),
      survey_fee: noAmountChange ? '0.00' : String(values.survey_fee || '0.00'),
      consultation_fee: noAmountChange ? '0.00' : String(values.consultation_fee || '0.00'),
      contract_rate:values.contract_rate,
      // Material references use their own explicit owner command.
      description: values.description,
    }

    try {
      if (amendmentId) {
        await updateMutation.mutateAsync(data)
      } else {
        await createMutation.mutateAsync(data)
      }
    } catch {
      // Retain file descriptors and form values; business failure never deletes bytes.
    }
  }

  const isLoading = isLoadingAmendment
  const isSubmitting = createMutation.isPending || updateMutation.isPending
  const changeType = Form.useWatch('change_type', form) || 'adjustment'
  const isRevision = changeType === 'revision'
  const isNoAmountChange = changeType === 'no_amount_change'
  const feeLabelSuffix = isRevision ? '修订后金额' : '增减金额'
  const designFee=Form.useWatch('design_fee',form)||'0.00'
  const surveyFee=Form.useWatch('survey_fee',form)||'0.00'
  const consultationFee=Form.useWatch('consultation_fee',form)||'0.00'
  let amendmentAmount='0.00'
  try{amendmentAmount=sumMoney(designFee,surveyFee,consultationFee)}catch{}
  const watchedSignDate = Form.useWatch('sign_date', form)
  const effectiveAmountAfterAmendment =
    contract && watchedSignDate
      ? getContractEffectiveAmount({
          ...contract,
          amendments: [
            ...(contract.amendments || []).filter(
              (item) => item.id !== amendmentId
            ),
            {
              id: amendmentId || '__draft__',
              amendment_number:
                form.getFieldValue('amendment_number') || '__draft__',
              sign_date: watchedSignDate.format('YYYY-MM-DD'),
              description: form.getFieldValue('description'),
              change_type: changeType,
              design_fee: isNoAmountChange
                ? '0.00'
                : form.getFieldValue('design_fee') || '0.00',
              survey_fee: isNoAmountChange
                ? '0.00'
                : form.getFieldValue('survey_fee') || '0.00',
              consultation_fee: isNoAmountChange
                ? '0.00'
                : form.getFieldValue('consultation_fee') || '0.00',
              amendment_amount: isNoAmountChange ? '0.00' : amendmentAmount,
              contract_id: contractId,
              created_at:
                existingAmendment?.created_at || new Date().toISOString(),
              updated_at:
                existingAmendment?.updated_at || new Date().toISOString(),
            } as ContractAmendment,
          ],
        })
      : undefined

  if (canManage === false && !amendmentId) {
    return (
      <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
        您没有权限创建补充协议
      </div>
    )
  }

  return (
    <Form
      name={`reference-oa-amendment-${contractId}`}
      form={form}
      layout="vertical"
      onFinish={handleSubmit}
      initialValues={{
        change_type: 'adjustment',
        design_fee: '0.00',
        survey_fee: '0.00',
        consultation_fee: '0.00',
      }}
    >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label="补充协议编号"
              name="amendment_number"
              rules={[{ required: true, message: '请输入补充协议编号' }]}
            >
              <Input placeholder="XIE-YYYY-XXX" disabled={canManage === false} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label="签订日期"
              name="sign_date"
              rules={[{ required: true, message: '请选择签订日期' }]}
            >
              <DatePicker
                style={{ width: '100%' }}
                format="YYYY-MM-DD"
                disabled={canManage === false}
              />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          label="协议影响方式"
          name="change_type"
          tooltip="增加/扣减按差额影响当前有效合同金额；修订以本协议金额替换此前有效金额。"
          rules={[{ required: true, message: '请选择协议影响方式' }]}
        >
          <Select
            disabled={canManage === false}
            options={[
              { value: 'adjustment', label: '增加/扣减合同金额' },
              { value: 'revision', label: '修订为新的合同金额' },
              { value: 'no_amount_change', label: '仅修订条款，不改金额' },
            ]}
          />
        </Form.Item>

        {isNoAmountChange ? (
          <div style={{ marginBottom: 16, color: '#666' }}>
            本协议只记录合同条款或文件变化，不影响当前有效合同金额。
          </div>
        ) : (
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item
              label={`设计费${feeLabelSuffix}`}
              name="design_fee"
              rules={
                isRevision
                  ? [{ validator:(_,v)=>nonnegative(v)?Promise.resolve():Promise.reject(new Error('设计费不能小于0')) }]
                  : []
              }
            >
              <InputNumber stringMode
                min={isRevision ? 0 : undefined}
                style={{ width: '100%' }}
                prefix="¥"
                precision={2}
                placeholder={isRevision ? '请输入修订后设计费' : '正数增加，负数扣减'}
                disabled={canManage === false}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={`勘察费${feeLabelSuffix}`}
              name="survey_fee"
              rules={
                isRevision
                  ? [{ validator:(_,v)=>nonnegative(v)?Promise.resolve():Promise.reject(new Error('勘察费不能小于0')) }]
                  : []
              }
            >
              <InputNumber stringMode
                min={isRevision ? 0 : undefined}
                style={{ width: '100%' }}
                prefix="¥"
                precision={2}
                placeholder={isRevision ? '请输入修订后勘察费' : '正数增加，负数扣减'}
                disabled={canManage === false}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              label={`咨询费${feeLabelSuffix}`}
              name="consultation_fee"
              rules={
                isRevision
                  ? [{ validator:(_,v)=>nonnegative(v)?Promise.resolve():Promise.reject(new Error('咨询费不能小于0')) }]
                  : []
              }
            >
              <InputNumber stringMode
                min={isRevision ? 0 : undefined}
                style={{ width: '100%' }}
                prefix="¥"
                precision={2}
                placeholder={isRevision ? '请输入修订后咨询费' : '正数增加，负数扣减'}
                disabled={canManage === false}
              />
            </Form.Item>
          </Col>
        </Row>
        )}

        {!isNoAmountChange && (
          <div style={{ marginBottom: 16, color: '#555' }}>
            {isRevision ? '修订后的合同金额' : '本次合同金额变更'}：
            {formatMoney(amendmentAmount)}
            {effectiveAmountAfterAmendment !== undefined && (
              <>
                <br />
                本协议加入后的预览金额（不是正式生效）：
                {formatMoney(effectiveAmountAfterAmendment)}
              </>
            )}
          </div>
        )}

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label="合同费率(%)"
              name="contract_rate"
              rules={[{ validator:(_,v)=>{try{rateBPS(v);return Promise.resolve()}catch{return Promise.reject(new Error('合同费率应在0-100之间'))}} }]}
            >
              <InputNumber stringMode
                min={0}
                max={100}
                precision={2}
                step={0.01}
                style={{ width: '100%' }}
                placeholder="例如：15.5 (表示15.5%)"
                disabled={canManage === false}
                addonAfter="%"
              />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          label="补充协议文件"
          tooltip="可选，支持上传补充协议文件"
        >
          {amendmentId?<ContractMaterials contractId={contractId} kind="amendment" amendmentId={amendmentId} label="协议附件" />:<AttachmentUploader
            helper="先保存本协议意图，再在协议卡片单独登记附件；保存意图不确认协议。"
            fileList={fileList}
            onChange={setFileList}
            beforeUpload={beforeUpload}
            disabled
            onDownload={(file) =>
              fileService.downloadFile(
                (file.response as ProjectFile | undefined)?.id || String(file.uid),
                file.name
              )
            }
          />}
        </Form.Item>

        <Form.Item label="备注" name="description">
          <TextArea
            rows={4}
            placeholder="请输入备注信息"
            disabled={canManage === false}
          />
        </Form.Item>

        <Form.Item>
          <Space>
            {canManage && (
            <Button
              type="primary"
              htmlType="submit"
                loading={isSubmitting}
                disabled={isLoading}
            >
                {amendmentId ? '更新' : '保存'}
            </Button>
            )}
            {onCancel && <Button onClick={onCancel}>取消</Button>}
          </Space>
        </Form.Item>
    </Form>
  )
}
