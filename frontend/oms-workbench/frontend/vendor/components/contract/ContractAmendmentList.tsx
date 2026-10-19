import { useState } from 'react'
import { Button, Empty, message, Modal, Skeleton } from 'antd'
import {
  CalendarOutlined,
  DeleteOutlined,
  EditOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { permissionService } from '@/services/permission'
import { ContractAmendmentForm } from './ContractAmendmentForm'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import type { ContractAmendment } from '@/types'
import dayjs from 'dayjs'
import { formatMoney } from '@/utils/money'
import {cents} from '@/contract-money.mjs'
import './ContractAmendmentList.css'
import {ContractAmendmentConfirmation} from '@/contract-amendment-confirmation'
import {ContractMaterials} from '@/contract-materials'

interface ContractAmendmentListProps {
  projectId: string
  contractId: string
  onChanged?: () => void
}

const changeTypeMeta: Record<
  string,
  { label: string; tone: 'adjustment' | 'revision' | 'neutral' }
> = {
  adjustment: { label: '增加 / 扣减', tone: 'adjustment' },
  revision: { label: '金额修订', tone: 'revision' },
  no_amount_change: { label: '仅条款修订', tone: 'neutral' },
}

const formatSignedMoney = (amount: string|number) =>
  cents(amount) > 0n ? `+${formatMoney(amount)}` : formatMoney(amount)

export const ContractAmendmentList = ({
  projectId,
  contractId,
  onChanged,
}: ContractAmendmentListProps) => {
  const [modalVisible, setModalVisible] = useState(false)
  const [editingAmendment, setEditingAmendment] =
    useState<ContractAmendment | null>(null)
  const queryClient = useQueryClient()

  const { data: canManage } = useQuery({
    queryKey: ['canManageContractAmendments', projectId,contractId],
    queryFn: () => permissionService.canManageContractAmendments(projectId,contractId),
    enabled: !!projectId,
  })

  const { data: amendments = [], isLoading } = useQuery({
    queryKey: ['contractAmendments', projectId, contractId],
    queryFn: () => businessService.getContractAmendments(projectId, contractId),
    enabled: !!projectId && !!contractId,
  })

  const deleteMutation = useMutation({
    mutationFn: (amendmentId: string) =>
      businessService.deleteContractAmendment(projectId, contractId, amendmentId),
    onSuccess: () => {
      message.success('未确认协议草稿已移出，历史保留')
      queryClient.invalidateQueries({
        queryKey: ['contractAmendments', projectId, contractId],
      })
      queryClient.invalidateQueries({ queryKey: ['contracts', projectId] })
      queryClient.invalidateQueries({ queryKey: ['contract', contractId] })
      queryClient.invalidateQueries({ queryKey: ['projectAmendments', projectId] })
      queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
      queryClient.invalidateQueries({ queryKey: ['businessStatistics', projectId] })
      queryClient.invalidateQueries({ queryKey: ['contractAmounts', projectId] })
      queryClient.invalidateQueries({ queryKey: ['companyRevenueSummary'] })
      queryClient.invalidateQueries({ queryKey: ['companyRevenueStatistics'] })
      queryClient.invalidateQueries({ queryKey: ['projectRevenueLedgersV2'] })
      onChanged?.()
    },
    onError: (error: any) => {
      message.error(error.message || '删除失败')
    },
  })

  const handleCreate = () => {
    setEditingAmendment(null)
    setModalVisible(true)
  }

  const handleEdit = (amendment: ContractAmendment) => {
    setEditingAmendment(amendment)
    setModalVisible(true)
  }

  const handleModalClose = () => {
    setModalVisible(false)
    setEditingAmendment(null)
  }

  const handleSuccess = () => {
    setModalVisible(false)
    setEditingAmendment(null)
    onChanged?.()
  }

  return (
    <div className="contract-amendment-list">
      <div className="contract-amendment-list__toolbar">
        <div className="contract-amendment-list__summary">
          <strong>{amendments.length} 份补充协议</strong>
          <span>保存仍是意图；显式确认才追加正式协议，确认后保留不可变历史。</span>
        </div>
        {canManage && (
          <CreateActionButton onClick={handleCreate} compact>
            新建补充协议
          </CreateActionButton>
        )}
      </div>

      {isLoading ? (
        <div className="contract-amendment-list__loading">
          <Skeleton active paragraph={{ rows: 4 }} title={false} />
        </div>
      ) : amendments.length === 0 ? (
        <div className="contract-amendment-list__empty">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前还没有补充协议"
          />
        </div>
      ) : (
        <div className="contract-amendment-list__stream">
          {amendments.map((amendment, index) => {
            const changeType = amendment.change_type || 'adjustment'
            const meta = changeTypeMeta[changeType] || changeTypeMeta.adjustment
            const amount = amendment.amendment_amount || 0
            const feeItems = [
              { label: '设计费', value: amendment.design_fee || 0 },
              { label: '勘察费', value: amendment.survey_fee || 0 },
              { label: '咨询费', value: amendment.consultation_fee || 0 },
            ]

            return (
              <article className="contract-amendment-list__item" key={amendment.id}>
                <header className="contract-amendment-list__item-header">
                  <div className="contract-amendment-list__identity">
                    <span className="contract-amendment-list__index">
                      {String(index + 1).padStart(2, '0')}
                    </span>
                    <div className="contract-amendment-list__identity-copy">
                      <strong>{amendment.amendment_number || '未填写协议编号'}</strong>
                      <span>
                        <CalendarOutlined />
                        {dayjs(amendment.sign_date).format('YYYY-MM-DD')}
                      </span>
                    </div>
                  </div>

                  <div className="contract-amendment-list__header-actions">
                    <span
                      className="contract-amendment-list__change-type"
                      data-tone={meta.tone}
                    >
                      {meta.label}
                    </span>
                    {canManage && !amendment.native_confirmation && (
                      <>
                        <Button
                          type="text"
                          size="small"
                          icon={<EditOutlined />}
                          onClick={() => handleEdit(amendment)}
                        >
                          编辑
                        </Button>
                        <DeleteConfirmButton
                          title="确认删除补充协议"
                          description="仅移出未确认协议草稿；保存结束标记及全部历史，不删除正式协议或文件。"
                          details={[
                            { label: '协议编号', value: amendment.amendment_number || '-' },
                            { label: '本次金额', value: formatMoney(amount) },
                          ]}
                          buttonProps={{
                            type: 'text',
                            size: 'small',
                            danger: true,
                            icon: <DeleteOutlined />,
                          }}
                          buttonText="删除"
                          onConfirm={() => deleteMutation.mutate(amendment.id)}
                        />
                      </>
                    )}
                  </div>
                </header>

                <ContractAmendmentConfirmation contractId={contractId} amendmentId={amendment.id} />

                <div className="contract-amendment-list__amounts">
                  <div>
                    <span>{changeType==='revision'?'修订目标金额':'本次金额'}</span>
                    <strong
                      data-sign={
                        changeType === 'no_amount_change'
                          ? 'neutral'
                          : cents(amount) > 0n
                            ? 'positive'
                            : cents(amount) < 0n
                              ? 'negative'
                              : 'neutral'
                      }
                    >
                      {changeType === 'no_amount_change'
                        ? '不影响金额'
                        : changeType==='revision'?formatMoney(amount):formatSignedMoney(amount)}
                    </strong>
                  </div>
                  <div>
                    <span>协议后预览金额</span>
                    <strong>
                      {amendment.effective_amount === undefined
                        ? '—'
                        : formatMoney(amendment.effective_amount)}
                    </strong>
                  </div>
                </div>

                {changeType !== 'no_amount_change' && (
                  <div className="contract-amendment-list__fee-breakdown">
                    <span className="contract-amendment-list__section-label">金额构成</span>
                    <div className="contract-amendment-list__fee-grid">
                      {feeItems.map((item) => (
                        <div key={item.label}>
                          <span>{item.label}</span>
                          <strong>{formatSignedMoney(item.value)}</strong>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {amendment.description && (
                  <div className="contract-amendment-list__description">
                    <span className="contract-amendment-list__section-label">备注</span>
                    <p>{amendment.description}</p>
                  </div>
                )}

                <section className="contract-amendment-list__files">
                  <ContractMaterials contractId={contractId} kind="amendment" amendmentId={amendment.id} label="协议附件" />
                </section>
              </article>
            )
          })}
        </div>
      )}

      <Modal
        title={editingAmendment ? '编辑补充协议' : '新建补充协议'}
        open={modalVisible}
        onCancel={handleModalClose}
        footer={null}
        width={760}
        destroyOnHidden
      >
        <ContractAmendmentForm
          projectId={projectId}
          contractId={contractId}
          amendmentId={editingAmendment?.id}
          onSuccess={handleSuccess}
          onCancel={handleModalClose}
        />
      </Modal>

    </div>
  )
}
