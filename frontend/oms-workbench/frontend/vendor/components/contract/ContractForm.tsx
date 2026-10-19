import { useEffect, useRef, useState } from 'react'
import type { FocusEvent, ReactNode } from 'react'
import {
  Button,
  Checkbox,
  Col,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Typography,
  message,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { permissionService } from '@/services/permission'
import { fileService } from '@/services/file'
import { AttachmentUploader } from '@/components/shared/AttachmentUploader'
import { InlineFilePreviewLayout } from '@/components/shared/InlineFilePreviewLayout'
import { ContractAmendmentList } from './ContractAmendmentList'
import type {
  BusinessChainType,
  CreateContractRequest,
  Contract,
  File as ProjectFile,
} from '@/types'
import dayjs from 'dayjs'
import type { UploadFile } from 'antd'
import {
  DEFAULT_BUSINESS_CHAIN,
  normalizeBusinessChainType,
} from '@/utils/businessChain'
import { errorIncludes, getErrorMessage } from '@/utils/error'
import { isWithinUploadLimit, MAX_UPLOAD_SIZE_LABEL } from '@/utils/upload'
import { getContractEffectiveAmount } from '@/utils/contractAmounts'
import { upsertContractInList } from '@/utils/contractCache'
import { formatMoney } from '@/utils/money'
import { sumMoney } from '@/contract-money.mjs'
import { ContractNativePreparation } from '@/contract-native-preparation'
import {ContractMaterials} from '@/contract-materials'
import {ContractSettlement} from '@/contract-settlement'
import './ContractForm.css'

interface ContractFormProps {
  projectId: string
  contractId?: string
  chainType?: BusinessChainType
  onSuccess?: () => void
  onSaved?: (contract: Contract) => void
  onCancel?: () => void
  closeOnSave?: boolean
  onPreviewStateChange?: (active: boolean) => void
}

type ContractFileSlot = 'contract' | 'finalization'

type UploadResult = {
  fileIds: string[]
  fileList: UploadFile[]
  newFileIds: string[]
}

type SubmitScope = 'base' | 'finalization'

type ContractFormValues = {
  contract_number: string
  sign_date: dayjs.Dayjs
  chain_type?: BusinessChainType
  contract_rate?: string
  contract_amount: string
  settlement_amount?: string
  settlement_follows_contract_amount?: boolean
  design_fee?: string
  survey_fee?: string
  consultation_fee?: string
}

const CHAIN_OPTIONS = [
  {
    label: '甲方合同',
    value: 'client_to_qualification',
  },
  {
    label: '京宏合同',
    value: 'qualification_to_our_company',
  },
] as const

const createDoneFileList = (
  files?: ProjectFile[],
  legacyFile?: ProjectFile
): UploadFile[] => {
  const source = files?.length ? files : legacyFile ? [legacyFile] : []
  return source.map((file) => ({
      uid: file.id,
      name: file.original_name,
      status: 'done',
      size: file.file_size,
      response: file,
    }))
}

const getContractFileBySlot = (contract: Contract | null | undefined, slot: ContractFileSlot) => {
  if (!contract) {
    return undefined
  }

  if (slot === 'contract') {
    return contract.contract_file
  }
  return contract.finalization_file
}

const selectNumberValueOnFocus = (event: FocusEvent<HTMLInputElement>) => {
  const input = event.target
  // A delayed select can steal focus from the next fee field and overwrite it.
  window.setTimeout(() => { if (document.activeElement === input) input.select() }, 0)
}

const FormBlock = ({
  title,
  description,
  actions,
  disabled = false,
  disabledHint,
  onDisabledClick,
  children,
}: {
  title: string
  description: string
  actions?: ReactNode
  disabled?: boolean
  disabledHint?: string
  onDisabledClick?: () => void
  children: ReactNode
}) => (
  <section
    className={[
      'contract-form__block',
      disabled ? 'contract-form__block--disabled' : '',
    ]
      .filter(Boolean)
      .join(' ')}
  >
    <header className="contract-form__block-header">
      <div>
        <h3 className="contract-form__block-title">{title}</h3>
        <p className="contract-form__block-description">{description}</p>
      </div>
    </header>
    {children}
    {actions && <div className="contract-form__block-actions">{actions}</div>}
    {disabled && onDisabledClick && (
      <button
        type="button"
        className="contract-form__block-overlay"
        onClick={onDisabledClick}
      >
        <span className="contract-form__block-overlay-badge">待开放</span>
        <strong className="contract-form__block-overlay-title">
          尚未完成基础合同设置
        </strong>
        <span className="contract-form__block-overlay-description">
          {disabledHint || '请先填写并保存原始合同后，再继续处理当前区域。'}
        </span>
      </button>
    )}
  </section>
)

export const ContractForm = ({
  projectId,
  contractId,
  chainType,
  onSuccess,
  onSaved,
  onCancel,
  closeOnSave = true,
  onPreviewStateChange,
}: ContractFormProps) => {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()
  const [contractFileList, setContractFileList] = useState<UploadFile[]>([])
  const [finalizationFileList, setFinalizationFileList] = useState<UploadFile[]>([])
  const [previewFile, setPreviewFile] = useState<ProjectFile | null>(null)
  const [previewUploadFile, setPreviewUploadFile] = useState<UploadFile | null>(null)
  const [activePreviewSlot, setActivePreviewSlot] = useState<ContractFileSlot | null>(null)
  const [persistedContract, setPersistedContract] = useState<Contract | null>(null)
  const submitScopeRef = useRef<SubmitScope>('base')
  const hasPreview = Boolean(previewFile || previewUploadFile)

  const fixedChainType = chainType
  const followsContractAmount =
    Form.useWatch('settlement_follows_contract_amount', form) ?? true
  const watchedSettlementAmount = Form.useWatch('settlement_amount', form) || 0

  const { data: canManage } = useQuery({
    queryKey: ['canManageContract', projectId, contractId || persistedContract?.id],
    queryFn: () => permissionService.canManageContract(projectId,contractId || persistedContract?.id),
    enabled: !!projectId,
  })

  const { data: existingContract, isLoading: isLoadingContract } = useQuery({
    queryKey: ['contract', contractId],
    queryFn: () => businessService.getContract(contractId!),
    enabled: !!contractId,
  })
  const activeContract = existingContract || persistedContract
  const activeContractId = contractId || persistedContract?.id || existingContract?.id

  useEffect(() => {
    if (!existingContract) {
      return
    }

    setPersistedContract(existingContract)

    form.setFieldsValue({
      contract_number: existingContract.contract_number,
      sign_date: existingContract.sign_date ? dayjs(existingContract.sign_date) : null,
      chain_type:
        fixedChainType || normalizeBusinessChainType(existingContract.chain_type),
        contract_rate: existingContract.contract_rate,
      contract_amount: existingContract.contract_amount,
      settlement_amount: existingContract.settlement_amount || 0,
      settlement_follows_contract_amount:
        existingContract.settlement_follows_contract_amount !== false,
      design_fee: existingContract.design_fee || 0,
      survey_fee: existingContract.survey_fee || 0,
      consultation_fee: existingContract.consultation_fee || 0,
    })

    setContractFileList(
      createDoneFileList(existingContract.contract_files, existingContract.contract_file)
    )
    setFinalizationFileList(
      createDoneFileList(
        existingContract.finalization_files,
        existingContract.finalization_file
      )
    )
  }, [existingContract, fixedChainType, form])

  useEffect(() => {
    onPreviewStateChange?.(hasPreview)
  }, [hasPreview, onPreviewStateChange])

  useEffect(
    () => () => {
      onPreviewStateChange?.(false)
    },
    [onPreviewStateChange]
  )

  const handleContractSaveError = (error: any, defaultMessage: string) => {
    if (errorIncludes(error, 'contract number already exists', '合同编号已存在')) {
      form.setFields([{ name: 'contract_number', errors: ['合同编号已存在'] }])
    }
    message.error(getErrorMessage(error, defaultMessage))
  }

  const applySavedContract = (
    savedContract: Contract,
    options: {
      successMessage?: string
      shouldClose?: boolean
    } = {}
  ) => {
    setPersistedContract(savedContract)
    queryClient.setQueryData<Contract[]>(
      ['contracts', projectId],
      (contracts) => upsertContractInList(contracts, savedContract)
    )
    onSaved?.(savedContract)
    queryClient.setQueryData(['contract', savedContract.id], savedContract)
    queryClient.invalidateQueries({ queryKey: ['contracts', projectId] })
    queryClient.invalidateQueries({
      queryKey: ['projectProgressAssistant', String(projectId)],
    })
    queryClient.invalidateQueries({
      queryKey: ['oa2026', 'progress', String(projectId)],
    })
    queryClient.invalidateQueries({ queryKey: ['contract', savedContract.id] })
    queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
    queryClient.invalidateQueries({ queryKey: ['businessStatistics', projectId] })
    queryClient.invalidateQueries({ queryKey: ['contractAmounts', projectId] })
    queryClient.invalidateQueries({ queryKey: ['companyRevenueSummary'] })
    queryClient.invalidateQueries({ queryKey: ['companyRevenueStatistics'] })
    queryClient.invalidateQueries({ queryKey: ['projectRevenueLedgersV2'] })

    setContractFileList(
      createDoneFileList(savedContract.contract_files, savedContract.contract_file)
    )
    setFinalizationFileList(
      createDoneFileList(savedContract.finalization_files, savedContract.finalization_file)
    )

    if (activePreviewSlot) {
      const nextPreviewFile = getContractFileBySlot(savedContract, activePreviewSlot)
      setPreviewFile(nextPreviewFile || null)
      setPreviewUploadFile(null)
    }

    if (options.successMessage) {
      message.success(options.successMessage)
    }

    if (options.shouldClose) {
      onSuccess?.()
    }
  }

  const createMutation = useMutation({
    mutationFn: (data: CreateContractRequest) =>
      businessService.createContract(projectId, data),
    onError: (error: any) => {
      handleContractSaveError(error, '创建失败')
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({
      contractId: targetContractId,
      data,
    }: {
      contractId: string
      data: CreateContractRequest
    }) => businessService.updateContract(targetContractId, data),
    onError: (error: any) => {
      handleContractSaveError(error, '更新失败')
    },
  })

  const setSlotState = (
    slot: ContractFileSlot,
    fileList: UploadFile[]
  ) => {
    if (slot === 'contract') {
      setContractFileList(fileList)
      return
    }

    setFinalizationFileList(fileList)
  }

  const handleFileChange =
    (slot: ContractFileSlot) =>
    (nextFileList: UploadFile[]) => {
      setSlotState(slot, nextFileList)

      if (activePreviewSlot !== slot) {
        return
      }

      if (nextFileList.length === 0) {
        setPreviewFile(null)
        setPreviewUploadFile(null)
        setActivePreviewSlot(null)
        return
      }

      if (!hasPreview) {
        return
      }

      const nextFile = nextFileList[nextFileList.length - 1]
      if (nextFile?.originFileObj) {
        setPreviewUploadFile(nextFile)
        setPreviewFile(null)
        return
      }

      const projectFile = getContractFileBySlot(activeContract, slot)
      const responseFile = nextFile?.response as ProjectFile | undefined
      setPreviewFile(projectFile || responseFile || null)
      setPreviewUploadFile(null)
    }

  const beforeUpload = (file: File) => {
    if (!isWithinUploadLimit(file)) {
      message.error(`文件大小不能超过${MAX_UPLOAD_SIZE_LABEL}`)
      return false
    }
    return false
  }

  const handleDownloadFile = async (currentFile: UploadFile) => {
    const fileId = typeof currentFile?.uid === 'string' ? currentFile.uid : undefined
    const fileName = currentFile?.name || '下载文件'

    if (!fileId) {
      return
    }

    try {
      await fileService.downloadFile(fileId, fileName)
    } catch (error: any) {
      message.error(getErrorMessage(error, '下载失败'))
    }
  }

  const getSlotFileList = (slot: ContractFileSlot) => {
    if (slot === 'contract') {
      return contractFileList
    }
    return finalizationFileList
  }

  const getSlotProjectFile = (slot: ContractFileSlot, file?: UploadFile) => {
    const files = slot === 'contract'
      ? activeContract?.contract_files || (activeContract?.contract_file ? [activeContract.contract_file] : [])
      : activeContract?.finalization_files || (activeContract?.finalization_file ? [activeContract.finalization_file] : [])
    if (!file) return files[0]
    return files.find((item) => item.id === file.uid)
  }

  const handlePreviewFile = (slot: ContractFileSlot, selectedFile?: UploadFile) => {
    setActivePreviewSlot(slot)
    const currentFile = selectedFile || getSlotFileList(slot)[0]
    if (currentFile?.originFileObj) {
      setPreviewUploadFile(currentFile)
      setPreviewFile(null)
      return
    }

    const projectFile = getSlotProjectFile(slot, currentFile)
    if (projectFile) {
      setPreviewFile(projectFile)
      setPreviewUploadFile(null)
      return
    }

    const responseFile = currentFile?.response as ProjectFile | undefined
    if (responseFile?.id) {
      setPreviewFile(responseFile)
      setPreviewUploadFile(null)
      return
    }

    message.info('当前还没有可预览的文件')
  }

  const handleClosePreview = () => {
    setPreviewFile(null)
    setPreviewUploadFile(null)
    setActivePreviewSlot(null)
  }

  const uploadManagedFiles = async ({
    slot,
    fileList,
    category,
    description,
  }: {
    slot: ContractFileSlot
    fileList: UploadFile[]
    category: 'contract_main' | 'contract_amendment' | 'finalization_sheet'
    description: string
  }): Promise<UploadResult> => {
    if (fileList.length === 0) {
      return { fileIds: [], fileList: [], newFileIds: [] }
    }

    const messageKey = `contract-upload-${slot}`
    const uploadedFileList: UploadFile[] = []
    const fileIds: string[] = []
    const newFileIds: string[] = []

    for (const currentFile of fileList) {
      const responseFile = currentFile.response as ProjectFile | undefined
      if (!currentFile.originFileObj || currentFile.status === 'done') {
        const existingID = responseFile?.id || String(currentFile.uid)
        if (existingID) fileIds.push(existingID)
        uploadedFileList.push(currentFile)
        continue
      }

      message.loading({ content: `正在上传${description}...`, key: messageKey })
      const uploadedFile = await fileService.uploadFile(
        projectId,
        currentFile.originFileObj,
        category,
        description
      )
      fileIds.push(uploadedFile.id)
      newFileIds.push(uploadedFile.id)
      uploadedFileList.push({
        ...currentFile,
        uid: uploadedFile.id,
        status: 'done',
        response: uploadedFile,
      })
    }

    setSlotState(slot, uploadedFileList)
    if (newFileIds.length > 0) {
      message.success({ content: `${description}上传成功`, key: messageKey })
    }

    return {
      fileIds,
      fileList: uploadedFileList,
      newFileIds,
    }
  }

  const buildContractData = (
    values: ContractFormValues,
    fileIds?: {
      contract_file_ids?: string[]
      finalization_file_ids?: string[]
    }
  ): CreateContractRequest => {
    const signDate = values.sign_date
      ? values.sign_date.format('YYYY-MM-DD')
      : ''

    return {
      contract_number: values.contract_number,
      sign_date: signDate,
      chain_type: fixedChainType || values.chain_type || DEFAULT_BUSINESS_CHAIN,
      contract_rate: values.contract_rate,
      contract_amount: values.contract_amount ?? '0.00',
      settlement_amount: followsContractAmount
        ? getContractEffectiveAmount(activeContract, values.contract_amount ?? '0.00')
        : values.settlement_amount ?? undefined,
      settlement_follows_contract_amount: followsContractAmount,
      design_fee: values.design_fee ?? '0.00',
      survey_fee: values.survey_fee ?? '0.00',
      consultation_fee: values.consultation_fee ?? '0.00',
      ...fileIds,
    }
  }

  const handleSubmit = async (values: ContractFormValues) => {
    if (!canManage && !contractId) {
      message.error('您没有权限创建合同')
      return
    }

    const successMessageMap: Record<SubmitScope, string> = {
      base: activeContractId ? '合同草稿已保存，尚未正式确认' : '合同草稿已保存并关联项目，协议草稿已开放',
      finalization: '定案意图已保存，尚未确认结算',
    }
    const dataWithoutFiles = buildContractData(values)
    let savedContract: Contract | null = null
    const newlyUploadedFileIds: string[] = []

    try {
      // Files are separately registered owner references, never in this Base
      // terms mutation and never removed because another save failed.
      savedContract = activeContractId
        ? await updateMutation.mutateAsync({
            contractId: activeContractId!,
            data: dataWithoutFiles,
          })
        : await createMutation.mutateAsync(dataWithoutFiles)

      applySavedContract(savedContract, {
        successMessage: successMessageMap[submitScopeRef.current] || '合同内容已保存',
        shouldClose: closeOnSave,
      })
    } catch (error: any) {
      if(savedContract)applySavedContract(savedContract)
      message.error(getErrorMessage(error, '合同操作尚未全部确认；输入保留，请继续原操作'))
    }
  }

  const designFee = Form.useWatch('design_fee', form) || 0
  const surveyFee = Form.useWatch('survey_fee', form) || 0
  const consultationFee = Form.useWatch('consultation_fee', form) || 0
  let referenceAmount='0.00'
  try{referenceAmount=sumMoney(designFee,surveyFee,consultationFee)}catch{/* form validation rejects invalid decimals */}
  const effectiveAmount = activeContract
    ? getContractEffectiveAmount(activeContract, referenceAmount)
    : referenceAmount
  const followupSectionsEnabled = Boolean(activeContractId)
  const settlementDisplayAmount = followsContractAmount
    ? effectiveAmount
    : watchedSettlementAmount
  const downloadable = Boolean(activeContractId)
  const handleLockedSectionClick = (sectionTitle: string) => {
    message.warning(`尚未完成基础合同设置，请先填写并保存原始合同后，再处理${sectionTitle}。`)
  }

  useEffect(() => {
    const currentAmount = form.getFieldValue('contract_amount') || 0
    if (String(currentAmount) !== referenceAmount) {
      form.setFieldValue('contract_amount', referenceAmount)
    }

    if (followsContractAmount) {
      form.setFieldValue('settlement_amount', effectiveAmount)
    }
  }, [referenceAmount, effectiveAmount, followsContractAmount, form])

  const isLoading = isLoadingContract
  const isSubmitting = createMutation.isPending || updateMutation.isPending
  const handleSubmitByScope = (scope: SubmitScope) => {
    submitScopeRef.current = scope
    form.submit()
  }

  return (
    <InlineFilePreviewLayout
      className="contract-form__layout"
      contentClassName="contract-form__form-column"
      file={previewFile}
      uploadFile={previewUploadFile}
      onClosePreview={handleClosePreview}
      defaultPreviewWidth={820}
      minPreviewWidth={780}
      minContentWidth={460}
    >
        <Form
          name={`reference-oa-contract-${projectId}`}
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{
            chain_type: fixedChainType || DEFAULT_BUSINESS_CHAIN,
            contract_amount: '0.00',
            settlement_amount: '0.00',
            settlement_follows_contract_amount: true,
            design_fee: '0.00',
            survey_fee: '0.00',
            consultation_fee: '0.00',
          }}
          className="contract-form"
          onKeyDown={e=>{if(e.key==='Enter'&&(e.target as Element).closest('[data-native-contract-settlement],[data-native-contract-preparation]'))e.preventDefault()}}
        >
        {isLoading ? (
          <div>加载中...</div>
        ) : (
          <>
          {!fixedChainType && (
            <Form.Item
              label="合同对象"
              name="chain_type"
              rules={[{ required: true, message: '请选择合同对象' }]}
            >
              <Select
                placeholder="请选择合同对象"
                options={CHAIN_OPTIONS as unknown as { label: string; value: string }[]}
                disabled={canManage === false}
              />
            </Form.Item>
          )}

          <section className="contract-form__settlement">
            <div className="contract-form__settlement-head">
              <div>
                <div className="contract-form__eyebrow">结算概览</div>
                <h2 className="contract-form__settlement-title">结算金额</h2>
                <p className="contract-form__settlement-description">
                此处为结算意图／预览，不是正式结算事实；正式记录在定案区单独确认。
                </p>
              </div>
              <Form.Item
                className="contract-form__checkbox-item"
                name="settlement_follows_contract_amount"
                valuePropName="checked"
              >
                <Checkbox disabled={canManage === false}>
                  保持与当前有效合同金额同步
                </Checkbox>
              </Form.Item>
            </div>

            <div className="contract-form__settlement-value">
              {formatMoney(settlementDisplayAmount)}
            </div>

            {followsContractAmount ? (
              <>
                <Form.Item name="settlement_amount" hidden>
                  <InputNumber stringMode />
                </Form.Item>
                <Typography.Text type="secondary" className="contract-form__settlement-hint">
                  当前会随有效合同金额自动同步；原合同或补充协议发生金额变更后，这里会一起变化。
                </Typography.Text>
              </>
            ) : (
              <Form.Item
                label="手动结算金额"
                name="settlement_amount"
                className="contract-form__compact-item"
              >
                <InputNumber stringMode
                  min={0}
                  precision={2}
                  style={{ width: '100%' }}
                  placeholder="请输入结算金额"
                  disabled={canManage === false}
                  prefix="¥"
                />
              </Form.Item>
            )}
          </section>

          <FormBlock
            title="原始合同区域"
            description="这里维护基础合同信息、费用明细以及合同原件文件。"
            actions={
              canManage ? (
                <Button
                  type="primary"
                  onClick={() => handleSubmitByScope('base')}
                  loading={isSubmitting}
                  disabled={isLoading}
                >
                  保存
                </Button>
              ) : undefined
            }
          >
            <Row gutter={[16, 0]}>
              <Col span={12}>
                <Form.Item
                  label="合同编号"
                  name="contract_number"
                  rules={[{ required: true, message: '请输入合同编号' }]}
                >
                  <Input placeholder="HT-YYYY-XXX" disabled={canManage === false} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="签订日期"
                  name="sign_date"
                  rules={[{ required: true, message: '请选择签订日期' }]}
                >
                  <DatePicker style={{ width: '100%' }} disabled={canManage === false} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="合同费率(%)" name="contract_rate">
                  <InputNumber stringMode
                    min={0}
                    max={100}
                    precision={2}
                    step={0.01}
                    style={{ width: '100%' }}
                    placeholder="例如：15.5"
                    disabled={canManage === false}
                    addonAfter="%"
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="原合同金额"
                  name="contract_amount"
                  rules={[{ required: true, message: '原合同金额不能为空' }]}
                >
                  <InputNumber stringMode
                    min={0}
                    precision={2}
                    style={{ width: '100%' }}
                    placeholder="自动汇总"
                    disabled
                    prefix="¥"
                  />
                </Form.Item>
              </Col>
            </Row>

            <div className="contract-form__subsection">
              <div className="contract-form__subsection-header">
                <h4 className="contract-form__subsection-title">费用明细</h4>
                <p className="contract-form__subsection-description">
                  设计费、勘察费、咨询费会自动汇总到原合同金额。
                </p>
              </div>
              <Row gutter={[16, 0]}>
                <Col span={8}>
                  <Form.Item label="设计费" name="design_fee">
                    <InputNumber stringMode
                      min={0}
                      precision={2}
                      style={{ width: '100%' }}
                      placeholder="0.00"
                      disabled={canManage === false}
                      onFocus={selectNumberValueOnFocus}
                    />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label="勘察费" name="survey_fee">
                    <InputNumber stringMode
                      min={0}
                      precision={2}
                      style={{ width: '100%' }}
                      placeholder="0.00"
                      disabled={canManage === false}
                      onFocus={selectNumberValueOnFocus}
                    />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label="咨询费" name="consultation_fee">
                    <InputNumber stringMode
                      min={0}
                      precision={2}
                      style={{ width: '100%' }}
                      placeholder="0.00"
                      disabled={canManage === false}
                      onFocus={selectNumberValueOnFocus}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <div className="contract-form__inline-note">
                原合同金额根据设计费、勘察费、咨询费自动汇总：
                {formatMoney(referenceAmount)}
              </div>
            </div>

            <div className="contract-form__file-grid">
              <Form.Item className="contract-form__file-item">
                {activeContractId?<ContractMaterials contractId={activeContractId} kind="contract" label="合同文件" />:<AttachmentUploader
                  label="合同文件"
                  helper="合同材料登记待接通；尚不能上传或声明附件已保存"
                  fileList={contractFileList}
                  disabled
                  onChange={handleFileChange('contract')}
                  beforeUpload={beforeUpload}
                  onPreview={(file) => handlePreviewFile('contract', file)}
                  onDownload={downloadable ? handleDownloadFile : undefined}
                />}
              </Form.Item>
            </div>
          </FormBlock>

          {activeContractId && <ContractNativePreparation contractId={activeContractId} />}

          <FormBlock
            title="补充区域"
            description={
              followupSectionsEnabled
                ? '每份补充协议单独记录其影响方式、金额和文件；可连续添加多份。'
                : '先保存基础合同，之后再回来维护多份补充协议。'
            }
            disabled={!followupSectionsEnabled}
            disabledHint="请先完成原始合同的编号、签订日期、费用明细和合同文件，再回来维护补充协议。"
            onDisabledClick={() => handleLockedSectionClick('补充区域')}
          >
            {followupSectionsEnabled && (
              <div className="contract-form__inline-note">
                当前合同与协议预览金额：{formatMoney(effectiveAmount)}
              </div>
            )}
            {activeContractId && (
              <ContractAmendmentList
                projectId={projectId}
                contractId={activeContractId}
                onChanged={() => {
                  queryClient.invalidateQueries({ queryKey: ['contract', activeContractId] })
                  queryClient.invalidateQueries({ queryKey: ['contracts', projectId] })
                  queryClient.invalidateQueries({ queryKey: ['contractAmounts', projectId] })
                }}
              />
            )}
          </FormBlock>

          <FormBlock
            title="定案表文件"
            description={
              followupSectionsEnabled
                ? '定案表单独放在末段维护，便于和基础合同、补充协议清晰分区。'
                : '基础合同保存后，再上传定案表文件。'
            }
            actions={
              canManage && followupSectionsEnabled ? (
                <Button
                  type="primary"
                  onClick={() => handleSubmitByScope('finalization')}
                  loading={isSubmitting}
                  disabled={isLoading}
                >
                  保存
                </Button>
              ) : undefined
            }
            disabled={!followupSectionsEnabled}
            disabledHint="请先完成并保存基础合同，之后再上传定案表文件。"
            onDisabledClick={() => handleLockedSectionClick('定案表')}
          >
            <div className="contract-form__file-grid">
              <Form.Item className="contract-form__file-item">
                {activeContractId?<ContractMaterials contractId={activeContractId} kind="finalization" label="定案表" />:<AttachmentUploader
                  label="定案表"
                  helper="定案材料登记待接通；保存结算意图不表示正式确认"
                  fileList={finalizationFileList}
                  disabled
                  onChange={handleFileChange('finalization')}
                  beforeUpload={beforeUpload}
                  onPreview={(file) => handlePreviewFile('finalization', file)}
                  onDownload={downloadable ? handleDownloadFile : undefined}
                />}
              </Form.Item>
            </div>
            {activeContractId?<ContractSettlement contractId={activeContractId} />:null}
          </FormBlock>

          <Form.Item className="contract-form__footer">
            <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
              {onCancel && <Button onClick={onCancel}>取消</Button>}
            </Space>
          </Form.Item>
          </>
        )}
        </Form>
    </InlineFilePreviewLayout>
  )
}
