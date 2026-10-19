import { useState } from 'react'
import { Alert, Select, Button, Space, message } from 'antd'
import { EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useProductionPersonnelPort, useProductionPersonnelPermissions } from '@/production-personnel-ports'
import { DisciplineManageModal } from './DisciplineManageModal'
import { DeleteConfirmButton } from '@/components/shared/ConfirmDeleteDialog'
import { CreateActionButton } from '@/components/shared/CreateActionButton'
import type { Discipline } from '@/production-personnel-ports'

const { Option } = Select

interface DisciplineSelectorProps {
  id?: string // Form supplies this for label focus and accessible naming.
  value?: string
  onChange?: (value: string) => void
  placeholder?: string
  allowManage?: boolean // 是否允许管理专业（新增、编辑、删除）
  disabled?: boolean // 是否禁用
  onDisciplineChange?: (discipline: Discipline | null) => void
}

export const DisciplineSelector = ({
  id,
  value,
  onChange,
  placeholder = '请选择专业',
  allowManage = true,
  disabled = false,
  onDisciplineChange,
}: DisciplineSelectorProps) => {
  const disciplineService = useProductionPersonnelPort()
  const { canManageDisciplines } = useProductionPersonnelPermissions(disciplineService.projectID)
  const [manageModalVisible, setManageModalVisible] = useState(false)
  const [editingDiscipline, setEditingDiscipline] = useState<Discipline | null>(null)
  const queryClient = useQueryClient()

  // 获取专业列表
  const { data: disciplines = [], isError: catalogFailed, isPending: catalogPending, refetch } = useQuery({
    queryKey: ['disciplines', { includeInactive: false }],
    queryFn: () => disciplineService.listDisciplines(false),
  })
  const selectedDiscipline = disciplines.find((d) => d.id === value)

  const handleSelectChange = (selectedValue: string) => {
    onChange?.(selectedValue)
    const discipline = disciplines.find((d) => d.id === selectedValue) || null
    onDisciplineChange?.(discipline)
  }

  const handleAdd = () => {
    setEditingDiscipline(null)
    setManageModalVisible(true)
  }

  const handleEdit = () => {
    if (!value) {
      message.warning('请先选择专业')
      return
    }
    const discipline = disciplines.find((d) => d.id === value)
    if (discipline) {
      setEditingDiscipline(discipline)
      setManageModalVisible(true)
    }
  }

  const handleDelete = async () => {
    if (!value) {
      message.warning('请先选择专业')
      return
    }
    try {
      await disciplineService.deleteDiscipline(value)
      message.success('删除专业成功')
      // 使查询缓存失效，触发重新获取
      queryClient.invalidateQueries({
        queryKey: ['disciplines'],
      })
      onChange?.(undefined as any)
      onDisciplineChange?.(null)
    } catch (error: any) {
      message.error(error.message || '删除专业失败')
    }
  }

  const handleManageSuccess = () => {
    // 使查询缓存失效，触发重新获取
    queryClient.invalidateQueries({
      queryKey: ['disciplines'],
    })
    setManageModalVisible(false)
    setEditingDiscipline(null)
  }

  return (
    <>
      {catalogFailed && <Alert type="error" showIcon message="专业目录读取失败，不作为空目录" action={<Button onClick={() => void refetch()}>重新读取专业</Button>} style={{marginBottom:12}}/>}
      <Space.Compact style={{ width: '100%' }}>
        <Select
          id={id}
          value={value}
          onChange={handleSelectChange}
          placeholder={placeholder}
          style={{ flex: 1 }}
          showSearch
          disabled={disabled || catalogPending || catalogFailed}
          filterOption={(input, option) =>
            (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
          }
          options={disciplines.map((d) => ({
            label: d.name,
            value: d.id,
          }))}
        />
        {allowManage && canManageDisciplines && (
          <>
            <CreateActionButton
              onClick={handleAdd}
              title="新增专业"
              aria-label="新增专业"
            />
            <Button
              icon={<EditOutlined />}
              onClick={handleEdit}
              disabled={!value}
              title="编辑专业"
            />
            <DeleteConfirmButton
              title="确认删除专业"
              description="删除后该专业将从专业库中移除，请确认不再需要。"
              details={[
                { label: '专业', value: selectedDiscipline?.name || value || '-' },
              ]}
              buttonProps={{
                type: 'default',
                icon: <DeleteOutlined />,
                disabled: !value,
                title: '删除专业',
              }}
              buttonText={null}
              onConfirm={handleDelete}
            />
          </>
        )}
      </Space.Compact>

      <DisciplineManageModal
        visible={manageModalVisible}
        discipline={editingDiscipline}
        onCancel={() => {
          setManageModalVisible(false)
          setEditingDiscipline(null)
        }}
        onSuccess={handleManageSuccess}
      />
    </>
  )
}
