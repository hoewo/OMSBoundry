import { useState } from 'react'
import { Button, Empty, Spin, message, Input, Drawer, Space, Tag } from 'antd'
import { CalendarOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import type { ManualBonusView } from '@/types'
import dayjs from 'dayjs'
import { formatMoney } from '@/utils/money'
import PersonnelName, { getPersonnelName } from './PersonnelName'

interface BonusAllocationListProps {
  projectId: string
  bonusType: 'business' | 'production'
  canManage?: boolean
}

/**
 * 可复用的奖金分配列表组件
 * 包含奖金记录列表、加载状态、空状态、错误状态
 * T498: 创建可复用的奖金分配列表组件
 */
export const BonusAllocationList = ({
  projectId,
  bonusType,
  canManage = false,
}: BonusAllocationListProps) => {
  const queryClient = useQueryClient()
  const [reason, setReason] = useState('')
  const [history, setHistory] = useState<any[] | null>(null)

  const { data: financial, isLoading, error } = useQuery({
    queryKey: ['bonusRecords', projectId, bonusType],
    queryFn: () => businessService.getBonusRecords(projectId, bonusType),
    enabled: !!projectId,
  })

  const bonusRecords = financial || []

  const deleteMutation = useMutation({
    mutationFn: (recordId: string) =>
      businessService.withdrawBonusAllocation(projectId, bonusType, recordId, reason),
    onSuccess: () => {
      message.success('奖金分配已撤回，历史保留；不代表撤销实际付款')
      queryClient.invalidateQueries({
        queryKey: ['bonusRecords', projectId, bonusType],
      })
      // Invalidate statistics query
      queryClient.invalidateQueries({
        queryKey: [bonusType === 'production' ? 'productionBonusStatistics' : 'businessBonusStatistics', projectId],
      })
    },
    onError: (error: any) => {
      message.error(error.message || '撤回未确认')
    },
  })

  const handleDelete = (recordId: string) => {
    deleteMutation.mutate(recordId)
  }

  const getRecipientName = (record: ManualBonusView) =>
    record.recipient
      ? getPersonnelName(record.recipient, '-')
      : '-'

  const getRecipientInitial = (name: string) =>
    name && name !== '-' ? name.trim().slice(0, 1) : '?'

  // Error state
  if (error) {
    return (
      <div className="bonus-allocation__feedback">
        <p>
          加载失败，请稍后重试
        </p>
        <Button
          onClick={() => {
            queryClient.invalidateQueries({
              queryKey: ['bonusRecords', projectId, bonusType],
            })
          }}
        >
          重试
        </Button>
      </div>
    )
  }

  // Empty state
  if (!isLoading && bonusRecords.length === 0) {
    return (
      <div className="bonus-allocation__empty">
        <Empty
          description="暂无奖金分配记录"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="bonus-allocation__loading">
        <Spin />
        <span>正在加载奖金分配...</span>
      </div>
    )
  }

  return (
    <><div className="bonus-allocation__list">
      {bonusRecords.map((record) => {
        const allocationMethod =
          record.allocation_method === 'fixed' ? '固定金额' : '按比例'
        const allocationValue =
          record.allocation_method === 'fixed'
            ? formatMoney(record.amount)
            : typeof record.allocation_ratio === 'number'
              ? `${record.allocation_ratio}%`
              : '-'
        const recipientName = getRecipientName(record)

        return (
          <article className="bonus-allocation__item" key={record.id}>
            <div className="bonus-allocation__cell bonus-allocation__cell--recipient">
              <span className="bonus-allocation__label">分配人员</span>
              {record.state === 'withdrawn' && <Tag>已撤回 · 历史保留</Tag>}
              <strong className="bonus-allocation__person">
                <span className="bonus-allocation__avatar">
                  {getRecipientInitial(recipientName)}
                </span>
                <span className="bonus-allocation__person-name">
                  {record.recipient ? (
                    <PersonnelName user={record.recipient} />
                  ) : (
                    recipientName
                  )}
                </span>
              </strong>
            </div>
            <div className="bonus-allocation__cell bonus-allocation__cell--work">
              <span className="bonus-allocation__label">工作内容</span>
              <strong>{record.allocation_role || '-'}</strong>
            </div>
            <div className="bonus-allocation__cell bonus-allocation__cell--date">
              <span className="bonus-allocation__label">登记日期</span>
              <strong>
                <CalendarOutlined />
                {record.occurred_at
                  ? dayjs(record.occurred_at).format('YYYY-MM-DD')
                  : '-'}
              </strong>
            </div>
            <div className="bonus-allocation__cell bonus-allocation__cell--ratio">
              <span className="bonus-allocation__label">分配方式</span>
              <strong>{allocationMethod}</strong>
            </div>
            <div className="bonus-allocation__cell">
              <span className="bonus-allocation__label">分配值</span>
              <strong>{allocationValue}</strong>
            </div>
            <div className="bonus-allocation__cell">
              <span className="bonus-allocation__label">比例池金额</span>
              <strong>
                {record.allocation_method === 'fixed'
                  ? '-'
                  : formatMoney(record.allocation_base_amount)}
              </strong>
            </div>
            <div className="bonus-allocation__cell bonus-allocation__cell--amount">
              <span className="bonus-allocation__label">最终奖金</span>
              <strong>{formatMoney(record.amount)}</strong>
            </div>
            {(record.can_withdraw || record.can_history) && (
              <div className="bonus-allocation__actions"><Space direction="vertical">
                {record.can_history && <Button type="text" onClick={async () => { try { setHistory(await businessService.getBonusHistory(projectId, bonusType, record.id)) } catch (error: any) { message.error(error.message || '历史读取未完成') } }}>历史</Button>}
                {record.can_withdraw &&
                <DeleteConfirmButton
                  title="确认撤回奖金分配"
                  description="撤回保留原分配与历史，不删除人员记录，也不撤销实际付款。"
                  confirmText="确认撤回"
                  confirmDisabled={!reason.trim()}
                  details={[
                    { label: '人员', value: recipientName },
                    { label: '最终奖金', value: formatMoney(record.amount) },
                  ]}
                  buttonProps={{ type: 'text', icon: <DeleteOutlined />, onClick: () => setReason('') }}
                  loading={deleteMutation.isPending}
                  buttonText="撤回"
                  onConfirm={() => handleDelete(record.id)}
                ><label>撤回原因（必填）<Input.TextArea value={reason} onChange={event => setReason(event.target.value)} maxLength={500} /></label></DeleteConfirmButton>}
              </Space></div>
            )}
          </article>
        )
      })}
    </div><Drawer title="奖金分配历史" open={history !== null} onClose={() => setHistory(null)} width={860} zIndex={1000} destroyOnHidden>
      {history?.map((entry: any) => <section key={entry.snapshot.revision} className="bonus-allocation__item"><div>revision {entry.snapshot.revision} · {entry.operation === 'record' ? '登记' : '撤回'} · {entry.snapshot.state}</div><div>{entry.snapshot.recipient.real_name} · {entry.snapshot.allocation_role}</div><div>{formatMoney(entry.snapshot.amount)} · {entry.snapshot.occurred_at}</div><div>{entry.reason || entry.snapshot.description || '无备注'}</div></section>)}
    </Drawer></>
  )
}
