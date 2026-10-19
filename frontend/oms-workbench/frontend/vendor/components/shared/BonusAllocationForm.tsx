import { useMemo, useState } from 'react'
import {
  Alert,
  Button,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Table,
  message,
} from 'antd'
import { DeleteOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { canManageBonus } from '@/services/permission'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import PersonnelSelect from '@/components/shared/PersonnelSelect'
import type {
  CreateBonusAllocationBatchRequest,
  BonusPersonnelOption,
} from '@/types'
import dayjs from 'dayjs'
import { formatAmount } from '@/utils/money'
import { bonusPreview } from '../../../src/bonus-adapter.mjs'
import { cents } from '../../../src/contract-money.mjs'

interface BonusAllocationFormProps {
  projectId: string
  bonusType: 'business' | 'production'
  onSuccess?: () => void
  onCancel?: () => void
}

type PersonnelOption = BonusPersonnelOption

type AllocationRow = {
  key: string
  optionKey?: string
  recipientId?: string
  roleLabel?: string
  personName?: string
  allocationMethod: 'ratio' | 'fixed'
  ratio: number
  fixedAmount: string
}

const createRow = (): AllocationRow => ({
  key: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
  allocationMethod: 'ratio',
  ratio: 0,
  fixedAmount: '0.00',
})

const clampRatio = (value: number, min: number, max: number) =>
  Math.min(Math.max(value, min), max)

export const BonusAllocationForm = ({
  projectId,
  bonusType,
  onSuccess,
  onCancel,
}: BonusAllocationFormProps) => {
  const [form] = Form.useForm()
  const queryClient = useQueryClient()
  const [rows, setRows] = useState<AllocationRow[]>([])

  const { data: canManage } = useQuery({
    queryKey: ['canManageBonus', projectId, bonusType],
    queryFn: () => canManageBonus(projectId, bonusType), enabled: !!projectId,
  })
  const { data: personnelOptions = [], error: candidateError, isLoading: candidatesLoading } = useQuery<PersonnelOption[]>({
    queryKey: ['bonusCandidates', projectId, bonusType],
    queryFn: () => businessService.getBonusCandidates(projectId, bonusType), enabled: !!projectId,
  })
  const baseAmount = Form.useWatch('base_amount', form) || '0.00'

  const selectedOptionKeys = useMemo(
    () => rows.map((row) => row.optionKey).filter(Boolean) as string[],
    [rows]
  )

  const totalRatio = useMemo(
    () =>
      rows.reduce((sum, row) => {
        if (row.allocationMethod !== 'ratio') return sum
        return sum + (Number(row.ratio) || 0)
      }, 0),
    [rows]
  )

  const createMutation = useMutation({
    mutationFn: (data: CreateBonusAllocationBatchRequest) =>
      businessService.createBonusAllocationBatch(projectId, data),
    onSuccess: () => {
      message.success(`${bonusType === 'business' ? '经营' : '生产'}奖金分配已登记（不代表实际付款）`)
      queryClient.invalidateQueries({ queryKey: ['bonusRecords', projectId, bonusType] })
      queryClient.invalidateQueries({
        queryKey: [bonusType === 'production' ? 'productionBonusStatistics' : 'businessBonusStatistics', projectId],
      })
      form.resetFields()
      setRows([])
      onSuccess?.()
    },
    onError: (error: any) => {
      message.error(error.message || '登记未确认')
    },
  })

  const updateRow = (rowKey: string, next: Partial<AllocationRow>) => {
    setRows((prev) => prev.map((row) => (row.key === rowKey ? { ...row, ...next } : row)))
  }

  const handleAddRow = () => {
    setRows((prev) => [...prev, createRow()])
  }

  const handleRemoveRow = (rowKey: string) => {
    setRows((prev) => prev.filter((row) => row.key !== rowKey))
  }

  const handleSelectPerson = (rowKey: string, optionKey: string) => {
    const option = personnelOptions.find((item) => item.uniqueKey === optionKey)
    if (!option) return

    updateRow(rowKey, {
      optionKey: option.uniqueKey,
      recipientId: option.userId,
      roleLabel: option.roleLabel,
      personName: option.name,
    })
  }

  const handleRatioChange = (rowKey: string, nextValue: number | null) => {
    const rawValue = nextValue ?? 0
    const otherRatioTotal = rows.reduce((sum, row) => {
      if (row.key === rowKey || row.allocationMethod !== 'ratio') return sum
      return sum + (Number(row.ratio) || 0)
    }, 0)
    const maxAllowed = Math.max(0, 100 - otherRatioTotal)
    const normalizedValue = clampRatio(rawValue, 0, maxAllowed)

    if (rawValue > maxAllowed) {
      message.warning(`分配比例合计不能超过 100%，当前最多可填写 ${maxAllowed}%`)
    }

    updateRow(rowKey, {
      ratio: normalizedValue,
    })
  }

  const handleFixedAmountChange = (rowKey: string, nextValue: string | null) => {
    updateRow(rowKey, {
      fixedAmount: nextValue || '0.00',
    })
  }

  const handleAllocationMethodChange = (
    rowKey: string,
    allocationMethod: 'ratio' | 'fixed'
  ) => {
    const currentRow = rows.find((row) => row.key === rowKey)
    if (!currentRow) return

    updateRow(rowKey, {
      allocationMethod,
      ratio: allocationMethod === 'ratio' ? currentRow.ratio : 0,
      fixedAmount: allocationMethod === 'fixed' ? currentRow.fixedAmount : '0.00',
    })
  }

  const handleSubmit = async () => {
    if (!canManage) {
      message.error(
        bonusType === 'business' ? '您没有权限管理经营信息' : '您没有权限管理生产信息'
      )
      return
    }

    try {
      const values = await form.validateFields()

      if (rows.length === 0) {
        message.warning('请先添加分配人员')
        return
      }

      const ratioRows = rows.filter((row) => row.allocationMethod === 'ratio')

      if (ratioRows.length > 0 && (!values.base_amount || cents(values.base_amount) <= 0n)) {
        message.warning('存在按比例分配的人员时，请先填写比例池金额')
        return
      }

      if (totalRatio > 100) {
        message.error('分配比例合计不能超过 100%')
        return
      }

      const hasIncompleteRow = rows.some(
        (row) =>
          !row.recipientId ||
          !row.roleLabel ||
          (row.allocationMethod === 'ratio' && row.ratio <= 0) ||
          (row.allocationMethod === 'fixed' && cents(row.fixedAmount) <= 0n)
      )
      if (hasIncompleteRow) {
        message.warning('请完善所有分配行后再登记')
        return
      }

      const payload: CreateBonusAllocationBatchRequest = {
        bonus_category: bonusType,
        base_amount: values.base_amount || '0.00',
        occurred_at: values.occurred_at.format('YYYY-MM-DD'),
        description: values.description,
        items: rows.map((row) => ({
          recipient_id: row.recipientId!,
          assignment_id: row.optionKey!,
          allocation_role: row.roleLabel!,
          allocation_method: row.allocationMethod,
          allocation_ratio: row.allocationMethod === 'ratio' ? row.ratio : 0,
          amount:
            row.allocationMethod === 'ratio'
              ? bonusPreview(values.base_amount, row.ratio)
              : row.fixedAmount,
        })),
      }

      await createMutation.mutateAsync(payload)
    } catch {
      // ignore validation errors
    }
  }

  const columns = [
    {
      title: '工作内容',
      dataIndex: 'roleLabel',
      key: 'roleLabel',
      width: 220,
      render: (_: unknown, record: AllocationRow) => record.roleLabel || '-',
    },
    {
      title: '人员姓名',
      key: 'person',
      width: 260,
      render: (_: unknown, record: AllocationRow) => {
        const currentSelected = record.optionKey
        const available = personnelOptions.filter(
          (option) =>
            !selectedOptionKeys.includes(option.uniqueKey) || option.uniqueKey === currentSelected
        )

        return (
          <PersonnelSelect
            placeholder="请选择人员"
            value={record.optionKey}
            style={{ width: '100%' }}
            onChange={(value) => handleSelectPerson(record.key, value)}
            personnelOptions={available.map((option) => ({
              value: option.uniqueKey,
              label: option.name,
              description: option.roleLabel,
              user: option.user,
            }))}
          />
        )
      },
    },
    {
      title: '分配方式',
      key: 'allocationMethod',
      width: 110,
      render: (_: unknown, record: AllocationRow) => (
        <Select
          value={record.allocationMethod}
          style={{ width: 84 }}
          options={[
            { label: '比例', value: 'ratio' },
            { label: '固定', value: 'fixed' },
          ]}
          onChange={(value) => handleAllocationMethodChange(record.key, value)}
        />
      ),
    },
    {
      title: '分配值',
      key: 'allocationValue',
      width: 190,
      render: (_: unknown, record: AllocationRow) => {
        if (record.allocationMethod === 'fixed') {
          return (
            <div className="bonus-allocation-form__value-shell bonus-allocation-form__value-shell--fixed">
              <span className="bonus-allocation-form__value-affix">¥</span>
              <InputNumber
                min="0"
                controls={false}
                value={record.fixedAmount}
                stringMode
                precision={2}
                className="bonus-allocation-form__value-input"
                style={{ width: '100%' }}
                onChange={(next) => handleFixedAmountChange(record.key, next)}
              />
            </div>
          )
        }

        return (
          <div className="bonus-allocation-form__value-shell bonus-allocation-form__value-shell--ratio">
            <InputNumber
              min={0}
              max={Math.max(
                0,
                100 -
                  rows.reduce((sum, row) => {
                    if (row.key === record.key || row.allocationMethod !== 'ratio') return sum
                    return sum + (Number(row.ratio) || 0)
                  }, 0)
              )}
              controls={false}
              value={record.ratio}
              precision={0}
              className="bonus-allocation-form__value-input"
              style={{ width: '100%' }}
              onChange={(next) => handleRatioChange(record.key, next)}
            />
            <span className="bonus-allocation-form__value-affix">%</span>
          </div>
        )
      },
    },
    {
      title: '预计奖金（元）',
      key: 'amount',
      width: 180,
      render: (_: unknown, record: AllocationRow) =>
        record.allocationMethod === 'ratio'
          ? formatAmount(bonusPreview(baseAmount, record.ratio || 0))
          : formatAmount(record.fixedAmount || '0.00'),
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: unknown, record: AllocationRow) => (
        <DeleteConfirmButton
          title="确认移除分配"
          description="移除后这条奖金分配不会随本次记录提交。"
          details={[{ label: '人员', value: record.personName || '未选择' }]}
          buttonProps={{ type: 'link', icon: <DeleteOutlined /> }}
          buttonText="删除"
          onConfirm={() => handleRemoveRow(record.key)}
        />
      ),
    },
  ]

  if (canManage === false) {
    const operation = businessService.getBonusOperationState(projectId, bonusType)
    if (operation.pending) return <Space direction="vertical" style={{ width: '100%' }}>
      <Alert type="warning" showIcon message={operation.working ? '正在确认原奖金登记，请勿重复提交' : '登记结果尚未确认；关闭表单后继续原奖金操作，不重复登记'} />
      <Button onClick={onCancel}>关闭表单（保留待续命令）</Button>
    </Space>
    return (
      <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
        您没有权限创建{bonusType === 'business' ? '经营' : '生产'}奖金记录
      </div>
    )
  }

  return (
    <Form
      form={form}
      layout="vertical"
      initialValues={{
        occurred_at: dayjs(),
        base_amount: '0.00',
        description: '',
      }}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {candidateError && <Alert type="error" showIcon message="有效分工读取未完成；不会使用旧人员目录代替。" description={candidateError.message} />}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: 16,
          }}
        >
          <Form.Item
            label="比例池金额"
            name="base_amount"
            rules={[
              { validator: async (_rule, value) => { if (cents(value || '0.00') < 0n) throw new Error('比例池金额不能小于 0') } },
            ]}
            style={{ marginBottom: 0 }}
          >
            <InputNumber<string> stringMode min="0" style={{ width: '100%' }} prefix="¥" precision={2} />
          </Form.Item>

          <Form.Item
            label="登记日期"
            name="occurred_at"
            rules={[{ required: true, message: '请选择登记日期' }]}
            style={{ marginBottom: 0 }}
          >
            <DatePicker style={{ width: '100%' }} format="YYYY-MM-DD" />
          </Form.Item>
        </div>

        <Form.Item label="备注" name="description" style={{ marginBottom: 0 }}>
          <Input.TextArea rows={2} placeholder="请输入备注信息（可选）" />
        </Form.Item>

        <Alert
          type="info"
          showIcon
          message="比例池金额只对“按比例”生效，固定金额直接计入本次登记（不代表付款），不占用剩余比例。"
        />

        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <div style={{ color: totalRatio > 100 ? '#ff4d4f' : '#595959' }}>
            当前比例池已分配：{totalRatio}%{' '}
            {totalRatio <= 100 ? `(剩余 ${Math.max(0, 100 - totalRatio)}%)` : ''} · 固定金额不会计入这里
          </div>
          <CreateActionButton
            compact
            onClick={handleAddRow}
            disabled={candidatesLoading || !!candidateError || personnelOptions.length === 0 || selectedOptionKeys.length >= personnelOptions.length}
          >
            添加人员
          </CreateActionButton>
        </div>

        <Table
          rowKey="key"
          columns={columns}
          dataSource={rows}
          pagination={false}
          locale={{ emptyText: '暂无分配人员，请点击“添加人员”' }}
        />

        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Space>
            <Button onClick={onCancel}>取消</Button>
            <Button
              type="primary"
              loading={createMutation.isPending}
              onClick={handleSubmit}
            >
              登记奖金分配
            </Button>
          </Space>
        </div>
      </Space>
    </Form>
  )
}
