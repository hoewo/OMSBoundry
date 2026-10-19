import { useEffect } from 'react'
import { Button, Drawer, Form, Input, Space, message } from 'antd'
import { useMutation } from '@tanstack/react-query'
import { useProductionPersonnelPort, useProductionPersonnelPermissions } from '@/production-personnel-ports'
import type { Discipline, CreateDisciplineRequest, UpdateDisciplineRequest } from '@/production-personnel-ports'

interface DisciplineManageModalProps {
  visible: boolean
  discipline?: Discipline | null
  onCancel: () => void
  onSuccess: () => void
}

export const DisciplineManageModal = ({
  visible,
  discipline,
  onCancel,
  onSuccess,
}: DisciplineManageModalProps) => {
  const [form] = Form.useForm()
  const disciplineService = useProductionPersonnelPort()
  const isEdit = !!discipline

  useEffect(() => {
    if (visible) {
      if (discipline) {
        form.setFieldsValue({
          name: discipline.name,
          description: discipline.description || '',
        })
      } else {
        form.resetFields()
      }
    }
  }, [visible, discipline, form])

  const createMutation = useMutation({
    mutationFn: async (data: CreateDisciplineRequest) => {
      return disciplineService.createDiscipline(data)
    },
    onSuccess: () => {
      message.success('创建专业成功')
      onSuccess()
    },
    onError: (error: any) => {
      message.error(error.message || '创建专业失败')
    },
  })

  const updateMutation = useMutation({
    mutationFn: async (data: UpdateDisciplineRequest) => {
      if (!discipline) throw new Error('专业ID不存在')
      return disciplineService.updateDiscipline(discipline.id, data)
    },
    onSuccess: () => {
      message.success('更新专业成功')
      onSuccess()
    },
    onError: (error: any) => {
      message.error(error.message || '更新专业失败')
    },
  })

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      if (isEdit) {
        await updateMutation.mutateAsync(values)
      } else {
        await createMutation.mutateAsync(values)
      }
    } catch (error) {
      // Form validation error, ignore
    }
  }

  return (
    <Drawer
      title={isEdit ? '编辑专业' : '新增专业'}
      open={visible}
      onClose={onCancel}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onCancel}>取消</Button>
          <Button
            type="primary"
            onClick={handleSubmit}
            loading={createMutation.isPending || updateMutation.isPending}
          >
            保存
          </Button>
        </Space>
      }
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          name: '',
          description: '',
        }}
      >
        <Form.Item
          name="name"
          label="专业名称"
          rules={[{ required: true, message: '请输入专业名称' }]}
        >
          <Input placeholder="请输入专业名称" />
        </Form.Item>
        <Form.Item name="description" label="专业描述">
          <Input.TextArea rows={3} placeholder="请输入专业描述（可选）" />
        </Form.Item>
      </Form>
    </Drawer>
  )
}
