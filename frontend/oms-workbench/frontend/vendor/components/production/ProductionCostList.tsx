import { useState } from 'react'
import { Button, Drawer, Empty, Spin, message, Modal, Input, Alert, Space } from 'antd'
import {
  CalendarOutlined,
  DownloadOutlined,
  EyeOutlined,
  RightOutlined,
  WalletOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { productionService } from '@/services/production'
import { fileService } from '@/services/file'
import { SectionTitle } from '@/components/shared/SectionTitle'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import { InlineFilePreviewLayout } from '@/components/shared/InlineFilePreviewLayout'
import { ProductionCostForm } from './ProductionCostForm'
import type {
  CostScope,
  File as ProjectFile,
  ProductionCost,
  ProductionCostType,
} from '@/types'
import type { UploadFile } from 'antd'
import dayjs from 'dayjs'
import { formatMoney } from '@/utils/money'
import '../project/ProjectWorkspace.css'
import './ProductionCostList.css'

interface ProductionCostListProps {
  projectId: string | number
  scope?: CostScope
  title?: string
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

const getCostTypeLabel = (costType: ProductionCostType, scope: CostScope) => {
  if (costType === 'expert_fee') {
    return scope === 'business' ? '评标专家费' : '方案评审专家费'
  }

  return COST_TYPE_LABELS[costType]
}

const COST_TYPE_TONES: Record<ProductionCostType, string> = {
  vehicle: 'blue',
  accommodation: 'teal',
  transport: 'amber',
  expert_fee: 'violet',
  bid_signup_fee: 'rose',
  bid_service_fee: 'cyan',
  other: 'slate',
}

const formatFileSize = (bytes?: number): string => {
  if (!bytes) return ''
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${Math.round((bytes / Math.pow(k, i)) * 100) / 100} ${sizes[i]}`
}

export const ProductionCostList = ({
  projectId,
  scope = 'production',
  title,
}: ProductionCostListProps) => {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingCost, setEditingCost] = useState<ProductionCost | null>(null)
  const [previewFile, setPreviewFile] = useState<ProjectFile | null>(null)
  const [previewUploadFile, setPreviewUploadFile] = useState<UploadFile | null>(null)
  const [invoiceRemovedInForm, setInvoiceRemovedInForm] = useState(false)
  const [withdrawReason, setWithdrawReason] = useState('')
  const [withdrawOpen, setWithdrawOpen] = useState(false)
  const [history, setHistory] = useState<any[] | null>(null)
  const queryClient = useQueryClient()
  const isBusiness = scope === 'business'
  const panelTitle = title || (isBusiness ? '经营成本管理' : '生产成本管理')
  const hasPreview = Boolean(previewFile || previewUploadFile)
  const canManage = productionService.canCreate(projectId, scope)

  const {
    data: costs = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['costs', projectId, scope],
    queryFn: () => productionService.listCosts(projectId, scope),
    enabled: !!projectId,
  })

  const { data: statistics } = useQuery({
    queryKey: ['costStatistics', projectId, scope],
    queryFn: () => productionService.getCostStatistics(projectId, scope),
    enabled: !!projectId,
  })

  const deleteMutation = useMutation({
    mutationFn: (costId: string) => productionService.withdrawCost(projectId, costId, scope, withdrawReason),
    onSuccess: () => {
      message.success('已撤回，原金额、附件引用和历史保留')
      setWithdrawOpen(false)
      setDrawerOpen(false)
      setEditingCost(null)
      handleClosePreview()
      queryClient.invalidateQueries({ queryKey: ['costs', projectId, scope] })
      queryClient.invalidateQueries({
        queryKey: ['costStatistics', projectId, scope],
      })
      queryClient.invalidateQueries({ queryKey: ['projectFinancial', projectId] })
    },
    onError: (error: any) => {
      message.error(error?.message || '撤回尚未确认')
    },
  })

  const handleSuccess = () => {
    setDrawerOpen(false)
    setEditingCost(null)
    setInvoiceRemovedInForm(false)
    handleClosePreview()
    refetch()
  }

  const handleCancel = () => {
    setDrawerOpen(false)
    setEditingCost(null)
    setInvoiceRemovedInForm(false)
    handleClosePreview()
  }

  const handleAdd = () => {
    setEditingCost(null)
    setInvoiceRemovedInForm(false)
    handleClosePreview()
    setDrawerOpen(true)
  }

  const handleOpen = (cost: ProductionCost) => {
    setHistory(null)
    setEditingCost(cost)
    setInvoiceRemovedInForm(false)
    handleClosePreview()
    setDrawerOpen(true)
  }

  const handleClosePreview = () => {
    setPreviewFile(null)
    setPreviewUploadFile(null)
  }

  const handlePreviewInvoice = (file: ProjectFile) => {
    setPreviewFile(file)
    setPreviewUploadFile(null)
  }

  const handlePreviewUploadInvoice = (file: UploadFile | null) => {
    setPreviewUploadFile(file)
    if (file) {
      setPreviewFile(null)
    }
  }

  const handleDownloadInvoice = async (file: ProjectFile) => {
    try {
      await fileService.downloadFile(
        file.id,
        file.original_name || file.file_name
      )
    } catch (error: any) {
      message.error(error?.message || '下载失败')
    }
  }

  const visibleCurrentInvoice = !invoiceRemovedInForm
    ? editingCost?.invoice_files?.[0] || editingCost?.invoice_file
    : undefined

  const totalCost = statistics?.total_cost
  const recordCount = statistics?.record_count ?? costs.length

  return (
    <>
      <section className="project-workspace__card cost-panel">
        <header className="cost-panel__header">
          <SectionTitle icon={<WalletOutlined />}>{panelTitle}</SectionTitle>
          {canManage && (
            <CreateActionButton
              className="cost-panel__add-button"
              size="small"
              compact
              onClick={handleAdd}
            >
              记录成本
            </CreateActionButton>
          )}
        </header>

        <div className="cost-panel__summary">
          <div className="cost-panel__summary-item">
            <span>总成本</span>
            <strong>{formatMoney(totalCost)}</strong>
          </div>
          <div className="cost-panel__summary-item">
            <span>记录数</span>
            <strong>{recordCount} 条</strong>
          </div>
          <div className="cost-panel__summary-note">
            {isBusiness
              ? '记录住宿、交通、评标专家费、投标报名费、中标服务费等经营侧支出'
              : '记录饭费、住宿、交通、方案评审专家费等生产侧支出'}
          </div>
        </div>

        {isLoading ? (
          <div className="cost-panel__loading">
            <Spin />
            <span>正在加载成本数据...</span>
          </div>
        ) : error ? (
          <div className="cost-panel__empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="无法加载成本数据，请稍后重试"
            >
              <Button type="primary" onClick={() => refetch()}>
                重新加载
              </Button>
            </Empty>
          </div>
        ) : costs.length === 0 ? (
          <div className="cost-panel__empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={`暂无${panelTitle}记录`}
            >
              {canManage && (
                <CreateActionButton className="cost-panel__add-button" onClick={handleAdd}>
                  记录成本
                </CreateActionButton>
              )}
            </Empty>
          </div>
        ) : (
          <div className="cost-panel__list">
            {costs.map((cost) => {
              const typeTone = COST_TYPE_TONES[cost.cost_type] || 'slate'
              const invoiceFiles = cost.invoice_files?.length
                ? cost.invoice_files
                : cost.invoice_file
                  ? [cost.invoice_file]
                  : []
              const invoice = invoiceFiles[0]
              const invoiceName = invoice?.original_name || invoice?.file_name

              return (
                <button
                  key={cost.id}
                  type="button"
                  className="cost-panel__item"
                  onClick={() => handleOpen(cost)}
                >
                  <div className="cost-panel__item-primary">
                    <span
                      className={`cost-panel__type cost-panel__type--${typeTone}`}
                    >
                      {getCostTypeLabel(cost.cost_type, scope) || cost.cost_type}
                    </span>
                    <div className="cost-panel__item-date">
                      <CalendarOutlined />
                      <span>
                        {cost.incurred_at
                          ? dayjs(cost.incurred_at).format('YYYY-MM-DD')
                          : '未设置时间'}
                      </span>
                    </div>
                  </div>

                  <div className="cost-panel__item-amount">
                    <strong>{formatMoney(cost.amount)}</strong>
                    <span>成本金额</span>
                    {cost.state === 'withdrawn' && <span>已撤回（原金额保留）</span>}
                  </div>

                  <div className="cost-panel__item-summary">
                    <div className="cost-panel__item-summary-line">
                      {cost.description || '暂无描述'}
                    </div>
                    <div className="cost-panel__item-summary-line">
                      {invoiceName
                        ? `发票 ${invoiceName}${
                            invoice?.file_size
                              ? ` · ${formatFileSize(invoice.file_size)}`
                              : ''
                          }${invoiceFiles.length > 1 ? ` 等 ${invoiceFiles.length} 个文件` : ''}`
                        : '未上传发票'}
                    </div>
                  </div>

                  <span className="cost-panel__item-arrow">
                    <RightOutlined />
                  </span>
                </button>
              )
            })}
          </div>
        )}
      </section>

      <Drawer
        zIndex={1000}
        title={editingCost ? `${panelTitle}详情` : `添加${panelTitle}`}
        open={drawerOpen}
        onClose={handleCancel}
        width={hasPreview ? 'calc(100vw - 24px)' : 640}
        destroyOnHidden
        extra={
          editingCost ? <Space>
            {editingCost.can_history && <Button onClick={async()=>{try{setHistory(await productionService.history(projectId,editingCost.id,scope))}catch(e:any){message.error(e.message)}}}>查看成本历史</Button>}
            {editingCost.can_withdraw && <Button danger onClick={()=>{setWithdrawReason('');setWithdrawOpen(true)}}>撤回</Button>}
          </Space> : null
        }
      >
        <InlineFilePreviewLayout
          className="cost-panel__drawer-layout"
          contentClassName="cost-panel__drawer-content"
          file={previewFile}
          uploadFile={previewUploadFile}
          onClosePreview={handleClosePreview}
          defaultPreviewWidth={820}
          minPreviewWidth={780}
          minContentWidth={420}
          minHeight="calc(100vh - 170px)"
          previewHeight="calc(100vh - 170px)"
        >
          {editingCost && <Alert type="info" message={`版本 ${editingCost.revision} · ${editingCost.state==='withdrawn'?'已撤回，只读':'已登记；修改将追加纠正版本'}`} style={{marginBottom:16}} />}
          {history && <div data-cost-history>{history.map(item=><p key={item.snapshot.revision}>版本 {item.snapshot.revision} · {item.operation} · {formatMoney(item.snapshot.amount)} · {item.reason}</p>)}</div>}
          {visibleCurrentInvoice && (
            <div className="cost-panel__drawer-file">
              <span className="cost-panel__drawer-file-name">
                当前发票：{visibleCurrentInvoice.original_name ||
                  visibleCurrentInvoice.file_name}
              </span>
              <div className="cost-panel__drawer-file-actions">
                <Button
                  size="small"
                  icon={<EyeOutlined />}
                  onClick={() => handlePreviewInvoice(visibleCurrentInvoice)}
                >
                  预览
                </Button>
                <Button
                  size="small"
                  icon={<DownloadOutlined />}
                  onClick={() => handleDownloadInvoice(visibleCurrentInvoice)}
                >
                  下载
                </Button>
              </div>
            </div>
          )}
          <ProductionCostForm
            key={editingCost?.id || 'new'}
            projectId={projectId}
            scope={scope}
            initialData={editingCost || undefined}
            readOnly={Boolean(editingCost && !editingCost.can_correct)}
            canManageFiles={productionService.canEvidence(projectId,scope,editingCost?.id)}
            onSuccess={handleSuccess}
            onCancel={handleCancel}
            onPreviewFile={handlePreviewInvoice}
            onPreviewUploadFile={handlePreviewUploadInvoice}
            onDownloadFile={handleDownloadInvoice}
            onClearPreview={handleClosePreview}
            onExistingInvoiceRemoved={() => setInvoiceRemovedInForm(true)}
            onExistingInvoiceRestored={() => setInvoiceRemovedInForm(false)}
          />
        </InlineFilePreviewLayout>
      </Drawer>
      <Modal zIndex={1100} title="撤回成本记录" open={withdrawOpen} onCancel={()=>setWithdrawOpen(false)} onOk={()=>editingCost && deleteMutation.mutate(editingCost.id)} okText="确认撤回" okButtonProps={{danger:true,disabled:!withdrawReason.trim()}} confirmLoading={deleteMutation.isPending}>
        <p>保留原金额、附件引用和历史，不删除文件。</p>
        <label>撤回原因<Input.TextArea aria-label="撤回原因" value={withdrawReason} onChange={event=>setWithdrawReason(event.target.value)} maxLength={500}/></label>
      </Modal>
    </>
  )
}
