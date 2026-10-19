import { useMemo, useState } from 'react'
import {
  Button,
  Drawer,
  Empty,
  message,
} from 'antd'
import {
  CalendarOutlined,
  DeleteOutlined,
  FileDoneOutlined,
  RightOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { permissionService } from '@/services/permission'
import { ContractForm } from '@/components/contract/ContractForm'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import { SectionTitle } from '@/components/shared/SectionTitle'
import type { BusinessChainType, Contract } from '@/types'
import dayjs from 'dayjs'
import {
  BUSINESS_CHAIN_ORDER,
  getContractChainType,
} from '@/utils/businessChain'
import {
  getContractEffectiveAmount,
  getSettlementDisplayAmount,
} from '@/utils/contractAmounts'
import { formatMoney } from '@/utils/money'
import './BusinessPanelSurface.css'
import './ContractAndAmendments.css'

interface ContractAndAmendmentsProps {
  projectId: string
  clientName?: string
  qualificationUnitName?: string
}

const FileStatusPill = ({
  label,
  active,
}: {
  label: string
  active: boolean
}) => (
  <span
    className={[
      'contract-panel__file-pill',
      active ? 'contract-panel__file-pill--active' : '',
    ]
      .filter(Boolean)
      .join(' ')}
  >
    {label}
  </span>
)

const getContractGroupCopy = (
  chainType: BusinessChainType,
  clientName?: string,
  qualificationUnitName?: string
) => {
  const client = clientName || '甲方'
  const qualificationUnit = qualificationUnitName || '资质单位'

  if (chainType === 'client_to_qualification') {
    return {
      kicker: '甲方相关',
      title: '甲方合同',
      description: `${client} 与 ${qualificationUnit} 签订的合同，记录基础合同、补充协议和定案信息。`,
      emptyText: '当前还没有甲方相关合同',
    }
  }

  return {
    kicker: '京宏相关',
    title: '京宏合同',
    description: `${qualificationUnit} 与京宏签订的合同，记录京宏侧合同、补充协议和定案信息。`,
    emptyText: '当前还没有京宏相关合同',
  }
}

export const ContractAndAmendments = ({
  projectId,
  clientName,
  qualificationUnitName,
}: ContractAndAmendmentsProps) => {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingContract, setEditingContract] = useState<Contract | null>(null)
  const [previewExpanded, setPreviewExpanded] = useState(false)
  const [activeChainType, setActiveChainType] =
    useState<BusinessChainType>('client_to_qualification')
  const queryClient = useQueryClient()

  const { data: canManage } = useQuery({
    queryKey: ['canManageBusinessInfo', projectId],
    queryFn: () => permissionService.canManageBusinessInfo(projectId),
    enabled: !!projectId,
  })

  const { data: contracts = [] } = useQuery({
    queryKey: ['contracts', projectId],
    queryFn: () => businessService.getContracts(projectId),
    enabled: !!projectId,
  })

  const deleteContractMutation = useMutation({
    mutationFn: (contractId: string) => businessService.deleteContract(contractId),
    onSuccess: () => {
      message.success('合同草稿已取消，历史保留')
      setDrawerOpen(false)
      setEditingContract(null)
      queryClient.invalidateQueries({ queryKey: ['contracts', projectId] })
    },
    onError: (error: any) => {
      message.error(error.message || '删除失败')
    },
  })

  const contractsByChain = useMemo(
    () =>
      BUSINESS_CHAIN_ORDER.reduce(
        (acc, chainType) => {
          acc[chainType] = contracts.filter(
            (contract) => getContractChainType(contract) === chainType
          )
          return acc
        },
        {} as Record<BusinessChainType, Contract[]>
      ),
    [contracts]
  )

  const handleCreateContract = (chainType: BusinessChainType) => {
    setActiveChainType(chainType)
    setEditingContract(null)
    setDrawerOpen(true)
  }

  const handleOpenContract = (contract: Contract) => {
    setActiveChainType(getContractChainType(contract))
    setEditingContract(contract)
    setDrawerOpen(true)
  }

  return (
    <>
      <section className="project-workspace__card business-panel-surface">
        <header className="business-panel-surface__header">
          <SectionTitle icon={<FileDoneOutlined />}>合同信息</SectionTitle>
        </header>
        <div className="business-panel-surface__body">
          <div className="business-panel-surface__groups">
            {BUSINESS_CHAIN_ORDER.map((chainType) => {
              const groupContracts = contractsByChain[chainType]
              const groupCopy = getContractGroupCopy(
                chainType,
                clientName,
                qualificationUnitName
              )

              return (
                <section
                  key={chainType}
                  className={[
                    'business-panel-surface__group',
                    'contract-panel__group',
                    chainType === 'client_to_qualification'
                      ? 'contract-panel__group--client'
                      : 'contract-panel__group--income',
                  ]
                    .filter(Boolean)
                    .join(' ')}
                >
                  <div className="contract-panel__group-head">
                    <div className="contract-panel__group-heading">
                      <span className="contract-panel__group-kicker">
                        {groupCopy.kicker}
                      </span>
                      <h4 className="business-panel-surface__group-title">
                        {groupCopy.title}
                      </h4>
                      <p className="business-panel-surface__group-desc">
                        {groupCopy.description}
                      </p>
                    </div>
                    <div className="contract-panel__group-actions">
                      <span className="contract-panel__group-count">
                        {groupContracts.length} 份合同
                      </span>
                      {canManage && (
                        <CreateActionButton
                          size="small"
                          compact
                          onClick={() => handleCreateContract(chainType)}
                        >
                          新增合同
                        </CreateActionButton>
                      )}
                    </div>
                  </div>

                  <div className="contract-panel__group-body">
                    {groupContracts.length === 0 ? (
                      <div className="contract-panel__empty">
                        <Empty
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                          description={groupCopy.emptyText}
                        />
                      </div>
                    ) : (
                      <div className="contract-panel__list">
                        {groupContracts.map((contract) => {
                          const amendmentCount = contract.amendments?.length || 0
                          const amendmentFileCount =
                            contract.amendments?.filter(
                              (amendment) =>
                                (amendment.amendment_files?.length || 0) > 0 ||
                                !!amendment.amendment_file
                            ).length || 0
                          const contractFileCount = contract.contract_files?.length ||
                            (contract.contract_file ? 1 : 0)
                          const finalizationFileCount = contract.finalization_files?.length ||
                            (contract.finalization_file ? 1 : 0)
                          return (
                          <button
                            key={contract.id}
                            type="button"
                            className="contract-panel__item"
                            onClick={() => handleOpenContract(contract)}
                          >
                            <div className="contract-panel__item-primary">
                              <span className="contract-panel__item-kicker">
                                合同编号
                              </span>
                              <div className="contract-panel__item-number">
                                {contract.contract_number || '未填写编号'}
                              </div>
                              <div className="contract-panel__item-date">
                                <CalendarOutlined />
                                <span>
                                  {dayjs(contract.sign_date).format('YYYY-MM-DD')}
                                </span>
                              </div>
                            </div>

                            <div className="contract-panel__item-amount">
                              <span className="contract-panel__item-amount-label">
                                {contract.native_settlement?contract.native_settlement.kind==='final'?'最终结算金额':'阶段结算金额':'结算意图金额'}
                              </span>
                              <strong>{formatMoney(contract.native_settlement?.amount??getSettlementDisplayAmount(contract))}</strong>
                              <span>{contract.native_settlement?`已冻结 · 来源版本 ${contract.native_settlement.source_revision} · 结算序号 ${contract.native_settlement.sequence}`:'未确认结算预览'}</span>
                            </div>

                            <div className="contract-panel__item-summary">
                              <div className="contract-panel__item-summary-line">
                                原合同金额 {formatMoney(contract.contract_amount)}
                                <span className="contract-panel__separator">·</span>
                                当前有效合同金额{' '}
                                {formatMoney(
                                  contract.current_effective_amount ??
                                    getContractEffectiveAmount(contract)
                                )}
                              </div>
                              <div className="contract-panel__item-summary-line">
                                设计费{' '}
                                {formatMoney(
                                  contract.current_design_fee ?? contract.design_fee
                                )}
                                <span className="contract-panel__separator">·</span>
                                勘察费{' '}
                                {formatMoney(
                                  contract.current_survey_fee ?? contract.survey_fee
                                )}
                                <span className="contract-panel__separator">·</span>
                                咨询费{' '}
                                {formatMoney(
                                  contract.current_consultation_fee ??
                                    contract.consultation_fee
                                )}
                                {!!contract.current_unallocated_amount && (
                                  <>
                                    <span className="contract-panel__separator">·</span>
                                    未分类金额{' '}
                                    {formatMoney(contract.current_unallocated_amount)}
                                  </>
                                )}
                              </div>
                            </div>

                            <div className="contract-panel__item-files">
                              <FileStatusPill
                                label={`合同${contractFileCount > 1 ? ` ${contractFileCount} 份` : ''}`}
                                active={contractFileCount > 0}
                              />
                              <FileStatusPill
                                label={`补充 ${amendmentCount} 份${
                                  amendmentCount > 0
                                    ? ` / ${amendmentFileCount} 份有文件`
                                    : ''
                                }`}
                                active={amendmentFileCount > 0}
                              />
                              <FileStatusPill
                                label={`定案${finalizationFileCount > 1 ? ` ${finalizationFileCount} 份` : ''}`}
                                active={finalizationFileCount > 0}
                              />
                            </div>

                            <span className="contract-panel__item-arrow">
                              <RightOutlined />
                            </span>
                          </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                </section>
              )
            })}
          </div>
        </div>
      </section>

      <Drawer
        title={editingContract ? '合同详情' : '新增合同'}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false)
          setEditingContract(null)
          setPreviewExpanded(false)
        }}
        width={previewExpanded ? 'min(1360px, calc(100vw - 48px))' : 780}
        destroyOnHidden
        extra={
          editingContract?.can_edit ? (
            <DeleteConfirmButton
              title="确认取消合同草稿"
              description="只取消尚未正式确认的合同草稿；客户扩展和历史保留，不删除合同或文件。"
              details={[
                { label: '合同编号', value: editingContract.contract_number || '-' },
                {
                  label: '结算金额',
                  value: formatMoney(getSettlementDisplayAmount(editingContract)),
                },
              ]}
              buttonProps={{ danger: true, icon: <DeleteOutlined /> }}
              loading={deleteContractMutation.isPending}
              buttonText="取消草稿"
              onConfirm={() => deleteContractMutation.mutate(editingContract.id)}
            />
          ) : null
        }
      >
        <ContractForm
          projectId={projectId}
          contractId={editingContract?.id}
          chainType={activeChainType}
          closeOnSave={false}
          onPreviewStateChange={setPreviewExpanded}
          onSaved={(savedContract) => {
            setEditingContract(savedContract)
            setActiveChainType(getContractChainType(savedContract))
          }}
          onCancel={() => {
            setDrawerOpen(false)
            setEditingContract(null)
            setPreviewExpanded(false)
          }}
        />
      </Drawer>
    </>
  )
}
