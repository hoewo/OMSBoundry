// Original OA InvoiceRecordList; only data/authorization/history ports adapted.
import { useMemo, useState } from 'react'
import { Button, Card, Drawer, Empty, message, Modal, Input } from 'antd'
import { AuditOutlined, DeleteOutlined, RightOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { canManageInvoice } from '@/services/permission'
import { invoiceForProject } from '@/services/registry'
import { OurInvoiceForm } from './OurInvoiceForm'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import type { BusinessChainType, InvoiceView } from '@/types'
import dayjs from 'dayjs'
import {
  getBusinessChainLabel,
  } from '@/utils/businessChain'
import { formatMoney } from '@/utils/money'
import './BusinessTransactionsPanel.css'

interface InvoiceRecordListProps {
  projectId: string
  chainType?: BusinessChainType
  title?: string
  subtitle?: string
  hideCard?: boolean
  groupTitle?: string
  groupDescription?: string
  groupKicker?: string
  groupTone?: 'client' | 'income'
}

const getFeeTypeLabel = (feeType?: string) => {
  switch (feeType) {
    case 'design_fee':
      return '设计费'
    case 'survey_fee':
      return '勘察费'
    case 'consultation_fee':
      return '咨询费'
    default:
      return feeType || '-'
  }
}

const getFeeTypeTone = (feeType?: string) => {
  switch (feeType) {
    case 'design_fee':
      return 'design'
    case 'survey_fee':
      return 'survey'
    case 'consultation_fee':
      return 'consultation'
    default:
      return 'default'
  }
}

export const InvoiceRecordList = ({
  projectId,
  chainType,
  title,
  subtitle,
  hideCard = false,
  groupTitle,
  groupDescription,
  groupKicker,
  groupTone = 'client',
}: InvoiceRecordListProps) => {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingRecord, setEditingRecord] = useState<InvoiceView | null>(
    null
  )
  const [previewExpanded, setPreviewExpanded] = useState(false)
  const queryClient = useQueryClient()

  const { data: canManage } = useQuery({
    queryKey: ['canManageInvoice', projectId],
    queryFn: () => canManageInvoice(projectId),
    enabled: !!projectId,
  })

  const { data: financial, isLoading } = useQuery({
    queryKey: ['projectInvoices', projectId],
    queryFn: () => businessService.getProjectInvoices(projectId),
    enabled: !!projectId,
  })

  const invoiceRecords = useMemo(() => (financial?.invoice_records || []).filter(record => !chainType || record.chain_type === chainType), [chainType, financial?.invoice_records])
  const [invalidating,setInvalidating]=useState(false),[reason,setReason]=useState('')
  const invalidate=useMutation({mutationFn:()=>businessService.invalidateInvoice(projectId,editingRecord!.id,reason),onSuccess:()=>{setInvalidating(false);setDrawerOpen(false);void queryClient.invalidateQueries({queryKey:['projectInvoices',projectId]})},onError:(e:Error)=>message.error(e.message)})
  const adapter=invoiceForProject(projectId)
  const handleCreate = () => {
    setEditingRecord(null)
    setDrawerOpen(true)
  }

  const handleEdit = (record: InvoiceView) => {
    setEditingRecord(record)
    setDrawerOpen(true)
  }

  const handleClose = () => {
    setDrawerOpen(false)
    setEditingRecord(null)
    setPreviewExpanded(false)
  }

  const handleSuccess = () => {
    setDrawerOpen(false)
    setEditingRecord(null)
    setPreviewExpanded(false)
    queryClient.invalidateQueries({ queryKey: ['projectInvoices', projectId] })
  }

  const listContent = (
    <div className="transaction-panel__group-body">
      {isLoading ? (
        <div className="transaction-panel__loading">加载中...</div>
      ) : invoiceRecords.length === 0 ? (
        <div className="transaction-panel__empty">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前还没有开票记录"
          />
        </div>
      ) : (
        <div className="transaction-panel__list">
          {invoiceRecords.map((record) =>
            (() => {
              return (
                <button
                  key={record.id}
                  type="button"
                  className={`transaction-panel__item transaction-panel__item--${getFeeTypeTone(record.fee_type)}`}
                  onClick={() => handleEdit(record)}
                >
                  <div className="transaction-panel__item-primary">
                    <div className="transaction-panel__fee-badge">
                      {getFeeTypeLabel(record.fee_type)}
                    </div>
                    <div className="transaction-panel__item-date">
                      <span>
                        {record.occurred_at
                          ? dayjs(record.occurred_at).format('YYYY-MM-DD')
                          : '-'}
                      </span>
                    </div>
                  </div>

                  <div className="transaction-panel__item-amount">
                    <strong>{record.amount_visible?formatMoney(record.amount):'无金额权限'}</strong>
                    <span>开票金额</span>
                  </div>

                  <div className="transaction-panel__item-summary">
                    <div className="transaction-panel__item-summary-line">
                      {record.recipient.legal_name}
                    </div>
                    <div className="transaction-panel__item-summary-line">
                      {record.status==='registration_invalidated'?'登记无效（保留历史）':`发票文件 ${record.invoice_files?.length||0} 个 · revision ${record.revision}`}
                    </div>
                  </div>

                  <span className="transaction-panel__item-arrow">
                    <RightOutlined />
                  </span>
                </button>
              )
            })()
          )}
        </div>
      )}

      <Drawer
        title={editingRecord ? '开票记录详情' : '新增开票记录'}
        open={drawerOpen}
        onClose={handleClose}
        width={previewExpanded ? 'min(1320px, calc(100vw - 48px))' : 640}
        destroyOnHidden
        extra={
          editingRecord?.can_invalidate ? <Button danger icon={<DeleteOutlined />} onClick={()=>{setReason('');setInvalidating(true)}}>登记无效</Button> : null
        }
      >
        <OurInvoiceForm
          projectId={projectId}
          recordId={editingRecord?.id}
          chainType={chainType}
          onPreviewStateChange={setPreviewExpanded}
          onSuccess={handleSuccess}
          onCancel={handleClose}
        />
      </Drawer>
      <Modal title="将本开票登记标记为无效？" open={invalidating} onCancel={()=>setInvalidating(false)} onOk={()=>invalidate.mutate()} okText="确认登记无效" okButtonProps={{disabled:!reason.trim()||!!adapter.pending,loading:invalidate.isPending}}><p>保留原登记、金额及历史；这不是税务作废或删除文件。</p><Input.TextArea aria-label="登记无效原因" value={reason} onChange={e=>setReason(e.target.value)} placeholder="必填：登记无效原因" maxLength={1000} /></Modal>
    </div>
  )

  if (hideCard) {
    return (
      <section
        className={[
          'business-panel-surface__group',
          'transaction-panel__group',
          `transaction-panel__group--${groupTone}`,
        ].join(' ')}
      >
        <div className="transaction-panel__group-head">
          <div className="transaction-panel__group-heading">
            {groupKicker && (
              <span className="transaction-panel__group-kicker">
                {groupKicker}
              </span>
            )}
            <h4 className="business-panel-surface__group-title">
              {groupTitle || getBusinessChainLabel(chainType)}
            </h4>
            {(groupDescription || subtitle) && (
              <p className="business-panel-surface__group-desc">
                {groupDescription || subtitle}
              </p>
            )}
          </div>
          <div className="transaction-panel__group-actions">
            <span className="transaction-panel__group-count">
              {invoiceRecords.length} 条记录
            </span>
            {canManage && (
              <CreateActionButton size="small" compact onClick={handleCreate}>
                新增开票
              </CreateActionButton>
            )}
          </div>
        </div>
        {listContent}
      </section>
    )
  }

  return (
    <Card
      title={
        <SectionTitle icon={<AuditOutlined />}>
          {title || '开票记录'}
        </SectionTitle>
      }
      extra={
        canManage && (
          <CreateActionButton size="small" compact onClick={handleCreate}>
            添加开票记录
          </CreateActionButton>
        )
      }
    >
      {listContent}
    </Card>
  )
}

